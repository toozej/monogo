// Package config loads app settings from the environment and .env files.
package config

import (
	"time"

	"github.com/toozej/monogo/apps/gphotos-lister/internal/lister"
	sharedconfig "github.com/toozej/monogo/pkg/config"
)

// Config holds the CLI and web settings.
type Config struct {
	Album       string        `env:"GPHOTOS_ALBUM"`
	Output      string        `env:"GPHOTOS_OUTPUT" envDefault:"liked-photos.txt"`
	Prefix      string        `env:"GPHOTOS_PREFIX"`
	Suffix      string        `env:"GPHOTOS_SUFFIX"`
	Copy        bool          `env:"GPHOTOS_COPY"`
	Source      string        `env:"GPHOTOS_SOURCE"`
	Destination string        `env:"GPHOTOS_DESTINATION"`
	NoOverwrite bool          `env:"GPHOTOS_NO_OVERWRITE"`
	Debug       bool          `env:"GPHOTOS_DEBUG"`
	Timeout     time.Duration `env:"GPHOTOS_TIMEOUT" envDefault:"15m"`
	WebHost     string        `env:"GPHOTOS_WEB_HOST" envDefault:"127.0.0.1"`
	WebPort     int           `env:"GPHOTOS_WEB_PORT" envDefault:"8080"`
	WebUsername string        `env:"GPHOTOS_WEB_USERNAME"`
	WebPassword string        `env:"GPHOTOS_WEB_PASSWORD"`
}

// Load reads the app configuration.
func Load() (Config, error) { return sharedconfig.Load[Config]() }

// Options returns the settings shared by the CLI and web UI.
func (c Config) Options() lister.Options {
	return lister.Options{Album: c.Album, Prefix: c.Prefix, Suffix: c.Suffix, Copy: c.Copy, Source: c.Source, Destination: c.Destination, NoOverwrite: c.NoOverwrite}
}
