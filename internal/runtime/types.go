package runtime

import (
	"net/http"
	"sync"
	"time"
)

type RuntimeEOLInfo struct {
	Runtime string
	Version string
	EOLDate time.Time
	IsEOL   bool
}

type EOLClient struct {
	httpClient *http.Client
	baseURL    string
	cache      map[string]*RuntimeEOLInfo
	cacheMu    sync.RWMutex
}

type ProductReleaseResponse struct {
	SchemaVersion string         `json:"schema_version"`
	GeneratedAt   string         `json:"generated_at"`
	Result        ProductRelease `json:"result"`
}

type ProductRelease struct {
	Name        string  `json:"name"`
	IsEol       bool    `json:"isEol"`
	EolFrom     *string `json:"eolFrom"`
	ReleaseDate string  `json:"releaseDate"`
}
