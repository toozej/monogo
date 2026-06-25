package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnvVars(t *testing.T) {
	tests := []struct {
		name           string
		mockEnv        map[string]string
		mockEnvFile    string
		expectError    bool
		expectUsername string
	}{
		{
			name:           "Valid environment variable",
			expectUsername: "testuser",
			expectError:    false,
			mockEnv: map[string]string{
				"USERNAME": "testuser",
			},
		},
		{
			name:           "Valid .env file",
			expectUsername: "testenvfileuser",
			expectError:    false,
			mockEnvFile:    "USERNAME=testenvfileuser\n",
		},
		{
			name:           "No environment variables or .env file",
			expectUsername: "",
			expectError:    false,
		},
		{
			name:           "Environment variable overrides .env file",
			expectUsername: "envuser",
			expectError:    false,
			mockEnvFile:    "USERNAME=fileuser\n",
			mockEnv: map[string]string{
				"USERNAME": "envuser",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original directory and change to temp directory
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current directory: %v", err)
			}

			// Save original USERNAME environment variable
			originalUsername := os.Getenv("USERNAME")
			defer func() {
				if originalUsername != "" {
					_ = os.Setenv("USERNAME", originalUsername)
				} else {
					_ = os.Unsetenv("USERNAME")
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

			// Clear USERNAME environment variable first
			_ = os.Unsetenv("USERNAME")

			// Create .env file if applicable
			if tt.mockEnvFile != "" {
				envPath := filepath.Join(tmpDir, ".env")
				if err := os.WriteFile(envPath, []byte(tt.mockEnvFile), 0o644); err != nil {
					t.Fatalf("Failed to write mock .env file: %v", err)
				}
			}

			// Set mock environment variables (these should override .env file)
			for key, value := range tt.mockEnv {
				_ = os.Setenv(key, value)
			}

			// Call function
			conf := GetEnvVars()

			// Verify output
			if conf.Username != tt.expectUsername {
				t.Errorf("expected username %q, got %q", tt.expectUsername, conf.Username)
			}
		})
	}
}
