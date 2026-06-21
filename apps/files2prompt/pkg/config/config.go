// Package config provides secure configuration management for the files2prompt application.
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
// Usage:
//
//	conf := config.GetEnvVars()
//	// Use conf in your application
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
// This struct defines all configurable parameters for the files2prompt
// application. Fields are tagged with struct tags that correspond to
// environment variable names for automatic parsing.
//
// Configuration options include:
//   - Paths: File and directory paths to process
//   - Extensions: File extensions to include in processing
//   - IncludeHidden: Whether to include hidden files and directories
//   - IgnoreGitignore: Whether to ignore .gitignore file rules
//   - IgnorePatterns: Custom patterns to ignore during processing
//   - OutputFile: Path for output file (stdout if empty)
//   - ClaudeXML: Enable XML output format for Claude AI
//   - LineNumbers: Include line numbers in output
//   - Markdown: Format output as Markdown with code blocks
//   - Null: Use null character separators for stdin input
//
// Example:
//
//	type Config struct {
//		Paths      []string `env:"PATHS"`
//		Extensions []string `env:"EXTENSIONS"`
//		// ... other fields
//	}
type Config struct {
	Paths           []string `env:"PATHS" envDefault:""`
	Extensions      []string `env:"EXTENSIONS" envDefault:""`
	IncludeHidden   bool     `env:"INCLUDE_HIDDEN" envDefault:"false"`
	IgnoreGitignore bool     `env:"IGNORE_GITIGNORE" envDefault:"false"`
	IgnorePatterns  []string `env:"IGNORE_PATTERNS" envDefault:""`
	OutputFile      string   `env:"OUTPUT_FILE" envDefault:""`
	ClaudeXML       bool     `env:"CLAUDE_XML" envDefault:"false"`
	LineNumbers     bool     `env:"LINE_NUMBERS" envDefault:"false"`
	Markdown        bool     `env:"MARKDOWN" envDefault:"false"`
	Null            bool     `env:"NULL" envDefault:"false"`
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
