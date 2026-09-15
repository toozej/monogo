// Package server serves the API and embedded WebAssembly application.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"github.com/toozej/monogo/apps/gocicle/internal/config"
	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/protocol"
	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"github.com/toozej/monogo/apps/gocicle/internal/service"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"github.com/toozej/monogo/apps/gocicle/internal/ui"
	"github.com/toozej/monogo/pkg/logging"
	"github.com/toozej/monogo/pkg/version"
	"gorm.io/gorm"
)

//go:embed web web/app.wasm
var assets embed.FS

type resources struct{ http.Handler }

func (resources) Resolve(path string) string { return path }

type Server struct {
	Service *service.Service
	Config  config.Config
	HTTP    *http.Server
	log     *logging.Logger
}

func New(cfg config.Config, svc *service.Service) *Server {
	s := &Server{Service: svc, Config: cfg, log: logging.NewLogger(logging.Config{Level: "info", Format: "json", Output: "stderr"})}
	s.HTTP = &http.Server{Addr: cfg.Listen, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 16 << 10}
	return s
}
func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.HTTP.BaseContext = func(net.Listener) context.Context { return ctx }
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		lastRetention := time.Time{}
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if err := s.Service.Tick(ctx, now); err != nil {
					s.log.Error("scheduler transaction failed")
				}
				if now.Sub(lastRetention) > time.Hour {
					if err := s.Service.Retain(ctx, s.Config.Retention); err != nil {
						s.log.Error("retention transaction failed")
					}
					lastRetention = now
				}
			}
		}
	}()
	workers.Add(1)
	go func() {
		defer workers.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.Service.Notify(ctx, s.Config.NotificationHosts); err != nil {
					s.log.Error("notification transaction failed")
				}
			}
		}
	}()
	done := make(chan error, 1)
	go func() { done <- s.HTTP.ListenAndServe() }()
	select {
	case err := <-done:
		cancel()
		workers.Wait()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		err := s.HTTP.Shutdown(shutdown)
		workers.Wait()
		return err
	}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		db, err := s.Service.DB.DB()
		if err == nil {
			err = db.PingContext(r.Context())
		}
		if err != nil {
			http.Error(w, "database is unavailable", http.StatusServiceUnavailable)
			return
		}
		respond(w, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/providers", s.publicProviders)
	mux.HandleFunc("POST /auth/{provider}/begin", s.beginLogin)
	mux.HandleFunc("GET /auth/{provider}/callback", s.completeLogin)
	mux.HandleFunc("GET /oauth-client-metadata.json", s.clientMetadata)
	mux.Handle("/swagger/", httpSwagger.Handler())
	mux.HandleFunc("GET /api/v1/me", s.user(s.me))
	mux.HandleFunc("POST /api/v1/logout", s.user(s.logout))
	mux.HandleFunc("POST /api/v1/tokens", s.user(s.token))
	mux.HandleFunc("GET /api/v1/tokens", s.user(s.tokens))
	mux.HandleFunc("DELETE /api/v1/tokens/{id}", s.user(s.revokeToken))
	mux.HandleFunc("GET /api/v1/projects", s.user(s.projects))
	mux.HandleFunc("POST /api/v1/projects", s.user(s.createProject))
	mux.HandleFunc("PUT /api/v1/projects/{project}", s.user(s.renameProject))
	mux.HandleFunc("DELETE /api/v1/projects/{project}", s.user(s.deleteProject))
	mux.HandleFunc("PUT /api/v1/preferences", s.user(s.preferences))
	mux.HandleFunc("PUT /api/v1/projects/{project}/preferences", s.user(s.preferences))
	mux.HandleFunc("GET /api/v1/projects/{project}/memberships", s.user(s.memberships))
	mux.HandleFunc("PUT /api/v1/projects/{project}/memberships/{user}", s.user(s.membership))
	mux.HandleFunc("GET /api/v1/projects/{project}/jobs", s.user(s.listJobs))
	mux.HandleFunc("POST /api/v1/jobs/validate", s.user(s.validate))
	mux.HandleFunc("POST /api/v1/jobs/import-preview", s.user(s.validate))
	mux.HandleFunc("POST /api/v1/projects/{project}/jobs/import", s.user(s.importJobs))
	mux.HandleFunc("GET /api/v1/projects/{project}/jobs/export", s.user(s.exportJobs))
	mux.HandleFunc("GET /api/v1/jobs/{job}", s.user(s.getJob))
	mux.HandleFunc("PUT /api/v1/jobs/{job}", s.user(s.updateJob))
	mux.HandleFunc("DELETE /api/v1/jobs/{job}", s.user(s.deleteJob))
	mux.HandleFunc("POST /api/v1/jobs/{job}/runs", s.user(s.startRun))
	mux.HandleFunc("GET /api/v1/projects/{project}/runs", s.user(s.runs))
	mux.HandleFunc("GET /api/v1/runs/{run}", s.user(s.run))
	mux.HandleFunc("POST /api/v1/runs/{run}/cancel", s.user(s.cancelRun))
	mux.HandleFunc("GET /api/v1/runs/{run}/logs", s.user(s.logs))
	mux.HandleFunc("GET /api/v1/runs/{run}/metrics", s.user(s.metrics))
	mux.HandleFunc("GET /api/v1/runs/{run}/events", s.user(s.events))
	mux.HandleFunc("POST /api/v1/schedules/preview", s.user(s.schedulePreview))
	mux.HandleFunc("GET /api/v1/secrets", s.user(s.secrets))
	mux.HandleFunc("POST /api/v1/secrets", s.user(s.createSecret))
	mux.HandleFunc("DELETE /api/v1/secrets/{secret}", s.user(s.deleteSecret))
	mux.HandleFunc("PUT /api/v1/secrets/{secret}/grants/{project}", s.user(s.grant))
	mux.HandleFunc("GET /api/v1/{resource}", s.user(s.listResources))
	mux.HandleFunc("POST /api/v1/{resource}", s.user(s.saveResource))
	mux.HandleFunc("PUT /api/v1/{resource}/{id}", s.user(s.saveResource))
	mux.HandleFunc("DELETE /api/v1/{resource}/{id}", s.user(s.deleteResource))
	mux.HandleFunc("POST /api/v1/notifications/{id}/test", s.user(s.testNotification))
	mux.HandleFunc("GET /api/v1/connections/{id}/repositories", s.user(s.repositories))
	mux.HandleFunc("POST /api/v1/connections/{id}/test", s.user(s.repositories))
	mux.HandleFunc("GET /api/v1/admin/users", s.user(s.users))
	mux.HandleFunc("POST /api/v1/admin/users", s.user(s.createUser))
	mux.HandleFunc("DELETE /api/v1/admin/users/{id}", s.user(s.deleteUser))
	mux.HandleFunc("PUT /api/v1/admin/users/{id}", s.user(s.updateUser))
	mux.HandleFunc("POST /api/v1/admin/users/{id}/identity", s.user(s.linkImportedIdentity))
	mux.HandleFunc("POST /api/v1/admin/invitations", s.user(s.invite))
	mux.HandleFunc("GET /api/v1/admin/runners", s.user(s.runners))
	mux.HandleFunc("PUT /api/v1/admin/runners/{id}", s.user(s.updateRunner))
	mux.HandleFunc("PUT /api/v1/admin/runners/{id}/grants/{user}", s.user(s.runnerGrant))
	mux.HandleFunc("POST /api/v1/admin/enrollments", s.user(s.enrollment))
	mux.HandleFunc("POST /api/v1/admin/providers", s.user(s.saveProvider))
	mux.HandleFunc("PUT /api/v1/admin/providers/{id}", s.user(s.saveProvider))
	mux.HandleFunc("GET /api/v1/admin/providers", s.user(s.adminProviders))
	mux.HandleFunc("DELETE /api/v1/admin/providers/{id}", s.user(s.deleteProvider))
	mux.HandleFunc("GET /api/v1/admin/configuration", s.user(s.exportConfiguration))
	mux.HandleFunc("POST /api/v1/admin/configuration", s.user(s.importConfiguration))
	mux.HandleFunc("GET /api/v1/admin/audit", s.user(s.audit))
	mux.HandleFunc("POST /api/v1/runner/enroll", s.enroll)
	mux.HandleFunc("POST /api/v1/projects/{project}/inspections", s.user(s.inspectRepository))
	mux.HandleFunc("GET /api/v1/inspections/{id}", s.user(s.inspection))
	mux.HandleFunc("POST /api/v1/runner/inspections/{id}/heartbeat", s.runner(s.inspectionHeartbeat))
	mux.HandleFunc("POST /api/v1/runner/inspections/{id}/report", s.runner(s.inspectionReport))
	mux.HandleFunc("POST /api/v1/runner/poll", s.runner(s.poll))
	mux.HandleFunc("POST /api/v1/runner/runs/{run}/heartbeat", s.runner(s.heartbeat))
	mux.HandleFunc("POST /api/v1/runner/runs/{run}/events", s.runner(s.report))
	ui.RegisterRoutes()
	mux.Handle("/", &app.Handler{Name: "gocicle", Title: "gocicle", Description: "Schedule container jobs", Version: version.Version, Resources: resources{http.FileServer(http.FS(assets))}})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/auth/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		s.proxyRequest(r)
		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		mux.ServeHTTP(w, r)
	})
}

