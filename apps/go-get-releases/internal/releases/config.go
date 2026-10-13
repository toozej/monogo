// Package releases stores stable GitHub release assets and manages retention.
package releases

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config specifies the download directory, selection rules, and retention count.
type Config struct {
	Directory    string       `yaml:"directory"`
	Keep         int          `yaml:"keep"`
	Retries      int          `yaml:"retries"`
	RequireAll   bool         `yaml:"require_all"`
	Targets      []Target     `yaml:"targets"`
	Repositories []Repository `yaml:"repositories"`
}

// Repository can override the global targets for one repository.
type Repository struct {
	Name    string   `yaml:"name"`
	Targets []Target `yaml:"targets,omitempty"`
}

// Target selects assets by platform and format, or by an explicit filename rule.
// Match overrides platform detection. Formats and Exclude still apply.
type Target struct {
	OS      string   `yaml:"os"`
	Arch    string   `yaml:"arch"`
	Formats []string `yaml:"formats"`
	Match   string   `yaml:"match,omitempty"`
	Exclude string   `yaml:"exclude,omitempty"`
}

// Defaults selects the requested desktop platforms and Linux AppImages.
func Defaults() Config {
	return Config{Directory: "releases", Keep: 3, Retries: 2, Targets: []Target{
		{OS: "windows", Arch: "amd64", Formats: []string{"exe", "msi", "msix", "zip", "7z", "tar.gz"}},
		{OS: "darwin", Arch: "amd64", Formats: []string{"dmg", "pkg", "zip", "tar.gz", "tar.xz"}},
		{OS: "darwin", Arch: "arm64", Formats: []string{"dmg", "pkg", "zip", "tar.gz", "tar.xz"}},
		{OS: "linux", Arch: "amd64", Formats: []string{"appimage"}},
		{OS: "linux", Arch: "arm64", Formats: []string{"appimage"}},
	}}
}

// Load reads a YAML configuration and rejects unknown fields.
func Load(path string) (Config, error) {
	c := Defaults()
	f, err := os.Open(path) // #nosec G304 -- The user explicitly selects the configuration path.
	if err != nil {
		return c, err
	}
	defer func() { _ = f.Close() }()
	d := yaml.NewDecoder(f)
	d.KnownFields(true)
	if err := d.Decode(&c); err != nil {
		return c, fmt.Errorf("read configuration: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return c, errors.New("configuration must contain one YAML document")
	}
	return c, nil
}

var repoPart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// NormalizeRepository accepts owner/repo or an HTTPS GitHub repository URL.
func NormalizeRepository(name string) (string, error) {
	name = strings.TrimSpace(name)
	if strings.Contains(name, "://") {
		u, err := url.Parse(name)
		if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return "", fmt.Errorf("invalid GitHub repository: %q", name)
		}
		name = strings.Trim(u.Path, "/")
	}
	name = strings.TrimSuffix(name, ".git")
	parts := strings.Split(name, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("use owner/repo for repository %q", name)
	}
	for _, p := range parts {
		if !repoPart.MatchString(p) || p == "." || p == ".." {
			return "", fmt.Errorf("invalid repository component: %q", p)
		}
	}
	return strings.ToLower(name), nil
}

// Validate checks all options before the tool writes files or sends requests.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Directory) == "" {
		return errors.New("directory must not be empty")
	}
	if c.Keep < 1 {
		return errors.New("keep must be at least 1")
	}
	if c.Retries < 1 || c.Retries > 10 {
		return errors.New("retries must be between 1 and 10")
	}
	if len(c.Repositories) == 0 {
		return errors.New("specify at least one repository")
	}
	if len(c.Targets) == 0 {
		return errors.New("specify at least one global target")
	}
	seen := map[string]bool{}
	for i := range c.Repositories {
		r := &c.Repositories[i]
		name, err := NormalizeRepository(r.Name)
		if err != nil {
			return err
		}
		if seen[name] {
			return fmt.Errorf("duplicate repository: %s", name)
		}
		seen[name] = true
		r.Name = name
		for j := range r.Targets {
			if err := r.Targets[j].validate(); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	for i := range c.Targets {
		if err := c.Targets[i].validate(); err != nil {
			return err
		}
	}
	return nil
}

func (t *Target) validate() error {
	t.OS = normalizeOS(t.OS)
	t.Arch = normalizeArch(t.Arch)
	if t.OS != "windows" && t.OS != "darwin" && t.OS != "linux" {
		return fmt.Errorf("unsupported OS: %s", t.OS)
	}
	if t.Arch != "amd64" && t.Arch != "arm64" && t.Arch != "386" && t.Arch != "arm" {
		return fmt.Errorf("unsupported architecture: %s", t.Arch)
	}
	if len(t.Formats) == 0 {
		return errors.New("target must contain at least one format")
	}
	for i, f := range t.Formats {
		f = strings.ToLower(strings.TrimPrefix(f, "."))
		if !supportedFormats[f] {
			return fmt.Errorf("unsupported package format: %s", f)
		}
		t.Formats[i] = f
	}
	for _, pattern := range []string{t.Match, t.Exclude} {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid filename rule: %w", err)
		}
	}
	return nil
}

// ParseTarget accepts OS/ARCH:FORMAT+FORMAT for the command line.
func ParseTarget(value string) (Target, error) {
	platform, formats, ok := strings.Cut(value, ":")
	osName, arch, slash := strings.Cut(platform, "/")
	if !ok || !slash {
		return Target{}, errors.New("use OS/ARCH:FORMAT+FORMAT for a target")
	}
	t := Target{OS: osName, Arch: arch, Formats: strings.Split(formats, "+")}
	return t, t.validate()
}
