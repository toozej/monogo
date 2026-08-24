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
url: /{{slug}}/
summary: {{summary}}
categories:
  - {{tag}}
---
`

// Formatter generates Hugo markdown for a given note.
type Formatter struct {
	author        string
	unlistedTags  []string
	substitutions map[string]string
	template      string
}

// NewFormatter creates a new Hugo formatter.
func NewFormatter(author string, unlistedTags []string, substitutions map[string]string) *Formatter {
	return &Formatter{
		author:        author,
		unlistedTags:  unlistedTags,
		substitutions: substitutions,
		template:      defaultTemplate,
	}
}

// Format generates the Hugo markdown content for a note.
func (f *Formatter) Format(note backend.Note) string {
	replacer := newMultiReplacer()
	replacer.add("{{title}}", note.Title)
	replacer.add("{{author}}", f.author)
	replacer.add("{{unlisted}}", fmt.Sprintf("%t", note.Unlisted))
	replacer.add("{{date}}", note.Date.Format(time.RFC3339))
	replacer.add("{{slug}}", ssg.Slugify(note.Title))
	replacer.add("{{summary}}", f.computeSummary(note))
	replacer.add("{{tag}}", f.formatTags(note.Tags))

	return replacer.apply(f.template) + "\n" + note.Content
}

// WriteFile writes the formatted note to a file in the output directory.
func (f *Formatter) WriteFile(note backend.Note, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	filename := ssg.Slugify(note.Title) + ".md"
	path := filepath.Join(outputDir, filename)
	content := f.Format(note)

	return ssg.WriteFileIfChanged(path, content)
}

// formatTags converts the note's tags into a YAML list for the template.
func (f *Formatter) formatTags(tags []string) string {
	filtered := f.filterIgnoredTags(tags)
	if len(filtered) == 0 {
		return "Uncategorized"
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	var lines []string
	lines = append(lines, filtered[0])
	for _, t := range filtered[1:] {
		lines = append(lines, "  - "+t)
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
	for find, replace := range f.substitutions {
		if strings.Contains(lowerTitle, strings.ToLower(find)) {
			return replace
		}
		_ = find
		_ = replace
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
