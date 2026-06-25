// Package config provides secure configuration management for the rss2socials application.
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
//	import "github.com/toozej/rss2socials/pkg/config"
//
//	func main() {
//	conf, err := config.GetEnvVars()
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Mastodon URL: %s\n", conf.MastodonURL)
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
// This struct defines all configurable parameters for the rss2socials
// application. Fields are tagged with struct tags that correspond to
// environment variable names for automatic parsing.
//
// Currently supported configuration:
//   - MastodonURL: Mastodon instance URL
//   - MastodonAccessToken: Access token for Mastodon API
//   - GotifyURL: Gotify instance URL
//   - GotifyToken: Token for Gotify notifications
//   - Debug: Enable debug logging
//   - FeedURL: RSS feed URL to watch
//   - Interval: Check interval in minutes (default 60)
type Config struct {
	// MastodonURL is the URL of the Mastodon instance.
	MastodonURL string `env:"MASTODON_URL"`
	// MastodonClientKey is the client key for the Mastodon application.
	MastodonClientKey string `env:"MASTODON_CLIENT_KEY"`
	// MastodonClientSecret is the client secret for the Mastodon application.
	MastodonClientSecret string `env:"MASTODON_CLIENT_SECRET"`
	// MastodonAccessToken is the access token for Mastodon API.
	MastodonAccessToken string `env:"MASTODON_ACCESS_TOKEN"`

	// GotifyURL is the URL of the Gotify instance.
	GotifyURL string `env:"GOTIFY_URL"`

	// GotifyToken is the token for Gotify notifications.
	GotifyToken string `env:"GOTIFY_TOKEN"`
	// GotifyNotifyOnSuccess enables Gotify notifications for successful posts.
	GotifyNotifyOnSuccess bool `env:"GOTIFY_NOTIFY_ON_SUCCESS"`

	// Debug enables debug-level logging.
	Debug bool `env:"DEBUG"`

	// FeedURL is the RSS feed URL to watch.
	FeedURL string `env:"FEED_URL"`

	// Interval is the check interval in minutes.
	Interval int `env:"INTERVAL" envDefault:"60"`

	// Category is the URL category filter (optional).
	Category string `env:"CATEGORY"`

	// SkipPrefixCategories is a list of categories that use the "Content - Link" format
	// instead of the default "New blog post: Link" format.
	SkipPrefixCategories []string `env:"SKIP_PREFIX_CATEGORIES" envSeparator:"," envDefault:"Thoughts"`

	// Bluesky configuration
	BlueskyHandle string `env:"BLUESKY_HANDLE"`
	BlueskyAppKey string `env:"BLUESKY_APPKEY"`
	BlueskyPDS    string `env:"BLUESKY_PDS"`

	// Threads configuration
	ThreadsUserID       string `env:"THREADS_USER_ID"`
	ThreadsToken        string `env:"THREADS_ACCESS_TOKEN"`
	ThreadsClientID     string `env:"THREADS_CLIENT_ID"`
	ThreadsClientSecret string `env:"THREADS_CLIENT_SECRET"`
	ThreadsRedirectURI  string `env:"THREADS_REDIRECT_URI"`

	// SocialSites specifies which social media sites to post to.
	// If empty, defaults to all sites with their required credentials fulfilled.
	// Valid values: "mastodon", "bluesky", "threads"
	SocialSites []string `env:"SOCIAL_SITES" envSeparator:","`

	// PostNewEntriesOnly prevents posting all existing RSS entries on first startup.
	// When true (default), only entries that appear after the first successful
	// feed check are posted. Existing entries are stored in the DB but not posted.
	PostNewEntriesOnly bool `env:"POST_NEW_ENTRIES_ONLY" envDefault:"true"`

	// ShortRun enables a short run mode that only processes the 3 most recent
	// RSS feed items instead of all items in the feed.
	ShortRun bool `env:"SHORT_RUN"`

	// DBPath is the filesystem path for the SQLite database.
	// Defaults to "./tooted_posts.db" when empty.
	DBPath string `env:"DB_PATH" envDefault:"./tooted_posts.db"`
}

