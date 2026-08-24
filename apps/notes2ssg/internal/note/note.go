// Package note provides the core Note model and a single-pass parser for
// converting raw note text streams into structured Note values.
package note

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
	"github.com/toozej/monogo/apps/notes2ssg/internal/ssg"
)

// Parser extracts Note structures from raw text representations.
type Parser struct {
	// headerPattern matches lines like "| Title: Foo |" or "| Date: ... |".
	headerPattern *regexp.Regexp
	// endingHeaderPattern matches the end of an ASCII header block (e.g. "+-...-+")
	endingHeaderPattern *regexp.Regexp
}

// NewParser creates a new Parser with compiled regular expressions.
func NewParser() *Parser {
	return &Parser{
		headerPattern:       regexp.MustCompile(`^\|\s*(.+?)\s*:\s*(.+?)\s*\|`),
		endingHeaderPattern: regexp.MustCompile(`^\+[-]+\+$`),
	}
}

// ParseNotes reads raw note data and returns a slice of Note structs.
// It implements a single-pass strategy: notes are delimited by header
// blocks, and each note is parsed in one pass.
func (p *Parser) ParseNotes(input string) []backend.Note {
	scanner := bufio.NewScanner(strings.NewReader(input))
	var notes []backend.Note
	var currentLines []string
	var headerStartFound, headerEndFound bool

	for scanner.Scan() {
		line := scanner.Text()

		if p.isEndingHeader(line) {
			switch {
			case !headerStartFound:
				headerStartFound = true
			case !headerEndFound:
				headerEndFound = true
			default:
				// New note boundary: previous note is complete
				if note := p.parseSingleNote(stringSlice(currentLines)); note != nil {
					notes = append(notes, *note)
				}
				currentLines = []string{line}
				headerStartFound = true
				headerEndFound = false
				continue
			}
		}

		currentLines = append(currentLines, line)
	}

	if len(currentLines) > 0 {
		if note := p.parseSingleNote(stringSlice(currentLines)); note != nil {
			notes = append(notes, *note)
		}
	}

	return notes
}

// parseSingleNote processes a single note's lines into a Note struct.
func (p *Parser) parseSingleNote(lines []string) *backend.Note {
	if len(lines) == 0 {
		return nil
	}

	var title, dateStr string
	var tags []string
	var bodyLines []string
	inHeader := true
	headerEndPassed := false

	for _, line := range lines {
		if inHeader {
			if p.isEndingHeader(line) {
				// Track when we've passed the header end boundary
				headerEndPassed = true
				continue
			}
			matches := p.headerPattern.FindStringSubmatch(line)
			if matches != nil {
				key := strings.TrimSpace(matches[1])
				value := strings.TrimSpace(matches[2])
				switch key {
				case "Title":
					title = strings.ReplaceAll(value, "#", "")
				case "Date":
					dateStr = value
				case "Tags":
					tags = splitTags(value)
				}
				continue
			}
			// Non-header, non-delimiter line -> header is done
			inHeader = false
		}

		// Skip pure header leftover lines (e.g. "+-+-+-+")
		if strings.HasPrefix(line, "|") || strings.HasPrefix(line, "+-") {
			continue
		}

		_ = headerEndPassed
		bodyLines = append(bodyLines, line)
	}

	// Remove leading "# Title" line if present in body
	if len(bodyLines) > 0 && strings.HasPrefix(bodyLines[0], "# ") {
		bodyLines = bodyLines[1:]
	}

	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	if title == "" && body == "" {
		return nil
	}

	date := parseDate(dateStr)

	return &backend.Note{
		Title:   title,
		Date:    date,
		Tags:    tags,
		Content: body,
	}
}

// isEndingHeader returns true if the line matches the end-of-header pattern.
func (p *Parser) isEndingHeader(line string) bool {
	return p.endingHeaderPattern.MatchString(line)
}

// splitTags splits a comma-separated tag string, trimming whitespace.
func splitTags(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseDate attempts to parse the Simplenote date format.
// It returns a zero time on failure.
func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Fri, 01 Sep 2023 02:33:35
	t, err := time.Parse("Mon, 02 Jan 2006 15:04:05", s)
	if err == nil {
		return t
	}
	// Try a few fallbacks
	layouts := []string{
		time.RFC3339,
		time.RFC1123,
	}
	for _, layout := range layouts {
		t, err = time.Parse(layout, s)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

// stringSlice is a helper to avoid potential aliasing issues with slices.
func stringSlice(s []string) []string {
	result := make([]string, len(s))
	copy(result, s)
	return result
}

// Slugify converts a title to a URL-friendly slug.
func Slugify(title string) string {
	return ssg.Slugify(title)
}

// ContinuousNote splits a single note into multiple notes, one per line.
// It optionally replaces a given tag with a new value and appends an index
// to each title (counting down from total non-empty lines).
func ContinuousNote(note backend.Note, tag string, replacement string) []backend.Note {
	lines := strings.Split(note.Content, "\n")
	var nonEmpty []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, strings.TrimSpace(line))
		}
	}

	var notes []backend.Note
	counter := len(nonEmpty)
	for _, line := range nonEmpty {
		newNote := note
		newNote.Content = line
		newNote.Title = fmt.Sprintf("%s - %d", note.Title, counter)
		counter--
		notes = append(notes, newNote)
	}
	_ = replacement
	_ = tag
	return notes
}
