package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// isRunningAsRoot reports whether the current process is running as root
// (UID 0), where Unix file permissions are not enforced.
func isRunningAsRoot() bool {
	return os.Getuid() == 0
}

// skipIfRoot skips the test if running as root, since root bypasses
// Unix file permission checks and the permission-based tests would fail.
func skipIfRoot(t *testing.T) {
	t.Helper()
	if isRunningAsRoot() {
		t.Skip("skipping: test relies on Unix file permissions which are not enforced when running as root")
	}
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

func TestCacheStore_LoadSaveRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	data := map[string]string{"key": "value"}
	if err := store.Save("test", data); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	var loaded map[string]string
	if err := store.Load("test", &loaded); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded["key"] != "value" {
		t.Errorf("expected value 'value', got %q", loaded["key"])
	}
}

func TestCacheStore_Expiration(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	data := map[string]string{"key": "value"}
	if err := store.Save("expired", data); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	oldTime := time.Now().Add(-48 * time.Hour)
	path := filepath.Join(tmpDir, "expired.json")
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatalf("failed to backdate file: %v", err)
	}

	var loaded map[string]string
	err := store.Load("expired", &loaded)
	if err == nil {
		t.Error("expected Load to return error for expired cache file")
	}
}

func TestCacheStore_ClearAll(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	if err := store.Save("file1", map[string]string{"a": "1"}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if err := store.Save("file2", map[string]string{"b": "2"}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := store.ClearAll(); err != nil {
		t.Fatalf("ClearAll failed: %v", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 files after ClearAll, got %d", len(entries))
	}
}

func TestCacheStore_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	data := map[string]string{"key": "value"}
	if err := store.Save("atomic", data); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	tmpPath := filepath.Join(tmpDir, "atomic.json.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("expected no temporary file to remain after atomic write")
	}

	path := filepath.Join(tmpDir, "atomic.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected cache file to exist: %v", err)
	}
}

func TestCacheStore_IsExpired(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	if err := store.Save("fresh", map[string]string{"key": "value"}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if store.IsExpired("fresh", 24*time.Hour) {
		t.Error("expected fresh file to not be expired")
	}

	oldTime := time.Now().Add(-48 * time.Hour)
	path := filepath.Join(tmpDir, "fresh.json")
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatalf("failed to backdate file: %v", err)
	}

	if !store.IsExpired("fresh", 24*time.Hour) {
		t.Error("expected backdated file to be expired")
	}
}

func TestCacheStore_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	if err := store.Save("delete-me", map[string]string{}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := store.Clear("delete-me"); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "delete-me.json")); !os.IsNotExist(err) {
		t.Error("expected cache file to be removed")
	}
}

func TestNewCacheStore_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	store, err := NewCacheStore("test-app")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}

	if store.dir == "" {
		t.Error("expected dir to be set")
	}

	info, err := os.Stat(store.dir)
	if err != nil {
		t.Fatalf("expected cache directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected cache directory to be a directory")
	}
}

func TestCacheStore_NilStore(t *testing.T) {
	var store *CacheStore

	if err := store.Load("test", nil); err == nil {
		t.Error("expected error when Load called on nil store")
	}
	if err := store.Save("test", nil); err == nil {
		t.Error("expected error when Save called on nil store")
	}
	if err := store.Clear("test"); err == nil {
		t.Error("expected error when Clear called on nil store")
	}
	if err := store.ClearAll(); err == nil {
		t.Error("expected error when ClearAll called on nil store")
	}
	if !store.IsExpired("test", time.Hour) {
		t.Error("expected IsExpired to return true on nil store")
	}
}

