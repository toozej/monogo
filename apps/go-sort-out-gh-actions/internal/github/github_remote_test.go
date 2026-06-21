package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_ListWorkflowFiles(t *testing.T) {
	tests := []struct {
		name         string
		ownerRepo    string
		ref          string
		responseBody string
		statusCode   int
		headers      map[string]string
		wantCount    int
		wantError    bool
	}{
		{
			name:      "success with yml and yaml files",
			ownerRepo: "owner/repo",
			ref:       "main",
			responseBody: `[
				{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"},
				{"name": "release.yaml", "path": ".github/workflows/release.yaml", "type": "file"},
				{"name": "notworkflow.txt", "path": ".github/workflows/notworkflow.txt", "type": "file"},
				{"name": "subdir", "path": ".github/workflows/subdir", "type": "dir"}
			]`,
			statusCode: 200,
			wantCount:  2,
			wantError:  false,
		},
		{
			name:         "empty directory",
			ownerRepo:    "owner/repo",
			ref:          "main",
			responseBody: `[]`,
			statusCode:   200,
			wantCount:    0,
			wantError:    false,
		},
		{
			name:       "404 no workflows dir",
			ownerRepo:  "owner/repo",
			ref:        "main",
			statusCode: 404,
			wantCount:  0,
			wantError:  false,
		},
		{
			name:       "rate limited",
			ownerRepo:  "owner/repo",
			ref:        "main",
			statusCode: 403,
			headers:    map[string]string{"X-RateLimit-Reset": "1640995200"},
			wantError:  true,
		},
		{
			name:      "invalid ownerRepo",
			ownerRepo: "invalid",
			ref:       "main",
			wantError: true,
		},
		{
			name:      "empty ref",
			ownerRepo: "owner/repo",
			ref:       "",
			responseBody: `[
				{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"}
			]`,
			statusCode: 200,
			wantCount:  1,
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
					t.Errorf("Expected Accept header, got %s", r.Header.Get("Accept"))
				}

				if tt.ref != "" {
					if !strings.Contains(r.URL.RawQuery, "ref="+tt.ref) {
						t.Errorf("Expected ref query param %s, got %s", tt.ref, r.URL.RawQuery)
					}
				} else {
					if strings.Contains(r.URL.RawQuery, "ref=") {
						t.Errorf("Expected no ref query param, got %s", r.URL.RawQuery)
					}
				}

				for k, v := range tt.headers {
					w.Header().Set(k, v)
				}

				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					if _, err := w.Write([]byte(tt.responseBody)); err != nil {
						t.Errorf("failed to write response body: %v", err)
					}
				}
			}))
			defer server.Close()

			client := newTestClient(server)
			ctx := context.Background()
			entries, err := client.ListWorkflowFiles(ctx, tt.ownerRepo, tt.ref)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.wantError && len(entries) != tt.wantCount {
				t.Errorf("Expected %d entries, got %d", tt.wantCount, len(entries))
			}
		})
	}
}

func TestClient_GetFileContent(t *testing.T) {
	fileContent := "name: CI\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v3\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(fileContent))

	tests := []struct {
		name         string
		ownerRepo    string
		path         string
		ref          string
		responseBody string
		statusCode   int
		headers      map[string]string
		wantContent  string
		wantError    bool
	}{
		{
			name:         "success with base64 content",
			ownerRepo:    "owner/repo",
			path:         ".github/workflows/ci.yml",
			ref:          "main",
			responseBody: fmt.Sprintf(`{"content": "%s", "encoding": "base64"}`, encoded),
			statusCode:   200,
			wantContent:  fileContent,
			wantError:    false,
		},
		{
			name:       "file not found",
			ownerRepo:  "owner/repo",
			path:       ".github/workflows/missing.yml",
			ref:        "main",
			statusCode: 404,
			wantError:  true,
		},
		{
			name:       "rate limited",
			ownerRepo:  "owner/repo",
			path:       ".github/workflows/ci.yml",
			ref:        "main",
			statusCode: 429,
			headers:    map[string]string{"X-RateLimit-Reset": "1640995200"},
			wantError:  true,
		},
		{
			name:      "invalid ownerRepo",
			ownerRepo: "invalid",
			path:      ".github/workflows/ci.yml",
			ref:       "main",
			wantError: true,
		},
		{
			name:         "base64 with newlines",
			ownerRepo:    "owner/repo",
			path:         ".github/workflows/ci.yml",
			ref:          "main",
			responseBody: mustMarshalContentResponse(insertNewlines(encoded, 76)),
			statusCode:   200,
			wantContent:  fileContent,
			wantError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				if !strings.Contains(r.URL.Path, tt.path) {
					t.Errorf("Expected path to contain %s, got %s", tt.path, r.URL.Path)
				}

				for k, v := range tt.headers {
					w.Header().Set(k, v)
				}

				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					if _, err := w.Write([]byte(tt.responseBody)); err != nil {
						t.Errorf("failed to write response body: %v", err)
					}
				}
			}))
			defer server.Close()

			client := newTestClient(server)
			ctx := context.Background()
			content, err := client.GetFileContent(ctx, tt.ownerRepo, tt.path, tt.ref)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.wantError && content != tt.wantContent {
				t.Errorf("Expected content %q, got %q", tt.wantContent, content)
			}
		})
	}
}

