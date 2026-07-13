package duplicate

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/go-listen/internal/types"
)

// DuplicateService implements the DuplicateDetector interface
type DuplicateService struct {
	spotify types.SpotifyService
	logger  *slog.Logger
}

// NewDuplicateService creates a new duplicate detection service
func NewDuplicateService(spotify types.SpotifyService, logger *slog.Logger) *DuplicateService {
	return &DuplicateService{
		spotify: spotify,
		logger:  logger,
	}
}

// CheckDuplicates checks if any of the provided tracks already exist in the playlist
func (d *DuplicateService) CheckDuplicates(playlistID string, tracks []types.Track) (*types.DuplicateResult, error) {
	if len(tracks) == 0 {
		d.logger.Debug("No tracks provided for duplicate check",
			"component", "duplicate_service",
			"operation", "check_duplicates",
			"playlist_id", playlistID,
		)
		return &types.DuplicateResult{
			HasDuplicates: false,
			Message:       "No tracks to check",
		}, nil
	}

	d.logger.Debug("Checking for duplicate tracks in playlist",
		"component", "duplicate_service",
		"operation", "check_duplicates",
		"playlist_id", playlistID,
		"track_count", len(tracks),
	)

	// Extract track IDs for batch checking
	trackIDs := make([]string, len(tracks))
	trackNames := make([]string, len(tracks))
	for i, track := range tracks {
		trackIDs[i] = track.ID
		trackNames[i] = track.Name
	}

	// Check which tracks exist in the playlist
	existsResults, err := d.spotify.CheckTracksInPlaylist(playlistID, trackIDs)
	if err != nil {
		d.logger.Error("Failed to check tracks in playlist",
			"error", err,
			"component", "duplicate_service",
			"operation", "check_duplicates",
			"playlist_id", playlistID,
			"track_count", len(tracks),
		)
		return nil, fmt.Errorf("failed to check tracks in playlist: %w", err)
	}

	// Collect duplicate tracks
	var duplicateTracks []types.Track
	var duplicateTrackNames []string
	for i, exists := range existsResults {
		if exists && i < len(tracks) {
			duplicateTracks = append(duplicateTracks, tracks[i])
			duplicateTrackNames = append(duplicateTrackNames, tracks[i].Name)
		}
	}

	hasDuplicates := len(duplicateTracks) > 0

	result := &types.DuplicateResult{
		HasDuplicates:   hasDuplicates,
		DuplicateTracks: duplicateTracks,
		LastAdded:       time.Now(), // This would ideally come from playlist metadata
	}

	if hasDuplicates {
		result.Message = fmt.Sprintf("Found %d duplicate track(s): %s",
			len(duplicateTracks), strings.Join(duplicateTrackNames, ", "))

		d.logger.Info("Duplicate tracks detected",
			"component", "duplicate_service",
			"operation", "check_duplicates",
			"playlist_id", playlistID,
			"duplicate_count", len(duplicateTracks),
			"duplicate_tracks", duplicateTrackNames,
			"total_tracks_checked", len(tracks),
		)
	} else {
		result.Message = "No duplicate tracks found"
		d.logger.Debug("No duplicate tracks found",
			"component", "duplicate_service",
			"operation", "check_duplicates",
			"playlist_id", playlistID,
			"total_tracks_checked", len(tracks),
		)
	}

	return result, nil
}

// CheckArtistInPlaylist checks if an artist's tracks already exist in the playlist
func (d *DuplicateService) CheckArtistInPlaylist(playlistID, artistID string) (*types.DuplicateResult, error) {
	d.logger.Debug("Checking if artist tracks exist in playlist",
		"component", "duplicate_service",
		"operation", "check_artist_duplicates",
		"playlist_id", playlistID,
		"artist_id", artistID,
	)

	// Get the artist's top tracks
	tracks, err := d.spotify.GetArtistTopTracks(artistID)
	if err != nil {
		d.logger.Error("Failed to get artist top tracks for duplicate check",
			"error", err,
			"component", "duplicate_service",
			"operation", "check_artist_duplicates",
			"artist_id", artistID,
			"playlist_id", playlistID,
		)
		return nil, fmt.Errorf("failed to get artist top tracks: %w", err)
	}

	if len(tracks) == 0 {
		d.logger.Debug("Artist has no tracks to check for duplicates",
			"component", "duplicate_service",
			"operation", "check_artist_duplicates",
			"artist_id", artistID,
			"playlist_id", playlistID,
		)
		return &types.DuplicateResult{
			HasDuplicates: false,
			Message:       "Artist has no tracks",
		}, nil
	}

	// Get artist name from the first track for better messaging
	var artistName string
	if len(tracks) > 0 && len(tracks[0].Artists) > 0 {
		artistName = tracks[0].Artists[0].Name
	}

	// Use CheckDuplicates to do the actual checking
	result, err := d.CheckDuplicates(playlistID, tracks)
	if err != nil {
		return nil, err
	}

	// Enhance the result with artist-specific information
	if result != nil {
		result.ArtistName = artistName

		if result.HasDuplicates {
			result.Message = fmt.Sprintf("Artist '%s' already has %d track(s) in this playlist (last added: %s). Use 'Add Anyway' to override.",
				artistName, len(result.DuplicateTracks), result.LastAdded.Format("2006-01-02 15:04:05"))

			d.logger.Info("Artist tracks already exist in playlist",
				"component", "duplicate_service",
				"operation", "check_artist_duplicates",
				"artist_name", artistName,
				"artist_id", artistID,
				"playlist_id", playlistID,
				"duplicate_count", len(result.DuplicateTracks),
				"last_added", result.LastAdded,
				"has_duplicates", true,
			)
		} else {
			result.Message = fmt.Sprintf("Artist '%s' tracks not found in playlist, safe to add", artistName)
			d.logger.Debug("Artist tracks not found in playlist",
				"component", "duplicate_service",
				"operation", "check_artist_duplicates",
				"artist_name", artistName,
				"artist_id", artistID,
				"playlist_id", playlistID,
				"has_duplicates", false,
			)
		}
	}

	return result, nil
}
