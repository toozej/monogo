package playlist

import (
	"log/slog"
	"strings"

	"github.com/toozej/monogo/apps/go-listen/internal/types"
)

// PlaylistService implements the PlaylistManager interface
type PlaylistService struct {
	spotify   types.SpotifyService
	duplicate types.DuplicateDetector
	logger    *slog.Logger
}

// NewPlaylistService creates a new playlist service
func NewPlaylistService(spotify types.SpotifyService, duplicate types.DuplicateDetector, logger *slog.Logger) *PlaylistService {
	return &PlaylistService{
		spotify:   spotify,
		duplicate: duplicate,
		logger:    logger,
	}
}

// NewService creates a new playlist service (alias for NewPlaylistService)
func NewService(spotify types.SpotifyService, logger *slog.Logger) *PlaylistService {
	return &PlaylistService{
		spotify: spotify,
		logger:  logger,
	}
}

// AddArtistToPlaylist adds an artist's top tracks to a playlist
func (p *PlaylistService) AddArtistToPlaylist(artistName, playlistID string, force bool) (*types.AddResult, error) {
	p.logger.Info("Starting to add artist to playlist",
		"component", "playlist_service",
		"operation", "add_artist",
		"artist_name", artistName,
		"playlist_id", playlistID,
		"force", force,
	)

	// Search for the artist
	artist, err := p.spotify.SearchArtist(artistName)
	if err != nil {
		p.logger.Error("Failed to search for artist",
			"error", err,
			"component", "playlist_service",
			"operation", "add_artist",
			"artist_name", artistName,
		)
		return &types.AddResult{
			Success: false,
			Message: "Failed to find artist: " + err.Error(),
		}, err
	}

	// Get the artist's top 5 tracks
	tracks, err := p.spotify.GetArtistTopTracks(artist.ID)
	if err != nil {
		p.logger.Error("Failed to get artist top tracks",
			"error", err,
			"component", "playlist_service",
			"operation", "add_artist",
			"artist_id", artist.ID,
			"artist_name", artist.Name,
		)
		return &types.AddResult{
			Success: false,
			Artist:  *artist,
			Message: "Failed to get artist's top tracks: " + err.Error(),
		}, err
	}

	if len(tracks) == 0 {
		p.logger.Warn("Artist has no tracks available",
			"component", "playlist_service",
			"operation", "add_artist",
			"artist_id", artist.ID,
			"artist_name", artist.Name,
		)
		return &types.AddResult{
			Success: false,
			Artist:  *artist,
			Message: "Artist has no tracks available",
		}, nil
	}

	// Check for duplicates if not forced
	var wasDuplicate bool
	if !force && p.duplicate != nil {
		duplicateResult, err := p.duplicate.CheckArtistInPlaylist(playlistID, artist.ID)
		if err != nil {
			p.logger.Warn("Failed to check for duplicates, proceeding anyway",
				"error", err,
				"component", "playlist_service",
				"operation", "duplicate_check",
				"artist_id", artist.ID,
				"playlist_id", playlistID,
			)
		} else if duplicateResult != nil && duplicateResult.HasDuplicates {
			p.logger.Info("Artist tracks already exist in playlist",
				"component", "playlist_service",
				"operation", "duplicate_check",
				"artist_id", artist.ID,
				"artist_name", artist.Name,
				"playlist_id", playlistID,
				"last_added", duplicateResult.LastAdded,
				"has_duplicates", true,
			)

			return &types.AddResult{
				Success:      false,
				Artist:       *artist,
				WasDuplicate: true,
				Message:      duplicateResult.Message,
			}, nil
		}
	}

	// Extract track IDs and names for logging
	trackIDs := make([]string, len(tracks))
	trackNames := make([]string, len(tracks))
	for i, track := range tracks {
		trackIDs[i] = track.ID
		trackNames[i] = track.Name
	}

	// Add tracks to playlist in batch with error handling
	err = p.spotify.AddTracksToPlaylist(playlistID, trackIDs)
	if err != nil {
		p.logger.Error("Failed to add tracks to playlist",
			"error", err,
			"component", "playlist_service",
			"operation", "add_tracks",
			"playlist_id", playlistID,
			"track_count", len(trackIDs),
			"artist_name", artist.Name,
		)

		// Check if it's a rate limiting error and provide appropriate message
		errorMessage := "Failed to add tracks to playlist: " + err.Error()
		if strings.Contains(strings.ToLower(err.Error()), "rate limit") ||
			strings.Contains(strings.ToLower(err.Error()), "429") {
			errorMessage = "Rate limited by Spotify API. Please try again later."
			p.logger.Warn("Spotify API rate limit encountered",
				"component", "playlist_service",
				"operation", "add_tracks",
				"event", "rate_limit_hit",
			)
		}

		return &types.AddResult{
			Success:      false,
			Artist:       *artist,
			TracksAdded:  tracks,
			WasDuplicate: wasDuplicate,
			Message:      errorMessage,
		}, err
	}

	p.logger.Info("Successfully added artist tracks to playlist",
		"component", "playlist_service",
		"operation", "add_tracks",
		"artist_name", artist.Name,
		"playlist_id", playlistID,
		"track_count", len(tracks),
		"track_names", trackNames,
	)

	return &types.AddResult{
		Success:      true,
		Artist:       *artist,
		TracksAdded:  tracks,
		WasDuplicate: wasDuplicate,
		Message:      "Successfully added " + artist.Name + "'s top tracks to playlist",
	}, nil
}

