// Package config provides secure configuration management for the files2prompt application.
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
// Usage:
//
//	conf := config.GetEnvVars()
//	// Use conf in your application
package config

import (
	sharedconfig "github.com/toozej/monogo/pkg/config"
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
