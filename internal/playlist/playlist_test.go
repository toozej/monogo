package playlist

import (
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/toozej/kmhd2spotify/internal/types"
)

// MockSpotifyService is a mock implementation of SpotifyService
type MockSpotifyService struct {
	playlists []types.Playlist
	err       error
}

func (m *MockSpotifyService) SearchArtist(query string) (*types.Artist, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *MockSpotifyService) GetArtistTopTracks(artistID string) ([]types.Track, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *MockSpotifyService) GetUserPlaylists(folderName string) ([]types.Playlist, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.playlists, nil
}

func (m *MockSpotifyService) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	return errors.New("not implemented in mock")
}

func (m *MockSpotifyService) CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *MockSpotifyService) GetAuthURL() string {
	return "mock-auth-url"
}

func (m *MockSpotifyService) IsAuthenticated() bool {
	return true
}

func (m *MockSpotifyService) CompleteAuth(code, state string) error {
	return nil
}

func (m *MockSpotifyService) CreatePlaylist(name, description string, public bool) (*types.Playlist, error) {
	return &types.Playlist{
		ID:         "test-playlist-id",
		Name:       name,
		URI:        "spotify:playlist:test-playlist-id",
		TrackCount: 0,
		EmbedURL:   "https://open.spotify.com/embed/playlist/test-playlist-id",
		IsIncoming: false,
	}, nil
}

// MockDuplicateDetector is a mock implementation of DuplicateDetector
type MockDuplicateDetector struct{}

func (m *MockDuplicateDetector) CheckDuplicates(playlistID string, tracks []types.Track) (*types.DuplicateResult, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *MockDuplicateDetector) CheckArtistInPlaylist(playlistID, artistID string) (*types.DuplicateResult, error) {
	return nil, errors.New("not implemented in mock")
}