func insertNewlines(s string, every int) string {
	var result string
	for i, c := range s {
		result += string(c)
		if (i+1)%every == 0 {
			result += "\n"
		}
	}
	return result
}

func mustMarshalContentResponse(content string) string {
	resp := struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}{
		Content:  content,
		Encoding: "base64",
	}
	data, err := json.Marshal(resp)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal content response: %v", err))
	}
	return string(data)
}

func TestClient_GetRemoteWorkflowContents(t *testing.T) {
	fileContent1 := "name: CI\non: push\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v3\n"
	fileContent2 := "name: Release\non: push\njobs:\n  build:\n    steps:\n      - uses: actions/setup-go@v4\n"
	encoded1 := base64.StdEncoding.EncodeToString([]byte(fileContent1))
	encoded2 := base64.StdEncoding.EncodeToString([]byte(fileContent2))

	t.Run("success with multiple files", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/contents/.github/workflows") && !strings.Contains(r.URL.Path, "/ci.yml") && !strings.Contains(r.URL.Path, "/release.yml") {
				w.WriteHeader(200)
				resp := `[
					{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"},
					{"name": "release.yml", "path": ".github/workflows/release.yml", "type": "file"}
				]`
				if _, err := w.Write([]byte(resp)); err != nil {
					t.Errorf("failed to write response body: %v", err)
				}
				return
			}
			if strings.Contains(r.URL.Path, "/ci.yml") {
				w.WriteHeader(200)
				if _, err := fmt.Fprintf(w, `{"content": "%s", "encoding": "base64"}`, encoded1); err != nil {
					t.Errorf("failed to write response body: %v", err)
				}
				return
			}
			if strings.Contains(r.URL.Path, "/release.yml") {
				w.WriteHeader(200)
				if _, err := fmt.Fprintf(w, `{"content": "%s", "encoding": "base64"}`, encoded2); err != nil {
					t.Errorf("failed to write response body: %v", err)
				}
				return
			}
			w.WriteHeader(404)
		}))
		defer server.Close()

		client := newTestClient(server)
		ctx := context.Background()
		contents, err := client.GetRemoteWorkflowContents(ctx, "owner/repo", "main")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(contents) != 2 {
			t.Errorf("Expected 2 files, got %d", len(contents))
		}
		if contents[".github/workflows/ci.yml"] != fileContent1 {
			t.Errorf("Unexpected content for ci.yml")
		}
		if contents[".github/workflows/release.yml"] != fileContent2 {
			t.Errorf("Unexpected content for release.yml")
		}
	})

	t.Run("404 no workflows dir", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
		}))
		defer server.Close()

		client := newTestClient(server)
		ctx := context.Background()
		contents, err := client.GetRemoteWorkflowContents(ctx, "owner/repo", "main")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(contents) != 0 {
			t.Errorf("Expected 0 files, got %d", len(contents))
		}
	})

	t.Run("one file fetch fails", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/contents/.github/workflows") && !strings.Contains(r.URL.Path, "/ci.yml") && !strings.Contains(r.URL.Path, "/release.yml") {
				w.WriteHeader(200)
				resp := `[
					{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"},
					{"name": "release.yml", "path": ".github/workflows/release.yml", "type": "file"}
				]`
				if _, err := w.Write([]byte(resp)); err != nil {
					t.Errorf("failed to write response body: %v", err)
				}
				return
			}
			if strings.Contains(r.URL.Path, "/ci.yml") {
				w.WriteHeader(200)
				if _, err := fmt.Fprintf(w, `{"content": "%s", "encoding": "base64"}`, encoded1); err != nil {
					t.Errorf("failed to write response body: %v", err)
				}
				return
			}
			w.WriteHeader(500)
		}))
		defer server.Close()

		client := newTestClient(server)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		contents, err := client.GetRemoteWorkflowContents(ctx, "owner/repo", "main")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(contents) != 1 {
			t.Errorf("Expected 1 file (one should fail silently), got %d", len(contents))
		}
		if contents[".github/workflows/ci.yml"] != fileContent1 {
			t.Errorf("Unexpected content for ci.yml")
		}
	})
}

