// Package web serves the embedded form and filename downloads.
package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/toozej/monogo/apps/gphotos-lister/internal/config"
	"github.com/toozej/monogo/apps/gphotos-lister/internal/lister"
	"github.com/toozej/monogo/apps/gphotos-lister/internal/photos"
)

//go:embed assets/*
var assets embed.FS

var index = template.Must(template.ParseFS(assets, "assets/index.html"))

// Server shares one reader between form requests.
type Server struct {
	Config config.Config
	Reader lister.AlbumReader
}

// ValidateBind requires credentials when the server accepts remote connections.
func ValidateBind(host, username, password string) error {
	if (username == "") != (password == "") {
		return fmt.Errorf("set both web username and web password")
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) && username == "" {
		return fmt.Errorf("set web username and web password for a non-loopback host")
	}
	return nil
}

// Handler creates the form, static asset, and submit routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /submit", s.handleSubmit)
	mux.HandleFunc("GET /style.css", func(w http.ResponseWriter, r *http.Request) {
		data, _ := assets.ReadFile("assets/style.css")
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("GET /script.js", func(w http.ResponseWriter, r *http.Request) {
		data, _ := assets.ReadFile("assets/script.js")
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(data)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		if s.Config.WebUsername != "" || s.Config.WebPassword != "" {
			username, password, ok := r.BasicAuth()
			userOK := subtle.ConstantTimeCompare([]byte(username), []byte(s.Config.WebUsername)) == 1
			passOK := subtle.ConstantTimeCompare([]byte(password), []byte(s.Config.WebPassword)) == 1
			if !ok || !userOK || !passOK {
				w.Header().Set("WWW-Authenticate", `Basic realm="gphotos-lister"`)
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		http.Error(w, "Cannot create form token", http.StatusInternalServerError)
		return
	}
	csrf := hex.EncodeToString(token)
	// #nosec G124 -- Local HTTP needs a non-Secure cookie. Direct TLS uses Secure.
	// nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
	http.SetCookie(w, &http.Cookie{Name: "gphotos_csrf", Value: csrf, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = index.Execute(w, struct {
		Token    string
		Defaults lister.Options
	}{csrf, s.Config.Options()})
}

func validCSRF(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if err != nil || u.Scheme != scheme || u.Host != r.Host {
			return false
		}
	}
	cookie, err := r.Cookie("gphotos_csrf")
	token := r.Header.Get("X-CSRF-Token")
	return err == nil && len(token) == 64 && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(token)) == 1
}

func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if !validCSRF(r) {
		http.Error(w, "Invalid form token", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var options lister.Options
	if err := decoder.Decode(&options); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		http.Error(w, "Send one JSON object", http.StatusBadRequest)
		return
	}
	if err := options.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := photos.ValidateAlbumURL(options.Album); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.Config.Timeout)
	defer cancel()
	result, err := lister.Run(ctx, s.Reader, options)
	response := struct {
		Result lister.Result `json:"result"`
		Error  string        `json:"error,omitempty"`
	}{Result: result}
	status := http.StatusOK
	if err != nil {
		response.Error = err.Error()
		status = http.StatusUnprocessableEntity
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}

// Serve starts the server and stops it when the context ends.
func (s *Server) Serve(ctx context.Context) error {
	if err := ValidateBind(s.Config.WebHost, s.Config.WebUsername, s.Config.WebPassword); err != nil {
		return err
	}
	if s.Config.WebPort < 1 || s.Config.WebPort > 65535 {
		return fmt.Errorf("web port must be between 1 and 65535")
	}
	if s.Config.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(s.Config.WebHost, strconv.Itoa(s.Config.WebPort)))
	if err != nil {
		return err
	}
	return s.serveHTTP(ctx, listener)
}

func (s *Server) serveHTTP(ctx context.Context, listener net.Listener) error {
	server := &http.Server{BaseContext: func(net.Listener) context.Context { return ctx }, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: s.Config.Timeout + time.Minute, IdleTimeout: time.Minute}
	shutdown := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
		case <-shutdown:
		}
	}()
	defer close(shutdown)
	err := server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		<-stopped
		return nil
	}
	return err
}
