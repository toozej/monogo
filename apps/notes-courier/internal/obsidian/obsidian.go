// Package obsidian reads and writes Markdown files in an Obsidian vault.
// It uses the same file operations as the notesmd-cli obsidian package.
package obsidian

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/notes-courier/internal/backend"
	"github.com/toozej/monogo/apps/notes-courier/internal/ssg"
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
		content, tags := parseContent(string(data))
		if tag != "" && !hasTag(tags, tag) {
			return nil
		}
		title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				break
			}
		}
		notes = append(notes, backend.Note{ID: filepath.ToSlash(rel), Title: title, Date: info.ModTime(), Tags: tags, Content: content})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading Obsidian vault: %w", err)
	}
	return notes, nil
}

func parseContent(raw string) (string, []string) {
	lines := strings.Split(raw, "\n")
	var tags []string
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		end := -1
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				end = i
				break
			}
		}
		if end > 0 {
			inTags := false
			for _, line := range lines[1:end] {
				trimmed := strings.TrimSpace(line)
				switch {
				case strings.HasPrefix(trimmed, "tags:"):
					inTags = true
					value := strings.TrimSpace(strings.TrimPrefix(trimmed, "tags:"))
					value = strings.Trim(value, "[]")
					for _, part := range strings.Split(value, ",") {
						part = strings.Trim(strings.TrimSpace(part), `"'`)
						if part != "" {
							tags = append(tags, strings.TrimPrefix(part, "#"))
						}
					}
				case inTags && strings.HasPrefix(trimmed, "- "):
					tags = append(tags, strings.Trim(strings.TrimPrefix(trimmed, "- "), `"'`))
				case trimmed != "":
					inTags = false
				}
			}
			lines = lines[end+1:]
		}
	}
	content := strings.TrimSpace(strings.Join(lines, "\n"))
	if start := strings.Index(content, markerPrefix); start >= 0 {
		content = strings.TrimSpace(content[:start])
	}
	for _, match := range inlineTag.FindAllStringSubmatch(content, -1) {
		if !hasTag(tags, match[1]) {
			tags = append(tags, match[1])
		}
	}
	return content, tags
}

func hasTag(tags []string, tag string) bool {
	for _, value := range tags {
		if value == tag {
			return true
		}
	}
	return false
}

// WriteNotes writes marked files under the configured folder. It does not replace unmarked files.
func (c *Client) WriteNotes(ctx context.Context, notes []backend.Note) error {
	root := filepath.Join(c.vault, c.folder)
	segment := c.vault
	for _, part := range strings.Split(c.folder, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		segment = filepath.Join(segment, part)
		info, err := os.Lstat(segment)
		if err == nil && !info.IsDir() {
			return fmt.Errorf("obsidian folder %q is not a directory", segment)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return fmt.Errorf("creating Obsidian folder: %w", err)
	}
	resolvedVault, err := filepath.EvalSymlinks(c.vault)
	if err != nil {
		return err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(resolvedVault, resolvedRoot)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("obsidian folder leaves the vault")
	}
	outputRoot, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("opening Obsidian destination root: %w", err)
	}
	defer func() { _ = outputRoot.Close() }()
	byMarker := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := outputRoot.ReadFile(rel)
		if err != nil {
			return err
		}
		if marker := findMarker(string(data)); marker != "" {
			if _, exists := byMarker[marker]; exists {
				return fmt.Errorf("duplicate Obsidian marker %q", marker)
			}
			byMarker[marker] = path
		}
		return nil
	}); err != nil {
		return fmt.Errorf("reading Obsidian destination: %w", err)
	}
	for _, n := range notes {
		if err := ctx.Err(); err != nil {
			return err
		}
		slug := ssg.SlugFor(n)
		if slug == "" {
			return fmt.Errorf("note %q has no output slug", n.Title)
		}
		path := filepath.Join(root, slug+".md")
		marker := noteMarker(n)
		if existingPath := byMarker[marker]; existingPath != "" {
			path = existingPath
		}
		content := strings.TrimSpace(n.Content)
		if n.Title != "" && !strings.HasPrefix(content, "# ") {
			content = "# " + n.Title + "\n\n" + content
		}
		if len(n.Tags) > 0 {
			for _, tag := range n.Tags {
				if tag != "" && !strings.Contains(content, "#"+tag) {
					content += "\n#" + strings.ReplaceAll(tag, " ", "-")
				}
			}
		}
		content = strings.TrimSpace(content) + "\n\n" + marker + "\n"
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("obsidian note %q is a symlink", path)
		}
		existing, err := os.ReadFile(path) // #nosec G304 -- path is under the configured vault.
		if err == nil {
			if string(existing) == content {
				continue
			}
			if !strings.Contains(string(existing), marker) {
				return fmt.Errorf("obsidian note %q exists without matching courier marker", path)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reading Obsidian note: %w", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return fmt.Errorf("writing Obsidian note: %w", err)
		} // #nosec G306
		byMarker[marker] = path
	}
	return nil
}

func findMarker(content string) string {
	start := strings.Index(content, markerPrefix)
	if start < 0 {
		return ""
	}
	end := strings.Index(content[start:], " -->")
	if end < 0 {
		return ""
	}
	return content[start : start+end+4]
}

func noteMarker(n backend.Note) string {
	id := n.ID
	if id == "" {
		id = n.Title + "\x00" + n.Date.UTC().Format(time.RFC3339Nano)
	}
	sum := sha256.Sum256([]byte(id))
	return fmt.Sprintf("%s%x -->", markerPrefix, sum[:12])
}
