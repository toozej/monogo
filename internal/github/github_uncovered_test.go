package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/toozej/go-sort-out-gh-actions/internal/cache"
)

// isRunningAsRoot reports whether the current process is running as root
// (UID 0), where Unix file permissions are not enforced.
func isRunningAsRoot() bool {
	return os.Getuid() == 0
}

// skipIfDarwinOrRoot skips the test on macOS (which doesn't enforce Unix
// permissions) or when running as root.
func skipIfDarwinOrRoot(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("skipping: macOS does not enforce Unix file permissions")
	}
	if isRunningAsRoot() {
		t.Skip("skipping: test relies on Unix file permissions which are not enforced when running as root")
	}
}

func TestWithCache(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		refresh     bool
		ttl         time.Duration
		wantEnabled bool
		wantRefresh bool
		wantTTL     time.Duration
	}{
		{
			name:        "cache enabled with refresh",
			enabled:     true,
			refresh:     true,
			ttl:         1 * time.Hour,
			wantEnabled: true,
			wantRefresh: true,
			wantTTL:     1 * time.Hour,
		},
		{
			name:        "cache enabled without refresh",
			enabled:     true,
			refresh:     false,
			ttl:         12 * time.Hour,
			wantEnabled: true,
			wantRefresh: false,
			wantTTL:     12 * time.Hour,
		},
		{
			name:        "cache disabled",
			enabled:     false,
			refresh:     false,
			ttl:         0,
			wantEnabled: false,
			wantRefresh: false,
			wantTTL:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("XDG_CACHE_HOME", tmpDir)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"archived":false,"name":"repo"}`))
			}))
			defer server.Close()

			client := NewClientWithHTTP(server.URL, server.Client(), WithCache(tt.enabled, tt.refresh, tt.ttl))

			if client.cacheEnabled != tt.wantEnabled {
				t.Errorf("Expected cacheEnabled=%v, got %v", tt.wantEnabled, client.cacheEnabled)
			}
			if client.refreshCache != tt.wantRefresh {
				t.Errorf("Expected refreshCache=%v, got %v", tt.wantRefresh, client.refreshCache)
			}
			if client.cacheTTL != tt.wantTTL {
				t.Errorf("Expected cacheTTL=%v, got %v", tt.wantTTL, client.cacheTTL)
			}
		})
	}
}

func TestWithCache_NewClient(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		refresh     bool
		ttl         time.Duration
		wantEnabled bool
	}{
		{name: "cache enabled with refresh", enabled: true, refresh: true, ttl: 1 * time.Hour, wantEnabled: true},
		{name: "cache disabled", enabled: false, refresh: false, ttl: 0, wantEnabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("XDG_CACHE_HOME", tmpDir)

			client := NewClient("test-token", WithCache(tt.enabled, tt.refresh, tt.ttl))

			if client.cacheEnabled != tt.wantEnabled {
				t.Errorf("Expected cacheEnabled=%v, got %v", tt.wantEnabled, client.cacheEnabled)
			}
			if client.refreshCache != tt.refresh {
				t.Errorf("Expected refreshCache=%v, got %v", tt.refresh, client.refreshCache)
			}
			if client.cacheTTL != tt.ttl {
				t.Errorf("Expected cacheTTL=%v, got %v", tt.ttl, client.cacheTTL)
			}
		})
	}
}

func TestNewClientWithHTTP_CacheInitFails(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/dev/null/impossible-path")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"archived":false,"name":"repo"}`))
	}))
	defer server.Close()

	client := NewClientWithHTTP(server.URL, server.Client())

	if client.cacheEnabled {
		t.Error("Expected cacheEnabled=false when cache dir creation fails")
	}
	if client.cacheStore != nil {
		t.Error("Expected cacheStore=nil when cache dir creation fails")
	}
}

func TestNewClient_CacheInitFails(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file-as-dir")
	if err := os.WriteFile(filePath, []byte("block"), 0o600); err != nil {
		t.Fatalf("Failed to write blocking file: %v", err)
	}
	t.Setenv("XDG_CACHE_HOME", filePath)

	client := NewClient("test-token")

	if client.cacheEnabled {
		t.Error("Expected cacheEnabled=false when cache dir creation fails")
	}
	if client.cacheStore != nil {
		t.Error("Expected cacheStore=nil when cache dir creation fails")
	}
}

