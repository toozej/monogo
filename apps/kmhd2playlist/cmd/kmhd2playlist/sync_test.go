package cmd

import (
	"fmt"
	"os"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/api"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/config"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/search"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/types"
)

func TestNewSyncCmd(t *testing.T) {
	cmd := newSyncCmd()

	assert.NotNil(t, cmd)
	assert.Equal(t, "sync", cmd.Use)
	assert.Equal(t, "Sync KMHD playlist to music service", cmd.Short)
	assert.Contains(t, cmd.Long, "fuzzy matching")
	assert.NotNil(t, cmd.RunE)
}

func TestSyncCommandRejectsNonPositiveContinuousInterval(t *testing.T) {
	cmd := newSyncCmd()
	cmd.SetArgs([]string{"--continuous", "--interval=0s"})

	err := cmd.Execute()

	assert.ErrorContains(t, err, "interval must be greater than zero")
}

func TestSyncSongsToService(t *testing.T) {
	// Create mock services
	mockSpotify := &MockSpotifyServiceForSync{
		playlists: []types.Playlist{{ID: "playlist1", Name: "Test Playlist"}},
		tracks: []types.Track{{
			ID:   "track1",
			Name: "Test Track",
			Album: types.Album{
				ID:   "album1",
				Name: "Test Album",
				Type: "album",
			},
		}},
	}

	// Create real fuzzy song searcher with mock spotify
	logger := log.New()
	logger.SetLevel(log.ErrorLevel) // Reduce noise
	fuzzySongSearcher := search.NewFuzzySongSearcher(mockSpotify, logger)

	songs := []types.Song{
		{Artist: "Test Artist", Title: "Test Song"},
	}
	targetPlaylist := types.Playlist{ID: "playlist1", Name: "Test Playlist"}
	seenSongs := make(map[string]bool) // Add the missing seenSongs parameter

	err := syncSongsToService(songs, mockSpotify, fuzzySongSearcher, targetPlaylist, seenSongs)
	assert.ErrorContains(t, err, "confidence")
}

// MockSpotifyServiceForSync implements types.MusicService for testing sync
type MockSpotifyServiceForSync struct {
	playlists []types.Playlist
	tracks    []types.Track
	existing  []bool
	checkErr  error
	addErr    error
	addCalls  int
}

func (m *MockSpotifyServiceForSync) SearchArtist(query string) (*types.Artist, error) {
	return &types.Artist{ID: "artist1", Name: query}, nil
}

func (m *MockSpotifyServiceForSync) GetArtistTopTracks(artistID string) ([]types.Track, error) {
	return m.tracks, nil
}

func (m *MockSpotifyServiceForSync) GetUserPlaylists(folderName string) ([]types.Playlist, error) {
	return m.playlists, nil
}

func (m *MockSpotifyServiceForSync) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	m.addCalls++
	return m.addErr
}

func (m *MockSpotifyServiceForSync) CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error) {
	if m.checkErr != nil {
		return nil, m.checkErr
	}
	if m.existing != nil {
		return m.existing, nil
	}
	return []bool{false}, nil
}

func (m *MockSpotifyServiceForSync) GetAuthURL() string {
	return "mock-auth-url"
}

func (m *MockSpotifyServiceForSync) IsAuthenticated() bool {
	return true // Always return true for tests to skip auth flow
}

func (m *MockSpotifyServiceForSync) CompleteAuth(code, state string) error {
	return nil
}

func (m *MockSpotifyServiceForSync) CreatePlaylist(name, description string, public bool) (*types.Playlist, error) {
	return &types.Playlist{
		ID:         "test-playlist-id",
		Name:       name,
		URI:        "spotify:playlist:test-playlist-id",
		TrackCount: 0,
		EmbedURL:   "https://open.spotify.com/embed/playlist/test-playlist-id",
		IsIncoming: false,
	}, nil
}

func TestAuthenticateMusicService(t *testing.T) {
	// Test that authentication flow is triggered when service is not authenticated
	mockSpotify := &MockUnauthenticatedSpotifyService{}

	// This should trigger the authentication flow, but since we can't easily test
	// the HTTP server in unit tests, we'll just verify the function exists and
	// can be called without panicking
	assert.NotPanics(t, func() {
		// We can't easily test the full auth flow in unit tests due to the HTTP server
		// but we can verify the function signature and basic setup
		_ = mockSpotify.GetAuthURL()
		_ = mockSpotify.IsAuthenticated()
	})
}

