package spotify

import (
	"time"

	"golang.org/x/oauth2"
)

// Artist represents a Spotify artist
type Artist struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	URI    string   `json:"uri"`
	Genres []string `json:"genres"`
}

// Album represents a Spotify album
type Album struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"album_type"`
}

// Track represents a Spotify track
type Track struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	URI      string   `json:"uri"`
	Artists  []Artist `json:"artists"`
	Duration int      `json:"duration_ms"`
	Album    Album    `json:"album"`
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

// SpotifyService defines the interface for Spotify operations
type SpotifyService interface {
	SearchArtist(query string) (*Artist, error)
	GetArtistTopTracks(artistID string) ([]Track, error)
	GetUserPlaylists(folderName string) ([]Playlist, error)
	AddTracksToPlaylist(playlistID string, trackIDs []string) error
	CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error)
}

// AuthResult represents the result of authentication
type AuthResult struct {
	Token     *oauth2.Token
	ExpiresAt time.Time
}
