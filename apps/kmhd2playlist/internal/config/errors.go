// Package config provides error definitions for configuration-related errors.
package config

import "errors"

// Configuration validation errors
var (
	// ErrMissingSpotifyClientID is returned when Spotify Client ID is not provided
	ErrMissingSpotifyClientID = errors.New("spotify client ID is required")

	// ErrMissingSpotifyClientSecret is returned when Spotify Client Secret is not provided
	ErrMissingSpotifyClientSecret = errors.New("spotify client secret is required")

	// ErrMissingSpotifyPlaylistID is returned when Spotify Playlist ID is not provided
	ErrMissingSpotifyPlaylistID = errors.New("spotify playlist ID is required")

	// ErrMissingSpotifyUsername is returned when Spotify Username is not provided
	ErrMissingSpotifyUsername = errors.New("spotify username is required")

	// ErrMissingYouTubeMusicCookie is returned when YouTube Music cookie is not provided
	ErrMissingYouTubeMusicCookie = errors.New("youtube music cookie is required")

	// ErrInvalidMusicClient is returned when MUSIC_CLIENT is not 'spotify' or 'youtube'
	ErrInvalidMusicClient = errors.New("MUSIC_CLIENT must be 'spotify' or 'youtube'")
)