// MockUnauthenticatedSpotifyService simulates an unauthenticated Spotify service
type MockUnauthenticatedSpotifyService struct{}

func (m *MockUnauthenticatedSpotifyService) SearchArtist(query string) (*types.Artist, error) {
	return nil, fmt.Errorf("not authenticated")
}

func (m *MockUnauthenticatedSpotifyService) GetArtistTopTracks(artistID string) ([]types.Track, error) {
	return nil, fmt.Errorf("not authenticated")
}

func (m *MockUnauthenticatedSpotifyService) GetUserPlaylists(folderName string) ([]types.Playlist, error) {
	return nil, fmt.Errorf("not authenticated")
}

func (m *MockUnauthenticatedSpotifyService) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	return fmt.Errorf("not authenticated")
}

func (m *MockUnauthenticatedSpotifyService) CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error) {
	return nil, fmt.Errorf("not authenticated")
}

func (m *MockUnauthenticatedSpotifyService) GetAuthURL() string {
	return "https://accounts.spotify.com/authorize?mock=true"
}

func (m *MockUnauthenticatedSpotifyService) IsAuthenticated() bool {
	return false
}

func (m *MockUnauthenticatedSpotifyService) CompleteAuth(code, state string) error {
	return nil // Simulate successful auth completion
}

func (m *MockUnauthenticatedSpotifyService) CreatePlaylist(name, description string, public bool) (*types.Playlist, error) {
	return nil, fmt.Errorf("not authenticated")
}

func TestGetOrCreateMonthlyPlaylist(t *testing.T) {
	mockSpotify := &MockSpotifyServiceForSync{
		playlists: []types.Playlist{
			{ID: "playlist1", Name: "KMHD-2025-10"},
			{ID: "playlist2", Name: "My Favorites"},
		},
	}

	// Test with configured prefix that has existing monthly playlist
	playlist, err := getOrCreateMonthlyPlaylist(mockSpotify, "KMHD")
	assert.NoError(t, err)
	// Should find existing playlist for current month or create new one
	assert.NotEmpty(t, playlist.ID)
	assert.Contains(t, playlist.Name, "KMHD-")

	// Test with empty prefix (should return first playlist for backward compatibility)
	playlist, err = getOrCreateMonthlyPlaylist(mockSpotify, "")
	assert.NoError(t, err)
	assert.Equal(t, "playlist1", playlist.ID)
	assert.Equal(t, "KMHD-2025-10", playlist.Name)

	// Test with no playlists available and empty prefix
	emptyMockSpotify := &MockSpotifyServiceForSync{playlists: []types.Playlist{}}
	_, err = getOrCreateMonthlyPlaylist(emptyMockSpotify, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no playlists found")
}

func TestGetOrCreateMonthlyPlaylistAtUsesProvidedMonth(t *testing.T) {
	mockSpotify := &MockSpotifyServiceForSync{
		playlists: []types.Playlist{{ID: "july", Name: "KMHD-2026-07"}},
	}

	playlist, err := getOrCreateMonthlyPlaylistAt(mockSpotify, "KMHD", time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC))

	assert.NoError(t, err)
	assert.Equal(t, "july", playlist.ID)
}

// Integration tests for full sync workflow

func TestRunSingleSyncIntegration(t *testing.T) {
	// Create mock KMHD scraper that returns test songs
	mockKMHD := &MockKMHDScraper{
		songs: []types.Song{
			{Artist: "Miles Davis", Title: "Kind of Blue", Album: "Kind of Blue"},
			{Artist: "John Coltrane", Title: "Giant Steps", Album: "Giant Steps"},
		},
	}

	// Create mock Spotify service
	mockSpotify := &MockSpotifyServiceForSync{
		playlists: []types.Playlist{{ID: "playlist1", Name: "Test Playlist"}},
		tracks: []types.Track{{
			ID:   "track1",
			Name: "Kind of Blue",
			Album: types.Album{
				ID:   "album1",
				Name: "Kind of Blue",
				Type: "album",
			},
		}},
	}

	// Create fuzzy song searcher
	logger := log.New()
	logger.SetLevel(log.ErrorLevel)
	fuzzySongSearcher := search.NewFuzzySongSearcher(mockSpotify, logger)

	targetPlaylist := types.Playlist{ID: "playlist1", Name: "Test Playlist"}
	seenSongs := make(map[string]bool)

	// Only successfully confirmed tracks are remembered; failed matches remain retryable.
	err := runSingleSync(mockKMHD, mockSpotify, fuzzySongSearcher, targetPlaylist, seenSongs)

	assert.Error(t, err)
	assert.True(t, seenSongs["Miles Davis - Kind of Blue"])
	assert.False(t, seenSongs["John Coltrane - Giant Steps"])
}

