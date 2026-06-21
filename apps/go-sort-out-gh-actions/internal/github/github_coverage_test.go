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

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/runtime"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
)

func newClientWithRawRedirect(rawServer *httptest.Server) *Client {
	transport := &redirectTransport{
		rawURL: rawServer.URL,
		base:   http.DefaultTransport,
	}
	httpClient := &http.Client{Transport: transport}
	return &Client{
		httpClient:    httpClient,
		token:         "test-token",
		baseURL:       "https://api.github.com",
		eolClient:     runtime.NewEOLClient(httpClient),
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
		cacheEnabled:  true,
		cacheTTL:      24 * time.Hour,
	}
}

type redirectTransport struct {
	rawURL string
	base   http.RoundTripper
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasPrefix(req.URL.Host, "raw.githubusercontent.com") {
		newURL := t.rawURL + req.URL.Path
		newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range req.Header {
			newReq.Header[k] = v
		}
		return t.base.RoundTrip(newReq)
	}
	return t.base.RoundTrip(req)
}

func TestNewClientWithHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"archived": false, "name": "repo"}`)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClientWithHTTP(server.URL, server.Client())
	if client.baseURL != server.URL {
		t.Errorf("Expected baseURL %s, got %s", server.URL, client.baseURL)
	}
	if client.token != "" {
		t.Errorf("Expected empty token, got %s", client.token)
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
	if client.repoInfoCache == nil {
		t.Error("Expected repoInfoCache to be initialized")
	}
	if client.eolClient == nil {
		t.Error("Expected eolClient to be initialized")
	}
}

func TestClient_setEOLClientForTest(t *testing.T) {
	eolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"schema_version":"1.0.0","result":{"name":"20","is_eol":false}}`)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer eolServer.Close()

	client := NewClient("test-token")
	client.SetEOLClientForTest(eolServer.URL, eolServer.Client())
	if client.eolClient == nil {
		t.Error("Expected eolClient to be set after setEOLClientForTest")
	}
}

func TestClient_LogRateLimits_Method(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		resp := `{"resources":{"core":{"limit":5000,"remaining":4999,"used":1,"reset":1640995200,"resource":"core"},"search":{"limit":30,"remaining":30,"used":0,"reset":1640995200,"resource":"search"}}}`
		if _, err := w.Write([]byte(resp)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	client.LogRateLimits(context.Background())
}

func TestClient_LogRateLimits_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := newTestClient(server)
	client.LogRateLimits(context.Background())
}

func TestClient_GetRateLimits_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetRateLimits(context.Background())
	if err == nil {
		t.Error("Expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected status code in error, got: %v", err)
	}
}

func TestClient_GetRateLimits_BadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`invalid json`)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetRateLimits(context.Background())
	if err == nil {
		t.Error("Expected error for bad JSON response")
	}
}

func TestClient_GetRateLimits_RequestError(t *testing.T) {
	client := newRequestErrorClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.GetRateLimits(ctx)
	if err == nil {
		t.Error("Expected network error")
	}
}

func TestClient_GetLatestRelease_RequestError(t *testing.T) {
	client := newRequestErrorClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.GetLatestRelease(ctx, "owner/repo")
	if err == nil {
		t.Error("Expected network error")
	}
}

func TestClient_GetLatestRelease_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetLatestRelease(context.Background(), "owner/repo")
	if err == nil {
		t.Error("Expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected status code in error, got: %v", err)
	}
}

func TestClient_GetLatestRelease_BadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`invalid json`)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetLatestRelease(context.Background(), "owner/repo")
	if err == nil {
		t.Error("Expected error for bad JSON")
	}
}

func TestClient_GetLatestRelease_SemverFallbackFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases/latest":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"tag_name":"custom-build-123","html_url":"https://github.com/owner/repo/releases/tag/custom-build-123"}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		case "/repos/owner/repo/tags":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`[{"name":"custom-build-123"},{"name":"another-non-semver"}]`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	release, err := client.GetLatestRelease(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if release.TagName != "custom-build-123" {
		t.Errorf("Expected original tag custom-build-123 when fallback fails, got %s", release.TagName)
	}
}