type userHandler func(http.ResponseWriter, *http.Request, service.Actor) error

func (s *Server) user(h userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, err := s.actor(r)
		if err != nil {
			failure(w, err)
			return
		}
		if a.Session && r.Method != "GET" {
			if r.Header.Get("Origin") != s.Config.PublicURL || security.Hash(r.Header.Get("X-CSRF-Token")) != a.CSRFHash {
				failure(w, service.ErrForbidden)
				return
			}
		}
		if err = h(w, r, a); err != nil {
			failure(w, err)
		}
	}
}
func (s *Server) actor(r *http.Request) (service.Actor, error) {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return s.Service.Authenticate(r.Context(), strings.TrimPrefix(auth, "Bearer "), false)
	}
	cookie, err := r.Cookie("__Host-gocicle")
	if err != nil {
		return service.Actor{}, service.ErrForbidden
	}
	return s.Service.Authenticate(r.Context(), cookie.Value, true)
}
func (s *Server) runner(h func(http.ResponseWriter, *http.Request, storage.Runner) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		runner, err := s.Service.Runner(token)
		if err == nil {
			err = h(w, r, runner)
		}
		if err != nil {
			failure(w, err)
		}
	}
}
func decode(r *http.Request, v any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return service.ErrInvalid
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return service.ErrInvalid
	}
	return nil
}
func respond(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, err error) {
	status := 500
	switch {
	case errors.Is(err, service.ErrForbidden), errors.Is(err, service.ErrApprovalRequired):
		status = 403
	case errors.Is(err, service.ErrConflict), errors.Is(err, gorm.ErrDuplicatedKey):
		status = 409
	case errors.Is(err, service.ErrInvalid), errors.Is(err, gorm.ErrForeignKeyViolated):
		status = 400
	case errors.Is(err, gorm.ErrRecordNotFound):
		status = 404
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": service.CleanError(err)})
}
func offset(r *http.Request) int {
	v, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if v < 0 || v > 1000000 {
		return 0
	}
	return v
}
func revision(r *http.Request) int64 {
	v, _ := strconv.ParseInt(strings.Trim(r.Header.Get("If-Match"), "\""), 10, 64)
	return v
}

// me handles the documented API operations.
// @Summary Me
// @Security Bearer
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /me [get]
func (s *Server) me(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	respond(w, a.User)
	return nil
}

// logout handles the documented API operations.
// @Summary Logout
// @Security Bearer
// @Produce json
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /logout [post]
func (s *Server) logout(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	if !a.Session {
		return service.ErrForbidden
	}
	cookie, _ := r.Cookie("__Host-gocicle")
	if err := s.Service.DB.Exec("DELETE FROM sessions WHERE hash=?", security.Hash(cookie.Value)).Error; err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: "__Host-gocicle", Path: "/", Secure: true, HttpOnly: true, MaxAge: -1, SameSite: http.SameSiteLaxMode})
	respond(w, map[string]bool{"loggedOut": true})
	return nil
}

