// Package backend defines the abstraction for fetching notes from various
// backends such as Simplenote and Usememos.
package backend

import (
	"context"
	"time"
)

// Note represents a single note fetched from any backend.
type Note struct {
	// ID and Source identify the source note across repeated exports.
	ID         string
	Source     string
	Title      string
	Slug       string
	Date       time.Time
	Modified   time.Time
	Tags       []string
	SystemTags []string
	Content    string
	Unlisted   bool
}

// Destination writes notes to a target platform.
type Destination interface {
	WriteNotes(ctx context.Context, notes []Note) error
}

// Backend is the interface implemented by note sources.
type Backend interface {
	// FetchNotes retrieves notes from the backend, optionally filtered by tag.
	FetchNotes(ctx context.Context, tag string) ([]Note, error)
}
