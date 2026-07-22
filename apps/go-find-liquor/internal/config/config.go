// Package config provides secure configuration management for the go-find-liquor application.
//
// This package handles loading configuration from multiple sources including YAML files,
// environment variables, and .env files with built-in security measures and legacy
// configuration migration support. It uses github.com/caarlos0/env for environment
// variable parsing and github.com/joho/godotenv for .env file loading.
//
// The configuration loading follows a priority order:
//  1. Environment variables (highest priority)
//  2. YAML configuration file (config.yaml or custom file)
//  3. .env file in current working directory
//  4. Default values (lowest priority)
//
// Key features:
//   - Multi-user configuration support with individual search preferences
//   - Legacy single-user configuration migration
//   - Secure .env file loading with path traversal protection
//   - Environment variable support with GFL_ prefix
//   - YAML and JSON serialization support
//   - Configuration validation
//
// Security features:
//   - Path traversal protection for .env file loading
//   - Secure file path resolution using filepath.Abs and filepath.Rel
//   - Validation against directory traversal attempts
//
// Configuration structure supports:
//   - Global settings (interval, user agent, verbose logging)
//   - Multiple users with individual preferences
//   - Per-user notification configurations with condensing options
//   - Legacy format automatic migration
//
// Example usage:
//
//	import "github.com/toozej/monogo/apps/go-find-liquor/internal/config"
//
//	func main() {
//		conf, err := config.GetConfig()
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		// Use multi-user configuration
//		for _, user := range conf.Users {
//			fmt.Printf("User: %s, Items: %v\n", user.Name, user.Items)
//		}
//	}
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	sharedconfig "github.com/toozej/monogo/pkg/config"
)

// CommonItem represents a commonly available liquor item used for health check searches
type CommonItem struct {
	Code string `yaml:"code" json:"code"`
	Name string `yaml:"name" json:"name"`
}

// NotificationConfig stores configuration for notification methods
type NotificationConfig struct {
	Type       string            `yaml:"type" json:"type"`
	Endpoint   string            `yaml:"endpoint" json:"endpoint"`
	Credential map[string]string `yaml:"credential" json:"credential"`
	Condense   bool              `yaml:"condense" json:"condense"`
}

// UserConfig represents configuration for a single user
type UserConfig struct {
	Name          string               `yaml:"name" json:"name"`
	Items         []string             `yaml:"items" json:"items"`
	Zipcode       string               `yaml:"zipcode" json:"zipcode"`
	Distance      int                  `yaml:"distance" json:"distance"`
	Notifications []NotificationConfig `yaml:"notifications" json:"notifications"`
}

// Config stores all configuration for the application
type Config struct {
	// Global settings.
	//
	// Interval and Distance intentionally omit envDefault: the shared loader
	// applies the YAML layer before env parsing, and an envDefault would
	// overwrite a value supplied via YAML. Their defaults are applied in
	// setDefaults instead.
	Interval  time.Duration `yaml:"interval" json:"interval" env:"GFL_INTERVAL"`
	UserAgent string        `yaml:"user_agent" json:"user_agent" env:"GFL_USER_AGENT"`
	Verbose   bool          `yaml:"verbose" json:"verbose" env:"GFL_VERBOSE"`
	// FlareSolverrURL optionally routes OLCC requests through a FlareSolverr
	// instance. It is normally set to the Docker Compose sidecar endpoint.
	FlareSolverrURL string `yaml:"flaresolverr_url" json:"flaresolverr_url" env:"GFL_FLARESOLVERR_URL"`

	// Commonly available items used for health check searches
	CommonItems []CommonItem `yaml:"common_items" json:"common_items"`

	// User-specific configurations
	Users []UserConfig `yaml:"users" json:"users"`

	// Legacy fields for backward compatibility (will be populated if old format detected)
	Items         []string             `yaml:"items,omitempty" json:"items,omitempty" env:"GFL_ITEMS" envSeparator:","`
	Zipcode       string               `yaml:"zipcode,omitempty" json:"zipcode,omitempty" env:"GFL_ZIPCODE"`
	Distance      int                  `yaml:"distance,omitempty" json:"distance,omitempty" env:"GFL_DISTANCE"`
	Notifications []NotificationConfig `yaml:"notifications,omitempty" json:"notifications,omitempty"`
}

// configFile holds the path to the config file set via CLI
var configFile string

// SetConfigFile sets the config file path for loading
func SetConfigFile(path string) {
	configFile = path
}

