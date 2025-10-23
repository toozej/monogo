package spotify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetTokenFilePath(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expectError bool
		expectPath  string
	}{
		{
			name:        "custom path via environment variable",
			envValue:    "/tmp/test/spotify_token.json",
			expectError: false,
			expectPath:  "/tmp/test/spotify_token.json",
		},
		{
			name:        "default path when no env var set",
			envValue:    "",
			expectError: false,
			expectPath:  "", // Will be set to actual default path in test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing env var
			originalEnv := os.Getenv("SPOTIFY_TOKEN_FILE_PATH")
			defer func() {
				if originalEnv != "" {
					os.Setenv("SPOTIFY_TOKEN_FILE_PATH", originalEnv)
				} else {
					os.Unsetenv("SPOTIFY_TOKEN_FILE_PATH")
				}
			}()

			// Set test environment
			if tt.envValue != "" {
				os.Setenv("SPOTIFY_TOKEN_FILE_PATH", tt.envValue)
			} else {
				os.Unsetenv("SPOTIFY_TOKEN_FILE_PATH")
			}

			// Call function
			result, err := getTokenFilePath()

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check path expectation
			if tt.envValue != "" {
				if result != tt.expectPath {
					t.Errorf("expected path %s, got %s", tt.expectPath, result)
				}
			} else {
				// For default path, just check it contains the expected filename
				if !filepath.IsAbs(result) {
					t.Errorf("expected absolute path, got %s", result)
				}
				if filepath.Base(result) != "spotify_token.json" {
					t.Errorf("expected filename spotify_token.json, got %s", filepath.Base(result))
				}
			}

			// Clean up test directory if created
			if tt.envValue != "" {
				os.RemoveAll(filepath.Dir(tt.envValue))
			}
		})
	}
}
