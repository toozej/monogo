package search

import (
	"fmt"
	"strings"

	"github.com/sahilm/fuzzy"
	"github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/internal/types"
)

// FuzzyArtistSearcher implements fuzzy matching for artist search
type FuzzyArtistSearcher struct {
	spotify types.SpotifyService
	logger  *logrus.Logger
}

// NewFuzzyArtistSearcher creates a new fuzzy artist searcher
func NewFuzzyArtistSearcher(spotifyService types.SpotifyService, logger *logrus.Logger) *FuzzyArtistSearcher {
	return &FuzzyArtistSearcher{
		spotify: spotifyService,
		logger:  logger,
	}
}

// FindBestMatch searches for an artist and returns the best fuzzy match with confidence score
func (f *FuzzyArtistSearcher) FindBestMatch(query string) (*types.Artist, float64, error) {
	if strings.TrimSpace(query) == "" {
		return nil, 0.0, fmt.Errorf("search query cannot be empty")
	}

	f.logger.WithField("query", query).Debug("Starting fuzzy artist search")

	// First, try direct search through Spotify
	artist, err := f.spotify.SearchArtist(query)
	if err != nil {
		f.logger.WithError(err).WithField("query", query).Error("Failed to search for artist")
		return nil, 0.0, fmt.Errorf("failed to search for artist: %w", err)
	}

	// Calculate fuzzy match confidence score
	confidence := f.calculateMatchConfidence(query, artist.Name)

	f.logger.WithFields(logrus.Fields{
		"query":       query,
		"artist_name": artist.Name,
		"confidence":  confidence,
	}).Info("Found artist match")

	return artist, confidence, nil
}

// calculateMatchConfidence calculates a confidence score between 0.0 and 1.0
// for how well the found artist matches the search query
func (f *FuzzyArtistSearcher) calculateMatchConfidence(query, artistName string) float64 {
	// Normalize strings for comparison
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	normalizedArtist := strings.ToLower(strings.TrimSpace(artistName))

	// Exact match gets perfect score
	if normalizedQuery == normalizedArtist {
		return 1.0
	}

	// Check if query is contained in artist name
	if strings.Contains(normalizedArtist, normalizedQuery) {
		// Calculate ratio based on length difference
		ratio := float64(len(normalizedQuery)) / float64(len(normalizedArtist))
		return 0.8 + (ratio * 0.2) // Score between 0.8 and 1.0
	}

	// Check if artist name is contained in query
	if strings.Contains(normalizedQuery, normalizedArtist) {
		ratio := float64(len(normalizedArtist)) / float64(len(normalizedQuery))
		return 0.7 + (ratio * 0.2) // Score between 0.7 and 0.9
	}

	// Use fuzzy matching for more complex cases
	matches := fuzzy.Find(normalizedQuery, []string{normalizedArtist})
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

// SearchMultipleArtists searches for multiple artists and returns them with confidence scores
func (f *FuzzyArtistSearcher) SearchMultipleArtists(queries []string) ([]ArtistMatch, error) {
	if len(queries) == 0 {
		return []ArtistMatch{}, nil
	}

	f.logger.WithField("query_count", len(queries)).Debug("Searching for multiple artists")

	results := make([]ArtistMatch, 0, len(queries))

	for _, query := range queries {
		artist, confidence, err := f.FindBestMatch(query)
		if err != nil {
			f.logger.WithError(err).WithField("query", query).Warn("Failed to find artist match")
			continue
		}

		results = append(results, ArtistMatch{
			Artist:     artist,
			Query:      query,
			Confidence: confidence,
		})
	}

	f.logger.WithFields(logrus.Fields{
		"queries_processed": len(queries),
		"matches_found":     len(results),
	}).Info("Completed multiple artist search")

	return results, nil
}

// ArtistMatch represents a search result with confidence score
type ArtistMatch struct {
	Artist     *types.Artist `json:"artist"`
	Query      string        `json:"query"`
	Confidence float64       `json:"confidence"`
}

// IsHighConfidence returns true if the match confidence is above 0.8
func (am ArtistMatch) IsHighConfidence() bool {
	return am.Confidence >= 0.8
}

// IsLowConfidence returns true if the match confidence is below 0.5
func (am ArtistMatch) IsLowConfidence() bool {
	return am.Confidence < 0.5
}
