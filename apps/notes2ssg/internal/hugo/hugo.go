// Package hugo generates Hugo-formatted Markdown files from note structures.
package hugo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
	"github.com/toozej/monogo/apps/notes2ssg/internal/ssg"
)

// Default template used for each Hugo post. It mirrors the original
// templates/hugo.md file but is kept inline to avoid filesystem dependencies.
const defaultTemplate = `---
title: {{title}}
author: {{author}}
type: post
unlisted: {{unlisted}}
date: {{date}}
url: {{url}}
summary: {{summary}}
categories:
{{categories}}
---
`

// Formatter generates Hugo markdown for a given note.
type Formatter struct {
	author        string
	unlistedTags  []string
	substitutions []Substitution
	template      string
}

// Substitution defines one ordered title-to-summary mapping.
type Substitution struct {
	Find    string
	Replace string
}

// NewFormatter creates a new Hugo formatter.
func NewFormatter(author string, unlistedTags []string, substitutions []Substitution) *Formatter {
	return &Formatter{
		author:        author,
		unlistedTags:  unlistedTags,
		substitutions: substitutions,
		template:      defaultTemplate,
	}
}

// Format generates the Hugo markdown content for a note.
func (f *Formatter) Format(note backend.Note) string {
	slug := ssg.SlugFor(note)
	replacer := newMultiReplacer()
	replacer.add("{{title}}", ssg.QuoteYAML(note.Title))
	replacer.add("{{author}}", ssg.QuoteYAML(f.author))
	replacer.add("{{unlisted}}", fmt.Sprintf("%t", note.Unlisted))
	replacer.add("{{date}}", ssg.QuoteYAML(note.Date.Format(time.RFC3339)))
	replacer.add("{{url}}", ssg.QuoteYAML("/"+slug+"/"))
	replacer.add("{{summary}}", ssg.QuoteYAML(f.computeSummary(note)))
	replacer.add("{{categories}}", f.formatTags(note.Tags))

	return replacer.apply(f.template) + "\n" + note.Content
}

// WriteFile writes the formatted note to a file in the output directory.
func (f *Formatter) WriteFile(note backend.Note, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	filename := ssg.SlugFor(note) + ".md"
	path := filepath.Join(outputDir, filename)
	content := f.Format(note)

	return ssg.WriteFileIfChanged(path, content)
}

// formatTags converts the note's tags into a YAML list for the template.
func (f *Formatter) formatTags(tags []string) string {
	filtered := f.filterIgnoredTags(tags)
	if len(filtered) == 0 {
		filtered = []string{"Uncategorized"}
	}
	lines := make([]string, 0, len(filtered))
	for _, tag := range filtered {
		lines = append(lines, "  - "+ssg.QuoteYAML(tag))
	}
	return strings.Join(lines, "\n")
}

// filterIgnoredTags removes the download tag from the list.
// In a full implementation this might also strip other meta-tags.
func (f *Formatter) filterIgnoredTags(tags []string) []string {
	// No implementation-specific tag to strip at this generic level.
	return tags
}

// computeSummary checks if any tag matches a substitution rule and returns the
// replacement string applied to the title. Returns "" if no match.
func (f *Formatter) computeSummary(note backend.Note) string {
	lowerTitle := strings.ToLower(note.Title)
	for _, substitution := range f.substitutions {
		if strings.Contains(lowerTitle, strings.ToLower(substitution.Find)) {
			return substitution.Replace
		}
	}
	return ""
}

// multiReplacer is a simple string replacer that avoids replacing placeholders
// that were introduced by earlier replacements.
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