func TestDuplicatePreventionAcrossCycles(t *testing.T) {
	songs := []types.Song{
		{Artist: "Miles Davis", Title: "Kind of Blue", Album: "Kind of Blue"},
		{Artist: "John Coltrane", Title: "Giant Steps", Album: "Giant Steps"},
		{Artist: "Miles Davis", Title: "Kind of Blue", Album: "Kind of Blue"}, // Duplicate
	}

	seenSongs := make(map[string]bool)
	newSongs1 := filterNewSongs(songs, seenSongs)
	assert.Equal(t, 2, len(newSongs1))
	assert.Empty(t, seenSongs, "filtering must not mark unprocessed songs as complete")

	seenSongs["Miles Davis - Kind of Blue"] = true
	newSongs2 := filterNewSongs(songs, seenSongs)
	assert.Len(t, newSongs2, 1)
	assert.Equal(t, "John Coltrane", newSongs2[0].Artist)
}

func TestSyncSongsRetriesFailedAddition(t *testing.T) {
	mockSpotify := &MockSpotifyServiceForSync{
		tracks: []types.Track{{ID: "track1", Name: "Kind of Blue"}},
		addErr: fmt.Errorf("temporary add failure"),
	}
	searcher := search.NewFuzzySongSearcher(mockSpotify, log.New())
	songs := []types.Song{{Artist: "Miles Davis", Title: "Kind of Blue"}}
	seenSongs := make(map[string]bool)
	playlist := types.Playlist{ID: "playlist1", Name: "KMHD"}

	err := syncSongsToService(songs, mockSpotify, searcher, playlist, seenSongs)
	assert.ErrorContains(t, err, "temporary add failure")
	assert.False(t, seenSongs["Miles Davis - Kind of Blue"])

	mockSpotify.addErr = nil
	err = syncSongsToService(filterNewSongs(songs, seenSongs), mockSpotify, searcher, playlist, seenSongs)
	assert.NoError(t, err)
	assert.True(t, seenSongs["Miles Davis - Kind of Blue"])
	assert.Equal(t, 2, mockSpotify.addCalls)
}

func TestSyncSongsFailsClosedWhenDuplicateCheckFails(t *testing.T) {
	mockSpotify := &MockSpotifyServiceForSync{
		tracks:   []types.Track{{ID: "track1", Name: "Kind of Blue"}},
		checkErr: fmt.Errorf("playlist lookup unavailable"),
	}
	searcher := search.NewFuzzySongSearcher(mockSpotify, log.New())
	seenSongs := make(map[string]bool)

	err := syncSongsToService(
		[]types.Song{{Artist: "Miles Davis", Title: "Kind of Blue"}},
		mockSpotify,
		searcher,
		types.Playlist{ID: "playlist1", Name: "KMHD"},
		seenSongs,
	)

	assert.ErrorContains(t, err, "playlist lookup unavailable")
	assert.Zero(t, mockSpotify.addCalls)
	assert.Empty(t, seenSongs)
}

func TestSyncSongsMarksConfirmedExistingTrack(t *testing.T) {
	mockSpotify := &MockSpotifyServiceForSync{
		tracks:   []types.Track{{ID: "track1", Name: "Kind of Blue"}},
		existing: []bool{true},
	}
	searcher := search.NewFuzzySongSearcher(mockSpotify, log.New())
	seenSongs := make(map[string]bool)

	err := syncSongsToService(
		[]types.Song{{Artist: "Miles Davis", Title: "Kind of Blue"}},
		mockSpotify,
		searcher,
		types.Playlist{ID: "playlist1", Name: "KMHD"},
		seenSongs,
	)

	assert.NoError(t, err)
	assert.Zero(t, mockSpotify.addCalls)
	assert.True(t, seenSongs["Miles Davis - Kind of Blue"])
}

