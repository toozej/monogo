package search

import (
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/internal/server"
)

// MockSpotifyService implements SpotifyService for testing
type MockSpotifyService struct {
	searchArtistFunc func(query string) (*server.Artist, error)
}

func (m *MockSpotifyService) SearchArtist(query string) (*server.Artist, error) {
	if m.searchArtistFunc != nil {
		return m.searchArtistFunc(query)
	}
	return nil, errors.New("not implemented")
}

func (m *MockSpotifyService) GetArtistTopTracks(artistID string) ([]server.Track, error) {
	return nil, errors.New("not implemented")
}

func (m *MockSpotifyService) GetUserPlaylists(folderName string) ([]server.Playlist, error) {
	return nil, errors.New("not implemented")
}

func (m *MockSpotifyService) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	return errors.New("not implemented")
}

func (m *MockSpotifyService) CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error) {
	return nil, errors.New("not implemented")
}

func (m *MockSpotifyService) GetAuthURL() string {
	return "mock-auth-url"
}

func (m *MockSpotifyService) IsAuthenticated() bool {
	return true
}

func (m *MockSpotifyService) CompleteAuth(code, state string) error {
	return nil
}

func TestNewFuzzyArtistSearcher(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockSpotify := &MockSpotifyService{}

	searcher := NewFuzzyArtistSearcher(mockSpotify, logger)

	if searcher == nil {
		t.Error("NewFuzzyArtistSearcher() returned nil")
		return
	}

	if searcher.spotify != mockSpotify {
		t.Error("NewFuzzyArtistSearcher() did not set spotify service correctly")
	}

	if searcher.logger != logger {
		t.Error("NewFuzzyArtistSearcher() did not set logger correctly")
	}
}

func TestFuzzyArtistSearcher_FindBestMatch(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name           string
		query          string
		mockArtist     *server.Artist
		mockError      error
		wantErr        bool
		wantConfidence float64
		minConfidence  float64
		maxConfidence  float64
	}{
		{
			name:      "empty query",
			query:     "",
			wantErr:   true,
			mockError: nil,
		},
		{
			name:      "whitespace only query",
			query:     "   ",
			wantErr:   true,
			mockError: nil,
		},
		{
			name:      "spotify search error",
			query:     "test artist",
			mockError: errors.New("spotify error"),
			wantErr:   true,
		},
		{
			name:  "exact match",
			query: "Taylor Swift",
			mockArtist: &server.Artist{
				ID:   "06HL4z0CvFAxyc27GXpf02",
				Name: "Taylor Swift",
				URI:  "spotify:artist:06HL4z0CvFAxyc27GXpf02",
			},
			wantConfidence: 1.0,
		},
		{
			name:  "case insensitive exact match",
			query: "taylor swift",
			mockArtist: &server.Artist{
				ID:   "06HL4z0CvFAxyc27GXpf02",
				Name: "Taylor Swift",
				URI:  "spotify:artist:06HL4z0CvFAxyc27GXpf02",
			},
			wantConfidence: 1.0,
		},
		{
			name:  "query contained in artist name",
			query: "Taylor",
			mockArtist: &server.Artist{
				ID:   "06HL4z0CvFAxyc27GXpf02",
				Name: "Taylor Swift",
				URI:  "spotify:artist:06HL4z0CvFAxyc27GXpf02",
			},
			minConfidence: 0.8,
			maxConfidence: 1.0,
		},
		{
			name:  "artist name contained in query",
			query: "Taylor Swift songs",
			mockArtist: &server.Artist{
				ID:   "06HL4z0CvFAxyc27GXpf02",
				Name: "Taylor Swift",
				URI:  "spotify:artist:06HL4z0CvFAxyc27GXpf02",
			},
			minConfidence: 0.7,
			maxConfidence: 0.9,
		},
		{
			name:  "fuzzy match",
			query: "Tylor Swft",
			mockArtist: &server.Artist{
				ID:   "06HL4z0CvFAxyc27GXpf02",
				Name: "Taylor Swift",
				URI:  "spotify:artist:06HL4z0CvFAxyc27GXpf02",
			},
			minConfidence: 0.1,
			maxConfidence: 0.7,
		},
		{
			name:  "poor match",
			query: "completely different",
			mockArtist: &server.Artist{
				ID:   "06HL4z0CvFAxyc27GXpf02",
				Name: "Taylor Swift",
				URI:  "spotify:artist:06HL4z0CvFAxyc27GXpf02",
			},
			minConfidence: 0.1,
			maxConfidence: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSpotify := &MockSpotifyService{
				searchArtistFunc: func(query string) (*server.Artist, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return tt.mockArtist, nil
				},
			}

			searcher := NewFuzzyArtistSearcher(mockSpotify, logger)
			artist, confidence, err := searcher.FindBestMatch(tt.query)

			if tt.wantErr {
				if err == nil {
					t.Errorf("FindBestMatch() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("FindBestMatch() unexpected error = %v", err)
				return
			}

			if artist == nil {
				t.Error("FindBestMatch() returned nil artist")
				return
			}

			if tt.wantConfidence > 0 {
				if confidence != tt.wantConfidence {
					t.Errorf("FindBestMatch() confidence = %v, want %v", confidence, tt.wantConfidence)
				}
			} else if tt.minConfidence > 0 || tt.maxConfidence > 0 {
				if confidence < tt.minConfidence || confidence > tt.maxConfidence {
					t.Errorf("FindBestMatch() confidence = %v, want between %v and %v", confidence, tt.minConfidence, tt.maxConfidence)
				}
			}

			if artist.ID != tt.mockArtist.ID {
				t.Errorf("FindBestMatch() artist ID = %v, want %v", artist.ID, tt.mockArtist.ID)
			}
		})
	}
}

