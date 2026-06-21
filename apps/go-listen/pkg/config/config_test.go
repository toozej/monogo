package config

import (
	"os"
	"testing"

	"github.com/caarlos0/env/v11"
)

func TestConfigLoading(t *testing.T) {
	tests := []struct {
		name        string
		mockEnv     map[string]string
		expectValid bool
		checkFunc   func(*testing.T, Config)
	}{
		{
			name: "Default configuration with Spotify credentials",
			mockEnv: map[string]string{
				"SPOTIFY_CLIENT_ID":     "test_client_id",
				"SPOTIFY_CLIENT_SECRET": "test_client_secret",
			},
			expectValid: true,
			checkFunc: func(t *testing.T, conf Config) {
				// Check server defaults
				if conf.Server.Host != "127.0.0.1" {
					t.Errorf("expected server host '127.0.0.1', got %q", conf.Server.Host)
				}
				if conf.Server.Port != 8080 {
					t.Errorf("expected server port 8080, got %d", conf.Server.Port)
				}

				// Check Spotify config
				if conf.Spotify.ClientID != "test_client_id" {
					t.Errorf("expected client ID 'test_client_id', got %q", conf.Spotify.ClientID)
				}
				if conf.Spotify.ClientSecret != "test_client_secret" {
					t.Errorf("expected client secret 'test_client_secret', got %q", conf.Spotify.ClientSecret)
				}
				if conf.Spotify.RedirectURL != "http://127.0.0.1:8080/callback" {
					t.Errorf("expected redirect URL 'http://127.0.0.1:8080/callback', got %q", conf.Spotify.RedirectURL)
				}

				// Check security defaults
				if conf.Security.RateLimit.RequestsPerSecond != 10 {
					t.Errorf("expected rate limit 10, got %d", conf.Security.RateLimit.RequestsPerSecond)
				}
				if conf.Security.RateLimit.Burst != 20 {
					t.Errorf("expected burst 20, got %d", conf.Security.RateLimit.Burst)
				}

				// Check logging defaults
				if conf.Logging.Level != "info" {
					t.Errorf("expected log level 'info', got %q", conf.Logging.Level)
				}
				if conf.Logging.Format != "text" {
					t.Errorf("expected log format 'text', got %q", conf.Logging.Format)
				}
				if conf.Logging.Output != "stdout" {
					t.Errorf("expected log output 'stdout', got %q", conf.Logging.Output)
				}
				if !conf.Logging.EnableHTTP {
					t.Error("expected HTTP logging to be enabled")
				}
			},
		},
		{
			name: "Custom configuration",
			mockEnv: map[string]string{
				"SERVER_HOST":                             "0.0.0.0",
				"SERVER_PORT":                             "3000",
				"SPOTIFY_CLIENT_ID":                       "custom_client_id",
				"SPOTIFY_CLIENT_SECRET":                   "custom_client_secret",
				"SPOTIFY_REDIRECT_URL":                    "https://example.com/callback",
				"SECURITY_RATE_LIMIT_REQUESTS_PER_SECOND": "5",
				"SECURITY_RATE_LIMIT_BURST":               "10",
				"LOGGING_LEVEL":                           "debug",
				"LOGGING_FORMAT":                          "json",
				"LOGGING_OUTPUT":                          "stderr",
				"LOGGING_ENABLE_HTTP":                     "false",
			},
			expectValid: true,
			checkFunc: func(t *testing.T, conf Config) {
				// Check custom server config
				if conf.Server.Host != "0.0.0.0" {
					t.Errorf("expected server host '0.0.0.0', got %q", conf.Server.Host)
				}
				if conf.Server.Port != 3000 {
					t.Errorf("expected server port 3000, got %d", conf.Server.Port)
				}

				// Check custom Spotify config
				if conf.Spotify.ClientID != "custom_client_id" {
					t.Errorf("expected client ID 'custom_client_id', got %q", conf.Spotify.ClientID)
				}
				if conf.Spotify.RedirectURL != "https://example.com/callback" {
					t.Errorf("expected redirect URL 'https://example.com/callback', got %q", conf.Spotify.RedirectURL)
				}

				// Check custom security config
				if conf.Security.RateLimit.RequestsPerSecond != 5 {
					t.Errorf("expected rate limit 5, got %d", conf.Security.RateLimit.RequestsPerSecond)
				}
				if conf.Security.RateLimit.Burst != 10 {
					t.Errorf("expected burst 10, got %d", conf.Security.RateLimit.Burst)
				}

				// Check custom logging config
				if conf.Logging.Level != "debug" {
					t.Errorf("expected log level 'debug', got %q", conf.Logging.Level)
				}
				if conf.Logging.Format != "json" {
					t.Errorf("expected log format 'json', got %q", conf.Logging.Format)
				}
				if conf.Logging.Output != "stderr" {
					t.Errorf("expected log output 'stderr', got %q", conf.Logging.Output)
				}
				if conf.Logging.EnableHTTP {
					t.Error("expected HTTP logging to be disabled")
				}
			},
		},
		{
			name:        "Default configuration without Spotify credentials",
			mockEnv:     map[string]string{},
			expectValid: true,
			checkFunc: func(t *testing.T, conf Config) {
				// Check that defaults are applied even without Spotify credentials
				if conf.Server.Host != "127.0.0.1" {
					t.Errorf("expected server host '127.0.0.1', got %q", conf.Server.Host)
				}
				if conf.Server.Port != 8080 {
					t.Errorf("expected server port 8080, got %d", conf.Server.Port)
				}

				// Spotify credentials should be empty (warnings will be printed)
				if conf.Spotify.ClientID != "" {
					t.Errorf("expected empty client ID, got %q", conf.Spotify.ClientID)
				}
				if conf.Spotify.ClientSecret != "" {
					t.Errorf("expected empty client secret, got %q", conf.Spotify.ClientSecret)
				}

				// Check that other defaults are still applied
				if conf.Security.RateLimit.RequestsPerSecond != 10 {
					t.Errorf("expected rate limit 10, got %d", conf.Security.RateLimit.RequestsPerSecond)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock environment variables
			var envKeys []string
			for key, value := range tt.mockEnv {
				os.Setenv(key, value)
				envKeys = append(envKeys, key)
			}
			defer func() {
				for _, key := range envKeys {
					os.Unsetenv(key)
				}
			}()

			// Test the configuration loading components separately to avoid os.Exit
			var conf Config
			if err := env.Parse(&conf); err != nil {
				t.Fatalf("Failed to parse config: %v", err)
			}

			if !tt.expectValid {
				t.Fatal("Expected invalid config but validation passed")
			}

			// Run custom checks
			if tt.checkFunc != nil {
				tt.checkFunc(t, conf)
			}
		})
	}
}

func TestServerConfigAddress(t *testing.T) {
	tests := []struct {
		name     string
		config   ServerConfig
		expected string
	}{
		{
			name:     "Default values",
			config:   ServerConfig{},
			expected: "127.0.0.1:8080",
		},
		{
			name:     "Custom host and port",
			config:   ServerConfig{Host: "0.0.0.0", Port: 3000},
			expected: "0.0.0.0:3000",
		},
		{
			name:     "Empty host with custom port",
			config:   ServerConfig{Port: 9000},
			expected: "127.0.0.1:9000",
		},
		{
			name:     "Custom host with zero port",
			config:   ServerConfig{Host: "example.com"},
			expected: "example.com:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.Address()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetEnvVars(t *testing.T) {
	tests := []struct {
		name        string
		mockEnv     map[string]string
		expectValid bool
		checkFunc   func(*testing.T, Config)
	}{
		{
			name: "GetEnvVars with debug false",
			mockEnv: map[string]string{
				"SPOTIFY_CLIENT_ID":     "test_client_id",
				"SPOTIFY_CLIENT_SECRET": "test_client_secret",
				"SERVER_HOST":           "0.0.0.0",
				"SERVER_PORT":           "9000",
			},
			expectValid: true,
			checkFunc: func(t *testing.T, conf Config) {
				if conf.Server.Host != "0.0.0.0" {
					t.Errorf("expected server host '0.0.0.0', got %q", conf.Server.Host)
				}
				if conf.Server.Port != 9000 {
					t.Errorf("expected server port 9000, got %d", conf.Server.Port)
				}
				if conf.Spotify.ClientID != "test_client_id" {
					t.Errorf("expected client ID 'test_client_id', got %q", conf.Spotify.ClientID)
				}
			},
		},
		{
			name: "GetEnvVars with debug true",
			mockEnv: map[string]string{
				"SPOTIFY_CLIENT_ID":     "debug_client_id",
				"SPOTIFY_CLIENT_SECRET": "debug_client_secret",
				"LOGGING_LEVEL":         "debug",
			},
			expectValid: true,
			checkFunc: func(t *testing.T, conf Config) {
				if conf.Spotify.ClientID != "debug_client_id" {
					t.Errorf("expected client ID 'debug_client_id', got %q", conf.Spotify.ClientID)
				}
				if conf.Logging.Level != "debug" {
					t.Errorf("expected log level 'debug', got %q", conf.Logging.Level)
				}
			},
		},
		{
			name: "GetEnvVars with minimal config",
			mockEnv: map[string]string{
				"SPOTIFY_CLIENT_ID":     "minimal_id",
				"SPOTIFY_CLIENT_SECRET": "minimal_secret",
			},
			expectValid: true,
			checkFunc: func(t *testing.T, conf Config) {
				// Should use defaults for unset values
				if conf.Server.Host != "127.0.0.1" {
					t.Errorf("expected default server host '127.0.0.1', got %q", conf.Server.Host)
				}
				if conf.Server.Port != 8080 {
					t.Errorf("expected default server port 8080, got %d", conf.Server.Port)
				}
				if conf.Security.RateLimit.RequestsPerSecond != 10 {
					t.Errorf("expected default rate limit 10, got %d", conf.Security.RateLimit.RequestsPerSecond)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment before test
			envVarsToClean := []string{
				"SERVER_HOST", "SERVER_PORT",
				"SPOTIFY_CLIENT_ID", "SPOTIFY_CLIENT_SECRET", "SPOTIFY_REDIRECT_URL",
				"SECURITY_RATE_LIMIT_REQUESTS_PER_SECOND", "SECURITY_RATE_LIMIT_BURST",
				"LOGGING_LEVEL", "LOGGING_FORMAT", "LOGGING_OUTPUT", "LOGGING_ENABLE_HTTP",
			}

			for _, key := range envVarsToClean {
				os.Unsetenv(key)
			}

			// Set mock environment variables
			var envKeys []string
			for key, value := range tt.mockEnv {
				os.Setenv(key, value)
				envKeys = append(envKeys, key)
			}
			defer func() {
				for _, key := range envKeys {
					os.Unsetenv(key)
				}
			}()

			// Test GetEnvVars function
			conf := GetEnvVars()

			// Run custom checks
			if tt.checkFunc != nil {
				tt.checkFunc(t, conf)
			}
		})
	}
}
