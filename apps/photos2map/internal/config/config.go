// Package config provides secure configuration management for the photos2map application.
//
// This package handles loading configuration from environment variables and .env files
// with built-in security measures to prevent path traversal attacks. It uses the
// loading mechanics (.env discovery, path-traversal protection, and environment
// parsing) provided by the shared github.com/toozej/monogo/pkg/config package.
//
// The configuration loading follows a priority order:
//  1. CLI flags (highest priority)
//  2. Environment variables
//  3. .env file in current working directory
//  4. Default values (if any)
//
// Security features:
//   - Path traversal protection for .env file loading
//   - Secure file path resolution using filepath.Abs and filepath.Rel
//   - Validation against directory traversal attempts
//
// Example usage:
//
//	import "github.com/toozej/monogo/apps/photos2map/internal/config"
//
//	func main() {
//		conf := config.GetEnvVars()
//		fmt.Printf("Debug: %t\n", conf.Debug)
//	}
package config

import (
	sharedconfig "github.com/toozej/monogo/pkg/config"
)

// Config represents the application configuration structure.
//
// This struct defines all configurable parameters for the photos2map
// application. Fields are tagged with struct tags that correspond to
// environment variable names for automatic parsing.
//
// Currently supported configuration:
//   - Debug: Enable debug logging, loaded from PHOTOS2MAP_DEBUG env var
//   - Dir: Directory to scan for images, loaded from PHOTOS2MAP_DIR env var
//   - Output: Output format (html or gpx), loaded from PHOTOS2MAP_OUTPUT env var
//
// Example:
//
//	type Config struct {
//		Debug  bool   `env:"PHOTOS2MAP_DEBUG"`
//		Dir    string `env:"PHOTOS2MAP_DIR" envDefault:"."`
//		Output string `env:"PHOTOS2MAP_OUTPUT" envDefault:"html"`
//	}
type Config struct {
	// Debug specifies whether to enable debug-level logging.
	// It is loaded from the PHOTOS2MAP_DEBUG environment variable.
	// If not set, defaults to false.
	Debug bool `env:"PHOTOS2MAP_DEBUG"`

	// Dir specifies the directory to scan for images.
	// It is loaded from the PHOTOS2MAP_DIR environment variable.
	// If not set, defaults to current directory (".").
	Dir string `env:"PHOTOS2MAP_DIR" envDefault:"."`

	// Output specifies the output format (html or gpx).
	// It is loaded from the PHOTOS2MAP_OUTPUT environment variable.
	// If not set, defaults to "html".
	Output string `env:"PHOTOS2MAP_OUTPUT" envDefault:"html"`
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
