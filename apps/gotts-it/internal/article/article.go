// Package article extracts readable text from a URL or a local HTML/text
// file using codeberg.org/readeck/go-readability/v2.
package article

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
)

// Source identifies where the article content should come from.
// Exactly one of URL or FilePath must be set.
type Source struct {
	URL      string
	FilePath string
}

// Article is the result of extracting readable content.
type Article struct {
	Title string
	Text  string
	URL   string
}

// FromSource reads the given source and returns the extracted article.
// When src.URL is set, the URL is fetched with the given timeout.
// When src.FilePath is set, the file is read from disk and parsed as HTML.
// Exactly one of URL or FilePath must be set.
func FromSource(ctx context.Context, src Source, timeout time.Duration) (Article, error) {
	if src.URL != "" {
		return fromURL(ctx, src.URL, timeout)
	}
	if src.FilePath != "" {
		return fromFile(src.FilePath)
	}
	return Article{}, fmt.Errorf("no source specified: provide exactly one of --url or --file")
}

func fromURL(ctx context.Context, rawURL string, timeout time.Duration) (Article, error) {
	doc, err := readability.FromURL(rawURL, timeout)
	if err != nil {
		return Article{}, fmt.Errorf("fetch article from %s: %w", rawURL, err)
	}

	var buf strings.Builder
	if err := doc.RenderText(&buf); err != nil {
		return Article{}, fmt.Errorf("render text from %s: %w", rawURL, err)
	}

	return Article{
		Title: doc.Title(),
		Text:  buf.String(),
		URL:   rawURL,
	}, nil
}

func fromFile(filePath string) (Article, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return Article{}, fmt.Errorf("resolve path %s: %w", filePath, err)
	}
	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return Article{}, fmt.Errorf("open root %s: %w", dir, err)
	}
	defer func() { _ = root.Close() }()

	f, err := root.Open(base)
	if err != nil {
		return Article{}, fmt.Errorf("open file %s: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	baseURL, _ := url.Parse("https://localhost/")
	doc, err := readability.FromReader(io.NopCloser(f), baseURL)
	if err != nil {
		return Article{}, fmt.Errorf("parse file %s: %w", filePath, err)
	}

	var buf strings.Builder
	if err := doc.RenderText(&buf); err != nil {
		return Article{}, fmt.Errorf("render text from file %s: %w", filePath, err)
	}

	return Article{
		Title: doc.Title(),
		Text:  buf.String(),
		URL:   filePath,
	}, nil
}
