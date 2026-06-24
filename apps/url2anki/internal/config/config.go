// Package config provides secure configuration management for the url2anki application.
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
//	import "github.com/toozej/monogo/apps/url2anki/internal/config"
//
//	func main() {
//		conf := config.GetEnvVars()
//		fmt.Printf("URL: %s\n", conf.URL)
//	}
package config

import (
	sharedconfig "github.com/toozej/monogo/pkg/config"
)

// Config represents the application configuration structure.
//
// This struct defines all configurable parameters for the url2anki
// application. Fields are tagged with struct tags that correspond to
// environment variable names for automatic parsing.
//
// Configuration options:
//   - URL: The URL to scrape for flashcards
//   - QuestionSelector: The HTML selector for questions
//   - AnswerSelector: The HTML selector for answers
//   - OutputFile: The filename to export flashcards to
//   - Preview: Whether to preview flashcards before exporting
//   - Debug: Whether to enable debug-level logging
type Config struct {
	// URL specifies the URL to scrape for flashcards.
	// It is loaded from the URL2ANKI_URL environment variable.
	URL string `env:"URL2ANKI_URL"`

	// QuestionSelector specifies the HTML selector for questions.
	// It is loaded from the URL2ANKI_QUESTION_SELECTOR environment variable.
	QuestionSelector string `env:"URL2ANKI_QUESTION_SELECTOR"`

	// AnswerSelector specifies the HTML selector for answers.
	// It is loaded from the URL2ANKI_ANSWER_SELECTOR environment variable.
	AnswerSelector string `env:"URL2ANKI_ANSWER_SELECTOR"`

	// OutputFile specifies the filename to export flashcards to.
	// It is loaded from the URL2ANKI_OUTPUT_FILE environment variable.
	// Defaults to "./anki_cards.csv" if not set.
	OutputFile string `env:"URL2ANKI_OUTPUT_FILE" envDefault:"./anki_cards.csv"`

	// Preview specifies whether to preview flashcards before exporting.
	// It is loaded from the URL2ANKI_PREVIEW environment variable.
	Preview bool `env:"URL2ANKI_PREVIEW"`

	// Debug specifies whether to enable debug-level logging.
	// It is loaded from the URL2ANKI_DEBUG environment variable.
	Debug bool `env:"URL2ANKI_DEBUG"`
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