func TestPlaylistService_GetIncomingPlaylists(t *testing.T) {
	tests := []struct {
		name           string
		mockPlaylists  []types.Playlist
		mockError      error
		expectedResult []types.Playlist
		expectedError  bool
	}{
		{
			name: "successful retrieval of incoming playlists",
			mockPlaylists: []types.Playlist{
				{
					ID:         "playlist1",
					Name:       "My Incoming Playlist",
					URI:        "spotify:playlist:playlist1",
					TrackCount: 10,
					EmbedURL:   "https://open.spotify.com/embed/playlist/playlist1",
					IsIncoming: true,
				},
				{
					ID:         "playlist2",
					Name:       "Another Incoming",
					URI:        "spotify:playlist:playlist2",
					TrackCount: 5,
					EmbedURL:   "https://open.spotify.com/embed/playlist/playlist2",
					IsIncoming: true,
				},
			},
			mockError: nil,
			expectedResult: []types.Playlist{
				{
					ID:         "playlist1",
					Name:       "My Incoming Playlist",
					URI:        "spotify:playlist:playlist1",
					TrackCount: 10,
					EmbedURL:   "https://open.spotify.com/embed/playlist/playlist1",
					IsIncoming: true,
				},
				{
					ID:         "playlist2",
					Name:       "Another Incoming",
					URI:        "spotify:playlist:playlist2",
					TrackCount: 5,
					EmbedURL:   "https://open.spotify.com/embed/playlist/playlist2",
					IsIncoming: true,
				},
			},
			expectedError: false,
		},
		{
			name:           "empty incoming folder",
			mockPlaylists:  []types.Playlist{},
			mockError:      nil,
			expectedResult: []types.Playlist{},
			expectedError:  false,
		},
		{
			name:           "spotify service error",
			mockPlaylists:  nil,
			mockError:      errors.New("spotify API error"),
			expectedResult: nil,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockSpotify := &MockSpotifyService{
				playlists: tt.mockPlaylists,
				err:       tt.mockError,
			}
			mockDuplicate := &MockDuplicateDetector{}
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel) // Reduce log noise in tests

			// Create service
			service := NewPlaylistService(mockSpotify, mockDuplicate, logger)

			// Execute
			result, err := service.GetIncomingPlaylists()

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestPlaylistService_FilterPlaylistsBySearch(t *testing.T) {
	tests := []struct {
		name           string
		playlists      []types.Playlist
		searchTerm     string
		expectedResult []types.Playlist
	}{
		{
			name: "filter by partial name match",
			playlists: []types.Playlist{
				{ID: "1", Name: "Rock Incoming", IsIncoming: true},
				{ID: "2", Name: "Jazz Collection", IsIncoming: true},
				{ID: "3", Name: "Pop Incoming", IsIncoming: true},
				{ID: "4", Name: "Classical Music", IsIncoming: true},
			},
			searchTerm: "incoming",
			expectedResult: []types.Playlist{
				{ID: "1", Name: "Rock Incoming", IsIncoming: true},
				{ID: "3", Name: "Pop Incoming", IsIncoming: true},
			},
		},
		{
			name: "case insensitive search",
			playlists: []types.Playlist{
				{ID: "1", Name: "ROCK MUSIC", IsIncoming: true},
				{ID: "2", Name: "jazz collection", IsIncoming: true},
				{ID: "3", Name: "Pop Songs", IsIncoming: true},
			},
			searchTerm: "ROCK",
			expectedResult: []types.Playlist{
				{ID: "1", Name: "ROCK MUSIC", IsIncoming: true},
			},
		},
		{
			name: "no matches found",
			playlists: []types.Playlist{
				{ID: "1", Name: "Rock Music", IsIncoming: true},
				{ID: "2", Name: "Jazz Collection", IsIncoming: true},
			},
			searchTerm:     "electronic",
			expectedResult: []types.Playlist{},
		},
		{
			name: "empty search term returns all playlists",
			playlists: []types.Playlist{
				{ID: "1", Name: "Rock Music", IsIncoming: true},
				{ID: "2", Name: "Jazz Collection", IsIncoming: true},
			},
			searchTerm: "",
			expectedResult: []types.Playlist{
				{ID: "1", Name: "Rock Music", IsIncoming: true},
				{ID: "2", Name: "Jazz Collection", IsIncoming: true},
			},
		},
		{
			name:           "empty playlist list",
			playlists:      []types.Playlist{},
			searchTerm:     "test",
			expectedResult: []types.Playlist{},
		},
		{
			name: "multiple word search",
			playlists: []types.Playlist{
				{ID: "1", Name: "My Favorite Rock Songs", IsIncoming: true},
				{ID: "2", Name: "Rock Collection", IsIncoming: true},
				{ID: "3", Name: "Jazz Favorites", IsIncoming: true},
			},
			searchTerm: "favorite",
			expectedResult: []types.Playlist{
				{ID: "1", Name: "My Favorite Rock Songs", IsIncoming: true},
				{ID: "3", Name: "Jazz Favorites", IsIncoming: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockSpotify := &MockSpotifyService{}
			mockDuplicate := &MockDuplicateDetector{}
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel) // Reduce log noise in tests

			service := NewPlaylistService(mockSpotify, mockDuplicate, logger)

			// Execute
			result := service.FilterPlaylistsBySearch(tt.playlists, tt.searchTerm)

			// Assert
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
func TestPlaylistService_AddArtistToPlaylist(t *testing.T) {
	tests := []struct {
		name                string
		artistName          string
		playlistID          string
		force               bool
		mockArtist          *types.Artist
		mockArtistError     error
		mockTracks          []types.Track
		mockTracksError     error
		mockDuplicateResult *types.DuplicateResult
		mockDuplicateError  error
		mockAddTracksError  error
		expectedSuccess     bool
		expectedDuplicate   bool
		expectedMessage     string
	}{
		{
			name:       "successful artist addition",
			artistName: "Test Artist",
			playlistID: "playlist123",
			force:      false,
			mockArtist: &types.Artist{
				ID:   "artist123",
				Name: "Test Artist",
				URI:  "spotify:artist:artist123",
			},
			mockArtistError: nil,
			mockTracks: []types.Track{
				{ID: "track1", Name: "Song 1", URI: "spotify:track:track1", Album: types.Album{ID: "album1", Name: "Album 1", Type: "album"}},
				{ID: "track2", Name: "Song 2", URI: "spotify:track:track2", Album: types.Album{ID: "album2", Name: "Album 2", Type: "album"}},
			},
			mockTracksError:     nil,
			mockDuplicateResult: &types.DuplicateResult{HasDuplicates: false},
			mockDuplicateError:  nil,
			mockAddTracksError:  nil,
			expectedSuccess:     true,
			expectedDuplicate:   false,
			expectedMessage:     "Successfully added Test Artist's top tracks to playlist",
		},
		{
			name:            "artist not found",
			artistName:      "Unknown Artist",
			playlistID:      "playlist123",
			force:           false,
			mockArtist:      nil,
			mockArtistError: errors.New("artist not found"),
			expectedSuccess: false,
			expectedMessage: "Failed to find artist: artist not found",
		},
		{
			name:       "artist has no tracks",
			artistName: "Test Artist",
			playlistID: "playlist123",
			force:      false,
			mockArtist: &types.Artist{
				ID:   "artist123",
				Name: "Test Artist",
				URI:  "spotify:artist:artist123",
			},
			mockArtistError:   nil,
			mockTracks:        []types.Track{},
			mockTracksError:   nil,
			expectedSuccess:   false,
			expectedDuplicate: false,
			expectedMessage:   "Artist has no tracks available",
		},
		{
			name:       "duplicate detected without force",
			artistName: "Test Artist",
			playlistID: "playlist123",
			force:      false,
			mockArtist: &types.Artist{
				ID:   "artist123",
				Name: "Test Artist",
				URI:  "spotify:artist:artist123",
			},
			mockArtistError: nil,
			mockTracks: []types.Track{
				{ID: "track1", Name: "Song 1", URI: "spotify:track:track1", Album: types.Album{ID: "album1", Name: "Album 1", Type: "album"}},
			},
			mockTracksError: nil,
			mockDuplicateResult: &types.DuplicateResult{
				HasDuplicates: true,
				Message:       "Artist tracks already exist in playlist",
			},
			mockDuplicateError: nil,
			expectedSuccess:    false,
			expectedDuplicate:  true,
			expectedMessage:    "Artist tracks already exist in playlist",
		},
		{
			name:       "force addition with duplicates",
			artistName: "Test Artist",
			playlistID: "playlist123",
			force:      true,
			mockArtist: &types.Artist{
				ID:   "artist123",
				Name: "Test Artist",
				URI:  "spotify:artist:artist123",
			},
			mockArtistError: nil,
			mockTracks: []types.Track{
				{ID: "track1", Name: "Song 1", URI: "spotify:track:track1"},
			},
			mockTracksError:    nil,
			mockAddTracksError: nil,
			expectedSuccess:    true,
			expectedDuplicate:  false,
			expectedMessage:    "Successfully added Test Artist's top tracks to playlist",
		},
		{
			name:       "failed to add tracks to playlist",
			artistName: "Test Artist",
			playlistID: "playlist123",
			force:      false,
			mockArtist: &types.Artist{
				ID:   "artist123",
				Name: "Test Artist",
				URI:  "spotify:artist:artist123",
			},
			mockArtistError: nil,
			mockTracks: []types.Track{
				{ID: "track1", Name: "Song 1", URI: "spotify:track:track1"},
			},
			mockTracksError:     nil,
			mockDuplicateResult: &types.DuplicateResult{HasDuplicates: false},
			mockDuplicateError:  nil,
			mockAddTracksError:  errors.New("playlist not found"),
			expectedSuccess:     false,
			expectedDuplicate:   false,
			expectedMessage:     "Failed to add tracks to playlist: playlist not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup enhanced mock
			mockSpotify := &EnhancedMockSpotifyService{
				artist:      tt.mockArtist,
				artistError: tt.mockArtistError,
				tracks:      tt.mockTracks,
				tracksError: tt.mockTracksError,
				addError:    tt.mockAddTracksError,
			}

			mockDuplicate := &EnhancedMockDuplicateDetector{
				result: tt.mockDuplicateResult,
				err:    tt.mockDuplicateError,
			}

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel) // Reduce log noise in tests

			service := NewPlaylistService(mockSpotify, mockDuplicate, logger)

			// Execute
			result, err := service.AddArtistToPlaylist(tt.artistName, tt.playlistID, tt.force)

			// Assert
			if result == nil {
				t.Fatal("Expected result but got nil")
			}

			if result.Success != tt.expectedSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectedSuccess, result.Success)
			}

			if result.WasDuplicate != tt.expectedDuplicate {
				t.Errorf("Expected duplicate %v, got %v", tt.expectedDuplicate, result.WasDuplicate)
			}

			if result.Message != tt.expectedMessage {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMessage, result.Message)
			}

			// Check error conditions
			if tt.mockArtistError != nil || tt.mockAddTracksError != nil {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else if !tt.expectedSuccess && !tt.expectedDuplicate {
				// Only expect error for certain failure cases
				if tt.name == "artist has no tracks" && err != nil {
					t.Error("Expected no error for 'no tracks' case")
				}
			}
		})
	}
}

// EnhancedMockSpotifyService provides more control over mock responses
type EnhancedMockSpotifyService struct {
	artist      *types.Artist
	artistError error
	tracks      []types.Track
	tracksError error
	addError    error
}

func (m *EnhancedMockSpotifyService) SearchArtist(query string) (*types.Artist, error) {
	return m.artist, m.artistError
}

func (m *EnhancedMockSpotifyService) GetArtistTopTracks(artistID string) ([]types.Track, error) {
	return m.tracks, m.tracksError
}

func (m *EnhancedMockSpotifyService) GetUserPlaylists(folderName string) ([]types.Playlist, error) {
	return nil, errors.New("not implemented in enhanced mock")
}

func (m *EnhancedMockSpotifyService) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	return m.addError
}

func (m *EnhancedMockSpotifyService) CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error) {
	return nil, errors.New("not implemented in enhanced mock")
}

