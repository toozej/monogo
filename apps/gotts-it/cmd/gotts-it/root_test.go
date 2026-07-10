package cmd

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/gotts-it/internal/article"
	"github.com/toozej/monogo/apps/gotts-it/internal/config"
	"github.com/toozej/monogo/apps/gotts-it/internal/tts"
)

func TestRootCmdStructure(t *testing.T) {
	if rootCmd.Use != "gotts-it" {
		t.Errorf("expected Use='gotts-it', got '%s'", rootCmd.Use)
	}
	if rootCmd.RunE == nil {
		t.Error("expected RunE to be set, got nil")
	}
	if rootCmd.PersistentPreRun == nil {
		t.Error("expected PersistentPreRun to be set, got nil")
	}
}

func TestRootCmdExactArgs(t *testing.T) {
	if err := rootCmd.Args(rootCmd, []string{}); err != nil {
		t.Errorf("expected no error with zero args, got: %v", err)
	}
	if err := rootCmd.Args(rootCmd, []string{"extra"}); err == nil {
		t.Error("expected error when args provided, got nil")
	}
}

func TestRootCmdHasSubcommands(t *testing.T) {
	subcommandNames := map[string]bool{}
	for _, cmd := range rootCmd.Commands() {
		subcommandNames[cmd.Name()] = true
	}

	for _, name := range []string{"man", "version", "server"} {
		if !subcommandNames[name] {
			t.Errorf("expected subcommand '%s' to be registered", name)
		}
	}
}

func TestRootCmdPersistentFlags(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("debug")
	if flag == nil {
		t.Fatal("expected persistent flag 'debug' to be registered")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected debug flag default 'false', got '%s'", flag.DefValue)
	}
	if flag.Shorthand != "d" {
		t.Errorf("expected debug flag shorthand 'd', got '%s'", flag.Shorthand)
	}
}

func TestRootCmdLocalFlags(t *testing.T) {
	tests := []struct {
		name      string
		flagName  string
		shorthand string
	}{
		{"url flag", "url", "U"},
		{"file flag", "file", "f"},
		{"output flag", "output", "o"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := rootCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("expected local flag '%s' to be registered", tt.flagName)
			}
			if tt.shorthand != "" && flag.Shorthand != tt.shorthand {
				t.Errorf("expected flag '%s' shorthand '%s', got '%s'", tt.flagName, tt.shorthand, flag.Shorthand)
			}
		})
	}
}

func TestRootCmdLocalFlagsNoShorthand(t *testing.T) {
	for _, name := range []string{} {
		t.Run(name+" flag", func(t *testing.T) {
			flag := rootCmd.Flags().Lookup(name)
			if flag == nil {
				t.Fatalf("expected local flag '%s' to be registered", name)
			}
		})
	}
}

func TestRootCmdPersistentFlagsNoShorthand(t *testing.T) {
	for _, name := range []string{"format", "voice", "model", "speed", "instructions", "fetch-timeout", "tts-timeout", "output-dir", "tts-backend", "lang"} {
		t.Run(name+" flag", func(t *testing.T) {
			flag := rootCmd.PersistentFlags().Lookup(name)
			if flag == nil {
				t.Fatalf("expected persistent flag '%s' to be registered", name)
			}
		})
	}
}

func TestRootCmdPreRun_DebugFalse(t *testing.T) {
	origDebug := debug
	debug = false
	defer func() { debug = origDebug }()

	rootCmdPreRun(rootCmd, []string{})
}

func TestRootCmdPreRun_DebugTrue(t *testing.T) {
	origDebug := debug
	debug = true
	defer func() { debug = origDebug }()

	rootCmdPreRun(rootCmd, []string{})
}

func TestRootCmdRequiresURLOrFile(t *testing.T) {
	rootCmd.SetArgs([]string{})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when neither --url nor --file provided")
	}
}

func TestRootCmdURLAndFileMutuallyExclusive(t *testing.T) {
	rootCmd.SetArgs([]string{"--url", "https://example.com", "--file", "article.html"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when both --url and --file provided")
	}
}

func TestRootPreRunAcceptsEnvironmentConfiguredSource(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		URL: "https://example.com/article", TTSBackend: "openai", TTSFormat: "mp3",
		TTSSpeed: 1, TTSTimeout: time.Second, FetchTimeout: time.Second,
	}

	cmd := &cobra.Command{Use: "gotts-it"}
	if err := rootCmdPreRunE(cmd, nil); err != nil {
		t.Fatalf("effective environment configuration was rejected: %v", err)
	}
}

