package cmd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/gotts-it/internal/article"
	"github.com/toozej/monogo/apps/gotts-it/internal/config"
	"github.com/toozej/monogo/apps/gotts-it/internal/tts"
)

func TestNewServerCmd_ReturnsCommand(t *testing.T) {
	cmd := newServerCmd()
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
}

func TestNewServerCmd_HasCorrectUse(t *testing.T) {
	cmd := newServerCmd()
	if cmd.Use != "server" {
		t.Errorf("expected Use='server', got '%s'", cmd.Use)
	}
}

func TestNewServerCmd_NoArgs(t *testing.T) {
	cmd := newServerCmd()
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Errorf("expected no error with zero args, got: %v", err)
	}
}

func TestNewServerCmd_RejectsArgs(t *testing.T) {
	cmd := newServerCmd()
	if err := cmd.Args(cmd, []string{"extra"}); err == nil {
		t.Error("expected error when args provided, got nil")
	}
}

func TestProcessLine_WithFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("fake-audio")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	htmlContent := `<!DOCTYPE html><html><head><title>Server Test Article</title></head><body><p>Hello from server.</p></body></html>`
	tmpDir := t.TempDir()
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		OutputDir:     tmpDir,
		TTSBackend:    "openai",
		OpenAIBaseURL: srv.URL + "/v1",
		OpenAIToken:   "test-key",
		TTSModel:      "test-model",
		TTSVoice:      "test-voice",
		TTSFormat:     "mp3",
		TTSSpeed:      1.0,
		TTSTimeout:    10 * time.Second,
		FetchTimeout:  30 * time.Second,
	}

	if err := processLine(context.Background(), htmlFile); err != nil {
		t.Fatalf("processLine: %v", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}

	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".mp3") {
			found = true
			data, err := os.ReadFile(filepath.Join(tmpDir, e.Name()))
			if err != nil {
				t.Fatalf("read output file: %v", err)
			}
			if string(data) != "fake-audio" {
				t.Errorf("expected 'fake-audio', got %q", string(data))
			}
		}
	}
	if !found {
		t.Error("expected mp3 file in output directory")
	}
}

func TestProcessLine_WithInvalidPath(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		OutputDir:     t.TempDir(),
		TTSBackend:    "openai",
		OpenAIBaseURL: "http://localhost:8000/v1",
		OpenAIToken:   "test-key",
		TTSModel:      "test-model",
		TTSVoice:      "test-voice",
		TTSFormat:     "mp3",
		TTSSpeed:      1.0,
		TTSTimeout:    10 * time.Second,
		FetchTimeout:  30 * time.Second,
	}

	err := processLine(context.Background(), "/nonexistent/path/article.html")
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestProcessLine_WithGoogleBackend(t *testing.T) {
	orig := tts.GtranslateRequest
	defer func() { tts.GtranslateRequest = orig }()

	tts.GtranslateRequest = func(ctx context.Context, client *http.Client, text, lang string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("google-audio")), nil
	}

	htmlContent := `<!DOCTYPE html><html><head><title>Google Server Test</title></head><body><p>Hello from google.</p></body></html>`
	tmpDir := t.TempDir()
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		OutputDir:           tmpDir,
		TTSBackend:          "google",
		GoogleTranslateLang: "en",
		TTSFormat:           "mp3",
		TTSTimeout:          10 * time.Second,
		FetchTimeout:        30 * time.Second,
	}

	if err := processLine(context.Background(), htmlFile); err != nil {
		t.Fatalf("processLine with google: %v", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}

	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".mp3") {
			found = true
			data, err := os.ReadFile(filepath.Join(tmpDir, e.Name()))
			if err != nil {
				t.Fatalf("read output file: %v", err)
			}
			if string(data) != "google-audio" {
				t.Errorf("expected 'google-audio', got %q", string(data))
			}
		}
	}
	if !found {
		t.Error("expected mp3 file in output directory")
	}
}

func TestProcessLine_WithInvalidBackend(t *testing.T) {
	htmlContent := `<!DOCTYPE html><html><head><title>Invalid Backend</title></head><body><p>Hello.</p></body></html>`
	tmpDir := t.TempDir()
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		OutputDir:    tmpDir,
		TTSBackend:   "invalid",
		TTSFormat:    "mp3",
		TTSTimeout:   10 * time.Second,
		FetchTimeout: 30 * time.Second,
	}

	err := processLine(context.Background(), htmlFile)
	if err == nil {
		t.Error("expected error for invalid TTS backend")
	}
	if !strings.Contains(err.Error(), "unknown TTS backend") {
		t.Errorf("expected 'unknown TTS backend' in error, got %q", err.Error())
	}
}

