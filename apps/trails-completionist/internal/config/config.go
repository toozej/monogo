// Package config provides secure configuration management for the trails-completionist application.
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
//	import "github.com/toozej/monogo/apps/trails-completionist/internal/config"
//
//	func main() {
//		conf := config.GetEnvVars()
//		fmt.Printf("Track files: %s\n", conf.TrackFiles)
//	}
package config

import (
	sharedconfig "github.com/toozej/monogo/pkg/config"
)

// Config represents the application configuration structure.
//
// This struct defines all configurable parameters for the trails-completionist
// application. Fields are tagged with struct tags that correspond to
// environment variable names for automatic parsing.
//
// Configuration parameters:
//   - OSMRegionFile: Path to OSM region file for trail data
//   - TrackFiles: Path to directory containing GPX track files
//   - InputFile: Path to input file containing trail information
//   - ChecklistFile: Path to output checklist file
//   - HTMLFile: Path to output HTML file
//   - Serve: Whether to serve the generated HTML file
type Config struct {
	// OSMRegionFile specifies the path to the OSM region file.
	// It is loaded from the OSM_REGION_FILE environment variable.
	OSMRegionFile string `env:"OSM_REGION_FILE"`

	// TrackFiles specifies the path to the directory containing GPX track files.
	// It is loaded from the TRACK_FILES environment variable.
	TrackFiles string `env:"TRACK_FILES"`

	// InputFile specifies the path to the input file containing trail information.
	// It is loaded from the INPUT_FILE environment variable.
	InputFile string `env:"INPUT_FILE"`

	// ChecklistFile specifies the path to the output checklist file.
	// It is loaded from the CHECKLIST_FILE environment variable.
	ChecklistFile string `env:"CHECKLIST_FILE"`

	// HTMLFile specifies the path to the output HTML file.
	// It is loaded from the HTML_FILE environment variable.
	HTMLFile string `env:"HTML_FILE"`

	// Serve specifies whether to serve the generated HTML file.
	// It is loaded from the SERVE environment variable.
	Serve bool `env:"SERVE"`
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
