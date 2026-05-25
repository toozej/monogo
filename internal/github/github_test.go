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

	"github.com/toozej/go-sort-out-gh-actions/internal/runtime"
	"github.com/toozej/go-sort-out-gh-actions/internal/workflow"
)

func newTestClient(server *httptest.Server) *Client {
	return &Client{
		httpClient:    server.Client(),
		token:         "test-token",
		baseURL:       server.URL,
		eolClient:     runtime.NewEOLClient(server.Client()),
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
	}
}

func TestClient_GetActionYML(t *testing.T) {
	actionContent := `name: Test Action
description: A test action
runs:
  using: node20
  main: dist/index.js
`
	encoded := base64.StdEncoding.EncodeToString([]byte(actionContent))

	tests := []struct {
		name         string
		ownerRepo    string
		ref          string
		responseBody string
		statusCode   int
		wantUsing    string
		wantError    bool
	}{
		{
			name:         "action.yml found with node20",
			ownerRepo:    "owner/repo",
			ref:          "v2",
			responseBody: fmt.Sprintf(`{"content": "%s"}`, encoded),
			statusCode:   200,
			wantUsing:    "node20",
			wantError:    false,
		},
		{
			name:       "action.yml not found",
			ownerRepo:  "owner/repo",
			ref:        "v1",
			statusCode: 404,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "action.yml") || strings.Contains(r.URL.Path, "action.yaml") {
					w.WriteHeader(tt.statusCode)
					if tt.responseBody != "" {
						if _, err := w.Write([]byte(tt.responseBody)); err != nil {
							t.Errorf("failed to write response body: %v", err)
						}
					}
					return
				}
				w.WriteHeader(404)
			}))
			defer server.Close()

			client := newTestClient(server)
			ctx := context.Background()
			content, err := client.GetActionYML(ctx, tt.ownerRepo, tt.ref)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.wantError {
				using, parseErr := ParseActionYML(content)
				if parseErr != nil {
					t.Errorf("Failed to parse action.yml: %v", parseErr)
				}
				if using != tt.wantUsing {
					t.Errorf("Expected using=%s, got %s", tt.wantUsing, using)
				}
			}
		})
	}
}

func TestClient_CheckMultipleRuntimeEOL(t *testing.T) {
	actionContent := `name: Test Action
runs:
  using: node20
  main: dist/index.js
`
	encoded := base64.StdEncoding.EncodeToString([]byte(actionContent))

	eolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/products/nodejs/releases/20") {
			eolFrom := "2026-04-30"
			resp := runtime.ProductReleaseResponse{
				SchemaVersion: "1.0.0",
				Result: runtime.ProductRelease{
					Name:    "20",
					IsEol:   true,
					EolFrom: &eolFrom,
				},
			}
			w.WriteHeader(200)
			body, _ := json.Marshal(resp)
			if _, err := w.Write(body); err != nil {
				t.Errorf("failed to write response body: %v", err)
			}
			return
		}
		w.WriteHeader(404)
	}))
	defer eolServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "action.yml") || strings.Contains(r.URL.Path, "action.yaml") {
			if strings.Contains(r.URL.Path, "eol-action") {
				w.WriteHeader(200)
				if _, err := w.Write([]byte(fmt.Sprintf(`{"content": "%s"}`, encoded))); err != nil {
					t.Errorf("failed to write response body: %v", err)
				}
				return
			}
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	client.eolClient = runtime.NewEOLClientWithHTTP(eolServer.URL, eolServer.Client())
	ctx := context.Background()

	refs := []workflow.ActionRef{
		{OwnerRepo: "owner/eol-action", Version: "v2"},
		{OwnerRepo: "owner/nonexistent", Version: "v1"},
	}

	results, errors := client.CheckMultipleRuntimeEOL(ctx, refs)

	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errors))
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if result, ok := results["owner/eol-action@v2"]; ok {
		if result.Version != "20" {
			t.Errorf("Expected version 20, got %s", result.Version)
		}
		if !result.IsEOL {
			t.Error("Expected IsEOL to be true")
		}
	}
}

