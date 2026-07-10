// Package tts provides text-to-speech synthesis via OpenAI-compatible
// endpoints or Google Translate's TTS API.
package tts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

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

	output, err := newAtomicOutput(outputPath)
	if err != nil {
		return err
	}
	defer output.abort()
	f := output.file

	client := &http.Client{Timeout: opts.Timeout}

	start := time.Now()
	for i, chunkText := range chunks {
		log.WithField("chunk", fmt.Sprintf("%d/%d", i+1, len(chunks))).Debugf("google translate tts %d chars", len(chunkText))

		audio, err := GtranslateRequest(ctx, client, chunkText, opts.Lang)
		if err != nil {
			return fmt.Errorf("google translate tts chunk %d/%d: %w", i+1, len(chunks), err)
		}

		if _, err := io.Copy(f, audio); err != nil {
			closeErr := audio.Close()
			if closeErr != nil {
				return fmt.Errorf("write chunk %d/%d: %w (close response: %v)", i+1, len(chunks), err, closeErr)
			}
			return fmt.Errorf("write chunk %d/%d: %w", i+1, len(chunks), err)
		}
		if err := audio.Close(); err != nil {
			return fmt.Errorf("close chunk %d/%d response: %w", i+1, len(chunks), err)
		}
	}
	if err := output.commit(); err != nil {
		return err
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
		"https://translate.google.com/translate_tts?ie=UTF-8&textlen=%d&client=tw-ob&q=%s&tl=%s",
		utf8.RuneCountInString(text), url.QueryEscape(text), lang,
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
	if max <= 0 || text == "" {
		return nil
	}
	remaining := []rune(text)
	if len(remaining) <= max {
		return []string{text}
	}

	var chunks []string
	for len(remaining) > max {
		prefix := string(remaining[:max])
		boundary := findGoogleTranslateBoundary(prefix, max)
		if boundary == 0 {
			chunks = append(chunks, prefix)
			remaining = []rune(strings.TrimSpace(string(remaining[max:])))
			continue
		}
		split := len([]rune(prefix[:boundary]))
		chunks = append(chunks, string(remaining[:split]))
		remaining = []rune(strings.TrimSpace(string(remaining[split:])))
	}
	if len(remaining) > 0 {
		chunks = append(chunks, string(remaining))
	}
	return chunks
}

func findGoogleTranslateBoundary(text string, max int) int {
	if max <= 0 {
		return 0
	}
	runes := []rune(text)
	if len(runes) > max {
		runes = runes[:max]
	}
	prefix := string(runes)

	separators := []string{". ", "! ", "? ", ", ", "; ", ": ", "\n"}
	best := 0
	for _, sep := range separators {
		idx := strings.LastIndex(prefix, sep)
		if idx > 0 {
			splitPoint := idx + len(sep)
			if splitPoint > best {
				best = splitPoint
			}
		}
	}

	if best == 0 {
		idx := strings.LastIndex(prefix, " ")
		if idx > 0 {
			best = idx + 1
		}
	}

	return best
}
