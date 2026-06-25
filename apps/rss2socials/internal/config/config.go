// Package config provides secure configuration management for the rss2socials application.
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
//	import "github.com/toozej/monogo/apps/rss2socials/internal/config"
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
	"strings"

	sharedconfig "github.com/toozej/monogo/pkg/config"
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
// variables and .env files.
//
// It delegates the loading mechanics (.env discovery, path-traversal
// protection, and environment parsing) to the shared pkg/config loader. It
// does not enforce required fields; call ValidateRequired for that, only when
// the main application functionality runs (not during subcommand execution
// like version, man, or completion).
//
// Returns:
//   - Config: A populated configuration struct with values from environment
//     variables and/or .env file
//   - error: Non-nil if a critical error occurs while loading configuration
//
// Example:
//
//	conf, err := config.GetEnvVars()
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Mastodon URL: %s\n", conf.MastodonURL)
func GetEnvVars() (Config, error) {
	// Delegate .env discovery, path-traversal protection, and environment
	// parsing to the shared loader.
	return sharedconfig.Load[Config]()
}

// ValidateRequired validates that all required configuration values are present.
//
// It should be called only when the main application functionality is invoked,
// not during subcommand execution like version, man, or completion commands,
// so that those subcommands work without a fully configured environment.
func ValidateRequired(conf Config) error {
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
		return fmt.Errorf("required environment variables not set: %s", strings.Join(missing, ", "))
	}
	return nil
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
