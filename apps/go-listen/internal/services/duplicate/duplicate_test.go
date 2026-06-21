package duplicate

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/internal/server"
)

// MockSpotifyService is a mock implementation of SpotifyService for testing
type MockSpotifyService struct {
	tracks           []server.Track
	tracksError      error
	checkResults     []bool
	checkError       error
	expectedTrackIDs []string
}

func (m *MockSpotifyService) SearchArtist(query string) (*server.Artist, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *MockSpotifyService) GetArtistTopTracks(artistID string) ([]server.Track, error) {
	return m.tracks, m.tracksError
}

func (m *MockSpotifyService) GetUserPlaylists(folderName string) ([]server.Playlist, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *MockSpotifyService) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	return errors.New("not implemented in mock")
}

func (m *MockSpotifyService) CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error) {
	// Verify expected track IDs if set
	if m.expectedTrackIDs != nil && !reflect.DeepEqual(trackIDs, m.expectedTrackIDs) {
		return nil, errors.New("unexpected track IDs")
	}
	return m.checkResults, m.checkError
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

func TestNewDuplicateService(t *testing.T) {
	mockSpotify := &MockSpotifyService{}
	logger := log.New()

	service := NewDuplicateService(mockSpotify, logger)

	if service == nil {
		t.Error("Expected service to be created, got nil")
		return
	}
	if service.spotify != mockSpotify {
		t.Error("Expected spotify service to be set correctly")
	}
	if service.logger != logger {
		t.Error("Expected logger to be set correctly")
	}
}

func TestDuplicateService_CheckDuplicates(t *testing.T) {
	tests := []struct {
		name           string
		playlistID     string
		tracks         []server.Track
		mockResponse   []bool
		mockError      error
		expectedResult *server.DuplicateResult
		expectedError  string
	}{
		{
			name:       "no tracks provided",
			playlistID: "playlist123",
			tracks:     []server.Track{},
			expectedResult: &server.DuplicateResult{
				HasDuplicates: false,
				Message:       "No tracks to check",
			},
		},
		{
			name:       "no duplicates found",
			playlistID: "playlist123",
			tracks: []server.Track{
				{ID: "track1", Name: "Song 1"},
				{ID: "track2", Name: "Song 2"},
			},
			mockResponse: []bool{false, false},
			expectedResult: &server.DuplicateResult{
				HasDuplicates:   false,
				DuplicateTracks: []server.Track{},
				Message:         "No duplicate tracks found",
			},
		},
		{
			name:       "some duplicates found",
			playlistID: "playlist123",
			tracks: []server.Track{
				{ID: "track1", Name: "Song 1"},
				{ID: "track2", Name: "Song 2"},
				{ID: "track3", Name: "Song 3"},
			},
			mockResponse: []bool{true, false, true},
			expectedResult: &server.DuplicateResult{
				HasDuplicates: true,
				DuplicateTracks: []server.Track{
					{ID: "track1", Name: "Song 1"},
					{ID: "track3", Name: "Song 3"},
				},
				Message: "Found 2 duplicate track(s): Song 1, Song 3",
			},
		},
		{
			name:       "all duplicates found",
			playlistID: "playlist123",
			tracks: []server.Track{
				{ID: "track1", Name: "Song 1"},
				{ID: "track2", Name: "Song 2"},
			},
			mockResponse: []bool{true, true},
			expectedResult: &server.DuplicateResult{
				HasDuplicates: true,
				DuplicateTracks: []server.Track{
					{ID: "track1", Name: "Song 1"},
					{ID: "track2", Name: "Song 2"},
				},
				Message: "Found 2 duplicate track(s): Song 1, Song 2",
			},
		},
		{
			name:       "spotify api error",
			playlistID: "playlist123",
			tracks: []server.Track{
				{ID: "track1", Name: "Song 1"},
			},
			mockError:     errors.New("spotify api error"),
			expectedError: "failed to check tracks in playlist: spotify api error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock
			mockSpotify := &MockSpotifyService{
				checkResults: tt.mockResponse,
				checkError:   tt.mockError,
			}

			// Set expected track IDs for validation
			if len(tt.tracks) > 0 {
				trackIDs := make([]string, len(tt.tracks))
				for i, track := range tt.tracks {
					trackIDs[i] = track.ID
				}
				mockSpotify.expectedTrackIDs = trackIDs
			}

			logger := log.New()
			logger.SetLevel(log.FatalLevel) // Suppress logs during testing

			service := NewDuplicateService(mockSpotify, logger)

			result, err := service.CheckDuplicates(tt.playlistID, tt.tracks)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.expectedError) {
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

				if result.HasDuplicates != tt.expectedResult.HasDuplicates {
					t.Errorf("Expected HasDuplicates %v, got %v", tt.expectedResult.HasDuplicates, result.HasDuplicates)
				}

				if result.Message != tt.expectedResult.Message {
					t.Errorf("Expected message '%s', got '%s'", tt.expectedResult.Message, result.Message)
				}

				if len(result.DuplicateTracks) != len(tt.expectedResult.DuplicateTracks) {
					t.Errorf("Expected %d duplicate tracks, got %d", len(tt.expectedResult.DuplicateTracks), len(result.DuplicateTracks))
				}

				// Check duplicate tracks match
				for i, expectedTrack := range tt.expectedResult.DuplicateTracks {
					if i < len(result.DuplicateTracks) {
						if result.DuplicateTracks[i].ID != expectedTrack.ID {
							t.Errorf("Expected track ID '%s', got '%s'", expectedTrack.ID, result.DuplicateTracks[i].ID)
						}
						if result.DuplicateTracks[i].Name != expectedTrack.Name {
							t.Errorf("Expected track name '%s', got '%s'", expectedTrack.Name, result.DuplicateTracks[i].Name)
						}
					}
				}

				// Verify LastAdded is set to a recent time
				if result.HasDuplicates || len(tt.tracks) > 0 {
					if time.Since(result.LastAdded) > time.Minute {
						t.Error("LastAdded timestamp should be recent")
					}
				}
			}
		})
	}
}

