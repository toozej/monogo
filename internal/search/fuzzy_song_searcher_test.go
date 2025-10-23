package search

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/toozej/kmhd2spotify/internal/types"
)

// MockSongSpotifyService for testing song search functionality
type MockSongSpotifyService struct{}

func (m *MockSongSpotifyService) SearchArtist(query string) (*types.Artist, error) {
	return &types.Artist{
		ID:     "test-artist-id",
		Name:   "Test Artist",
		URI:    "spotify:artist:test-artist-id",
		Genres: []string{"jazz"},
	}, nil
}

func (m *MockSongSpotifyService) GetArtistTopTracks(artistID string) ([]types.Track, error) {
	return []types.Track{
		{
			ID:   "test-track-1",
			Name: "Test Song",
			URI:  "spotify:track:test-track-1",
			Artists: []types.Artist{
				{
					ID:   "test-artist-id",
					Name: "Test Artist",
					URI:  "spotify:artist:test-artist-id",
				},
			},
			Duration: 180000,
			Album: types.Album{
				ID:   "test-album-1",
				Name: "Test Album",
				Type: "album",
			},
		},
		{
			ID:   "test-track-2",
			Name: "Another Song",
			URI:  "spotify:track:test-track-2",
			Artists: []types.Artist{
				{
					ID:   "test-artist-id",
					Name: "Test Artist",
					URI:  "spotify:artist:test-artist-id",
				},
			},
			Duration: 200000,
			Album: types.Album{
				ID:   "test-album-2",
				Name: "Another Album",
				Type: "album",
			},
		},
		{
			ID:   "test-track-3",
			Name: "Similar Song",
			URI:  "spotify:track:test-track-3",
			Artists: []types.Artist{
				{
					ID:   "test-artist-id",
					Name: "Test Artist",
					URI:  "spotify:artist:test-artist-id",
				},
			},
			Duration: 190000,
			Album: types.Album{
				ID:   "test-album-3",
				Name: "Test Album Deluxe Edition",
				Type: "album",
			},
		},
		{
			ID:   "test-track-4",
			Name: "Track Without Album",
			URI:  "spotify:track:test-track-4",
			Artists: []types.Artist{
				{
					ID:   "test-artist-id",
					Name: "Test Artist",
					URI:  "spotify:artist:test-artist-id",
				},
			},
			Duration: 170000,
			Album: types.Album{
				ID:   "",
				Name: "",
				Type: "",
			},
		},
	}, nil
}

func (m *MockSongSpotifyService) GetUserPlaylists(folderName string) ([]types.Playlist, error) {
	return []types.Playlist{}, nil
}

func (m *MockSongSpotifyService) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	return nil
}

func (m *MockSongSpotifyService) CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error) {
	return []bool{false}, nil
}

func (m *MockSongSpotifyService) GetAuthURL() string {
	return "http://test.com/auth"
}

func (m *MockSongSpotifyService) IsAuthenticated() bool {
	return true
}

func (m *MockSongSpotifyService) CompleteAuth(code, state string) error {
	return nil
}

func (m *MockSongSpotifyService) CreatePlaylist(name, description string, public bool) (*types.Playlist, error) {
	return &types.Playlist{
		ID:         "test-playlist-id",
		Name:       name,
		URI:        "spotify:playlist:test-playlist-id",
		TrackCount: 0,
		EmbedURL:   "https://open.spotify.com/embed/playlist/test-playlist-id",
		IsIncoming: false,
	}, nil
}

func TestFuzzySongSearcher_FindBestSongMatch(t *testing.T) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	// Test with both artist and song query
	match, err := searcher.FindBestSongMatch("Test Artist", "Test Song")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if match == nil {
		t.Fatal("Expected match, got nil")
	}

	if match.Artist.Name != "Test Artist" {
		t.Errorf("Expected artist name 'Test Artist', got '%s'", match.Artist.Name)
	}

	if match.Track.Name != "Test Song" {
		t.Errorf("Expected track name 'Test Song', got '%s'", match.Track.Name)
	}

	if match.OverallConfidence <= 0 {
		t.Errorf("Expected positive confidence, got %f", match.OverallConfidence)
	}

	// Test with only artist query
	match2, err := searcher.FindBestSongMatch("Test Artist", "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if match2.SongConfidence != 1.0 {
		t.Errorf("Expected song confidence 1.0 when no song query, got %f", match2.SongConfidence)
	}
}