func TestClient_GetLatestSemverTag_NoSemverTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[{"name":"custom-build-123"},{"name":"nightly-20240101"}]`))
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetLatestSemverTag(context.Background(), "owner/repo")
	if err == nil {
		t.Error("Expected error when no semver tags found")
	}
	if !strings.Contains(err.Error(), "no semver tags found") {
		t.Errorf("Expected no semver tags error, got: %v", err)
	}
}

func TestClient_GetLatestSemverTag_InvalidOwnerRepo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	client := newTestClient(server)
	_, err := client.GetLatestSemverTag(context.Background(), "invalid")
	if err == nil {
		t.Error("Expected error for invalid owner/repo")
	}
}

func TestClient_GetLatestSemverTag_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Reset", "1640995200")
		w.WriteHeader(403)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetLatestSemverTag(context.Background(), "owner/repo")
	if err == nil {
		t.Error("Expected rate limit error")
	}
}

func TestClient_GetLatestSemverTag_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetLatestSemverTag(context.Background(), "owner/repo")
	if err == nil {
		t.Error("Expected not found error")
	}
}

func TestClient_GetLatestSemverTag_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetLatestSemverTag(context.Background(), "owner/repo")
	if err == nil {
		t.Error("Expected server error")
	}
}

func TestClient_GetLatestSemverTag_BadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetLatestSemverTag(context.Background(), "owner/repo")
	if err == nil {
		t.Error("Expected JSON decode error")
	}
}

func TestClient_GetLatestSemverTag_RequestError(t *testing.T) {
	client := newRequestErrorClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.GetLatestSemverTag(ctx, "owner/repo")
	if err == nil {
		t.Error("Expected network error")
	}
}

func TestIsSemverTag(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{"v1.0.0", true},
		{"v2.3.4", true},
		{"1.0.0", true},
		{"v0.0.1", true},
		{"not-semver", false},
		{"", false},
		{"v1", true},
		{"codeql-bundle-v2.25.5", false},
		{" v1.0.0 ", true},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			if got := isSemverTag(tt.tag); got != tt.want {
				t.Errorf("isSemverTag(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestClient_GetActionYML_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Reset", "1640995200")
		w.WriteHeader(403)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetActionYML(context.Background(), "owner/repo", "", "v1")
	if err == nil {
		t.Error("Expected rate limit error")
	}
}

func TestClient_GetActionYML_Non200Non404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetActionYML(context.Background(), "owner/repo", "", "v1")
	if err == nil {
		t.Error("Expected error for 500 status")
	}
}

func TestClient_GetActionYML_InvalidOwnerRepo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	client := newTestClient(server)
	_, err := client.GetActionYML(context.Background(), "invalid", "", "v1")
	if err == nil {
		t.Error("Expected error for invalid owner/repo")
	}
}

func TestClient_GetActionYML_NotFoundWithActionPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetActionYML(context.Background(), "owner/repo", "sub", "v1")
	if err == nil {
		t.Error("Expected error when action.yml not found with action path")
	}
	if !strings.Contains(err.Error(), "sub") {
		t.Errorf("Expected action path in error message, got: %v", err)
	}
}

func TestClient_GetActionYML_RequestError(t *testing.T) {
	client := newRequestErrorClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.GetActionYML(ctx, "owner/repo", "", "v1")
	if err == nil {
		t.Error("Expected network error")
	}
}

func TestClient_GetActionYML_BadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetActionYML(context.Background(), "owner/repo", "", "v1")
	if err == nil {
		t.Error("Expected JSON decode error")
	}
}

func TestClient_GetActionYML_BadBase64(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"content": "!!!invalid-base64!!!"}`))
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetActionYML(context.Background(), "owner/repo", "", "v1")
	if err == nil {
		t.Error("Expected base64 decode error")
	}
}

