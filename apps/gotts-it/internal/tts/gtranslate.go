// Package tts provides text-to-speech synthesis via OpenAI-compatible
// endpoints or Google Translate's TTS API.
package tts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const gtranslateTextLimit = 200

// GoogleTranslateOptions holds configuration for Google Translate TTS.
type GoogleTranslateOptions struct {
	Lang    string
	Timeout time.Duration
}

// SynthesizeGoogleTranslate sends text to Google Translate's TTS endpoint
// and writes the returned audio bytes to outputPath. Long inputs are
// chunked at sentence boundaries (<=200 chars per request, Google's limit)
// and the resulting audio segments are concatenated.
func SynthesizeGoogleTranslate(ctx context.Context, text, outputPath string, opts GoogleTranslateOptions) error {
	chunks := gtranslateChunk(text, gtranslateTextLimit)
	if len(chunks) == 0 {
		return fmt.Errorf("no text to synthesize")
	}

	if opts.Lang == "" {
		opts.Lang = "en"
	}

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve output path %s: %w", outputPath, err)
	}
	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("open root %s: %w", dir, err)
	}
	defer root.Close()

	f, err := root.Create(base)
	if err != nil {
		return fmt.Errorf("create output file %s: %w", outputPath, err)
	}
	defer f.Close()

	client := &http.Client{Timeout: opts.Timeout}

	start := time.Now()
	for i, chunkText := range chunks {
		log.WithField("chunk", fmt.Sprintf("%d/%d", i+1, len(chunks))).Debugf("google translate tts %d chars", len(chunkText))

		audio, err := GtranslateRequest(ctx, client, chunkText, opts.Lang)
		if err != nil {
			return fmt.Errorf("google translate tts chunk %d/%d: %w", i+1, len(chunks), err)
		}

		if _, err := io.Copy(f, audio); err != nil {
			_ = audio.Close()
			return fmt.Errorf("write chunk %d/%d: %w", i+1, len(chunks), err)
		}
		_ = audio.Close()
	}

	log.Infof("synthesized %d chunks via google translate in %s", len(chunks), time.Since(start).Round(time.Millisecond))
	return nil
}

// GtranslateRequest is the function used to fetch audio from the Google
// Translate TTS endpoint. It is exported so tests can replace it with a
// mock implementation.
var GtranslateRequest = gtranslateFromText

func gtranslateFromText(ctx context.Context, client *http.Client, text, lang string) (io.ReadCloser, error) {
	reqURL := fmt.Sprintf(
		"http://translate.google.com/translate_tts?ie=UTF-8&textlen=%d&client=tw-ob&q=%s&tl=%s",
		len(text), url.QueryEscape(text), lang,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "gotts-it/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google translate tts request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("google translate tts returned status %d: %s", resp.StatusCode, string(body))
	}

	return resp.Body, nil
}

// gtranslateChunk splits text into pieces of at most max characters,
// preferring sentence boundaries. Google Translate has a ~200 char limit.
func gtranslateChunk(text string, max int) []string {
	if len(text) == 0 {
		return nil
	}
	if len(text) <= max {
		return []string{text}
	}

	var chunks []string
	remaining := text
	for len(remaining) > max {
		boundary := findGoogleTranslateBoundary(remaining, max)
		if boundary == 0 {
			boundary = max
		}
		chunks = append(chunks, remaining[:boundary])
		remaining = strings.TrimSpace(remaining[boundary:])
	}
	if len(remaining) > 0 {
		chunks = append(chunks, remaining)
	}
	return chunks
}

func findGoogleTranslateBoundary(text string, max int) int {
	searchEnd := max
	if searchEnd > len(text) {
		searchEnd = len(text)
	}

	separators := []string{". ", "! ", "? ", ", ", "; ", ": ", "\n"}
	best := 0
	for _, sep := range separators {
		idx := strings.LastIndex(text[:searchEnd], sep)
		if idx > 0 {
			splitPoint := idx + len(sep)
			if splitPoint > best {
				best = splitPoint
			}
		}
	}

	if best == 0 {
		idx := strings.LastIndex(text[:searchEnd], " ")
		if idx > 0 {
			best = idx + 1
		}
	}

	return best
}
