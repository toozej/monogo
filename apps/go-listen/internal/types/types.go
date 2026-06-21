package types

import (
	"net/http"
	"time"
)

// SpotifyService defines the interface for Spotify API operations
type SpotifyService interface {
	SearchArtist(query string) (*Artist, error)
	GetArtistTopTracks(artistID string) ([]Track, error)
	GetUserPlaylists(folderName string) ([]Playlist, error)
	AddTracksToPlaylist(playlistID string, trackIDs []string) error
	CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error)
	GetAuthURL() string
	IsAuthenticated() bool
	CompleteAuth(code, state string) error
}

// PlaylistManager defines the interface for playlist management operations
type PlaylistManager interface {
	AddArtistToPlaylist(artistName, playlistID string, force bool) (*AddResult, error)
	GetIncomingPlaylists() ([]Playlist, error)
	GetTop5Tracks(artistID string) ([]Track, error)
	FilterPlaylistsBySearch(playlists []Playlist, searchTerm string) []Playlist
	AddTracksToPlaylist(playlistID string, trackIDs []string) error
	CheckForDuplicates(playlistID string, trackIDs []string) (*DuplicateResult, error)
}

// DuplicateDetector defines the interface for duplicate detection
type DuplicateDetector interface {
	CheckDuplicates(playlistID string, tracks []Track) (*DuplicateResult, error)
	CheckArtistInPlaylist(playlistID, artistID string) (*DuplicateResult, error)
}

// ArtistSearcher defines the interface for artist search with fuzzy matching
type ArtistSearcher interface {
	FindBestMatch(query string) (*Artist, float64, error)
}

// RateLimiter defines the interface for rate limiting functionality
type RateLimiter interface {
	Allow(ip string) bool
	Reset(ip string)
}

// SecurityMiddleware defines the interface for security middleware
type SecurityMiddleware interface {
	SecurityHeaders(next http.Handler) http.Handler
	RateLimit(next http.Handler) http.Handler
	InputValidation(next http.Handler) http.Handler
	CSRFProtection(next http.Handler) http.Handler
	GenerateCSRFToken() string
}

// Core data models

// Artist represents a Spotify artist
type Artist struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	URI    string   `json:"uri"`
	Genres []string `json:"genres"`
}

// Track represents a Spotify track
type Track struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	URI      string   `json:"uri"`
	Artists  []Artist `json:"artists"`
	Duration int      `json:"duration_ms"`
}

// Playlist represents a Spotify playlist
type Playlist struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	URI        string `json:"uri"`
	TrackCount int    `json:"track_count"`
	EmbedURL   string `json:"embed_url"`
	IsIncoming bool   `json:"is_incoming"`
}

// AddResult represents the result of adding an artist to a playlist
type AddResult struct {
	Success      bool     `json:"success"`
	Artist       Artist   `json:"artist"`
	TracksAdded  []Track  `json:"tracks_added"`
	Playlist     Playlist `json:"playlist"`
	WasDuplicate bool     `json:"was_duplicate"`
	Message      string   `json:"message"`
}

// DuplicateResult represents the result of duplicate detection
type DuplicateResult struct {
	HasDuplicates   bool      `json:"has_duplicates"`
	DuplicateTracks []Track   `json:"duplicate_tracks"`
	LastAdded       time.Time `json:"last_added"`
	ArtistName      string    `json:"artist_name"`
	Message         string    `json:"message"`
}

// API request/response models

// AddArtistRequest represents the request to add an artist
type AddArtistRequest struct {
	ArtistName string `json:"artist_name" validate:"required,min=1,max=100"`
	PlaylistID string `json:"playlist_id" validate:"required"`
	Force      bool   `json:"force"`
}

// APIResponse represents a generic API response
type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// PlaylistSearchRequest represents a request to search playlists
type PlaylistSearchRequest struct {
	SearchTerm string `json:"search_term" validate:"max=100"`
}

// ScrapeArtistsRequest represents a request to scrape artists from a URL
type ScrapeArtistsRequest struct {
	URL         string `json:"url" validate:"required,url"`
	CSSSelector string `json:"css_selector" validate:"max=500"`
	PlaylistID  string `json:"playlist_id" validate:"required"`
	Force       bool   `json:"force"`
}

// ScrapeArtistsResponse represents the response from scraping artists
type ScrapeArtistsResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// WebUIResponse represents a response for the web UI
type WebUIResponse struct {
	Success     bool       `json:"success"`
	Message     string     `json:"message"`
	Data        any        `json:"data,omitempty"`
	IsDuplicate bool       `json:"is_duplicate,omitempty"`
	LastAdded   *time.Time `json:"last_added,omitempty"`
}
