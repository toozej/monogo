// Package cache provides a persistent on-disk cache for JSON-serializable data.
//
// It determines an OS-specific cache directory, supports TTL-based expiration,
// and uses atomic file writes (temp + rename) to prevent partial cache files.
package cache

// CacheStore manages persistent disk cache files in an OS-specific directory.
type CacheStore struct {
	dir       string
	renameDst string
}
