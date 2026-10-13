// Package releases stores stable forge release assets and manages retention.
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
	ReleaseMatch string       `yaml:"release_match,omitempty"`
	AllAssets    bool         `yaml:"all_assets"`
	Keep         int          `yaml:"keep"`
	Retries      int          `yaml:"retries"`
	RequireAll   bool         `yaml:"require_all"`
	Targets      []Target     `yaml:"targets"`
	Repositories []Repository `yaml:"repositories"`
}

// Repository can override the global targets for one repository.
type Repository struct {
	Name         string   `yaml:"name"`
	ReleaseMatch string   `yaml:"release_match,omitempty"`
	Targets      []Target `yaml:"targets,omitempty"`
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

var tangledDID = regexp.MustCompile(`^did:plc:[a-z2-7]{24}$`)

var repoPart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// NormalizeRepository accepts owner/repo, host/path, or an HTTPS repository URL.
// Unqualified repositories use GitHub. Tangled host aliases use tangled.sh.
func NormalizeRepository(name string) (string, error) {
	host, path, err := repositoryParts(name)
	if err != nil {
		return "", err
	}
	if host == "github.com" {
		return strings.ToLower(path), nil
	}
	return host + "/" + path, nil
}

func repositoryParts(name string) (string, string, error) {
	name = strings.TrimSpace(name)
	host, path := "github.com", name
	if strings.Contains(name, "://") {
		u, err := url.Parse(name)
		if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" {
			return "", "", fmt.Errorf("invalid repository URL: %q", name)
		}
		host, path = strings.ToLower(u.Host), strings.Trim(u.Path, "/")
	} else if first, rest, ok := strings.Cut(name, "/"); ok && strings.Contains(first, ".") {
		// A two-component GitHub name can have a dot in its owner.
		if strings.Contains(rest, "/") || supportedHost(strings.ToLower(first)) {
			host, path = strings.ToLower(first), rest
		}
	}
	if !supportedHost(host) {
		return "", "", fmt.Errorf("unsupported forge hostname: %s", host)
	}
	if host == "tangled.org" {
		host = "tangled.sh"
	}
	path = strings.TrimSuffix(path, ".git")
	if host == "tangled.sh" {
		path = strings.TrimPrefix(path, "@")
	}
	parts := strings.Split(path, "/")
	if len(parts) < 2 || (host != "gitlab.com" && len(parts) != 2) {
		return "", "", fmt.Errorf("use [host/]owner/repo for repository %q", name)
	}
	for i, part := range parts {
		if host == "tangled.sh" && i == 0 && tangledDID.MatchString(part) {
			continue
		}
		if !repoPart.MatchString(part) || part == "." || part == ".." {
			return "", "", fmt.Errorf("invalid repository component: %q", part)
		}
	}
	return host, path, nil
}

func supportedHost(host string) bool {
	return host == "github.com" || host == "gitlab.com" || host == "codeberg.org" || host == "tangled.sh" || host == "tangled.org"
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
	if c.AllAssets && c.RequireAll {
		return errors.New("all-assets and require-all cannot be used together")
	}
	if _, err := regexp.Compile(c.ReleaseMatch); err != nil {
		return fmt.Errorf("invalid release filter: %w", err)
	}
	if len(c.Targets) == 0 && !c.AllAssets {
		return errors.New("specify at least one global target")
	}
	seen := map[string]bool{}
	for i := range c.Repositories {
		r := &c.Repositories[i]
		if _, err := regexp.Compile(r.ReleaseMatch); err != nil {
			return fmt.Errorf("invalid release filter for %s: %w", r.Name, err)
		}
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
