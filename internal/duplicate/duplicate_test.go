package duplicate

import (
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/toozej/kmhd2spotify/internal/types"
)

// MockSpotifyService is a mock implementation of the SpotifyService interface
type MockSpotifyService struct {
	// For GetArtistTopTracks
	getArtistTopTracksFunc func(artistID string) ([]types.Track, error)

	// For CheckTracksInPlaylist
	checkTracksInPlaylistFunc func(playlistID string, trackIDs []string) ([]bool, error)

	// For other methods (not used in duplicate tests but needed for interface)
	searchArtistFunc        func(query string) (*types.Artist, error)
	getUserPlaylistsFunc    func(folderName string) ([]types.Playlist, error)
	addTracksToPlaylistFunc func(playlistID string, trackIDs []string) error
	createPlaylistFunc      func(name, description string, public bool) (*types.Playlist, error)
	getAuthURLFunc          func() string
	isAuthenticatedFunc     func() bool
	completeAuthFunc        func(code, state string) error
}

func (m *MockSpotifyService) SearchArtist(query string) (*types.Artist, error) {
	if m.searchArtistFunc != nil {
		return m.searchArtistFunc(query)
	}
	return nil, errors.New("not implemented")
}

func (m *MockSpotifyService) GetArtistTopTracks(artistID string) ([]types.Track, error) {
	if m.getArtistTopTracksFunc != nil {
		return m.getArtistTopTracksFunc(artistID)
	}
	return nil, errors.New("not implemented")
}

func (m *MockSpotifyService) GetUserPlaylists(folderName string) ([]types.Playlist, error) {
	if m.getUserPlaylistsFunc != nil {
		return m.getUserPlaylistsFunc(folderName)
	}
	return nil, errors.New("not implemented")
}

func (m *MockSpotifyService) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	if m.addTracksToPlaylistFunc != nil {
		return m.addTracksToPlaylistFunc(playlistID, trackIDs)
	}
	return errors.New("not implemented")
}

func (m *MockSpotifyService) CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error) {
	if m.checkTracksInPlaylistFunc != nil {
		return m.checkTracksInPlaylistFunc(playlistID, trackIDs)
	}
	return nil, errors.New("not implemented")
}

func (m *MockSpotifyService) GetAuthURL() string {
	if m.getAuthURLFunc != nil {
		return m.getAuthURLFunc()
	}
	return ""
}

func (m *MockSpotifyService) IsAuthenticated() bool {
	if m.isAuthenticatedFunc != nil {
		return m.isAuthenticatedFunc()
	}
	return false
}

func (m *MockSpotifyService) CompleteAuth(code, state string) error {
	if m.completeAuthFunc != nil {
		return m.completeAuthFunc(code, state)
	}
	return errors.New("not implemented")
}

func (m *MockSpotifyService) CreatePlaylist(name, description string, public bool) (*types.Playlist, error) {
	if m.createPlaylistFunc != nil {
		return m.createPlaylistFunc(name, description, public)
	}
	return nil, errors.New("not implemented")
}

func TestNewDuplicateService(t *testing.T) {
	logger := logrus.New()
	mockSpotify := &MockSpotifyService{}

	service := NewDuplicateService(mockSpotify, logger)

	assert.NotNil(t, service)
	assert.Equal(t, mockSpotify, service.spotify)
	assert.Equal(t, logger, service.logger)
}

