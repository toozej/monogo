package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvVars(t *testing.T) {
	tests := []struct {
		name            string
		mockEnv         map[string]string
		mockEnvFile     string
		expectError     bool
		expectSpotifyID string
	}{
		{
			name: "Valid environment variables",
			mockEnv: map[string]string{
				"SPOTIFY_CLIENT_ID":     "test-spotify-id",
				"SPOTIFY_CLIENT_SECRET": "test-spotify-secret",
				"SPOTIFY_REDIRECT_URI":  "http://localhost:8080/callback",
			},
			expectError:     false,
			expectSpotifyID: "test-spotify-id",
		},
		{
			name:            "Valid .env file",
			mockEnvFile:     "SPOTIFY_CLIENT_ID=test-env-spotify-id\nSPOTIFY_CLIENT_SECRET=test-spotify-secret\nSPOTIFY_REDIRECT_URI=http://localhost:8080/callback\n",
			expectError:     false,
			expectSpotifyID: "test-env-spotify-id",
		},
		{
			name: "Environment variable overrides .env file",
			mockEnv: map[string]string{
				"SPOTIFY_CLIENT_ID":     "env-spotify-id",
				"SPOTIFY_CLIENT_SECRET": "test-spotify-secret",
				"SPOTIFY_REDIRECT_URI":  "http://localhost:8080/callback",
			},
			mockEnvFile:     "SPOTIFY_CLIENT_ID=file-spotify-id\nSPOTIFY_CLIENT_SECRET=file-spotify-secret\nSPOTIFY_REDIRECT_URI=http://localhost:8080/callback\n",
			expectError:     false,
			expectSpotifyID: "env-spotify-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original directory and change to temp directory
			originalDir, err := os.Getwd()
			assert.NoError(t, err, "Failed to get current directory")

			tmpDir := t.TempDir()
			err = os.Chdir(tmpDir)
			assert.NoError(t, err, "Failed to change to temp directory")
			defer func() {
				chdirErr := os.Chdir(originalDir)
				assert.NoError(t, chdirErr, "Failed to restore original directory")
			}()

			// Clear environment variables first
			_ = os.Unsetenv("SPOTIFY_CLIENT_ID")
			_ = os.Unsetenv("SPOTIFY_CLIENT_SECRET")
			_ = os.Unsetenv("SPOTIFY_REDIRECT_URI")
			_ = os.Unsetenv("MUSIC_CLIENT")
			_ = os.Unsetenv("YOUTUBEMUSIC_COOKIE")

			// Create .env file if applicable
			if tt.mockEnvFile != "" {
				envPath := filepath.Join(tmpDir, ".env")
				err = os.WriteFile(envPath, []byte(tt.mockEnvFile), 0644)
				assert.NoError(t, err, "Failed to write mock .env file")
			}

			// Set mock environment variables (these should override .env file)
			for key, value := range tt.mockEnv {
				_ = os.Setenv(key, value)
			}

			// Test for expected behavior
			if tt.expectError {
				// For error cases, we can't easily test os.Exit in Go tests
				// So we'll just verify that required fields are missing
				t.Skip("Cannot easily test os.Exit behavior in Go tests")
			} else {
				// For success cases, test normal behavior
				conf := GetEnvVars()

				// Verify output
				assert.Equal(t, tt.expectSpotifyID, conf.Spotify.ClientID, "expected Spotify ClientID %q, got %q", tt.expectSpotifyID, conf.Spotify.ClientID)
				// Verify KMHD config has default values
				assert.Equal(t, "https://www.kmhd.org/pf/api/v3/content/fetch/playlist", conf.KMHD.APIEndpoint)
				assert.Equal(t, 30, conf.KMHD.HTTPTimeout)
				// Verify Spotify config has default token file path
				assert.Equal(t, "~/.config/kmhd2playlist/spotify_token.json", conf.Spotify.TokenFilePath)
			}
		})
	}

}

func TestValidateConfigRequiresOnlySelectedProvider(t *testing.T) {
	validConfig := func() Config {
		return Config{
			KMHD: KMHDConfig{
				APIEndpoint: "https://example.com/playlist",
				HTTPTimeout: 30,
			},
			Server: ServerConfig{Port: 8080},
		}
	}

	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "Spotify ignores YouTube Music cookie",
			config: func() Config {
				conf := validConfig()
				conf.MusicClient = "spotify"
				conf.Spotify = SpotifyConfig{
					ClientID:     "spotify-client-id",
					ClientSecret: "spotify-client-secret",
					RedirectURL:  "http://localhost:8080/callback",
				}
				return conf
			}(),
		},
		{
			name: "YouTube Music ignores Spotify configuration",
			config: func() Config {
				conf := validConfig()
				conf.MusicClient = "youtube"
				conf.YouTubeMusic.Cookie = "SAPISID=example"
				return conf
			}(),
		},
		{
			name: "Spotify requires Spotify credentials and redirect URI",
			config: func() Config {
				conf := validConfig()
				conf.MusicClient = "spotify"
				return conf
			}(),
			wantErr: "SPOTIFY_CLIENT_ID is required when MUSIC_CLIENT=spotify",
		},
		{
			name: "YouTube Music requires auth headers or a legacy cookie",
			config: func() Config {
				conf := validConfig()
				conf.MusicClient = "youtube"
				return conf
			}(),
			wantErr: "YOUTUBEMUSIC_AUTH_FILE_PATH or YOUTUBEMUSIC_COOKIE is required when MUSIC_CLIENT=youtube",
		},
		{
			name: "YouTube Music accepts browser auth headers without a cookie environment variable",
			config: func() Config {
				conf := validConfig()
				conf.MusicClient = "youtube"
				conf.YouTubeMusic.AuthFilePath = "/app/auth/youtubemusic-headers.json"
				return conf
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(&tt.config)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestSpotifyConfig_GetTokenFilePath(t *testing.T) {
	tests := []struct {
		name           string
		tokenFilePath  string
		expectError    bool
		expectContains string
	}{
		{
			name:           "Default path with tilde expansion",
			tokenFilePath:  "~/.config/kmhd2playlist/spotify_token.json",
			expectError:    false,
			expectContains: ".config/kmhd2playlist/spotify_token.json",
		},
		{
			name:           "Absolute path",
			tokenFilePath:  "/tmp/test_token.json",
			expectError:    false,
			expectContains: "/tmp/test_token.json",
		},
		{
			name:           "Relative path",
			tokenFilePath:  "data/token.json",
			expectError:    false,
			expectContains: "data/token.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SpotifyConfig{
				TokenFilePath: tt.tokenFilePath,
			}

			result, err := config.GetTokenFilePath()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, result, tt.expectContains)

				// Verify the directory was created
				dir := filepath.Dir(result)
				_, err := os.Stat(dir)
				assert.NoError(t, err, "Token directory should be created")
			}
		})
	}
}
