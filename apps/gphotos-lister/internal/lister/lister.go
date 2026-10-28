// Package lister writes filename lists and copies matching local files.
package lister

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AlbumReader reads original filenames of photos with album hearts.
type AlbumReader interface {
	LikedFilenames(context.Context, string) ([]string, error)
}

// Options controls the list and optional local copies.
type Options struct {
	Album       string `json:"album"`
	Prefix      string `json:"prefix"`
	Suffix      string `json:"suffix"`
	Copy        bool   `json:"copy"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	NoOverwrite bool   `json:"no_overwrite"`
}

// Result includes the list and the outcome of each local copy.
type Result struct {
	ListReady bool     `json:"list_ready"`
	Filenames []string `json:"filenames"`
	Text      string   `json:"text"`
	Copied    []string `json:"copied"`
	Missing   []string `json:"missing"`
	Ambiguous []string `json:"ambiguous"`
	Skipped   []string `json:"skipped"`
}

// Validate checks options before the reader requests the album.
func (o Options) Validate() error {
	if strings.TrimSpace(o.Album) == "" {
		return fmt.Errorf("enter a Google Photos album URL")
	}
	if strings.ContainsAny(o.Prefix+o.Suffix, "\r\n\x00") {
		return fmt.Errorf("prefix and suffix must not contain line breaks or NUL")
	}
	if o.Copy && (o.Source == "" || o.Destination == "") {
		return fmt.Errorf("copy requires source and destination directories")
	}
	return nil
}

// ValidFilename rejects paths and characters that can create extra list lines.
func ValidFilename(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\r\n\x00") && !filepath.IsAbs(name)
}

// Run reads the album and optionally copies local files by their original names.
func Run(ctx context.Context, reader AlbumReader, options Options) (Result, error) {
	result := Result{Filenames: []string{}, Copied: []string{}, Missing: []string{}, Ambiguous: []string{}, Skipped: []string{}}
	if err := options.Validate(); err != nil {
		return result, err
	}
	names, err := reader.LikedFilenames(ctx, options.Album)
	if err != nil {
		return result, fmt.Errorf("read album: %w", err)
	}
	seen := make(map[string]bool)
	var text strings.Builder
	for _, name := range names {
		if !ValidFilename(name) {
			return result, fmt.Errorf("album returned an invalid filename: %q", name)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		result.Filenames = append(result.Filenames, name)
		text.WriteString(options.Prefix + name + options.Suffix + "\n")
	}
	result.Text = text.String()
	result.ListReady = true
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if options.Copy {
		err = copyMatches(ctx, options, &result)
	}
	return result, err
}

// WriteText writes a complete list without truncating an existing file on failure.
func WriteText(path, text string) error {
	if strings.ToLower(filepath.Ext(path)) != ".txt" {
		return fmt.Errorf("output path must end with .txt")
	}
	return atomicWrite(path, func(file *os.File) error { _, err := file.WriteString(text); return err }, false)
}