// token handles the documented API operations.
// @Summary Token
// @Security Bearer
// @Produce json
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /tokens [post]
func (s *Server) token(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v struct {
		Scopes    []string  `json:"scopes"`
		ExpiresAt time.Time `json:"expiresAt"`
	}
	if err := decode(r, &v); err != nil {
		return err
	}
	token, err := s.Service.CreateToken(a, v.Scopes, v.ExpiresAt)
	if err == nil {
		respond(w, map[string]string{"token": token})
	}
	return err
}

// projects returns projects visible to the authenticated user.
// @Summary List accessible projects
// @Security Bearer
// @Produce json
// @Success 200 {array} storage.Project
// @Router /projects [get]
func (s *Server) projects(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	rows, err := s.Service.ListProjects(a, offset(r))
	if err == nil {
		respond(w, rows)
	}
	return err
}

// createProject handles the documented API operations.
// @Summary Create project
// @Security Bearer
// @Produce json
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /projects [post]
func (s *Server) createProject(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v storage.Project
	if err := decode(r, &v); err != nil {
		return err
	}
	p, err := s.Service.CreateProject(a, v)
	if err == nil {
		respond(w, p)
	}
	return err
}

// deleteProject handles the documented API operations.
// @Summary Delete project
// @Security Bearer
// @Produce json
// @Param project path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /projects/{project} [delete]
func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	err := s.Service.DeleteProject(a, r.PathValue("project"), revision(r))
	if err == nil {
		respond(w, map[string]bool{"deleted": true})
	}
	return err
}