func (m *EnhancedMockSpotifyService) GetAuthURL() string {
	return "mock-auth-url"
}

func (m *EnhancedMockSpotifyService) IsAuthenticated() bool {
	return true
}

func (m *EnhancedMockSpotifyService) CompleteAuth(code, state string) error {
	return nil
}

func (m *EnhancedMockSpotifyService) CreatePlaylist(name, description string, public bool) (*types.Playlist, error) {
	return &types.Playlist{
		ID:         "test-playlist-id",
		Name:       name,
		URI:        "spotify:playlist:test-playlist-id",
		TrackCount: 0,
		EmbedURL:   "https://open.spotify.com/embed/playlist/test-playlist-id",
		IsIncoming: false,
	}, nil
}

// EnhancedMockDuplicateDetector provides more control over duplicate detection
type EnhancedMockDuplicateDetector struct {
	result *types.DuplicateResult
	err    error
}

func (m *EnhancedMockDuplicateDetector) CheckDuplicates(playlistID string, tracks []types.Track) (*types.DuplicateResult, error) {
	return m.result, m.err
}

func (m *EnhancedMockDuplicateDetector) CheckArtistInPlaylist(playlistID, artistID string) (*types.DuplicateResult, error) {
	return m.result, m.err
}
func TestPlaylistService_AddArtistToPlaylist_RateLimiting(t *testing.T) {
	tests := []struct {
		name            string
		addTracksError  error
		expectedMessage string
	}{
		{
			name:            "rate limit error with 429",
			addTracksError:  errors.New("HTTP 429: Rate limit exceeded"),
			expectedMessage: "Rate limited by Spotify API. Please try again later.",
		},
		{
			name:            "rate limit error with text",
			addTracksError:  errors.New("Rate limit exceeded, please try again"),
			expectedMessage: "Rate limited by Spotify API. Please try again later.",
		},
		{
			name:            "other error",
			addTracksError:  errors.New("playlist not found"),
			expectedMessage: "Failed to add tracks to playlist: playlist not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSpotify := &EnhancedMockSpotifyService{
				artist: &types.Artist{
					ID:   "artist123",
					Name: "Test Artist",
					URI:  "spotify:artist:artist123",
				},
				artistError: nil,
				tracks: []types.Track{
					{ID: "track1", Name: "Song 1", URI: "spotify:track:track1"},
				},
				tracksError: nil,
				addError:    tt.addTracksError,
			}

			mockDuplicate := &EnhancedMockDuplicateDetector{
				result: &types.DuplicateResult{HasDuplicates: false},
				err:    nil,
			}

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			service := NewPlaylistService(mockSpotify, mockDuplicate, logger)

			result, err := service.AddArtistToPlaylist("Test Artist", "playlist123", false)

			if result == nil {
				t.Fatal("Expected result but got nil")
			}

			if result.Success {
				t.Error("Expected failure but got success")
			}

			if result.Message != tt.expectedMessage {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMessage, result.Message)
			}

			if err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

// TestPlaylistService_AddArtistToPlaylist_OverrideScenarios tests override functionality
func TestPlaylistService_AddArtistToPlaylist_OverrideScenarios(t *testing.T) {
	tests := []struct {
		name                string
		force               bool
		mockDuplicateResult *types.DuplicateResult
		mockDuplicateError  error
		expectedSuccess     bool
		expectedMessage     string
		shouldCheckDups     bool
	}{
		{
			name:  "force bypass with duplicates",
			force: true,
			mockDuplicateResult: &types.DuplicateResult{
				HasDuplicates: true,
				Message:       "Artist tracks already exist",
			},
			mockDuplicateError: nil,
			expectedSuccess:    true,
			expectedMessage:    "Successfully added Test Artist's top tracks to playlist",
			shouldCheckDups:    false, // Force should bypass duplicate check
		},
		{
			name:  "no force with duplicates blocks addition",
			force: false,
			mockDuplicateResult: &types.DuplicateResult{
				HasDuplicates: true,
				Message:       "Artist 'Test Artist' already has tracks in playlist. Use 'Add Anyway' to override.",
			},
			mockDuplicateError: nil,
			expectedSuccess:    false,
			expectedMessage:    "Artist 'Test Artist' already has tracks in playlist. Use 'Add Anyway' to override.",
			shouldCheckDups:    true,
		},
		{
			name:  "no force with no duplicates proceeds",
			force: false,
			mockDuplicateResult: &types.DuplicateResult{
				HasDuplicates: false,
				Message:       "No duplicates found",
			},
			mockDuplicateError: nil,
			expectedSuccess:    true,
			expectedMessage:    "Successfully added Test Artist's top tracks to playlist",
			shouldCheckDups:    true,
		},
		{
			name:                "force with duplicate check error proceeds anyway",
			force:               true,
			mockDuplicateResult: nil,
			mockDuplicateError:  errors.New("duplicate check failed"),
			expectedSuccess:     true,
			expectedMessage:     "Successfully added Test Artist's top tracks to playlist",
			shouldCheckDups:     false,
		},
		{
			name:                "no force with duplicate check error proceeds with warning",
			force:               false,
			mockDuplicateResult: nil,
			mockDuplicateError:  errors.New("duplicate check failed"),
			expectedSuccess:     true,
			expectedMessage:     "Successfully added Test Artist's top tracks to playlist",
			shouldCheckDups:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSpotify := &EnhancedMockSpotifyService{
				artist: &types.Artist{
					ID:   "artist123",
					Name: "Test Artist",
					URI:  "spotify:artist:artist123",
				},
				artistError: nil,
				tracks: []types.Track{
					{ID: "track1", Name: "Song 1", URI: "spotify:track:track1"},
					{ID: "track2", Name: "Song 2", URI: "spotify:track:track2"},
				},
				tracksError: nil,
				addError:    nil,
			}

			mockDuplicate := &EnhancedMockDuplicateDetector{
				result: tt.mockDuplicateResult,
				err:    tt.mockDuplicateError,
			}

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			service := NewPlaylistService(mockSpotify, mockDuplicate, logger)

			result, err := service.AddArtistToPlaylist("Test Artist", "playlist123", tt.force)

			if err != nil && tt.expectedSuccess {
				t.Errorf("Unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("Expected result but got nil")
			}

			if result.Success != tt.expectedSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectedSuccess, result.Success)
			}

			if result.Message != tt.expectedMessage {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMessage, result.Message)
			}

			// Verify that WasDuplicate is set correctly
			if !tt.force && tt.mockDuplicateResult != nil && tt.mockDuplicateResult.HasDuplicates && tt.mockDuplicateError == nil {
				if !result.WasDuplicate {
					t.Error("Expected WasDuplicate to be true when duplicates are detected without force")
				}
			}

			// For successful operations, verify tracks are included
			if result.Success {
				if len(result.TracksAdded) == 0 {
					t.Error("Expected tracks to be added for successful operations")
				}
				if result.Artist.Name != "Test Artist" {
					t.Errorf("Expected artist name 'Test Artist', got '%s'", result.Artist.Name)
				}
			}
		})
	}
}