// GetEnvVars loads and returns the application configuration from environment
// variables and .env files with comprehensive security validation.
//
// This function performs the following operations:
//  1. Securely determines the current working directory
//  2. Constructs and validates the .env file path to prevent traversal attacks
//  3. Loads .env file if it exists in the current directory
//  4. Parses environment variables into the Config struct
//  5. Validates required fields
//  6. Returns the populated configuration
//
// Security measures implemented:
//   - Path traversal detection and prevention using filepath.Rel
//   - Absolute path resolution for secure path operations
//   - Validation against ".." sequences in relative paths
//   - Safe file existence checking before loading
//
// Returns:
//   - Config: A populated configuration struct with values from environment
//     variables and/or .env file
//   - error: Non-nil if any critical error occurs during configuration loading
//
// Example:
//
//	conf, err := config.GetEnvVars()
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Mastodon URL: %s\n", conf.MastodonURL)
func GetEnvVars() (Config, error) {
	// Get current working directory for secure file operations
	cwd, err := os.Getwd()
	if err != nil {
		return Config{}, fmt.Errorf("error getting current working directory: %w", err)
	}

	// Construct secure path for .env file within current directory
	envPath := filepath.Join(cwd, ".env")

	// Ensure the path is within our expected directory (prevent traversal)
	cleanEnvPath, err := filepath.Abs(envPath)
	if err != nil {
		return Config{}, fmt.Errorf("error resolving .env file path: %w", err)
	}
	cleanCwd, err := filepath.Abs(cwd)
	if err != nil {
		return Config{}, fmt.Errorf("error resolving current directory: %w", err)
	}
	relPath, err := filepath.Rel(cleanCwd, cleanEnvPath)
	if err != nil || strings.Contains(relPath, "..") {
		return Config{}, fmt.Errorf("error: .env file path traversal detected")
	}

	// Load .env file if it exists
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			return Config{}, fmt.Errorf("error loading .env file: %w", err)
		}
	}

	// Parse environment variables into config struct
	var conf Config
	if err := env.Parse(&conf); err != nil {
		return Config{}, fmt.Errorf("error parsing environment variables: %w", err)
	}

	// Validate required configuration
	var missing []string
	if conf.MastodonURL == "" {
		missing = append(missing, "MASTODON_URL")
	}
	if conf.MastodonClientKey == "" {
		missing = append(missing, "MASTODON_CLIENT_KEY")
	}
	if conf.MastodonClientSecret == "" {
		missing = append(missing, "MASTODON_CLIENT_SECRET")
	}
	if conf.MastodonAccessToken == "" {
		missing = append(missing, "MASTODON_ACCESS_TOKEN")
	}
	if conf.GotifyURL == "" {
		missing = append(missing, "GOTIFY_URL")
	}
	if conf.GotifyToken == "" {
		missing = append(missing, "GOTIFY_TOKEN")
	}
	if len(missing) > 0 {
		return conf, fmt.Errorf("required environment variables not set: %s", strings.Join(missing, ", "))
	}

	return conf, nil
}

// EnabledSites returns the list of social media sites that should be posted to.
// If SocialSites is explicitly set, only those sites are returned.
// Otherwise, it defaults to all sites that have their required credentials fulfilled.
func (c Config) EnabledSites() []string {
	if len(c.SocialSites) > 0 {
		return c.SocialSites
	}

	var sites []string
	if c.MastodonURL != "" && c.MastodonAccessToken != "" {
		sites = append(sites, "mastodon")
	}
	if c.BlueskyHandle != "" && c.BlueskyAppKey != "" {
		sites = append(sites, "bluesky")
	}
	if c.ThreadsToken != "" && c.ThreadsClientID != "" && c.ThreadsClientSecret != "" {
		sites = append(sites, "threads")
	}
	return sites
}
