// Package config provides reusable configuration-loading plumbing shared
// across monogo apps.
//
// This package owns the *mechanics* of configuration loading, not any
// app-specific configuration types. It:
//   - discovers and loads a .env file from the current working directory,
//     guarding against path traversal,
//   - optionally reads a YAML file as a base configuration layer, and
//   - parses environment variables into a caller-provided struct using
//     github.com/caarlos0/env.
//
// Each app declares its own Config struct (by convention under
// apps/<app>/internal/config) and uses this package to populate it. The
// loader is generic, so it never needs to know about app fields:
//
//	type Config struct {
//		Username string `env:"USERNAME"`
//	}
//
//	conf, err := config.Load[Config]()
//
// Loading order (later sources override earlier ones):
//  1. YAML file values (optional base layer, when WithYAMLFile is used)
//  2. .env file in the current working directory
//  3. Process environment variables and `envDefault` struct tags (highest priority)
//
// Note on YAML + `envDefault`: github.com/caarlos0/env applies `envDefault`
// values during Parse regardless of whether a field already holds a value, so
// a default will overwrite a value supplied via YAML. For fields that should
// be configurable through YAML, omit `envDefault` and apply the default in
// your own validation/defaulting code so YAML values are preserved.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Test seams: these indirections allow tests to exercise error paths that are
// otherwise difficult to trigger (working-directory and path-resolution
// failures, process exit).
var (
	osGetwd     = os.Getwd
	filepathAbs = filepath.Abs
	osExit      = os.Exit
)

// options holds the resolved configuration for a single load operation.
type options struct {
	loadDotEnv   bool
	dotEnvPath   string // when empty, defaults to "<cwd>/.env"
	yamlPath     string // when empty, no YAML file is read
	yamlRequired bool
}

func defaultOptions() options {
	return options{loadDotEnv: true}
}

// Option customizes configuration loading.
type Option func(*options)

// WithoutDotEnv disables .env discovery and loading. Use it for apps that
// should be configured purely from the process environment.
func WithoutDotEnv() Option {
	return func(o *options) { o.loadDotEnv = false }
}

// WithDotEnvPath overrides the .env file path. The path must resolve to a
// location within the current working directory.
func WithDotEnvPath(path string) Option {
	return func(o *options) { o.dotEnvPath = path }
}

// WithYAMLFile reads the given YAML file as a base configuration layer before
// environment parsing. A missing file is not an error and an empty path is
// ignored, which makes it convenient for "load config.yaml if it exists"
// behavior.
func WithYAMLFile(path string) Option {
	return func(o *options) {
		o.yamlPath = path
		o.yamlRequired = false
	}
}

// WithRequiredYAMLFile is like WithYAMLFile but returns an error if the file
// does not exist or cannot be read. Use it when a config file path was
// explicitly provided by the user.
func WithRequiredYAMLFile(path string) Option {
	return func(o *options) {
		o.yamlPath = path
		o.yamlRequired = true
	}
}

// Load builds a value of type T from the optional YAML layer, the .env file,
// and the process environment. T must be a struct type.
func Load[T any](opts ...Option) (T, error) {
	var target T
	if err := LoadInto(&target, opts...); err != nil {
		var zero T
		return zero, err
	}
	return target, nil
}

// MustLoad is like Load but prints the error to stderr and exits the process
// on failure. It exists mainly for CLI entrypoints that load configuration
// during package initialization and cannot propagate an error.
func MustLoad[T any](opts ...Option) T {
	conf, err := Load[T](opts...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		osExit(1)
	}
	return conf
}

// LoadInto populates the struct pointed to by target from the optional YAML
// layer, the .env file, and the process environment. target must be a non-nil
// pointer to a struct.
func LoadInto(target any, opts ...Option) error {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	if o.loadDotEnv {
		if err := loadDotEnv(o.dotEnvPath); err != nil {
			return err
		}
	}

	if o.yamlPath != "" {
		if err := loadYAMLFile(o.yamlPath, target, o.yamlRequired); err != nil {
			return err
		}
	}

	if err := env.Parse(target); err != nil {
		return fmt.Errorf("error parsing environment variables: %w", err)
	}

	return nil
}

// loadDotEnv discovers and loads a .env file, guarding against path traversal
// outside the current working directory. A missing file is not an error.
func loadDotEnv(path string) error {
	cwd, err := osGetwd()
	if err != nil {
		return fmt.Errorf("error getting current working directory: %w", err)
	}

	envPath := path
	if envPath == "" {
		envPath = filepath.Join(cwd, ".env")
	}

	// Ensure the resolved path stays within the current working directory.
	cleanEnvPath, err := filepathAbs(envPath)
	if err != nil {
		return fmt.Errorf("error resolving .env file path: %w", err)
	}
	cleanCwd, err := filepathAbs(cwd)
	if err != nil {
		return fmt.Errorf("error resolving current directory: %w", err)
	}
	relPath, err := filepath.Rel(cleanCwd, cleanEnvPath)
	if err != nil || strings.Contains(relPath, "..") {
		return fmt.Errorf(".env file path traversal detected")
	}

	// Load the .env file only if it exists; it will not override variables that
	// are already present in the environment.
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			return fmt.Errorf("error loading .env file: %w", err)
		}
	}

	return nil
}

// loadYAMLFile reads path and unmarshals its contents into target. The file is
// read through os.OpenRoot scoped to its parent directory to prevent the read
// from escaping that directory. When required is false a missing file is
// silently ignored.
func loadYAMLFile(path string, target any, required bool) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) && !required {
			return nil
		}
		return fmt.Errorf("failed to stat config file %s: %w", path, err)
	}

	// Resolve to an absolute path for consistent, predictable handling.
	absPath, err := filepathAbs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve config file path: %w", err)
	}

	// os.OpenRoot.ReadFile requires paths relative to the root directory;
	// absolute paths are rejected with "path escapes from parent".
	dir := filepath.Dir(absPath)
	name := filepath.Base(absPath)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("failed to create secure root filesystem: %w", err)
	}
	defer func() { _ = root.Close() }()

	data, err := root.ReadFile(name)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal YAML config: %w", err)
	}

	return nil
}
