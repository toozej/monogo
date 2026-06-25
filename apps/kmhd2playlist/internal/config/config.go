// Package config provides secure configuration management for the kmhd2playlist application.
//
// This package handles loading configuration from environment variables and .env files
// with built-in security measures to prevent path traversal attacks. It uses the
// loading mechanics (.env discovery, path-traversal protection, and environment
// parsing) provided by the shared github.com/toozej/monogo/pkg/config package.
//
// The configuration loading follows a priority order:
//  1. Environment variables (highest priority)
//  2. .env file in current working directory
//  3. Default values (if any)
//
// Security features:
//   - Path traversal protection for .env file loading
//   - Secure file path resolution using filepath.Abs and filepath.Rel
//   - Validation against directory traversal attempts
//
// Example usage:
//
//	import "github.com/toozej/monogo/apps/kmhd2playlist/internal/config"
//
//	func main() {
//		conf := config.GetEnvVars()
//		fmt.Printf("Username: %s\n", conf.Username)
//	}
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sharedconfig "github.com/toozej/monogo/pkg/config"
)

// Config represents the main application configuration with nested service configurations.
type Config struct {
	MusicClient  string             `env:"MUSIC_CLIENT" envDefault:"spotify"`
	Spotify      SpotifyConfig      `envPrefix:"SPOTIFY_"`
	YouTubeMusic YouTubeMusicConfig `envPrefix:"YOUTUBEMUSIC_"`
	KMHD         KMHDConfig         `envPrefix:"KMHD_"`
	Server       ServerConfig       `envPrefix:"SERVER_"`
}

// SpotifyConfig represents the configuration for Spotify API integration.
//
// This struct contains all the necessary configuration parameters for
// authenticating and interacting with the Spotify API.
type SpotifyConfig struct {
	// ClientID is the Spotify application client ID.
	ClientID string `env:"CLIENT_ID"`

	// ClientSecret is the Spotify application client secret.
	ClientSecret string `env:"CLIENT_SECRET"` // #nosec G117 -- OAuth client secret, expected in config

	// RedirectURL is the callback URL for OAuth authentication.
	RedirectURL string `env:"REDIRECT_URI"`

	// PlaylistNamePrefix is the prefix for monthly Spotify playlists to sync KMHD songs to.
	// Monthly playlists will be created with format: "{prefix}-YYYY-MM" (e.g., "KMHD-2025-10")
	PlaylistNamePrefix string `env:"PLAYLIST_NAME_PREFIX"`

	// TokenFilePath is the path where the Spotify authentication token is stored.
	// If not specified, defaults to ~/.config/kmhd2playlist/spotify_token.json
	TokenFilePath string `env:"TOKEN_FILE_PATH" envDefault:"~/.config/kmhd2playlist/spotify_token.json"`
}

// YouTubeMusicConfig represents the configuration for YouTube Music API integration.
type YouTubeMusicConfig struct {
	Cookie string `env:"COOKIE"`

	PlaylistNamePrefix string `env:"PLAYLIST_NAME_PREFIX"`

	TokenFilePath string `env:"TOKEN_FILE_PATH" envDefault:"~/.config/kmhd2playlist/youtubemusic_token.json"`
}

// KMHDConfig represents the configuration for KMHD JSON API integration.
//
// This struct contains all the necessary configuration parameters for
// fetching playlist data from the KMHD JSON API.
type KMHDConfig struct {
	// APIEndpoint is the JSON API endpoint URL for fetching playlist data.
	APIEndpoint string `env:"API_ENDPOINT" envDefault:"https://www.kmhd.org/pf/api/v3/content/fetch/playlist"`

	// HTTPTimeout is the timeout for HTTP requests in seconds.
	HTTPTimeout int `env:"HTTP_TIMEOUT" envDefault:"30"`
}

// ServerConfig represents the server configuration.
type ServerConfig struct {
	Host string `env:"HOST" envDefault:"127.0.0.1"`
	Port int    `env:"PORT" envDefault:"8080"`
}

