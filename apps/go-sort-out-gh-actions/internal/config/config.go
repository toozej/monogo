// Package config provides secure configuration management for the go-sort-out-gh-actions application.
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
//	import "github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/config"
//
//	func main() {
//		conf := config.GetEnvVars()
//		fmt.Printf("Username: %s\n", conf.Username)
//	}
package config

import (
	"fmt"
	"os"
	"time"

	sharedconfig "github.com/toozej/monogo/pkg/config"
)

// Config represents the application configuration structure.
//
// This struct defines all configurable parameters for the go-sort-out-gh-actions
// application. Fields are tagged with struct tags that correspond to
// environment variable names for automatic parsing.
//
// Currently supported configuration:
//   - GitHubToken: GitHub API token for API calls, loaded from GH_TOKEN env var
//   - GitHubTokenFallback: Fallback GitHub token from GITHUB_TOKEN env var
//   - Notification: Detailed configuration for notification providers
//   - CreateIssues: Whether to create GitHub issues when archived actions are found
//
// Example:
//
//	type Config struct {
//		GitHubToken string `env:"GH_TOKEN"`
//		GitHubTokenFallback string `env:"GITHUB_TOKEN"`
//		Notification NotificationConfig
//		CreateIssues bool `env:"CREATE_ISSUES" envDefault:"false"`
//	}
type NotificationConfig struct {
	GotifyEndpoint string `env:"GOTIFY_ENDPOINT"`
	GotifyToken    string `env:"GOTIFY_TOKEN"`

	SlackToken     string `env:"SLACK_TOKEN"`
	SlackChannelID string `env:"SLACK_CHANNEL_ID"`

	TelegramToken  string `env:"TELEGRAM_TOKEN"`
	TelegramChatID int64  `env:"TELEGRAM_CHAT_ID"`

	DiscordToken     string `env:"DISCORD_TOKEN"`
	DiscordChannelID string `env:"DISCORD_CHANNEL_ID"`

	PushoverToken       string `env:"PUSHOVER_TOKEN"`
	PushoverRecipientID string `env:"PUSHOVER_RECIPIENT_ID"`

	PushbulletToken          string `env:"PUSHBULLET_TOKEN"`
	PushbulletDeviceNickname string `env:"PUSHBULLET_DEVICE_NICKNAME"`

	Condense bool `env:"NOTIFY_CONDENSE" envDefault:"false"`
}

type Config struct {
	// GitHubToken specifies the GitHub API token for making API calls.
	// It is loaded from the GH_TOKEN environment variable.
	GitHubToken string `env:"GH_TOKEN"`

	// GitHubTokenFallback is a fallback token loaded from GITHUB_TOKEN.
	GitHubTokenFallback string `env:"GITHUB_TOKEN"`

	// Notification specifies configuration for all supported notification providers.
	Notification NotificationConfig

	// CreateIssues specifies whether to create GitHub issues in the repository
	// when archived actions are found.
	CreateIssues bool `env:"CREATE_ISSUES" envDefault:"false"`

	// NoCache disables reading and writing of persistent disk cache.
	NoCache bool `env:"NO_CACHE" envDefault:"false"`

	// RefreshCache ignores existing cache and overwrites it after the run.
	RefreshCache bool `env:"REFRESH_CACHE" envDefault:"false"`

	// CacheTTL is the duration for which cache files remain valid.
	CacheTTL time.Duration `env:"CACHE_TTL" envDefault:"24h"`

	// MCPAddr is the host:port address for the MCP server's SSE transport.
	MCPAddr string `env:"MCP_ADDR" envDefault:"localhost:8080"`

	// MCPTransport is the transport mode for the MCP server ("stdio" or "sse").
	MCPTransport string `env:"MCP_TRANSPORT" envDefault:"stdio"`
}

// osExit is a seam so tests can assert that GetEnvVars exits on a load failure.
var osExit = os.Exit

// GetEnvVars loads and returns the application configuration, terminating the
// process via os.Exit on failure. It is retained for CLI entrypoints that load
// configuration during package initialization.
func GetEnvVars() Config {
	conf, err := loadConfig()
	if err != nil {
		fmt.Println(err)
		osExit(1)
	}
	return conf
}

// Load loads and returns the application configuration, returning any error to
// the caller instead of exiting.
func Load() (Config, error) {
	return loadConfig()
}

// loadConfig populates the configuration using the shared pkg/config loader
// (which handles .env discovery, path-traversal protection, and environment
// parsing) and then merges the GitHub tokens so that GH_TOKEN takes priority
// over GITHUB_TOKEN.
func loadConfig() (Config, error) {
	var conf Config
	if err := sharedconfig.LoadInto(&conf); err != nil {
		return conf, err
	}

	if conf.GitHubToken == "" && conf.GitHubTokenFallback != "" {
		conf.GitHubToken = conf.GitHubTokenFallback
	}

	return conf, nil
}