func TestCacheStore_EmptyDir(t *testing.T) {
	store := &CacheStore{dir: ""}

	if err := store.Load("test", nil); err == nil {
		t.Error("expected error when Load called on empty-dir store")
	}
	if err := store.Save("test", nil); err == nil {
		t.Error("expected error when Save called on empty-dir store")
	}
	if err := store.Clear("test"); err == nil {
		t.Error("expected error when Clear called on empty-dir store")
	}
	if err := store.ClearAll(); err == nil {
		t.Error("expected error when ClearAll called on empty-dir store")
	}
	if !store.IsExpired("test", time.Hour) {
		t.Error("expected IsExpired to return true on empty-dir store")
	}
}

func TestCacheStore_Load_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	var loaded map[string]string
	if err := store.Load("nonexistent", &loaded); err == nil {
		t.Error("expected error when loading nonexistent cache file")
	}
}

func TestCacheStore_Load_UnmarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	if err := os.WriteFile(filepath.Join(tmpDir, "bad.json"), []byte("not-json"), 0600); err != nil {
		t.Fatalf("Failed to write bad cache file: %v", err)
	}

	var loaded map[string]string
	if err := store.Load("bad", &loaded); err == nil {
		t.Error("expected unmarshal error for invalid JSON")
	}
}

func TestCacheStore_Clear_NonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	if err := store.Clear("nonexistent"); err != nil {
		t.Errorf("Clear should not error for nonexistent file: %v", err)
	}
}

func TestCacheStore_ClearAll_SkipsSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	if err := store.Save("file1", map[string]string{"a": "1"}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := store.ClearAll(); err != nil {
		t.Fatalf("ClearAll failed: %v", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry (subdir) after ClearAll, got %d", len(entries))
	}
}

func TestCacheStore_Save_MarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	if err := store.Save("bad", make(chan int)); err == nil {
		t.Error("expected error when saving unmarshallable data")
	}
}

func TestCacheStore_IsExpired_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	if !store.IsExpired("nonexistent", time.Hour) {
		t.Error("expected IsExpired to return true for missing file")
	}
}

func TestNewCacheStore_XDGCacheHome(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	store, err := NewCacheStore("xdg-app")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}
	expected := filepath.Join(tmpDir, "xdg-app")
	if store.dir != expected {
		t.Errorf("expected dir %q, got %q", expected, store.dir)
	}
}

func TestNewCacheStore_DefaultCacheDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping default cache dir test on windows")
	}

	t.Setenv("XDG_CACHE_HOME", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	store, err := NewCacheStore("default-app")
	if err != nil {
		t.Fatalf("NewCacheStore failed: %v", err)
	}

	var expected string
	switch runtime.GOOS {
	case "darwin":
		expected = filepath.Join(home, "Library", "Caches", "default-app")
	default:
		expected = filepath.Join(home, ".cache", "default-app")
	}

	if store.dir != expected {
		t.Errorf("expected dir %q, got %q", expected, store.dir)
	}
}

