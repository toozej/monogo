// Package config provides secure configuration management for the go-listen application.
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
//	import "github.com/toozej/monogo/apps/go-listen/internal/config"
//
//	func main() {
//		conf := config.GetEnvVars()
//		fmt.Printf("Server: %s\n", conf.Server.Address())
//	}
package config

import (
	"fmt"

	sharedconfig "github.com/toozej/monogo/pkg/config"
)

// Config represents the application configuration structure.
//
// This struct defines all configurable parameters for the go-listen
// application. Fields are tagged with struct tags that correspond to
// environment variable names for automatic parsing.
//
// Configuration sections:
//   - Server: HTTP server configuration (host, port)
//   - Spotify: Spotify API credentials and settings
//   - Security: Security-related settings (rate limiting)
//   - Logging: Logging configuration (level, format, output)
//
// Example:
//
//	conf := config.GetEnvVars()
//	fmt.Printf("Server will run on: %s\n", conf.Server.Address())
type Config struct {
	Server   ServerConfig   `envPrefix:"SERVER_"`
	Spotify  SpotifyConfig  `envPrefix:"SPOTIFY_"`
	Security SecurityConfig `envPrefix:"SECURITY_"`
	Logging  LoggingConfig  `envPrefix:"LOGGING_"`
	Scraper  ScraperConfig  `envPrefix:"SCRAPER_"`
}

type ServerConfig struct {
	Host         string `env:"HOST" envDefault:"127.0.0.1"`
	Port         int    `env:"PORT" envDefault:"8080"`
	ReadTimeout  int    `env:"READ_TIMEOUT_SECONDS" envDefault:"30"`
	WriteTimeout int    `env:"WRITE_TIMEOUT_SECONDS" envDefault:"60"`
	IdleTimeout  int    `env:"IDLE_TIMEOUT_SECONDS" envDefault:"120"`
}

type SpotifyConfig struct {
	ClientID     string `env:"CLIENT_ID"`
	ClientSecret string `env:"CLIENT_SECRET"` // #nosec G117 -- OAuth client secret, expected in config
	RedirectURL  string `env:"REDIRECT_URL" envDefault:"http://127.0.0.1:8080/callback"`
}

type SecurityConfig struct {
	RateLimit RateLimitConfig `envPrefix:"RATE_LIMIT_"`
}

type RateLimitConfig struct {
	RequestsPerSecond int `env:"REQUESTS_PER_SECOND" envDefault:"10"`
	Burst             int `env:"BURST" envDefault:"20"`
}

type LoggingConfig struct {
	Level      string `env:"LEVEL" envDefault:"info"`
	Format     string `env:"FORMAT" envDefault:"text"`
	Output     string `env:"OUTPUT" envDefault:"stdout"`
	EnableHTTP bool   `env:"ENABLE_HTTP" envDefault:"true"`
}

type ScraperConfig struct {
	TimeoutSeconds int    `env:"TIMEOUT_SECONDS" envDefault:"30"`
	MaxRetries     int    `env:"MAX_RETRIES" envDefault:"3"`
	RetryBackoff   int    `env:"RETRY_BACKOFF_SECONDS" envDefault:"2"`
	UserAgent      string `env:"USER_AGENT" envDefault:"go-listen/1.0 (Web Scraper)"`
	MaxContentSize int64  `env:"MAX_CONTENT_SIZE" envDefault:"10485760"` // 10MB in bytes
}

// Address returns the server address
func (s ServerConfig) Address() string {
	if s.Host == "" {
		s.Host = "127.0.0.1"
	}
	if s.Port == 0 {
		s.Port = 8080
	}
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// GetEnvVars loads and returns the application configuration, terminating the
// process via os.Exit on failure. The loading mechanics (.env discovery,
// path-traversal protection, and environment parsing) are provided by the
// shared github.com/toozej/monogo/pkg/config package.
func GetEnvVars() Config {
	return sharedconfig.MustLoad[Config]()
}

// Load loads and returns the application configuration, returning any error to
// the caller instead of exiting.
func Load() (Config, error) {
	return sharedconfig.Load[Config]()
}
