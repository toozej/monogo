package server

import (
	"net/http"

	"github.com/toozej/monogo/apps/gocicle/internal/service"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
)

// tokens lists token metadata without bearer values.
// @Summary List API token metadata
// @Security Bearer
// @Produce json
// @Success 200 {array} service.TokenMetadata
// @Router /tokens [get]
func (s *Server) tokens(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	rows, err := s.Service.Tokens(a, offset(r))
	if err == nil {
		respond(w, rows)
	}
	return err
}

// revokeToken revokes an owned automation credential.
// @Summary Revoke API token
// @Security Bearer
// @Param id path string true "Token identifier"
// @Success 200 {object} map[string]bool
// @Router /tokens/{id} [delete]
func (s *Server) revokeToken(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	err := s.Service.RevokeToken(a, r.PathValue("id"))
	if err == nil {
		respond(w, map[string]bool{"revoked": true})
	}
	return err
}

// renameProject changes the portable project name.
// @Summary Rename project
// @Security Bearer
// @Param project path string true "Project identifier"
// @Param If-Match header string true "Current project version"
// @Param body body map[string]string true "Name"
// @Success 200 {object} map[string]int64
// @Router /projects/{project} [put]
func (s *Server) renameProject(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var input struct{ Name string }
	if err := decode(r, &input); err != nil {
		return err
	}
	err := s.Service.RenameProject(a, r.PathValue("project"), input.Name, revision(r))
	if err == nil {
		respond(w, map[string]int64{"version": revision(r) + 1})
	}
	return err
}

// createUser creates a disabled account without an identity.
// @Summary Create disabled user
// @Security Bearer
// @Param body body map[string]string true "Name"
// @Success 200 {object} storage.User
// @Router /admin/users [post]
func (s *Server) createUser(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var input struct{ Name string }
	if err := decode(r, &input); err != nil {
		return err
	}
	user, err := s.Service.CreateUser(a, input.Name)
	if err == nil {
		respond(w, user)
	}
	return err
}

// deleteUser deletes a disabled user without retained resource references.
// @Summary Delete disabled user
// @Security Bearer
// @Param id path string true "User identifier"
// @Param If-Match header string true "Current user version"
// @Success 200 {object} map[string]bool
// @Router /admin/users/{id} [delete]
func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	err := s.Service.DeleteUser(a, r.PathValue("id"), revision(r))
	if err == nil {
		respond(w, map[string]bool{"deleted": true})
	}
	return err
}

// adminProviders lists full instance settings with credential references.
// @Summary List provider settings
// @Security Bearer
// @Success 200 {array} storage.Provider
// @Router /admin/providers [get]
func (s *Server) adminProviders(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	if err := service.Admin(a); err != nil {
		return err
	}
	var rows []storage.Provider
	err := s.Service.DB.Order("id").Limit(100).Offset(offset(r)).Find(&rows).Error
	if err == nil {
		respond(w, rows)
	}
	return err
}

// deleteProvider deletes an unreferenced provider instance.
// @Summary Delete provider instance
// @Security Bearer
// @Param id path string true "Provider identifier"
// @Param If-Match header string true "Current provider version"
// @Success 200 {object} map[string]bool
// @Router /admin/providers/{id} [delete]
func (s *Server) deleteProvider(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	err := s.Service.DeleteProvider(a, r.PathValue("id"), revision(r))
	if err == nil {
		respond(w, map[string]bool{"deleted": true})
	}
	return err
}
