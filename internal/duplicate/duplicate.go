package duplicate

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/kmhd2spotify/internal/types"
)

// DuplicateService implements the DuplicateDetector interface
type DuplicateService struct {
	spotify types.SpotifyService
	logger  *log.Logger
}

// NewDuplicateService creates a new duplicate detection service
func NewDuplicateService(spotify types.SpotifyService, logger *log.Logger) *DuplicateService {
	return &DuplicateService{
		spotify: spotify,
		logger:  logger,
	}
}

// CheckDuplicates checks if any of the provided tracks already exist in the playlist
func (d *DuplicateService) CheckDuplicates(playlistID string, tracks []types.Track) (*types.DuplicateResult, error) {
	if len(tracks) == 0 {
		d.logger.WithFields(log.Fields{
			"component":   "duplicate_service",
			"operation":   "check_duplicates",
			"playlist_id": playlistID,
		}).Debug("No tracks provided for duplicate check")
		return &types.DuplicateResult{
			HasDuplicates: false,
			Message:       "No tracks to check",
		}, nil
	}

	d.logger.WithFields(log.Fields{
		"component":   "duplicate_service",
		"operation":   "check_duplicates",
		"playlist_id": playlistID,
		"track_count": len(tracks),
	}).Debug("Checking for duplicate tracks in playlist")

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
		d.logger.WithError(err).WithFields(log.Fields{
			"component":   "duplicate_service",
			"operation":   "check_duplicates",
			"playlist_id": playlistID,
			"track_count": len(tracks),
		}).Error("Failed to check tracks in playlist")
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

		d.logger.WithFields(log.Fields{
			"component":            "duplicate_service",
			"operation":            "check_duplicates",
			"playlist_id":          playlistID,
			"duplicate_count":      len(duplicateTracks),
			"duplicate_tracks":     duplicateTrackNames,
			"total_tracks_checked": len(tracks),
		}).Info("Duplicate tracks detected")
	} else {
		result.Message = "No duplicate tracks found"
		d.logger.WithFields(log.Fields{
			"component":            "duplicate_service",
			"operation":            "check_duplicates",
			"playlist_id":          playlistID,
			"total_tracks_checked": len(tracks),
		}).Debug("No duplicate tracks found")
	}

	return result, nil
}

// CheckArtistInPlaylist checks if an artist's tracks already exist in the playlist
func (d *DuplicateService) CheckArtistInPlaylist(playlistID, artistID string) (*types.DuplicateResult, error) {
	d.logger.WithFields(log.Fields{
		"component":   "duplicate_service",
		"operation":   "check_artist_duplicates",
		"playlist_id": playlistID,
		"artist_id":   artistID,
	}).Debug("Checking if artist tracks exist in playlist")

	// Get the artist's top tracks
	tracks, err := d.spotify.GetArtistTopTracks(artistID)
	if err != nil {
		d.logger.WithError(err).WithFields(log.Fields{
			"component":   "duplicate_service",
			"operation":   "check_artist_duplicates",
			"artist_id":   artistID,
			"playlist_id": playlistID,
		}).Error("Failed to get artist top tracks for duplicate check")
		return nil, fmt.Errorf("failed to get artist top tracks: %w", err)
	}

	if len(tracks) == 0 {
		d.logger.WithFields(log.Fields{
			"component":   "duplicate_service",
			"operation":   "check_artist_duplicates",
			"artist_id":   artistID,
			"playlist_id": playlistID,
		}).Debug("Artist has no tracks to check for duplicates")
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

			d.logger.WithFields(log.Fields{
				"component":       "duplicate_service",
				"operation":       "check_artist_duplicates",
				"artist_name":     artistName,
				"artist_id":       artistID,
				"playlist_id":     playlistID,
				"duplicate_count": len(result.DuplicateTracks),
				"last_added":      result.LastAdded,
				"has_duplicates":  true,
			}).Info("Artist tracks already exist in playlist")
		} else {
			result.Message = fmt.Sprintf("Artist '%s' tracks not found in playlist, safe to add", artistName)
			d.logger.WithFields(log.Fields{
				"component":      "duplicate_service",
				"operation":      "check_artist_duplicates",
				"artist_name":    artistName,
				"artist_id":      artistID,
				"playlist_id":    playlistID,
				"has_duplicates": false,
			}).Debug("Artist tracks not found in playlist")
		}
	}

	return result, nil
}
