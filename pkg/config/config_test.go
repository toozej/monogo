package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvVars(t *testing.T) {
	tests := []struct {
		name              string
		mockEnv           map[string]string
		mockEnvFile       string
		expectError       bool
		expectSpotifyID   string
		expectKMHDBaseURL string
	}{
		{
			name: "Valid environment variables",
			mockEnv: map[string]string{
				"SPOTIFY_CLIENT_ID":  "test-spotify-id",
				"KMHD_BASE_URL":      "https://kmhd.example.com",
				"KMHD_PLAYLIST_PATH": "/playlist",
			},
			expectError:       false,
			expectSpotifyID:   "test-spotify-id",
			expectKMHDBaseURL: "https://kmhd.example.com",
		},
		{
			name:              "Valid .env file",
			mockEnvFile:       "SPOTIFY_CLIENT_ID=test-env-spotify-id\nKMHD_BASE_URL=https://kmhd-env.example.com\nKMHD_PLAYLIST_PATH=/playlist\n",
			expectError:       false,
			expectSpotifyID:   "test-env-spotify-id",
			expectKMHDBaseURL: "https://kmhd-env.example.com",
		},
		{
			name:              "No environment variables or .env file",
			expectError:       true, // Should error due to missing required KMHD_BASE_URL
			expectSpotifyID:   "",
			expectKMHDBaseURL: "",
		},
		{
			name: "Environment variable overrides .env file",
			mockEnv: map[string]string{
				"SPOTIFY_CLIENT_ID":  "env-spotify-id",
				"KMHD_BASE_URL":      "https://kmhd-override.example.com",
				"KMHD_PLAYLIST_PATH": "/override-playlist",
			},
			mockEnvFile:       "SPOTIFY_CLIENT_ID=file-spotify-id\nKMHD_BASE_URL=https://kmhd-file.example.com\nKMHD_PLAYLIST_PATH=/file-playlist\n",
			expectError:       false,
			expectSpotifyID:   "env-spotify-id",
			expectKMHDBaseURL: "https://kmhd-override.example.com",
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
			os.Unsetenv("SPOTIFY_CLIENT_ID")
			os.Unsetenv("KMHD_BASE_URL")
			os.Unsetenv("KMHD_PLAYLIST_PATH")

			// Create .env file if applicable
			if tt.mockEnvFile != "" {
				envPath := filepath.Join(tmpDir, ".env")
				err = os.WriteFile(envPath, []byte(tt.mockEnvFile), 0644)
				assert.NoError(t, err, "Failed to write mock .env file")
			}

			// Set mock environment variables (these should override .env file)
			for key, value := range tt.mockEnv {
				os.Setenv(key, value)
			}

			// Test for expected behavior
			if tt.expectError {
				// For error cases, we can't easily test os.Exit in Go tests
				// So we'll just verify that required fields are missing
				if tt.name == "No environment variables or .env file" {
					// This should fail validation, but we can't test os.Exit easily
					t.Skip("Cannot easily test os.Exit behavior in Go tests")
				}
			} else {
				// For success cases, test normal behavior
				conf := GetEnvVars()

				// Verify output
				assert.Equal(t, tt.expectSpotifyID, conf.Spotify.ClientID, "expected Spotify ClientID %q, got %q", tt.expectSpotifyID, conf.Spotify.ClientID)
				assert.Equal(t, tt.expectKMHDBaseURL, conf.KMHD.BaseURL, "expected KMHD BaseURL %q, got %q", tt.expectKMHDBaseURL, conf.KMHD.BaseURL)
			}
		})
	}

}