// preferences handles the documented API operations.
// @Summary Preferences
// @Security Bearer
// @Produce json
// @Param project path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /preferences [put]
// @Router /projects/{project}/preferences [put]
func (s *Server) preferences(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var p jobs.Preferences
	if err := decode(r, &p); err != nil {
		return err
	}
	err := s.Service.Preferences(a, r.PathValue("project"), p, revision(r))
	if err == nil {
		respond(w, p)
	}
	return err
}

// membership handles the documented API operations.
// @Summary Membership
// @Security Bearer
// @Produce json
// @Param project path string true "Record identifier"
// @Param user path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /projects/{project}/memberships/{user} [put]
func (s *Server) membership(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v struct {
		Role string `json:"role"`
	}
	if err := decode(r, &v); err != nil {
		return err
	}
	err := s.Service.Membership(a, r.PathValue("project"), r.PathValue("user"), v.Role)
	if err == nil {
		respond(w, v)
	}
	return err
}

// memberships handles the documented API operations.
// @Summary Memberships
// @Security Bearer
// @Produce json
// @Param project path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /projects/{project}/memberships [get]
func (s *Server) memberships(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	project := r.PathValue("project")
	if err := s.Service.Authorize(a, project, "viewer"); err != nil {
		return err
	}
	var rows []struct {
		UserID string `json:"userID"`
		Role   string `json:"role"`
	}
	err := s.Service.DB.Table("memberships").Where("project_id=?", project).Order("user_id").Limit(100).Offset(offset(r)).Find(&rows).Error
	if err == nil {
		respond(w, rows)
	}
	return err
}

// listJobs handles the documented API operations.
// @Summary List jobs
// @Security Bearer
// @Produce json
// @Param project path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /projects/{project}/jobs [get]
func (s *Server) listJobs(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	project := r.PathValue("project")
	if err := s.Service.Authorize(a, project, "viewer"); err != nil {
		return err
	}
	var rows []storage.Job
	err := s.Service.DB.Where("project_id=?", project).Order("name,id").Limit(100).Offset(offset(r)).Find(&rows).Error
	if err == nil {
		respond(w, rows)
	}
	return err
}