func TestCacheStore_FilePath(t *testing.T) {
	store := &CacheStore{dir: "/tmp/cache"}
	result := store.filePath("mycache")
	expected := filepath.Join("/tmp/cache", "mycache.json")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestCacheStore_FilePath_TraversalSafe(t *testing.T) {
	store := &CacheStore{dir: "/tmp/cache"}
	result := store.filePath("../../../etc/passwd")
	expected := filepath.Join("/tmp/cache", "passwd.json")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestCacheDirWithOS(t *testing.T) {
	tests := []struct {
		name        string
		appName     string
		goos        string
		xdg         string
		localApp    string
		wantDir     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "XDG_CACHE_HOME takes priority on linux",
			appName: "myapp",
			goos:    "linux",
			xdg:     "/custom/xdg/cache",
			wantDir: filepath.Join("/custom/xdg/cache", "myapp"),
		},
		{
			name:    "XDG_CACHE_HOME takes priority on darwin",
			appName: "myapp",
			goos:    "darwin",
			xdg:     "/custom/xdg/cache",
			wantDir: filepath.Join("/custom/xdg/cache", "myapp"),
		},
		{
			name:    "XDG_CACHE_HOME takes priority on windows",
			appName: "myapp",
			goos:    "windows",
			xdg:     "/custom/xdg/cache",
			wantDir: filepath.Join("/custom/xdg/cache", "myapp"),
		},
		{
			name:    "darwin default without XDG",
			appName: "macapp",
			goos:    "darwin",
			xdg:     "",
			wantDir: filepath.Join("Library", "Caches", "macapp"),
		},
		{
			name:     "windows with LOCALAPPDATA set",
			appName:  "winapp",
			goos:     "windows",
			xdg:      "",
			localApp: "/custom/local/appdata",
			wantDir:  filepath.Join("/custom/local/appdata", "winapp", "Cache"),
		},
		{
			name:     "windows with LOCALAPPDATA empty falls back to home",
			appName:  "winapp2",
			goos:     "windows",
			xdg:      "",
			localApp: "",
		},
		{
			name:    "linux default without XDG",
			appName: "linuxapp",
			goos:    "linux",
			xdg:     "",
			wantDir: filepath.Join(".cache", "linuxapp"),
		},
		{
			name:    "freebsd default without XDG",
			appName: "bsdapp",
			goos:    "freebsd",
			xdg:     "",
			wantDir: filepath.Join(".cache", "bsdapp"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.xdg != "" {
				t.Setenv("XDG_CACHE_HOME", tt.xdg)
			} else {
				t.Setenv("XDG_CACHE_HOME", "")
			}
			if tt.goos == "windows" {
				if tt.localApp != "" {
					t.Setenv("LOCALAPPDATA", tt.localApp)
				} else {
					t.Setenv("LOCALAPPDATA", "")
				}
			}

			dir, err := cacheDirWithOS(tt.appName, tt.goos)
			if (err != nil) != tt.wantErr {
				t.Fatalf("cacheDirWithOS() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.errContains != "" && filepath.Base(err.Error()) != tt.errContains {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if tt.wantDir != "" {
				if !filepath.IsAbs(tt.wantDir) {
					if !strings.Contains(dir, tt.wantDir) {
						t.Errorf("expected dir to contain %q, got %q", tt.wantDir, dir)
					}
				} else {
					if dir != tt.wantDir {
						t.Errorf("expected %q, got %q", tt.wantDir, dir)
					}
				}
			}
			if dir == "" {
				t.Error("expected non-empty cache directory")
			}
		})
	}
}

func TestCacheDirWithOS_WindowsHomeDirFallback(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("LOCALAPPDATA", "")

	dir, err := cacheDirWithOS("winhome", "windows")
	if err != nil {
		t.Fatalf("cacheDirWithOS failed: %v", err)
	}

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		t.Skip("cannot determine home dir")
	}
	expected := filepath.Join(home, "winhome", "Cache")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestCacheDirWithOS_LinuxHomeDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	dir, err := cacheDirWithOS("linuxapp", "linux")
	if err != nil {
		t.Fatalf("cacheDirWithOS failed: %v", err)
	}
	expected := filepath.Join(home, ".cache", "linuxapp")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestCacheStore_LoadSaveComplexData(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	type ComplexData struct {
		Name   string   `json:"name"`
		Values []string `json:"values"`
		Nested struct {
			Count int `json:"count"`
		} `json:"nested"`
	}

	original := ComplexData{
		Name:   "test",
		Values: []string{"a", "b", "c"},
	}
	original.Nested.Count = 42

	if err := store.Save("complex", original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	var loaded ComplexData
	if err := store.Load("complex", &loaded); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("expected Name %q, got %q", original.Name, loaded.Name)
	}
	if loaded.Nested.Count != original.Nested.Count {
		t.Errorf("expected Count %d, got %d", original.Nested.Count, loaded.Nested.Count)
	}
}

func TestCacheStore_Save_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	data := map[string]string{"key": "value"}
	if err := store.Save("test", data); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	tmpPath := filepath.Join(tmpDir, "test.json.tmp")
	if err := os.WriteFile(tmpPath, []byte("stale"), 0600); err != nil {
		t.Fatalf("Failed to create stale tmp file: %v", err)
	}

	if err := store.Save("test", data); err != nil {
		t.Errorf("Save with existing tmp file should succeed: %v", err)
	}
}

func TestCacheStore_ClearAll_NonexistentDir(t *testing.T) {
	store := &CacheStore{dir: "/nonexistent/path/that/does/not/exist"}
	if err := store.ClearAll(); err == nil {
		t.Error("expected error when ClearAll on nonexistent directory")
	}
}

func TestCacheStore_Load_ReadError(t *testing.T) {
	skipIfDarwinOrRoot(t)
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	path := filepath.Join(tmpDir, "unreadable.json")
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("Failed to chmod file: %v", err)
	}
	defer func() {
		_ = os.Chmod(path, 0o600)
	}()

	var loaded map[string]string
	err := store.Load("unreadable", &loaded)
	if err == nil {
		t.Error("expected error when reading unreadable file")
	}
}

