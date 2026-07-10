package tts

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openai/openai-go/v3"
)

func TestSynthesize_SingleChunk(t *testing.T) {
	audioData := []byte("fake-mp3-data")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/audio/speech") {
			t.Errorf("expected path to end with /audio/speech, got %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var params map[string]any
		if err := json.Unmarshal(body, &params); err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}

		if params["input"] != "Hello world" {
			t.Errorf("expected input 'Hello world', got %v", params["input"])
		}
		if params["model"] != "test-model" {
			t.Errorf("expected model 'test-model', got %v", params["model"])
		}
		if params["voice"] != "test-voice" {
			t.Errorf("expected voice 'test-voice', got %v", params["voice"])
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write(audioData); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	opts := Options{
		BaseURL: srv.URL + "/v1",
		APIKey:  "test-key",
		Model:   "test-model",
		Voice:   "test-voice",
		Format:  "mp3",
		Speed:   1.0,
		Timeout: 10 * time.Second,
	}

	err := Synthesize(context.Background(), "Hello world", outputPath, opts)
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != string(audioData) {
		t.Errorf("expected output %q, got %q", audioData, data)
	}
}

func TestSynthesize_MultipleChunks(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		body, _ := io.ReadAll(r.Body)
		var params map[string]any
		if err := json.Unmarshal(body, &params); err != nil {
			t.Logf("warning: failed to parse request body: %v", err)
		}

		input, _ := params["input"].(string)
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write([]byte("chunk-" + input[:min(len(input), 10)] + "-")); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	longText := strings.Repeat("This is a sentence. ", 500)

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	opts := Options{
		BaseURL: srv.URL + "/v1",
		APIKey:  "test-key",
		Model:   "test-model",
		Voice:   "test-voice",
		Format:  "mp3",
		Speed:   1.0,
		Timeout: 10 * time.Second,
	}

	err := Synthesize(context.Background(), longText, outputPath, opts)
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}

	if callCount < 2 {
		t.Errorf("expected multiple chunk requests, got %d", callCount)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty output file")
	}
}

func TestSynthesize_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write([]byte("internal server error")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	opts := Options{
		BaseURL: srv.URL + "/v1",
		APIKey:  "test-key",
		Model:   "test-model",
		Voice:   "test-voice",
		Format:  "mp3",
		Speed:   1.0,
		Timeout: 10 * time.Second,
	}

	err := Synthesize(context.Background(), "Hello world", outputPath, opts)
	if err == nil {
		t.Error("expected error for non-2xx response")
	}
}

func TestSynthesizeFailurePreservesExistingOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	if err := os.WriteFile(outputPath, []byte("existing-audio"), 0600); err != nil {
		t.Fatal(err)
	}
	err := Synthesize(context.Background(), "Hello", outputPath, Options{
		BaseURL: srv.URL + "/v1", Model: "model", Voice: "voice", Format: "mp3", Timeout: time.Second,
	})
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

func TestSynthesize_EmptyText(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	opts := Options{
		BaseURL: "http://localhost:8000/v1",
		APIKey:  "test-key",
		Model:   "test-model",
		Voice:   "test-voice",
		Format:  "mp3",
		Speed:   1.0,
		Timeout: 10 * time.Second,
	}

	err := Synthesize(context.Background(), "", outputPath, opts)
	if err == nil {
		t.Error("expected error for empty text")
	}
}

func TestSynthesize_FLACMultipleChunks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write([]byte("flac-data")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	longText := strings.Repeat("This is a sentence. ", 500)

	outputPath := filepath.Join(t.TempDir(), "output.flac")
	opts := Options{
		BaseURL: srv.URL + "/v1",
		APIKey:  "test-key",
		Model:   "test-model",
		Voice:   "test-voice",
		Format:  "flac",
		Speed:   1.0,
		Timeout: 10 * time.Second,
	}

	err := Synthesize(context.Background(), longText, outputPath, opts)
	if err == nil {
		t.Error("expected error for FLAC with multiple chunks")
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Errorf("failed FLAC synthesis left an output file: %v", statErr)
	}
}

func TestSynthesize_ForwardsParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var params map[string]any
		if err := json.Unmarshal(body, &params); err != nil {
			t.Logf("warning: failed to parse request body: %v", err)
		}

		if params["model"] != "custom-model" {
			t.Errorf("expected model 'custom-model', got %v", params["model"])
		}
		if params["voice"] != "custom-voice" {
			t.Errorf("expected voice 'custom-voice', got %v", params["voice"])
		}
		if params["response_format"] != "wav" {
			t.Errorf("expected response_format 'wav', got %v", params["response_format"])
		}
		if speed, ok := params["speed"].(float64); !ok || speed != 1.5 {
			t.Errorf("expected speed 1.5, got %v", params["speed"])
		}
		if params["instructions"] != "speak slowly" {
			t.Errorf("expected instructions 'speak slowly', got %v", params["instructions"])
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write(createWAVFile(100)); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	outputPath := filepath.Join(t.TempDir(), "output.wav")
	opts := Options{
		BaseURL:      srv.URL + "/v1",
		APIKey:       "test-key",
		Model:        "custom-model",
		Voice:        "custom-voice",
		Format:       "wav",
		Speed:        1.5,
		Instructions: "speak slowly",
		Timeout:      10 * time.Second,
	}

	err := Synthesize(context.Background(), "Test input", outputPath, opts)
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
}

