package web

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	_ "github.com/toozej/monogo/apps/RSSFFS/docs"
	"github.com/toozej/monogo/apps/RSSFFS/internal/config"
	"github.com/toozej/monogo/pkg/version"
)

// @title RSSFFS API
// @version 1.0
// @description HTTP API for discovering RSS feeds, subscribing through the configured RSS reader, listing categories, and viewing application logs. When web credentials are configured, operations require HTTP Basic authentication. Submissions also require an X-CSRF-Token header matching the csrf_token cookie issued by the web interface.
// @license.name MIT
// @license.url https://github.com/toozej/monogo/blob/main/apps/RSSFFS/LICENSE
// @BasePath /
// @schemes http https
// @securityDefinitions.basic BasicAuth

// Server represents the HTTP server with configuration and debug settings
type Server struct {
	config      config.Config
	debug       bool
	server      *http.Server
	rateLimiter *RateLimiter
	logHook     *WebUIHook
}

const serverWriteTimeout = 3 * time.Minute

// NewServer creates a new Server instance with the provided configuration
func NewServer(conf config.Config, debug bool) *Server {
	// Create log hook for capturing logs for web UI
	logHook := NewWebUIHook(100) // Buffer last 100 log entries

	// Add the hook to logrus
	log.AddHook(logHook)

	return &Server{
		config:      conf,
		debug:       debug,
		rateLimiter: NewRateLimiter(10, time.Minute), // 10 requests per minute
		logHook:     logHook,
	}
}

// SetupRoutes configures HTTP routes and middleware
func (s *Server) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Wrap handlers with middleware
	mux.HandleFunc("/", s.withMiddleware(s.handleIndex))
	mux.HandleFunc("/submit", s.withMiddleware(s.handleSubmit))
	mux.HandleFunc("/categories", s.withMiddleware(s.handleCategories))
	mux.HandleFunc("/logs", s.withMiddleware(s.handleLogs))
	mux.HandleFunc("/logs/stream", s.withMiddleware(s.handleLogsSSE))
	mux.HandleFunc("/static/", s.withMiddleware(s.handleStatic))
	mux.Handle("/swagger/", s.withMiddleware(httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	).ServeHTTP))

	// Direct routes for common assets (for backward compatibility and convenience)
	mux.HandleFunc("/style.css", s.withMiddleware(s.handleDirectAsset))
	mux.HandleFunc("/script.js", s.withMiddleware(s.handleDirectAsset))
	mux.HandleFunc("/favicon.svg", s.withMiddleware(s.handleDirectAsset))

	return mux
}

