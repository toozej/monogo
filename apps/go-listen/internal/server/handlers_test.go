package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/internal/middleware"
	"github.com/toozej/go-listen/internal/types"
	"github.com/toozej/go-listen/pkg/config"
	"github.com/toozej/go-listen/pkg/logging"
)

// Mock implementations for testing
type mockPlaylistManager struct {
	playlists []types.Playlist
	addResult *types.AddResult
	addError  error
}

func (m *mockPlaylistManager) AddArtistToPlaylist(artistName, playlistID string, force bool) (*types.AddResult, error) {
	if m.addError != nil {
		return nil, m.addError
	}
	return m.addResult, nil
}

func (m *mockPlaylistManager) GetIncomingPlaylists() ([]types.Playlist, error) {
	return m.playlists, nil
}

func (m *mockPlaylistManager) GetTop5Tracks(artistID string) ([]types.Track, error) {
	return nil, nil
}

func (m *mockPlaylistManager) FilterPlaylistsBySearch(playlists []types.Playlist, searchTerm string) []types.Playlist {
	filtered := make([]types.Playlist, 0)
	searchLower := strings.ToLower(searchTerm)
	for _, playlist := range playlists {
		if strings.Contains(strings.ToLower(playlist.Name), searchLower) {
			filtered = append(filtered, playlist)
		}
	}
	return filtered
}

func (m *mockPlaylistManager) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	if m.addError != nil {
		return m.addError
	}
	return nil
}

func (m *mockPlaylistManager) CheckForDuplicates(playlistID string, trackIDs []string) (*types.DuplicateResult, error) {
	if m.addError != nil {
		return nil, m.addError
	}
	return &types.DuplicateResult{
		HasDuplicates: false,
	}, nil
}

func createTestServer() (*Server, *mockPlaylistManager) {
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
			EnableHTTP: false,
		},
	}

	logrusLogger := log.New()
	logrusLogger.SetLevel(log.ErrorLevel) // Reduce noise in tests

	logger := &logging.Logger{Logger: logrusLogger}

	// Initialize middleware components like in NewServer
	rateLimiter := middleware.NewRateLimiter(cfg.Security.RateLimit.RequestsPerSecond, cfg.Security.RateLimit.Burst)
	securityMiddleware := middleware.NewSecurityMiddleware(logger.Logger, rateLimiter)
	loggingMiddleware := middleware.NewLoggingMiddleware(logger)

	mockPlaylist := &mockPlaylistManager{}
	server := &Server{
		router:             http.NewServeMux(),
		config:             cfg,
		logger:             logger,
		playlist:           mockPlaylist,
		rateLimiter:        rateLimiter,
		securityMiddleware: securityMiddleware,
		loggingMiddleware:  loggingMiddleware,
	}

	return server, mockPlaylist
}

func TestHandleIndex(t *testing.T) {
	server, _ := createTestServer()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedType   string
	}{
		{
			name:           "GET root path",
			method:         "GET",
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedType:   "text/html",
		},
		{
			name:           "POST not allowed",
			method:         "POST",
			path:           "/",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "non-root path returns 404",
			method:         "GET",
			path:           "/nonexistent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			w := httptest.NewRecorder()

			server.handleIndex(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedType != "" {
				contentType := w.Header().Get("Content-Type")
				if !strings.Contains(contentType, tt.expectedType) {
					t.Errorf("Expected content type to contain %s, got %s", tt.expectedType, contentType)
				}
			}
		})
	}
}

func TestHandleGetPlaylists(t *testing.T) {
	server, mockPlaylist := createTestServer()

	// Set up mock playlists
	mockPlaylists := []types.Playlist{
		{
			ID:         "playlist1",
			Name:       "My Incoming Playlist",
			URI:        "spotify:playlist:playlist1",
			TrackCount: 10,
		},
		{
			ID:         "playlist2",
			Name:       "Another Playlist",
			URI:        "spotify:playlist:playlist2",
			TrackCount: 5,
		},
		{
			ID:         "playlist3",
			Name:       "Rock Incoming",
			URI:        "spotify:playlist:playlist3",
			TrackCount: 15,
		},
	}

	mockPlaylist.playlists = mockPlaylists

	tests := []struct {
		name           string
		method         string
		query          string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "GET all playlists",
			method:         "GET",
			query:          "",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
		{
			name:           "GET with search filter - incoming",
			method:         "GET",
			query:          "?search=incoming",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "GET with search filter - rock",
			method:         "GET",
			query:          "?search=rock",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "GET with search filter - no matches",
			method:         "GET",
			query:          "?search=nonexistent",
			expectedStatus: http.StatusOK,
			expectedCount:  0,
		},
		{
			name:           "GET with empty search",
			method:         "GET",
			query:          "?search=",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
		{
			name:           "POST not allowed",
			method:         "POST",
			query:          "",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "PUT not allowed",
			method:         "PUT",
			query:          "",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "DELETE not allowed",
			method:         "DELETE",
			query:          "",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/playlists"+tt.query, http.NoBody)
			w := httptest.NewRecorder()

			server.handleGetPlaylists(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var response types.APIResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if !response.Success {
					t.Error("Expected successful response")
				}

				playlists, ok := response.Data.([]interface{})
				if !ok {
					t.Fatal("Expected data to be array of playlists")
				}

				if len(playlists) != tt.expectedCount {
					t.Errorf("Expected %d playlists, got %d", tt.expectedCount, len(playlists))
				}

				// Verify embed URLs are generated for playlists
				if len(playlists) > 0 {
					firstPlaylist := playlists[0].(map[string]interface{})
					embedURL, exists := firstPlaylist["embed_url"]
					if !exists {
						t.Error("Expected embed_url field in playlist response")
					}
					if embedURL == "" {
						t.Error("Expected non-empty embed_url")
					}
				}
			}
		})
	}
}