// GetConfig is the primary entrypoint to the config package. It loads
// configuration from an optional YAML file, a .env file, and the process
// environment via the shared pkg/config loader, then applies defaults,
// migrates any legacy single-user configuration, and validates the result.
//
// Environment variables take priority over YAML values. Defaults for fields
// that may also be supplied via YAML are applied in setDefaults (rather than
// via envDefault) so a YAML value is never overwritten by a default.
func GetConfig() (Config, error) {
	var conf Config

	// A config file explicitly set via the CLI must exist; the default
	// config.yaml in the working directory is optional.
	var opts []sharedconfig.Option
	switch {
	case configFile != "":
		opts = append(opts, sharedconfig.WithRequiredYAMLFile(configFile))
	default:
		if _, err := os.Stat("config.yaml"); err == nil {
			opts = append(opts, sharedconfig.WithYAMLFile("config.yaml"))
		}
	}

	if err := sharedconfig.LoadInto(&conf, opts...); err != nil {
		return conf, fmt.Errorf("failed to load configuration: %w", err)
	}

	setDefaults(&conf)

	// Check for legacy configuration format and migrate if needed
	if isLegacyConfig(conf) {
		migratedConfig, err := migrateLegacyConfig(conf)
		if err != nil {
			return conf, fmt.Errorf("failed to migrate legacy config: %w", err)
		}
		conf = migratedConfig
	}

	// Validate configuration
	if err := validateConfig(conf); err != nil {
		return conf, fmt.Errorf("invalid configuration: %w", err)
	}

	return conf, nil
}

// setDefaults applies defaults for fields that are not configurable through
// envDefault (because doing so would overwrite YAML-supplied values).
func setDefaults(conf *Config) {
	if conf.Interval == 0 {
		conf.Interval = 12 * time.Hour
	}
	if conf.Distance == 0 {
		conf.Distance = 10
	}
	for i := range conf.Users {
		if conf.Users[i].Distance == 0 {
			conf.Users[i].Distance = 10
		}
	}
}

// isLegacyConfig detects if the configuration is in the old format
func isLegacyConfig(config Config) bool {
	// Legacy format has items, zipcode, or notifications at root level
	// and no users array
	return len(config.Users) == 0 && (len(config.Items) > 0 || config.Zipcode != "" || len(config.Notifications) > 0)
}

// migrateLegacyConfig converts legacy configuration to multi-user format
func migrateLegacyConfig(config Config) (Config, error) {
	if len(config.Items) == 0 {
		return config, fmt.Errorf("legacy configuration must have items specified")
	}

	if config.Zipcode == "" {
		return config, fmt.Errorf("legacy configuration must have zipcode specified")
	}

	// Create a single user from legacy configuration
	user := UserConfig{
		Name:          "default",
		Items:         config.Items,
		Zipcode:       config.Zipcode,
		Distance:      config.Distance,
		Notifications: config.Notifications,
	}

	// Set default distance if not specified
	if user.Distance == 0 {
		user.Distance = 10
	}

	// Create new config with migrated user
	newConfig := Config{
		Interval:    config.Interval,
		UserAgent:   config.UserAgent,
		Verbose:     config.Verbose,
		CommonItems: config.CommonItems,
		Users:       []UserConfig{user},
	}

	fmt.Printf("Migrated legacy configuration to multi-user format with user '%s'\n", user.Name)

	return newConfig, nil
}

// validateConfig validates the configuration structure
func validateConfig(config Config) error {
	if len(config.Users) == 0 {
		return fmt.Errorf("at least one user must be configured")
	}

	seenNames := make(map[string]struct{}, len(config.Users))
	for i, user := range config.Users {
		if strings.TrimSpace(user.Name) == "" {
			return fmt.Errorf("user %d must have a name", i)
		}
		if _, exists := seenNames[user.Name]; exists {
			return fmt.Errorf("duplicate user name %q", user.Name)
		}
		seenNames[user.Name] = struct{}{}

		if len(user.Items) == 0 {
			return fmt.Errorf("user '%s' must have at least one item to search for", user.Name)
		}

		if strings.TrimSpace(user.Zipcode) == "" {
			return fmt.Errorf("user '%s' must have a zipcode specified", user.Name)
		}

		if user.Distance <= 0 {
			return fmt.Errorf("user '%s' must have a positive distance", user.Name)
		}

		for itemIndex, item := range user.Items {
			if strings.TrimSpace(item) == "" {
				return fmt.Errorf("user '%s' item %d must not be empty", user.Name, itemIndex)
			}
		}
	}
	if config.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	if config.FlareSolverrURL != "" {
		if err := validateHTTPURL("flaresolverr_url", config.FlareSolverrURL); err != nil {
			return err
		}
	}

	return nil
}

func validateHTTPURL(name, value string) error {
	// Keep the endpoint validation here so an invalid optional integration fails
	// during startup instead of after a search has begun.
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return fmt.Errorf("%s must be an absolute HTTP(S) URL", name)
	}
	return nil
}
