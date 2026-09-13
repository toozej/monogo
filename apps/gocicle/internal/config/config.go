// Package config loads control plane and runner settings.
package config

import (
	"errors"
	"net/netip"
	"net/url"
	"time"

	shared "github.com/toozej/monogo/pkg/config"
)

type Config struct {
	DatabaseURL       string        `env:"GOCICLE_DATABASE_URL"`
	KeyFile           string        `env:"GOCICLE_KEY_FILE"`
	Listen            string        `env:"GOCICLE_LISTEN" envDefault:"127.0.0.1:8080"`
	PublicURL         string        `env:"GOCICLE_PUBLIC_URL" envDefault:"https://gocicle.example.com"`
	APIURL            string        `env:"GOCICLE_API_URL"`
	Token             string        `env:"GOCICLE_TOKEN"`
	Runtime           string        `env:"GOCICLE_RUNTIME" envDefault:"docker"`
	Socket            string        `env:"GOCICLE_SOCKET" envDefault:"/var/run/docker.sock"`
	StateDir          string        `env:"GOCICLE_STATE_DIR" envDefault:"/var/lib/gocicle-runner"`
	Retention         time.Duration `env:"GOCICLE_RETENTION" envDefault:"720h"`
	NotificationHosts []string      `env:"GOCICLE_NOTIFICATION_HOSTS" envSeparator:","`
	TrustedProxies    []string      `env:"GOCICLE_TRUSTED_PROXIES" envSeparator:","`
}

func Load() (Config, error) { return shared.Load[Config]() }
func (c Config) ValidateServer() error {
	if c.DatabaseURL == "" || c.KeyFile == "" {
		return errors.New("set GOCICLE_DATABASE_URL and GOCICLE_KEY_FILE")
	}
	u, err := url.Parse(c.PublicURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("GOCICLE_PUBLIC_URL must be an HTTPS origin")
	}
	for _, value := range c.TrustedProxies {
		if _, err := netip.ParsePrefix(value); err != nil {
			return errors.New("trusted proxies must be CIDR prefixes")
		}
	}
	if c.Retention < time.Hour {
		return errors.New("retention must be at least one hour")
	}
	return nil
}