func TestParseActionYML(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantUsing string
		wantError bool
	}{
		{
			name:      "node20 runtime",
			content:   "name: Test\nruns:\n  using: node20\n  main: index.js\n",
			wantUsing: "node20",
			wantError: false,
		},
		{
			name:      "docker runtime",
			content:   "name: Test\nruns:\n  using: docker\n  image: Dockerfile\n",
			wantUsing: "docker",
			wantError: false,
		},
		{
			name:      "composite action",
			content:   "name: Test\nruns:\n  using: composite\n  steps:\n    - run: echo hello\n      shell: bash\n",
			wantUsing: "composite",
			wantError: false,
		},
		{
			name:      "invalid yaml",
			content:   "{{invalid",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			using, err := ParseActionYML(tt.content)
			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.wantError && using != tt.wantUsing {
				t.Errorf("Expected using=%s, got %s", tt.wantUsing, using)
			}
		})
	}
}

func TestClient_IsRepoArchived(t *testing.T) {
	tests := []struct {
		name         string
		ownerRepo    string
		responseBody string
		statusCode   int
		headers      map[string]string
		expected     bool
		expectError  bool
	}{
		{
			name:      "archived repository",
			ownerRepo: "owner/repo",
			responseBody: `{
				"name": "repo",
				"full_name": "owner/repo",
				"archived": true,
				"private": false,
				"html_url": "https://github.com/owner/repo"
			}`,
			statusCode:  200,
			expected:    true,
			expectError: false,
		},
		{
			name:      "active repository",
			ownerRepo: "owner/repo",
			responseBody: `{
				"name": "repo",
				"full_name": "owner/repo",
				"archived": false,
				"private": false,
				"html_url": "https://github.com/owner/repo"
			}`,
			statusCode:  200,
			expected:    false,
			expectError: false,
		},
		{
			name:        "repository not found",
			ownerRepo:   "owner/nonexistent",
			statusCode:  404,
			expected:    false,
			expectError: true,
		},
		{
			name:        "rate limited without reset time",
			ownerRepo:   "owner/repo",
			statusCode:  403,
			expected:    false,
			expectError: true,
		},
		{
			name:        "rate limited with reset time",
			ownerRepo:   "owner/repo",
			statusCode:  403,
			headers:     map[string]string{"X-RateLimit-Reset": "1640995200"},
			expected:    false,
			expectError: true,
		},
		{
			name:        "rate limited with bad reset time",
			ownerRepo:   "owner/repo",
			statusCode:  403,
			headers:     map[string]string{"X-RateLimit-Reset": "bad"},
			expected:    false,
			expectError: true,
		},
		{
			name:        "empty ownerRepo",
			ownerRepo:   " ",
			expected:    false,
			expectError: true,
		},
		{
			name:        "invalid ownerRepo format",
			ownerRepo:   "owner",
			expected:    false,
			expectError: true,
		},
		{
			name:         "with https prefix and @ref",
			ownerRepo:    "https://github.com/owner/repo@v1",
			responseBody: `{"archived": true}`,
			statusCode:   200,
			expected:     true,
			expectError:  false,
		},
		{
			name:        "non 200/403/404 status",
			ownerRepo:   "owner/repo",
			statusCode:  500,
			expected:    false,
			expectError: true,
		},
		{
			name:         "bad json response",
			ownerRepo:    "owner/repo",
			responseBody: `invalid json`,
			statusCode:   200,
			expected:     false,
			expectError:  true,
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
				if r.Header.Get("User-Agent") == "" {
					t.Errorf("Expected User-Agent header")
				}

				for k, v := range tt.headers {
					w.Header().Set(k, v)
				}

				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					if _, err := w.Write([]byte(tt.responseBody)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
						t.Errorf("failed to write response body: %v", err)
					}
				}
			}))
			defer server.Close()

			client := newTestClient(server)
			ctx := context.Background()
			archived, repoInfo, err := client.IsRepoArchived(ctx, tt.ownerRepo)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				if archived != tt.expected {
					t.Errorf("Expected archived=%v, got %v", tt.expected, archived)
				}
				if tt.expected && repoInfo == nil {
					t.Error("Expected repo info for archived repo")
				}
			}
		})
	}
}

func TestClient_IsRepoArchived_RequestError(t *testing.T) {
	client := &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "http://127.0.0.1:0",
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
	}
	_, _, err := client.IsRepoArchived(context.Background(), "owner/repo")
	if err == nil {
		t.Error("Expected network error")
	}
}

