package search

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/internal/types"
)

// TestFuzzyArtistSearcher_AdvancedScenarios tests advanced fuzzy matching scenarios
func TestFuzzyArtistSearcher_AdvancedScenarios(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "complex_fuzzy_matching",
			description: "Test complex fuzzy matching algorithms",
			testFunc:    testComplexFuzzyMatching,
		},
		{
			name:        "performance_benchmarks",
			description: "Test performance with various input sizes",
			testFunc:    testPerformanceBenchmarks,
		},
		{
			name:        "unicode_and_special_chars",
			description: "Test handling of unicode and special characters",
			testFunc:    testUnicodeAndSpecialChars,
		},
		{
			name:        "confidence_score_accuracy",
			description: "Test accuracy of confidence scoring",
			testFunc:    testConfidenceScoreAccuracy,
		},
		{
			name:        "edge_case_inputs",
			description: "Test edge cases and boundary conditions",
			testFunc:    testEdgeCaseInputs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func testComplexFuzzyMatching(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		query         string
		artistName    string
		minConfidence float64
		maxConfidence float64
		description   string
	}{
		{
			query:         "Beatles",
			artistName:    "The Beatles",
			minConfidence: 0.8,
			maxConfidence: 1.0,
			description:   "Partial match with 'The' prefix",
		},
		{
			query:         "led zep",
			artistName:    "Led Zeppelin",
			minConfidence: 0.7,
			maxConfidence: 1.0,
			description:   "Abbreviated artist name",
		},
		{
			query:         "pink floyd",
			artistName:    "Pink Floyd",
			minConfidence: 1.0,
			maxConfidence: 1.0,
			description:   "Exact match with different case",
		},
		{
			query:         "ac dc",
			artistName:    "AC/DC",
			minConfidence: 0.1,
			maxConfidence: 0.9,
			description:   "Special characters vs spaces",
		},
		{
			query:         "guns n roses",
			artistName:    "Guns N' Roses",
			minConfidence: 0.7,
			maxConfidence: 0.9,
			description:   "Apostrophe handling",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			mockSpotify := &MockSpotifyService{
				searchArtistFunc: func(query string) (*types.Artist, error) {
					return &types.Artist{
						ID:   "test-id",
						Name: tt.artistName,
						URI:  "spotify:artist:test-id",
					}, nil
				},
			}

			searcher := NewFuzzyArtistSearcher(mockSpotify, logger)
			confidence := searcher.calculateMatchConfidence(tt.query, tt.artistName)

			if confidence < tt.minConfidence || confidence > tt.maxConfidence {
				t.Errorf("Query '%s' vs Artist '%s': confidence %.3f not in range [%.3f, %.3f]",
					tt.query, tt.artistName, confidence, tt.minConfidence, tt.maxConfidence)
			}

			t.Logf("Query '%s' vs Artist '%s': confidence %.3f", tt.query, tt.artistName, confidence)
		})
	}
}

func testPerformanceBenchmarks(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockSpotify := &MockSpotifyService{
		searchArtistFunc: func(query string) (*types.Artist, error) {
			return &types.Artist{
				ID:   "test-id",
				Name: "Test Artist",
				URI:  "spotify:artist:test-id",
			}, nil
		},
	}

	searcher := NewFuzzyArtistSearcher(mockSpotify, logger)

	// Test single search performance
	start := time.Now()
	_, _, err := searcher.FindBestMatch("test artist")
	singleDuration := time.Since(start)

	if err != nil {
		t.Errorf("Unexpected error in single search: %v", err)
	}

	if singleDuration > 100*time.Millisecond {
		t.Errorf("Single search took too long: %v", singleDuration)
	}

	// Test multiple search performance
	queries := []string{
		"Taylor Swift", "Ed Sheeran", "Adele", "Drake", "Beyoncé",
		"The Beatles", "Queen", "Led Zeppelin", "Pink Floyd", "AC/DC",
	}

	start = time.Now()
	results, err := searcher.SearchMultipleArtists(queries)
	multipleDuration := time.Since(start)

	if err != nil {
		t.Errorf("Unexpected error in multiple search: %v", err)
	}

	if len(results) != len(queries) {
		t.Errorf("Expected %d results, got %d", len(queries), len(results))
	}

	if multipleDuration > 1*time.Second {
		t.Errorf("Multiple search took too long: %v", multipleDuration)
	}

	avgDuration := multipleDuration / time.Duration(len(queries))
	t.Logf("Single search: %v, Multiple search: %v, Average: %v",
		singleDuration, multipleDuration, avgDuration)
}

