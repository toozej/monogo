package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/providers"
	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"github.com/toozej/monogo/apps/gocicle/internal/service"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"golang.org/x/oauth2"
)

func (s *Server) publicProviders(w http.ResponseWriter, r *http.Request) {
	var rows []storage.Provider
	if err := s.Service.DB.Select("id,name,kind").Find(&rows).Error; err != nil {
		failure(w, err)
		return
	}
	respond(w, rows)
}
func (s *Server) beginLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != s.Config.PublicURL {
		failure(w, service.ErrForbidden)
		return
	}
	var v struct {
		Hint       string `json:"hint"`
		Invitation string `json:"invitation"`
		Link       bool   `json:"link"`
	}
	if err := decode(r, &v); err != nil {
		failure(w, err)
		return
	}
	var actor *service.Actor
	if v.Link {
		a, err := s.actor(r)
		if err != nil || !a.Session || security.Hash(r.Header.Get("X-CSRF-Token")) != a.CSRFHash {
			failure(w, service.ErrForbidden)
			return
		}
		actor = &a
	}
	location, state, err := s.Service.BeginLogin(r.Context(), r.PathValue("provider"), s.Config.PublicURL, v.Hint, v.Invitation, actor)
	if err != nil {
		failure(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "__Host-gocicle-oauth", Value: state, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	respond(w, map[string]string{"url": location})
}
func (s *Server) completeLogin(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie("__Host-gocicle-oauth")
	if err != nil || state == "" || cookie.Value != state {
		failure(w, service.ErrForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "__Host-gocicle-oauth", Path: "/", Secure: true, HttpOnly: true, MaxAge: -1, SameSite: http.SameSiteLaxMode})
	token, csrf, err := s.Service.CompleteLogin(r.Context(), r.PathValue("provider"), s.Config.PublicURL, state, r.URL.Query().Get("code"), r.URL.Query().Get("iss"))
	if err != nil {
		failure(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "__Host-gocicle", Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 86400})
	// The browser must read this CSRF token and submit it as a header. The session cookie remains HttpOnly.
	// nosemgrep: go.lang.security.audit.net.cookie-missing-httponly.cookie-missing-httponly
	http.SetCookie(w, &http.Cookie{Name: "__Host-gocicle-csrf", Value: csrf, Path: "/", Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: 86400}) // #nosec G124 -- This cookie contains only the browser-readable CSRF token.
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func (s *Server) clientMetadata(w http.ResponseWriter, r *http.Request) {
	var rows []storage.Provider
	if err := s.Service.DB.Where("kind='tangled'").Find(&rows).Error; err != nil {
		failure(w, err)
		return
	}
	callbacks := []string{}
	for _, row := range rows {
		callbacks = append(callbacks, s.Config.PublicURL+"/auth/"+row.ID+"/callback")
	}
	metadata := providers.ClientMetadata(s.Config.PublicURL, "")
	metadata["redirect_uris"] = callbacks
	respond(w, metadata)
}

// secrets handles the documented API operations.
// @Summary Secrets
// @Security Bearer
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /secrets [get]
func (s *Server) secrets(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	if !a.Can("read") {
		return service.ErrForbidden
	}
	var rows []storage.Secret
	err := s.Service.DB.Where("owner_id=?", a.User.ID).Order("id").Limit(100).Offset(offset(r)).Find(&rows).Error
	if err == nil {
		respond(w, rows)
	}
	return err
}

// createSecret handles the documented API operations.
// @Summary Create secret
// @Security Bearer
// @Produce json
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /secrets [post]
func (s *Server) createSecret(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v struct {
		Name  string          `json:"name"`
		Kind  string          `json:"kind"`
		Value json.RawMessage `json:"value"`
	}
	if err := decode(r, &v); err != nil {
		return err
	}
	secret, err := s.Service.CreateSecret(a, v.Name, v.Kind, v.Value)
	if err == nil {
		respond(w, secret)
	}
	return err
}

// deleteSecret handles the documented API operations.
// @Summary Delete secret
// @Security Bearer
// @Produce json
// @Param secret path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /secrets/{secret} [delete]
func (s *Server) deleteSecret(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	if !a.Can("write") {
		return service.ErrForbidden
	}
	q := s.Service.DB.Where("id=? AND owner_id=? AND version=?", r.PathValue("secret"), a.User.ID, revision(r)).Delete(&storage.Secret{})
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return service.ErrConflict
	}
	respond(w, map[string]bool{"deleted": true})
	return nil
}

// grant handles the documented API operations.
// @Summary Grant
// @Security Bearer
// @Produce json
// @Param project path string true "Record identifier"
// @Param secret path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /secrets/{secret}/grants/{project} [put]
func (s *Server) grant(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v struct {
		Allow bool `json:"allow"`
	}
	if err := decode(r, &v); err != nil {
		return err
	}
	err := s.Service.Grant(a, r.PathValue("secret"), r.PathValue("project"), v.Allow)
	if err == nil {
		respond(w, v)
	}
	return err
}

// listResources handles the documented API operations.
// @Summary List resources
// @Security Bearer
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /connections [get]
// @Router /ssh-keys [get]
// @Router /notifications [get]
func (s *Server) listResources(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	table := service.ResourceTable(r.PathValue("resource"))
	if table == "" || !a.Can("read") {
		return service.ErrForbidden
	}
	var rows []storage.Resource
	err := s.Service.DB.Table(table).Where("owner_id=?", a.User.ID).Order("id").Limit(100).Offset(offset(r)).Find(&rows).Error
	if err == nil {
		respond(w, rows)
	}
	return err
}

// saveResource handles the documented API operations.
// @Summary Save resource
// @Security Bearer
// @Produce json
// @Param id path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /connections [post]
// @Router /ssh-keys [post]
// @Router /notifications [post]
// @Router /connections/{id} [put]
// @Router /ssh-keys/{id} [put]
// @Router /notifications/{id} [put]
func (s *Server) saveResource(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v storage.Resource
	if err := decode(r, &v); err != nil {
		return err
	}
	v.ID = r.PathValue("id")
	if v.ID != "" {
		v.Version = revision(r)
	}
	row, err := s.Service.SaveResource(a, r.PathValue("resource"), v)
	if err == nil {
		respond(w, row)
	}
	return err
}

// deleteResource handles the documented API operations.
// @Summary Delete resource
// @Security Bearer
// @Produce json
// @Param id path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /connections/{id} [delete]
// @Router /ssh-keys/{id} [delete]
// @Router /notifications/{id} [delete]
func (s *Server) deleteResource(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	table := service.ResourceTable(r.PathValue("resource"))
	if table == "" || !a.Can("write") {
		return service.ErrForbidden
	}
	q := s.Service.DB.Table(table).Where("id=? AND owner_id=? AND version=?", r.PathValue("id"), a.User.ID, revision(r)).Delete(&storage.Resource{})
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return service.ErrConflict
	}
	respond(w, map[string]bool{"deleted": true})
	return nil
}