// validate handles the documented API operations.
// @Summary Validate
// @Security Bearer
// @Produce json
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /jobs/validate [post]
// @Router /jobs/import-preview [post]
func (s *Server) validate(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	if !a.Can("write") {
		return service.ErrForbidden
	}
	var v struct {
		YAML string `json:"yaml"`
	}
	if err := decode(r, &v); err != nil {
		return err
	}
	document, err := jobs.Parse([]byte(v.YAML))
	if err != nil {
		return fmt.Errorf("%w: %s", service.ErrInvalid, err)
	}
	respond(w, document)
	return nil
}

// importJobs handles the documented API operations.
// @Summary Import jobs
// @Security Bearer
// @Produce json
// @Param project path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /projects/{project}/jobs/import [post]
func (s *Server) importJobs(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v service.Import
	if err := decode(r, &v); err != nil {
		return err
	}
	rows, err := s.Service.Import(a, r.PathValue("project"), r.Header.Get("Idempotency-Key"), v)
	if err == nil {
		respond(w, rows)
	}
	return err
}

// exportJobs handles the documented API operations.
// @Summary Export jobs
// @Security Bearer
// @Produce json
// @Param project path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /projects/{project}/jobs/export [get]
func (s *Server) exportJobs(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	d, err := s.Service.Export(a, r.PathValue("project"))
	if err != nil {
		return err
	}
	b, err := d.YAML()
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Disposition", `attachment; filename=".gocicle.yaml"`)
	_, err = w.Write(b) // #nosec G705 -- The response is a YAML attachment with nosniff. It is not HTML.
	return err
}

// getJob handles the documented API operations.
// @Summary Get job
// @Security Bearer
// @Produce json
// @Param job path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /jobs/{job} [get]
func (s *Server) getJob(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var job storage.Job
	if err := s.Service.DB.First(&job, "id=?", r.PathValue("job")).Error; err != nil {
		return err
	}
	if err := s.Service.Authorize(a, job.ProjectID, "viewer"); err != nil {
		return err
	}
	var rev storage.Revision
	if err := s.Service.DB.First(&rev, "id=?", job.RevisionID).Error; err != nil {
		return err
	}
	respond(w, map[string]any{"job": job, "revision": rev})
	return nil
}

// updateJob handles the documented API operations.
// @Summary Update job
// @Security Bearer
// @Produce json
// @Param job path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /jobs/{job} [put]
func (s *Server) updateJob(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v struct {
		Spec    jobs.Spec `json:"spec"`
		Enabled bool      `json:"enabled"`
	}
	if err := decode(r, &v); err != nil {
		return err
	}
	job, err := s.Service.UpdateJob(a, r.PathValue("job"), revision(r), v.Spec, v.Enabled)
	if err == nil {
		respond(w, job)
	}
	return err
}

// deleteJob handles the documented API operations.
// @Summary Delete job
// @Security Bearer
// @Produce json
// @Param job path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /jobs/{job} [delete]
func (s *Server) deleteJob(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	err := s.Service.DeleteJob(a, r.PathValue("job"), revision(r))
	if err == nil {
		respond(w, map[string]bool{"deleted": true})
	}
	return err
}

// startRun handles the documented API operations.
// @Summary Start run
// @Security Bearer
// @Produce json
// @Param job path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /jobs/{job}/runs [post]
func (s *Server) startRun(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	run, err := s.Service.Start(a, r.PathValue("job"), r.Header.Get("Idempotency-Key"))
	if err == nil {
		respond(w, run)
	}
	return err
}

// runs handles the documented API operations.
// @Summary Runs
// @Security Bearer
// @Produce json
// @Param project path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /projects/{project}/runs [get]
func (s *Server) runs(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	project := r.PathValue("project")
	if err := s.Service.Authorize(a, project, "viewer"); err != nil {
		return err
	}
	var runs []storage.Run
	err := s.Service.DB.Where("project_id=?", project).Order("created_at DESC,id").Limit(100).Offset(offset(r)).Find(&runs).Error
	if err == nil {
		respond(w, runs)
	}
	return err
}
func (s *Server) authorizedRun(a service.Actor, id string) (storage.Run, error) {
	var run storage.Run
	if err := s.Service.DB.First(&run, "id=?", id).Error; err != nil {
		return run, err
	}
	return run, s.Service.Authorize(a, run.ProjectID, "viewer")
}