func TestFuzzySongSearcher_FindBestSongMatchWithAlbum(t *testing.T) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	// Test with album query that matches first track
	match, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Test Song", "Test Album")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if match == nil {
		t.Fatal("Expected match, got nil")
	}

	if match.AlbumQuery != "Test Album" {
		t.Errorf("Expected album query 'Test Album', got '%s'", match.AlbumQuery)
	}

	if match.AlbumConfidence <= 0.5 {
		t.Errorf("Expected album confidence > 0.5 for matching album, got %f", match.AlbumConfidence)
	}

	if match.Track.Name != "Test Song" {
		t.Errorf("Expected track name 'Test Song', got '%s'", match.Track.Name)
	}

	// Test with album query that matches second track better
	match2, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Another Song", "Another Album")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if match2.Track.Name != "Another Song" {
		t.Errorf("Expected track name 'Another Song', got '%s'", match2.Track.Name)
	}

	if match2.AlbumConfidence <= 0.5 {
		t.Errorf("Expected album confidence > 0.5 for matching album, got %f", match2.AlbumConfidence)
	}

	// Test with empty album query - should return neutral album confidence
	match3, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Test Song", "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if match3.AlbumConfidence != 0.5 {
		t.Errorf("Expected album confidence 0.5 for empty album query, got %f", match3.AlbumConfidence)
	}

	// Test backward compatibility - FindBestSongMatch should work the same as FindBestSongMatchWithAlbum with empty album
	match4, err := searcher.FindBestSongMatch("Test Artist", "Test Song")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if match4.AlbumQuery != "" {
		t.Errorf("Expected empty album query for backward compatibility, got '%s'", match4.AlbumQuery)
	}

	if match4.AlbumConfidence != 0.5 {
		t.Errorf("Expected album confidence 0.5 for backward compatibility, got %f", match4.AlbumConfidence)
	}
}

func TestFuzzySongSearcher_calculateAlbumConfidence(t *testing.T) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	// Test exact album match
	track := &types.Track{
		Album: types.Album{Name: "Test Album"},
	}
	confidence := searcher.calculateAlbumConfidence("Test Album", track)
	if confidence != 1.0 {
		t.Errorf("Expected confidence 1.0 for exact match, got %f", confidence)
	}

	// Test partial album match
	confidence = searcher.calculateAlbumConfidence("Test", track)
	if confidence <= 0.5 {
		t.Errorf("Expected confidence > 0.5 for partial match, got %f", confidence)
	}

	// Test case insensitive match
	confidence = searcher.calculateAlbumConfidence("test album", track)
	if confidence != 1.0 {
		t.Errorf("Expected confidence 1.0 for case insensitive match, got %f", confidence)
	}

	// Test empty album query - should return neutral score
	confidence = searcher.calculateAlbumConfidence("", track)
	if confidence != 0.5 {
		t.Errorf("Expected confidence 0.5 for empty query, got %f", confidence)
	}

	// Test whitespace-only album query - should return neutral score
	confidence = searcher.calculateAlbumConfidence("   ", track)
	if confidence != 0.5 {
		t.Errorf("Expected confidence 0.5 for whitespace query, got %f", confidence)
	}

	// Test track with empty album name - should return neutral score
	emptyAlbumTrack := &types.Track{
		Album: types.Album{Name: ""},
	}
	confidence = searcher.calculateAlbumConfidence("Test Album", emptyAlbumTrack)
	if confidence != 0.5 {
		t.Errorf("Expected confidence 0.5 for empty album name, got %f", confidence)
	}

	// Test nil track - should return neutral score
	confidence = searcher.calculateAlbumConfidence("Test Album", nil)
	if confidence != 0.5 {
		t.Errorf("Expected confidence 0.5 for nil track, got %f", confidence)
	}

	// Test fuzzy album match
	confidence = searcher.calculateAlbumConfidence("Test Albm", track) // Missing 'u'
	if confidence <= 0.1 || confidence >= 1.0 {
		t.Errorf("Expected fuzzy match confidence between 0.1 and 1.0, got %f", confidence)
	}

	// Test no match
	confidence = searcher.calculateAlbumConfidence("Completely Different Album", track)
	if confidence <= 0.0 || confidence > 0.7 {
		t.Errorf("Expected low confidence for no match, got %f", confidence)
	}
}