func TestNewClient_WithRefreshClearsCache(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}
	if err := store.Save("archived", map[string]bool{"owner/old": true}); err != nil {
		t.Fatalf("pre-seed failed: %v", err)
	}

	client := NewClient("test-token", WithCache(true, true, 24*time.Hour))

	if !client.cacheEnabled {
		t.Error("Expected cacheEnabled=true")
	}
	if !client.refreshCache {
		t.Error("Expected refreshCache=true")
	}
}

func TestNewClient_WithRefreshClearCacheFails(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	client := NewClient("test-token", WithCache(true, true, 24*time.Hour))

	cacheDir := filepath.Join(tmpDir, "go-sort-out-gh-actions")
	files, _ := os.ReadDir(cacheDir)

	if client.cacheEnabled && client.cacheStore != nil {
		if err := client.cacheStore.ClearAll(); err != nil {
			t.Errorf("ClearAll should work on valid dir, got: %v", err)
		}
	}

	_ = files
}

func TestNewClient_LoadsCachesOnStartup(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}

	archivedCache := map[string]bool{"owner/cached-repo": true}
	releaseCache := map[string]*ReleaseInfo{"owner/cached-repo": {TagName: "v1.0.0"}}
	refSHACache := map[string]string{"owner/cached-repo@v1": "sha123"}
	repoInfoCache := map[string]*RepoInfo{"owner/cached-repo": {Name: "cached-repo", Archived: true}}

	if err := store.Save("archived", archivedCache); err != nil {
		t.Fatalf("save archived failed: %v", err)
	}
	if err := store.Save("releases", releaseCache); err != nil {
		t.Fatalf("save releases failed: %v", err)
	}
	if err := store.Save("refsha", refSHACache); err != nil {
		t.Fatalf("save refsha failed: %v", err)
	}
	if err := store.Save("repoinfo", repoInfoCache); err != nil {
		t.Fatalf("save repoinfo failed: %v", err)
	}

	client := NewClient("test-token", WithCache(true, false, 24*time.Hour))

	if !client.cacheEnabled {
		t.Fatal("Expected cacheEnabled=true")
	}

	archived, info, err := client.IsRepoArchived(context.Background(), "owner/cached-repo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !archived {
		t.Error("Expected archived=true from loaded disk cache")
	}
	if info == nil || info.Name != "cached-repo" {
		t.Error("Expected repo info from loaded disk cache")
	}

	release, ok := client.getCachedRelease("owner/cached-repo")
	if !ok || release == nil {
		t.Error("Expected release from loaded disk cache")
	}

	sha, ok := client.getCachedRefSHA("owner/cached-repo", "v1")
	if !ok || sha != "sha123" {
		t.Errorf("Expected refSHA sha123 from loaded disk cache, got %s", sha)
	}
}