func TestClient_IsRepoArchived_BadURL(t *testing.T) {
	client := &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "://bad\x00url",
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
	}
	_, _, err := client.IsRepoArchived(context.Background(), "owner/repo")
	if err == nil {
		t.Error("Expected URL creation error")
	}
}

func TestClient_CheckMultipleRepos(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "error") {
			w.WriteHeader(500)
			return
		}
		response := `{"archived": false}`
		w.WriteHeader(200)
		if _, err := w.Write([]byte(response)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()
	repos := []string{"owner/repo1", "owner/error_repo"}

	archived, errors := client.CheckMultipleRepos(ctx, repos)

	if len(archived) != 1 {
		t.Errorf("Expected 1 successful result, got %d", len(archived))
	}

	if len(errors) != 1 {
		t.Errorf("Expected 1 error result, got %d", len(errors))
	}
	if _, ok := errors["owner/error_repo"]; !ok {
		t.Error("Expected error for owner/error_repo")
	}
}

func TestClient_IsRepoArchived_Caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"archived": true, "name": "repo"}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	archived1, _, err := client.IsRepoArchived(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !archived1 {
		t.Error("Expected archived=true")
	}

	archived2, _, err := client.IsRepoArchived(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error on cached call: %v", err)
	}
	if !archived2 {
		t.Error("Expected archived=true from cache")
	}

	if callCount != 1 {
		t.Errorf("Expected 1 API call (second should be cached), got %d", callCount)
	}
}

func TestNewClient(t *testing.T) {
	token := "test-token"
	client := NewClient(token)

	if client.token != token {
		t.Errorf("Expected token %s, got %s", token, client.token)
	}

	if client.baseURL != "https://api.github.com" {
		t.Errorf("Expected baseURL https://api.github.com, got %s", client.baseURL)
	}

	if client.httpClient == nil {
		t.Error("Expected httpClient to be set")
	}

	if client.archivedCache == nil {
		t.Error("Expected archivedCache to be initialized")
	}
	if client.releaseCache == nil {
		t.Error("Expected releaseCache to be initialized")
	}
	if client.refSHACache == nil {
		t.Error("Expected refSHACache to be initialized")
	}
	if client.eolClient == nil {
		t.Error("Expected eolClient to be initialized")
	}
}

func TestClient_GetLatestRelease(t *testing.T) {
	tests := []struct {
		name         string
		ownerRepo    string
		responseBody string
		statusCode   int
		headers      map[string]string
		expectError  bool
		expectedTag  string
	}{
		{
			name:      "valid release",
			ownerRepo: "owner/repo",
			responseBody: `{
				"tag_name": "v1.2.3",
				"name": "Release 1.2.3",
				"draft": false,
				"prerelease": false,
				"html_url": "https://github.com/owner/repo/releases/tag/v1.2.3"
			}`,
			statusCode:  200,
			expectError: false,
			expectedTag: "v1.2.3",
		},
		{
			name:        "no releases found",
			ownerRepo:   "owner/repo",
			statusCode:  404,
			expectError: true,
		},
		{
			name:        "rate limited",
			ownerRepo:   "owner/repo",
			statusCode:  403,
			expectError: true,
		},
		{
			name:        "empty ownerRepo",
			ownerRepo:   "",
			expectError: true,
		},
		{
			name:        "invalid ownerRepo format",
			ownerRepo:   "invalid",
			expectError: true,
		},
		{
			name:         "with @ref suffix",
			ownerRepo:    "owner/repo@v1",
			responseBody: `{"tag_name": "v2.0.0"}`,
			statusCode:   200,
			expectError:  false,
			expectedTag:  "v2.0.0",
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

				for k, v := range tt.headers {
					w.Header().Set(k, v)
				}

				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					if _, err := w.Write([]byte(tt.responseBody)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
						t.Errorf("failed to write response body: %v", err)
					}
				}
			}))
			defer server.Close()

			client := newTestClient(server)
			ctx := context.Background()
			release, err := client.GetLatestRelease(ctx, tt.ownerRepo)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && release != nil {
				if release.TagName != tt.expectedTag {
					t.Errorf("Expected tag %s, got %s", tt.expectedTag, release.TagName)
				}
			}
		})
	}
}

