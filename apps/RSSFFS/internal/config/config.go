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
//	import "github.com/toozej/monogo/apps/RSSFFS/internal/config"
//
//	func main() {
//		conf := config.GetEnvVars()
//		fmt.Printf("API Endpoint: %s\n", conf.RSSReaderEndpoint)
//	}
package config

import (
	"fmt"
	"os"

	sharedconfig "github.com/toozej/monogo/pkg/config"
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
	// It is loaded from the RSS_READER_ENDPOINT environment variable or the
	// rss_reader_endpoint YAML key. This field is required.
	RSSReaderEndpoint string `env:"RSS_READER_ENDPOINT" yaml:"rss_reader_endpoint"`

	// RSSReaderAPIKey specifies the API key for RSS reader authentication.
	// It is loaded from the RSS_READER_API_KEY environment variable or the
	// rss_reader_api_key YAML key. This field is required.
	RSSReaderAPIKey string `env:"RSS_READER_API_KEY" yaml:"rss_reader_api_key"`

	// WebHost specifies the host address to bind the web server to. It is loaded
	// from the WEB_HOST environment variable or the web_host YAML key and
	// defaults to "127.0.0.1" (applied in setDefaults rather than via envDefault
	// so a YAML value is not overwritten).
	WebHost string `env:"WEB_HOST" yaml:"web_host"`

	// WebPort specifies the port number for the web server to listen on. It is
	// loaded from the WEB_PORT environment variable or the web_port YAML key and
	// defaults to 8080 (applied in setDefaults).
	WebPort int `env:"WEB_PORT" yaml:"web_port"`

	// WebUsername and WebPassword enable HTTP Basic authentication for the web
	// interface. They are required when binding to a non-loopback address.
	WebUsername string `env:"WEB_USERNAME" yaml:"web_username"`
	WebPassword string `env:"WEB_PASSWORD" yaml:"web_password"`

	// SingleURLMode specifies whether to use single URL mode for RSS feed discovery.
	// When enabled, only checks for RSS feeds on the provided URL's domain
	// without traversing to other domains found on the webpage. It is loaded from
	// the RSSFFS_SINGLE_URL_MODE environment variable or the rssffs_single_url_mode
	// YAML key and defaults to false (traversal mode).
	SingleURLMode bool `env:"RSSFFS_SINGLE_URL_MODE" yaml:"rssffs_single_url_mode"`
}

// GetEnvVars loads and returns the application configuration, terminating the
// process via os.Exit on failure (including when a required field is missing).
// It is retained for CLI entrypoints that load configuration during package
// initialization.
func GetEnvVars() Config {
	conf, err := Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return conf
}

// Load loads the application configuration from an optional config.yaml file,
// a .env file, and the process environment via the shared pkg/config loader,
// applies defaults, and validates required fields. Environment variables take
// priority over YAML values.
func Load() (Config, error) {
	var conf Config

	// A config.yaml in the working directory is optional; when present it
	// provides the base configuration that the environment can override.
	var opts []sharedconfig.Option
	if _, err := os.Stat("config.yaml"); err == nil {
		opts = append(opts, sharedconfig.WithYAMLFile("config.yaml"))
	}

	if err := sharedconfig.LoadInto(&conf, opts...); err != nil {
		return conf, err
	}

	setDefaults(&conf)

	if err := validateRequired(conf); err != nil {
		return conf, err
	}

	return conf, nil
}

// setDefaults applies defaults for fields that are not configured via
// envDefault (so a YAML-supplied value is never overwritten by a default).
func setDefaults(conf *Config) {
	if conf.WebHost == "" {
		conf.WebHost = "127.0.0.1"
	}
	if conf.WebPort == 0 {
		conf.WebPort = 8080
	}
}

// validateRequired ensures the mandatory RSS reader fields are present.
func validateRequired(conf Config) error {
	if conf.RSSReaderEndpoint == "" {
		return fmt.Errorf("RSS reader API endpoint must be provided via RSS_READER_ENDPOINT or config.yaml")
	}
	if conf.RSSReaderAPIKey == "" {
		return fmt.Errorf("RSS reader API key must be provided via RSS_READER_API_KEY or config.yaml")
	}
	return nil
}
