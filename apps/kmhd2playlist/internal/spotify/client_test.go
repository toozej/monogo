package spotify

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/config"
)

func TestOAuthStateIsRandomAndExpires(t *testing.T) {
	newTestClient := func() *Client {
		client, err := NewClient(config.SpotifyConfig{
			ClientID:      "client-id",
			ClientSecret:  "client-secret",
			RedirectURL:   "http://127.0.0.1:8080/callback",
			TokenFilePath: filepath.Join(t.TempDir(), "spotify_token.json"),
		}, logrus.New())
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}
		return client
	}

	first := newTestClient()
	second := newTestClient()
	if first.state == second.state {
		t.Fatal("OAuth state must be unique per authentication attempt")
	}
	if err := first.CompleteAuth("code", "wrong-state"); err == nil {
		t.Fatal("CompleteAuth() accepted an incorrect OAuth state")
	}

	first.stateTime = time.Now().Add(-11 * time.Minute)
	if err := first.CompleteAuth("code", first.state); err == nil {
		t.Fatal("CompleteAuth() accepted an expired OAuth state")
	}
}

func TestSpotifyConfigGetTokenFilePath(t *testing.T) {
	tests := []struct {
		name           string
		tokenFilePath  string
		expectError    bool
		expectContains string
	}{
		{
			name:           "custom absolute path",
			tokenFilePath:  "/tmp/test/spotify_token.json",
			expectError:    false,
			expectContains: "/tmp/test/spotify_token.json",
		},
		{
			name:           "default path with tilde",
			tokenFilePath:  "~/.config/kmhd2playlist/spotify_token.json",
			expectError:    false,
			expectContains: ".config/kmhd2playlist/spotify_token.json",
		},
		{
			name:           "relative path",
			tokenFilePath:  "data/spotify_token.json",
			expectError:    false,
			expectContains: "data/spotify_token.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.SpotifyConfig{
				TokenFilePath: tt.tokenFilePath,
			}

			result, err := cfg.GetTokenFilePath()

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !tt.expectError {
				// Check path contains expected substring
				if tt.expectContains != "" && !contains(result, tt.expectContains) {
					t.Errorf("expected path to contain %s, got %s", tt.expectContains, result)
				}

				// Check that the path is absolute
				if !filepath.IsAbs(result) {
					t.Errorf("expected absolute path, got %s", result)
				}

				// Check that filename is correct
				if filepath.Base(result) != "spotify_token.json" {
					t.Errorf("expected filename spotify_token.json, got %s", filepath.Base(result))
				}
			}
		})
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(substr) > 0 && findSubstring(s, substr)))
}

// findSubstring finds if substr exists in s
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