// testNotification handles the documented API operations.
// @Summary Test notification
// @Security Bearer
// @Produce json
// @Param id path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /notifications/{id}/test [post]
func (s *Server) testNotification(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	err := s.Service.TestNotification(r.Context(), a, r.PathValue("id"), s.Config.NotificationHosts)
	if err == nil {
		respond(w, map[string]bool{"sent": true})
	}
	return err
}

// repositories handles the documented API operations.
// @Summary Repositories
// @Security Bearer
// @Produce json
// @Param id path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /connections/{id}/repositories [get]
// @Router /connections/{id}/test [post]
func (s *Server) repositories(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	if !a.Can("write") {
		return service.ErrForbidden
	}
	var row struct{ ProviderID, SecretID string }
	if err := s.Service.DB.Table("connections").Where("id=? AND owner_id=?", r.PathValue("id"), a.User.ID).Take(&row).Error; err != nil {
		return service.ErrForbidden
	}
	adapter, err := s.Service.Provider(row.ProviderID, s.Config.PublicURL)
	if err != nil {
		return err
	}
	plain, err := s.Service.Decrypt(row.SecretID)
	if err != nil {
		return err
	}
	var secret struct{ Value, Subject, PDS string }
	if json.Unmarshal(plain, &secret) != nil {
		return service.ErrInvalid
	}
	token := (&oauth2.Token{AccessToken: secret.Value}).WithExtra(map[string]any{"subject": secret.Subject, "pds": secret.PDS})
	repos, err := adapter.Repositories(r.Context(), token, r.URL.Query().Get("cursor"))
	if err == nil {
		respond(w, repos)
	}
	return err
}

// users handles the documented API operations.
// @Summary Users
// @Security Bearer
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/users [get]
func (s *Server) users(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	if err := service.Admin(a); err != nil {
		return err
	}
	var rows []storage.User
	err := s.Service.DB.Order("id").Limit(100).Offset(offset(r)).Find(&rows).Error
	if err == nil {
		respond(w, rows)
	}
	return err
}

// updateUser handles the documented API operations.
// @Summary Update user
// @Security Bearer
// @Produce json
// @Param id path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/users/{id} [put]
func (s *Server) updateUser(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var user storage.User
	if err := decode(r, &user); err != nil {
		return err
	}
	user.ID = r.PathValue("id")
	user.Version = revision(r)
	err := s.Service.UpdateUser(a, user)
	if err == nil {
		respond(w, user)
	}
	return err
}

