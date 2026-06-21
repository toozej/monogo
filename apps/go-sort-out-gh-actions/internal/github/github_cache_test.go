package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/toozej/go-sort-out-gh-actions/internal/cache"
)

func TestClient_CacheLoadAndSave(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"archived":true,"name":"repo","full_name":"owner/repo"}`)); err != nil {
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	// Use a real CacheStore in a temp directory via NewCacheStore
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	// First client: populates in-memory and disk cache
	client1 := newTestClient(server)
	client1.cacheStore, _ = cache.NewCacheStore("go-sort-out-gh-actions")
	client1.cacheEnabled = true
	client1.cacheTTL = 24 * time.Hour

	ctx := context.Background()
	archived1, _, err := client1.IsRepoArchived(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !archived1 {
		t.Error("Expected archived=true")
	}

	// Flush cache to disk
	if err := client1.FlushCache(); err != nil {
		t.Fatalf("FlushCache failed: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("Expected 1 API call, got %d", callCount)
	}

	// Second client: loads from disk, should not make API call
	client2 := newTestClient(server)
	client2.cacheStore, _ = cache.NewCacheStore("go-sort-out-gh-actions")
	client2.cacheEnabled = true
	client2.cacheTTL = 24 * time.Hour
	client2.loadAllCaches()

	archived2, _, err := client2.IsRepoArchived(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error on cached call: %v", err)
	}
	if !archived2 {
		t.Error("Expected archived=true from cache")
	}

	if callCount != 1 {
		t.Errorf("Expected still 1 API call after loading disk cache, got %d", callCount)
	}
}

func TestClient_NoCacheMode(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"archived":true,"name":"repo","full_name":"owner/repo"}`)); err != nil {
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	client.cacheEnabled = false

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
		t.Fatalf("Unexpected error: %v", err)
	}
	if !archived2 {
		t.Error("Expected archived=true")
	}

	if callCount != 2 {
		t.Errorf("Expected 2 API calls with cache disabled, got %d", callCount)
	}
}

func TestClient_RefreshCacheClearsDisk(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"archived":true,"name":"repo","full_name":"owner/repo"}`)); err != nil {
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	// Pre-seed a stale disk cache
	store, err := cache.NewCacheStore("go-sort-out-gh-actions")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}
	if err := store.Save("archived", map[string]bool{"owner/repo": false}); err != nil {
		t.Fatalf("pre-seed failed: %v", err)
	}

	// Create client with refresh mode
	client := newTestClient(server)
	client.cacheStore = store
	client.cacheEnabled = true
	client.refreshCache = true
	client.cacheTTL = 24 * time.Hour

	// Refresh mode should clear disk cache
	if err := client.cacheStore.ClearAll(); err != nil {
		t.Fatalf("ClearAll failed: %v", err)
	}

	ctx := context.Background()
	archived, _, err := client.IsRepoArchived(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !archived {
		t.Error("Expected archived=true (stale cache should have been ignored)")
	}

	if callCount != 1 {
		t.Errorf("Expected 1 API call after refresh, got %d", callCount)
	}
}

func TestClient_CloseFlushesCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"archived":true,"name":"repo","full_name":"owner/repo"}`)); err != nil {
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	client := newTestClient(server)
	client.cacheStore, _ = cache.NewCacheStore("go-sort-out-gh-actions")
	client.cacheEnabled = true
	client.cacheTTL = 24 * time.Hour

	ctx := context.Background()
	_, _, err := client.IsRepoArchived(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	cacheFile := filepath.Join(tmpDir, "go-sort-out-gh-actions", "archived.json")
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Error("Expected cache file to be written after Close")
	}
}
