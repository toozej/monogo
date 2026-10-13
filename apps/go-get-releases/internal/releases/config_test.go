package releases

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeRepository(t *testing.T) {
	for _, value := range []string{"Owner/Repo", "https://github.com/Owner/Repo.git", " Owner/Repo "} {
		got, err := NormalizeRepository(value)
		if err != nil || got != "owner/repo" {
			t.Fatalf("%q: %s, %v", value, got, err)
		}
	}
	for _, value := range []string{"repo", "a/b/c", "a/..", "../repo", "a/a b", "http://github.com/a/b", "https://evil.test/a/b", "https://github.com/a/b?x=y", "https://github.com/a/b#x", "https://user@github.com/a/b", "https://github.com:443/a/b"} {
		if _, err := NormalizeRepository(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Config)
	}{
		{"directory", func(c *Config) { c.Directory = "" }},
		{"keep", func(c *Config) { c.Keep = 0 }},
		{"retry", func(c *Config) { c.Retries = 0 }},
		{"excess retries", func(c *Config) { c.Retries = 11 }},
		{"no repositories", func(c *Config) { c.Repositories = nil }},
		{"duplicate", func(c *Config) { c.Repositories = append(c.Repositories, Repository{Name: "OWNER/REPO"}) }},
		{"invalid repository", func(c *Config) { c.Repositories[0].Name = ".." }},
		{"targets", func(c *Config) { c.Targets = nil }},
		{"OS", func(c *Config) { c.Targets[0].OS = "other" }},
		{"architecture", func(c *Config) { c.Targets[0].Arch = "mips" }},
		{"formats", func(c *Config) { c.Targets[0].Formats = nil }},
		{"format", func(c *Config) { c.Targets[0].Formats = []string{"doc"} }},
		{"match", func(c *Config) { c.Targets[0].Match = "[" }},
		{"exclude", func(c *Config) { c.Targets[0].Exclude = "[" }},
		{"repository target", func(c *Config) { c.Repositories[0].Targets = []Target{{OS: "invalid"}} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Defaults()
			c.Repositories = []Repository{{Name: "owner/repo"}}
			tc.change(&c)
			if err := c.Validate(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	c := Defaults()
	c.Repositories = []Repository{{Name: "OWNER/REPO"}}
	c.Targets[0].OS = "win"
	c.Targets[0].Arch = "x86_64"
	c.Targets[0].Formats = []string{".ZIP"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.Repositories[0].Name != "owner/repo" || c.Targets[0].OS != "windows" || c.Targets[0].Arch != "amd64" || c.Targets[0].Formats[0] != "zip" {
		t.Fatal("options were not normalized")
	}
}

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	for _, tc := range []struct {
		data  string
		valid bool
	}{
		{"repositories:\n  - name: owner/repo\n", true},
		{"unknown: true\n", false}, {"keep: [\n", false}, {"keep: 3\n---\nkeep: 4\n", false}, {"", false},
	} {
		if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
			t.Fatal(err)
		}
		c, err := Load(path)
		if (err == nil) != tc.valid {
			t.Fatalf("%q: %v", tc.data, err)
		}
		if tc.valid && (c.Keep != 3 || len(c.Targets) != 5 || len(c.Repositories) != 1) {
			t.Fatal(c)
		}
	}
	if _, err := Load(path + ".missing"); err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestParseTarget(t *testing.T) {
	got, err := ParseTarget("macos/aarch64:dmg+zip")
	if err != nil || got.OS != "darwin" || got.Arch != "arm64" || strings.Join(got.Formats, ",") != "dmg,zip" {
		t.Fatalf("%+v %v", got, err)
	}
	for _, value := range []string{"linux", "linux:zip", "linux/amd64", "other/amd64:zip", "linux/amd64:bad", "linux/amd64:"} {
		if _, err := ParseTarget(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}
