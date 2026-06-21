package avatar

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDefaultConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{
			name:     "DefaultAvatarURL has expected value",
			got:      DefaultAvatarURL,
			expected: "https://github.com/toozej.png",
		},
		{
			name:     "DefaultAvatarPath points to local avatar",
			got:      DefaultAvatarPath,
			expected: "./img/avatar.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %q, expected %q", tt.got, tt.expected)
			}
		})
	}
}

func TestPrintFallback(t *testing.T) {
	var buf bytes.Buffer
	PrintFallback(&buf)

	output := buf.String()

	tests := []struct {
		name     string
		contains string
	}{
		{
			name:     "output contains gotts-it",
			contains: "gotts-it",
		},
		{
			name:     "output contains ascii art",
			contains: "____",
		},
		{
			name:     "output contains ____ header",
			contains: "____",
		},
		{
			name:     "output contains ___/ footer",
			contains: "____/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(output, tt.contains) {
				t.Errorf("PrintFallback output does not contain %q, got:\n%s", tt.contains, output)
			}
		})
	}
}

func TestPrintFallback_WritesToWriter(t *testing.T) {
	var buf bytes.Buffer
	PrintFallback(&buf)
	if buf.Len() == 0 {
		t.Error("PrintFallback should write data to the writer")
	}
}