func TestDuplicateService_CheckArtistInPlaylist(t *testing.T) {
	tests := []struct {
		name              string
		playlistID        string
		artistID          string
		mockTracks        []server.Track
		mockTracksError   error
		mockCheckResponse []bool
		mockCheckError    error
		expectedResult    *server.DuplicateResult
		expectedError     string
	}{
		{
			name:       "artist has no tracks",
			playlistID: "playlist123",
			artistID:   "artist123",
			mockTracks: []server.Track{},
			expectedResult: &server.DuplicateResult{
				HasDuplicates: false,
				Message:       "Artist has no tracks",
			},
		},
		{
			name:       "artist tracks not in playlist",
			playlistID: "playlist123",
			artistID:   "artist123",
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
			mockCheckResponse: []bool{false, false},
			expectedResult: &server.DuplicateResult{
				HasDuplicates:   false,
				DuplicateTracks: []server.Track{},
				ArtistName:      "Test Artist",
				Message:         "Artist 'Test Artist' tracks not found in playlist, safe to add",
			},
		},
		{
			name:       "artist tracks already in playlist",
			playlistID: "playlist123",
			artistID:   "artist123",
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
			mockCheckResponse: []bool{true, true},
			expectedResult: &server.DuplicateResult{
				HasDuplicates: true,
				DuplicateTracks: []server.Track{
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
				ArtistName: "Test Artist",
			},
		},
		{
			name:            "error getting artist tracks",
			playlistID:      "playlist123",
			artistID:        "artist123",
			mockTracksError: errors.New("failed to get tracks"),
			expectedError:   "failed to get artist top tracks: failed to get tracks",
		},
		{
			name:       "error checking tracks in playlist",
			playlistID: "playlist123",
			artistID:   "artist123",
			mockTracks: []server.Track{
				{
					ID:   "track1",
					Name: "Song 1",
					Artists: []server.Artist{
						{ID: "artist123", Name: "Test Artist"},
					},
				},
			},
			mockCheckError: errors.New("spotify api error"),
			expectedError:  "failed to check tracks in playlist: spotify api error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSpotify := &MockSpotifyService{
				tracks:       tt.mockTracks,
				tracksError:  tt.mockTracksError,
				checkResults: tt.mockCheckResponse,
				checkError:   tt.mockCheckError,
			}

			// Set expected track IDs for validation if we have tracks
			if len(tt.mockTracks) > 0 && tt.mockTracksError == nil {
				trackIDs := make([]string, len(tt.mockTracks))
				for i, track := range tt.mockTracks {
					trackIDs[i] = track.ID
				}
				mockSpotify.expectedTrackIDs = trackIDs
			}

			logger := log.New()
			logger.SetLevel(log.FatalLevel) // Suppress logs during testing

			service := NewDuplicateService(mockSpotify, logger)

			result, err := service.CheckArtistInPlaylist(tt.playlistID, tt.artistID)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.expectedError) {
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

				if result.HasDuplicates != tt.expectedResult.HasDuplicates {
					t.Errorf("Expected HasDuplicates %v, got %v", tt.expectedResult.HasDuplicates, result.HasDuplicates)
				}

				if result.ArtistName != tt.expectedResult.ArtistName {
					t.Errorf("Expected ArtistName '%s', got '%s'", tt.expectedResult.ArtistName, result.ArtistName)
				}

				// For duplicate cases, check that the message contains expected elements
				if result.HasDuplicates {
					if !strings.Contains(result.Message, tt.expectedResult.ArtistName) {
						t.Errorf("Expected message to contain artist name '%s', got '%s'", tt.expectedResult.ArtistName, result.Message)
					}
					if !strings.Contains(result.Message, "already has") {
						t.Errorf("Expected message to contain 'already has', got '%s'", result.Message)
					}
					if !strings.Contains(result.Message, "Add Anyway") {
						t.Errorf("Expected message to contain 'Add Anyway', got '%s'", result.Message)
					}
				} else if result.Message != tt.expectedResult.Message {
					t.Errorf("Expected message '%s', got '%s'", tt.expectedResult.Message, result.Message)
				}

				if len(result.DuplicateTracks) != len(tt.expectedResult.DuplicateTracks) {
					t.Errorf("Expected %d duplicate tracks, got %d", len(tt.expectedResult.DuplicateTracks), len(result.DuplicateTracks))
				}

				// Verify LastAdded is set to a recent time when there are duplicates
				if result.HasDuplicates {
					if time.Since(result.LastAdded) > time.Minute {
						t.Error("LastAdded timestamp should be recent")
					}
				}
			}
		})
	}
}

func TestDuplicateService_CheckArtistInPlaylist_EdgeCases(t *testing.T) {
	t.Run("artist with no artist info in tracks", func(t *testing.T) {
		mockSpotify := &MockSpotifyService{
			tracks: []server.Track{
				{ID: "track1", Name: "Song 1", Artists: []server.Artist{}},
			},
			tracksError:      nil,
			checkResults:     []bool{false},
			checkError:       nil,
			expectedTrackIDs: []string{"track1"},
		}

		logger := log.New()
		logger.SetLevel(log.FatalLevel)

		service := NewDuplicateService(mockSpotify, logger)

		result, err := service.CheckArtistInPlaylist("playlist123", "artist123")

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("Expected result but got nil")
		}
		if result.ArtistName != "" {
			t.Errorf("Expected empty artist name, got '%s'", result.ArtistName)
		}
		if result.HasDuplicates {
			t.Error("Expected no duplicates")
		}
	})
}
