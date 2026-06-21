// Package config provides secure configuration management for the gotts-it application.
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
//	import "github.com/toozej/gotts-it/pkg/config"
//
//	func main() {
//
// conf := config.GetEnvVars()
// fmt.Printf("URL: %s\n", conf.URL)
//
//	}
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config represents the application configuration structure.
//
// This struct defines all configurable parameters for the gotts-it
// application. Fields are tagged with struct tags that correspond to
// environment variable names for automatic parsing.
//
// Example:
//
//	conf := config.GetEnvVars()
//	fmt.Printf("URL: %s\n", conf.URL)
type Config struct {
	// URL is the article URL to fetch and convert to speech.
	URL string `env:"URL"`
	// File is the path to a local HTML or text file to convert to speech.
	File string `env:"FILE"`
	// Output is the path to the output audio file.
	Output string `env:"OUTPUT"`
	// OpenAIBaseURL is the base URL of the OpenAI-compatible TTS endpoint.
	OpenAIBaseURL string `env:"OPENAI_BASE_URL" envDefault:"http://localhost:8000/v1"`
	// OpenAIToken is the API key for the TTS endpoint.
	// Speaches ignores it but the openai-go SDK requires a non-empty value.
	OpenAIToken string `env:"OPENAI_API_KEY"`
	// TTSModel is the TTS model ID (e.g. speaches-ai/Kokoro-82M-v1.0-ONNX).
	TTSModel string `env:"TTS_MODEL" envDefault:"speaches-ai/Kokoro-82M-v1.0-ONNX"`
	// TTSVoice is the voice ID for speech synthesis (e.g. af_heart).
	TTSVoice string `env:"TTS_VOICE" envDefault:"af_heart"`
	// TTSFormat is the output audio format: mp3, wav, flac, or pcm.
	TTSFormat string `env:"TTS_FORMAT" envDefault:"mp3"`
	// TTSSpeed is the speech speed from 0.25 to 4.0; 1.0 is the default.
	TTSSpeed float64 `env:"TTS_SPEED" envDefault:"1.0"`
	// TTSInstructions is an optional model instructions string.
	TTSInstructions string `env:"TTS_INSTRUCTIONS"`
	// TTSTimeout is the HTTP request timeout for each TTS chunk request.
	TTSTimeout time.Duration `env:"TTS_TIMEOUT" envDefault:"5m"`
	// FetchTimeout is the HTTP request timeout for fetching an article URL.
	FetchTimeout time.Duration `env:"FETCH_TIMEOUT" envDefault:"30s"`
	// OutputDir is the output directory for audio files (default: current directory).
	OutputDir string `env:"OUTPUT_DIR"`
	// TTSBackend selects the TTS backend: "openai" or "google".
	TTSBackend string `env:"TTS_BACKEND" envDefault:"openai"`
	// GoogleTranslateLang is the language code for Google Translate TTS (e.g. en, fr, de).
	GoogleTranslateLang string `env:"GOOGLE_TRANSLATE_LANG" envDefault:"en"`
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
// // Load configuration
// conf := config.GetEnvVars()
//
// // Use configuration
//
//	if conf.URL != "" {
//		fmt.Printf("Converting URL: %s\n", conf.URL)
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
