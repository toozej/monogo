// Package config provides configuration management for the monogo application.
//
// This package handles loading configuration from environment variables and a .env file
// in the current working directory. It uses the github.com/caarlos0/env library for
// environment variable parsing and github.com/joho/godotenv for .env file loading.
//
// The configuration loading follows a priority order:
//  1. Environment variables (highest priority)
//  2. .env file in current working directory
//  3. Default values (if any)
//
// Example usage:
//
//	import "github.com/toozej/monogo/pkg/config"
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

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config represents the application configuration structure.
//
// This struct defines all configurable parameters for the monogo
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

// GetEnvVars loads and returns the application configuration from environment
// variables and a .env file.
//
// This function performs the following operations:
//  1. Determines the current working directory
//  2. Loads the .env file if it exists in the current directory
//  3. Parses environment variables into the Config struct
//  4. Returns the populated configuration
//
// The function will terminate the program with os.Exit(1) if any critical
// errors occur during configuration loading, such as:
//   - Current directory access failures
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
//	if conf.Username != "" {
//		fmt.Printf("Hello, %s!\n", conf.Username)
//	}
func GetEnvVars() Config {
	// Get current working directory to locate the .env file
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current working directory: %s\n", err)
		os.Exit(1)
	}

	// Load .env file from the current working directory if it exists
	envPath := filepath.Join(cwd, ".env")
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
