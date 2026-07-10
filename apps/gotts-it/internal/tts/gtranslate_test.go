package tts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGtranslateChunk_SmallText(t *testing.T) {
	chunks := gtranslateChunk("Hello", 200)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "Hello" {
		t.Errorf("expected 'Hello', got %q", chunks[0])
	}
}

func TestGtranslateChunk_LargeText(t *testing.T) {
	text := strings.Repeat("This is a sentence. ", 50)
	chunks := gtranslateChunk(text, 200)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}
	total := 0
	for _, c := range chunks {
		total += len(c)
	}
	trimmedLen := len(strings.TrimSpace(text))
	if total < trimmedLen {
		t.Errorf("expected at least %d total chars, got %d", trimmedLen, total)
	}
}

func TestGtranslateChunk_EmptyText(t *testing.T) {
	chunks := gtranslateChunk("", 200)
	if chunks != nil {
		t.Errorf("expected nil for empty text, got %v", chunks)
	}
}

func TestGtranslateChunk_HardSplit(t *testing.T) {
	text := strings.Repeat("a", 500)
	chunks := gtranslateChunk(text, 200)
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}
	total := 0
	for _, c := range chunks {
		total += len(c)
	}
	if total != 500 {
		t.Errorf("expected total 500 chars, got %d", total)
	}
}

func TestGtranslateChunk_WithBoundary(t *testing.T) {
	text := "First sentence here. Second sentence here. Third one."
	chunks := gtranslateChunk(text, 30)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}
}

func TestFindGoogleTranslateBoundary_NoBoundary(t *testing.T) {
	text := strings.Repeat("x", 500)
	boundary := findGoogleTranslateBoundary(text, 200)
	if boundary != 0 {
		t.Errorf("expected 0 for no boundary, got %d", boundary)
	}
}

func TestFindGoogleTranslateBoundary_WithBoundary(t *testing.T) {
	text := "Hello world. This is a test. And more."
	boundary := findGoogleTranslateBoundary(text, len(text))
	if boundary == 0 {
		t.Error("expected non-zero boundary for text with sentence endings")
	}
}

func TestGtranslateRequestUsesHTTPS(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" {
			t.Fatalf("request scheme = %q, want https", req.URL.Scheme)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("audio")),
			Header:     make(http.Header),
		}, nil
	})}

	body, err := gtranslateFromText(context.Background(), client, "hello", "en")
	if err != nil {
		t.Fatal(err)
	}
	_ = body.Close()
}

func TestSynthesizeGoogleTranslate_EmptyText(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	opts := GoogleTranslateOptions{
		Lang:    "en",
		Timeout: 10 * time.Second,
	}

	err := SynthesizeGoogleTranslate(context.Background(), "", outputPath, opts)
	if err == nil {
		t.Error("expected error for empty text")
	}
}

func TestSynthesizeGoogleTranslate_WithMockServer(t *testing.T) {
	orig := GtranslateRequest
	defer func() { GtranslateRequest = orig }()

	GtranslateRequest = func(ctx context.Context, c *http.Client, text, lang string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("fake-gtranslate-audio")), nil
	}

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	opts := GoogleTranslateOptions{
		Lang:    "en",
		Timeout: 10 * time.Second,
	}

	err := SynthesizeGoogleTranslate(context.Background(), "Hello world", outputPath, opts)
	if err != nil {
		t.Fatalf("SynthesizeGoogleTranslate: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "fake-gtranslate-audio" {
		t.Errorf("expected 'fake-gtranslate-audio', got %q", string(data))
	}
}

func TestSynthesizeGoogleTranslate_Non2xx(t *testing.T) {
	orig := GtranslateRequest
	defer func() { GtranslateRequest = orig }()

	GtranslateRequest = func(ctx context.Context, c *http.Client, text, lang string) (io.ReadCloser, error) {
		return nil, fmt.Errorf("google translate tts returned status 403: forbidden")
	}

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	opts := GoogleTranslateOptions{
		Lang:    "en",
		Timeout: 10 * time.Second,
	}

	err := SynthesizeGoogleTranslate(context.Background(), "Hello world", outputPath, opts)
	if err == nil {
		t.Error("expected error for non-2xx response")
	}
}

func TestSynthesizeGoogleTranslateFailurePreservesExistingOutput(t *testing.T) {
	orig := GtranslateRequest
	defer func() { GtranslateRequest = orig }()
	requests := 0
	GtranslateRequest = func(ctx context.Context, c *http.Client, text, lang string) (io.ReadCloser, error) {
		requests++
		if requests == 1 {
			return io.NopCloser(strings.NewReader("partial")), nil
		}
		return nil, fmt.Errorf("temporary failure")
	}

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	if err := os.WriteFile(outputPath, []byte("existing-audio"), 0600); err != nil {
		t.Fatal(err)
	}
	err := SynthesizeGoogleTranslate(context.Background(), strings.Repeat("sentence. ", 50), outputPath, GoogleTranslateOptions{Timeout: time.Second})
	if err == nil {
		t.Fatal("expected synthesis to fail")
	}
	data, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "existing-audio" {
		t.Fatalf("failed synthesis replaced existing output with %q", data)
	}
}

func TestGtranslateChunkPreservesUTF8(t *testing.T) {
	text := strings.Repeat("界", gtranslateTextLimit+1)
	chunks := gtranslateChunk(text, gtranslateTextLimit)
	if len(chunks) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(chunks))
	}
	for i, chunk := range chunks {
		if strings.ToValidUTF8(chunk, "") != chunk {
			t.Fatalf("chunk %d contains invalid UTF-8", i)
		}
		if len([]rune(chunk)) > gtranslateTextLimit {
			t.Fatalf("chunk %d exceeds character limit", i)
		}
	}
	if strings.Join(chunks, "") != text {
		t.Fatal("chunking changed Unicode text")
	}
}

func TestSynthesizeGoogleTranslate_DefaultLang(t *testing.T) {
	var capturedLang string

	orig := GtranslateRequest
	defer func() { GtranslateRequest = orig }()

	GtranslateRequest = func(ctx context.Context, c *http.Client, text, lang string) (io.ReadCloser, error) {
		capturedLang = lang
		return io.NopCloser(strings.NewReader("audio")), nil
	}

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	opts := GoogleTranslateOptions{
		Timeout: 10 * time.Second,
	}

	err := SynthesizeGoogleTranslate(context.Background(), "Hello", outputPath, opts)
	if err != nil {
		t.Fatalf("SynthesizeGoogleTranslate: %v", err)
	}
	if capturedLang != "en" {
		t.Errorf("expected default lang 'en', got %q", capturedLang)
	}
}

func TestSynthesizeGoogleTranslate_ContextCancelled(t *testing.T) {
	orig := GtranslateRequest
	defer func() { GtranslateRequest = orig }()

	GtranslateRequest = func(ctx context.Context, c *http.Client, text, lang string) (io.ReadCloser, error) {
		return nil, fmt.Errorf("context canceled")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	opts := GoogleTranslateOptions{
		Lang:    "en",
		Timeout: 10 * time.Second,
	}

	err := SynthesizeGoogleTranslate(ctx, "Hello world", outputPath, opts)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}
