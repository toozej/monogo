// Package config provides secure configuration management for the ghreleases2rss application.
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
//	import "github.com/toozej/monogo/apps/ghreleases2rss/internal/config"
//
//	func main() {
//		conf := config.GetEnvVars()
//		fmt.Printf("Miniflux URL: %s\n", conf.MinifluxURL)
//	}
package config

import (
	"fmt"

	sharedconfig "github.com/toozej/monogo/pkg/config"
)

// Config represents the application configuration structure.
//
// This struct defines all configurable parameters for the ghreleases2rss
// application. Fields are tagged with struct tags that correspond to
// environment variable names for automatic parsing.
//
// Currently supported configuration:
//   - MinifluxAPIKey: The API key for Miniflux RSS reader
//   - MinifluxURL: The URL endpoint for Miniflux API
//
// Example:
//
//	type Config struct {
//		MinifluxAPIKey string `env:"MINIFLUX_API_KEY"`
//		MinifluxURL    string `env:"MINIFLUX_URL"`
//	}
type Config struct {
	// MinifluxAPIKey specifies the API key for Miniflux operations.
	// It is loaded from the MINIFLUX_API_KEY environment variable.
	// This field is required for the application to function.
	MinifluxAPIKey string `env:"MINIFLUX_API_KEY"`

	// MinifluxURL specifies the URL endpoint for Miniflux API.
	// It is loaded from the MINIFLUX_URL environment variable.
	// This field is required for the application to function.
	MinifluxURL string `env:"MINIFLUX_URL"`
}

// GetEnvVars loads and returns the application configuration, terminating the
// process via os.Exit on failure. It delegates the loading mechanics (.env
// discovery, path-traversal protection, and environment parsing) to the shared
// pkg/config loader and is retained for CLI entrypoints that load
// configuration during package initialization.
func GetEnvVars() Config {
	return sharedconfig.MustLoad[Config]()
}

// Load loads and returns the application configuration, returning any error to
// the caller instead of exiting.
func Load() (Config, error) {
	return sharedconfig.Load[Config]()
}

// ValidateRequired validates that all required configuration values are present.
//
// This function checks that essential configuration fields are not empty.
// It should be called only when the main application functionality is invoked,
// not during subcommand execution like version, man, or completion commands.
//
// Parameters:
//   - conf: The configuration struct to validate
//
// Returns:
//   - error: An error if any required fields are missing, nil otherwise
//
// Example:
//
//	conf := config.GetEnvVars()
//	if err := config.ValidateRequired(conf); err != nil {
//		fmt.Printf("Configuration error: %s\n", err)
//		os.Exit(1)
//	}
func ValidateRequired(conf Config) error {
	if conf.MinifluxAPIKey == "" || conf.MinifluxURL == "" {
		return fmt.Errorf("MINIFLUX_API_KEY and MINIFLUX_URL environment variables are required")
	}
	return nil
}
