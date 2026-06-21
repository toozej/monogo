package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/internal/middleware"
	"github.com/toozej/go-listen/internal/types"
	"github.com/toozej/go-listen/pkg/config"
	"github.com/toozej/go-listen/pkg/logging"
)

// TestServerIntegration tests complete request flows through the server
func TestServerIntegration(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T, server *Server)
	}{
		{
			name:        "complete_web_interface_flow",
			description: "Test complete flow from web interface to API responses",
			testFunc:    testCompleteWebInterfaceFlow,
		},
		{
			name:        "api_endpoint_integration",
			description: "Test API endpoints with real request/response cycles",
			testFunc:    testAPIEndpointIntegration,
		},
		{
			name:        "error_handling_integration",
			description: "Test error handling and recovery scenarios",
			testFunc:    testErrorHandlingIntegration,
		},
		{
			name:        "security_integration",
			description: "Test rate limiting and security protection mechanisms",
			testFunc:    testSecurityIntegration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := createIntegrationTestServer()
			tt.testFunc(t, server)
		})
	}
}

func createIntegrationTestServer() (*Server, *enhancedMockPlaylistManager) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Spotify: config.SpotifyConfig{
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
		},
		Security: config.SecurityConfig{
			RateLimit: config.RateLimitConfig{
				RequestsPerSecond: 10,
				Burst:             20,
			},
		},
		Logging: config.LoggingConfig{
			Level:      "error",
			Format:     "json",
			Output:     "stdout",
			EnableHTTP: true,
		},
	}

	logrusLogger := log.New()
	logrusLogger.SetLevel(log.ErrorLevel)
	logger := &logging.Logger{Logger: logrusLogger}

	// Initialize middleware components
	rateLimiter := middleware.NewRateLimiter(cfg.Security.RateLimit.RequestsPerSecond, cfg.Security.RateLimit.Burst)
	securityMiddleware := middleware.NewSecurityMiddleware(logger.Logger, rateLimiter)
	loggingMiddleware := middleware.NewLoggingMiddleware(logger)

	// Create enhanced mock playlist manager for integration tests
	mockPlaylist := &enhancedMockPlaylistManager{
		playlists: []types.Playlist{
			{
				ID:         "playlist1",
				Name:       "Rock Incoming",
				URI:        "spotify:playlist:playlist1",
				TrackCount: 15,
				EmbedURL:   "https://open.spotify.com/embed/playlist/playlist1",
				IsIncoming: true,
			},
			{
				ID:         "playlist2",
				Name:       "Jazz Incoming",
				URI:        "spotify:playlist:playlist2",
				TrackCount: 8,
				EmbedURL:   "https://open.spotify.com/embed/playlist/playlist2",
				IsIncoming: true,
			},
		},
		addResults: map[string]*types.AddResult{
			"success": {
				Success: true,
				Artist: types.Artist{
					ID:   "artist1",
					Name: "Test Artist",
					URI:  "spotify:artist:artist1",
				},
				TracksAdded: []types.Track{
					{ID: "track1", Name: "Song 1", URI: "spotify:track:track1"},
					{ID: "track2", Name: "Song 2", URI: "spotify:track:track2"},
				},
				Message: "Successfully added Test Artist's top tracks to playlist",
			},
			"duplicate": {
				Success:      false,
				WasDuplicate: true,
				Artist: types.Artist{
					ID:   "artist1",
					Name: "Test Artist",
					URI:  "spotify:artist:artist1",
				},
				Message: "Artist 'Test Artist' already has tracks in this playlist. Use 'Add Anyway' to override.",
			},
		},
	}

	server := &Server{
		router:             http.NewServeMux(),
		config:             cfg,
		logger:             logger,
		playlist:           mockPlaylist,
		rateLimiter:        rateLimiter,
		securityMiddleware: securityMiddleware,
		loggingMiddleware:  loggingMiddleware,
	}

	// Set up routes
	server.setupRoutes()

	return server, mockPlaylist
}

// enhancedMockPlaylistManager provides more sophisticated mocking for integration tests
type enhancedMockPlaylistManager struct {
	playlists  []types.Playlist
	addResults map[string]*types.AddResult
	callCount  int
	mu         sync.Mutex
}