// GetEnvVars loads and returns the application configuration from environment
// variables and .env files with comprehensive security validation.
//
// This function performs the following operations:
//  1. Securely determines the current working directory
//  2. Constructs and validates the .env file path to prevent traversal attacks
//  3. Loads .env file if it exists in the current directory
//  4. Parses environment variables into the Config struct
//  5. Validates the configuration
//  6. Returns the populated configuration
//
// Security measures implemented:
//   - Path traversal detection and prevention using filepath.Rel
//   - Absolute path resolution for secure path operations
//   - Validation against ".." sequences in relative paths
//   - Safe file existence checking before loading
//
// The function will terminate the program with os.Exit(1) if any critical
// errors occur during configuration loading, such as:
//   - Current directory access failures
//   - Path traversal attempts detected
//   - .env file parsing errors
//   - Environment variable parsing failures
//   - Configuration validation errors
//
// Returns:
//   - Config: A populated configuration struct with values from environment
//     variables and/or .env file
//
// Example:
//
//	// Load configuration
//	conf := config.GetEnvVars()
//
//	// Use configuration
//	fmt.Printf("Spotify Client ID: %s\n", conf.Spotify.ClientID)
//	fmt.Printf("KMHD API Endpoint: %s\n", conf.KMHD.APIEndpoint)
func GetEnvVars() Config {
	// Delegate .env discovery, path-traversal protection, and environment
	// parsing to the shared loader; it exits the process on failure.
	conf := sharedconfig.MustLoad[Config]()

	// Validate configuration
	if err := validateConfig(&conf); err != nil {
		fmt.Printf("Configuration validation error: %s\n", err)
		fmt.Println("Please check your configuration and try again.")
		os.Exit(1)
	}

	return conf
}

// Address returns the server address
func (s ServerConfig) Address() string {
	if s.Host == "" {
		s.Host = "127.0.0.1"
	}
	if s.Port == 0 {
		s.Port = 8080
	}
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// GetTokenFilePath returns the resolved token file path, handling tilde expansion
// and ensuring the directory exists.
func (s SpotifyConfig) GetTokenFilePath() (string, error) {
	tokenPath := s.TokenFilePath

	// Handle tilde expansion
	if strings.HasPrefix(tokenPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		tokenPath = filepath.Join(homeDir, tokenPath[2:])
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(tokenPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Ensure the directory exists
	tokenDir := filepath.Dir(absPath)
	if err := os.MkdirAll(tokenDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create token directory %s: %w", tokenDir, err)
	}

	return absPath, nil
}

// GetTokenFilePath returns the resolved token file path, handling tilde expansion
// and ensuring the directory exists.
func (y YouTubeMusicConfig) GetTokenFilePath() (string, error) {
	tokenPath := y.TokenFilePath

	if strings.HasPrefix(tokenPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		tokenPath = filepath.Join(homeDir, tokenPath[2:])
	}

	absPath, err := filepath.Abs(tokenPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	tokenDir := filepath.Dir(absPath)
	if err := os.MkdirAll(tokenDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create token directory %s: %w", tokenDir, err)
	}

	return absPath, nil
}

// validateConfig validates the configuration
func validateConfig(conf *Config) error {
	var errors []string

	// Validate MUSIC_CLIENT
	switch conf.MusicClient {
	case "spotify":
		if conf.Spotify.ClientID == "" {
			fmt.Println("Warning: SPOTIFY_CLIENT_ID is not set. The application will not be able to connect to Spotify.")
			fmt.Println("Please set your Spotify credentials to use the application.")
		}
		if conf.Spotify.ClientSecret == "" {
			fmt.Println("Warning: SPOTIFY_CLIENT_SECRET is not set. The application will not be able to connect to Spotify.")
		}
	case "youtube":
		if conf.YouTubeMusic.Cookie == "" {
			fmt.Println("Warning: YOUTUBEMUSIC_COOKIE is not set. The application will not be able to connect to YouTube Music.")
			fmt.Println("Please set your YouTube Music cookie to use the application.")
		}
	default:
		errors = append(errors, "MUSIC_CLIENT must be 'spotify' or 'youtube'")
	}

	// Validate server configuration
	if conf.Server.Port < 1 || conf.Server.Port > 65535 {
		errors = append(errors, "server port must be between 1 and 65535")
	}

	// Validate KMHD configuration
	if conf.KMHD.APIEndpoint == "" {
		errors = append(errors, "KMHD API endpoint is required")
	}
	if conf.KMHD.HTTPTimeout <= 0 {
		errors = append(errors, "KMHD HTTP timeout must be greater than 0")
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration errors:\n- %s", strings.Join(errors, "\n- "))
	}

	return nil
}