func TestFuzzySongSearcher_EnhancedConfidenceCalculation(t *testing.T) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	// Test confidence calculation with perfect matches
	match, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Test Song", "Test Album")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify individual confidence scores
	if match.ArtistConfidence != 1.0 {
		t.Errorf("Expected artist confidence 1.0 for exact match, got %f", match.ArtistConfidence)
	}
	if match.SongConfidence != 1.0 {
		t.Errorf("Expected song confidence 1.0 for exact match, got %f", match.SongConfidence)
	}
	if match.AlbumConfidence != 1.0 {
		t.Errorf("Expected album confidence 1.0 for exact match, got %f", match.AlbumConfidence)
	}

	// Verify weighted overall confidence calculation (50% artist + 35% song + 15% album)
	expectedOverall := (1.0 * 0.5) + (1.0 * 0.35) + (1.0 * 0.15)
	if match.OverallConfidence != expectedOverall {
		t.Errorf("Expected overall confidence %f, got %f", expectedOverall, match.OverallConfidence)
	}

	// Test with mixed confidence scores
	match2, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Different Song", "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Album confidence should be neutral (0.5) for empty query
	if match2.AlbumConfidence != 0.5 {
		t.Errorf("Expected album confidence 0.5 for empty query, got %f", match2.AlbumConfidence)
	}

	// Overall confidence should reflect the weighting
	expectedOverall2 := (match2.ArtistConfidence * 0.5) + (match2.SongConfidence * 0.35) + (0.5 * 0.15)
	if match2.OverallConfidence != expectedOverall2 {
		t.Errorf("Expected overall confidence %f, got %f", expectedOverall2, match2.OverallConfidence)
	}
}

func TestFuzzySongSearcher_AlbumMatchingScenarios(t *testing.T) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	// Test album matching influences track selection
	// When searching for "Another Song" with "Another Album", should select second track
	match, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Song", "Another Album")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should select "Another Song" because it has better album match
	if match.Track.Name != "Another Song" {
		t.Errorf("Expected track 'Another Song' due to album match, got '%s'", match.Track.Name)
	}

	// Test that album matching doesn't override strong song matches
	match2, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Test Song", "Another Album")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should still select "Test Song" because song match is weighted higher than album
	if match2.Track.Name != "Test Song" {
		t.Errorf("Expected track 'Test Song' due to strong song match, got '%s'", match2.Track.Name)
	}
}

func TestFuzzySongSearcher_BackwardCompatibility(t *testing.T) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	// Test that FindBestSongMatch still works as before
	match1, err := searcher.FindBestSongMatch("Test Artist", "Test Song")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test that FindBestSongMatchWithAlbum with empty album gives same result
	match2, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Test Song", "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Results should be equivalent
	if match1.Artist.Name != match2.Artist.Name {
		t.Errorf("Backward compatibility failed: artist names differ")
	}
	if match1.Track.Name != match2.Track.Name {
		t.Errorf("Backward compatibility failed: track names differ")
	}
	if match1.ArtistConfidence != match2.ArtistConfidence {
		t.Errorf("Backward compatibility failed: artist confidence differs")
	}
	if match1.SongConfidence != match2.SongConfidence {
		t.Errorf("Backward compatibility failed: song confidence differs")
	}
	if match1.AlbumConfidence != match2.AlbumConfidence {
		t.Errorf("Backward compatibility failed: album confidence differs")
	}
	if match1.OverallConfidence != match2.OverallConfidence {
		t.Errorf("Backward compatibility failed: overall confidence differs")
	}

	// Both should have empty album query and neutral album confidence
	if match1.AlbumQuery != "" || match2.AlbumQuery != "" {
		t.Error("Expected empty album query for backward compatibility")
	}
	if match1.AlbumConfidence != 0.5 || match2.AlbumConfidence != 0.5 {
		t.Error("Expected neutral album confidence for backward compatibility")
	}
}

