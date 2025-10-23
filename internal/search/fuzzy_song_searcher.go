package search

import (
	"fmt"
	"strings"

	"github.com/sahilm/fuzzy"
	"github.com/sirupsen/logrus"
	"github.com/toozej/kmhd2spotify/internal/types"
)

// FuzzySongSearcher implements fuzzy matching for both artist and song search
type FuzzySongSearcher struct {
	spotify types.SpotifyService
	logger  *logrus.Logger
}

// NewFuzzySongSearcher creates a new fuzzy song searcher
func NewFuzzySongSearcher(spotifyService types.SpotifyService, logger *logrus.Logger) *FuzzySongSearcher {
	return &FuzzySongSearcher{
		spotify: spotifyService,
		logger:  logger,
	}
}

// SongMatch represents a search result with confidence score for artist, song, and album
type SongMatch struct {
	Artist            *types.Artist `json:"artist"`
	Track             *types.Track  `json:"track"`
	ArtistQuery       string        `json:"artist_query"`
	SongQuery         string        `json:"song_query"`
	AlbumQuery        string        `json:"album_query"`
	ArtistConfidence  float64       `json:"artist_confidence"`
	SongConfidence    float64       `json:"song_confidence"`
	AlbumConfidence   float64       `json:"album_confidence"`
	OverallConfidence float64       `json:"overall_confidence"`
}

// FindBestSongMatch searches for both artist and song, returning the best match with confidence scores
// This method maintains backward compatibility by calling FindBestSongMatchWithAlbum with empty album query
func (f *FuzzySongSearcher) FindBestSongMatch(artistQuery, songQuery string) (*SongMatch, error) {
	return f.FindBestSongMatchWithAlbum(artistQuery, songQuery, "")
}

// FindBestSongMatchWithAlbum searches for artist, song, and album, returning the best match with confidence scores
func (f *FuzzySongSearcher) FindBestSongMatchWithAlbum(artistQuery, songQuery, albumQuery string) (*SongMatch, error) {
	if strings.TrimSpace(artistQuery) == "" {
		return nil, fmt.Errorf("artist query cannot be empty")
	}

	f.logger.WithFields(logrus.Fields{
		"artist_query": artistQuery,
		"song_query":   songQuery,
		"album_query":  albumQuery,
	}).Debug("Starting fuzzy song search")

	// First, search for the artist
	artist, err := f.spotify.SearchArtist(artistQuery)
	if err != nil {
		f.logger.WithError(err).WithFields(logrus.Fields{
			"artist_query": artistQuery,
		}).Error("Failed to search for artist")
		return nil, fmt.Errorf("failed to search for artist: %w", err)
	}

	// Calculate artist match confidence
	artistConfidence := f.calculateMatchConfidence(artistQuery, artist.Name)

	// Get top tracks for the artist
	tracks, err := f.spotify.GetArtistTopTracks(artist.ID)
	if err != nil {
		f.logger.WithError(err).WithFields(logrus.Fields{
			"artist_id":   artist.ID,
			"artist_name": artist.Name,
		}).Error("Failed to get artist tracks")
		return nil, fmt.Errorf("failed to get artist tracks: %w", err)
	}

	if len(tracks) == 0 {
		return nil, fmt.Errorf("no tracks found for artist %s", artist.Name)
	}

	// If no song query provided, return the first track
	if strings.TrimSpace(songQuery) == "" {
		albumConfidence := f.calculateAlbumConfidence(albumQuery, &tracks[0])
		overallConfidence := (artistConfidence * 0.5) + (1.0 * 0.35) + (albumConfidence * 0.15)

		return &SongMatch{
			Artist:            artist,
			Track:             &tracks[0],
			ArtistQuery:       artistQuery,
			SongQuery:         "",
			AlbumQuery:        albumQuery,
			ArtistConfidence:  artistConfidence,
			SongConfidence:    1.0, // No song to match, so perfect score
			AlbumConfidence:   albumConfidence,
			OverallConfidence: overallConfidence,
		}, nil
	}

	// Find the best matching track considering song and album confidence
	bestTrack := &tracks[0]
	bestSongConfidence := f.calculateMatchConfidence(songQuery, tracks[0].Name)
	bestAlbumConfidence := f.calculateAlbumConfidence(albumQuery, &tracks[0])
	bestOverallConfidence := (artistConfidence * 0.5) + (bestSongConfidence * 0.35) + (bestAlbumConfidence * 0.15)

	for i := 1; i < len(tracks); i++ {
		trackSongConfidence := f.calculateMatchConfidence(songQuery, tracks[i].Name)
		trackAlbumConfidence := f.calculateAlbumConfidence(albumQuery, &tracks[i])
		trackOverallConfidence := (artistConfidence * 0.5) + (trackSongConfidence * 0.35) + (trackAlbumConfidence * 0.15)

		if trackOverallConfidence > bestOverallConfidence {
			bestTrack = &tracks[i]
			bestSongConfidence = trackSongConfidence
			bestAlbumConfidence = trackAlbumConfidence
			bestOverallConfidence = trackOverallConfidence
		}
	}

	// Use the calculated best overall confidence
	overallConfidence := bestOverallConfidence

	result := &SongMatch{
		Artist:            artist,
		Track:             bestTrack,
		ArtistQuery:       artistQuery,
		SongQuery:         songQuery,
		AlbumQuery:        albumQuery,
		ArtistConfidence:  artistConfidence,
		SongConfidence:    bestSongConfidence,
		AlbumConfidence:   bestAlbumConfidence,
		OverallConfidence: overallConfidence,
	}

	// Log basic match information at Info level
	f.logger.WithFields(logrus.Fields{
		"artist_query":       artistQuery,
		"song_query":         songQuery,
		"matched_artist":     artist.Name,
		"matched_song":       bestTrack.Name,
		"overall_confidence": overallConfidence,
	}).Info("Found song match")

	// Log detailed album matching information at Debug level to avoid verbosity in production
	if albumQuery != "" {
		f.logger.WithFields(logrus.Fields{
			"album_query":        albumQuery,
			"matched_album":      bestTrack.Album.Name,
			"album_confidence":   bestAlbumConfidence,
			"artist_confidence":  artistConfidence,
			"song_confidence":    bestSongConfidence,
			"confidence_weights": "artist:50%, song:35%, album:15%",
			"confidence_calculation": fmt.Sprintf("(%.3f * 0.5) + (%.3f * 0.35) + (%.3f * 0.15) = %.3f",
				artistConfidence, bestSongConfidence, bestAlbumConfidence, overallConfidence),
		}).Debug("Album matching details")
	}

	return result, nil
}

