// Package slug converts article titles, URLs, and file paths into
// filesystem-safe filenames suitable for output audio file names.
package slug

import (
	"path/filepath"
	"regexp"
	"strings"
)

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// FromTitle returns a slug derived from the given title.
// It lowercases, replaces non-alphanumeric runs with "-",
// trims leading/trailing dashes, and caps the result at 80 characters.
func FromTitle(title string) string {
	s := strings.ToLower(title)
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
	}
	if s == "" {
		s = "untitled"
	}
	return s
}

// FromURL returns a slug derived from a URL by stripping the scheme
// and replacing non-alphanumeric characters with dashes.
func FromURL(u string) string {
	s := strings.TrimPrefix(u, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimRight(s, "/")
	s = strings.ToLower(s)
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
	}
	if s == "" {
		s = "untitled"
	}
	return s
}

// FromFilePath returns a slug derived from a file path by using the
// file's base name (without extension).
func FromFilePath(p string) string {
	base := filepath.Base(p)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return FromTitle(name)
}
