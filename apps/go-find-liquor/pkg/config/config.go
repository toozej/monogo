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
//	import "github.com/toozej/go-find-liquor/pkg/config"
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
	"path/filepath"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
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
	// Global settings
	Interval  time.Duration `yaml:"interval" json:"interval" env:"GFL_INTERVAL" envDefault:"12h"`
	UserAgent string        `yaml:"user_agent" json:"user_agent" env:"GFL_USER_AGENT"`
	Verbose   bool          `yaml:"verbose" json:"verbose" env:"GFL_VERBOSE" envDefault:"false"`

	// Commonly available items used for health check searches
	CommonItems []CommonItem `yaml:"common_items" json:"common_items"`

	// User-specific configurations
	Users []UserConfig `yaml:"users" json:"users"`

	// Legacy fields for backward compatibility (will be populated if old format detected)
	Items         []string             `yaml:"items,omitempty" json:"items,omitempty" env:"GFL_ITEMS" envSeparator:","`
	Zipcode       string               `yaml:"zipcode,omitempty" json:"zipcode,omitempty" env:"GFL_ZIPCODE"`
	Distance      int                  `yaml:"distance,omitempty" json:"distance,omitempty" env:"GFL_DISTANCE" envDefault:"10"`
	Notifications []NotificationConfig `yaml:"notifications,omitempty" json:"notifications,omitempty"`
}

// configFile holds the path to the config file set via CLI
var configFile string

// SetConfigFile sets the config file path for loading
func SetConfigFile(path string) {
	configFile = path
}

// GetConfig is the primary entrypoint to the config package, loading configuration structs from .env and yaml files
func GetConfig() (Config, error) {
	var config Config

	// Load .env file if it exists (with security checks)
	if err := loadEnvFile(); err != nil {
		return config, fmt.Errorf("failed to load .env file: %w", err)
	}

	// Parse environment variables first (they have highest priority)
	if err := env.Parse(&config); err != nil {
		return config, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	// Load YAML config file if specified or if default exists
	yamlConfig, err := loadYAMLConfig()
	if err != nil {
		return config, fmt.Errorf("failed to load YAML config: %w", err)
	}

	// Merge YAML config with env config (env takes priority)
	config = mergeConfigs(yamlConfig, config)

	// Check for legacy configuration format and migrate if needed
	if isLegacyConfig(config) {
		migratedConfig, err := migrateLegacyConfig(config)
		if err != nil {
			return config, fmt.Errorf("failed to migrate legacy config: %w", err)
		}
		config = migratedConfig
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return config, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadEnvFile securely loads .env file from current directory
func loadEnvFile() error {
	// Get current working directory for secure file operations
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current working directory: %w", err)
	}

	// Construct secure path for .env file within current directory
	envPath := filepath.Join(cwd, ".env")

	// Ensure the path is within our expected directory (prevent traversal)
	cleanEnvPath, err := filepath.Abs(envPath)
	if err != nil {
		return fmt.Errorf("error resolving .env file path: %w", err)
	}
	cleanCwd, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("error resolving current directory: %w", err)
	}
	relPath, err := filepath.Rel(cleanCwd, cleanEnvPath)
	if err != nil || strings.Contains(relPath, "..") {
		return fmt.Errorf("error: .env file path traversal detected")
	}

	// Load .env file if it exists
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			return fmt.Errorf("error loading .env file: %w", err)
		}
	}

	return nil
}

// loadYAMLConfig loads configuration from YAML file
func loadYAMLConfig() (Config, error) {
	var config Config

	// Determine which config file to load
	var configPath string
	if configFile != "" {
		configPath = configFile
	} else if _, err := os.Stat("config.yaml"); err == nil {
		configPath = "config.yaml"
	} else {
		// No config file to load, return empty config
		return config, nil
	}

	// Resolve config path to an absolute path for consistent handling
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return config, fmt.Errorf("failed to resolve config file path: %w", err)
	}

	// The parent directory of the config file becomes the root for os.OpenRoot.
	// os.OpenRoot.ReadFile requires paths relative to the root directory;
	// absolute paths are rejected with "path escapes from parent".
	configDir := filepath.Dir(absConfigPath)
	configName := filepath.Base(absConfigPath)

	// Create a root filesystem scoped to the config file's parent directory
	root, err := os.OpenRoot(configDir)
	if err != nil {
		return config, fmt.Errorf("failed to create secure root filesystem: %w", err)
	}
	defer root.Close()

	// Read and parse YAML file using scoped root with relative path
	data, err := root.ReadFile(configName)
	if err != nil {
		return config, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to unmarshal YAML config: %w", err)
	}

	return config, nil
}

// mergeConfigs merges YAML config with env config, giving priority to env values
func mergeConfigs(yamlConfig, envConfig Config) Config {
	result := yamlConfig

	// Override with env values if they are set (non-zero values)
	if envConfig.Interval != 0 {
		result.Interval = envConfig.Interval
	}
	if envConfig.UserAgent != "" {
		result.UserAgent = envConfig.UserAgent
	}
	if envConfig.Verbose {
		result.Verbose = envConfig.Verbose
	}

	// Legacy fields - only override if env has values
	if len(envConfig.Items) > 0 {
		result.Items = envConfig.Items
	}
	if envConfig.Zipcode != "" {
		result.Zipcode = envConfig.Zipcode
	}
	if envConfig.Distance != 0 {
		result.Distance = envConfig.Distance
	}

	// Set defaults if not set in either config
	if result.Interval == 0 {
		result.Interval = 12 * time.Hour
	}
	if result.Distance == 0 {
		result.Distance = 10
	}

	return result
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
		Interval:  config.Interval,
		UserAgent: config.UserAgent,
		Verbose:   config.Verbose,
		Users:     []UserConfig{user},
	}

	fmt.Printf("Migrated legacy configuration to multi-user format with user '%s'\n", user.Name)

	return newConfig, nil
}

// validateConfig validates the configuration structure
func validateConfig(config Config) error {
	if len(config.Users) == 0 {
		return fmt.Errorf("at least one user must be configured")
	}

	for i, user := range config.Users {
		if user.Name == "" {
			return fmt.Errorf("user %d must have a name", i)
		}

		if len(user.Items) == 0 {
			return fmt.Errorf("user '%s' must have at least one item to search for", user.Name)
		}

		if user.Zipcode == "" {
			return fmt.Errorf("user '%s' must have a zipcode specified", user.Name)
		}

		if user.Distance <= 0 {
			return fmt.Errorf("user '%s' must have a positive distance", user.Name)
		}
	}

	return nil
}