func (m *enhancedMockPlaylistManager) AddArtistToPlaylist(artistName, playlistID string, force bool) (*types.AddResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++

	// Simulate different scenarios based on artist name
	switch strings.ToLower(artistName) {
	case "error artist":
		return nil, fmt.Errorf("spotify API error")
	case "rate limit artist":
		return nil, fmt.Errorf("HTTP 429: Rate limit exceeded")
	case "duplicate artist":
		if force {
			return m.addResults["success"], nil
		}
		return m.addResults["duplicate"], nil
	default:
		return m.addResults["success"], nil
	}
}

func (m *enhancedMockPlaylistManager) GetIncomingPlaylists() ([]types.Playlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.playlists, nil
}

func (m *enhancedMockPlaylistManager) GetTop5Tracks(artistID string) ([]types.Track, error) {
	return nil, nil
}

func (m *enhancedMockPlaylistManager) FilterPlaylistsBySearch(playlists []types.Playlist, searchTerm string) []types.Playlist {
	if searchTerm == "" {
		return playlists
	}

	filtered := make([]types.Playlist, 0)
	searchLower := strings.ToLower(searchTerm)
	for _, playlist := range playlists {
		if strings.Contains(strings.ToLower(playlist.Name), searchLower) {
			filtered = append(filtered, playlist)
		}
	}
	return filtered
}

func (m *enhancedMockPlaylistManager) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return nil
}

func (m *enhancedMockPlaylistManager) CheckForDuplicates(playlistID string, trackIDs []string) (*types.DuplicateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return &types.DuplicateResult{
		HasDuplicates: false,
	}, nil
}

func (m *enhancedMockPlaylistManager) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func testCompleteWebInterfaceFlow(t *testing.T, server *Server) {
	// Test 1: Load main page
	req := httptest.NewRequest("GET", "/", http.NoBody)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for main page, got %d", w.Code)
	}

	if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
		t.Error("Expected HTML content type for main page")
	}

	// Test 2: Get playlists for dropdown
	req = httptest.NewRequest("GET", "/api/playlists", http.NoBody)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for playlists API, got %d", w.Code)
	}

	var playlistResponse types.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &playlistResponse); err != nil {
		t.Fatalf("Failed to unmarshal playlist response: %v", err)
	}

	if !playlistResponse.Success {
		t.Error("Expected successful playlist response")
	}

	playlists := playlistResponse.Data.([]interface{})
	if len(playlists) != 2 {
		t.Errorf("Expected 2 playlists, got %d", len(playlists))
	}

	// Test 3: Search playlists
	req = httptest.NewRequest("GET", "/api/playlists?search=rock", http.NoBody)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for playlist search, got %d", w.Code)
	}

	if err := json.Unmarshal(w.Body.Bytes(), &playlistResponse); err != nil {
		t.Fatalf("Failed to unmarshal search response: %v", err)
	}

	filteredPlaylists := playlistResponse.Data.([]interface{})
	if len(filteredPlaylists) != 1 {
		t.Errorf("Expected 1 filtered playlist, got %d", len(filteredPlaylists))
	}

	// Test 4: Add artist to playlist
	addRequest := types.AddArtistRequest{
		ArtistName: "Test Artist",
		PlaylistID: "playlist1",
		Force:      false,
	}

	reqBody, _ := json.Marshal(addRequest)
	req = httptest.NewRequest("POST", "/api/add-artist", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", server.securityMiddleware.GenerateCSRFToken())
	w = httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for add artist, got %d", w.Code)
	}

	var addResponse types.WebUIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &addResponse); err != nil {
		t.Fatalf("Failed to unmarshal add response: %v", err)
	}

	if !addResponse.Success {
		t.Errorf("Expected successful add response, got: %s", addResponse.Message)
	}

	// Verify correlation ID is consistent across requests
	correlationID := w.Header().Get("X-Correlation-ID")
	if correlationID == "" {
		t.Error("Expected correlation ID in response headers")
	}
}

