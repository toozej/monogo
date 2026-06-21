package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/internal/middleware"
	"github.com/toozej/go-listen/internal/services/playlist"
	"github.com/toozej/go-listen/internal/services/scraper"
	"github.com/toozej/go-listen/internal/services/spotify"
	"github.com/toozej/go-listen/internal/types"
	"github.com/toozej/go-listen/pkg/config"
	"github.com/toozej/go-listen/pkg/logging"
)

//go:embed static/*
var staticFiles embed.FS

// ScraperService defines the interface for web scraping operations
type ScraperService interface {
	ScrapeArtists(url, cssSelector string) ([]string, error)
	ScrapeAndAddToPlaylist(url, cssSelector, playlistID string, force bool) (*scraper.ScrapeResult, error)
}

// Server represents the HTTP server
type Server struct {
	router             *http.ServeMux
	spotify            types.SpotifyService
	playlist           types.PlaylistManager
	scraper            ScraperService
	config             *config.Config
	logger             *logging.Logger
	rateLimiter        *middleware.RateLimiter
	securityMiddleware *middleware.SecurityMiddleware
	loggingMiddleware  *middleware.LoggingMiddleware
	server             *http.Server
}

// NewServer creates a new server instance with all components properly wired
func NewServer(cfg *config.Config) *Server {
	// Validate configuration
	if cfg == nil {
		panic("configuration cannot be nil")
	}

	// Set default logging configuration if not provided
	loggingCfg := cfg.Logging
	if loggingCfg.Level == "" {
		loggingCfg.Level = "info"
	}
	if loggingCfg.Format == "" {
		loggingCfg.Format = "json"
	}
	if loggingCfg.Output == "" {
		loggingCfg.Output = "stdout"
	}

	// Initialize structured logger (foundation component)
	logger := logging.NewLogger(loggingCfg)
	logger.WithComponent("server").Info("Initializing server components")

	// Initialize Spotify service (core business logic)
	spotifyService := spotify.NewService(cfg.Spotify, logger.Logger)

	// Initialize playlist manager (depends on Spotify service)
	playlistManager := playlist.NewService(spotifyService, logger.Logger)

	// Initialize rate limiter with default values if not configured
	requestsPerSecond := cfg.Security.RateLimit.RequestsPerSecond
	if requestsPerSecond == 0 {
		requestsPerSecond = 10 // Default to 10 requests per second
	}
	burst := cfg.Security.RateLimit.Burst
	if burst == 0 {
		burst = 20 // Default to burst of 20
	}

	// Initialize rate limiter (security component)
	rateLimiter := middleware.NewRateLimiter(requestsPerSecond, burst)

	// Initialize security middleware (depends on rate limiter)
	securityMiddleware := middleware.NewSecurityMiddleware(logger.Logger, rateLimiter)

	// Initialize logging middleware (depends on logger)
	loggingMiddleware := middleware.NewLoggingMiddleware(logger)

	logger.WithComponent("server").WithFields(logrus.Fields{
		"spotify_configured": cfg.Spotify.ClientID != "",
		"rate_limit_rps":     requestsPerSecond,
		"rate_limit_burst":   burst,
		"logging_level":      loggingCfg.Level,
		"http_logging":       loggingCfg.EnableHTTP,
	}).Info("Server components initialized successfully")

	return &Server{
		router:             http.NewServeMux(),
		spotify:            spotifyService,
		playlist:           playlistManager,
		config:             cfg,
		logger:             logger,
		rateLimiter:        rateLimiter,
		securityMiddleware: securityMiddleware,
		loggingMiddleware:  loggingMiddleware,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.setupRoutes()

	s.server = &http.Server{
		Addr:         s.config.Server.Address(),
		Handler:      s.router,
		ReadTimeout:  time.Duration(s.config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(s.config.Server.IdleTimeout) * time.Second,
	}

	s.logger.WithComponent("server").WithFields(logrus.Fields{
		"address":       s.config.Server.Address(),
		"read_timeout":  s.config.Server.ReadTimeout,
		"write_timeout": s.config.Server.WriteTimeout,
		"idle_timeout":  s.config.Server.IdleTimeout,
	}).Info("Starting HTTP server")
	return s.server.ListenAndServe()
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.WithComponent("server").Info("Shutting down HTTP server")
	return s.server.Shutdown(ctx)
}

// SetScraperService sets the scraper service for the server
func (s *Server) SetScraperService(scraper ScraperService) {
	s.scraper = scraper
}

// GetSpotifyService returns the server's Spotify service for reuse by other components
func (s *Server) GetSpotifyService() types.SpotifyService {
	return s.spotify
}

// GetPlaylistManager returns the server's playlist manager for reuse by other components
func (s *Server) GetPlaylistManager() types.PlaylistManager {
	return s.playlist
}

// setupRoutes configures all HTTP routes with security and logging middleware
func (s *Server) setupRoutes() {
	// Create a new mux for routes that will have middleware applied
	protectedMux := http.NewServeMux()

	// Static file serving (no middleware needed)
	s.setupStaticRoutes()

	// Web interface routes
	protectedMux.HandleFunc("/", s.handleIndex)
	protectedMux.HandleFunc("/api/csrf-token", s.handleCSRFToken)

	// Authentication routes (no CSRF protection needed for these)
	s.router.HandleFunc("/auth", s.handleAuth)
	s.router.HandleFunc("/callback", s.handleCallback)

	// API routes
	protectedMux.HandleFunc("/api/add-artist", s.handleAddArtist)
	protectedMux.HandleFunc("/api/playlists", s.handleGetPlaylists)
	protectedMux.HandleFunc("/api/auth-status", s.handleAuthStatus)
	protectedMux.HandleFunc("/api/scrape-artists", s.handleScrapeArtists)

	// Apply middleware chain: logging -> security
	var handler http.Handler = protectedMux

	// Apply security middleware chain
	handler = s.securityMiddleware.SecurityHeaders(
		s.securityMiddleware.RateLimit(
			s.securityMiddleware.InputValidation(
				s.securityMiddleware.CSRFProtection(handler),
			),
		),
	)

	// Apply logging middleware (outermost layer)
	if s.config.Logging.EnableHTTP {
		handler = s.loggingMiddleware.LogRequests(handler)
	}

	// Mount the protected handler
	s.router.Handle("/", handler)
}

// setupStaticRoutes configures static file serving using embedded files
func (s *Server) setupStaticRoutes() {
	staticHandler := http.FileServer(http.FS(staticFiles))
	s.router.Handle("/static/", staticHandler)
}

// handleCSRFToken generates and returns a CSRF token
func (s *Server) handleCSRFToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := s.securityMiddleware.GenerateCSRFToken()
	if token == "" {
		http.Error(w, "Failed to generate CSRF token", http.StatusInternalServerError)
		return
	}

	response := map[string]string{"csrf_token": token}
	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleIndex serves the main web interface
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Only serve index.html for the root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Serve the embedded index.html file
	indexHTML, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		s.logger.WithContext(r.Context()).WithField("component", "server").WithError(err).Error("Failed to read index.html")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	// nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
	if _, err := w.Write(indexHTML); err != nil {
		s.logger.WithContext(r.Context()).WithField("component", "server").WithError(err).Error("Failed to write index.html response")
	}
}

// handleGetPlaylists retrieves and filters playlists from the "Incoming" folder
func (s *Server) handleGetPlaylists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.logger.WithContext(r.Context()).WithFields(logrus.Fields{
		"component": "server",
		"operation": "get_playlists",
	}).Debug("Handling playlist retrieval request")

	// Get playlists from the playlist manager
	playlists, err := s.playlist.GetIncomingPlaylists()
	if err != nil {
		s.logger.WithContext(r.Context()).WithField("component", "server").WithError(err).Error("Failed to retrieve playlists")
		s.writeJSONError(w, "Failed to retrieve playlists: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check for search filter
	searchTerm := r.URL.Query().Get("search")
	if searchTerm != "" {
		playlists = s.playlist.FilterPlaylistsBySearch(playlists, searchTerm)
		s.logger.WithContext(r.Context()).WithFields(logrus.Fields{
			"component":      "server",
			"search_term":    searchTerm,
			"filtered_count": len(playlists),
		}).Debug("Filtered playlists by search term")
	}

	// Generate embed URLs for each playlist (create a copy to avoid data races)
	playlistsCopy := make([]types.Playlist, len(playlists))
	for i, playlist := range playlists {
		playlistsCopy[i] = playlist
		playlistsCopy[i].EmbedURL = s.generateEmbedURL(playlist.URI)
	}

	response := types.APIResponse{
		Success: true,
		Data:    playlistsCopy,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleAddArtist handles adding an artist to a playlist with duplicate checking
func (s *Server) handleAddArtist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.AddArtistRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.logger.WithContext(r.Context()).WithField("component", "server").WithError(err).Warn("Invalid JSON request")
		s.writeJSONError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := s.validateAddArtistRequest(&req); err != nil {
		s.logger.WithContext(r.Context()).WithError(err).WithFields(logrus.Fields{
			"component":   "server",
			"artist_name": req.ArtistName,
			"playlist_id": req.PlaylistID,
		}).Warn("Invalid add artist request")
		s.writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.logger.WithContext(r.Context()).WithFields(logrus.Fields{
		"component":   "server",
		"artist_name": req.ArtistName,
		"playlist_id": req.PlaylistID,
		"force":       req.Force,
	}).Info("Processing add artist request")

	// Add artist to playlist
	result, err := s.playlist.AddArtistToPlaylist(req.ArtistName, req.PlaylistID, req.Force)
	if err != nil {
		s.logger.WithContext(r.Context()).WithField("component", "server").WithError(err).Error("Failed to add artist to playlist")
		s.writeJSONError(w, "Failed to add artist: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create appropriate response based on result
	switch {
	case result.Success:
		response := types.WebUIResponse{
			Success: true,
			Message: result.Message,
			Data:    result,
		}
		s.writeJSONResponse(w, response, http.StatusOK)
	case result.WasDuplicate:
		response := types.WebUIResponse{
			Success:     false,
			Message:     result.Message,
			IsDuplicate: true,
			Data:        result,
		}
		s.writeJSONResponse(w, response, http.StatusOK)
	default:
		s.writeJSONError(w, result.Message, http.StatusBadRequest)
	}
}

// handleScrapeArtists handles scraping artists from a URL and adding them to a playlist
func (s *Server) handleScrapeArtists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if scraper service is available
	if s.scraper == nil {
		s.logger.WithContext(r.Context()).WithField("component", "server").Error("Scraper service not initialized")
		s.writeJSONError(w, "Scraper service not available", http.StatusServiceUnavailable)
		return
	}

	var req types.ScrapeArtistsRequest
	if err := s.parseJSONRequest(r, &req); err != nil {
		s.logger.WithContext(r.Context()).WithField("component", "server").WithError(err).Warn("Invalid JSON request")
		s.writeJSONError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := s.validateScrapeArtistsRequest(&req); err != nil {
		s.logger.WithContext(r.Context()).WithError(err).WithFields(logrus.Fields{
			"component":    "server",
			"url":          req.URL,
			"css_selector": req.CSSSelector,
			"playlist_id":  req.PlaylistID,
		}).Warn("Invalid scrape artists request")
		s.writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.logger.WithContext(r.Context()).WithFields(logrus.Fields{
		"component":    "server",
		"url":          req.URL,
		"css_selector": req.CSSSelector,
		"playlist_id":  req.PlaylistID,
		"force":        req.Force,
	}).Info("Processing scrape artists request")

	// Perform scraping operation
	result, err := s.scraper.ScrapeAndAddToPlaylist(req.URL, req.CSSSelector, req.PlaylistID, req.Force)
	if err != nil {
		s.logger.WithContext(r.Context()).WithField("component", "server").WithError(err).Error("Failed to scrape artists")
		s.writeJSONError(w, "Failed to scrape artists: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return successful response
	response := types.ScrapeArtistsResponse{
		Success: true,
		Data:    result,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// Helper methods

// parseJSONRequest parses JSON request body into the provided struct
func (s *Server) parseJSONRequest(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

// validateAddArtistRequest validates the add artist request
func (s *Server) validateAddArtistRequest(req *types.AddArtistRequest) error {
	if strings.TrimSpace(req.ArtistName) == "" {
		return fmt.Errorf("artist name is required")
	}
	if len(req.ArtistName) > 100 {
		return fmt.Errorf("artist name too long (max 100 characters)")
	}
	if strings.TrimSpace(req.PlaylistID) == "" {
		return fmt.Errorf("playlist ID is required")
	}
	return nil
}

// validateScrapeArtistsRequest validates the scrape artists request
func (s *Server) validateScrapeArtistsRequest(req *types.ScrapeArtistsRequest) error {
	// Validate URL
	if strings.TrimSpace(req.URL) == "" {
		return fmt.Errorf("URL is required")
	}

	// Basic URL validation - check for http/https scheme
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		return fmt.Errorf("URL must start with http:// or https://")
	}

	// Validate CSS selector length
	if len(req.CSSSelector) > 500 {
		return fmt.Errorf("CSS selector too long (max 500 characters)")
	}

	// Validate playlist ID
	if strings.TrimSpace(req.PlaylistID) == "" {
		return fmt.Errorf("playlist ID is required")
	}

	return nil
}

// generateEmbedURL generates a Spotify embed URL from a playlist URI
func (s *Server) generateEmbedURL(playlistURI string) string {
	// Convert spotify:playlist:ID to https://open.spotify.com/embed/playlist/ID
	if strings.HasPrefix(playlistURI, "spotify:playlist:") {
		playlistID := strings.TrimPrefix(playlistURI, "spotify:playlist:")
		// Add theme parameter for better embedding experience
		return fmt.Sprintf("https://open.spotify.com/embed/playlist/%s?utm_source=generator&theme=0", playlistID)
	}
	return ""
}

// writeJSONResponse writes a JSON response
func (s *Server) writeJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.WithField("component", "server").WithError(err).Error("Failed to encode JSON response")
	}
}

// writeJSONError writes a JSON error response
func (s *Server) writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	response := types.APIResponse{
		Success: false,
		Error:   message,
	}
	s.writeJSONResponse(w, response, statusCode)
}

// handleAuth redirects to Spotify authorization
func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authURL := s.spotify.GetAuthURL()
	if authURL == "" {
		http.Error(w, "Authentication not available", http.StatusInternalServerError)
		return
	}

	s.logger.WithContext(r.Context()).WithFields(logrus.Fields{
		"component": "server",
		"operation": "auth_redirect",
		"auth_url":  authURL,
	}).Info("Redirecting to Spotify authentication")

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// handleCallback handles the OAuth callback from Spotify
func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		s.logger.WithContext(r.Context()).WithFields(logrus.Fields{
			"component": "server",
			"operation": "auth_callback",
			"error":     errorParam,
		}).Error("Spotify authentication error")
		http.Error(w, "Authentication failed: "+errorParam, http.StatusBadRequest)
		return
	}

	if code == "" || state == "" {
		s.logger.WithContext(r.Context()).WithField("component", "server").Error("Missing code or state in callback")
		http.Error(w, "Invalid callback parameters", http.StatusBadRequest)
		return
	}

	s.logger.WithContext(r.Context()).WithFields(logrus.Fields{
		"component": "server",
		"operation": "auth_callback",
		"state":     state,
	}).Info("Processing Spotify authentication callback")

	err := s.spotify.CompleteAuth(code, state)
	if err != nil {
		s.logger.WithContext(r.Context()).WithField("component", "server").WithError(err).Error("Failed to complete authentication")
		http.Error(w, "Authentication failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.logger.WithContext(r.Context()).WithField("component", "server").Info("Spotify authentication completed successfully")

	// Redirect to main page with success message
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>Authentication Successful</title>
			<style>
				body { font-family: Arial, sans-serif; text-align: center; margin-top: 50px; }
				.success { color: green; }
				.button { background-color: #1db954; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px; }
			</style>
		</head>
		<body>
			<h1 class="success">✓ Authentication Successful!</h1>
			<p>You have successfully authenticated with Spotify.</p>
			<a href="/" class="button">Go to Application</a>
		</body>
		</html>
	`)); err != nil {
		s.logger.WithContext(r.Context()).WithField("component", "server").WithError(err).Error("Failed to write authentication success response")
	}
}

// handleAuthStatus returns the current authentication status
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	isAuthenticated := s.spotify.IsAuthenticated()
	authURL := ""
	if !isAuthenticated {
		authURL = s.spotify.GetAuthURL()
	}

	response := types.APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"authenticated": isAuthenticated,
			"auth_url":      authURL,
		},
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}
