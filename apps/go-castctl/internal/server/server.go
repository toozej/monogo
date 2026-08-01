// Package server provides go-castctl's HTTP server and API handlers.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
	"github.com/toozej/monogo/apps/go-castctl/internal/castor"
	"github.com/toozej/monogo/apps/go-castctl/internal/config"
	"github.com/toozej/monogo/apps/go-castctl/internal/ui"
	"github.com/toozej/monogo/pkg/version"
)

const maxRequestBody = 1 << 20

//go:embed web
var webResources embed.FS

type embeddedResources struct {
	http.Handler
}

func (embeddedResources) Resolve(path string) string { return path }

// Server owns the configured HTTP listener.
type Server struct {
	service castor.Service
	server  *http.Server
}

// New constructs a server with all API aliases and go-app resources registered.
func New(cfg config.Server, service castor.Service) *Server {
	ui.RegisterRoutes()
	s := &Server{service: service}
	mux := http.NewServeMux()
	for _, path := range []string{"/send", "/cast"} {
		mux.HandleFunc(path, s.handleSend)
	}
	for _, path := range []string{"/receive", "/view", "/watch"} {
		mux.HandleFunc(path, s.handleReceive)
	}
	mux.HandleFunc("/devices", s.handleDevices)
	mux.Handle("/", &app.Handler{
		Name:            "go-castctl",
		ShortName:       "go-castctl",
		Title:           "go-castctl",
		Description:     "Cast web video to a TV or watch it locally",
		BackgroundColor: "#0b1020",
		ThemeColor:      "#6d5dfc",
		Version:         version.Version,
		Resources: embeddedResources{
			Handler: http.FileServer(http.FS(webResources)),
		},
	})
	s.server = &http.Server{
		Addr:         cfg.Address(),
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
	return s
}

// Handler exposes the configured mux for tests and embedding.
func (s *Server) Handler() http.Handler { return s.server.Handler }

// Run serves until the context is cancelled, then shuts down cleanly.
func (s *Server) Run(ctx context.Context) error {
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("go-castctl listening", "address", s.server.Addr)
		serverErr <- s.server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.server.WriteTimeout)
		defer cancel()
		if closer, ok := s.service.(io.Closer); ok {
			_ = closer.Close()
		}
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}
		return nil
	}
}

type request struct {
	URL    string        `json:"url"`
	Device castor.Device `json:"device"`
}

type apiResponse struct {
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
	StreamURL string `json:"streamURL,omitempty"`
	BitRate   int64  `json:"bitrate,omitempty"`
}

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	devices, err := s.service.Devices(r.Context())
	if err != nil {
		slog.Error("discover devices", "error", err)
		writeError(w, http.StatusBadGateway, "Castor could not discover devices")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Devices []castor.Device `json:"devices"`
	}{Devices: devices})
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	payload, err := decodeRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateRequest(payload, true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.service.Send(r.Context(), payload.URL, payload.Device); err != nil {
		slog.Error("start cast", "error", err)
		writeError(w, http.StatusBadGateway, "Castor could not start playback")
		return
	}
	writeJSON(w, http.StatusAccepted, apiResponse{Status: "accepted", Message: "Castor session started"})
}

func (s *Server) handleReceive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	payload, err := decodeRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateRequest(payload, false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	stream, err := s.service.Receive(r.Context(), payload.URL)
	if err != nil {
		slog.Error("resolve stream", "error", err)
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, "Castor timed out while resolving the stream")
			return
		}
		writeError(w, http.StatusBadGateway, "Castor could not resolve a browser-playable stream")
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "ready", StreamURL: stream.URL, BitRate: stream.BitRate})
}

func decodeRequest(w http.ResponseWriter, r *http.Request) (request, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var payload request
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			return request{}, errors.New("request body must be valid JSON")
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return request{}, errors.New("request body must contain one JSON object")
		}
		return payload, nil
	}
	if err := r.ParseForm(); err != nil {
		return request{}, errors.New("request body must be a valid form")
	}
	return request{
		URL: r.Form.Get("url"),
		Device: castor.Device{
			Name:    r.Form.Get("device_name"),
			Type:    r.Form.Get("device_type"),
			Address: r.Form.Get("device_address"),
		},
	}, nil
}

func validateRequest(payload request, requireDevice bool) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(payload.URL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("url must be an absolute HTTP or HTTPS URL")
	}
	if parsed.User != nil {
		return errors.New("url must not include credentials")
	}
	if !requireDevice {
		return nil
	}
	if strings.TrimSpace(payload.Device.Name) == "" {
		return errors.New("device.name is required")
	}
	switch payload.Device.Type {
	case "chromecast", "dlna":
		return nil
	default:
		return errors.New("device.type must be chromecast or dlna")
	}
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiResponse{Status: "error", Message: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