func TestClient_ListWorkflowFiles_RequestError(t *testing.T) {
	client := &Client{
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		token:         "test-token",
		baseURL:       "http://127.0.0.1:0",
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
		cacheEnabled:  true,
		cacheTTL:      24 * time.Hour,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.ListWorkflowFiles(ctx, "owner/repo", "main")
	if err == nil {
		t.Error("Expected network error")
	}
}

func TestClient_GetFileContent_RequestError(t *testing.T) {
	client := &Client{
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		token:         "test-token",
		baseURL:       "http://127.0.0.1:0",
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
		cacheEnabled:  true,
		cacheTTL:      24 * time.Hour,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.GetFileContent(ctx, "owner/repo", ".github/workflows/ci.yml", "main")
	if err == nil {
		t.Error("Expected network error")
	}
}

func TestClient_GetFileContent_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"content": "", "encoding": "base64"}`)); err != nil {
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()
	_, err := client.GetFileContent(ctx, "owner/repo", ".github/workflows/ci.yml", "main")
	if err == nil {
		t.Error("Expected error for empty content")
	}
}

func TestClient_GetFileContent_InvalidBase64(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"content": "not-valid-base64!!!", "encoding": "base64"}`)); err != nil {
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()
	_, err := client.GetFileContent(ctx, "owner/repo", ".github/workflows/ci.yml", "main")
	if err == nil {
		t.Error("Expected error for invalid base64")
	}
}

func TestClient_ListWorkflowFiles_BadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`invalid json`)); err != nil {
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()
	_, err := client.ListWorkflowFiles(ctx, "owner/repo", "main")
	if err == nil {
		t.Error("Expected JSON decode error")
	}
}

func TestClient_GetFileContent_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()
	_, err := client.GetFileContent(ctx, "owner/repo", ".github/workflows/ci.yml", "main")
	if err == nil {
		t.Error("Expected error for 500 status")
	}
}

func TestClient_ListWorkflowFiles_Non200Non404Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()
	_, err := client.ListWorkflowFiles(ctx, "owner/repo", "main")
	if err == nil {
		t.Error("Expected error for 500 status")
	}
}

func TestContentEntry_JSONRoundTrip(t *testing.T) {
	entry := ContentEntry{
		Name:        "ci.yml",
		Path:        ".github/workflows/ci.yml",
		Type:        "file",
		Content:     "bmFtZTogQ0k=",
		Encoding:    "base64",
		DownloadURL: "https://raw.githubusercontent.com/owner/repo/main/.github/workflows/ci.yml",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal ContentEntry: %v", err)
	}

	var decoded ContentEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ContentEntry: %v", err)
	}

	if decoded.Name != entry.Name {
		t.Errorf("Expected Name %s, got %s", entry.Name, decoded.Name)
	}
	if decoded.Type != entry.Type {
		t.Errorf("Expected Type %s, got %s", entry.Type, decoded.Type)
	}
	if decoded.Encoding != entry.Encoding {
		t.Errorf("Expected Encoding %s, got %s", entry.Encoding, decoded.Encoding)
	}
}