func TestNewCacheStore_InvalidDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/dev/null/impossible")
	_, err := NewCacheStore("test-app")
	if err == nil {
		t.Error("expected error when cache dir cannot be created")
	}
}

func TestCacheStore_Save_WriteFailure(t *testing.T) {
	skipIfDarwinOrRoot(t)
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	if err := os.Chmod(tmpDir, 0o500); err != nil {
		t.Fatalf("Failed to chmod directory: %v", err)
	}
	defer func() {
		_ = os.Chmod(tmpDir, 0o700)
	}()

	err := store.Save("readonly", map[string]string{"key": "value"})
	if err == nil {
		t.Error("expected error when writing to readonly directory")
	}
}

func TestCacheStore_IsExpired_WithTTL(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	if err := store.Save("ttl-test", map[string]string{"key": "value"}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Use a generous TTL so the "fresh" check is reliable even on slow
	// filesystems (e.g., Docker overlayfs) where Save→IsExpired latency
	// can exceed sub-millisecond durations.
	if store.IsExpired("ttl-test", 1*time.Second) {
		t.Error("expected fresh file to not be expired with short TTL")
	}

	oldTime := time.Now().Add(-2 * time.Second)
	path := filepath.Join(tmpDir, "ttl-test.json")
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatalf("failed to backdate file: %v", err)
	}

	if !store.IsExpired("ttl-test", 1*time.Second) {
		t.Error("expected backdated file to be expired with short TTL")
	}
}

func TestCacheDir_WindowsLocalAppData(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("LOCALAPPDATA", "/custom/local/appdata")

	dir, err := cacheDirWithOS("win-app", "windows")
	if err != nil {
		t.Fatalf("cacheDirWithOS failed: %v", err)
	}
	expected := filepath.Join("/custom/local/appdata", "win-app", "Cache")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestCacheDir_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		appName string
		xdg     string
	}{
		{
			name:    "XDG_CACHE_HOME set",
			appName: "myapp",
			xdg:     "/custom/xdg/cache",
		},
		{
			name:    "XDG_CACHE_HOME empty uses OS default",
			appName: "fallback-app",
			xdg:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.xdg != "" {
				t.Setenv("XDG_CACHE_HOME", tt.xdg)
			} else {
				t.Setenv("XDG_CACHE_HOME", "")
			}

			dir, err := cacheDir(tt.appName)
			if err != nil {
				t.Fatalf("cacheDir failed: %v", err)
			}

			if tt.xdg != "" {
				expected := filepath.Join(tt.xdg, tt.appName)
				if dir != expected {
					t.Errorf("expected %q, got %q", expected, dir)
				}
			}
			if dir == "" {
				t.Error("expected non-empty cache directory")
			}
		})
	}
}

func TestNewCacheStore_MkdirAllFailure(t *testing.T) {
	skipIfDarwinOrRoot(t)
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o500); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer func() {
		_ = os.Chmod(readOnlyDir, 0o700)
	}()

	t.Setenv("XDG_CACHE_HOME", filepath.Join(readOnlyDir, "nested", "path"))

	_, err := NewCacheStore("test-mkdir-fail")
	if err == nil {
		t.Error("expected error when MkdirAll fails")
	}
}