func TestClient_GetLatestRelease_Caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"tag_name": "v1.0.0"}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	release1, err := client.GetLatestRelease(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if release1.TagName != "v1.0.0" {
		t.Errorf("Expected tag v1.0.0, got %s", release1.TagName)
	}

	release2, err := client.GetLatestRelease(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error on cached call: %v", err)
	}
	if release2.TagName != "v1.0.0" {
		t.Errorf("Expected tag v1.0.0 from cache, got %s", release2.TagName)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 API call (second should be cached), got %d", callCount)
	}
}

func TestClient_CheckMultipleReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "error") {
			w.WriteHeader(500)
			return
		}
		response := `{"tag_name": "v1.0.0"}`
		w.WriteHeader(200)
		if _, err := w.Write([]byte(response)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()
	repos := []string{"owner/repo1", "owner/error_repo"}

	releases, errors := client.CheckMultipleReleases(ctx, repos)

	if len(releases) != 1 {
		t.Errorf("Expected 1 successful result, got %d", len(releases))
	}

	if len(errors) != 1 {
		t.Errorf("Expected 1 error result, got %d", len(errors))
	}
	if _, ok := errors["owner/error_repo"]; !ok {
		t.Error("Expected error for owner/error_repo")
	}
}

func TestClient_GetRefSHA(t *testing.T) {
	tests := []struct {
		name         string
		ownerRepo    string
		ref          string
		responseBody string
		statusCode   int
		wantSHA      string
		wantError    bool
	}{
		{
			name:      "tag exists",
			ownerRepo: "owner/repo",
			ref:       "v1.0.0",
			responseBody: `{
				"ref": "refs/tags/v1.0.0",
				"object": {
					"sha": "abc123def456",
					"type": "commit",
					"url": "https://api.github.com/repos/owner/repo/git/commits/abc123def456"
				}
			}`,
			statusCode: 200,
			wantSHA:    "abc123def456",
			wantError:  false,
		},
		{
			name:      "branch exists",
			ownerRepo: "owner/repo",
			ref:       "main",
			responseBody: `{
				"ref": "refs/heads/main",
				"object": {
					"sha": "def789ghi012",
					"type": "commit",
					"url": "https://api.github.com/repos/owner/repo/git/commits/def789ghi012"
				}
			}`,
			statusCode: 200,
			wantSHA:    "def789ghi012",
			wantError:  false,
		},
		{
			name:       "ref not found",
			ownerRepo:  "owner/repo",
			ref:        "nonexistent",
			statusCode: 404,
			wantError:  true,
		},
		{
			name:      "empty ownerRepo",
			ownerRepo: "",
			ref:       "v1",
			wantError: true,
		},
		{
			name:      "invalid ownerRepo format",
			ownerRepo: "invalid",
			ref:       "v1",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				if callCount == 1 && tt.statusCode == 200 && strings.Contains(tt.name, "branch") {
					w.WriteHeader(404)
					return
				}
				if callCount == 2 && strings.Contains(tt.name, "tag") {
					t.Error("Should not try branch for tag test")
				}
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					if _, err := w.Write([]byte(tt.responseBody)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
						t.Errorf("failed to write response body: %v", err)
					}
				}
			}))
			defer server.Close()

			client := newTestClient(server)
			ctx := context.Background()
			sha, err := client.GetRefSHA(ctx, tt.ownerRepo, tt.ref)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.wantError && sha != tt.wantSHA {
				t.Errorf("Expected SHA %s, got %s", tt.wantSHA, sha)
			}
		})
	}
}

func TestClient_GetRefSHA_Caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"ref":"refs/tags/v1","object":{"sha":"cached-sha","type":"commit"}}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	sha1, err := client.GetRefSHA(ctx, "owner/repo", "v1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if sha1 != "cached-sha" {
		t.Errorf("Expected SHA cached-sha, got %s", sha1)
	}

	sha2, err := client.GetRefSHA(ctx, "owner/repo", "v1")
	if err != nil {
		t.Fatalf("Unexpected error on cached call: %v", err)
	}
	if sha2 != "cached-sha" {
		t.Errorf("Expected SHA cached-sha from cache, got %s", sha2)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 API call (second should be cached), got %d", callCount)
	}
}

