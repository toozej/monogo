// Package obsidian reads and writes Markdown files in an Obsidian vault.
package obsidian

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/notes-courier/internal/backend"
	"gopkg.in/yaml.v3"
)

const markerPrefix = "<!-- notes-courier:"

var inlineTag = regexp.MustCompile(`(?:^|\s)#([\pL\pN_/-]+)`)

// Client accesses notes in one local vault.
type Client struct{ vault, folder string }

// NewClient creates a vault client. The folder is relative to the vault root.
func NewClient(vault, folder string) (*Client, error) {
	if vault == "" {
		return nil, errors.New("OBSIDIAN_VAULT_PATH is required")
	}
	info, err := os.Stat(vault)
	if err != nil {
		return nil, fmt.Errorf("opening Obsidian vault: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("obsidian vault must be a directory")
	}
	folder = filepath.Clean(folder)
	if folder == "." {
		folder = ""
	}
	if filepath.IsAbs(folder) || folder == ".." || strings.HasPrefix(folder, ".."+string(filepath.Separator)) {
		return nil, errors.New("OBSIDIAN_FOLDER must stay inside the vault")
	}
	return &Client{vault: vault, folder: folder}, nil
}

// FetchNotes lists Markdown files in the vault. It skips Obsidian settings and hidden files.
func (c *Client) FetchNotes(ctx context.Context, tag string) ([]backend.Note, error) {
	var notes []backend.Note
	vaultRoot, err := os.OpenRoot(c.vault)
	if err != nil {
		return nil, fmt.Errorf("opening Obsidian vault root: %w", err)
	}
	defer func() { _ = vaultRoot.Close() }()
	err = filepath.WalkDir(c.vault, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == c.vault {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		rel, err := filepath.Rel(c.vault, path)
		if err != nil {
			return err
		}
		data, err := vaultRoot.ReadFile(rel)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		n, err := parseContent(string(data))
		if err != nil {
			return fmt.Errorf("reading properties in %s: %w", rel, err)
		}
		if tag != "" && !hasTag(n.Tags, tag) {
			return nil
		}
		n.ID, n.Source = filepath.ToSlash(rel), "obsidian"
		if n.Title == "" {
			n.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			for _, line := range strings.Split(n.Content, "\n") {
				if strings.HasPrefix(line, "# ") {
					n.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
					break
				}
			}
		}
		if n.Date.IsZero() {
			n.Date = info.ModTime()
		}
		notes = append(notes, n)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading Obsidian vault: %w", err)
	}
	return notes, nil
}

func parseContent(raw string) (backend.Note, error) {
	n := backend.Note{Content: raw}
	lines := strings.SplitAfter(raw, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) != "---" {
				continue
			}
			var properties struct {
				Title    string    `yaml:"title"`
				Created  time.Time `yaml:"created"`
				Modified time.Time `yaml:"modified"`
				Tags     yaml.Node `yaml:"tags"`
			}
			if err := yaml.Unmarshal([]byte(strings.Join(lines[1:i], "")), &properties); err != nil {
				return n, err
			}
			n.Title, n.Date, n.Modified = properties.Title, properties.Created, properties.Modified
			switch properties.Tags.Kind {
			case yaml.SequenceNode:
				if err := properties.Tags.Decode(&n.Tags); err != nil {
					return n, err
				}
			case yaml.ScalarNode:
				for _, tag := range strings.Split(properties.Tags.Value, ",") {
					if tag = strings.TrimSpace(tag); tag != "" {
						n.Tags = append(n.Tags, strings.TrimPrefix(tag, "#"))
					}
				}
			case 0:
			default:
				return n, errors.New("obsidian tags must be a list or a string")
			}
			n.Content = strings.Join(lines[i+1:], "")
			// The exporter puts one empty line between properties and the body.
			n.Content = strings.TrimPrefix(strings.TrimPrefix(n.Content, "\r\n"), "\n")
			break
		}
	}
	if start := strings.Index(n.Content, markerPrefix); start >= 0 {
		n.Content = strings.TrimSpace(n.Content[:start])
	}
	for _, match := range inlineTag.FindAllStringSubmatch(n.Content, -1) {
		if !hasTag(n.Tags, match[1]) {
			n.Tags = append(n.Tags, match[1])
		}
	}
	return n, nil
}

func hasTag(tags []string, tag string) bool {
	for _, value := range tags {
		if value == tag {
			return true
		}
	}
	return false
}

// ExportDirectory prepares the import folder inside the vault.
func (c *Client) ExportDirectory() (string, error) {
	root := filepath.Join(c.vault, c.folder)
	segment := c.vault
	for _, part := range strings.Split(c.folder, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		segment = filepath.Join(segment, part)
		info, err := os.Lstat(segment)
		if err == nil && !info.IsDir() {
			return "", fmt.Errorf("obsidian folder %q is not a directory", segment)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", fmt.Errorf("creating Obsidian folder: %w", err)
	}
	resolvedVault, err := filepath.EvalSymlinks(c.vault)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(resolvedVault, resolvedRoot)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("obsidian folder leaves the vault")
	}
	return root, nil
}