func TestDuplicateService_CheckDuplicates(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce log noise in tests

	tests := []struct {
		name          string
		playlistID    string
		tracks        []types.Track
		mockExists    []bool
		mockError     error
		expectedError bool
		checkResult   func(*testing.T, *types.DuplicateResult)
	}{
		{
			name:          "no tracks provided",
			playlistID:    "playlist123",
			tracks:        []types.Track{},
			mockExists:    []bool{},
			mockError:     nil,
			expectedError: false,
			checkResult: func(t *testing.T, result *types.DuplicateResult) {
				assert.False(t, result.HasDuplicates)
				assert.Equal(t, "No tracks to check", result.Message)
				// For no tracks case, LastAdded should remain zero (as per implementation)
				assert.True(t, result.LastAdded.IsZero())
				assert.Empty(t, result.DuplicateTracks)
			},
		},
		{
			name:       "no duplicates found",
			playlistID: "playlist123",
			tracks: []types.Track{
				{ID: "track1", Name: "Song 1", Album: types.Album{ID: "album1", Name: "Album 1", Type: "album"}},
				{ID: "track2", Name: "Song 2", Album: types.Album{ID: "album2", Name: "Album 2", Type: "album"}},
			},
			mockExists:    []bool{false, false},
			mockError:     nil,
			expectedError: false,
			checkResult: func(t *testing.T, result *types.DuplicateResult) {
				assert.False(t, result.HasDuplicates)
				assert.Equal(t, "No duplicate tracks found", result.Message)
				assert.Empty(t, result.DuplicateTracks)
				assert.False(t, result.LastAdded.IsZero())
				assert.WithinDuration(t, time.Now(), result.LastAdded, 1*time.Second)
			},
		},
		{
			name:       "some duplicates found",
			playlistID: "playlist123",
			tracks: []types.Track{
				{ID: "track1", Name: "Song 1", Album: types.Album{ID: "album1", Name: "Album 1", Type: "album"}},
				{ID: "track2", Name: "Song 2", Album: types.Album{ID: "album2", Name: "Album 2", Type: "album"}},
				{ID: "track3", Name: "Song 3", Album: types.Album{ID: "album3", Name: "Album 3", Type: "album"}},
			},
			mockExists:    []bool{true, false, true},
			mockError:     nil,
			expectedError: false,
			checkResult: func(t *testing.T, result *types.DuplicateResult) {
				assert.True(t, result.HasDuplicates)
				assert.Equal(t, "Found 2 duplicate track(s): Song 1, Song 3", result.Message)
				assert.Len(t, result.DuplicateTracks, 2)
				assert.Equal(t, "track1", result.DuplicateTracks[0].ID)
				assert.Equal(t, "Song 1", result.DuplicateTracks[0].Name)
				assert.Equal(t, "track3", result.DuplicateTracks[1].ID)
				assert.Equal(t, "Song 3", result.DuplicateTracks[1].Name)
				assert.False(t, result.LastAdded.IsZero())
				assert.WithinDuration(t, time.Now(), result.LastAdded, 1*time.Second)
			},
		},
		{
			name:       "all duplicates found",
			playlistID: "playlist123",
			tracks: []types.Track{
				{ID: "track1", Name: "Song 1", Album: types.Album{ID: "album1", Name: "Album 1", Type: "album"}},
				{ID: "track2", Name: "Song 2", Album: types.Album{ID: "album2", Name: "Album 2", Type: "album"}},
			},
			mockExists:    []bool{true, true},
			mockError:     nil,
			expectedError: false,
			checkResult: func(t *testing.T, result *types.DuplicateResult) {
				assert.True(t, result.HasDuplicates)
				assert.Equal(t, "Found 2 duplicate track(s): Song 1, Song 2", result.Message)
				assert.Len(t, result.DuplicateTracks, 2)
				assert.Equal(t, "track1", result.DuplicateTracks[0].ID)
				assert.Equal(t, "Song 1", result.DuplicateTracks[0].Name)
				assert.Equal(t, "track2", result.DuplicateTracks[1].ID)
				assert.Equal(t, "Song 2", result.DuplicateTracks[1].Name)
				assert.False(t, result.LastAdded.IsZero())
				assert.WithinDuration(t, time.Now(), result.LastAdded, 1*time.Second)
			},
		},
		{
			name:       "spotify api error",
			playlistID: "playlist123",
			tracks: []types.Track{
				{ID: "track1", Name: "Song 1", Album: types.Album{ID: "album1", Name: "Album 1", Type: "album"}},
			},
			mockExists:    []bool{},
			mockError:     errors.New("spotify API error"),
			expectedError: true,
			checkResult:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSpotify := &MockSpotifyService{
				checkTracksInPlaylistFunc: func(playlistID string, trackIDs []string) ([]bool, error) {
					return tt.mockExists, tt.mockError
				},
			}

			service := NewDuplicateService(mockSpotify, logger)

			result, err := service.CheckDuplicates(tt.playlistID, tt.tracks)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				tt.checkResult(t, result)
			}
		})
	}
}