func testAPIEndpointIntegration(t *testing.T, server *Server) {
	tests := []struct {
		name           string
		method         string
		path           string
		body           interface{}
		headers        map[string]string
		expectedStatus int
		validateFunc   func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:           "get_playlists_api",
			method:         "GET",
			path:           "/api/playlists",
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response types.APIResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}
				if !response.Success {
					t.Error("Expected successful response")
				}
			},
		},
		{
			name:   "add_artist_api_success",
			method: "POST",
			path:   "/api/add-artist",
			body: types.AddArtistRequest{
				ArtistName: "Test Artist",
				PlaylistID: "playlist1",
				Force:      false,
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response types.WebUIResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}
				if !response.Success {
					t.Errorf("Expected successful response, got: %s", response.Message)
				}
			},
		},
		{
			name:   "add_artist_api_duplicate",
			method: "POST",
			path:   "/api/add-artist",
			body: types.AddArtistRequest{
				ArtistName: "Duplicate Artist",
				PlaylistID: "playlist1",
				Force:      false,
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response types.WebUIResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}
				if response.Success {
					t.Error("Expected duplicate detection to prevent success")
				}
				if !response.IsDuplicate {
					t.Error("Expected duplicate flag to be set")
				}
			},
		},
		{
			name:   "add_artist_api_force_override",
			method: "POST",
			path:   "/api/add-artist",
			body: types.AddArtistRequest{
				ArtistName: "Duplicate Artist",
				PlaylistID: "playlist1",
				Force:      true,
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response types.WebUIResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}
				if !response.Success {
					t.Errorf("Expected force override to succeed, got: %s", response.Message)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody []byte
			if tt.body != nil {
				var err error
				reqBody, err = json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("Failed to marshal request body: %v", err)
				}
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(reqBody))

			// Add headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Add CSRF token for POST requests
			if tt.method == "POST" {
				req.Header.Set("X-CSRF-Token", server.securityMiddleware.GenerateCSRFToken())
			}

			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, w)
			}

			// Verify security headers are present
			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Error("Expected security headers to be set")
			}

			// Verify correlation ID is set
			if w.Header().Get("X-Correlation-ID") == "" {
				t.Error("Expected correlation ID header")
			}
		})
	}
}

func testErrorHandlingIntegration(t *testing.T, server *Server) {
	tests := []struct {
		name           string
		method         string
		path           string
		body           interface{}
		headers        map[string]string
		expectedStatus int
		description    string
	}{
		{
			name:   "api_error_handling",
			method: "POST",
			path:   "/api/add-artist",
			body: types.AddArtistRequest{
				ArtistName: "Error Artist",
				PlaylistID: "playlist1",
				Force:      false,
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Test API error handling",
		},
		{
			name:   "rate_limit_error_handling",
			method: "POST",
			path:   "/api/add-artist",
			body: types.AddArtistRequest{
				ArtistName: "Rate Limit Artist",
				PlaylistID: "playlist1",
				Force:      false,
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			expectedStatus: http.StatusInternalServerError,
			description:    "Test rate limit error handling",
		},
		{
			name:           "invalid_json_handling",
			method:         "POST",
			path:           "/api/add-artist",
			body:           "invalid json",
			headers:        map[string]string{"Content-Type": "application/json"},
			expectedStatus: http.StatusBadRequest,
			description:    "Test invalid JSON handling",
		},
		{
			name:   "validation_error_handling",
			method: "POST",
			path:   "/api/add-artist",
			body: types.AddArtistRequest{
				ArtistName: "",
				PlaylistID: "playlist1",
				Force:      false,
			},
			headers:        map[string]string{"Content-Type": "application/json"},
			expectedStatus: http.StatusBadRequest,
			description:    "Test validation error handling",
		},
		{
			name:           "method_not_allowed_handling",
			method:         "PUT",
			path:           "/api/playlists",
			expectedStatus: http.StatusForbidden, // CSRF protection blocks it first
			description:    "Test method not allowed handling",
		},
		{
			name:           "not_found_handling",
			method:         "GET",
			path:           "/nonexistent",
			expectedStatus: http.StatusNotFound,
			description:    "Test 404 handling",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody []byte
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					reqBody = []byte(str)
				} else {
					var err error
					reqBody, err = json.Marshal(tt.body)
					if err != nil {
						t.Fatalf("Failed to marshal request body: %v", err)
					}
				}
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(reqBody))

			// Add headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Add CSRF token for POST requests
			if tt.method == "POST" {
				req.Header.Set("X-CSRF-Token", server.securityMiddleware.GenerateCSRFToken())
			}

			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("%s: Expected status %d, got %d", tt.description, tt.expectedStatus, w.Code)
			}

			// Verify error responses have proper structure
			if w.Code >= 400 {
				contentType := w.Header().Get("Content-Type")
				if !strings.Contains(contentType, "application/json") &&
					!strings.Contains(contentType, "text/html") &&
					!strings.Contains(contentType, "text/plain") {
					t.Errorf("%s: Expected JSON, HTML, or plain text content type for error, got %s", tt.description, contentType)
				}
			}

			// Verify correlation ID is still set for errors
			if w.Header().Get("X-Correlation-ID") == "" {
				t.Errorf("%s: Expected correlation ID header even for errors", tt.description)
			}
		})
	}
}

