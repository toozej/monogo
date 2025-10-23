package playlist

import (
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/kmhd2spotify/internal/types"
)

// PlaylistService implements the PlaylistManager interface
type PlaylistService struct {
	spotify   types.SpotifyService
	duplicate types.DuplicateDetector
	logger    *log.Logger
}

// NewPlaylistService creates a new playlist service
func NewPlaylistService(spotify types.SpotifyService, duplicate types.DuplicateDetector, logger *log.Logger) *PlaylistService {
	return &PlaylistService{
		spotify:   spotify,
		duplicate: duplicate,
		logger:    logger,
	}
}

// NewService creates a new playlist service (alias for NewPlaylistService)
func NewService(spotify types.SpotifyService, logger *log.Logger) *PlaylistService {
	return &PlaylistService{
		spotify: spotify,
		logger:  logger,
	}
}

// AddArtistToPlaylist adds an artist's top tracks to a playlist
func (p *PlaylistService) AddArtistToPlaylist(artistName, playlistID string, force bool) (*types.AddResult, error) {
	p.logger.WithFields(log.Fields{
		"component":   "playlist_service",
		"operation":   "add_artist",
		"artist_name": artistName,
		"playlist_id": playlistID,
		"force":       force,
	}).Info("Starting to add artist to playlist")

	// Search for the artist
	artist, err := p.spotify.SearchArtist(artistName)
	if err != nil {
		p.logger.WithError(err).WithFields(log.Fields{
			"component":   "playlist_service",
			"operation":   "add_artist",
			"artist_name": artistName,
		}).Error("Failed to search for artist")
		return &types.AddResult{
			Success: false,
			Message: "Failed to find artist: " + err.Error(),
		}, err
	}

	// Get the artist's top 5 tracks
	tracks, err := p.spotify.GetArtistTopTracks(artist.ID)
	if err != nil {
		p.logger.WithError(err).WithFields(log.Fields{
			"component":   "playlist_service",
			"operation":   "add_artist",
			"artist_id":   artist.ID,
			"artist_name": artist.Name,
		}).Error("Failed to get artist top tracks")
		return &types.AddResult{
			Success: false,
			Artist:  *artist,
			Message: "Failed to get artist's top tracks: " + err.Error(),
		}, err
	}

	if len(tracks) == 0 {
		p.logger.WithFields(log.Fields{
			"component":   "playlist_service",
			"operation":   "add_artist",
			"artist_id":   artist.ID,
			"artist_name": artist.Name,
		}).Warn("Artist has no tracks available")
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
			p.logger.WithError(err).WithFields(log.Fields{
				"component":   "playlist_service",
				"operation":   "duplicate_check",
				"artist_id":   artist.ID,
				"playlist_id": playlistID,
			}).Warn("Failed to check for duplicates, proceeding anyway")
		} else if duplicateResult != nil && duplicateResult.HasDuplicates {
			p.logger.WithFields(log.Fields{
				"component":      "playlist_service",
				"operation":      "duplicate_check",
				"artist_id":      artist.ID,
				"artist_name":    artist.Name,
				"playlist_id":    playlistID,
				"last_added":     duplicateResult.LastAdded,
				"has_duplicates": true,
			}).Info("Artist tracks already exist in playlist")

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
		p.logger.WithError(err).WithFields(log.Fields{
			"component":   "playlist_service",
			"operation":   "add_tracks",
			"playlist_id": playlistID,
			"track_count": len(trackIDs),
			"artist_name": artist.Name,
		}).Error("Failed to add tracks to playlist")

		// Check if it's a rate limiting error and provide appropriate message
		errorMessage := "Failed to add tracks to playlist: " + err.Error()
		if strings.Contains(strings.ToLower(err.Error()), "rate limit") ||
			strings.Contains(strings.ToLower(err.Error()), "429") {
			errorMessage = "Rate limited by Spotify API. Please try again later."
			p.logger.WithFields(log.Fields{
				"component": "playlist_service",
				"operation": "add_tracks",
				"event":     "rate_limit_hit",
			}).Warn("Spotify API rate limit encountered")
		}

		return &types.AddResult{
			Success:      false,
			Artist:       *artist,
			TracksAdded:  tracks,
			WasDuplicate: wasDuplicate,
			Message:      errorMessage,
		}, err
	}

	p.logger.WithFields(log.Fields{
		"component":   "playlist_service",
		"operation":   "add_tracks",
		"artist_name": artist.Name,
		"playlist_id": playlistID,
		"track_count": len(tracks),
		"track_names": trackNames,
	}).Info("Successfully added artist tracks to playlist")

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
	p.logger.WithFields(log.Fields{
		"component":   "playlist_service",
		"operation":   "get_incoming_playlists",
		"folder_name": "Incoming",
	}).Debug("Fetching playlists from 'Incoming' folder")

	playlists, err := p.spotify.GetUserPlaylists("Incoming")
	if err != nil {
		p.logger.WithError(err).WithFields(log.Fields{
			"component":   "playlist_service",
			"operation":   "get_incoming_playlists",
			"folder_name": "Incoming",
		}).Error("Failed to fetch playlists from Incoming folder")
		return nil, err
	}

	playlistNames := make([]string, len(playlists))
	for i, playlist := range playlists {
		playlistNames[i] = playlist.Name
	}

	p.logger.WithFields(log.Fields{
		"component":      "playlist_service",
		"operation":      "get_incoming_playlists",
		"folder_name":    "Incoming",
		"playlist_count": len(playlists),
		"playlist_names": playlistNames,
	}).Info("Successfully fetched Incoming playlists")
	return playlists, nil
}

// GetTop5Tracks gets the top 5 tracks for an artist
func (p *PlaylistService) GetTop5Tracks(artistID string) ([]types.Track, error) {
	p.logger.WithFields(log.Fields{
		"component": "playlist_service",
		"operation": "get_top_tracks",
		"artist_id": artistID,
	}).Debug("Fetching top tracks for artist")

	tracks, err := p.spotify.GetArtistTopTracks(artistID)
	if err != nil {
		p.logger.WithError(err).WithFields(log.Fields{
			"component": "playlist_service",
			"operation": "get_top_tracks",
			"artist_id": artistID,
		}).Error("Failed to fetch top tracks for artist")
		return nil, err
	}

	p.logger.WithFields(log.Fields{
		"component":   "playlist_service",
		"operation":   "get_top_tracks",
		"artist_id":   artistID,
		"track_count": len(tracks),
	}).Debug("Successfully fetched top tracks for artist")

	return tracks, nil
}

// FilterPlaylistsBySearch filters playlists by search term
func (p *PlaylistService) FilterPlaylistsBySearch(playlists []types.Playlist, searchTerm string) []types.Playlist {
	if searchTerm == "" {
		return playlists
	}

	p.logger.WithFields(log.Fields{
		"component":   "playlist_service",
		"operation":   "filter_playlists",
		"search_term": searchTerm,
	}).Debug("Filtering playlists by search term")

	filtered := make([]types.Playlist, 0)
	searchLower := strings.ToLower(searchTerm)

	for _, playlist := range playlists {
		if strings.Contains(strings.ToLower(playlist.Name), searchLower) {
			filtered = append(filtered, playlist)
		}
	}

	p.logger.WithFields(log.Fields{
		"component":      "playlist_service",
		"operation":      "filter_playlists",
		"search_term":    searchTerm,
		"original_count": len(playlists),
		"filtered_count": len(filtered),
	}).Debug("Playlist filtering completed")

	return filtered
}