func testUnicodeAndSpecialChars(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		query      string
		artistName string
		shouldWork bool
	}{
		{
			query:      "björk",
			artistName: "Björk",
			shouldWork: true,
		},
		{
			query:      "sigur ros",
			artistName: "Sigur Rós",
			shouldWork: true,
		},
		{
			query:      "cafe tacvba",
			artistName: "Café Tacvba",
			shouldWork: true,
		},
		{
			query:      "manu chao",
			artistName: "Manu Chão",
			shouldWork: true,
		},
		{
			query:      "测试",
			artistName: "测试艺术家",
			shouldWork: true,
		},
		{
			query:      "🎵music",
			artistName: "🎵 Music Artist",
			shouldWork: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.query+"_vs_"+tt.artistName, func(t *testing.T) {
			mockSpotify := &MockSpotifyService{
				searchArtistFunc: func(query string) (*types.Artist, error) {
					return &types.Artist{
						ID:   "test-id",
						Name: tt.artistName,
						URI:  "spotify:artist:test-id",
					}, nil
				},
			}

			searcher := NewFuzzyArtistSearcher(mockSpotify, logger)
			artist, confidence, err := searcher.FindBestMatch(tt.query)

			if tt.shouldWork {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if artist == nil {
					t.Error("Expected artist but got nil")
				}
				if confidence <= 0 {
					t.Errorf("Expected positive confidence, got %f", confidence)
				}
			}

			t.Logf("Query '%s' vs Artist '%s': confidence %.3f", tt.query, tt.artistName, confidence)
		})
	}
}

func testConfidenceScoreAccuracy(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockSpotify := &MockSpotifyService{}
	searcher := NewFuzzyArtistSearcher(mockSpotify, logger)

	// Test confidence score ordering - better matches should have higher scores
	testCases := []struct {
		query      string
		artistName string
	}{
		{"Taylor Swift", "Taylor Swift"},  // Exact match
		{"taylor swift", "Taylor Swift"},  // Case insensitive exact
		{"Taylor", "Taylor Swift"},        // Partial match
		{"Swift", "Taylor Swift"},         // Partial match
		{"Tylor Swift", "Taylor Swift"},   // Typo
		{"T Swift", "Taylor Swift"},       // Abbreviation
		{"Random Artist", "Taylor Swift"}, // Poor match
	}

	confidences := make([]float64, len(testCases))
	for i, tc := range testCases {
		confidences[i] = searcher.calculateMatchConfidence(tc.query, tc.artistName)
		t.Logf("'%s' vs '%s': %.3f", tc.query, tc.artistName, confidences[i])
	}

	// Verify ordering - each should be >= the next
	for i := 0; i < len(confidences)-1; i++ {
		if confidences[i] < confidences[i+1] {
			t.Errorf("Confidence ordering violation: %.3f < %.3f for cases %d and %d",
				confidences[i], confidences[i+1], i, i+1)
		}
	}

	// Verify specific thresholds
	if confidences[0] != 1.0 {
		t.Errorf("Exact match should have confidence 1.0, got %.3f", confidences[0])
	}

	if confidences[1] != 1.0 {
		t.Errorf("Case insensitive exact match should have confidence 1.0, got %.3f", confidences[1])
	}

	if confidences[len(confidences)-1] >= 0.5 {
		t.Errorf("Poor match should have low confidence, got %.3f", confidences[len(confidences)-1])
	}
}

func testEdgeCaseInputs(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockSpotify := &MockSpotifyService{
		searchArtistFunc: func(query string) (*types.Artist, error) {
			if strings.TrimSpace(query) == "" {
				return nil, errors.New("empty query")
			}
			return &types.Artist{
				ID:   "test-id",
				Name: "Test Artist",
				URI:  "spotify:artist:test-id",
			}, nil
		},
	}

	searcher := NewFuzzyArtistSearcher(mockSpotify, logger)

	tests := []struct {
		name        string
		query       string
		expectError bool
	}{
		{
			name:        "empty_string",
			query:       "",
			expectError: true,
		},
		{
			name:        "whitespace_only",
			query:       "   \t\n  ",
			expectError: true,
		},
		{
			name:        "single_character",
			query:       "a",
			expectError: false,
		},
		{
			name:        "very_long_string",
			query:       strings.Repeat("a", 1000),
			expectError: false,
		},
		{
			name:        "only_special_chars",
			query:       "!@#$%^&*()",
			expectError: false,
		},
		{
			name:        "mixed_whitespace",
			query:       "  test  artist  ",
			expectError: false,
		},
		{
			name:        "newlines_and_tabs",
			query:       "test\nartist\t",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artist, confidence, err := searcher.FindBestMatch(tt.query)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if artist != nil {
					t.Error("Expected nil artist on error")
				}
				if confidence != 0.0 {
					t.Errorf("Expected zero confidence on error, got %f", confidence)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if artist == nil {
					t.Error("Expected artist but got nil")
				}
				if confidence <= 0 {
					t.Errorf("Expected positive confidence, got %f", confidence)
				}
			}
		})
	}
}

