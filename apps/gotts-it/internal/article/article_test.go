package article

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFromSource(t *testing.T) {
	tests := []struct {
		name        string
		source      Source
		expectTitle string
		expectText  string
		expectError bool
	}{
		{
			name: "local HTML file",
			source: Source{
				FilePath: "",
			},
			expectTitle: "Test Article",
			expectText:  "Hello world",
			expectError: false,
		},
		{
			name:        "no source specified",
			source:      Source{},
			expectError: true,
		},
	}

	htmlContent := `<!DOCTYPE html><html><head><title>Test Article</title></head><body><p>Hello world</p></body></html>`

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "local HTML file" {
				tmpDir := t.TempDir()
				fpath := filepath.Join(tmpDir, "test.html")
				if err := os.WriteFile(fpath, []byte(htmlContent), 0644); err != nil {
					t.Fatalf("write test file: %v", err)
				}
				tt.source.FilePath = fpath
			}

			art, err := FromSource(context.Background(), tt.source, 30*time.Second)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if art.Title != tt.expectTitle {
				t.Errorf("expected title %q, got %q", tt.expectTitle, art.Title)
			}
			if art.Text == "" {
				t.Error("expected non-empty text")
			}
		})
	}
}

func TestFromSource_LocalFile(t *testing.T) {
	htmlContent := `<!DOCTYPE html><html><head><title>My Page</title></head><body><p>Paragraph one.</p><p>Paragraph two.</p></body></html>`

	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "article.html")
	if err := os.WriteFile(fpath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	art, err := FromSource(context.Background(), Source{FilePath: fpath}, 30*time.Second)
	if err != nil {
		t.Fatalf("FromSource: %v", err)
	}
	if art.Title != "My Page" {
		t.Errorf("expected title %q, got %q", "My Page", art.Title)
	}
	if art.Text == "" {
		t.Error("expected non-empty text")
	}
}

func TestFromSource_URL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping URL test in short mode")
	}

	htmlContent := `<!DOCTYPE html><html><head><title>URL Article</title></head><body><p>Fetched from URL.</p></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(htmlContent)); err != nil {
			t.Logf("warning: write response: %v", err)
		}
	}))
	defer srv.Close()

	art, err := FromSource(context.Background(), Source{URL: srv.URL}, 30*time.Second)
	if err != nil {
		t.Fatalf("FromSource URL: %v", err)
	}
	if art.Title != "URL Article" {
		t.Errorf("expected title %q, got %q", "URL Article", art.Title)
	}
	if art.Text == "" {
		t.Error("expected non-empty text")
	}
}

func TestFromSource_NoSource(t *testing.T) {
	_, err := FromSource(context.Background(), Source{}, 30*time.Second)
	if err == nil {
		t.Error("expected error when no source specified")
	}
}

func TestFromSource_NonexistentFile(t *testing.T) {
	_, err := FromSource(context.Background(), Source{FilePath: "/nonexistent/file.html"}, 30*time.Second)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFromSource_InvalidHTML(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "bad.html")
	if err := os.WriteFile(fpath, []byte("this is not html at all"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	art, err := FromSource(context.Background(), Source{FilePath: fpath}, 30*time.Second)
	if err != nil {
		t.Fatalf("FromSource with invalid HTML: %v", err)
	}
	if art.Text == "" && art.Title == "" {
		t.Error("expected at least some content from invalid HTML")
	}
}

func TestFromSource_URLFetchError(t *testing.T) {
	_, err := FromSource(context.Background(), Source{URL: "http://localhost:1/unreachable"}, 1*time.Second)
	if err == nil {
		t.Error("expected error for unreachable URL")
	}
}

func TestFromSource_FileWithComplexHTML(t *testing.T) {
	htmlContent := `<!DOCTYPE html>
<html>
<head><title>Complex Article</title></head>
<body>
	<nav>Navigation</nav>
	<article>
		<h1>Main Title</h1>
		<p>First paragraph with some text.</p>
		<p>Second paragraph with more content.</p>
	</article>
	<footer>Footer content</footer>
</body>
</html>`

	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "complex.html")
	if err := os.WriteFile(fpath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	art, err := FromSource(context.Background(), Source{FilePath: fpath}, 30*time.Second)
	if err != nil {
		t.Fatalf("FromSource: %v", err)
	}
	if art.Title != "Complex Article" {
		t.Errorf("expected title 'Complex Article', got %q", art.Title)
	}
	if art.Text == "" {
		t.Error("expected non-empty text")
	}
}