// withMiddleware applies logging, security headers, rate limiting, and CORS middleware
func (s *Server) withMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Apply defensive headers to every response, including authentication and
		// rate-limit failures that return before reaching the route handler.
		s.setSecurityHeaders(w)
		if !s.isAuthorized(r) {
			w.Header().Set("WWW-Authenticate", `Basic realm="RSSFFS", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// Log request
		start := time.Now()
		if s.debug {
			log.Debugf("Request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		}

		// Rate limiting (only for POST requests to prevent abuse)
		if r.Method == "POST" {
			clientIP := getClientIP(r)
			if !s.rateLimiter.IsAllowed(clientIP) {
				log.Warnf("Rate limit exceeded for IP: %s", clientIP)
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}
		}

		// Call the actual handler
		next(w, r)

		// Log response time
		if s.debug {
			log.Debugf("Response: %s %s completed in %v", r.Method, r.URL.Path, time.Since(start))
		}
	}
}

func (s *Server) isAuthorized(r *http.Request) bool {
	if s.config.WebUsername == "" && s.config.WebPassword == "" {
		return true
	}
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	usernameOK := subtle.ConstantTimeCompare([]byte(username), []byte(s.config.WebUsername)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(s.config.WebPassword)) == 1
	return usernameOK && passwordOK
}

// setSecurityHeaders sets comprehensive security headers
func (s *Server) setSecurityHeaders(w http.ResponseWriter) {
	// Prevent MIME type sniffing
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Prevent clickjacking
	w.Header().Set("X-Frame-Options", "DENY")

	// XSS protection (legacy, but still useful for older browsers)
	w.Header().Set("X-XSS-Protection", "1; mode=block")

	// Referrer policy
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Content Security Policy
	csp := "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"connect-src 'self'; " +
		"form-action 'self'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'"
	w.Header().Set("Content-Security-Policy", csp)

	// Strict Transport Security (HSTS) - only if HTTPS
	// w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

	// Permissions Policy (formerly Feature Policy)
	w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

	// Prevent caching of sensitive content
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	}
}

// handleIndex serves the main HTML page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Only serve index for root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Generate CSRF token and set it as a cookie
	csrfToken, err := GenerateCSRFToken()
	if err != nil {
		log.Errorf("Error generating CSRF token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// HttpOnly=false required for JS CSRF token read; Secure set based on TLS availability
	// #nosec G124 -- cookie security attributes set intentionally for CSRF token JS access
	// nosemgrep: go.lang.security.audit.net.cookie-missing-httponly.cookie-missing-httponly, go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    csrfToken,
		Path:     "/",
		Expires:  time.Now().Add(1 * time.Hour),
		HttpOnly: false, // Must be false so JS can read it
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	// Get version info
	versionInfo, err := version.Get()
	if err != nil {
		log.Warnf("Could not get version info: %v", err)
		// Fallback to a default version info struct
		versionInfo = version.Info{
			Version: "local",
		}
	}

	// Render the index template
	data := TemplateData{
		Title:   "RSSFFS - RSS Feed Finder and Subscriber",
		Debug:   s.debug,
		Version: versionInfo.Version,
	}

	if err := RenderTemplate(w, "index.html", data); err != nil {
		log.Errorf("Error rendering template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleStatic serves embedded static assets
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract asset path from URL
	assetPath := r.URL.Path[len("/static/"):]
	if assetPath == "" {
		http.NotFound(w, r)
		return
	}

	// Serve the asset
	ServeAsset(w, r, assetPath)
}

// handleDirectAsset serves assets directly from root path (e.g., /style.css, /script.js)
func (s *Server) handleDirectAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract asset name from URL path (remove leading slash)
	assetPath := strings.TrimPrefix(r.URL.Path, "/")
	if assetPath == "" {
		http.NotFound(w, r)
		return
	}

	// Serve the asset
	ServeAsset(w, r, assetPath)
}

// Start starts the HTTP server on the specified host and port
func (s *Server) Start(host string, port int) error {
	if err := s.validateBindAuthentication(host); err != nil {
		return err
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.SetupRoutes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	log.Infof("Starting web server on http://%s", addr)

	// Start server in a goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	return s.waitForShutdown()
}

func (s *Server) validateBindAuthentication(host string) error {
	hasUsername := s.config.WebUsername != ""
	hasPassword := s.config.WebPassword != ""
	if hasUsername != hasPassword {
		return fmt.Errorf("both WEB_USERNAME and WEB_PASSWORD must be configured together")
	}
	normalizedHost := strings.TrimSuffix(strings.ToLower(host), ".")
	ip := net.ParseIP(normalizedHost)
	isLoopback := normalizedHost == "localhost" || (ip != nil && ip.IsLoopback())
	if !isLoopback && !hasUsername {
		return fmt.Errorf("WEB_USERNAME and WEB_PASSWORD are required when binding the web server to non-loopback host %q", host)
	}
	return nil
}

// waitForShutdown waits for interrupt signal and gracefully shuts down the server
func (s *Server) waitForShutdown() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal is received
	<-quit
	log.Info("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := s.server.Shutdown(ctx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
		return err
	}

	log.Info("Server exited")
	return nil
}
