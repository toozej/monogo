// Package config provides secure configuration management for the lego-stego application.
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
//	import "github.com/toozej/monogo/apps/lego-stego/internal/config"
//
//	func main() {
//		conf := config.GetEnvVars()
//		fmt.Printf("Username: %s\n", conf.Username)
//	}
package config

import (
	sharedconfig "github.com/toozej/monogo/pkg/config"
)

// Config represents the application configuration structure.
//
// This struct defines all configurable parameters for the lego-stego
// application. Fields are tagged with struct tags that correspond to
// environment variable names for automatic parsing.
//
// Currently supported configuration:
//   - Username: The username for the application, loaded from USERNAME env var
//
// Example:
//
//	type Config struct {
//		Username string `env:"USERNAME"`
//	}
type Config struct {
	// Username specifies the username for application operations.
	// It is loaded from the USERNAME environment variable.
	// If not set, defaults to empty string.
	Username string `env:"USERNAME"`
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
