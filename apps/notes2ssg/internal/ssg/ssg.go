package ssg

import (
	"fmt"
	"os"
	"strings"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
)

// Formatter converts notes into static-site-generator-specific Markdown files.
type Formatter interface {
	Format(note backend.Note) string
	WriteFile(note backend.Note, outputDir string) error
}

// Slugify converts a title into a URL-friendly slug.
func Slugify(title string) string {
	var sb strings.Builder
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			sb.WriteRune(r + ('a' - 'A'))
		case r == ' ', r == '-', r == '_':
			sb.WriteRune('-')
		}
	}
	slug := sb.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	return slug
}

// WriteFileIfChanged writes the given content to path only when the file is
// missing or its contents differ. Prevents unnecessary filesystem churn.
func WriteFileIfChanged(path string, content string) error {
	if existing, err := os.ReadFile(path); err == nil && string(existing) == content { // #nosec G304
		fmt.Printf("The file '%s' already exists with the same content.\n", path)
		return nil
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil { // #nosec G306
		return fmt.Errorf("writing file %s: %w", path, err)
	}
	fmt.Printf("File '%s' has been written successfully.\n", path)
	return nil
}
