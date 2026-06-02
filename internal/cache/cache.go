package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// NewCacheStore creates a CacheStore for the given application name.
// It determines the OS-specific cache directory and creates it if needed.
func NewCacheStore(appName string) (*CacheStore, error) {
	dir, err := cacheDirWithOS(appName, runtime.GOOS)
	if err != nil {
		return nil, fmt.Errorf("failed to determine cache directory: %w", err)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &CacheStore{dir: dir}, nil
}

// Load reads a named cache file and unmarshals it into v.
// Returns an error if the file is missing, expired, or cannot be unmarshaled.
func (s *CacheStore) Load(name string, v interface{}) error {
	if s == nil || s.dir == "" {
		return fmt.Errorf("cache store not initialized")
	}

	path := s.filePath(name)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cache file not found: %w", err)
	}

	// Default TTL check: if older than 24 hours, treat as expired.
	// Callers can use IsExpired for custom TTL checks before calling Load.
	if time.Since(info.ModTime()) > 24*time.Hour {
		return fmt.Errorf("cache file expired")
	}

	data, err := os.ReadFile(path) // #nosec G304 -- path is constructed internally via filePath() using filepath.Base(name) within the controlled cache directory, preventing directory traversal.
	if err != nil {
		return fmt.Errorf("failed to read cache file: %w", err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal cache file: %w", err)
	}

	return nil
}

// Save marshals v and writes it atomically to a named cache file.
func (s *CacheStore) Save(name string, v interface{}) error {
	if s == nil || s.dir == "" {
		return fmt.Errorf("cache store not initialized")
	}

	path := s.filePath(name)
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temporary cache file: %w", err)
	}

	renamePath := path
	if s.renameDst != "" {
		renamePath = filepath.Join(s.renameDst, filepath.Base(name)+".json")
	}

	if err := os.Rename(tmpPath, renamePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit cache file: %w", err)
	}

	return nil
}

// Clear removes a named cache file.
func (s *CacheStore) Clear(name string) error {
	if s == nil || s.dir == "" {
		return fmt.Errorf("cache store not initialized")
	}

	path := s.filePath(name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}
	return nil
}

// ClearAll removes all cache files in the cache directory.
func (s *CacheStore) ClearAll() error {
	if s == nil || s.dir == "" {
		return fmt.Errorf("cache store not initialized")
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, entry.Name())); err != nil {
			return fmt.Errorf("failed to remove cache file %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// IsExpired reports whether a named cache file is missing or older than the given TTL.
func (s *CacheStore) IsExpired(name string, ttl time.Duration) bool {
	if s == nil || s.dir == "" {
		return true
	}

	path := s.filePath(name)
	info, err := os.Stat(path)
	if err != nil {
		return true
	}

	return time.Since(info.ModTime()) > ttl
}

func (s *CacheStore) filePath(name string) string {
	return filepath.Join(s.dir, filepath.Base(name)+".json")
}

func cacheDir(appName string) (string, error) {
	return cacheDirWithOS(appName, runtime.GOOS)
}

func cacheDirWithOS(appName, goos string) (string, error) {
	// Respect XDG_CACHE_HOME universally when set.
	xdgCacheHome := os.Getenv("XDG_CACHE_HOME")
	if xdgCacheHome != "" {
		return filepath.Join(xdgCacheHome, appName), nil
	}

	switch goos {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Caches", appName), nil
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			localAppData = home
		}
		return filepath.Join(localAppData, appName, "Cache"), nil
	default:
		// Linux and other Unix-like systems
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".cache", appName), nil
	}
}