// TestPlaylistService_OverrideButtonFunctionality tests the override button workflow
func TestPlaylistService_OverrideButtonFunctionality(t *testing.T) {
	// Simulate the workflow: first call without force (detects duplicates), second call with force (overrides)

	mockSpotify := &EnhancedMockSpotifyService{
		artist: &types.Artist{
			ID:   "artist123",
			Name: "Test Artist",
			URI:  "spotify:artist:artist123",
		},
		artistError: nil,
		tracks: []types.Track{
			{ID: "track1", Name: "Song 1", URI: "spotify:track:track1"},
		},
		tracksError: nil,
		addError:    nil,
	}

	mockDuplicate := &EnhancedMockDuplicateDetector{
		result: &types.DuplicateResult{
			HasDuplicates: true,
			Message:       "Artist 'Test Artist' already has tracks in playlist. Use 'Add Anyway' to override.",
		},
		err: nil,
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := NewPlaylistService(mockSpotify, mockDuplicate, logger)

	// First call without force - should detect duplicates and fail
	result1, err1 := service.AddArtistToPlaylist("Test Artist", "playlist123", false)

	if err1 != nil {
		t.Errorf("Unexpected error on first call: %v", err1)
	}
	if result1 == nil {
		t.Fatal("Expected result on first call but got nil")
	}
	if result1.Success {
		t.Error("Expected first call to fail due to duplicates")
	}
	if !result1.WasDuplicate {
		t.Error("Expected WasDuplicate to be true on first call")
	}

	// Second call with force - should succeed despite duplicates
	result2, err2 := service.AddArtistToPlaylist("Test Artist", "playlist123", true)

	if err2 != nil {
		t.Errorf("Unexpected error on second call: %v", err2)
	}
	if result2 == nil {
		t.Fatal("Expected result on second call but got nil")
	}
	if !result2.Success {
		t.Error("Expected second call to succeed with force")
	}
	if result2.WasDuplicate {
		t.Error("Expected WasDuplicate to be false on forced call")
	}
}

// TestPlaylistService_APIOverrideParameter tests API-style override with force parameter
func TestPlaylistService_APIOverrideParameter(t *testing.T) {
	tests := []struct {
		name        string
		forceParam  bool
		description string
	}{
		{
			name:        "API call with force=true",
			forceParam:  true,
			description: "API should bypass duplicate detection when force=true",
		},
		{
			name:        "API call with force=false",
			forceParam:  false,
			description: "API should respect duplicate detection when force=false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSpotify := &EnhancedMockSpotifyService{
				artist: &types.Artist{
					ID:   "artist123",
					Name: "API Test Artist",
					URI:  "spotify:artist:artist123",
				},
				artistError: nil,
				tracks: []types.Track{
					{ID: "track1", Name: "API Song 1", URI: "spotify:track:track1"},
				},
				tracksError: nil,
				addError:    nil,
			}

			mockDuplicate := &EnhancedMockDuplicateDetector{
				result: &types.DuplicateResult{
					HasDuplicates: true,
					Message:       "API duplicate detected",
				},
				err: nil,
			}

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			service := NewPlaylistService(mockSpotify, mockDuplicate, logger)

			result, err := service.AddArtistToPlaylist("API Test Artist", "playlist123", tt.forceParam)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("Expected result but got nil")
			}

			if tt.forceParam {
				// With force=true, should succeed despite duplicates
				if !result.Success {
					t.Error("Expected success with force=true")
				}
				if result.WasDuplicate {
					t.Error("Expected WasDuplicate to be false with force=true")
				}
			} else {
				// With force=false, should fail due to duplicates
				if result.Success {
					t.Error("Expected failure with force=false and duplicates")
				}
				if !result.WasDuplicate {
					t.Error("Expected WasDuplicate to be true with force=false and duplicates")
				}
			}
		})
	}
}