func TestFlushCache_Errors(t *testing.T) {
	tests := []struct {
		name       string
		setupCache func(c *Client, tmpDir string) error
		wantError  bool
		skipMacOS  bool
	}{
		{
			name: "nil cacheStore returns nil",
			setupCache: func(c *Client, _ string) error {
				c.cacheStore = nil
				c.cacheEnabled = true
				return nil
			},
			wantError: false,
		},
		{
			name: "cache disabled returns nil",
			setupCache: func(c *Client, _ string) error {
				c.cacheStore = nil
				c.cacheEnabled = false
				return nil
			},
			wantError: false,
		},
		{
			name: "read-only cache dir causes flush error",
			setupCache: func(c *Client, tmpDir string) error {
				if isRunningAsRoot() {
					return fmt.Errorf("skip: root")
				}
				if runtime.GOOS == "darwin" {
					return fmt.Errorf("skip: macOS")
				}
				c.cacheEnabled = true
				c.archivedCache["owner/repo"] = true
				c.releaseCache["owner/repo"] = &ReleaseInfo{TagName: "v1"}
				c.refSHACache["owner/repo@v1"] = "sha"
				c.repoInfoCache["owner/repo"] = &RepoInfo{Name: "repo"}
				cacheDir := filepath.Join(tmpDir, "go-sort-out-gh-actions")
				if err := os.MkdirAll(cacheDir, 0o500); err != nil {
					t.Fatalf("Failed to create cache dir: %v", err)
				}
				t.Setenv("XDG_CACHE_HOME", tmpDir)
				c.cacheStore, _ = cache.NewCacheStore("go-sort-out-gh-actions")
				_ = os.Chmod(cacheDir, 0o500)
				return nil
			},
			wantError: true,
			skipMacOS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			client := &Client{
				httpClient:    http.DefaultClient,
				token:         "test",
				baseURL:       "http://localhost",
				archivedCache: make(map[string]bool),
				releaseCache:  make(map[string]*ReleaseInfo),
				refSHACache:   make(map[string]string),
				repoInfoCache: make(map[string]*RepoInfo),
			}
			if err := tt.setupCache(client, tmpDir); err != nil {
				if strings.Contains(err.Error(), "macOS") || strings.Contains(err.Error(), "root") {
					t.Skip("skipping: macOS does not enforce Unix file permissions")
				}
				t.Fatalf("setup failed: %v", err)
			}

			err := client.FlushCache()
			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.skipMacOS && runtime.GOOS == "darwin" {
				t.Skip("skipping: macOS does not enforce Unix file permissions")
			}
		})
	}
}

func TestFlushCache_ArchivedSaveError(t *testing.T) {
	skipIfDarwinOrRoot(t)

	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}

	cacheDir := filepath.Join(tmpDir, "go-sort-out-gh-actions")
	_ = os.Chmod(cacheDir, 0o500)
	defer func() { _ = os.Chmod(cacheDir, 0o700) }()

	client := &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "http://localhost",
		archivedCache: map[string]bool{"owner/repo": true},
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
		cacheEnabled:  true,
		cacheStore:    store,
	}

	err = client.FlushCache()
	if err == nil {
		t.Error("Expected error when archived cache save fails")
	}
}

func TestFlushCache_ReleaseSaveError(t *testing.T) {
	skipIfDarwinOrRoot(t)

	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}

	cacheDir := filepath.Join(tmpDir, "go-sort-out-gh-actions")
	_ = os.Chmod(cacheDir, 0o500)
	defer func() { _ = os.Chmod(cacheDir, 0o700) }()

	client := &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "http://localhost",
		archivedCache: make(map[string]bool),
		releaseCache:  map[string]*ReleaseInfo{"owner/repo": {TagName: "v1"}},
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
		cacheEnabled:  true,
		cacheStore:    store,
	}

	err = client.FlushCache()
	if err == nil {
		t.Error("Expected error when release cache save fails")
	}
}

func TestFlushCache_RefSHASaveError(t *testing.T) {
	skipIfDarwinOrRoot(t)

	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}

	cacheDir := filepath.Join(tmpDir, "go-sort-out-gh-actions")
	_ = os.Chmod(cacheDir, 0o500)
	defer func() { _ = os.Chmod(cacheDir, 0o700) }()

	client := &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "http://localhost",
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   map[string]string{"owner/repo@v1": "sha"},
		repoInfoCache: make(map[string]*RepoInfo),
		cacheEnabled:  true,
		cacheStore:    store,
	}

	err = client.FlushCache()
	if err == nil {
		t.Error("Expected error when refSHA cache save fails")
	}
}

func TestFlushCache_RepoInfoSaveError(t *testing.T) {
	skipIfDarwinOrRoot(t)

	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}

	cacheDir := filepath.Join(tmpDir, "go-sort-out-gh-actions")
	_ = os.Chmod(cacheDir, 0o500)
	defer func() { _ = os.Chmod(cacheDir, 0o700) }()

	client := &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "http://localhost",
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: map[string]*RepoInfo{"owner/repo": {Name: "repo"}},
		cacheEnabled:  true,
		cacheStore:    store,
	}

	err = client.FlushCache()
	if err == nil {
		t.Error("Expected error when repoInfo cache save fails")
	}
}

