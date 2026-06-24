// Package config defines the monogo starter application's configuration.
//
// The struct lives with the app, while the loading mechanics (.env discovery,
// path-traversal protection, and environment parsing) are provided by the
// shared github.com/toozej/monogo/pkg/config package.
package config

import sharedconfig "github.com/toozej/monogo/pkg/config"

// Config represents the monogo application configuration. Fields are tagged
// with the environment variable names used to populate them.
type Config struct {
	// Username specifies the username for application operations. It is loaded
	// from the USERNAME environment variable and defaults to empty.
	Username string `env:"USERNAME"`
}

// GetEnvVars loads and returns the application configuration, terminating the
// process via os.Exit on failure. It is retained for CLI entrypoints that load
// configuration during package initialization.
func GetEnvVars() Config {
	return sharedconfig.MustLoad[Config]()
}

// Load loads and returns the application configuration, returning any error to
// the caller instead of exiting.
func Load() (Config, error) {
	return sharedconfig.Load[Config]()
}