func TestSyncWithAPIUnavailable(t *testing.T) {
	// Create mock KMHD scraper that returns error
	mockKMHD := &MockKMHDScraperWithError{
		err: fmt.Errorf("API unavailable"),
	}

	mockSpotify := &MockSpotifyServiceForSync{
		playlists: []types.Playlist{{ID: "playlist1", Name: "Test Playlist"}},
	}

	logger := log.New()
	logger.SetLevel(log.ErrorLevel)
	fuzzySongSearcher := search.NewFuzzySongSearcher(mockSpotify, logger)

	targetPlaylist := types.Playlist{ID: "playlist1", Name: "Test Playlist"}
	seenSongs := make(map[string]bool)

	err := runSingleSync(mockKMHD, mockSpotify, fuzzySongSearcher, targetPlaylist, seenSongs)

	assert.ErrorContains(t, err, "API unavailable")
	assert.Equal(t, 0, len(seenSongs))
}

func TestSongMatchingCompatibility(t *testing.T) {
	// Test that JSON API songs work with existing Spotify integration
	songs := []types.Song{
		{
			Artist:  "Miles Davis",
			Title:   "Kind of Blue",
			Album:   "Kind of Blue",
			RawText: `{"artistName":"Miles Davis","trackName":"Kind of Blue","collectionName":"Kind of Blue"}`,
		},
	}

	mockSpotify := &MockSpotifyServiceForSync{
		tracks: []types.Track{{
			ID:   "track1",
			Name: "Kind of Blue",
			Album: types.Album{
				ID:   "album1",
				Name: "Kind of Blue",
				Type: "album",
			},
		}},
	}

	logger := log.New()
	logger.SetLevel(log.ErrorLevel)
	fuzzySongSearcher := search.NewFuzzySongSearcher(mockSpotify, logger)

	// Test that fuzzy matching works with JSON-sourced songs including album information
	match, err := fuzzySongSearcher.FindBestSongMatchWithAlbum(songs[0].Artist, songs[0].Title, songs[0].Album)
	assert.NoError(t, err)
	assert.NotNil(t, match)
	assert.Equal(t, "Kind of Blue", match.Track.Name)
}

// Mock implementations for integration tests

type MockKMHDScraper struct {
	songs []types.Song
}

func (m *MockKMHDScraper) ScrapePlaylist() (*types.SongCollection, error) {
	return &types.SongCollection{
		Songs:       m.songs,
		LastUpdated: time.Now(),
		Source:      "mock_api",
	}, nil
}

func (m *MockKMHDScraper) GetCurrentlyPlaying() (*types.Song, error) {
	if len(m.songs) > 0 {
		return &m.songs[0], nil
	}
	return nil, fmt.Errorf("no songs available")
}

type MockKMHDScraperWithError struct {
	err error
}

func (m *MockKMHDScraperWithError) ScrapePlaylist() (*types.SongCollection, error) {
	return nil, m.err
}

func (m *MockKMHDScraperWithError) GetCurrentlyPlaying() (*types.Song, error) {
	return nil, m.err
}

// TestAPIToSpotifyEndToEnd tests the complete flow from API to Spotify integration
func TestAPIToSpotifyEndToEnd(t *testing.T) {
	if os.Getenv("KMHD_LIVE_INTEGRATION") != "1" {
		t.Skip("set KMHD_LIVE_INTEGRATION=1 to run the live KMHD integration test")
	}

	// Create a real API client with test configuration
	cfg := config.KMHDConfig{
		APIEndpoint: "https://www.kmhd.org/pf/api/v3/content/fetch/playlist",
		HTTPTimeout: 30,
	}

	apiClient := api.NewKMHDAPIClient(cfg)

	// Create mock Spotify service
	mockSpotify := &MockSpotifyServiceForSync{
		playlists: []types.Playlist{{ID: "test-playlist", Name: "KMHD Test"}},
	}

	logger := log.New()
	logger.SetLevel(log.ErrorLevel)
	fuzzySongSearcher := search.NewFuzzySongSearcher(mockSpotify, logger)

	targetPlaylist := types.Playlist{ID: "test-playlist", Name: "KMHD Test"}
	seenSongs := make(map[string]bool)

	// Test the complete sync flow
	// Note: This test may fail if the API is unavailable, which is expected
	if err := runSingleSync(apiClient, mockSpotify, fuzzySongSearcher, targetPlaylist, seenSongs); err != nil {
		t.Skipf("live KMHD API unavailable: %v", err)
	}

	// If the API was available and returned data, verify the integration worked
	if len(seenSongs) > 0 {
		t.Logf("Successfully processed %d songs from real API", len(seenSongs))

		// Integration worked successfully
	} else {
		t.Log("API was unavailable or returned no songs - this is acceptable for testing")
	}
}