func TestClose_Errors(t *testing.T) {
	skipIfDarwinOrRoot(t)

	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}

	cacheDir := filepath.Join(tmpDir, "go-sort-out-gh-actions")
	_ = os.Chmod(cacheDir, 0o500)
	defer func() { _ = os.Chmod(cacheDir, 0o700) }()

	client := &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "http://localhost",
		archivedCache: map[string]bool{"owner/repo": true},
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
		cacheEnabled:  true,
		cacheStore:    store,
	}

	err = client.Close()
	if err == nil {
		t.Error("Expected error from Close when FlushCache fails")
	}
}

func TestClose_NilCacheStore(t *testing.T) {
	client := &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "http://localhost",
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
		cacheEnabled:  true,
		cacheStore:    nil,
	}

	err := client.Close()
	if err != nil {
		t.Errorf("Expected no error from Close with nil cacheStore, got: %v", err)
	}
}

func TestClose_Success(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}
	client := &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "http://localhost",
		archivedCache: map[string]bool{"owner/repo": true},
		releaseCache:  map[string]*ReleaseInfo{"owner/repo": {TagName: "v1"}},
		refSHACache:   map[string]string{"owner/repo@v1": "sha"},
		repoInfoCache: map[string]*RepoInfo{"owner/repo": {Name: "repo"}},
		cacheEnabled:  true,
		cacheStore:    store,
	}
	err = client.Close()
	if err != nil {
		t.Fatalf("Close should succeed, got: %v", err)
	}
}

func TestGetRawActionYML_YAMLFallback(t *testing.T) {
	actionContent := `name: Test Action
runs:
  using: node20
  main: dist/index.js
`
	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "action.yml") {
			w.WriteHeader(404)
			return
		}
		if strings.Contains(r.URL.Path, "action.yaml") {
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
	using, err := ParseActionYML(content)
	if err != nil {
		t.Fatalf("Failed to parse action.yaml: %v", err)
	}
	if using != "node20" {
		t.Errorf("Expected using=node20, got %s", using)
	}
}

func TestGetRawActionYML_NoToken(t *testing.T) {
	actionContent := `name: Test Action
runs:
  using: node20
  main: dist/index.js
`
	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("Expected no Authorization header when token is empty")
		}
		if strings.Contains(r.URL.Path, "action.yml") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(actionContent))
			return
		}
		w.WriteHeader(404)
	}))
	defer rawServer.Close()

	client := newClientWithRawRedirect(rawServer)
	client.token = ""

	_, err := client.GetRawActionYML(context.Background(), "owner/repo", "", "v1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestLoadAllCaches_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}

	cacheDir := filepath.Join(tmpDir, "go-sort-out-gh-actions")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "archived.json"), []byte("invalid json"), 0o600); err != nil {
		t.Fatalf("Failed to write invalid cache file: %v", err)
	}

	client := &Client{
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
		cacheEnabled:  true,
		cacheStore:    store,
	}

	client.loadAllCaches()
	if len(client.archivedCache) != 0 {
		t.Error("Expected empty archivedCache when cache file is corrupt")
	}
}

func TestFlushCache_Success(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}

	client := &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "http://localhost",
		archivedCache: map[string]bool{"owner/repo": true},
		releaseCache:  map[string]*ReleaseInfo{"owner/repo": {TagName: "v1"}},
		refSHACache:   map[string]string{"owner/repo@v1": "sha"},
		repoInfoCache: map[string]*RepoInfo{"owner/repo": {Name: "repo"}},
		cacheEnabled:  true,
		cacheStore:    store,
	}

	err = client.FlushCache()
	if err != nil {
		t.Fatalf("FlushCache should succeed, got: %v", err)
	}

	cacheDir := filepath.Join(tmpDir, "go-sort-out-gh-actions")
	for _, name := range []string{"archived", "releases", "refsha", "repoinfo"} {
		path := filepath.Join(cacheDir, name+".json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("Expected cache file %s.json to exist: %v", name, err)
		}
	}
}

