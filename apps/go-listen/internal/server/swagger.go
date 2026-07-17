package server

import (
	"github.com/toozej/monogo/apps/go-listen/internal/services/scraper"
	"github.com/toozej/monogo/apps/go-listen/internal/types"
)

// SwaggerCSRFTokenResponse documents the CSRF token response.
type SwaggerCSRFTokenResponse struct {
	CSRFToken string `json:"csrf_token"`
}

// SwaggerPlaylistsResponse documents a successful playlist-list response.
type SwaggerPlaylistsResponse struct {
	Success bool             `json:"success"`
	Data    []types.Playlist `json:"data"`
}

// SwaggerAddArtistResponse documents a successful or duplicate add-artist
// response. A duplicate is represented by Success=false and IsDuplicate=true
// while still returning HTTP 200 so the caller can choose to retry with force.
type SwaggerAddArtistResponse struct {
	Success     bool             `json:"success"`
	Message     string           `json:"message"`
	Data        *types.AddResult `json:"data,omitempty"`
	IsDuplicate bool             `json:"is_duplicate,omitempty"`
}

// SwaggerScrapeArtistsResponse documents a successful scrape response.
type SwaggerScrapeArtistsResponse struct {
	Success bool                  `json:"success"`
	Data    *scraper.ScrapeResult `json:"data"`
	Error   string                `json:"error,omitempty"`
}

// SwaggerAuthStatusResponse documents the Spotify authentication status
// response.
type SwaggerAuthStatusResponse struct {
	Success bool                  `json:"success"`
	Data    SwaggerAuthStatusData `json:"data"`
}

// SwaggerAuthStatusData contains the current Spotify session state.
type SwaggerAuthStatusData struct {
	Authenticated bool   `json:"authenticated"`
	AuthURL       string `json:"auth_url"`
}