func TestFuzzyArtistSearcher_calculateMatchConfidence(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockSpotify := &MockSpotifyService{}
	searcher := NewFuzzyArtistSearcher(mockSpotify, logger)

	tests := []struct {
		name           string
		query          string
		artistName     string
		wantConfidence float64
		minConfidence  float64
		maxConfidence  float64
	}{
		{
			name:           "exact match",
			query:          "Taylor Swift",
			artistName:     "Taylor Swift",
			wantConfidence: 1.0,
		},
		{
			name:           "case insensitive exact match",
			query:          "taylor swift",
			artistName:     "Taylor Swift",
			wantConfidence: 1.0,
		},
		{
			name:          "query contained in artist",
			query:         "Taylor",
			artistName:    "Taylor Swift",
			minConfidence: 0.8,
			maxConfidence: 1.0,
		},
		{
			name:          "artist contained in query",
			query:         "Taylor Swift songs",
			artistName:    "Taylor Swift",
			minConfidence: 0.7,
			maxConfidence: 0.9,
		},
		{
			name:          "fuzzy match",
			query:         "Tylor",
			artistName:    "Taylor",
			minConfidence: 0.1,
			maxConfidence: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := searcher.calculateMatchConfidence(tt.query, tt.artistName)

			if tt.wantConfidence > 0 {
				if confidence != tt.wantConfidence {
					t.Errorf("calculateMatchConfidence() = %v, want %v", confidence, tt.wantConfidence)
				}
			} else {
				if confidence < tt.minConfidence || confidence > tt.maxConfidence {
					t.Errorf("calculateMatchConfidence() = %v, want between %v and %v", confidence, tt.minConfidence, tt.maxConfidence)
				}
			}
		})
	}
}

func TestFuzzyArtistSearcher_SearchMultipleArtists(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockArtists := map[string]*server.Artist{
		"Taylor Swift": {
			ID:   "06HL4z0CvFAxyc27GXpf02",
			Name: "Taylor Swift",
			URI:  "spotify:artist:06HL4z0CvFAxyc27GXpf02",
		},
		"Ed Sheeran": {
			ID:   "6eUKZXaKkcviH0Ku9w2n3V",
			Name: "Ed Sheeran",
			URI:  "spotify:artist:6eUKZXaKkcviH0Ku9w2n3V",
		},
	}

	mockSpotify := &MockSpotifyService{
		searchArtistFunc: func(query string) (*server.Artist, error) {
			// Simple mock that returns exact matches
			for name, artist := range mockArtists {
				if query == name {
					return artist, nil
				}
			}
			return nil, errors.New("artist not found")
		},
	}

	searcher := NewFuzzyArtistSearcher(mockSpotify, logger)

	tests := []struct {
		name      string
		queries   []string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "empty queries",
			queries:   []string{},
			wantCount: 0,
		},
		{
			name:      "single valid query",
			queries:   []string{"Taylor Swift"},
			wantCount: 1,
		},
		{
			name:      "multiple valid queries",
			queries:   []string{"Taylor Swift", "Ed Sheeran"},
			wantCount: 2,
		},
		{
			name:      "mixed valid and invalid queries",
			queries:   []string{"Taylor Swift", "Unknown Artist", "Ed Sheeran"},
			wantCount: 2, // Only valid ones should be returned
		},
		{
			name:      "all invalid queries",
			queries:   []string{"Unknown Artist 1", "Unknown Artist 2"},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := searcher.SearchMultipleArtists(tt.queries)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SearchMultipleArtists() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("SearchMultipleArtists() unexpected error = %v", err)
				return
			}

			if len(results) != tt.wantCount {
				t.Errorf("SearchMultipleArtists() returned %d results, want %d", len(results), tt.wantCount)
			}

			// Verify each result has required fields
			for i, result := range results {
				if result.Artist == nil {
					t.Errorf("SearchMultipleArtists() result[%d] has nil artist", i)
				}
				if result.Query == "" {
					t.Errorf("SearchMultipleArtists() result[%d] has empty query", i)
				}
				if result.Confidence <= 0 {
					t.Errorf("SearchMultipleArtists() result[%d] has invalid confidence: %v", i, result.Confidence)
				}
			}
		})
	}
}

func TestArtistMatch_IsHighConfidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		want       bool
	}{
		{"high confidence", 0.9, true},
		{"exactly 0.8", 0.8, true},
		{"medium confidence", 0.7, false},
		{"low confidence", 0.3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := ArtistMatch{Confidence: tt.confidence}
			if got := match.IsHighConfidence(); got != tt.want {
				t.Errorf("ArtistMatch.IsHighConfidence() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestArtistMatch_IsLowConfidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		want       bool
	}{
		{"high confidence", 0.9, false},
		{"medium confidence", 0.7, false},
		{"exactly 0.5", 0.5, false},
		{"low confidence", 0.3, true},
		{"very low confidence", 0.1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := ArtistMatch{Confidence: tt.confidence}
			if got := match.IsLowConfidence(); got != tt.want {
				t.Errorf("ArtistMatch.IsLowConfidence() = %v, want %v", got, tt.want)
			}
		})
	}
}