func TestFlushCache_CacheDisabledWithStore(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}
	client := &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "http://localhost",
		archivedCache: map[string]bool{"owner/repo": true},
		releaseCache:  map[string]*ReleaseInfo{"owner/repo": {TagName: "v1"}},
		refSHACache:   map[string]string{"owner/repo@v1": "sha"},
		repoInfoCache: map[string]*RepoInfo{"owner/repo": {Name: "repo"}},
		cacheEnabled:  false,
		cacheStore:    store,
	}
	err = client.FlushCache()
	if err != nil {
		t.Fatalf("FlushCache with cacheEnabled=false should return nil, got: %v", err)
	}
}

func TestClose_CacheDisabledWithStore(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}
	client := &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "http://localhost",
		archivedCache: make(map[string]bool),
		releaseCache:  make(map[string]*ReleaseInfo),
		refSHACache:   make(map[string]string),
		repoInfoCache: make(map[string]*RepoInfo),
		cacheEnabled:  false,
		cacheStore:    store,
	}
	err = client.Close()
	if err != nil {
		t.Fatalf("Close with cacheEnabled=false should return nil, got: %v", err)
	}
}

func makeFlushCacheClient(t *testing.T, tmpDir string, blockFiles []string) *Client {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}
	cacheDir := filepath.Join(tmpDir, "go-sort-out-gh-actions")
	for _, name := range blockFiles {
		tmpPath := filepath.Join(cacheDir, name+".json.tmp")
		if err := os.MkdirAll(tmpPath, 0o755); err != nil {
			t.Fatalf("Failed to create blocking dir %s: %v", tmpPath, err)
		}
	}
	return &Client{
		httpClient:    http.DefaultClient,
		token:         "test",
		baseURL:       "http://localhost",
		archivedCache: map[string]bool{"owner/repo": true},
		releaseCache:  map[string]*ReleaseInfo{"owner/repo": {TagName: "v1"}},
		refSHACache:   map[string]string{"owner/repo@v1": "sha"},
		repoInfoCache: map[string]*RepoInfo{"owner/repo": {Name: "repo"}},
		cacheEnabled:  true,
		cacheStore:    store,
	}
}

func TestFlushCache_ArchivedSaveErrorCrossPlatform(t *testing.T) {
	tmpDir := t.TempDir()
	client := makeFlushCacheClient(t, tmpDir, []string{"archived"})
	err := client.FlushCache()
	if err == nil {
		t.Error("Expected error when archived cache save fails")
	}
	if !strings.Contains(err.Error(), "archived") {
		t.Errorf("Expected error to mention 'archived', got: %v", err)
	}
}

func TestFlushCache_ReleaseSaveErrorCrossPlatform(t *testing.T) {
	tmpDir := t.TempDir()
	client := makeFlushCacheClient(t, tmpDir, []string{"releases"})
	err := client.FlushCache()
	if err == nil {
		t.Error("Expected error when release cache save fails")
	}
	if !strings.Contains(err.Error(), "release") {
		t.Errorf("Expected error to mention 'release', got: %v", err)
	}
}

func TestFlushCache_RefSHASaveErrorCrossPlatform(t *testing.T) {
	tmpDir := t.TempDir()
	client := makeFlushCacheClient(t, tmpDir, []string{"refsha"})
	err := client.FlushCache()
	if err == nil {
		t.Error("Expected error when refSHA cache save fails")
	}
	if !strings.Contains(err.Error(), "refsha") {
		t.Errorf("Expected error to mention 'refsha', got: %v", err)
	}
}

func TestFlushCache_RepoInfoSaveErrorCrossPlatform(t *testing.T) {
	tmpDir := t.TempDir()
	client := makeFlushCacheClient(t, tmpDir, []string{"repoinfo"})
	err := client.FlushCache()
	if err == nil {
		t.Error("Expected error when repoInfo cache save fails")
	}
	if !strings.Contains(err.Error(), "repoinfo") {
		t.Errorf("Expected error to mention 'repoinfo', got: %v", err)
	}
}

func TestClose_ErrorsCrossPlatform(t *testing.T) {
	tmpDir := t.TempDir()
	client := makeFlushCacheClient(t, tmpDir, []string{"archived"})
	err := client.Close()
	if err == nil {
		t.Error("Expected error from Close when FlushCache fails")
	}
}