func TestSynthesize_DefaultAPIKey(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write([]byte("audio")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	opts := Options{
		BaseURL: srv.URL + "/v1",
		APIKey:  "",
		Model:   "test-model",
		Voice:   "test-voice",
		Format:  "mp3",
		Speed:   1.0,
		Timeout: 10 * time.Second,
	}

	err := Synthesize(context.Background(), "Hello", outputPath, opts)
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}

	if authHeader == "" {
		t.Error("expected Authorization header to be set")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestRewriteWAVHeader(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		expectError bool
	}{
		{"valid wav", createWAVFile(1000), false},
		{"too small", []byte("RIFF"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.wav")
			if err := os.WriteFile(path, tt.content, 0644); err != nil {
				t.Fatalf("write wav: %v", err)
			}

			err := rewriteWAVHeader(path)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("rewriteWAVHeader: %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read wav: %v", err)
			}
			if len(data) < 44 {
				t.Fatalf("file too small after rewrite: %d bytes", len(data))
			}
			if string(data[:4]) != "RIFF" {
				t.Errorf("expected RIFF header, got %q", data[:4])
			}
		})
	}
}

func createWAVFile(dataSize int) []byte {
	totalSize := 44 + dataSize
	buf := make([]byte, totalSize)
	copy(buf[0:4], "RIFF")
	writeLE32(buf, 4, uint32(totalSize-8))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	writeLE32(buf, 16, 16)
	writeLE16(buf, 20, 1)
	writeLE16(buf, 22, 1)
	writeLE32(buf, 24, 44100)
	writeLE32(buf, 28, 44100*2)
	writeLE16(buf, 32, 2)
	writeLE16(buf, 34, 16)
	copy(buf[36:40], "data")
	writeLE32(buf, 40, uint32(dataSize))
	for i := 44; i < totalSize; i++ {
		buf[i] = byte(i % 256)
	}
	return buf
}

func writeLE32(buf []byte, offset int, val uint32) {
	buf[offset] = byte(val)
	buf[offset+1] = byte(val >> 8)
	buf[offset+2] = byte(val >> 16)
	buf[offset+3] = byte(val >> 24)
}

func writeLE16(buf []byte, offset int, val uint16) {
	buf[offset] = byte(val)
	buf[offset+1] = byte(val >> 8)
}

func TestWriteUint32LE(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.bin")
	if err := os.WriteFile(path, make([]byte, 8), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	defer func() { _ = root.Close() }()

	f, err := root.OpenFile(filepath.Base(path), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer func() { _ = f.Close() }()

	if err := writeUint32LE(f, 0x12345678, 0); err != nil {
		t.Fatalf("writeUint32LE: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if data[0] != 0x78 || data[1] != 0x56 || data[2] != 0x34 || data[3] != 0x12 {
		t.Errorf("expected little-endian 0x12345678, got %x", data[:4])
	}
}

func TestWriteChunk_PCM(t *testing.T) {
	f, err := os.CreateTemp("", "chunk-*.pcm")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer func() { _ = f.Close() }()
	defer func() { _ = os.Remove(f.Name()) }()

	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("pcm-data")),
	}
	if err := writeChunk(f, resp, "pcm", true, true); err != nil {
		t.Fatalf("writeChunk pcm: %v", err)
	}
}

func TestWriteChunk_WAV(t *testing.T) {
	f, err := os.CreateTemp("", "chunk-*.wav")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer func() { _ = f.Close() }()
	defer func() { _ = os.Remove(f.Name()) }()

	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(string(createWAVFile(100)))),
	}
	if err := writeChunk(f, resp, "wav", true, true); err != nil {
		t.Fatalf("writeChunk wav: %v", err)
	}
}

func TestWriteChunk_DefaultFormat(t *testing.T) {
	f, err := os.CreateTemp("", "chunk-*.bin")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer func() { _ = f.Close() }()
	defer func() { _ = os.Remove(f.Name()) }()

	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("default-data")),
	}
	if err := writeChunk(f, resp, "ogg", true, true); err != nil {
		t.Fatalf("writeChunk default: %v", err)
	}
}

func TestWriteChunk_FLACNotFirst(t *testing.T) {
	f, err := os.CreateTemp("", "chunk-*.flac")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer func() { _ = f.Close() }()
	defer func() { _ = os.Remove(f.Name()) }()

	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("flac-data")),
	}
	if err := writeChunk(f, resp, "flac", false, false); err == nil {
		t.Error("expected error for FLAC not-first chunk")
	}
}