func TestRenderFromFile_InvalidPath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		width      int
		height     int
		wantErrSub string
	}{
		{
			name:       "nonexistent path returns render error",
			path:       "/nonexistent/path/avatar.png",
			width:      40,
			height:     20,
			wantErrSub: "failed to render avatar image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := RenderFromFile(tt.path, tt.width, tt.height, &buf)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

func TestRenderFromFile_InvalidFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		width   int
		height  int
		wantErr bool
	}{
		{
			name:    "invalid image content returns error",
			content: "not an image",
			width:   40,
			height:  20,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "avatar-invalid-*.png")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			var buf bytes.Buffer
			err = RenderFromFile(tmpFile.Name(), tt.width, tt.height, &buf)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRenderFromFile_MockedSuccess(t *testing.T) {
	origRenderImage := renderImage
	defer func() { renderImage = origRenderImage }()

	renderImage = func(path string, width, height int) (string, error) {
		return "mock-rendered-image-output", nil
	}

	var buf bytes.Buffer
	err := RenderFromFile("/fake/path.png", 40, 20, &buf)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buf.String(), "mock-rendered-image-output") {
		t.Errorf("expected rendered output in buffer, got: %q", buf.String())
	}
}

func TestRenderFromFile_MockedRenderError(t *testing.T) {
	origRenderImage := renderImage
	defer func() { renderImage = origRenderImage }()

	renderImage = func(path string, width, height int) (string, error) {
		return "", errors.New("terminal does not support image rendering")
	}

	var buf bytes.Buffer
	err := RenderFromFile("/fake/path.png", 40, 20, &buf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to render avatar image") {
		t.Errorf("error %q does not contain %q", err.Error(), "failed to render avatar image")
	}
	if !strings.Contains(err.Error(), "terminal does not support image rendering") {
		t.Errorf("error %q does not contain wrapped error message", err.Error())
	}
}

func TestRenderFromURL_InvalidURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		width      int
		height     int
		wantErrSub string
	}{
		{
			name:       "http scheme rejected",
			url:        "http://127.0.0.1:0/invalid.png",
			width:      40,
			height:     20,
			wantErrSub: "URL scheme must be https",
		},
		{
			name:       "empty hostname rejected",
			url:        "https:///invalid.png",
			width:      40,
			height:     20,
			wantErrSub: "must have a hostname",
		},
		{
			name:       "file scheme rejected",
			url:        "file:///etc/passwd",
			width:      40,
			height:     20,
			wantErrSub: "URL scheme must be https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := RenderFromURL(tt.url, tt.width, tt.height, &buf)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

func TestRenderFromURL_UnreachableHTTPS(t *testing.T) {
	var buf bytes.Buffer
	err := RenderFromURL("https://127.0.0.1:0/invalid.png", 40, 20, &buf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to fetch avatar") {
		t.Errorf("error %q does not contain %q", err.Error(), "failed to fetch avatar")
	}
}

func TestRenderFromURL_HTTPError(t *testing.T) {
	origHTTPGet := httpGet
	defer func() { httpGet = origHTTPGet }()

	tests := []struct {
		name       string
		statusCode int
		wantErrSub string
	}{
		{
			name:       "404 response returns HTTP error",
			statusCode: http.StatusNotFound,
			wantErrSub: "HTTP 404",
		},
		{
			name:       "500 response returns HTTP error",
			statusCode: http.StatusInternalServerError,
			wantErrSub: "HTTP 500",
		},
		{
			name:       "403 response returns HTTP error",
			statusCode: http.StatusForbidden,
			wantErrSub: "HTTP 403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			httpGet = func(url string) (*http.Response, error) {
				return http.Get(server.URL + "/avatar.png")
			}

			var buf bytes.Buffer
			err := RenderFromURL("https://example.com/avatar.png", 40, 20, &buf)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

func TestRenderFromURL_MockedHTTPGetError(t *testing.T) {
	origHTTPGet := httpGet
	defer func() { httpGet = origHTTPGet }()

	httpGet = func(url string) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}

	var buf bytes.Buffer
	err := RenderFromURL("https://example.com/avatar.png", 40, 20, &buf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to fetch avatar") {
		t.Errorf("error %q does not contain %q", err.Error(), "failed to fetch avatar")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error %q does not contain wrapped error", err.Error())
	}
}

func TestRenderFromURL_MockedSuccess(t *testing.T) {
	origHTTPGet := httpGet
	defer func() { httpGet = origHTTPGet }()

	origRenderImage := renderImage
	defer func() { renderImage = origRenderImage }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pngData := []byte{
			0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(pngData); err != nil { // nosem todo:lint - false positive: binary PNG data in test mock server, not user-facing HTML
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	httpGet = func(url string) (*http.Response, error) {
		return http.Get(server.URL + "/avatar.png")
	}

	renderImage = func(path string, width, height int) (string, error) {
		return "mock-rendered-from-url", nil
	}

	var buf bytes.Buffer
	err := RenderFromURL("https://example.com/avatar.png", 40, 20, &buf)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(buf.String(), "mock-rendered-from-url") {
		t.Errorf("expected rendered output in buffer, got: %q", buf.String())
	}
}

func TestRenderFromURL_MockedSuccessThenRenderFails(t *testing.T) {
	origHTTPGet := httpGet
	defer func() { httpGet = origHTTPGet }()

	origRenderImage := renderImage
	defer func() { renderImage = origRenderImage }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(pngData); err != nil { // nosem todo:lint - false positive: binary PNG data in test mock server, not user-facing HTML
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	httpGet = func(url string) (*http.Response, error) {
		return http.Get(server.URL + "/avatar.png")
	}

	renderImage = func(path string, width, height int) (string, error) {
		return "", errors.New("render engine failed")
	}

	var buf bytes.Buffer
	err := RenderFromURL("https://example.com/avatar.png", 40, 20, &buf)
	if err == nil {
		t.Fatal("expected error when render fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to render avatar image") {
		t.Errorf("error %q does not contain %q", err.Error(), "failed to render avatar image")
	}
}

func TestRenderFromURL_MockedHTTPGetWithBodyReadError(t *testing.T) {
	origHTTPGet := httpGet
	defer func() { httpGet = origHTTPGet }()

	httpGet = func(url string) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(&errorReader{}),
		}, nil
	}

	var buf bytes.Buffer
	err := RenderFromURL("https://example.com/avatar.png", 40, 20, &buf)
	if err == nil {
		t.Fatal("expected error when body read fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to save avatar image") {
		t.Errorf("error %q does not contain %q", err.Error(), "failed to save avatar image")
	}
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

func TestRenderFromFile_MockedRenderImageIncludesPath(t *testing.T) {
	origRenderImage := renderImage
	defer func() { renderImage = origRenderImage }()

	var capturedPath string
	renderImage = func(path string, width, height int) (string, error) {
		capturedPath = path
		return "", errors.New("test error")
	}

	var buf bytes.Buffer
	_ = RenderFromFile("/some/specific/path.png", 10, 5, &buf)

	if capturedPath != "/some/specific/path.png" {
		t.Errorf("renderImage received path %q, want %q", capturedPath, "/some/specific/path.png")
	}
}

func TestRenderFromFile_ErrorWrapping(t *testing.T) {
	origRenderImage := renderImage
	defer func() { renderImage = origRenderImage }()

	renderImage = func(path string, width, height int) (string, error) {
		return "", fmt.Errorf("open %s: no such file", path)
	}

	var buf bytes.Buffer
	err := RenderFromFile("/missing.png", 40, 20, &buf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "/missing.png") {
		t.Errorf("error %q should contain the file path", err.Error())
	}
}

func TestRenderFromURL_MockedTempFileCreationError(t *testing.T) {
	origHTTPGet := httpGet
	defer func() { httpGet = origHTTPGet }()

	httpGet = func(url string) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("fake-image-data")),
		}, nil
	}

	var buf bytes.Buffer
	err := RenderFromURL("https://example.com/avatar.png", 40, 20, &buf)
	if err != nil {
		if !strings.Contains(err.Error(), "failed to render avatar image") &&
			!strings.Contains(err.Error(), "failed to save avatar image") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestValidateAvatarURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "valid https", url: "https://example.com/avatar.png", wantErr: false},
		{name: "http rejected", url: "http://example.com/avatar.png", wantErr: true},
		{name: "ftp rejected", url: "ftp://example.com/avatar.png", wantErr: true},
		{name: "file rejected", url: "file:///etc/passwd", wantErr: true},
		{name: "empty hostname", url: "https:///avatar.png", wantErr: true},
		{name: "empty string", url: "", wantErr: true},
		{name: "no scheme", url: "example.com/avatar.png", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAvatarURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAvatarURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}
