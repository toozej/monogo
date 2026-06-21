package spotify

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/pkg/config"
	"golang.org/x/oauth2"
)

func TestNewClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	tests := []struct {
		name      string
		config    config.SpotifyConfig
		wantErr   bool
		errString string
	}{
		{
			name: "missing client ID",
			config: config.SpotifyConfig{
				ClientID:     "",
				ClientSecret: "test-secret",
			},
			wantErr:   true,
			errString: "spotify client ID and secret are required",
		},
		{
			name: "missing client secret",
			config: config.SpotifyConfig{
				ClientID:     "test-id",
				ClientSecret: "",
			},
			wantErr:   true,
			errString: "spotify client ID and secret are required",
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
			client, err := NewClient(tt.config, logger)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewClient() expected error but got none")
					return
				}
				if tt.errString != "" && err.Error() != tt.errString {
					t.Errorf("NewClient() error = %v, want %v", err.Error(), tt.errString)
				}
				return
			}

			if err != nil {
				t.Errorf("NewClient() unexpected error = %v", err)
				return
			}

			if client == nil {
				t.Error("NewClient() returned nil client")
			}
		})
	}
}

func TestClient_RefreshToken(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test with invalid credentials to test error handling
	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	client := &Client{
		config: cfg,
		logger: logger,
		token:  nil, // No token, should trigger refresh
		ctx:    context.Background(),
	}

	err := client.RefreshToken()
	if err == nil {
		t.Error("RefreshToken() expected error with invalid credentials but got none")
	}
}

func TestClient_RefreshToken_NotNeeded(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	// Create a token that doesn't need refresh (expires in 1 hour)
	futureTime := time.Now().Add(1 * time.Hour)

	client := &Client{
		config: cfg,
		logger: logger,
		token: &oauth2.Token{
			AccessToken: "test-token",
			TokenType:   "Bearer",
			Expiry:      futureTime,
		},
		ctx:        context.Background(),
		isUserAuth: true, // Set to true to simulate authenticated state
	}

	err := client.RefreshToken()
	if err != nil {
		t.Errorf("RefreshToken() unexpected error when token doesn't need refresh: %v", err)
	}
}

func TestClient_SearchArtist_NoToken(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	client := &Client{
		config: cfg,
		logger: logger,
		token:  nil,
		ctx:    context.Background(),
	}

	_, err := client.SearchArtist("test artist")
	if err == nil {
		t.Error("SearchArtist() expected error when no valid token but got none")
	}
}

func TestClient_GetArtistTopTracks_NoToken(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	client := &Client{
		config: cfg,
		logger: logger,
		token:  nil,
		ctx:    context.Background(),
	}

	_, err := client.GetArtistTopTracks("test-artist-id")
	if err == nil {
		t.Error("GetArtistTopTracks() expected error when no valid token but got none")
	}
}

func TestClient_GetUserPlaylists_NoToken(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	client := &Client{
		config: cfg,
		logger: logger,
		token:  nil,
		ctx:    context.Background(),
	}

	_, err := client.GetUserPlaylists("Incoming")
	if err == nil {
		t.Error("GetUserPlaylists() expected error when no valid token but got none")
	}
}

func TestClient_AddTracksToPlaylist_NoTracks(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	client := &Client{
		config: cfg,
		logger: logger,
		ctx:    context.Background(),
	}

	err := client.AddTracksToPlaylist("test-playlist-id", []string{})
	if err == nil {
		t.Error("AddTracksToPlaylist() expected error when no tracks provided but got none")
	}

	expectedErr := "no tracks provided to add"
	if err.Error() != expectedErr {
		t.Errorf("AddTracksToPlaylist() error = %v, want %v", err.Error(), expectedErr)
	}
}

func TestClient_CheckTracksInPlaylist_NoTracks(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	client := &Client{
		config: cfg,
		logger: logger,
		ctx:    context.Background(),
	}

	results, err := client.CheckTracksInPlaylist("test-playlist-id", []string{})
	if err != nil {
		t.Errorf("CheckTracksInPlaylist() unexpected error with empty tracks: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("CheckTracksInPlaylist() expected empty results but got %v", results)
	}
}