func TestClient_GetActionYML_YamlFallback(t *testing.T) {
	actionContent := "name: Test Action\nruns:\n  using: node20\n  main: dist/index.js\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(actionContent))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "action.yml") {
			w.WriteHeader(404)
			return
		}
		if strings.Contains(r.URL.Path, "action.yaml") {
			w.WriteHeader(200)
			_, _ = fmt.Fprintf(w, `{"content": "%s"}`, encoded)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	content, err := client.GetActionYML(context.Background(), "owner/repo", "", "v1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	using, parseErr := ParseActionYML(content)
	if parseErr != nil {
		t.Fatalf("Failed to parse action.yaml: %v", parseErr)
	}
	if using != "node20" {
		t.Errorf("Expected using=node20, got %s", using)
	}
}

func TestClient_GetRawActionYML_Success(t *testing.T) {
	actionContent := "name: Test Action\nruns:\n  using: node20\n  main: dist/index.js\n"
	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "action.yml") || strings.Contains(r.URL.Path, "action.yaml") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(actionContent))
			return
		}
		w.WriteHeader(404)
	}))
	defer rawServer.Close()

	client := newClientWithRawRedirect(rawServer)
	content, err := client.GetRawActionYML(context.Background(), "owner/repo", "", "v1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	using, parseErr := ParseActionYML(content)
	if parseErr != nil {
		t.Fatalf("Failed to parse action.yml: %v", parseErr)
	}
	if using != "node20" {
		t.Errorf("Expected using=node20, got %s", using)
	}
}

func TestClient_GetRawActionYML_WithToken(t *testing.T) {
	actionContent := "name: Test\nruns:\n  using: node20\n  main: index.js\n"
	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token test-token" {
			t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
		}
		if strings.Contains(r.URL.Path, "action.yml") || strings.Contains(r.URL.Path, "action.yaml") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(actionContent))
			return
		}
		w.WriteHeader(404)
	}))
	defer rawServer.Close()

	client := newClientWithRawRedirect(rawServer)
	client.token = "test-token"
	_, err := client.GetRawActionYML(context.Background(), "owner/repo", "", "v1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestClient_GetRawActionYML_Subpath(t *testing.T) {
	actionContent := "name: Test\nruns:\n  using: node16\n  main: index.js\n"
	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "init/action.yml") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(actionContent))
			return
		}
		w.WriteHeader(404)
	}))
	defer rawServer.Close()

	client := newClientWithRawRedirect(rawServer)
	content, err := client.GetRawActionYML(context.Background(), "owner/repo", "init", "v1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	using, parseErr := ParseActionYML(content)
	if parseErr != nil {
		t.Fatalf("Failed to parse action.yml: %v", parseErr)
	}
	if using != "node16" {
		t.Errorf("Expected using=node16, got %s", using)
	}
}

func TestClient_GetRawActionYML_Non200Non404(t *testing.T) {
	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer rawServer.Close()

	client := newClientWithRawRedirect(rawServer)
	_, err := client.GetRawActionYML(context.Background(), "owner/repo", "", "v1")
	if err == nil {
		t.Error("Expected error for 500 status")
	}
}

func TestClient_GetRawActionYML_NotFound(t *testing.T) {
	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer rawServer.Close()

	client := newClientWithRawRedirect(rawServer)
	_, err := client.GetRawActionYML(context.Background(), "owner/repo", "", "v1")
	if err == nil {
		t.Error("Expected error when action.yml not found")
	}
}

func TestClient_GetRawActionYML_NotFoundWithActionPath(t *testing.T) {
	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer rawServer.Close()

	client := newClientWithRawRedirect(rawServer)
	_, err := client.GetRawActionYML(context.Background(), "owner/repo", "sub", "v1")
	if err == nil {
		t.Error("Expected error when action.yml not found with action path")
	}
	if !strings.Contains(err.Error(), "sub") {
		t.Errorf("Expected action path in error message, got: %v", err)
	}
}

func TestClient_GetRawActionYML_InvalidOwnerRepo(t *testing.T) {
	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer rawServer.Close()
	client := newClientWithRawRedirect(rawServer)
	_, err := client.GetRawActionYML(context.Background(), "invalid", "", "v1")
	if err == nil {
		t.Error("Expected error for invalid owner/repo")
	}
}

func TestClient_GetRawActionYML_RequestError(t *testing.T) {
	client := newRequestErrorClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.GetRawActionYML(ctx, "owner/repo", "", "v1")
	if err == nil {
		t.Error("Expected network error")
	}
}

func TestClient_CompareRefSHAs_ErrorOnFirstRef(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, _, _, err := client.CompareRefSHAs(context.Background(), "owner/repo", "nonexistent", "v1")
	if err == nil {
		t.Error("Expected error when first ref not found")
	}
}