func TestSynthesize_WAVMultipleChunks(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		wavData := createWAVFile(100)
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write(wavData); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	longText := strings.Repeat("This is a sentence. ", 500)

	outputPath := filepath.Join(t.TempDir(), "output.wav")
	opts := Options{
		BaseURL: srv.URL + "/v1",
		APIKey:  "test-key",
		Model:   "test-model",
		Voice:   "test-voice",
		Format:  "wav",
		Speed:   1.0,
		Timeout: 10 * time.Second,
	}

	err := Synthesize(context.Background(), longText, outputPath, opts)
	if err != nil {
		t.Fatalf("Synthesize WAV multiple chunks: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) < 44 {
		t.Fatalf("expected wav file >= 44 bytes, got %d", len(data))
	}
	if string(data[:4]) != "RIFF" {
		t.Errorf("expected RIFF header, got %q", data[:4])
	}
	if got := bytes.Count(data, []byte("RIFF")); got != 1 {
		t.Errorf("expected one WAV header, got %d", got)
	}
	wantSize := 44 + callCount*100
	if len(data) != wantSize {
		t.Errorf("WAV size = %d, want %d", len(data), wantSize)
	}
	if got := int(binary.LittleEndian.Uint32(data[40:44])); got != callCount*100 {
		t.Errorf("WAV data size = %d, want %d", got, callCount*100)
	}
}

func TestSynthesize_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	opts := Options{
		BaseURL: srv.URL + "/v1",
		APIKey:  "test-key",
		Model:   "test-model",
		Voice:   "test-voice",
		Format:  "mp3",
		Speed:   1.0,
		Timeout: 10 * time.Second,
	}

	err := Synthesize(ctx, "Hello world", outputPath, opts)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestSynthesize_InvalidOutputDir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write([]byte("audio")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	opts := Options{
		BaseURL: srv.URL + "/v1",
		APIKey:  "test-key",
		Model:   "test-model",
		Voice:   "test-voice",
		Format:  "mp3",
		Speed:   1.0,
		Timeout: 10 * time.Second,
	}

	err := Synthesize(context.Background(), "Hello world", "/nonexistent/dir/output.mp3", opts)
	if err == nil {
		t.Error("expected error for invalid output directory")
	}
}

func TestSynthesize_ZeroSpeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var params map[string]any
		if err := json.Unmarshal(body, &params); err != nil {
			t.Logf("warning: failed to parse request body: %v", err)
		}
		if _, ok := params["speed"]; ok {
			t.Error("expected speed to be omitted when zero")
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write([]byte("audio")); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	opts := Options{
		BaseURL:      srv.URL + "/v1",
		APIKey:       "test-key",
		Model:        "test-model",
		Voice:        "test-voice",
		Format:       "mp3",
		Speed:        0,
		Instructions: "",
		Timeout:      10 * time.Second,
	}

	err := Synthesize(context.Background(), "Hello", outputPath, opts)
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
}

func TestResponseFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected openai.AudioSpeechNewParamsResponseFormat
	}{
		{"wav", openai.AudioSpeechNewParamsResponseFormatWAV},
		{"flac", openai.AudioSpeechNewParamsResponseFormatFLAC},
		{"pcm", openai.AudioSpeechNewParamsResponseFormatPCM},
		{"mp3", openai.AudioSpeechNewParamsResponseFormatMP3},
		{"unknown", openai.AudioSpeechNewParamsResponseFormatMP3},
		{"WAV", openai.AudioSpeechNewParamsResponseFormatWAV},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := responseFormat(tt.input)
			if got != tt.expected {
				t.Errorf("responseFormat(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestChunk_HardSplit(t *testing.T) {
	text := strings.Repeat("a", 5000)
	chunks := chunk(text, 4096)
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks for 5000 chars, got %d", len(chunks))
	}
	total := 0
	for _, c := range chunks {
		total += len(c)
	}
	if total != 5000 {
		t.Errorf("expected total 5000 chars, got %d", total)
	}
}

func TestChunkPreservesUTF8AndCountsCharacters(t *testing.T) {
	text := strings.Repeat("界", MaxInputChars+1)
	chunks := chunk(text, MaxInputChars)
	if len(chunks) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(chunks))
	}
	for i, chunk := range chunks {
		if strings.ToValidUTF8(chunk, "") != chunk {
			t.Fatalf("chunk %d contains invalid UTF-8", i)
		}
		if len([]rune(chunk)) > MaxInputChars {
			t.Fatalf("chunk %d exceeds character limit", i)
		}
	}
	if strings.Join(chunks, "") != text {
		t.Fatal("chunking changed Unicode text")
	}
}

func TestFindSentenceBoundary_NoBoundary(t *testing.T) {
	text := strings.Repeat("x", 5000)
	boundary := findSentenceBoundary(text, 4096)
	if boundary != 0 {
		t.Errorf("expected 0 boundary for text with no sentence endings, got %d", boundary)
	}
}

func TestFindSentenceBoundary_WithBoundary(t *testing.T) {
	text := "This is sentence one. And sentence two. And three."
	boundary := findSentenceBoundary(text, len(text))
	if boundary == 0 {
		t.Error("expected non-zero boundary for text with sentence endings")
	}
}

func TestRewriteWAVHeader_NonexistentDir(t *testing.T) {
	err := rewriteWAVHeader("/nonexistent/dir/test.wav")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}