// invite handles the documented API operations.
// @Summary Invite
// @Security Bearer
// @Produce json
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/invitations [post]
func (s *Server) invite(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	token, err := s.Service.Invite(a)
	if err == nil {
		respond(w, map[string]string{"invitation": token})
	}
	return err
}

// runners handles the documented API operations.
// @Summary Runners
// @Security Bearer
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/runners [get]
func (s *Server) runners(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	if err := service.Admin(a); err != nil {
		return err
	}
	var rows []storage.Runner
	err := s.Service.DB.Order("id").Limit(100).Offset(offset(r)).Find(&rows).Error
	if err == nil {
		respond(w, rows)
	}
	return err
}

// updateRunner handles the documented API operations.
// @Summary Update runner
// @Security Bearer
// @Produce json
// @Param id path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/runners/{id} [put]
func (s *Server) updateRunner(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var runner storage.Runner
	if err := decode(r, &runner); err != nil {
		return err
	}
	runner.ID = r.PathValue("id")
	runner.Version = revision(r)
	err := s.Service.UpdateRunner(a, runner)
	if err == nil {
		respond(w, runner)
	}
	return err
}

// runnerGrant handles the documented API operations.
// @Summary Runner grant
// @Security Bearer
// @Produce json
// @Param id path string true "Record identifier"
// @Param user path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/runners/{id}/grants/{user} [put]
func (s *Server) runnerGrant(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v struct {
		Allow bool `json:"allow"`
	}
	if err := decode(r, &v); err != nil {
		return err
	}
	err := s.Service.RunnerGrant(a, r.PathValue("id"), r.PathValue("user"), v.Allow)
	if err == nil {
		respond(w, v)
	}
	return err
}

// enrollment handles the documented API operations.
// @Summary Enrollment
// @Security Bearer
// @Produce json
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/enrollments [post]
func (s *Server) enrollment(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v struct {
		Name          string   `json:"name"`
		Labels        []string `json:"labels"`
		ApprovedRoots []string `json:"approvedRoots"`
	}
	if err := decode(r, &v); err != nil {
		return err
	}
	token, err := s.Service.EnrollToken(a, v.Name, v.Labels, v.ApprovedRoots)
	if err == nil {
		respond(w, map[string]string{"token": token})
	}
	return err
}

// saveProvider handles the documented API operations.
// @Summary Save provider
// @Security Bearer
// @Produce json
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/providers [post]
// @Router /admin/providers/{id} [put]
func (s *Server) saveProvider(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	if err := service.Admin(a); err != nil {
		return err
	}
	var row storage.Provider
	if err := decode(r, &row); err != nil {
		return err
	}
	if id := r.PathValue("id"); id != "" {
		row.ID = id
		row.Version = revision(r)
	}
	row, err := s.Service.SaveProvider(a, row)
	if err != nil {
		return err
	}
	respond(w, row)
	return nil
}

// exportConfiguration handles the documented API operations.
// @Summary Export configuration
// @Security Bearer
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/configuration [get]
func (s *Server) exportConfiguration(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	out, err := s.Service.ExportConfiguration(a)
	if err == nil {
		respond(w, out)
	}
	return err
}

// importConfiguration handles the documented API operations.
// @Summary Import configuration
// @Security Bearer
// @Produce json
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/configuration [post]
func (s *Server) importConfiguration(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v struct {
		Configuration service.Configuration `json:"configuration"`
		Credentials   map[string]string     `json:"credentials"`
	}
	if err := decode(r, &v); err != nil {
		return err
	}
	err := s.Service.ImportConfiguration(a, r.Header.Get("Idempotency-Key"), v.Configuration, v.Credentials)
	if err == nil {
		respond(w, map[string]bool{"imported": true})
	}
	return err
}

// audit handles the documented API operations.
// @Summary Audit
// @Security Bearer
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/audit [get]
func (s *Server) audit(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	if err := service.Admin(a); err != nil {
		return err
	}
	var rows []struct {
		ID, ActorID, Action, Target string
		CreatedAt                   time.Time
	}
	err := s.Service.DB.Table("audit_events").Order("created_at DESC,id").Limit(100).Offset(offset(r)).Find(&rows).Error
	if err == nil {
		respond(w, rows)
	}
	return err
}

// linkImportedIdentity handles the documented API operations.
// @Summary Link imported identity
// @Security Bearer
// @Produce json
// @Param id path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/users/{id}/identity [post]
func (s *Server) linkImportedIdentity(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v struct{ ProviderID, Subject string }
	if err := decode(r, &v); err != nil {
		return err
	}
	err := s.Service.LinkImportedIdentity(a, r.PathValue("id"), v.ProviderID, v.Subject)
	if err == nil {
		respond(w, map[string]bool{"linked": true})
	}
	return err
}