func TestClient_CompareRefSHAs_ErrorOnSecondRef(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "v1") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ref":"refs/tags/v1","object":{"sha":"abc123","type":"commit"}}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	same, sha1, sha2, err := client.CompareRefSHAs(context.Background(), "owner/repo", "v1", "nonexistent")
	if err == nil {
		t.Error("Expected error when second ref not found")
	}
	if same {
		t.Error("Expected same=false on error")
	}
	if sha1 != "abc123" {
		t.Errorf("Expected first SHA to be populated, got %s", sha1)
	}
	if sha2 != "" {
		t.Errorf("Expected second SHA to be empty, got %s", sha2)
	}
}

func TestClient_GetRefSHA_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Reset", "1640995200")
		w.WriteHeader(403)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetRefSHA(context.Background(), "owner/repo", "v1")
	if err == nil {
		t.Error("Expected rate limit error")
	}
}

func TestClient_GetRefSHA_InvalidOwnerRepo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	client := newTestClient(server)
	_, err := client.GetRefSHA(context.Background(), "invalid", "v1")
	if err == nil {
		t.Error("Expected error for invalid owner/repo")
	}
}

func TestClient_GetRefSHA_RequestError(t *testing.T) {
	client := newRequestErrorClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.GetRefSHA(ctx, "owner/repo", "v1")
	if err == nil {
		t.Error("Expected network error")
	}
}

func TestClient_GetRefSHA_BadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetRefSHA(context.Background(), "owner/repo", "v1")
	if err == nil {
		t.Error("Expected JSON decode error")
	}
}

func TestClient_GetRefSHA_BranchFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/refs/tags/main") {
			w.WriteHeader(404)
			return
		}
		if strings.Contains(r.URL.Path, "/git/refs/heads/main") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ref":"refs/heads/main","object":{"sha":"branchSHA789","type":"commit"}}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	sha, err := client.GetRefSHA(context.Background(), "owner/repo", "main")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if sha != "branchSHA789" {
		t.Errorf("Expected branchSHA789, got %s", sha)
	}
}

func TestClient_DereferenceTag_RequestError(t *testing.T) {
	client := newRequestErrorClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.dereferenceTag(ctx, "owner", "repo", "deadbeef")
	if err == nil {
		t.Error("Expected network error")
	}
}

func TestClient_DereferenceTag_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.dereferenceTag(context.Background(), "owner", "repo", "deadbeef")
	if err == nil {
		t.Error("Expected error for non-200 status")
	}
}

func TestClient_DereferenceTag_BadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.dereferenceTag(context.Background(), "owner", "repo", "deadbeef")
	if err == nil {
		t.Error("Expected JSON decode error")
	}
}

func TestClient_GetRepoInfo_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"name":"repo","full_name":"owner/repo","archived":false,"updated_at":"2025-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	client := newTestClient(server)
	info, err := client.GetRepoInfo(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if info.Name != "repo" {
		t.Errorf("Expected name=repo, got %s", info.Name)
	}
	if info.Archived {
		t.Error("Expected archived=false")
	}
}

func TestClient_GetRepoInfo_CacheHit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"name":"repo","full_name":"owner/repo","archived":true,"updated_at":"2025-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	info1, err := client.GetRepoInfo(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if info1.Name != "repo" {
		t.Errorf("Expected name=repo, got %s", info1.Name)
	}

	info2, err := client.GetRepoInfo(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error on cached call: %v", err)
	}
	if info2.Name != "repo" {
		t.Errorf("Expected name=repo from cache, got %s", info2.Name)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 API call (second should use cache), got %d", callCount)
	}
}

func TestClient_GetRepoInfo_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetRepoInfo(context.Background(), "owner/repo")
	if err == nil {
		t.Error("Expected error when repo not found")
	}
}

