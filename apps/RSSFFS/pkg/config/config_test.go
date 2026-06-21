package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnvVars(t *testing.T) {
	tests := []struct {
		name                string
		mockEnv             map[string]string
		mockEnvFile         string
		expectError         bool
		expectRSSEndpoint   string
		expectRSSAPIKey     string
		expectWebHost       string
		expectWebPort       int
		expectSingleURLMode bool
		expectExitCall      bool
	}{
		{
			name: "Valid environment variables",
			mockEnv: map[string]string{
				"RSS_READER_ENDPOINT": "https://miniflux.example.com",
				"RSS_READER_API_KEY":  "test-api-key",
			},
			expectError:         false,
			expectRSSEndpoint:   "https://miniflux.example.com",
			expectRSSAPIKey:     "test-api-key",
			expectWebHost:       "127.0.0.1", // default value
			expectWebPort:       8080,        // default value
			expectSingleURLMode: false,       // default value
		},
		{
			name:                "Valid .env file",
			mockEnvFile:         "RSS_READER_ENDPOINT=https://miniflux.example.com\nRSS_READER_API_KEY=test-env-file-key\n",
			expectError:         false,
			expectRSSEndpoint:   "https://miniflux.example.com",
			expectRSSAPIKey:     "test-env-file-key",
			expectWebHost:       "127.0.0.1", // default value
			expectWebPort:       8080,        // default value
			expectSingleURLMode: false,       // default value
		},
		{
			name: "Environment variable overrides .env file",
			mockEnv: map[string]string{
				"RSS_READER_ENDPOINT": "https://env.example.com",
				"RSS_READER_API_KEY":  "env-api-key",
			},
			mockEnvFile:         "RSS_READER_ENDPOINT=https://file.example.com\nRSS_READER_API_KEY=file-api-key\n",
			expectError:         false,
			expectRSSEndpoint:   "https://env.example.com",
			expectRSSAPIKey:     "env-api-key",
			expectWebHost:       "127.0.0.1", // default value
			expectWebPort:       8080,        // default value
			expectSingleURLMode: false,       // default value
		},
		{
			name: "Web server configuration from environment variables",
			mockEnv: map[string]string{
				"RSS_READER_ENDPOINT": "https://miniflux.example.com",
				"RSS_READER_API_KEY":  "test-api-key",
				"WEB_HOST":            "0.0.0.0",
				"WEB_PORT":            "9090",
			},
			expectError:         false,
			expectRSSEndpoint:   "https://miniflux.example.com",
			expectRSSAPIKey:     "test-api-key",
			expectWebHost:       "0.0.0.0",
			expectWebPort:       9090,
			expectSingleURLMode: false, // default value
		},
		{
			name: "Single URL mode enabled via environment variable",
			mockEnv: map[string]string{
				"RSS_READER_ENDPOINT":    "https://miniflux.example.com",
				"RSS_READER_API_KEY":     "test-api-key",
				"RSSFFS_SINGLE_URL_MODE": "true",
			},
			expectError:         false,
			expectRSSEndpoint:   "https://miniflux.example.com",
			expectRSSAPIKey:     "test-api-key",
			expectWebHost:       "127.0.0.1", // default value
			expectWebPort:       8080,        // default value
			expectSingleURLMode: true,
		},
		{
			name: "Single URL mode disabled via environment variable",
			mockEnv: map[string]string{
				"RSS_READER_ENDPOINT":    "https://miniflux.example.com",
				"RSS_READER_API_KEY":     "test-api-key",
				"RSSFFS_SINGLE_URL_MODE": "false",
			},
			expectError:         false,
			expectRSSEndpoint:   "https://miniflux.example.com",
			expectRSSAPIKey:     "test-api-key",
			expectWebHost:       "127.0.0.1", // default value
			expectWebPort:       8080,        // default value
			expectSingleURLMode: false,
		},
		{
			name:                "Single URL mode from .env file",
			mockEnvFile:         "RSS_READER_ENDPOINT=https://miniflux.example.com\nRSS_READER_API_KEY=test-env-file-key\nRSSFFS_SINGLE_URL_MODE=true\n",
			expectError:         false,
			expectRSSEndpoint:   "https://miniflux.example.com",
			expectRSSAPIKey:     "test-env-file-key",
			expectWebHost:       "127.0.0.1", // default value
			expectWebPort:       8080,        // default value
			expectSingleURLMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original directory and change to temp directory
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current directory: %v", err)
			}

			// Save original environment variables
			originalEndpoint := os.Getenv("RSS_READER_ENDPOINT")
			originalAPIKey := os.Getenv("RSS_READER_API_KEY")
			originalWebHost := os.Getenv("WEB_HOST")
			originalWebPort := os.Getenv("WEB_PORT")
			originalSingleURLMode := os.Getenv("RSSFFS_SINGLE_URL_MODE")
			defer func() {
				if originalEndpoint != "" {
					_ = os.Setenv("RSS_READER_ENDPOINT", originalEndpoint)
				} else {
					_ = os.Unsetenv("RSS_READER_ENDPOINT")
				}
				if originalAPIKey != "" {
					_ = os.Setenv("RSS_READER_API_KEY", originalAPIKey)
				} else {
					_ = os.Unsetenv("RSS_READER_API_KEY")
				}
				if originalWebHost != "" {
					_ = os.Setenv("WEB_HOST", originalWebHost)
				} else {
					_ = os.Unsetenv("WEB_HOST")
				}
				if originalWebPort != "" {
					_ = os.Setenv("WEB_PORT", originalWebPort)
				} else {
					_ = os.Unsetenv("WEB_PORT")
				}
				if originalSingleURLMode != "" {
					_ = os.Setenv("RSSFFS_SINGLE_URL_MODE", originalSingleURLMode)
				} else {
					_ = os.Unsetenv("RSSFFS_SINGLE_URL_MODE")
				}
			}()

			tmpDir := t.TempDir()
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("Failed to change to temp directory: %v", err)
			}
			defer func() {
				if err := os.Chdir(originalDir); err != nil {
					t.Errorf("Failed to restore original directory: %v", err)
				}
			}()

			// Clear environment variables first
			_ = os.Unsetenv("RSS_READER_ENDPOINT")
			_ = os.Unsetenv("RSS_READER_API_KEY")
			_ = os.Unsetenv("WEB_HOST")
			_ = os.Unsetenv("WEB_PORT")
			_ = os.Unsetenv("RSSFFS_SINGLE_URL_MODE")

			// Create .env file if applicable
			if tt.mockEnvFile != "" {
				envPath := filepath.Join(tmpDir, ".env")
				if err := os.WriteFile(envPath, []byte(tt.mockEnvFile), 0644); err != nil {
					t.Fatalf("Failed to write mock .env file: %v", err)
				}
			}

			// Set mock environment variables (these should override .env file)
			for key, value := range tt.mockEnv {
				_ = os.Setenv(key, value)
			}

			// Call function - only test cases that shouldn't exit
			if !tt.expectExitCall {
				conf := GetEnvVars()

				// Verify output
				if conf.RSSReaderEndpoint != tt.expectRSSEndpoint {
					t.Errorf("expected RSS endpoint %q, got %q", tt.expectRSSEndpoint, conf.RSSReaderEndpoint)
				}
				if conf.RSSReaderAPIKey != tt.expectRSSAPIKey {
					t.Errorf("expected RSS API key %q, got %q", tt.expectRSSAPIKey, conf.RSSReaderAPIKey)
				}
				if conf.WebHost != tt.expectWebHost {
					t.Errorf("expected web host %q, got %q", tt.expectWebHost, conf.WebHost)
				}
				if conf.WebPort != tt.expectWebPort {
					t.Errorf("expected web port %d, got %d", tt.expectWebPort, conf.WebPort)
				}
				if conf.SingleURLMode != tt.expectSingleURLMode {
					t.Errorf("expected single URL mode %t, got %t", tt.expectSingleURLMode, conf.SingleURLMode)
				}
			}
		})
	}
}