func TestRootPreRunRejectsAmbiguousEnvironmentSources(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		URL: "https://example.com/article", File: "article.html", TTSBackend: "openai", TTSFormat: "mp3",
		TTSSpeed: 1, TTSTimeout: time.Second, FetchTimeout: time.Second,
	}

	cmd := &cobra.Command{Use: "gotts-it"}
	if err := rootCmdPreRunE(cmd, nil); err == nil {
		t.Fatal("expected ambiguous source configuration to fail")
	}
}

func TestRootPreRunRejectsNonMP3GoogleOutput(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		URL: "https://example.com/article", TTSBackend: "google", TTSFormat: "wav",
		TTSSpeed: 1, TTSTimeout: time.Second, FetchTimeout: time.Second,
	}

	cmd := &cobra.Command{Use: "gotts-it"}
	if err := rootCmdPreRunE(cmd, nil); err == nil {
		t.Fatal("expected Google Translate WAV output to be rejected")
	}
}

func TestURLFlagParsing(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()

	flag := rootCmd.Flags().Lookup("url")
	if flag == nil {
		t.Fatal("expected 'url' flag to exist")
	}
}

func TestOutputFlagParsing(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()

	flag := rootCmd.Flags().Lookup("output")
	if flag == nil {
		t.Fatal("expected 'output' flag to exist")
	}
}

func TestFormatFlagParsing(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("format")
	if flag == nil {
		t.Fatal("expected 'format' flag to exist")
	}
	if flag.DefValue != "mp3" {
		t.Errorf("expected format default 'mp3', got '%s'", flag.DefValue)
	}
}

func TestVoiceFlagParsing(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("voice")
	if flag == nil {
		t.Fatal("expected 'voice' flag to exist")
	}
}

func TestModelFlagParsing(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("model")
	if flag == nil {
		t.Fatal("expected 'model' flag to exist")
	}
}

func TestSpeedFlagParsing(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("speed")
	if flag == nil {
		t.Fatal("expected 'speed' flag to exist")
	}
}

func TestVersionSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"version"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("version subcommand execution failed: %v", err)
	}

	if stdout.String() == "" {
		t.Error("expected version subcommand to produce output")
	}
}

func TestManSubcommand(t *testing.T) {
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"man"})
	err := rootCmd.Execute()

	_ = w.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("man subcommand execution failed: %v", err)
	}
}

func TestRootCmdRejectsArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"invalid-arg"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when invalid args provided")
	}
	rootCmd.SetArgs([]string{})
}

func TestDebugFlagParsing(t *testing.T) {
	origDebug := debug
	defer func() { debug = origDebug }()

	rootCmd.SetArgs([]string{"-d", "version"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error with debug flag, got: %v", err)
	}
	rootCmd.SetArgs([]string{})
}

func TestDefaultOutputPath(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		url      string
		format   string
		expected string
	}{
		{"from title", "Readability", "", "mp3", "readability.mp3"},
		{"from url", "", "https://en.wikipedia.org/wiki/Readability", "mp3", "en-wikipedia-org-wiki-readability.mp3"},
		{"wav format", "My Article", "", "wav", "my-article.wav"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			art := article.Article{Title: tt.title, Text: "test", URL: tt.url}
			got := defaultOutputPath(art, tt.format)
			if !strings.HasSuffix(got, "."+tt.format) {
				t.Errorf("expected output to end with .%s, got %s", tt.format, got)
			}
		})
	}
}

func TestRootCmdIsCobraCommand(t *testing.T) {
	var _ = rootCmd
}

func TestRootCmdRun_WithFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("fake-audio")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	htmlContent := `<!DOCTYPE html><html><head><title>Test Run Article</title></head><body><p>Hello from file.</p></body></html>`
	tmpDir := t.TempDir()
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "output.mp3")

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		File:          htmlFile,
		Output:        outputPath,
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

	if err := rootCmdRunE(rootCmd, []string{}); err != nil {
		t.Fatalf("rootCmdRunE: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "fake-audio" {
		t.Errorf("expected output 'fake-audio', got %q", string(data))
	}
}

func TestRootCmdRun_WithURLError(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		URL:           "http://localhost:1/fail",
		TTSBackend:    "openai",
		OpenAIBaseURL: "http://localhost:1/v1",
		OpenAIToken:   "test-key",
		TTSModel:      "test-model",
		TTSVoice:      "test-voice",
		TTSFormat:     "mp3",
		TTSSpeed:      1.0,
		TTSTimeout:    1 * time.Second,
		FetchTimeout:  1 * time.Second,
	}

	err := rootCmdRunE(rootCmd, []string{})
	if err == nil {
		t.Error("expected error for unreachable URL, got nil")
	}
}

