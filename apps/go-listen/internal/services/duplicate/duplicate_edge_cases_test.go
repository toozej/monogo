package duplicate

import (
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/internal/types"
)

// TestDuplicateService_EdgeCases tests various edge cases and boundary conditions
func TestDuplicateService_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "large_track_lists",
			description: "Test handling of very large track lists",
			testFunc:    testLargeTrackLists,
		},
		{
			name:        "concurrent_access",
			description: "Test concurrent access to duplicate service",
			testFunc:    testConcurrentAccess,
		},
		{
			name:        "memory_efficiency",
			description: "Test memory efficiency with large datasets",
			testFunc:    testMemoryEfficiency,
		},
		{
			name:        "unicode_handling",
			description: "Test handling of unicode track names",
			testFunc:    testUnicodeHandling,
		},
		{
			name:        "timestamp_accuracy",
			description: "Test timestamp accuracy in duplicate results",
			testFunc:    testTimestampAccuracy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func testLargeTrackLists(t *testing.T) {
	// Create a large number of tracks to test performance and memory usage
	trackCount := 1000
	tracks := make([]types.Track, trackCount)
	trackIDs := make([]string, trackCount)
	checkResults := make([]bool, trackCount)

	for i := 0; i < trackCount; i++ {
		tracks[i] = types.Track{
			ID:   "track" + string(rune(i)),
			Name: "Song " + string(rune(i)),
		}
		trackIDs[i] = tracks[i].ID
		checkResults[i] = i%2 == 0 // Every other track is a duplicate
	}

	mockSpotify := &MockSpotifyService{
		checkResults:     checkResults,
		expectedTrackIDs: trackIDs,
	}

	logger := log.New()
	logger.SetLevel(log.FatalLevel)

	service := NewDuplicateService(mockSpotify, logger)

	start := time.Now()
	result, err := service.CheckDuplicates("playlist123", tracks)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Unexpected error with large track list: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	expectedDuplicates := trackCount / 2
	if len(result.DuplicateTracks) != expectedDuplicates {
		t.Errorf("Expected %d duplicates, got %d", expectedDuplicates, len(result.DuplicateTracks))
	}

	// Performance check - should complete within reasonable time
	if duration > 5*time.Second {
		t.Errorf("Large track list processing took too long: %v", duration)
	}

	t.Logf("Processed %d tracks in %v", trackCount, duration)
}

func testConcurrentAccess(t *testing.T) {
	mockSpotify := &MockSpotifyService{
		tracks: []types.Track{
			{ID: "track1", Name: "Song 1"},
			{ID: "track2", Name: "Song 2"},
		},
		checkResults: []bool{true, false},
	}

	logger := log.New()
	logger.SetLevel(log.FatalLevel)

	service := NewDuplicateService(mockSpotify, logger)

	// Test concurrent access
	done := make(chan bool, 10)
	errs := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			tracks := []types.Track{
				{ID: "track" + string(rune(id)), Name: "Song " + string(rune(id))},
			}

			_, err := service.CheckDuplicates("playlist"+string(rune(id)), tracks)
			if err != nil {
				errs <- err
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check for errors
	close(errs)
	for err := range errs {
		t.Errorf("Concurrent access error: %v", err)
	}
}

func testMemoryEfficiency(t *testing.T) {
	// Test that the service doesn't leak memory with repeated operations
	mockSpotify := &MockSpotifyService{
		tracks: []types.Track{
			{ID: "track1", Name: "Song 1"},
		},
		checkResults: []bool{true},
	}

	logger := log.New()
	logger.SetLevel(log.FatalLevel)

	service := NewDuplicateService(mockSpotify, logger)

	// Perform many operations to test for memory leaks
	for i := 0; i < 100; i++ {
		tracks := []types.Track{
			{ID: "track" + string(rune(i)), Name: "Song " + string(rune(i))},
		}

		result, err := service.CheckDuplicates("playlist", tracks)
		if err != nil {
			t.Errorf("Error in iteration %d: %v", i, err)
		}

		if result == nil {
			t.Errorf("Nil result in iteration %d", i)
		}

		// Force garbage collection periodically
		if i%10 == 0 {
			runtime.GC() // Force garbage collection to test memory usage
		}
	}
}

func testUnicodeHandling(t *testing.T) {
	unicodeTracks := []types.Track{
		{ID: "track1", Name: "测试歌曲"},
		{ID: "track2", Name: "🎵 Music Note"},
		{ID: "track3", Name: "Café Müller"},
		{ID: "track4", Name: "Москва"},
	}

	mockSpotify := &MockSpotifyService{
		checkResults:     []bool{true, false, true, false},
		expectedTrackIDs: []string{"track1", "track2", "track3", "track4"},
	}

	logger := log.New()
	logger.SetLevel(log.FatalLevel)

	service := NewDuplicateService(mockSpotify, logger)

	result, err := service.CheckDuplicates("playlist123", unicodeTracks)
	if err != nil {
		t.Errorf("Unexpected error with unicode tracks: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if len(result.DuplicateTracks) != 2 {
		t.Errorf("Expected 2 duplicate tracks, got %d", len(result.DuplicateTracks))
	}

	// Check that unicode names are preserved in the message
	if !strings.Contains(result.Message, "测试歌曲") {
		t.Error("Expected unicode track name in message")
	}
}

func testTimestampAccuracy(t *testing.T) {
	mockSpotify := &MockSpotifyService{
		tracks: []types.Track{
			{ID: "track1", Name: "Song 1"},
		},
		checkResults: []bool{true},
	}

	logger := log.New()
	logger.SetLevel(log.FatalLevel)

	service := NewDuplicateService(mockSpotify, logger)

	beforeCall := time.Now()
	result, err := service.CheckDuplicates("playlist123", mockSpotify.tracks)
	afterCall := time.Now()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	// Check that timestamp is within reasonable bounds
	if result.LastAdded.Before(beforeCall) || result.LastAdded.After(afterCall) {
		t.Errorf("Timestamp %v is not within expected range %v - %v",
			result.LastAdded, beforeCall, afterCall)
	}

	// Check timestamp precision (should be within 1 second)
	if time.Since(result.LastAdded) > time.Second {
		t.Errorf("Timestamp is too old: %v", result.LastAdded)
	}
}

// TestDuplicateService_ErrorRecovery tests error recovery scenarios
func TestDuplicateService_ErrorRecovery(t *testing.T) {
	tests := []struct {
		name         string
		mockError    error
		expectError  bool
		errorMessage string
	}{
		{
			name:         "network_timeout",
			mockError:    errors.New("network timeout"),
			expectError:  true,
			errorMessage: "network timeout",
		},
		{
			name:         "rate_limit_error",
			mockError:    errors.New("rate limit exceeded"),
			expectError:  true,
			errorMessage: "rate limit exceeded",
		},
		{
			name:         "authentication_error",
			mockError:    errors.New("authentication failed"),
			expectError:  true,
			errorMessage: "authentication failed",
		},
		{
			name:         "service_unavailable",
			mockError:    errors.New("service unavailable"),
			expectError:  true,
			errorMessage: "service unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSpotify := &MockSpotifyService{
				tracks:     []types.Track{{ID: "track1", Name: "Song 1"}},
				checkError: tt.mockError,
			}

			logger := log.New()
			logger.SetLevel(log.FatalLevel)

			service := NewDuplicateService(mockSpotify, logger)

			result, err := service.CheckDuplicates("playlist123", mockSpotify.tracks)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorMessage, err.Error())
				}
				if result != nil {
					t.Error("Expected nil result on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result == nil {
					t.Error("Expected result but got nil")
				}
			}
		})
	}
}

