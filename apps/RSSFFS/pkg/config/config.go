// Package config provides secure configuration management for the RSSFFS application.
//
// This package handles loading configuration from environment variables and .env files
// with built-in security measures to prevent path traversal attacks. It uses the
// github.com/caarlos0/env library for environment variable parsing and
// github.com/joho/godotenv for .env file loading.
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
//	import "github.com/toozej/RSSFFS/pkg/config"
//
//	func main() {
//		conf := config.GetEnvVars()
//		fmt.Printf("API Endpoint: %s\n", conf.RSSReaderEndpoint)
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
// This struct defines all configurable parameters for the RSSFFS
// application. Fields are tagged with struct tags that correspond to
// environment variable names for automatic parsing.
//
// Currently supported configuration:
//   - RSSReaderEndpoint: The RSS reader API endpoint URL
//   - RSSReaderAPIKey: The API key for RSS reader authentication
//   - WebHost: The host address for the web server (default: 127.0.0.1)
//   - WebPort: The port number for the web server (default: 8080)
//   - SingleURLMode: Enable single URL mode for RSS discovery (default: false)
//
// Example:
//
//	type Config struct {
//		RSSReaderEndpoint string `env:"RSS_READER_ENDPOINT"`
//		RSSReaderAPIKey   string `env:"RSS_READER_API_KEY"`
//	}
type Config struct {
	// RSSReaderEndpoint specifies the RSS reader API endpoint URL.
	// It is loaded from the RSS_READER_ENDPOINT environment variable.
	// This field is required for the application to function.
	RSSReaderEndpoint string `env:"RSS_READER_ENDPOINT"`

	// RSSReaderAPIKey specifies the API key for RSS reader authentication.
	// It is loaded from the RSS_READER_API_KEY environment variable.
	// This field is required for the application to function.
	RSSReaderAPIKey string `env:"RSS_READER_API_KEY"`

	// WebHost specifies the host address to bind the web server to.
	// It is loaded from the WEB_HOST environment variable.
	// If not specified, defaults to "127.0.0.1".
	WebHost string `env:"WEB_HOST" envDefault:"127.0.0.1"`

	// WebPort specifies the port number for the web server to listen on.
	// It is loaded from the WEB_PORT environment variable.
	// If not specified, defaults to 8080.
	WebPort int `env:"WEB_PORT" envDefault:"8080"`

	// SingleURLMode specifies whether to use single URL mode for RSS feed discovery.
	// When enabled, only checks for RSS feeds on the provided URL's domain
	// without traversing to other domains found on the webpage.
	// It is loaded from the RSSFFS_SINGLE_URL_MODE environment variable.
	// If not specified, defaults to false (traversal mode).
	SingleURLMode bool `env:"RSSFFS_SINGLE_URL_MODE" envDefault:"false"`
}

// GetEnvVars loads and returns the application configuration from environment
// variables and .env files with comprehensive security validation.
//
// This function performs the following operations:
//  1. Securely determines the current working directory
//  2. Constructs and validates the .env file path to prevent traversal attacks
//  3. Loads .env file if it exists in the current directory
//  4. Parses environment variables into the Config struct
//  5. Validates required configuration fields
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
//   - Missing required configuration values
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
//	if conf.RSSReaderEndpoint != "" {
//		fmt.Printf("Using RSS Reader at: %s\n", conf.RSSReaderEndpoint)
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

	// Validate required configuration
	if conf.RSSReaderEndpoint == "" {
		fmt.Printf("Error: RSS reader API endpoint must be provided via RSS_READER_ENDPOINT environment variable\n")
		os.Exit(1)
	}

	if conf.RSSReaderAPIKey == "" {
		fmt.Printf("Error: RSS reader API key must be provided via RSS_READER_API_KEY environment variable\n")
		os.Exit(1)
	}

	return conf
}