func TestClient_GetRefSHA_AnnotatedTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/tags/tagobj123") {
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"object":{"sha":"commitSHA456","type":"commit"}}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
				t.Errorf("failed to write response body: %v", err)
			}
			return
		}
		if strings.Contains(r.URL.Path, "/git/refs/tags/v2") {
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v2","object":{"sha":"tagobj123","type":"tag","url":"https://api.github.com/repos/owner/repo/git/tags/tagobj123"}}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
				t.Errorf("failed to write response body: %v", err)
			}
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	sha, err := client.GetRefSHA(ctx, "owner/repo", "v2")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if sha != "commitSHA456" {
		t.Errorf("Expected dereferenced commit SHA commitSHA456, got %s", sha)
	}
}

func TestClient_GetRefSHA_AnnotatedTagDereferenceFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/tags/tagobj123") {
			w.WriteHeader(404)
			return
		}
		if strings.Contains(r.URL.Path, "/git/refs/tags/v2") {
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v2","object":{"sha":"tagobj123","type":"tag","url":"https://api.github.com/repos/owner/repo/git/tags/tagobj123"}}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
				t.Errorf("failed to write response body: %v", err)
			}
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	_, err := client.GetRefSHA(ctx, "owner/repo", "v2")
	if err == nil {
		t.Error("Expected error when tag object dereference fails")
	}
}

func TestClient_CompareRefSHAs_AnnotatedVsLightweight(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/git/refs/tags/v2.3.9"):
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v2.3.9","object":{"sha":"sameCommitSHA","type":"commit"}}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
				t.Errorf("failed to write response body: %v", err)
			}
		case strings.Contains(r.URL.Path, "/git/refs/tags/v2"):
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v2","object":{"sha":"tagobjAAA","type":"tag","url":"https://api.github.com/repos/owner/repo/git/tags/tagobjAAA"}}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
				t.Errorf("failed to write response body: %v", err)
			}
		case strings.Contains(r.URL.Path, "/git/tags/tagobjAAA"):
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"object":{"sha":"sameCommitSHA","type":"commit"}}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
				t.Errorf("failed to write response body: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	same, sha1, sha2, err := client.CompareRefSHAs(ctx, "owner/repo", "v2", "v2.3.9")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !same {
		t.Errorf("Expected SHAs to match (annotated v2 dereferences to same commit as lightweight v2.3.9), got %s vs %s", sha1, sha2)
	}
}

func TestClient_CompareRefSHAs_AnnotatedVsLightweight_Different(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/git/refs/tags/v2.3.9"):
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v2.3.9","object":{"sha":"newCommitSHA","type":"commit"}}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
				t.Errorf("failed to write response body: %v", err)
			}
		case strings.Contains(r.URL.Path, "/git/refs/tags/v2"):
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v2","object":{"sha":"tagobjAAA","type":"tag","url":"https://api.github.com/repos/owner/repo/git/tags/tagobjAAA"}}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
				t.Errorf("failed to write response body: %v", err)
			}
		case strings.Contains(r.URL.Path, "/git/tags/tagobjAAA"):
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"object":{"sha":"oldCommitSHA","type":"commit"}}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
				t.Errorf("failed to write response body: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	same, sha1, sha2, err := client.CompareRefSHAs(ctx, "owner/repo", "v2", "v2.3.9")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if same {
		t.Errorf("Expected SHAs to differ (v2 points to old commit, v2.3.9 points to new), got %s == %s", sha1, sha2)
	}
	if sha1 != "oldCommitSHA" {
		t.Errorf("Expected sha1=oldCommitSHA, got %s", sha1)
	}
	if sha2 != "newCommitSHA" {
		t.Errorf("Expected sha2=newCommitSHA, got %s", sha2)
	}
}