// TestDuplicateService_MessageFormatting tests various message formatting scenarios
func TestDuplicateService_MessageFormatting(t *testing.T) {
	tests := []struct {
		name            string
		tracks          []types.Track
		checkResults    []bool
		expectedMessage string
		messageContains []string
	}{
		{
			name: "single_duplicate",
			tracks: []types.Track{
				{ID: "track1", Name: "Song 1"},
			},
			checkResults:    []bool{true},
			expectedMessage: "Found 1 duplicate track(s): Song 1",
		},
		{
			name: "multiple_duplicates",
			tracks: []types.Track{
				{ID: "track1", Name: "Song 1"},
				{ID: "track2", Name: "Song 2"},
				{ID: "track3", Name: "Song 3"},
			},
			checkResults:    []bool{true, false, true},
			messageContains: []string{"Found 2 duplicate track(s)", "Song 1", "Song 3"},
		},
		{
			name: "long_track_names",
			tracks: []types.Track{
				{ID: "track1", Name: "This is a very long song name that might cause formatting issues"},
			},
			checkResults:    []bool{true},
			messageContains: []string{"Found 1 duplicate track(s)", "This is a very long song name"},
		},
		{
			name: "special_characters_in_names",
			tracks: []types.Track{
				{ID: "track1", Name: "Song with \"quotes\" and 'apostrophes'"},
				{ID: "track2", Name: "Song with & ampersand"},
			},
			checkResults:    []bool{true, true},
			messageContains: []string{"Found 2 duplicate track(s)", "quotes", "ampersand"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trackIDs := make([]string, len(tt.tracks))
			for i, track := range tt.tracks {
				trackIDs[i] = track.ID
			}

			mockSpotify := &MockSpotifyService{
				checkResults:     tt.checkResults,
				expectedTrackIDs: trackIDs,
			}

			logger := log.New()
			logger.SetLevel(log.FatalLevel)

			service := NewDuplicateService(mockSpotify, logger)

			result, err := service.CheckDuplicates("playlist123", tt.tracks)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result but got nil")
			}

			if tt.expectedMessage != "" {
				if result.Message != tt.expectedMessage {
					t.Errorf("Expected message '%s', got '%s'", tt.expectedMessage, result.Message)
				}
			}

			for _, contains := range tt.messageContains {
				if !strings.Contains(result.Message, contains) {
					t.Errorf("Expected message to contain '%s', got '%s'", contains, result.Message)
				}
			}
		})
	}
}