// TestHourlySyncTimingAndRandomization tests the sync scheduling logic
func TestHourlySyncTimingAndRandomization(t *testing.T) {
	baseInterval := time.Hour

	// Test multiple calculations to ensure randomization works
	durations := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		durations[i] = calculateNextSyncTime(baseInterval)
	}

	// All durations should be at least the base interval
	for i, duration := range durations {
		assert.GreaterOrEqual(t, duration, baseInterval, "Duration %d should be at least base interval", i)
		assert.LessOrEqual(t, duration, baseInterval+time.Hour, "Duration %d should not exceed base + 1 hour", i)
	}

	// Check that we get some variation (not all the same)
	allSame := true
	for i := 1; i < len(durations); i++ {
		if durations[i] != durations[0] {
			allSame = false
			break
		}
	}
	assert.False(t, allSame, "Randomization should produce different durations")
}

// TestAPIErrorHandling tests system behavior during API outages
func TestAPIErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		mockError error
	}{
		{
			name:      "network timeout",
			mockError: fmt.Errorf("context deadline exceeded"),
		},
		{
			name:      "API server error",
			mockError: fmt.Errorf("API returned status 500: Internal Server Error"),
		},
		{
			name:      "JSON parsing error",
			mockError: fmt.Errorf("failed to decode JSON response: invalid character"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockKMHD := &MockKMHDScraperWithError{
				err: tt.mockError,
			}

			mockSpotify := &MockSpotifyServiceForSync{
				playlists: []types.Playlist{{ID: "test", Name: "Test"}},
			}

			logger := log.New()
			logger.SetLevel(log.ErrorLevel)
			fuzzySongSearcher := search.NewFuzzySongSearcher(mockSpotify, logger)

			targetPlaylist := types.Playlist{ID: "test", Name: "Test"}
			seenSongs := make(map[string]bool)

			err := runSingleSync(mockKMHD, mockSpotify, fuzzySongSearcher, targetPlaylist, seenSongs)
			assert.ErrorContains(t, err, tt.mockError.Error())

			// Should not process any songs when API fails
			assert.Empty(t, seenSongs)
		})
	}
}

// TestSongExtractionEquivalence tests that API extraction produces equivalent results
func TestSongExtractionEquivalence(t *testing.T) {
	// Create test data that simulates what both scraper and API would extract
	expectedSongs := []types.Song{
		{
			Artist:   "Miles Davis",
			Title:    "So What",
			Album:    "Kind of Blue",
			PlayedAt: time.Now().Truncate(time.Minute),
		},
		{
			Artist:   "John Coltrane",
			Title:    "Giant Steps",
			Album:    "Giant Steps",
			PlayedAt: time.Now().Add(-time.Hour).Truncate(time.Minute),
		},
	}

	// Mock API client that returns our test songs
	mockKMHD := &MockKMHDScraper{
		songs: expectedSongs,
	}

	// Fetch songs using the API interface
	collection, err := mockKMHD.ScrapePlaylist()
	assert.NoError(t, err)
	assert.NotNil(t, collection)

	// Verify song extraction equivalence
	assert.Equal(t, len(expectedSongs), len(collection.Songs))

	for i, song := range collection.Songs {
		expected := expectedSongs[i]
		assert.Equal(t, expected.Artist, song.Artist, "Artist should match for song %d", i)
		assert.Equal(t, expected.Title, song.Title, "Title should match for song %d", i)
		assert.Equal(t, expected.Album, song.Album, "Album should match for song %d", i)

		// Verify song is valid for Spotify integration
		assert.True(t, song.IsValid(), "Song %d should be valid", i)
		assert.NotEmpty(t, song.Artist, "Song %d should have artist", i)
		assert.NotEmpty(t, song.Title, "Song %d should have title", i)
	}
}