func TestHandleAddArtist(t *testing.T) {
	server, mockPlaylist := createTestServer()

	tests := []struct {
		name            string
		method          string
		requestBody     interface{}
		mockResult      *types.AddResult
		mockError       error
		expectedStatus  int
		expectedSuccess bool
	}{
		{
			name:   "successful add",
			method: "POST",
			requestBody: types.AddArtistRequest{
				ArtistName: "Test Artist",
				PlaylistID: "playlist1",
				Force:      false,
			},
			mockResult: &types.AddResult{
				Success: true,
				Message: "Successfully added Test Artist's top tracks to playlist",
			},
			expectedStatus:  http.StatusOK,
			expectedSuccess: true,
		},
		{
			name:   "successful add with force parameter",
			method: "POST",
			requestBody: types.AddArtistRequest{
				ArtistName: "Test Artist",
				PlaylistID: "playlist1",
				Force:      true,
			},
			mockResult: &types.AddResult{
				Success: true,
				Message: "Successfully added Test Artist's top tracks to playlist (forced)",
			},
			expectedStatus:  http.StatusOK,
			expectedSuccess: true,
		},
		{
			name:   "duplicate detected",
			method: "POST",
			requestBody: types.AddArtistRequest{
				ArtistName: "Test Artist",
				PlaylistID: "playlist1",
				Force:      false,
			},
			mockResult: &types.AddResult{
				Success:      false,
				WasDuplicate: true,
				Message:      "Artist already exists in playlist",
			},
			expectedStatus:  http.StatusOK,
			expectedSuccess: false,
		},
		{
			name:   "service error",
			method: "POST",
			requestBody: types.AddArtistRequest{
				ArtistName: "Test Artist",
				PlaylistID: "playlist1",
				Force:      false,
			},
			mockError:      fmt.Errorf("spotify API error"),
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "invalid request - empty artist name",
			method: "POST",
			requestBody: types.AddArtistRequest{
				ArtistName: "",
				PlaylistID: "playlist1",
				Force:      false,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "invalid request - empty playlist ID",
			method: "POST",
			requestBody: types.AddArtistRequest{
				ArtistName: "Test Artist",
				PlaylistID: "",
				Force:      false,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "invalid request - artist name too long",
			method: "POST",
			requestBody: types.AddArtistRequest{
				ArtistName: strings.Repeat("a", 101),
				PlaylistID: "playlist1",
				Force:      false,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GET not allowed",
			method:         "GET",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "PUT not allowed",
			method:         "PUT",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "DELETE not allowed",
			method:         "DELETE",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "invalid JSON",
			method:         "POST",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty request body",
			method:         "POST",
			requestBody:    nil,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock
			mockPlaylist.addResult = tt.mockResult
			mockPlaylist.addError = tt.mockError

			var body []byte
			if tt.requestBody != nil {
				if str, ok := tt.requestBody.(string); ok {
					body = []byte(str)
				} else {
					var err error
					body, err = json.Marshal(tt.requestBody)
					if err != nil {
						t.Fatalf("Failed to marshal request body: %v", err)
					}
				}
			}

			req := httptest.NewRequest(tt.method, "/api/add-artist", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.handleAddArtist(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var response types.WebUIResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if response.Success != tt.expectedSuccess {
					t.Errorf("Expected success %v, got %v", tt.expectedSuccess, response.Success)
				}
			}
		})
	}
}

func TestGenerateEmbedURL(t *testing.T) {
	server, _ := createTestServer()

	tests := []struct {
		name        string
		playlistURI string
		expected    string
	}{
		{
			name:        "valid playlist URI",
			playlistURI: "spotify:playlist:37i9dQZF1DXcBWIGoYBM5M",
			expected:    "https://open.spotify.com/embed/playlist/37i9dQZF1DXcBWIGoYBM5M?utm_source=generator&theme=0",
		},
		{
			name:        "invalid URI format",
			playlistURI: "invalid:uri:format",
			expected:    "",
		},
		{
			name:        "empty URI",
			playlistURI: "",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.generateEmbedURL(tt.playlistURI)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestValidateAddArtistRequest(t *testing.T) {
	server, _ := createTestServer()

	tests := []struct {
		name    string
		request *types.AddArtistRequest
		wantErr bool
	}{
		{
			name: "valid request",
			request: &types.AddArtistRequest{
				ArtistName: "Test Artist",
				PlaylistID: "playlist1",
			},
			wantErr: false,
		},
		{
			name: "empty artist name",
			request: &types.AddArtistRequest{
				ArtistName: "",
				PlaylistID: "playlist1",
			},
			wantErr: true,
		},
		{
			name: "whitespace only artist name",
			request: &types.AddArtistRequest{
				ArtistName: "   ",
				PlaylistID: "playlist1",
			},
			wantErr: true,
		},
		{
			name: "too long artist name",
			request: &types.AddArtistRequest{
				ArtistName: strings.Repeat("a", 101),
				PlaylistID: "playlist1",
			},
			wantErr: true,
		},
		{
			name: "empty playlist ID",
			request: &types.AddArtistRequest{
				ArtistName: "Test Artist",
				PlaylistID: "",
			},
			wantErr: true,
		},
		{
			name: "whitespace only playlist ID",
			request: &types.AddArtistRequest{
				ArtistName: "Test Artist",
				PlaylistID: "   ",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.validateAddArtistRequest(tt.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAddArtistRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAPIIntegration tests the integration between playlist and add-artist endpoints
func TestAPIIntegration(t *testing.T) {
	server, mockPlaylist := createTestServer()

	// Set up mock playlists
	mockPlaylists := []types.Playlist{
		{
			ID:         "playlist1",
			Name:       "My Incoming Playlist",
			URI:        "spotify:playlist:playlist1",
			TrackCount: 10,
		},
	}

	mockPlaylist.playlists = mockPlaylists
	mockPlaylist.addResult = &types.AddResult{
		Success: true,
		Message: "Successfully added Test Artist's top tracks to playlist",
		Artist: types.Artist{
			ID:   "artist1",
			Name: "Test Artist",
		},
		TracksAdded: []types.Track{
			{ID: "track1", Name: "Song 1"},
			{ID: "track2", Name: "Song 2"},
		},
	}

	// Test 1: Get playlists
	req1 := httptest.NewRequest("GET", "/api/playlists", http.NoBody)
	w1 := httptest.NewRecorder()
	server.handleGetPlaylists(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for playlists, got %d", w1.Code)
	}

	var playlistResponse types.APIResponse
	if err := json.Unmarshal(w1.Body.Bytes(), &playlistResponse); err != nil {
		t.Fatalf("Failed to unmarshal playlist response: %v", err)
	}

	playlists := playlistResponse.Data.([]interface{})
	if len(playlists) != 1 {
		t.Fatalf("Expected 1 playlist, got %d", len(playlists))
	}

	// Extract playlist ID for next test
	firstPlaylist := playlists[0].(map[string]interface{})
	playlistID := firstPlaylist["id"].(string)

	// Test 2: Add artist to the retrieved playlist
	addRequest := types.AddArtistRequest{
		ArtistName: "Test Artist",
		PlaylistID: playlistID,
		Force:      false,
	}

	reqBody, _ := json.Marshal(addRequest)
	req2 := httptest.NewRequest("POST", "/api/add-artist", bytes.NewReader(reqBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	server.handleAddArtist(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for add artist, got %d", w2.Code)
	}

	var addResponse types.WebUIResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &addResponse); err != nil {
		t.Fatalf("Failed to unmarshal add artist response: %v", err)
	}

	if !addResponse.Success {
		t.Error("Expected successful add artist response")
	}
}

// TestAPIErrorHandling tests error scenarios across API endpoints
func TestAPIErrorHandling(t *testing.T) {
	server, mockPlaylist := createTestServer()

	// Test playlist endpoint with service error
	mockPlaylist.addError = fmt.Errorf("service unavailable")

	// Test add-artist with service error
	addRequest := types.AddArtistRequest{
		ArtistName: "Test Artist",
		PlaylistID: "playlist1",
		Force:      false,
	}

	reqBody, _ := json.Marshal(addRequest)
	req := httptest.NewRequest("POST", "/api/add-artist", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleAddArtist(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for service error, got %d", w.Code)
	}

	var response types.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	if response.Success {
		t.Error("Expected unsuccessful response for service error")
	}

	if response.Error == "" {
		t.Error("Expected error message in response")
	}
}
