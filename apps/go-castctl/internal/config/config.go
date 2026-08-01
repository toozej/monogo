// Package config defines go-castctl's environment-backed configuration.
package config

import (
	"fmt"
	"net"
	"time"

	sharedconfig "github.com/toozej/monogo/pkg/config"
)

// Config is the complete go-castctl configuration.
type Config struct {
	Server Server `envPrefix:"SERVER_"`
	Castor Castor `envPrefix:"CASTOR_"`
}

// Server configures the HTTP listener and its timeouts.
type Server struct {
	Host         string        `env:"HOST" envDefault:"127.0.0.1"`
	Port         int           `env:"PORT" envDefault:"8080"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT" envDefault:"15s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"2m"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT" envDefault:"2m"`
}

// Castor configures the external Castor command.
type Castor struct {
	Binary     string        `env:"BINARY" envDefault:"castor"`
	ConfigPath string        `env:"CONFIG" envDefault:"config.yaml"`
	Timeout    time.Duration `env:"TIMEOUT" envDefault:"2m"`
}

// Address returns a net/http-compatible listener address.
func (s Server) Address() string {
	return net.JoinHostPort(s.Host, fmt.Sprintf("%d", s.Port))
}

// Validate rejects invalid or unexpectedly exposed listener settings.
func (c Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("SERVER_PORT must be between 1 and 65535")
	}
	if c.Castor.Binary == "" {
		return fmt.Errorf("CASTOR_BINARY cannot be empty")
	}
	if c.Castor.Timeout <= 0 {
		return fmt.Errorf("CASTOR_TIMEOUT must be positive")
	}
	return nil
}

// Load reads configuration from the environment and an optional .env file.
func Load() (Config, error) {
	return sharedconfig.Load[Config]()
}