func TestCacheStore_Save_RenameTargetLocked(t *testing.T) {
	skipIfDarwinOrRoot(t)

	tmpDir := t.TempDir()
	dstDir := filepath.Join(tmpDir, "dst")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("Failed to create dst dir: %v", err)
	}
	if err := os.Chmod(dstDir, 0o000); err != nil {
		t.Fatalf("Failed to chmod dst dir: %v", err)
	}
	defer func() {
		_ = os.Chmod(dstDir, 0o755)
	}()

	crossStore := &CacheStore{dir: tmpDir, renameDst: dstDir}

	err := crossStore.Save("cross-device", map[string]string{"key": "value"})
	if err == nil {
		t.Error("expected error when rename target directory is read-only")
	}
}

func TestCacheStore_Clear_PermissionDenied(t *testing.T) {
	skipIfDarwinOrRoot(t)

	tmpDir := t.TempDir()
	deletedDir := filepath.Join(tmpDir, "deleted")
	if err := os.MkdirAll(deletedDir, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	store := &CacheStore{dir: deletedDir}
	if err := store.Save("will-delete", map[string]string{"key": "value"}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := os.Remove(filepath.Join(deletedDir, "will-delete.json")); err != nil {
		t.Fatalf("Failed to remove file: %v", err)
	}

	if err := store.Clear("will-delete"); err != nil {
		t.Errorf("Clear on already-deleted file should not error: %v", err)
	}

	readonlyDir := filepath.Join(tmpDir, "noclear")
	if err := os.MkdirAll(readonlyDir, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(readonlyDir, "target.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := os.Chmod(readonlyDir, 0o500); err != nil {
		t.Fatalf("Failed to chmod dir: %v", err)
	}
	defer func() {
		_ = os.Chmod(readonlyDir, 0o755)
	}()

	readonlyStore := &CacheStore{dir: readonlyDir}
	err := readonlyStore.Clear("target")
	if err == nil {
		t.Error("expected error when clearing file in read-only directory")
	}
}

func TestCacheStore_ClearAll_RemoveFileFailure(t *testing.T) {
	skipIfDarwinOrRoot(t)

	tmpDir := t.TempDir()
	readonlyDir := filepath.Join(tmpDir, "noclearall")
	if err := os.MkdirAll(readonlyDir, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(readonlyDir, "file1.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	if err := os.Chmod(readonlyDir, 0o500); err != nil {
		t.Fatalf("Failed to chmod dir: %v", err)
	}
	defer func() {
		_ = os.Chmod(readonlyDir, 0o755)
	}()

	readonlyStore := &CacheStore{dir: readonlyDir}
	err := readonlyStore.ClearAll()
	if err == nil {
		t.Error("expected error when removing files in read-only directory")
	}
}

func TestCacheStore_ClearAll_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	if err := store.ClearAll(); err != nil {
		t.Fatalf("ClearAll on empty directory should not error: %v", err)
	}
}

func TestCacheStore_Save_NilData(t *testing.T) {
	tmpDir := t.TempDir()
	store := &CacheStore{dir: tmpDir}

	if err := store.Save("nil-test", nil); err != nil {
		t.Fatalf("Save with nil value should succeed: %v", err)
	}

	var loaded interface{}
	if err := store.Load("nil-test", &loaded); err != nil {
		t.Fatalf("Load after saving nil should succeed: %v", err)
	}
}

func TestNewCacheStore_CacheDirError(t *testing.T) {
	skipIfRoot(t)
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "/nonexistent/home/that/does/not/exist")

	_, err := NewCacheStore("test-app")
	if err == nil {
		t.Error("expected error when cache dir cannot be determined")
	}
}

func init() {
	_ = json.Marshal
}
