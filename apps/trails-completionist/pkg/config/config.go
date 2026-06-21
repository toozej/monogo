// Package config provides secure configuration management for the trails-completionist application.
//
// This package handles loading configuration from environment variables and .env files
// with built-in security measures to prevent path traversal attacks. It uses the
// github.com/caarlos0/env library for environment variable parsing and
// github.com/joho/godotenv for .env file loading.
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
//	import "github.com/toozej/trails-completionist/pkg/config"
//
//	func main() {
//		conf := config.GetEnvVars()
//		fmt.Printf("Track files: %s\n", conf.TrackFiles)
//	}
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
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

// GetEnvVars loads and returns the application configuration from environment
// variables and .env files with comprehensive security validation.
//
// This function performs the following operations:
//  1. Securely determines the current working directory
//  2. Constructs and validates the .env file path to prevent traversal attacks
//  3. Loads .env file if it exists in the current directory
//  4. Parses environment variables into the Config struct
//  5. Returns the populated configuration
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
//	if conf.TrackFiles != "" {
//		fmt.Printf("Track files directory: %s\n", conf.TrackFiles)
//	}
func GetEnvVars() Config {
	// Get current working directory for secure file operations
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current working directory: %s\n", err)
		os.Exit(1)
	}

	// Construct secure path for .env file within current directory
	envPath := filepath.Join(cwd, ".env")

	// Ensure the path is within our expected directory (prevent traversal)
	cleanEnvPath, err := filepath.Abs(envPath)
	if err != nil {
		fmt.Printf("Error resolving .env file path: %s\n", err)
		os.Exit(1)
	}
	cleanCwd, err := filepath.Abs(cwd)
	if err != nil {
		fmt.Printf("Error resolving current directory: %s\n", err)
		os.Exit(1)
	}
	relPath, err := filepath.Rel(cleanCwd, cleanEnvPath)
	if err != nil || strings.Contains(relPath, "..") {
		fmt.Printf("Error: .env file path traversal detected\n")
		os.Exit(1)
	}

	// Load .env file if it exists
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			fmt.Printf("Error loading .env file: %s\n", err)
			os.Exit(1)
		}
	}

	// Parse environment variables into config struct
	var conf Config
	if err := env.Parse(&conf); err != nil {
		fmt.Printf("Error parsing environment variables: %s\n", err)
		os.Exit(1)
	}

	return conf
}
