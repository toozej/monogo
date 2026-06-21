package server

import (
	"github.com/toozej/go-listen/internal/types"
)

// Re-export types for backward compatibility
type SpotifyService = types.SpotifyService
type PlaylistManager = types.PlaylistManager
type DuplicateDetector = types.DuplicateDetector
type ArtistSearcher = types.ArtistSearcher
type RateLimiter = types.RateLimiter

type Artist = types.Artist
type Track = types.Track
type Playlist = types.Playlist
type AddResult = types.AddResult
type DuplicateResult = types.DuplicateResult

type AddArtistRequest = types.AddArtistRequest
type APIResponse = types.APIResponse
type PlaylistSearchRequest = types.PlaylistSearchRequest
type ScrapeArtistsRequest = types.ScrapeArtistsRequest
type ScrapeArtistsResponse = types.ScrapeArtistsResponse
type WebUIResponse = types.WebUIResponse
