package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnvVars(t *testing.T) {
	_ = os.Setenv("BACKEND", "simplenote")
	t.Cleanup(func() { _ = os.Unsetenv("BACKEND") })

	conf := GetEnvVars()
	if conf.Backend != "simplenote" {
		t.Errorf("expected Backend 'simplenote', got %q", conf.Backend)
	}
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	conf, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if conf.Backend != "simplenote" {
		t.Errorf("expected default Backend 'simplenote', got %q", conf.Backend)
	}
	if conf.SSGType != "hugo" {
		t.Errorf("expected default SSGType 'hugo', got %q", conf.SSGType)
	}
	if conf.Author != "root" {
		t.Errorf("expected default Author 'root', got %q", conf.Author)
	}
	if conf.PollingCycle != 3600 {
		t.Errorf("expected default PollingCycle 3600, got %d", conf.PollingCycle)
	}
	if conf.Debug != false {
		t.Errorf("expected default Debug false, got %v", conf.Debug)
	}
}

func TestLoadWithEnv(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	_ = os.Setenv("BACKEND", "usememos")
	_ = os.Setenv("MEMOS_URL", "http://memos.example.com")
	_ = os.Setenv("MEMOS_TOKEN", "token123")
	_ = os.Setenv("TAG_TO_DOWNLOAD", "blog")
	_ = os.Setenv("OUTPUT_DIR", "/tmp/out")
	_ = os.Setenv("AUTHOR", "alice")
	_ = os.Setenv("SSG_TYPE", "vite")
	_ = os.Setenv("VITE_SUBTITLE", "My Subtitle")
	t.Cleanup(func() {
		_ = os.Unsetenv("BACKEND")
		_ = os.Unsetenv("MEMOS_URL")
		_ = os.Unsetenv("MEMOS_TOKEN")
		_ = os.Unsetenv("TAG_TO_DOWNLOAD")
		_ = os.Unsetenv("OUTPUT_DIR")
		_ = os.Unsetenv("AUTHOR")
		_ = os.Unsetenv("SSG_TYPE")
		_ = os.Unsetenv("VITE_SUBTITLE")
	})

	conf, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if conf.Backend != "usememos" {
		t.Errorf("expected Backend 'usememos', got %q", conf.Backend)
	}
	if conf.MemosURL != "http://memos.example.com" {
		t.Errorf("expected MemosURL, got %q", conf.MemosURL)
	}
	if conf.MemosToken != "token123" {
		t.Errorf("expected MemosToken, got %q", conf.MemosToken)
	}
	if conf.TagToDownload != "blog" {
		t.Errorf("expected TagToDownload 'blog', got %q", conf.TagToDownload)
	}
	if conf.OutputDir != "/tmp/out" {
		t.Errorf("expected OutputDir '/tmp/out', got %q", conf.OutputDir)
	}
	if conf.Author != "alice" {
		t.Errorf("expected Author 'alice', got %q", conf.Author)
	}
	if conf.SSGType != "vite" {
		t.Errorf("expected SSGType 'vite', got %q", conf.SSGType)
	}
	if conf.ViteSubtitle != "My Subtitle" {
		t.Errorf("expected ViteSubtitle 'My Subtitle', got %q", conf.ViteSubtitle)
	}
}

func TestLoadWithDotEnv(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("BACKEND=usememos\nMEMOS_URL=http://memos.local\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	conf, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if conf.Backend != "usememos" {
		t.Errorf("expected Backend 'usememos', got %q", conf.Backend)
	}
	if conf.MemosURL != "http://memos.local" {
		t.Errorf("expected MemosURL, got %q", conf.MemosURL)
	}
}