func TestClient_CheckMultipleRuntimeEOL_RawActionFallback(t *testing.T) {
	actionContent := "name: Test Action\nruns:\n  using: node16\n  main: dist/index.js\n"

	eolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/products/nodejs/releases/16") {
			eolFrom := "2023-09-01"
			resp := runtime.ProductReleaseResponse{
				SchemaVersion: "1.0.0",
				Result: runtime.ProductRelease{
					Name:    "16",
					IsEol:   true,
					EolFrom: &eolFrom,
				},
			}
			w.WriteHeader(200)
			body, _ := json.Marshal(resp)
			_, _ = w.Write(body)
			return
		}
		w.WriteHeader(404)
	}))
	defer eolServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer apiServer.Close()

	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "action.yml") || strings.Contains(r.URL.Path, "action.yaml") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(actionContent))
			return
		}
		w.WriteHeader(404)
	}))
	defer rawServer.Close()

	client := newClientWithRawRedirect(rawServer)
	client.baseURL = apiServer.URL
	client.eolClient = runtime.NewEOLClientWithHTTP(eolServer.URL, eolServer.Client())

	ctx := context.Background()
	refs := []workflow.ActionRef{
		{OwnerRepo: "owner/raw-action", Version: "v1"},
	}

	results, errors := client.CheckMultipleRuntimeEOL(ctx, refs)
	if len(errors) != 0 {
		t.Fatalf("Expected 0 errors, got %d: %v", len(errors), errors)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if result, ok := results["owner/raw-action@v1"]; ok {
		if result.Version != "16" {
			t.Errorf("Expected version 16, got %s", result.Version)
		}
		if !result.IsEOL {
			t.Error("Expected IsEOL to be true")
		}
	}
}

func TestClient_CheckMultipleRuntimeEOL_ParseError(t *testing.T) {
	actionContent := "{{invalid yaml"

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer apiServer.Close()

	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "action.yml") || strings.Contains(r.URL.Path, "action.yaml") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(actionContent))
			return
		}
		w.WriteHeader(404)
	}))
	defer rawServer.Close()

	client := newClientWithRawRedirect(rawServer)
	client.baseURL = apiServer.URL

	ctx := context.Background()
	refs := []workflow.ActionRef{
		{OwnerRepo: "owner/bad-action", Version: "v1"},
	}

	_, errors := client.CheckMultipleRuntimeEOL(ctx, refs)
	if len(errors) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errors))
	}
}

func TestClient_CheckMultipleRuntimeEOL_EOLCheckError(t *testing.T) {
	actionContent := "name: Test Action\nruns:\n  using: node20\n  main: dist/index.js\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(actionContent))

	eolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer eolServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "action.yml") || strings.Contains(r.URL.Path, "action.yaml") {
			w.WriteHeader(200)
			_, _ = fmt.Fprintf(w, `{"content": "%s"}`, encoded)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	client.eolClient = runtime.NewEOLClientWithHTTP(eolServer.URL, eolServer.Client())

	ctx := context.Background()
	refs := []workflow.ActionRef{
		{OwnerRepo: "owner/eol-error", Version: "v1"},
	}

	_, errors := client.CheckMultipleRuntimeEOL(ctx, refs)
	if len(errors) != 1 {
		t.Fatalf("Expected 1 error for EOL check failure, got %d", len(errors))
	}
}

func TestClient_CheckMultipleRuntimeEOL_NotEOL(t *testing.T) {
	actionContent := "name: Test Action\nruns:\n  using: node22\n  main: dist/index.js\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(actionContent))

	eolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/products/nodejs/releases/22") {
			resp := runtime.ProductReleaseResponse{
				SchemaVersion: "1.0.0",
				Result: runtime.ProductRelease{
					Name:  "22",
					IsEol: false,
				},
			}
			w.WriteHeader(200)
			body, _ := json.Marshal(resp)
			_, _ = w.Write(body)
			return
		}
		w.WriteHeader(404)
	}))
	defer eolServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "action.yml") || strings.Contains(r.URL.Path, "action.yaml") {
			w.WriteHeader(200)
			_, _ = fmt.Fprintf(w, `{"content": "%s"}`, encoded)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestClient(server)
	client.eolClient = runtime.NewEOLClientWithHTTP(eolServer.URL, eolServer.Client())

	ctx := context.Background()
	refs := []workflow.ActionRef{
		{OwnerRepo: "owner/current-action", Version: "v1"},
	}

	results, errors := client.CheckMultipleRuntimeEOL(ctx, refs)
	if len(errors) != 0 {
		t.Fatalf("Expected 0 errors, got %d", len(errors))
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results (not EOL), got %d", len(results))
	}
}