func TestClient_CompareRefSHAs(t *testing.T) {
	tests := []struct {
		name      string
		ownerRepo string
		ref1      string
		ref2      string
		responses []struct {
			path   string
			sha    string
			status int
		}
		wantSame  bool
		wantSHA1  string
		wantSHA2  string
		wantError bool
	}{
		{
			name:      "same SHA",
			ownerRepo: "owner/repo",
			ref1:      "v1",
			ref2:      "v1.0.0",
			responses: []struct {
				path   string
				sha    string
				status int
			}{
				{path: "v1", sha: "abc123", status: 200},
				{path: "v1.0.0", sha: "abc123", status: 200},
			},
			wantSame:  true,
			wantSHA1:  "abc123",
			wantSHA2:  "abc123",
			wantError: false,
		},
		{
			name:      "different SHA",
			ownerRepo: "owner/repo",
			ref1:      "v1",
			ref2:      "v1.0.1",
			responses: []struct {
				path   string
				sha    string
				status int
			}{
				{path: "v1", sha: "abc123", status: 200},
				{path: "v1.0.1", sha: "def456", status: 200},
			},
			wantSame:  false,
			wantSHA1:  "abc123",
			wantSHA2:  "def456",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if callCount < len(tt.responses) {
					resp := tt.responses[callCount]
					callCount++
					w.WriteHeader(resp.status)
					responseBody := fmt.Sprintf(`{
						"ref": "refs/tags/%s",
						"object": {
							"sha": "%s",
							"type": "commit"
						}
					}`, resp.path, resp.sha)
					if _, err := w.Write([]byte(responseBody)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
						t.Errorf("failed to write response body: %v", err)
					}
				}
			}))
			defer server.Close()

			client := newTestClient(server)
			ctx := context.Background()
			same, sha1, sha2, err := client.CompareRefSHAs(ctx, tt.ownerRepo, tt.ref1, tt.ref2)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.wantError {
				if same != tt.wantSame {
					t.Errorf("Expected same=%v, got %v", tt.wantSame, same)
				}
				if sha1 != tt.wantSHA1 {
					t.Errorf("Expected SHA1 %s, got %s", tt.wantSHA1, sha1)
				}
				if sha2 != tt.wantSHA2 {
					t.Errorf("Expected SHA2 %s, got %s", tt.wantSHA2, sha2)
				}
			}
		})
	}
}

func TestClient_CheckMultipleStale(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "error") {
			w.WriteHeader(500)
			return
		}
		if strings.Contains(r.URL.Path, "stale-repo") {
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{
				"name": "repo",
				"full_name": "owner/stale-repo",
				"archived": false,
				"updated_at": "2020-01-01T00:00:00Z"
			}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
				t.Errorf("failed to write response body: %v", err)
			}
			return
		}
		if strings.Contains(r.URL.Path, "fresh-repo") {
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{
				"name": "repo",
				"full_name": "owner/fresh-repo",
				"archived": false,
				"updated_at": "2099-01-01T00:00:00Z"
			}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
				t.Errorf("failed to write response body: %v", err)
			}
			return
		}
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"archived": false, "updated_at": "2025-01-01T00:00:00Z"}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	staleThreshold := 365 * 24 * time.Hour
	results, errors := client.CheckMultipleStale(ctx, []string{"owner/stale-repo", "owner/fresh-repo", "owner/error"}, staleThreshold)

	if len(errors) != 1 {
		t.Errorf("Expected 1 error result, got %d", len(errors))
	}
	if _, ok := errors["owner/error"]; !ok {
		t.Error("Expected error for owner/error")
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 stale result (stale-repo), got %d", len(results))
	}
	if _, ok := results["owner/stale-repo"]; !ok {
		t.Error("Expected stale result for owner/stale-repo")
	}
	if results["owner/stale-repo"] != nil && !results["owner/stale-repo"].StaleByAge {
		t.Error("Expected stale-repo to be flagged as stale by age")
	}
}

func TestClient_LogRateLimits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{
		"resources": {
			"core": {
				"limit": 5000,
				"remaining": 4999,
				"used": 1,
				"reset": 1640995200,
				"resource": "core"
			},
			"search": {
				"limit": 30,
				"remaining": 30,
				"used": 0,
				"reset": 1640995200,
				"resource": "search"
			}
		}
	}`)); err != nil { // nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	info, err := client.GetRateLimits(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if info.Limit != 5000 {
		t.Errorf("Expected limit 5000, got %d", info.Limit)
	}
	if info.Remaining != 4999 {
		t.Errorf("Expected remaining 4999, got %d", info.Remaining)
	}
}

func TestClient_HandleRateLimit_429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Reset", "1640995200")
		w.WriteHeader(429)
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	_, err := client.GetLatestRelease(ctx, "owner/repo")
	if err == nil {
		t.Error("Expected rate limit error for 429")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("Expected rate limit error message, got: %v", err)
	}
}

func TestCleanOwnerRepo(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"owner/repo", "owner/repo"},
		{"owner/repo@v1", "owner/repo"},
		{"https://github.com/owner/repo", "owner/repo"},
		{"  owner/repo  ", "owner/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanOwnerRepo(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