// run handles the documented API operations.
// @Summary Run
// @Security Bearer
// @Produce json
// @Param run path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /runs/{run} [get]
func (s *Server) run(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	run, err := s.authorizedRun(a, r.PathValue("run"))
	if err == nil {
		respond(w, run)
	}
	return err
}

// cancelRun handles the documented API operations.
// @Summary Cancel run
// @Security Bearer
// @Produce json
// @Param run path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /runs/{run}/cancel [post]
func (s *Server) cancelRun(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	err := s.Service.Cancel(a, r.PathValue("run"))
	if err == nil {
		respond(w, map[string]bool{"cancelRequested": true})
	}
	return err
}

// logs handles the documented API operations.
// @Summary Logs
// @Security Bearer
// @Produce json
// @Param run path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /runs/{run}/logs [get]
func (s *Server) logs(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	run, err := s.authorizedRun(a, r.PathValue("run"))
	if err != nil {
		return err
	}
	var rows []struct {
		Sequence int64  `json:"sequence"`
		Content  string `json:"content"`
	}
	err = s.Service.DB.Table("log_chunks").Where("run_id=?", run.ID).Order("sequence").Limit(100).Offset(offset(r)).Find(&rows).Error
	if err == nil {
		respond(w, map[string]any{"chunks": rows, "truncated": run.LogTruncated})
	}
	return err
}

// metrics handles the documented API operations.
// @Summary Metrics
// @Security Bearer
// @Produce json
// @Param run path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /runs/{run}/metrics [get]
func (s *Server) metrics(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	run, err := s.authorizedRun(a, r.PathValue("run"))
	if err != nil {
		return err
	}
	var rows []struct {
		Sequence int64        `json:"sequence"`
		Data     storage.JSON `json:"data"`
	}
	err = s.Service.DB.Table("metric_samples").Where("run_id=?", run.ID).Order("sequence").Limit(100).Offset(offset(r)).Find(&rows).Error
	if err == nil {
		respond(w, rows)
	}
	return err
}

// events handles the documented API operations.
// @Summary Events
// @Security Bearer
// @Produce json
// @Param run path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /runs/{run}/events [get]
func (s *Server) events(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	run, err := s.authorizedRun(a, r.PathValue("run"))
	if err != nil {
		return err
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("streaming is unavailable")
	}
	sequence, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Cache-Control", "no-cache")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(5 * time.Minute)
	defer deadline.Stop()
	for {
		current, err := s.actor(r)
		if err != nil {
			return nil
		}
		if err := s.Service.Authorize(current, run.ProjectID, "viewer"); err != nil {
			return nil
		}
		var rows []struct {
			Sequence int64
			Kind     string
			Data     storage.JSON
		}
		if err := s.Service.DB.Table("run_events").Where("run_id=? AND sequence>?", run.ID, sequence).Order("sequence").Limit(100).Find(&rows).Error; err != nil {
			return nil
		}
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(15 * time.Second))
		for _, row := range rows {
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", row.Sequence, row.Kind, row.Data); err != nil {
				return nil
			}
			sequence = row.Sequence
		}
		_, _ = fmt.Fprint(w, ": heartbeat\n\n")
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return nil
		case <-deadline.C:
			return nil
		case <-ticker.C:
		}
	}
}

// schedulePreview handles the documented API operations.
// @Summary Schedule preview
// @Security Bearer
// @Produce json
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /schedules/preview [post]
func (s *Server) schedulePreview(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v struct{ Schedule, Timezone string }
	if err := decode(r, &v); err != nil {
		return err
	}
	if v.Timezone == "" {
		v.Timezone = "UTC"
	}
	schedule, err := jobs.Schedule(v.Schedule, v.Timezone)
	if err != nil {
		return service.ErrInvalid
	}
	next := time.Now()
	times := []time.Time{}
	for i := 0; i < 10; i++ {
		next = schedule.Next(next)
		times = append(times, next)
	}
	respond(w, times)
	return nil
}
func (s *Server) enroll(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Token string `json:"token"`
	}
	if err := decode(r, &v); err != nil {
		failure(w, err)
		return
	}
	runner, token, err := s.Service.Enroll(v.Token)
	if err != nil {
		failure(w, err)
		return
	}
	respond(w, map[string]any{"runner": runner, "token": token})
}

