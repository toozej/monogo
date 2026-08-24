// Package backend defines the abstraction for fetching notes from various
// backends such as Simplenote and Usememos.
package backend

import (
	"context"
	"time"
)

// Note represents a single note fetched from any backend.
type Note struct {
	Title    string
	Date     time.Time
	Tags     []string
	Content  string
	Unlisted bool
}

// Backend is the interface implemented by note sources.
type Backend interface {
	// FetchNotes retrieves notes from the backend, optionally filtered by tag.
	FetchNotes(ctx context.Context, tag string) ([]Note, error)
}