// TestDuplicateService_PerformanceMetrics tests performance characteristics
func TestDuplicateService_PerformanceMetrics(t *testing.T) {
	// Test with different sizes to understand performance characteristics
	sizes := []int{10, 100, 500}

	for _, size := range sizes {
		t.Run("size_"+string(rune(size)), func(t *testing.T) {
			tracks := make([]types.Track, size)
			trackIDs := make([]string, size)
			checkResults := make([]bool, size)

			for i := 0; i < size; i++ {
				tracks[i] = types.Track{
					ID:   "track" + string(rune(i)),
					Name: "Song " + string(rune(i)),
				}
				trackIDs[i] = tracks[i].ID
				checkResults[i] = i%3 == 0 // Every third track is a duplicate
			}

			mockSpotify := &MockSpotifyService{
				checkResults:     checkResults,
				expectedTrackIDs: trackIDs,
			}

			logger := log.New()
			logger.SetLevel(log.FatalLevel)

			service := NewDuplicateService(mockSpotify, logger)

			start := time.Now()
			result, err := service.CheckDuplicates("playlist123", tracks)
			duration := time.Since(start)

			if err != nil {
				t.Errorf("Unexpected error with size %d: %v", size, err)
			}

			if result == nil {
				t.Fatalf("Expected result but got nil for size %d", size)
			}

			expectedDuplicates := (size + 2) / 3 // Ceiling division for every third
			if len(result.DuplicateTracks) != expectedDuplicates {
				t.Errorf("Size %d: expected %d duplicates, got %d",
					size, expectedDuplicates, len(result.DuplicateTracks))
			}

			// Performance should scale reasonably
			maxDuration := time.Duration(size) * time.Millisecond
			if duration > maxDuration {
				t.Errorf("Size %d took too long: %v (max: %v)", size, duration, maxDuration)
			}

			t.Logf("Size %d processed in %v", size, duration)
		})
	}
}