// GetIncomingPlaylists gets playlists from the "Incoming" folder
func (p *PlaylistService) GetIncomingPlaylists() ([]types.Playlist, error) {
	p.logger.Debug("Fetching playlists from 'Incoming' folder",
		"component", "playlist_service",
		"operation", "get_incoming_playlists",
		"folder_name", "Incoming",
	)

	playlists, err := p.spotify.GetUserPlaylists("Incoming")
	if err != nil {
		p.logger.Error("Failed to fetch playlists from Incoming folder",
			"error", err,
			"component", "playlist_service",
			"operation", "get_incoming_playlists",
			"folder_name", "Incoming",
		)
		return nil, err
	}

	playlistNames := make([]string, len(playlists))
	for i, playlist := range playlists {
		playlistNames[i] = playlist.Name
	}

	p.logger.Info("Successfully fetched Incoming playlists",
		"component", "playlist_service",
		"operation", "get_incoming_playlists",
		"folder_name", "Incoming",
		"playlist_count", len(playlists),
		"playlist_names", playlistNames,
	)
	return playlists, nil
}

// GetTop5Tracks gets the top 5 tracks for an artist
func (p *PlaylistService) GetTop5Tracks(artistID string) ([]types.Track, error) {
	p.logger.Debug("Getting top 5 tracks for artist",
		"component", "playlist_service",
		"operation", "get_top5_tracks",
		"artist_id", artistID,
	)

	tracks, err := p.spotify.GetArtistTopTracks(artistID)
	if err != nil {
		p.logger.Error("Failed to get artist top tracks",
			"error", err,
			"component", "playlist_service",
			"operation", "get_top5_tracks",
			"artist_id", artistID,
		)
		return nil, err
	}

	p.logger.Info("Successfully retrieved artist top tracks",
		"component", "playlist_service",
		"operation", "get_top5_tracks",
		"artist_id", artistID,
		"track_count", len(tracks),
	)

	return tracks, nil
}

// AddTracksToPlaylist adds tracks to a playlist
func (p *PlaylistService) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	p.logger.Debug("Adding tracks to playlist",
		"component", "playlist_service",
		"operation", "add_tracks_to_playlist",
		"playlist_id", playlistID,
		"track_count", len(trackIDs),
	)

	err := p.spotify.AddTracksToPlaylist(playlistID, trackIDs)
	if err != nil {
		p.logger.Error("Failed to add tracks to playlist",
			"error", err,
			"component", "playlist_service",
			"operation", "add_tracks_to_playlist",
			"playlist_id", playlistID,
			"track_count", len(trackIDs),
		)
		return err
	}

	p.logger.Info("Successfully added tracks to playlist",
		"component", "playlist_service",
		"operation", "add_tracks_to_playlist",
		"playlist_id", playlistID,
		"track_count", len(trackIDs),
	)

	return nil
}

// CheckForDuplicates checks if tracks already exist in a playlist
func (p *PlaylistService) CheckForDuplicates(playlistID string, trackIDs []string) (*types.DuplicateResult, error) {
	p.logger.Debug("Checking for duplicate tracks in playlist",
		"component", "playlist_service",
		"operation", "check_duplicates",
		"playlist_id", playlistID,
		"track_count", len(trackIDs),
	)

	duplicateFlags, err := p.spotify.CheckTracksInPlaylist(playlistID, trackIDs)
	if err != nil {
		p.logger.Error("Failed to check for duplicate tracks",
			"error", err,
			"component", "playlist_service",
			"operation", "check_duplicates",
			"playlist_id", playlistID,
			"track_count", len(trackIDs),
		)
		return nil, err
	}

	// Count duplicates
	duplicateCount := 0
	for _, isDuplicate := range duplicateFlags {
		if isDuplicate {
			duplicateCount++
		}
	}

	hasDuplicates := duplicateCount > 0
	message := ""
	if hasDuplicates {
		message = "Some tracks already exist in the playlist"
	}

	result := &types.DuplicateResult{
		HasDuplicates: hasDuplicates,
		Message:       message,
	}

	p.logger.Info("Completed duplicate check",
		"component", "playlist_service",
		"operation", "check_duplicates",
		"playlist_id", playlistID,
		"track_count", len(trackIDs),
		"duplicate_count", duplicateCount,
		"has_duplicates", hasDuplicates,
	)

	return result, nil
}

// FilterPlaylistsBySearch filters playlists by search term
func (p *PlaylistService) FilterPlaylistsBySearch(playlists []types.Playlist, searchTerm string) []types.Playlist {
	if searchTerm == "" {
		return playlists
	}

	p.logger.Debug("Filtering playlists by search term",
		"component", "playlist_service",
		"operation", "filter_playlists",
		"search_term", searchTerm,
	)

	filtered := make([]types.Playlist, 0)
	searchLower := strings.ToLower(searchTerm)

	for _, playlist := range playlists {
		if strings.Contains(strings.ToLower(playlist.Name), searchLower) {
			filtered = append(filtered, playlist)
		}
	}

	p.logger.Debug("Playlist filtering completed",
		"component", "playlist_service",
		"operation", "filter_playlists",
		"search_term", searchTerm,
		"original_count", len(playlists),
		"filtered_count", len(filtered),
	)

	return filtered
}