func TestRootCmdRun_TTSError(t *testing.T) {
	htmlContent := `<!DOCTYPE html><html><head><title>TTSError</title></head><body><p>Text.</p></body></html>`
	tmpDir := t.TempDir()
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("error")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		File:          htmlFile,
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

	err := rootCmdRunE(rootCmd, []string{})
	if err == nil {
		t.Error("expected error for TTS failure, got nil")
	}
}

func TestDefaultOutputPath_WithFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "my-article.html")
	if err := os.WriteFile(fpath, []byte("test"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	art := article.Article{Title: "", Text: "test", URL: fpath}
	got := defaultOutputPath(art, "mp3")
	if !strings.HasSuffix(got, ".mp3") {
		t.Errorf("expected .mp3 suffix, got %s", got)
	}
	if !strings.Contains(got, "my-article") {
		t.Errorf("expected slug from file path to contain 'my-article', got %s", got)
	}
}

func TestDefaultOutputPath_NoTitleNoURL(t *testing.T) {
	art := article.Article{Title: "", Text: "test", URL: ""}
	got := defaultOutputPath(art, "mp3")
	if got != "output.mp3" {
		t.Errorf("expected 'output.mp3', got %q", got)
	}
}

func TestDefaultOutputPath_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	candidate := filepath.Join(tmpDir, "readability.mp3")
	if err := os.WriteFile(candidate, []byte("existing"), 0644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Logf("warning: chdir back: %v", err)
		}
	}()

	art := article.Article{Title: "Readability", Text: "test", URL: ""}
	got := defaultOutputPath(art, "mp3")
	if got == "readability.mp3" {
		t.Error("expected different filename since readability.mp3 already exists")
	}
	if !strings.HasSuffix(got, ".mp3") {
		t.Errorf("expected .mp3 suffix, got %s", got)
	}
}

func TestDefaultOutputPathChecksConfiguredDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readability.mp3"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	got := defaultOutputPathInDir(article.Article{Title: "Readability"}, "mp3", dir)
	want := filepath.Join(dir, "readability-2.mp3")
	if got != want {
		t.Fatalf("defaultOutputPathInDir() = %q, want %q", got, want)
	}
}

func TestExecute_VersionSubcommand(t *testing.T) {
	rootCmd.SetArgs([]string{"version"})
	defer rootCmd.SetArgs([]string{})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	origConf := conf
	defer func() { conf = origConf }()

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}
}

func TestRootCmdRun_WithURLOnServer(t *testing.T) {
	htmlContent := `<!DOCTYPE html><html><head><title>URL Article</title></head><body><p>From URL.</p></body></html>`
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(htmlContent)); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer httpSrv.Close()

	ttsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("audio-data")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer ttsSrv.Close()

	outputPath := filepath.Join(t.TempDir(), "output.mp3")

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		URL:           httpSrv.URL,
		Output:        outputPath,
		TTSBackend:    "openai",
		OpenAIBaseURL: ttsSrv.URL + "/v1",
		OpenAIToken:   "test-key",
		TTSModel:      "test-model",
		TTSVoice:      "test-voice",
		TTSFormat:     "mp3",
		TTSSpeed:      1.0,
		TTSTimeout:    10 * time.Second,
		FetchTimeout:  10 * time.Second,
	}

	if err := rootCmdRunE(rootCmd, []string{}); err != nil {
		t.Fatalf("rootCmdRunE: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "audio-data" {
		t.Errorf("expected 'audio-data', got %q", string(data))
	}
}

func TestOutputDirFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("output-dir")
	if flag == nil {
		t.Fatal("expected 'output-dir' flag to exist")
	}
}

func TestDefaultOutputPath_WithOutputDir(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()

	tmpDir := t.TempDir()
	htmlContent := `<!DOCTYPE html><html><head><title>OutputDir Article</title></head><body><p>Hello.</p></body></html>`
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "audio_output")

	conf = config.Config{
		File:          htmlFile,
		OutputDir:     outputDir,
		Output:        "",
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

	outputPathResult := conf.Output
	if outputPathResult == "" {
		outputPathResult = defaultOutputPath(article.Article{Title: "OutputDir Article", Text: "test"}, conf.TTSFormat)
	}
	if conf.Output == "" && conf.OutputDir != "" {
		outputPathResult = filepath.Join(conf.OutputDir, filepath.Base(outputPathResult))
	}

	if !strings.HasPrefix(outputPathResult, outputDir) {
		t.Errorf("expected output path to start with %q, got %q", outputDir, outputPathResult)
	}
	if !strings.HasSuffix(outputPathResult, ".mp3") {
		t.Errorf("expected output path to end with .mp3, got %q", outputPathResult)
	}
}