func TestPlaylistService_GetTop5Tracks(t *testing.T) {
	tests := []struct {
		name          string
		artistID      string
		mockTracks    []types.Track
		mockError     error
		expectError   bool
		expectedCount int
	}{
		{
			name:     "successful retrieval of top tracks",
			artistID: "artist123",
			mockTracks: []types.Track{
				{ID: "track1", Name: "Song 1", URI: "spotify:track:track1"},
				{ID: "track2", Name: "Song 2", URI: "spotify:track:track2"},
				{ID: "track3", Name: "Song 3", URI: "spotify:track:track3"},
			},
			mockError:     nil,
			expectError:   false,
			expectedCount: 3,
		},
		{
			name:          "no tracks available",
			artistID:      "artist456",
			mockTracks:    []types.Track{},
			mockError:     nil,
			expectError:   false,
			expectedCount: 0,
		},
		{
			name:          "spotify service error",
			artistID:      "artist789",
			mockTracks:    nil,
			mockError:     errors.New("spotify API error"),
			expectError:   true,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup enhanced mock
			mockSpotify := &EnhancedMockSpotifyService{
				tracks:      tt.mockTracks,
				tracksError: tt.mockError,
			}

			mockDuplicate := &EnhancedMockDuplicateDetector{}
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel) // Reduce log noise in tests

			service := NewPlaylistService(mockSpotify, mockDuplicate, logger)

			// Execute
			result, err := service.GetTop5Tracks(tt.artistID)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)
				assert.Equal(t, tt.mockTracks, result)
			}
		})
	}
}
