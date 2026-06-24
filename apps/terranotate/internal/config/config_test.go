package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnvVars(t *testing.T) {
	tests := []struct {
		name        string
		mockEnvFile string
	}{
		{
			name:        "Valid .env file",
			mockEnvFile: "TERRAFORM_VERSION=1.5.0\n",
		},
		{
			name: "No environment variables or .env file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original directory and change to temp directory
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current directory: %v", err)
			}

			tmpDir := t.TempDir()
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("Failed to change to temp directory: %v", err)
			}
			defer func() {
				if err := os.Chdir(originalDir); err != nil {
					t.Errorf("Failed to restore original directory: %v", err)
				}
			}()

			// Create .env file if applicable
			if tt.mockEnvFile != "" {
				envPath := filepath.Join(tmpDir, ".env")
				if err := os.WriteFile(envPath, []byte(tt.mockEnvFile), 0644); err != nil {
					t.Fatalf("Failed to write mock .env file: %v", err)
				}
			}

			// Call function - we just want to ensure it doesn't panic/exit
			cfg := GetEnvVars()
			if tt.mockEnvFile != "" && cfg.TerraformVersion != "1.5.0" {
				t.Errorf("Expected TerraformVersion to be '1.5.0', got '%s'", cfg.TerraformVersion)
			}
		})
	}
}