func testSecurityIntegration(t *testing.T, server *Server) {
	// Test rate limiting
	t.Run("rate_limiting_integration", func(t *testing.T) {
		// Make multiple rapid requests to trigger rate limiting
		var wg sync.WaitGroup
		results := make(chan int, 25)

		for i := 0; i < 25; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest("GET", "/api/playlists", http.NoBody)
				req.RemoteAddr = "192.168.1.100:12345" // Same IP for all requests
				w := httptest.NewRecorder()
				server.router.ServeHTTP(w, req)
				results <- w.Code
			}()
		}

		wg.Wait()
		close(results)

		successCount := 0
		rateLimitCount := 0

		for code := range results {
			switch code {
			case http.StatusOK:
				successCount++
			case http.StatusTooManyRequests:
				rateLimitCount++
			}
		}

		if rateLimitCount == 0 {
			t.Error("Expected some requests to be rate limited")
		}

		if successCount == 0 {
			t.Error("Expected some requests to succeed")
		}

		t.Logf("Rate limiting test: %d successful, %d rate limited", successCount, rateLimitCount)
	})

	// Test CSRF protection
	t.Run("csrf_protection_integration", func(t *testing.T) {
		addRequest := types.AddArtistRequest{
			ArtistName: "Test Artist",
			PlaylistID: "playlist1",
			Force:      false,
		}

		reqBody, _ := json.Marshal(addRequest)

		// Test without CSRF token
		req := httptest.NewRequest("POST", "/api/add-artist", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected CSRF protection to block request without token, got status %d", w.Code)
		}

		// Test with valid CSRF token
		req = httptest.NewRequest("POST", "/api/add-artist", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", server.securityMiddleware.GenerateCSRFToken())
		w = httptest.NewRecorder()
		server.router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected valid CSRF token to allow request, got status %d", w.Code)
		}
	})

	// Test input validation
	t.Run("input_validation_integration", func(t *testing.T) {
		maliciousInputs := []string{
			"/api/playlists?search=%3Cscript%3Ealert%28%27xss%27%29%3C%2Fscript%3E",
			"/api/playlists?search=..%2F..%2F..%2Fetc%2Fpasswd",
			"/api/playlists?search=1%27%20OR%20%271%27%3D%271",
		}

		for _, path := range maliciousInputs {
			req := httptest.NewRequest("GET", path, http.NoBody)
			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected malicious input to be blocked for path %s, got status %d", path, w.Code)
			}
		}
	})

	// Test security headers
	t.Run("security_headers_integration", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", http.NoBody)
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)

		expectedHeaders := map[string]string{
			"X-Content-Type-Options": "nosniff",
			"X-Frame-Options":        "DENY",
			"X-XSS-Protection":       "1; mode=block",
		}

		for header, expectedValue := range expectedHeaders {
			if got := w.Header().Get(header); got != expectedValue {
				t.Errorf("Expected header %s to be %s, got %s", header, expectedValue, got)
			}
		}
	})
}

// TestEndToEndWorkflow tests a complete end-to-end workflow
func TestEndToEndWorkflow(t *testing.T) {
	server, mockPlaylist := createIntegrationTestServer()

	// Step 1: User loads the main page
	req := httptest.NewRequest("GET", "/", http.NoBody)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to load main page: status %d", w.Code)
	}

	// Step 2: User gets list of playlists
	req = httptest.NewRequest("GET", "/api/playlists", http.NoBody)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to get playlists: status %d", w.Code)
	}

	var playlistResponse types.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &playlistResponse); err != nil {
		t.Fatalf("Failed to parse playlist response: %v", err)
	}

	playlists := playlistResponse.Data.([]interface{})
	if len(playlists) == 0 {
		t.Fatal("No playlists returned")
	}

	// Step 3: User searches for specific playlist
	req = httptest.NewRequest("GET", "/api/playlists?search=rock", http.NoBody)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to search playlists: status %d", w.Code)
	}

	// Step 4: User attempts to add artist (duplicate detected)
	addRequest := types.AddArtistRequest{
		ArtistName: "Duplicate Artist",
		PlaylistID: "playlist1",
		Force:      false,
	}

	reqBody, _ := json.Marshal(addRequest)
	req = httptest.NewRequest("POST", "/api/add-artist", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", server.securityMiddleware.GenerateCSRFToken())
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to process add artist request: status %d", w.Code)
	}

	var addResponse types.WebUIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &addResponse); err != nil {
		t.Fatalf("Failed to parse add response: %v", err)
	}

	if addResponse.Success {
		t.Error("Expected duplicate detection to prevent success")
	}

	if !addResponse.IsDuplicate {
		t.Error("Expected duplicate flag to be set")
	}

	// Step 5: User overrides duplicate detection
	addRequest.Force = true
	reqBody, _ = json.Marshal(addRequest)
	req = httptest.NewRequest("POST", "/api/add-artist", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", server.securityMiddleware.GenerateCSRFToken())
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to process override request: status %d", w.Code)
	}

	if err := json.Unmarshal(w.Body.Bytes(), &addResponse); err != nil {
		t.Fatalf("Failed to parse override response: %v", err)
	}

	if !addResponse.Success {
		t.Errorf("Expected override to succeed, got: %s", addResponse.Message)
	}

	// Verify that the mock was called appropriately
	if mockPlaylist.GetCallCount() < 3 {
		t.Errorf("Expected multiple calls to playlist manager, got %d", mockPlaylist.GetCallCount())
	}

	t.Log("End-to-end workflow completed successfully")
}