func TestSongMatch_ConfidenceMethods(t *testing.T) {
	highConfidenceMatch := SongMatch{OverallConfidence: 0.9}
	lowConfidenceMatch := SongMatch{OverallConfidence: 0.3}
	mediumConfidenceMatch := SongMatch{OverallConfidence: 0.6}

	if !highConfidenceMatch.IsHighConfidence() {
		t.Error("Expected high confidence match to return true for IsHighConfidence")
	}

	if highConfidenceMatch.IsLowConfidence() {
		t.Error("Expected high confidence match to return false for IsLowConfidence")
	}

	if !lowConfidenceMatch.IsLowConfidence() {
		t.Error("Expected low confidence match to return true for IsLowConfidence")
	}

	if lowConfidenceMatch.IsHighConfidence() {
		t.Error("Expected low confidence match to return false for IsHighConfidence")
	}

	if mediumConfidenceMatch.IsHighConfidence() {
		t.Error("Expected medium confidence match to return false for IsHighConfidence")
	}

	if mediumConfidenceMatch.IsLowConfidence() {
		t.Error("Expected medium confidence match to return false for IsLowConfidence")
	}
}
func TestFuzzySongSearcher_IntegrationWithAlbumData(t *testing.T) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	// Test end-to-end search with exact album match
	match, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Test Song", "Test Album")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify all fields are populated correctly
	if match.Artist == nil {
		t.Fatal("Expected artist to be populated")
	}
	if match.Track == nil {
		t.Fatal("Expected track to be populated")
	}
	if match.ArtistQuery != "Test Artist" {
		t.Errorf("Expected artist query 'Test Artist', got '%s'", match.ArtistQuery)
	}
	if match.SongQuery != "Test Song" {
		t.Errorf("Expected song query 'Test Song', got '%s'", match.SongQuery)
	}
	if match.AlbumQuery != "Test Album" {
		t.Errorf("Expected album query 'Test Album', got '%s'", match.AlbumQuery)
	}

	// Verify confidence scores are reasonable
	if match.ArtistConfidence <= 0 || match.ArtistConfidence > 1 {
		t.Errorf("Artist confidence out of range: %f", match.ArtistConfidence)
	}
	if match.SongConfidence <= 0 || match.SongConfidence > 1 {
		t.Errorf("Song confidence out of range: %f", match.SongConfidence)
	}
	if match.AlbumConfidence <= 0 || match.AlbumConfidence > 1 {
		t.Errorf("Album confidence out of range: %f", match.AlbumConfidence)
	}
	if match.OverallConfidence <= 0 || match.OverallConfidence > 1 {
		t.Errorf("Overall confidence out of range: %f", match.OverallConfidence)
	}

	// Test that album matching improves accuracy
	matchWithAlbum, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Similar Song", "Test Album Deluxe Edition")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	matchWithoutAlbum, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Similar Song", "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Album matching should improve overall confidence when there's a good album match
	if matchWithAlbum.OverallConfidence <= matchWithoutAlbum.OverallConfidence {
		t.Errorf("Expected album matching to improve confidence: with album %f, without album %f",
			matchWithAlbum.OverallConfidence, matchWithoutAlbum.OverallConfidence)
	}
}