func TestDuplicateService_CheckArtistInPlaylist(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce log noise in tests

	artistName := "Test Artist"

	tests := []struct {
		name            string
		playlistID      string
		artistID        string
		mockTracks      []types.Track
		mockTracksError error
		mockExists      []bool
		mockExistsError error
		expectedError   bool
		checkResult     func(*testing.T, *types.DuplicateResult)
	}{
		{
			name:            "artist has no tracks",
			playlistID:      "playlist123",
			artistID:        "artist123",
			mockTracks:      []types.Track{},
			mockTracksError: nil,
			expectedError:   false,
			checkResult: func(t *testing.T, result *types.DuplicateResult) {
				assert.False(t, result.HasDuplicates)
				assert.Equal(t, "Artist has no tracks", result.Message)
				assert.Equal(t, "", result.ArtistName) // No artist name when no tracks
				assert.Empty(t, result.DuplicateTracks)
				// LastAdded should remain zero when no tracks (as per implementation)
				assert.True(t, result.LastAdded.IsZero())
			},
		},
		{
			name:       "artist tracks with no duplicates",
			playlistID: "playlist123",
			artistID:   "artist123",
			mockTracks: []types.Track{
				{ID: "track1", Name: "Song 1", Artists: []types.Artist{{Name: artistName}}, Album: types.Album{ID: "album1", Name: "Album 1", Type: "album"}},
				{ID: "track2", Name: "Song 2", Artists: []types.Artist{{Name: artistName}}, Album: types.Album{ID: "album2", Name: "Album 2", Type: "album"}},
			},
			mockTracksError: nil,
			mockExists:      []bool{false, false},
			mockExistsError: nil,
			expectedError:   false,
			checkResult: func(t *testing.T, result *types.DuplicateResult) {
				assert.False(t, result.HasDuplicates)
				assert.Contains(t, result.Message, "tracks not found in playlist, safe to add")
				assert.Equal(t, artistName, result.ArtistName)
				assert.Empty(t, result.DuplicateTracks)
				assert.False(t, result.LastAdded.IsZero())
				assert.WithinDuration(t, time.Now(), result.LastAdded, 1*time.Second)
			},
		},
		{
			name:       "artist tracks with duplicates",
			playlistID: "playlist123",
			artistID:   "artist123",
			mockTracks: []types.Track{
				{ID: "track1", Name: "Song 1", Artists: []types.Artist{{Name: artistName}}, Album: types.Album{ID: "album1", Name: "Album 1", Type: "album"}},
				{ID: "track2", Name: "Song 2", Artists: []types.Artist{{Name: artistName}}, Album: types.Album{ID: "album2", Name: "Album 2", Type: "album"}},
			},
			mockTracksError: nil,
			mockExists:      []bool{true, false},
			mockExistsError: nil,
			expectedError:   false,
			checkResult: func(t *testing.T, result *types.DuplicateResult) {
				assert.True(t, result.HasDuplicates)
				assert.Contains(t, result.Message, "already has")
				assert.Equal(t, artistName, result.ArtistName)
				assert.Len(t, result.DuplicateTracks, 1)
				assert.Equal(t, "track1", result.DuplicateTracks[0].ID)
				assert.False(t, result.LastAdded.IsZero())
				assert.WithinDuration(t, time.Now(), result.LastAdded, 1*time.Second)
			},
		},
		{
			name:            "get artist tracks error",
			playlistID:      "playlist123",
			artistID:        "artist123",
			mockTracks:      nil,
			mockTracksError: errors.New("failed to get artist tracks"),
			expectedError:   true,
			checkResult:     nil,
		},
		{
			name:       "check tracks in playlist error",
			playlistID: "playlist123",
			artistID:   "artist123",
			mockTracks: []types.Track{
				{ID: "track1", Name: "Song 1", Artists: []types.Artist{{Name: artistName}}, Album: types.Album{ID: "album1", Name: "Album 1", Type: "album"}},
			},
			mockTracksError: nil,
			mockExists:      nil,
			mockExistsError: errors.New("failed to check tracks"),
			expectedError:   true,
			checkResult:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSpotify := &MockSpotifyService{
				getArtistTopTracksFunc: func(artistID string) ([]types.Track, error) {
					return tt.mockTracks, tt.mockTracksError
				},
				checkTracksInPlaylistFunc: func(playlistID string, trackIDs []string) ([]bool, error) {
					return tt.mockExists, tt.mockExistsError
				},
			}

			service := NewDuplicateService(mockSpotify, logger)

			result, err := service.CheckArtistInPlaylist(tt.playlistID, tt.artistID)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				tt.checkResult(t, result)
			}
		})
	}
}

func TestDuplicateService_CheckArtistInPlaylist_NoArtistName(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockSpotify := &MockSpotifyService{
		getArtistTopTracksFunc: func(artistID string) ([]types.Track, error) {
			return []types.Track{
				{ID: "track1", Name: "Song 1", Album: types.Album{ID: "album1", Name: "Album 1", Type: "album"}}, // No artist name in track
			}, nil
		},
		checkTracksInPlaylistFunc: func(playlistID string, trackIDs []string) ([]bool, error) {
			return []bool{false}, nil
		},
	}

	service := NewDuplicateService(mockSpotify, logger)

	result, err := service.CheckArtistInPlaylist("playlist123", "artist123")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "", result.ArtistName) // Should be empty when no artist name in tracks
	assert.Contains(t, result.Message, "tracks not found in playlist, safe to add")
	assert.False(t, result.LastAdded.IsZero())
	assert.WithinDuration(t, time.Now(), result.LastAdded, 1*time.Second)
}
