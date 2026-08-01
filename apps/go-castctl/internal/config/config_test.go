package config

import (
	"testing"
	"time"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, key := range []string{"SERVER_HOST", "SERVER_PORT", "CASTOR_BINARY", "CASTOR_CONFIG", "CASTOR_TIMEOUT"} {
		t.Setenv(key, "")
	}
	// Empty environment variables intentionally override defaults, so remove
	// the keys after t.Setenv has registered restoration.
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("CASTOR_BINARY", "/opt/bin/castor")
	t.Setenv("CASTOR_CONFIG", "/tmp/castor.yaml")
	t.Setenv("CASTOR_TIMEOUT", "45s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.Address() != "127.0.0.1:9090" || cfg.Castor.Binary != "/opt/bin/castor" || cfg.Castor.Timeout != 45*time.Second {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidate(t *testing.T) {
	valid := Config{Server: Server{Port: 8080}, Castor: Castor{Binary: "castor", Timeout: time.Second}}
	tests := []struct {
		name string
		edit func(*Config)
	}{
		{name: "low port", edit: func(c *Config) { c.Server.Port = 0 }},
		{name: "high port", edit: func(c *Config) { c.Server.Port = 70000 }},
		{name: "missing binary", edit: func(c *Config) { c.Castor.Binary = "" }},
		{name: "invalid timeout", edit: func(c *Config) { c.Castor.Timeout = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.edit(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate returned nil error")
			}
		})
	}
}
