package releases

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// Source provides normalized release metadata and asset streams.
// IDs must remain stable across runs and unique within each repository.
type Source interface {
	VerifyRepository(context.Context, string) error
	Releases(context.Context, string) ([]*Release, error)
	Assets(context.Context, string, int64) ([]*Asset, error)
	Download(context.Context, string, int64) (io.ReadCloser, error)
}

// Timestamp stores a publication or modification time.
type Timestamp struct{ time.Time }

// Release stores the metadata used for release selection and retention.
type Release struct {
	ID          int64
	TagName     string
	Name        *string
	Draft       bool
	Prerelease  bool
	PublishedAt *Timestamp
}

func (r *Release) GetID() int64        { return r.ID }
func (r *Release) GetTagName() string  { return r.TagName }
func (r *Release) GetName() string     { return value(r.Name) }
func (r *Release) GetDraft() bool      { return r.Draft }
func (r *Release) GetPrerelease() bool { return r.Prerelease }

// Asset stores download metadata. A nil Size means the forge does not provide a size.
type Asset struct {
	ID        *int64
	Name      *string
	Size      *int
	State     *string
	Digest    *string
	UpdatedAt *Timestamp
}

func value[T any](p *T) (v T) {
	if p != nil {
		return *p
	}
	return v
}
func (a *Asset) GetID() int64 {
	if a == nil {
		return 0
	}
	return value(a.ID)
}
func (a *Asset) GetName() string {
	if a == nil {
		return ""
	}
	return value(a.Name)
}
func (a *Asset) GetSize() int {
	if a == nil {
		return 0
	}
	return value(a.Size)
}
func (a *Asset) GetState() string {
	if a == nil {
		return ""
	}
	return value(a.State)
}
func (a *Asset) GetDigest() string {
	if a == nil {
		return ""
	}
	return value(a.Digest)
}

func stableID(key string) int64 {
	sum := sha256.Sum256([]byte(key))
	return int64(binary.BigEndian.Uint64(sum[:8]) & ((1 << 63) - 1)) // #nosec G115 -- Mask limits the value to int64.
}

// Sources selects a client by repository hostname.
type Sources map[string]Source

func (s Sources) selectSource(repo string) (Source, string, error) {
	host, path, err := repositoryParts(repo)
	if err != nil {
		return nil, "", err
	}
	source := s[host]
	if source == nil {
		return nil, "", fmt.Errorf("no release source for %s", host)
	}
	return source, path, nil
}

func (s Sources) VerifyRepository(ctx context.Context, repo string) error {
	source, path, err := s.selectSource(repo)
	if err != nil {
		return err
	}
	return source.VerifyRepository(ctx, path)
}
func (s Sources) Releases(ctx context.Context, repo string) ([]*Release, error) {
	source, path, err := s.selectSource(repo)
	if err != nil {
		return nil, err
	}
	return source.Releases(ctx, path)
}
func (s Sources) Assets(ctx context.Context, repo string, id int64) ([]*Asset, error) {
	source, path, err := s.selectSource(repo)
	if err != nil {
		return nil, err
	}
	return source.Assets(ctx, path, id)
}
func (s Sources) Download(ctx context.Context, repo string, id int64) (io.ReadCloser, error) {
	source, path, err := s.selectSource(repo)
	if err != nil {
		return nil, err
	}
	return source.Download(ctx, path, id)
}
