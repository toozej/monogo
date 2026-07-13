// Package article extracts readable text from a URL or a local HTML/text
// file using codeberg.org/readeck/go-readability/v2.
package article

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
)

const maxArticleBytes = 10 << 20

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
	if (src.URL == "") == (src.FilePath == "") {
		return Article{}, fmt.Errorf("provide exactly one article URL or file path")
	}
	if src.URL != "" {
		return fromURL(ctx, src.URL, timeout)
	}
	return fromFile(src.FilePath)
}

func fromURL(ctx context.Context, rawURL string, timeout time.Duration) (Article, error) {
	if timeout <= 0 {
		return Article{}, fmt.Errorf("fetch timeout must be greater than zero")
	}
	pageURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return Article{}, fmt.Errorf("parse article URL %q: %w", rawURL, err)
	}
	if (pageURL.Scheme != "http" && pageURL.Scheme != "https") || pageURL.Host == "" {
		return Article{}, fmt.Errorf("article URL must use http or https and include a host")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL.String(), nil)
	if err != nil {
		return Article{}, fmt.Errorf("create article request: %w", err)
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return Article{}, fmt.Errorf("fetch article from %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Article{}, fmt.Errorf("fetch article from %s: unexpected HTTP status %s", rawURL, resp.Status)
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || (mediaType != "text/html" && mediaType != "application/xhtml+xml") {
		return Article{}, fmt.Errorf("fetch article from %s: response is not HTML", rawURL)
	}
	body, err := readLimited(resp.Body)
	if err != nil {
		return Article{}, fmt.Errorf("read article from %s: %w", rawURL, err)
	}
	doc, err := readability.FromReader(bytes.NewReader(body), pageURL)
	if err != nil {
		return Article{}, fmt.Errorf("parse article from %s: %w", rawURL, err)
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
	body, err := readLimited(f)
	if err != nil {
		return Article{}, fmt.Errorf("read file %s: %w", filePath, err)
	}
	doc, err := readability.FromReader(bytes.NewReader(body), baseURL)
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

func readLimited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxArticleBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxArticleBytes {
		return nil, fmt.Errorf("article exceeds %d-byte limit", maxArticleBytes)
	}
	return data, nil
}