func TestServerOutputPath_FromTitle(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{OutputDir: "/tmp/out"}

	art := article.Article{Title: "Hello World", Text: "test"}
	got := serverOutputPath(art, "mp3")
	expected := "/tmp/out/hello-world.mp3"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestServerOutputPathAvoidsExistingFile(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()
	dir := t.TempDir()
	conf = config.Config{OutputDir: dir}
	if err := os.WriteFile(filepath.Join(dir, "hello-world.mp3"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	got := serverOutputPath(article.Article{Title: "Hello World"}, "mp3")
	want := filepath.Join(dir, "hello-world-2.mp3")
	if got != want {
		t.Fatalf("serverOutputPath() = %q, want %q", got, want)
	}
}

func TestServerCmdRunEReturnsBatchFailures(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	_, _ = w.WriteString("# comment\n\n/nonexistent/article.html\n")
	_ = w.Close()

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{OutputDir: t.TempDir(), FetchTimeout: time.Second}

	err = serverCmdRunE(newServerCmd(), nil)
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("expected line-numbered batch failure, got %v", err)
	}
}

func TestServerOutputPath_FromURL(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{OutputDir: "/tmp/out"}

	art := article.Article{Title: "", Text: "test", URL: "https://example.com/article"}
	got := serverOutputPath(art, "mp3")
	expected := "/tmp/out/example-com-article.mp3"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestServerOutputPath_Default(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{OutputDir: "/tmp/out"}

	art := article.Article{Title: "", Text: "test", URL: ""}
	got := serverOutputPath(art, "mp3")
	expected := "/tmp/out/output.mp3"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestIsURL_HTTP(t *testing.T) {
	if !isURL("http://example.com") {
		t.Error("expected http:// URL to be true")
	}
}

func TestIsURL_HTTPS(t *testing.T) {
	if !isURL("https://example.com") {
		t.Error("expected https:// URL to be true")
	}
}

func TestIsURL_FilePath(t *testing.T) {
	if isURL("/path/to/file.html") {
		t.Error("expected file path to be false")
	}
}

func TestServerCmdRunE_EmptyStdin(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r
	if err := w.Close(); err != nil {
		t.Fatalf("close write end: %v", err)
	}

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		OutputDir:     t.TempDir(),
		TTSBackend:    "openai",
		OpenAIBaseURL: "http://localhost:8000/v1",
		OpenAIToken:   "test-key",
		TTSModel:      "test-model",
		TTSVoice:      "test-voice",
		TTSFormat:     "mp3",
		TTSSpeed:      1.0,
		TTSTimeout:    10 * time.Second,
		FetchTimeout:  30 * time.Second,
	}

	cmd := newServerCmd()
	if err := serverCmdRunE(cmd, []string{}); err != nil {
		t.Fatalf("serverCmdRunE with empty stdin: %v", err)
	}
}

func TestServerCmdRunE_WithOutputDir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("fake-audio")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	htmlContent := `<!DOCTYPE html><html><head><title>OutputDir Server Test</title></head><body><p>Hello.</p></body></html>`
	tmpDir := t.TempDir()
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "audio_out")

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r

	go func() {
		_, _ = w.Write([]byte(htmlFile + "\n"))
		_ = w.Close()
	}()

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		OutputDir:     outputDir,
		TTSBackend:    "openai",
		OpenAIBaseURL: srv.URL + "/v1",
		OpenAIToken:   "test-key",
		TTSModel:      "test-model",
		TTSVoice:      "test-voice",
		TTSFormat:     "mp3",
		TTSSpeed:      1.0,
		TTSTimeout:    10 * time.Second,
		FetchTimeout:  30 * time.Second,
	}

	cmd := newServerCmd()
	if err := serverCmdRunE(cmd, []string{}); err != nil {
		t.Fatalf("serverCmdRunE: %v", err)
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected output file in output directory")
	}
}

func TestServerCmdRunE_CommentLines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("fake-audio")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	htmlContent := `<!DOCTYPE html><html><head><title>Comment Test</title></head><body><p>Hello.</p></body></html>`
	tmpDir := t.TempDir()
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "audio_out")

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r

	go func() {
		_, _ = w.Write([]byte("# this is a comment\n" + htmlFile + "\n"))
		_ = w.Close()
	}()

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		OutputDir:     outputDir,
		TTSBackend:    "openai",
		OpenAIBaseURL: srv.URL + "/v1",
		OpenAIToken:   "test-key",
		TTSModel:      "test-model",
		TTSVoice:      "test-voice",
		TTSFormat:     "mp3",
		TTSSpeed:      1.0,
		TTSTimeout:    10 * time.Second,
		FetchTimeout:  30 * time.Second,
	}

	cmd := newServerCmd()
	if err := serverCmdRunE(cmd, []string{}); err != nil {
		t.Fatalf("serverCmdRunE: %v", err)
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected exactly 1 output file (comment line should be skipped), got %d", len(entries))
	}
}

func TestServerCmdRunE_EmptyLines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("fake-audio")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	htmlContent := `<!DOCTYPE html><html><head><title>Empty Line Test</title></head><body><p>Hello.</p></body></html>`
	tmpDir := t.TempDir()
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "audio_out")

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r

	go func() {
		_, _ = w.Write([]byte("\n\n" + htmlFile + "\n\n"))
		_ = w.Close()
	}()

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		OutputDir:     outputDir,
		TTSBackend:    "openai",
		OpenAIBaseURL: srv.URL + "/v1",
		OpenAIToken:   "test-key",
		TTSModel:      "test-model",
		TTSVoice:      "test-voice",
		TTSFormat:     "mp3",
		TTSSpeed:      1.0,
		TTSTimeout:    10 * time.Second,
		FetchTimeout:  30 * time.Second,
	}

	cmd := newServerCmd()
	if err := serverCmdRunE(cmd, []string{}); err != nil {
		t.Fatalf("serverCmdRunE: %v", err)
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected exactly 1 output file (empty lines should be skipped), got %d", len(entries))
	}
}

func TestNewServerCmd_IsCobraCommand(t *testing.T) {
	var _ = newServerCmd()
}
