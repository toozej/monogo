// Package vite formats notes for the go-vite static site generator.
package vite

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
	"github.com/toozej/monogo/apps/notes2ssg/internal/ssg"
)

// defaultTemplate mirrors sn2ssg-py's templates/vite.md content so we avoid
// external template dependencies.
const defaultTemplate = `template:
  slug: {{slug}}
  title: {{title}}
  subtitle: {{subtitle}}
  date: {{date}}
`

// Formatter generates go-vite compatible markdown files.
type Formatter struct {
	defaultSubtitle string
	template        string
}

// NewFormatter constructs a vite formatter with the provided subtitle fallback.
func NewFormatter(defaultSubtitle string) *Formatter {
	return &Formatter{
		defaultSubtitle: defaultSubtitle,
		template:        defaultTemplate,
	}
}

// Format renders the note as go-vite markdown with front matter.
func (f *Formatter) Format(note backend.Note) string {
	slug := ssg.SlugFor(note)
	replacer := newMultiReplacer()
	replacer.add("{{slug}}", ssg.QuoteYAML(slug))
	replacer.add("{{title}}", ssg.QuoteYAML(note.Title))
	replacer.add("{{subtitle}}", ssg.QuoteYAML(f.subtitle(note)))
	replacer.add("{{date}}", ssg.QuoteYAML(note.Date.Format(time.DateOnly)))

	return replacer.apply(f.template) + noteBody(note.Content)
}

// WriteFile writes the formatted note to outputDir when content changes.
func (f *Formatter) WriteFile(note backend.Note, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	filename := ssg.SlugFor(note) + ".md"
	path := filepath.Join(outputDir, filename)
	content := f.Format(note)

	return ssg.WriteFileIfChanged(path, content)
}

func (f *Formatter) subtitle(note backend.Note) string {
	return f.defaultSubtitle
}

func noteBody(content string) string {
	body := strings.TrimLeft(content, "\n")
	if body == "" {
		return "\n"
	}
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return "\n" + body
}

type multiReplacer struct {
	pairs [][2]string
}

func newMultiReplacer() *multiReplacer {
	return &multiReplacer{pairs: make([][2]string, 0)}
}

func (m *multiReplacer) add(old, new string) {
	m.pairs = append(m.pairs, [2]string{old, new})
}

func (m *multiReplacer) apply(s string) string {
	for _, p := range m.pairs {
		s = strings.ReplaceAll(s, p[0], p[1])
	}
	return s
}