func TestRootCmdRunE_WithOutputDir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("fake-audio")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	htmlContent := `<!DOCTYPE html><html><head><title>OutputDir Test</title></head><body><p>Hello.</p></body></html>`
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "audio_out")

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		File:          htmlFile,
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

	if err := rootCmdRunE(rootCmd, []string{}); err != nil {
		t.Fatalf("rootCmdRunE: %v", err)
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected output file in output directory")
	}

	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".mp3") {
			found = true
			data, err := os.ReadFile(filepath.Join(outputDir, e.Name()))
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

func TestRootCmdRunE_OutputDirAndOutputFlag_OutputTakesPrecedence(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("fake-audio")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	htmlContent := `<!DOCTYPE html><html><head><title>Precedence Test</title></head><body><p>Hello.</p></body></html>`
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	explicitOutput := filepath.Join(tmpDir, "explicit-output.mp3")
	outputDir := filepath.Join(tmpDir, "audio_out")

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		File:          htmlFile,
		Output:        explicitOutput,
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

	if err := rootCmdRunE(rootCmd, []string{}); err != nil {
		t.Fatalf("rootCmdRunE: %v", err)
	}

	data, err := os.ReadFile(explicitOutput)
	if err != nil {
		t.Fatalf("read explicit output: %v", err)
	}
	if string(data) != "fake-audio" {
		t.Errorf("expected 'fake-audio', got %q", string(data))
	}

	if _, err := os.Stat(outputDir); err == nil {
		entries, _ := os.ReadDir(outputDir)
		if len(entries) > 0 {
			t.Error("expected output-dir to be ignored when --output is specified")
		}
	}
}

func TestTTSBackendFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("tts-backend")
	if flag == nil {
		t.Fatal("expected 'tts-backend' flag to exist")
	}
	if flag.DefValue != "openai" {
		t.Errorf("expected tts-backend default 'openai', got '%s'", flag.DefValue)
	}
}

func TestGoogleTranslateLangFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("lang")
	if flag == nil {
		t.Fatal("expected 'lang' flag to exist")
	}
	if flag.DefValue != "en" {
		t.Errorf("expected lang default 'en', got '%s'", flag.DefValue)
	}
}

func TestRootCmdRunWithGoogleBackend(t *testing.T) {
	orig := tts.GtranslateRequest
	defer func() { tts.GtranslateRequest = orig }()

	tts.GtranslateRequest = func(ctx context.Context, client *http.Client, text, lang string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("google-audio")), nil
	}

	tmpDir := t.TempDir()
	htmlContent := `<!DOCTYPE html><html><head><title>Google TTS Test</title></head><body><p>Hello world.</p></body></html>`
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "google-output.mp3")

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		File:                htmlFile,
		Output:              outputPath,
		TTSBackend:          "google",
		GoogleTranslateLang: "en",
		TTSFormat:           "mp3",
		TTSTimeout:          10 * time.Second,
		FetchTimeout:        30 * time.Second,
	}

	if err := rootCmdRunE(rootCmd, []string{}); err != nil {
		t.Fatalf("rootCmdRunE: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "google-audio" {
		t.Errorf("expected 'google-audio', got %q", string(data))
	}
}

func TestRootCmdRunWithInvalidBackend(t *testing.T) {
	tmpDir := t.TempDir()
	htmlContent := `<!DOCTYPE html><html><head><title>Invalid Backend</title></head><body><p>Hello.</p></body></html>`
	htmlFile := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write html file: %v", err)
	}

	origConf := conf
	defer func() { conf = origConf }()
	conf = config.Config{
		File:         htmlFile,
		TTSBackend:   "invalid",
		TTSFormat:    "mp3",
		TTSTimeout:   10 * time.Second,
		FetchTimeout: 30 * time.Second,
	}

	err := rootCmdRunE(rootCmd, []string{})
	if err == nil {
		t.Error("expected error for invalid TTS backend")
	}
	if !strings.Contains(err.Error(), "unknown TTS backend") {
		t.Errorf("expected 'unknown TTS backend' in error, got %q", err.Error())
	}
}
