package types

import (
	"fmt"
	"time"
)

// SpotifyService defines the interface for Spotify API operations
type SpotifyService interface {
	SearchArtist(query string) (*Artist, error)
	GetArtistTopTracks(artistID string) ([]Track, error)
	GetUserPlaylists(folderName string) ([]Playlist, error)
	AddTracksToPlaylist(playlistID string, trackIDs []string) error
	CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error)
	CreatePlaylist(name, description string, public bool) (*Playlist, error)
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
}

// DuplicateDetector defines the interface for duplicate detection
type DuplicateDetector interface {
	CheckDuplicates(playlistID string, tracks []Track) (*DuplicateResult, error)
	CheckArtistInPlaylist(playlistID, artistID string) (*DuplicateResult, error)
}

// KMHDScraper defines the interface for KMHD playlist scraping operations
type KMHDScraper interface {
	ScrapePlaylist() (*SongCollection, error)
	GetCurrentlyPlaying() (*Song, error)
}

// Core data models

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

// WebUIResponse represents a response for the web UI
type WebUIResponse struct {
	Success     bool       `json:"success"`
	Message     string     `json:"message"`
	Data        any        `json:"data,omitempty"`
	IsDuplicate bool       `json:"is_duplicate,omitempty"`
	LastAdded   *time.Time `json:"last_added,omitempty"`
}

// Scraping models

// Song represents a song scraped from KMHD playlist
type Song struct {
	Artist   string    `json:"artist"`
	Title    string    `json:"title"`
	Album    string    `json:"album"`
	PlayedAt time.Time `json:"played_at"`
	RawText  string    `json:"raw_text"`
}

// IsValid checks if the song has the minimum required fields
func (s *Song) IsValid() bool {
	return s.Artist != "" && s.Title != ""
}

// String returns a string representation of the song
func (s *Song) String() string {
	if s.Album != "" {
		return fmt.Sprintf("%s - %s (%s)", s.Artist, s.Title, s.Album)
	}
	return fmt.Sprintf("%s - %s", s.Artist, s.Title)
}

// SongCollection represents a collection of scraped songs
type SongCollection struct {
	Songs       []Song    `json:"songs"`
	LastUpdated time.Time `json:"last_updated"`
	Source      string    `json:"source"`
}

// AddSong adds a song to the collection
func (sc *SongCollection) AddSong(song Song) {
	sc.Songs = append(sc.Songs, song)
}