func TestFuzzySongSearcher_EdgeCasesWithAlbumData(t *testing.T) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	// Test with malformed album names (extra whitespace, special characters)
	match, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Test Song", "  Test Album  ")
	if err != nil {
		t.Fatalf("Expected no error with whitespace in album query, got %v", err)
	}
	if match.AlbumConfidence != 1.0 {
		t.Errorf("Expected perfect album match despite whitespace, got confidence %f", match.AlbumConfidence)
	}

	// Test with track that has no album information
	match2, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Track Without Album", "Some Album")
	if err != nil {
		t.Fatalf("Expected no error with missing album data, got %v", err)
	}
	if match2.AlbumConfidence != 0.5 {
		t.Errorf("Expected neutral album confidence for track without album, got %f", match2.AlbumConfidence)
	}

	// Test with very long album query
	longAlbumQuery := "This Is A Very Long Album Name That Might Cause Issues With Fuzzy Matching Algorithms"
	match3, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Test Song", longAlbumQuery)
	if err != nil {
		t.Fatalf("Expected no error with long album query, got %v", err)
	}
	// Should handle gracefully without crashing
	if match3.AlbumConfidence < 0 || match3.AlbumConfidence > 1 {
		t.Errorf("Album confidence out of range with long query: %f", match3.AlbumConfidence)
	}

	// Test with special characters in album name
	match4, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Test Song", "Test Album (Deluxe)")
	if err != nil {
		t.Fatalf("Expected no error with special characters in album query, got %v", err)
	}
	// Should handle special characters gracefully
	if match4.AlbumConfidence < 0 || match4.AlbumConfidence > 1 {
		t.Errorf("Album confidence out of range with special characters: %f", match4.AlbumConfidence)
	}
}

func TestFuzzySongSearcher_AlbumMatchingAccuracy(t *testing.T) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	// Test that album matching helps select the correct track when song names are similar
	// "Similar Song" exists, but we want to match it with "Test Album Deluxe Edition"
	match, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Similar", "Test Album Deluxe Edition")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should select "Similar Song" because it has the best album match
	if match.Track.Name != "Similar Song" {
		t.Errorf("Expected 'Similar Song' due to album match, got '%s'", match.Track.Name)
	}

	// Test that strong song matches still win over weak album matches
	match2, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Test Song", "Wrong Album Name")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should still select "Test Song" because song match is weighted higher
	if match2.Track.Name != "Test Song" {
		t.Errorf("Expected 'Test Song' due to strong song match, got '%s'", match2.Track.Name)
	}

	// Verify that the album confidence is low for the wrong album
	if match2.AlbumConfidence > 0.5 {
		t.Errorf("Expected low album confidence for wrong album, got %f", match2.AlbumConfidence)
	}
}

func TestFuzzySongSearcher_ConfidenceWeightingValidation(t *testing.T) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	// Test that confidence weighting follows the expected formula: 50% artist + 35% song + 15% album
	match, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Test Song", "Test Album")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Calculate expected overall confidence manually
	expectedOverall := (match.ArtistConfidence * 0.5) + (match.SongConfidence * 0.35) + (match.AlbumConfidence * 0.15)

	// Allow for small floating point differences
	tolerance := 0.0001
	if abs(match.OverallConfidence-expectedOverall) > tolerance {
		t.Errorf("Confidence weighting incorrect: expected %f, got %f", expectedOverall, match.OverallConfidence)
	}

	// Test with different confidence combinations
	match2, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Different Song", "Different Album")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedOverall2 := (match2.ArtistConfidence * 0.5) + (match2.SongConfidence * 0.35) + (match2.AlbumConfidence * 0.15)
	if abs(match2.OverallConfidence-expectedOverall2) > tolerance {
		t.Errorf("Confidence weighting incorrect for different match: expected %f, got %f", expectedOverall2, match2.OverallConfidence)
	}
}

// Helper function for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// Performance test to measure impact of album matching
func BenchmarkFuzzySongSearcher_WithoutAlbum(b *testing.B) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := searcher.FindBestSongMatch("Test Artist", "Test Song")
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkFuzzySongSearcher_WithAlbum(b *testing.B) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := searcher.FindBestSongMatchWithAlbum("Test Artist", "Test Song", "Test Album")
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkFuzzySongSearcher_AlbumConfidenceCalculation(b *testing.B) {
	mockSpotify := &MockSongSpotifyService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	searcher := NewFuzzySongSearcher(mockSpotify, logger)

	// Get a test track for benchmarking
	tracks, _ := mockSpotify.GetArtistTopTracks("test-artist-id")
	testTrack := tracks[0]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = searcher.calculateAlbumConfidence("Test Album", &testTrack)
	}
}
