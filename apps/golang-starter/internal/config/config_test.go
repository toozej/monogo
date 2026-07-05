package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnvVars(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Ensure a clean environment for USERNAME and restore it afterward.
	_ = os.Unsetenv("USERNAME")
	t.Cleanup(func() { _ = os.Unsetenv("USERNAME") })

	// No env var set: Username should be empty.
	if conf := GetEnvVars(); conf.Username != "" {
		t.Errorf("expected empty Username, got %q", conf.Username)
	}

	// Env var set: Username should be populated.
	t.Setenv("USERNAME", "alice")
	if conf := GetEnvVars(); conf.Username != "alice" {
		t.Errorf("expected Username 'alice', got %q", conf.Username)
	}
}

func TestLoadWithDotEnv(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	_ = os.Unsetenv("USERNAME")
	t.Cleanup(func() { _ = os.Unsetenv("USERNAME") })

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("USERNAME=fromdotenv\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	conf, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if conf.Username != "fromdotenv" {
		t.Errorf("expected Username 'fromdotenv', got %q", conf.Username)
	}
}
