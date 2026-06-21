package config

import (
	"os"
	"testing"
)

func TestConfigIntegration(t *testing.T) {
	// Define test cases with different scenarios
	tests := []struct {
		name        string
		envVars     map[string]string
		expectPanic bool
	}{
		{
			name: "Valid environment variables",
			envVars: map[string]string{
				"MINIFLUX_API_KEY": "valid-api-key",
				"MINIFLUX_URL":     "https://miniflux.example.com",
			},
			expectPanic: false,
		},
	}

	// Iterate through test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables for the test
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Test configuration loading
			if !tt.expectPanic {
				conf := GetEnvVars()
				if conf.MinifluxAPIKey == "" || conf.MinifluxURL == "" {
					t.Errorf("Expected valid configuration but got empty values")
				}
			}

			// Clean up environment variables after the test
			for key := range tt.envVars {
				os.Unsetenv(key)
			}
		})
	}
}

func TestValidateRequired(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		expectErr bool
	}{
		{
			name: "Valid configuration",
			config: Config{
				MinifluxAPIKey: "valid-api-key",
				MinifluxURL:    "https://miniflux.example.com",
			},
			expectErr: false,
		},
		{
			name: "Missing API key",
			config: Config{
				MinifluxAPIKey: "",
				MinifluxURL:    "https://miniflux.example.com",
			},
			expectErr: true,
		},
		{
			name: "Missing URL",
			config: Config{
				MinifluxAPIKey: "valid-api-key",
				MinifluxURL:    "",
			},
			expectErr: true,
		},
		{
			name: "Missing both",
			config: Config{
				MinifluxAPIKey: "",
				MinifluxURL:    "",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequired(tt.config)
			if tt.expectErr && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}
