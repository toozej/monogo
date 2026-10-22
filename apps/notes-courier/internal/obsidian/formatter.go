// Package obsidian reads vaults and exports notes as Obsidian Markdown files.
package obsidian

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/toozej/monogo/apps/notes-courier/internal/backend"
	"github.com/toozej/monogo/apps/notes-courier/internal/ssg"
)

// Formatter writes Obsidian properties before the original Markdown content.
type Formatter struct{}

// NewFormatter creates an Obsidian formatter.
func NewFormatter() *Formatter { return &Formatter{} }

// Format preserves source metadata in flat YAML properties.
func (f *Formatter) Format(n backend.Note) string {
	var out strings.Builder
	out.WriteString("---\n")
	fmt.Fprintf(&out, "title: %s\naliases:\n  - %s\nsource: %s\n",
		ssg.QuoteYAML(n.Title), ssg.QuoteYAML(n.Title), ssg.QuoteYAML(n.Source))
	if n.Source == "simplenote" {
		fmt.Fprintf(&out, "simplenote_id: %s\n", ssg.QuoteYAML(n.ID))
	} else {
		fmt.Fprintf(&out, "source_id: %s\n", ssg.QuoteYAML(n.ID))
	}
	for _, date := range []struct {
		name  string
		value time.Time
	}{{"created", n.Date}, {"modified", n.Modified}} {
		if !date.value.IsZero() {
			fmt.Fprintf(&out, "%s: %s\n", date.name, date.value.UTC().Format(time.RFC3339Nano))
		}
	}
	writeList(&out, "tags", normalizeTags(n.Tags))
	if n.Source == "simplenote" {
		writeList(&out, "simplenote_tags", n.Tags)
		writeList(&out, "simplenote_system_tags", n.SystemTags)
	} else {
		writeList(&out, "source_tags", n.Tags)
	}
	out.WriteString("---\n\n")
	out.WriteString(n.Content)
	return out.String()
}

// WriteFile exports one note and retains other imports.
func (f *Formatter) WriteFile(n backend.Note, outputDir string) error {
	return f.Export(context.Background(), outputDir, []backend.Note{n}, false)
}

func writeList(out *strings.Builder, name string, values []string) {
	if len(values) == 0 {
		fmt.Fprintf(out, "%s: []\n", name)
		return
	}
	fmt.Fprintf(out, "%s:\n", name)
	for _, value := range values {
		fmt.Fprintf(out, "  - %s\n", ssg.QuoteYAML(value))
	}
}

// normalizeTags maps unsupported characters to hyphens and preserves raw tags separately.
func normalizeTags(tags []string) []string {
	var result []string
	seen := make(map[string]bool)
	for _, raw := range tags {
		var value strings.Builder
		for _, r := range strings.TrimLeft(strings.TrimSpace(raw), "#") {
			switch {
			case r == ':':
				value.WriteRune('/')
			case unicode.IsLetter(r), unicode.IsNumber(r), unicode.IsMark(r), r > unicode.MaxASCII && unicode.IsSymbol(r), r == '_', r == '-', r == '/':
				value.WriteRune(r)
			default:
				value.WriteRune('-')
			}
		}
		tag := strings.Trim(value.String(), "-/")
		if tag == "" {
			tag = "untagged"
		}
		if strings.TrimFunc(tag, unicode.IsNumber) == "" {
			tag = "tag-" + tag
		}
		key := strings.ToLower(tag)
		if !seen[key] {
			seen[key] = true
			result = append(result, tag)
		}
	}
	return result
}