// TestConcurrentRequestHandling tests handling of concurrent requests
func TestConcurrentRequestHandling(t *testing.T) {
	server, _ := createIntegrationTestServer()

	numGoroutines := 20
	numRequestsPerGoroutine := 5
	var wg sync.WaitGroup
	type testResult struct {
		statusCode int
		endpoint   string
		error      error
	}

	results := make(chan testResult, numGoroutines*numRequestsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numRequestsPerGoroutine; j++ {
				// Alternate between different endpoints
				switch j % 3 {
				case 0:
					// GET playlists
					req := httptest.NewRequest("GET", "/api/playlists", http.NoBody)
					req.RemoteAddr = fmt.Sprintf("192.168.1.%d:12345", goroutineID%255)
					w := httptest.NewRecorder()
					server.router.ServeHTTP(w, req)
					results <- testResult{statusCode: w.Code, endpoint: "playlists"}

				case 1:
					// GET main page
					req := httptest.NewRequest("GET", "/", http.NoBody)
					req.RemoteAddr = fmt.Sprintf("192.168.1.%d:12345", goroutineID%255)
					w := httptest.NewRecorder()
					server.router.ServeHTTP(w, req)
					results <- testResult{statusCode: w.Code, endpoint: "main"}

				case 2:
					// POST add artist
					addRequest := types.AddArtistRequest{
						ArtistName: fmt.Sprintf("Artist %d-%d", goroutineID, j),
						PlaylistID: "playlist1",
						Force:      false,
					}
					reqBody, _ := json.Marshal(addRequest)
					req := httptest.NewRequest("POST", "/api/add-artist", bytes.NewReader(reqBody))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-CSRF-Token", server.securityMiddleware.GenerateCSRFToken())
					req.RemoteAddr = fmt.Sprintf("192.168.1.%d:12345", goroutineID%255)
					w := httptest.NewRecorder()
					server.router.ServeHTTP(w, req)
					results <- testResult{statusCode: w.Code, endpoint: "add-artist"}
				}
			}
		}(i)
	}

	wg.Wait()
	close(results)

	// Analyze results
	statusCounts := make(map[int]int)
	endpointCounts := make(map[string]int)
	errorCount := 0

	for res := range results {
		statusCounts[res.statusCode]++
		endpointCounts[res.endpoint]++
		if res.error != nil {
			errorCount++
		}
	}

	// Verify results
	totalRequests := numGoroutines * numRequestsPerGoroutine
	processedRequests := 0
	for status, count := range statusCounts {
		processedRequests += count
		t.Logf("Status %d: %d requests", status, count)
	}

	if processedRequests != totalRequests {
		t.Errorf("Expected %d total requests, processed %d", totalRequests, processedRequests)
	}

	if errorCount > 0 {
		t.Errorf("Got %d errors during concurrent processing", errorCount)
	}

	// Most requests should succeed (some might be rate limited)
	successCount := statusCounts[200]
	if successCount < totalRequests/2 {
		t.Errorf("Expected at least half of requests to succeed, got %d/%d", successCount, totalRequests)
	}

	t.Logf("Concurrent test completed: %d total requests, %d successful, %d rate limited",
		totalRequests, statusCounts[200], statusCounts[429])
}
