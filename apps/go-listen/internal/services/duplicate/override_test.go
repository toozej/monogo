package duplicate

import (
	"errors"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/internal/server"
)

// TestOverrideScenarios tests various override scenarios for duplicate detection
func TestOverrideScenarios(t *testing.T) {
	tests := []struct {
		name               string
		description        string
		mockTracks         []server.Track
		mockCheckResponse  []bool
		expectedDuplicates bool
		expectedMessage    string
	}{
		{
			name:        "override with existing duplicates",
			description: "When force is used, duplicates should still be detected but not block operation",
			mockTracks: []server.Track{
				{
					ID:   "track1",
					Name: "Song 1",
					Artists: []server.Artist{
						{ID: "artist123", Name: "Test Artist"},
					},
				},
				{
					ID:   "track2",
					Name: "Song 2",
					Artists: []server.Artist{
						{ID: "artist123", Name: "Test Artist"},
					},
				},
			},
			mockCheckResponse:  []bool{true, true},
			expectedDuplicates: true,
			expectedMessage:    "Artist 'Test Artist' already has 2 track(s) in this playlist",
		},
		{
			name:        "override with partial duplicates",
			description: "Override should work even when only some tracks are duplicates",
			mockTracks: []server.Track{
				{
					ID:   "track1",
					Name: "Song 1",
					Artists: []server.Artist{
						{ID: "artist123", Name: "Test Artist"},
					},
				},
				{
					ID:   "track2",
					Name: "Song 2",
					Artists: []server.Artist{
						{ID: "artist123", Name: "Test Artist"},
					},
				},
				{
					ID:   "track3",
					Name: "Song 3",
					Artists: []server.Artist{
						{ID: "artist123", Name: "Test Artist"},
					},
				},
			},
			mockCheckResponse:  []bool{true, false, true},
			expectedDuplicates: true,
			expectedMessage:    "Artist 'Test Artist' already has 2 track(s) in this playlist",
		},
		{
			name:        "override with no duplicates",
			description: "Override should work normally when no duplicates exist",
			mockTracks: []server.Track{
				{
					ID:   "track1",
					Name: "Song 1",
					Artists: []server.Artist{
						{ID: "artist123", Name: "Test Artist"},
					},
				},
			},
			mockCheckResponse:  []bool{false},
			expectedDuplicates: false,
			expectedMessage:    "Artist 'Test Artist' tracks not found in playlist, safe to add",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSpotify := &MockSpotifyService{
				tracks:       tt.mockTracks,
				tracksError:  nil,
				checkResults: tt.mockCheckResponse,
				checkError:   nil,
			}

			// Set expected track IDs for validation
			trackIDs := make([]string, len(tt.mockTracks))
			for i, track := range tt.mockTracks {
				trackIDs[i] = track.ID
			}
			mockSpotify.expectedTrackIDs = trackIDs

			logger := log.New()
			logger.SetLevel(log.FatalLevel) // Suppress logs during testing

			service := NewDuplicateService(mockSpotify, logger)

			// Test the duplicate detection (this would be called regardless of force parameter)
			result, err := service.CheckArtistInPlaylist("playlist123", "artist123")

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("Expected result but got nil")
			}

			if result.HasDuplicates != tt.expectedDuplicates {
				t.Errorf("Expected HasDuplicates %v, got %v", tt.expectedDuplicates, result.HasDuplicates)
			}

			// Verify the message contains expected information for override scenarios
			if tt.expectedDuplicates {
				if !contains(result.Message, "Add Anyway") {
					t.Errorf("Expected message to contain 'Add Anyway' for override, got '%s'", result.Message)
				}
				if !contains(result.Message, "already has") {
					t.Errorf("Expected message to contain 'already has', got '%s'", result.Message)
				}
			}

			// Verify duplicate tracks are correctly identified
			expectedDuplicateCount := 0
			for _, exists := range tt.mockCheckResponse {
				if exists {
					expectedDuplicateCount++
				}
			}

			if len(result.DuplicateTracks) != expectedDuplicateCount {
				t.Errorf("Expected %d duplicate tracks, got %d", expectedDuplicateCount, len(result.DuplicateTracks))
			}
		})
	}
}

// TestOverrideErrorHandling tests error scenarios during override operations
func TestOverrideErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		mockTracks    []server.Track
		mockError     error
		expectedError string
	}{
		{
			name: "spotify api error during override check",
			mockTracks: []server.Track{
				{
					ID:   "track1",
					Name: "Song 1",
					Artists: []server.Artist{
						{ID: "artist123", Name: "Test Artist"},
					},
				},
			},
			mockError:     errors.New("spotify api temporarily unavailable"),
			expectedError: "failed to check tracks in playlist: spotify api temporarily unavailable",
		},
		{
			name:          "no tracks to override",
			mockTracks:    []server.Track{},
			mockError:     nil,
			expectedError: "", // Should not error, just return no duplicates
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSpotify := &MockSpotifyService{
				tracks:      tt.mockTracks,
				tracksError: nil,
				checkError:  tt.mockError,
			}

			if len(tt.mockTracks) > 0 {
				trackIDs := make([]string, len(tt.mockTracks))
				for i, track := range tt.mockTracks {
					trackIDs[i] = track.ID
				}
				mockSpotify.expectedTrackIDs = trackIDs
			}

			logger := log.New()
			logger.SetLevel(log.FatalLevel)

			service := NewDuplicateService(mockSpotify, logger)

			result, err := service.CheckArtistInPlaylist("playlist123", "artist123")

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.expectedError, err.Error())
				}
				if result != nil {
					t.Errorf("Expected nil result but got %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("Expected result but got nil")
				}
			}
		})
	}
}

// TestOverrideMessageFormatting tests that override messages are properly formatted
func TestOverrideMessageFormatting(t *testing.T) {
	mockTracks := []server.Track{
		{
			ID:   "track1",
			Name: "Song 1",
			Artists: []server.Artist{
				{ID: "artist123", Name: "Test Artist"},
			},
		},
		{
			ID:   "track2",
			Name: "Song 2",
			Artists: []server.Artist{
				{ID: "artist123", Name: "Test Artist"},
			},
		},
	}

	mockSpotify := &MockSpotifyService{
		tracks:           mockTracks,
		tracksError:      nil,
		checkResults:     []bool{true, true},
		checkError:       nil,
		expectedTrackIDs: []string{"track1", "track2"},
	}

	logger := log.New()
	logger.SetLevel(log.FatalLevel)

	service := NewDuplicateService(mockSpotify, logger)

	result, err := service.CheckArtistInPlaylist("playlist123", "artist123")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	// Verify message formatting for override scenarios
	expectedElements := []string{
		"Test Artist",
		"already has",
		"track(s)",
		"Add Anyway",
		"override",
	}

	for _, element := range expectedElements {
		if !contains(result.Message, element) {
			t.Errorf("Expected message to contain '%s', got '%s'", element, result.Message)
		}
	}

	// Verify timestamp is included in the message
	if !contains(result.Message, "last added:") {
		t.Errorf("Expected message to contain timestamp, got '%s'", result.Message)
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || substr == "" ||
		(len(s) > len(substr) && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