func TestClient_CheckMultipleStale_Deprecated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"name": "repo",
			"full_name": "owner/deprecated-repo",
			"archived": false,
			"updated_at": "2025-01-01T00:00:00Z",
			"deprecated": true,
			"deprecation_warning_message": "This action is deprecated, use new-action instead"
		}`))
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()
	staleThreshold := 365 * 24 * time.Hour

	results, errors := client.CheckMultipleStale(ctx, []string{"owner/deprecated-repo"}, staleThreshold)
	if len(errors) != 0 {
		t.Fatalf("Expected 0 errors, got %d", len(errors))
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results["owner/deprecated-repo"] == nil {
		t.Fatal("Expected stale result for deprecated repo")
	}
	if !results["owner/deprecated-repo"].Deprecated {
		t.Error("Expected deprecated=true")
	}
	if results["owner/deprecated-repo"].DeprecationMessage != "This action is deprecated, use new-action instead" {
		t.Errorf("Expected deprecation message, got: %s", results["owner/deprecated-repo"].DeprecationMessage)
	}
}

func TestClient_CheckMultipleStale_InvalidUpdatedAt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"name": "repo",
			"full_name": "owner/repo",
			"archived": false,
			"updated_at": "invalid-date"
		}`))
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()
	staleThreshold := 365 * 24 * time.Hour

	results, errors := client.CheckMultipleStale(ctx, []string{"owner/repo"}, staleThreshold)
	if len(errors) != 0 {
		t.Fatalf("Expected 0 errors, got %d", len(errors))
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results (invalid date means no stale-by-age), got %d", len(results))
	}
}

func TestClient_CheckMultipleStale_NotStale(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"name": "repo",
			"full_name": "owner/fresh-repo",
			"archived": false,
			"updated_at": "2099-01-01T00:00:00Z"
		}`))
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()
	staleThreshold := 365 * 24 * time.Hour

	results, errors := client.CheckMultipleStale(ctx, []string{"owner/fresh-repo"}, staleThreshold)
	if len(errors) != 0 {
		t.Fatalf("Expected 0 errors, got %d", len(errors))
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for fresh repo, got %d", len(results))
	}
}

func TestActionManifestPaths(t *testing.T) {
	tests := []struct {
		name       string
		actionPath string
		wantLen    int
		wantFirst  string
	}{
		{name: "empty action path", actionPath: "", wantLen: 2, wantFirst: "action.yml"},
		{name: "with action path", actionPath: "init", wantLen: 4, wantFirst: "init/action.yml"},
		{name: "with trailing slash", actionPath: "sub/", wantLen: 4, wantFirst: "sub/action.yml"},
		{name: "with leading slash", actionPath: "/sub", wantLen: 4, wantFirst: "sub/action.yml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := actionManifestPaths(tt.actionPath)
			if len(paths) != tt.wantLen {
				t.Errorf("Expected %d paths, got %d", tt.wantLen, len(paths))
			}
			if paths[0] != tt.wantFirst {
				t.Errorf("Expected first path %s, got %s", tt.wantFirst, paths[0])
			}
		})
	}
}

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantRepo  string
		wantError bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"owner/repo@v1", "owner", "repo", false},
		{"https://github.com/owner/repo", "owner", "repo", false},
		{"invalid", "", "", true},
		{"", "", "", true},
		{"a/b/c", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, repo, err := parseOwnerRepo(tt.input)
			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.wantError {
				if owner != tt.wantOwner {
					t.Errorf("Expected owner=%s, got %s", tt.wantOwner, owner)
				}
				if repo != tt.wantRepo {
					t.Errorf("Expected repo=%s, got %s", tt.wantRepo, repo)
				}
			}
		})
	}
}

func TestClient_LogRateLimits_NilResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	client := newTestClient(server)
	client.logRateLimits(nil)
}

func TestClient_HandleRateLimit_NotRateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := newTestClient(server)
	resp, _ := client.httpClient.Get(server.URL)
	err := client.handleRateLimit(resp)
	if err != nil {
		t.Errorf("Expected no rate limit error for 200 response, got: %v", err)
	}
}