// poll handles the documented API operations.
// @Summary Poll
// @Security Bearer
// @Produce json
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /runner/poll [post]
func (s *Server) poll(w http.ResponseWriter, r *http.Request, runner storage.Runner) error {
	assignment, err := s.Service.Poll(runner)
	if err == nil {
		respond(w, assignment)
	}
	return err
}

// heartbeat handles the documented API operations.
// @Summary Heartbeat
// @Security Bearer
// @Produce json
// @Param run path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /runner/runs/{run}/heartbeat [post]
func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request, runner storage.Runner) error {
	expiry, cancel, err := s.Service.Heartbeat(runner, r.PathValue("run"), r.Header.Get("X-Gocicle-Lease"))
	if err == nil {
		respond(w, map[string]any{"expiresAt": expiry, "cancel": cancel})
	}
	return err
}

// report handles the documented API operations.
// @Summary Report
// @Security Bearer
// @Produce json
// @Param run path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /runner/runs/{run}/events [post]
func (s *Server) report(w http.ResponseWriter, r *http.Request, runner storage.Runner) error {
	var event protocol.Event
	if err := decode(r, &event); err != nil {
		return err
	}
	err := s.Service.Report(runner, r.PathValue("run"), r.Header.Get("X-Gocicle-Lease"), event)
	if err == nil {
		respond(w, map[string]bool{"accepted": true})
	}
	return err
}

// inspectRepository handles the documented API operations.
// @Summary Inspect repository
// @Security Bearer
// @Produce json
// @Param project path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /projects/{project}/inspections [post]
func (s *Server) inspectRepository(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	var v struct {
		RunnerID string `json:"runnerID"`
	}
	if err := decode(r, &v); err != nil {
		return err
	}
	row, err := s.Service.InspectRepository(a, r.PathValue("project"), v.RunnerID)
	if err == nil {
		respond(w, row)
	}
	return err
}

// inspection handles the documented API operations.
// @Summary Inspection
// @Security Bearer
// @Produce json
// @Param id path string true "Record identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /inspections/{id} [get]
func (s *Server) inspection(w http.ResponseWriter, r *http.Request, a service.Actor) error {
	row, err := s.Service.Inspection(a, r.PathValue("id"))
	if err == nil {
		respond(w, row)
	}
	return err
}

// inspectionHeartbeat handles the documented API operations.
// @Summary Inspection heartbeat
// @Security Bearer
// @Produce json
// @Param id path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /runner/inspections/{id}/heartbeat [post]
func (s *Server) inspectionHeartbeat(w http.ResponseWriter, r *http.Request, runner storage.Runner) error {
	err := s.Service.InspectionHeartbeat(runner, r.PathValue("id"), r.Header.Get("X-Gocicle-Lease"))
	if err == nil {
		respond(w, map[string]bool{"accepted": true})
	}
	return err
}

// inspectionReport handles the documented API operations.
// @Summary Inspection report
// @Security Bearer
// @Produce json
// @Param id path string true "Record identifier"
// @Accept json
// @Param body body map[string]interface{} false "Request fields are described in the app README"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /runner/inspections/{id}/report [post]
func (s *Server) inspectionReport(w http.ResponseWriter, r *http.Request, runner storage.Runner) error {
	var v struct {
		YAML  string `json:"yaml"`
		Error string `json:"error"`
	}
	if err := decode(r, &v); err != nil {
		return err
	}
	err := s.Service.InspectionReport(runner, r.PathValue("id"), r.Header.Get("X-Gocicle-Lease"), v.YAML, v.Error)
	if err == nil {
		respond(w, map[string]bool{"accepted": true})
	}
	return err
}