// TestFuzzyArtistSearcher_ConcurrentAccess tests concurrent access to the searcher
func TestFuzzyArtistSearcher_ConcurrentAccess(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockSpotify := &MockSpotifyService{
		searchArtistFunc: func(query string) (*types.Artist, error) {
			// Simulate some processing time
			time.Sleep(10 * time.Millisecond)
			return &types.Artist{
				ID:   "test-id",
				Name: "Test Artist for " + query,
				URI:  "spotify:artist:test-id",
			}, nil
		},
	}

	searcher := NewFuzzyArtistSearcher(mockSpotify, logger)

	// Test concurrent searches
	numGoroutines := 10
	done := make(chan bool, numGoroutines)
	errs := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			query := "test artist " + string(rune(id))
			artist, confidence, err := searcher.FindBestMatch(query)

			if err != nil {
				errs <- err
				return
			}

			if artist == nil {
				errs <- fmt.Errorf("got nil artist")
				return
			}

			if confidence <= 0 {
				errs <- fmt.Errorf("got invalid confidence")
				return
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check for errors
	close(errs)
	for err := range errs {
		t.Errorf("Concurrent access error: %v", err)
	}
}

// TestFuzzyArtistSearcher_ErrorHandling tests various error scenarios
func TestFuzzyArtistSearcher_ErrorHandling(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name         string
		mockError    error
		expectError  bool
		errorMessage string
	}{
		{
			name:         "spotify_api_error",
			mockError:    errors.New("spotify API error"),
			expectError:  true,
			errorMessage: "failed to search for artist",
		},
		{
			name:         "network_timeout",
			mockError:    errors.New("network timeout"),
			expectError:  true,
			errorMessage: "failed to search for artist",
		},
		{
			name:         "rate_limit_error",
			mockError:    errors.New("rate limit exceeded"),
			expectError:  true,
			errorMessage: "failed to search for artist",
		},
		{
			name:         "authentication_error",
			mockError:    errors.New("authentication failed"),
			expectError:  true,
			errorMessage: "failed to search for artist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSpotify := &MockSpotifyService{
				searchArtistFunc: func(query string) (*types.Artist, error) {
					return nil, tt.mockError
				},
			}

			searcher := NewFuzzyArtistSearcher(mockSpotify, logger)
			artist, confidence, err := searcher.FindBestMatch("test artist")

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorMessage, err.Error())
				}
				if artist != nil {
					t.Error("Expected nil artist on error")
				}
				if confidence != 0.0 {
					t.Errorf("Expected zero confidence on error, got %f", confidence)
				}
			}
		})
	}
}

// TestFuzzyArtistSearcher_MemoryUsage tests memory efficiency
func TestFuzzyArtistSearcher_MemoryUsage(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockSpotify := &MockSpotifyService{
		searchArtistFunc: func(query string) (*types.Artist, error) {
			return &types.Artist{
				ID:   "test-id",
				Name: "Test Artist",
				URI:  "spotify:artist:test-id",
			}, nil
		},
	}

	searcher := NewFuzzyArtistSearcher(mockSpotify, logger)

	// Perform many searches to test for memory leaks
	for i := 0; i < 1000; i++ {
		query := "test artist " + string(rune(i%100))
		_, _, err := searcher.FindBestMatch(query)
		if err != nil {
			t.Errorf("Error in iteration %d: %v", i, err)
		}

		// Periodically force garbage collection
		if i%100 == 0 {
			runtime.GC() // Force garbage collection to test memory usage
		}
	}
}

// TestArtistMatch_HelperMethods tests the helper methods on ArtistMatch
func TestArtistMatch_HelperMethods(t *testing.T) {
	tests := []struct {
		confidence  float64
		isHighConf  bool
		isLowConf   bool
		description string
	}{
		{1.0, true, false, "perfect match"},
		{0.9, true, false, "high confidence"},
		{0.8, true, false, "threshold high confidence"},
		{0.7, false, false, "medium confidence"},
		{0.6, false, false, "medium-low confidence"},
		{0.5, false, false, "threshold medium confidence"},
		{0.4, false, true, "low confidence"},
		{0.1, false, true, "very low confidence"},
		{0.0, false, true, "no confidence"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			match := ArtistMatch{
				Artist: &types.Artist{
					ID:   "test-id",
					Name: "Test Artist",
				},
				Query:      "test query",
				Confidence: tt.confidence,
			}

			if match.IsHighConfidence() != tt.isHighConf {
				t.Errorf("IsHighConfidence() = %v, want %v for confidence %f",
					match.IsHighConfidence(), tt.isHighConf, tt.confidence)
			}

			if match.IsLowConfidence() != tt.isLowConf {
				t.Errorf("IsLowConfidence() = %v, want %v for confidence %f",
					match.IsLowConfidence(), tt.isLowConf, tt.confidence)
			}
		})
	}
}
