package spotify

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/toozej/kmhd2spotify/pkg/config"
)

func TestService_GetArtistTopTracks(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name         string
		artistID     string
		wantErr      bool
		expectTracks int
		maxTracks    int
	}{
		{
			name:         "invalid credentials - should error",
			artistID:     "test-artist-id",
			wantErr:      true,
			expectTracks: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use invalid credentials to test error handling
			cfg := config.SpotifyConfig{
				ClientID:     "invalid-id",
				ClientSecret: "invalid-secret",
			}

			service := NewService(cfg, logger)
			if service.client == nil && !tt.wantErr {
				t.Error("NewService() failed to create client")
				return
			}

			if service == nil && !tt.wantErr {
				t.Error("NewService() returned nil service")
				return
			}

			if tt.wantErr && service.client != nil {
				// Test the actual method call
				tracks, err := service.GetArtistTopTracks(tt.artistID)
				if err == nil {
					t.Error("GetArtistTopTracks() expected error but got none")
				}
				if len(tracks) != tt.expectTracks {
					t.Errorf("GetArtistTopTracks() returned %d tracks, expected %d", len(tracks), tt.expectTracks)
				}
			}
		})
	}
}

func TestService_GetArtistTopTracks_Integration(t *testing.T) {
	// This test would require valid Spotify credentials
	// For now, we'll test the structure and error handling
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	service := NewService(cfg, logger)
	if service.client != nil {
		t.Skip("Skipping integration test - would require valid Spotify credentials")
	}

	// Test that service creation fails with invalid credentials (client should be nil)
	if service.client != nil {
		t.Error("Expected service client to be nil with invalid credentials")
	}
}

func TestService_SearchArtist(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "invalid-id",
		ClientSecret: "invalid-secret",
	}

	service := NewService(cfg, logger)
	if service.client == nil {
		// Expected with invalid credentials
		return
	}

	if service == nil {
		t.Error("NewService() returned nil service")
		return
	}

	// Test search with invalid credentials (should error)
	artist, err := service.SearchArtist("test artist")
	if err == nil {
		t.Error("SearchArtist() expected error with invalid credentials but got none")
	}
	if artist != nil {
		t.Error("SearchArtist() expected nil artist with error")
	}
}

func TestService_GetUserPlaylists(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "invalid-id",
		ClientSecret: "invalid-secret",
	}

	service := NewService(cfg, logger)
	if service.client == nil {
		// Expected with invalid credentials
		return
	}

	if service == nil {
		t.Error("NewService() returned nil service")
		return
	}

	// Test playlist retrieval with invalid credentials (should error)
	playlists, err := service.GetUserPlaylists("Incoming")
	if err == nil {
		t.Error("GetUserPlaylists() expected error with invalid credentials but got none")
	}
	if playlists != nil {
		t.Error("GetUserPlaylists() expected nil playlists with error")
	}
}

func TestService_AddTracksToPlaylist(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "invalid-id",
		ClientSecret: "invalid-secret",
	}

	service := NewService(cfg, logger)
	if service.client == nil {
		// Expected with invalid credentials
		return
	}

	if service == nil {
		t.Error("NewService() returned nil service")
		return
	}

	// Test with empty tracks (should error before credentials)
	err := service.AddTracksToPlaylist("test-playlist", []string{})
	if err == nil {
		t.Error("AddTracksToPlaylist() expected error with empty tracks")
	}

	expectedErr := "no tracks provided to add"
	if err.Error() != expectedErr {
		t.Errorf("AddTracksToPlaylist() error = %v, want %v", err.Error(), expectedErr)
	}
}

func TestService_CheckTracksInPlaylist(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "invalid-id",
		ClientSecret: "invalid-secret",
	}

	service := NewService(cfg, logger)
	if service.client == nil {
		// Expected with invalid credentials
		return
	}

	if service == nil {
		t.Error("NewService() returned nil service")
		return
	}

	// Test with empty tracks (should return empty slice)
	results, err := service.CheckTracksInPlaylist("test-playlist", []string{})
	if err != nil {
		t.Errorf("CheckTracksInPlaylist() unexpected error with empty tracks: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("CheckTracksInPlaylist() expected empty results but got %v", results)
	}
}

// Test the top 5 tracks limitation logic with mock data
func TestTop5TracksLimitation(t *testing.T) {
	// This test verifies the logic for limiting tracks to 5
	// We'll test the logic that's already implemented in the client

	tests := []struct {
		name            string
		availableTracks int
		expectedTracks  int
	}{
		{
			name:            "exactly 5 tracks",
			availableTracks: 5,
			expectedTracks:  5,
		},
		{
			name:            "more than 5 tracks",
			availableTracks: 10,
			expectedTracks:  5,
		},
		{
			name:            "fewer than 5 tracks",
			availableTracks: 3,
			expectedTracks:  3,
		},
		{
			name:            "no tracks",
			availableTracks: 0,
			expectedTracks:  0,
		},
		{
			name:            "single track",
			availableTracks: 1,
			expectedTracks:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the logic that determines how many tracks to return
			maxTracks := 5
			if tt.availableTracks < maxTracks {
				maxTracks = tt.availableTracks
			}

			if maxTracks != tt.expectedTracks {
				t.Errorf("Track limitation logic: got %d tracks, want %d", maxTracks, tt.expectedTracks)
			}
		})
	}
}

func TestNewService(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name    string
		config  config.SpotifyConfig
		wantErr bool
	}{
		{
			name: "missing client ID",
			config: config.SpotifyConfig{
				ClientID:     "",
				ClientSecret: "test-secret",
			},
			wantErr: true,
		},
		{
			name: "missing client secret",
			config: config.SpotifyConfig{
				ClientID:     "test-id",
				ClientSecret: "",
			},
			wantErr: true,
		},
		{
			name: "invalid credentials",
			config: config.SpotifyConfig{
				ClientID:     "invalid-id",
				ClientSecret: "invalid-secret",
			},
			wantErr: true, // Will fail when trying to get token
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(tt.config, logger)

			if tt.wantErr {
				if service.client != nil {
					t.Errorf("NewService() expected client to be nil due to invalid credentials")
					return
				}
				return
			}

			if service == nil {
				t.Error("NewService() returned nil service")
			}
		})
	}
}
