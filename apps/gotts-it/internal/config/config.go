// Package config provides secure configuration management for the gotts-it application.
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
//	import "github.com/toozej/monogo/apps/gotts-it/internal/config"
//
//	func main() {
//
// conf := config.GetEnvVars()
// fmt.Printf("URL: %s\n", conf.URL)
//
//	}
package config

import (
	"time"

	sharedconfig "github.com/toozej/monogo/pkg/config"
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