// calculateMatchConfidence calculates a confidence score between 0.0 and 1.0
// for how well the found item matches the search query
func (f *FuzzySongSearcher) calculateMatchConfidence(query, itemName string) float64 {
	// Normalize strings for comparison
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	normalizedItem := strings.ToLower(strings.TrimSpace(itemName))

	// Exact match gets perfect score
	if normalizedQuery == normalizedItem {
		return 1.0
	}

	// Check if query is contained in item name
	if strings.Contains(normalizedItem, normalizedQuery) {
		// Calculate ratio based on length difference
		ratio := float64(len(normalizedQuery)) / float64(len(normalizedItem))
		return 0.8 + (ratio * 0.2) // Score between 0.8 and 1.0
	}

	// Check if item name is contained in query
	if strings.Contains(normalizedQuery, normalizedItem) {
		ratio := float64(len(normalizedItem)) / float64(len(normalizedQuery))
		return 0.7 + (ratio * 0.2) // Score between 0.7 and 0.9
	}

	// Use fuzzy matching for more complex cases
	matches := fuzzy.Find(normalizedQuery, []string{normalizedItem})
	if len(matches) > 0 {
		// Convert fuzzy score to confidence (fuzzy scores are typically 0-100+)
		// We'll normalize to 0.0-0.7 range for fuzzy matches
		fuzzyScore := float64(matches[0].Score)
		maxExpectedScore := float64(len(normalizedQuery) * 2) // Rough estimate
		confidence := (fuzzyScore / maxExpectedScore) * 0.7

		// Cap at 0.7 and ensure minimum of 0.1 for any match
		if confidence > 0.7 {
			confidence = 0.7
		}
		if confidence < 0.1 {
			confidence = 0.1
		}

		return confidence
	}

	// If no fuzzy match found, return low confidence
	return 0.1
}

// calculateAlbumConfidence calculates a confidence score between 0.0 and 1.0
// for how well the album query matches the track's album name
func (f *FuzzySongSearcher) calculateAlbumConfidence(albumQuery string, track *types.Track) float64 {
	// Handle empty/null album queries - return neutral score
	if strings.TrimSpace(albumQuery) == "" {
		f.logger.WithFields(logrus.Fields{
			"reason": "empty_album_query",
		}).Trace("Using neutral album confidence")
		return 0.5
	}

	// Handle tracks with no album information - return neutral score
	if track == nil || strings.TrimSpace(track.Album.Name) == "" {
		f.logger.WithFields(logrus.Fields{
			"reason":      "missing_track_album_data",
			"album_query": albumQuery,
		}).Trace("Using neutral album confidence")
		return 0.5
	}

	// Use the same fuzzy matching logic as other fields
	confidence := f.calculateMatchConfidence(albumQuery, track.Album.Name)

	f.logger.WithFields(logrus.Fields{
		"album_query": albumQuery,
		"track_album": track.Album.Name,
		"confidence":  confidence,
	}).Trace("Calculated album confidence")

	return confidence
}

// IsHighConfidence returns true if the overall match confidence is above 0.8
func (sm SongMatch) IsHighConfidence() bool {
	return sm.OverallConfidence >= 0.8
}

// IsLowConfidence returns true if the overall match confidence is below 0.5
func (sm SongMatch) IsLowConfidence() bool {
	return sm.OverallConfidence < 0.5
}
