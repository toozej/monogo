package youtubemusic

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/config"
)

const (
	testValidCookie    = "SAPISID=test_cookie_value; HSID=foo"
	testValidCookieAlt = "SAPISID=new_cookie_value; HSID=foo"
)

func TestNewClient_WithEmptyCookie(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        "",
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "cookie is required")
}

func TestNewClient_WithValidCookie(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.True(t, client.IsAuthenticated())
}

func TestNewClient_WithCookieMissingSAPISID(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        "CONSENT=YES+1; HSID=foo",
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "SAPISID")
}

func TestClient_IsAuthenticated(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)
	assert.True(t, client.IsAuthenticated())

	// Test setting to unauthenticated
	client.mu.Lock()
	client.isAuthed = false
	client.mu.Unlock()
	assert.False(t, client.IsAuthenticated())
}

func TestClient_GetAuthURL(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)
	assert.Equal(t, "https://music.youtube.com", client.GetAuthURL())
}

func TestClient_CompleteAuth(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)

	// Set to unauthenticated first
	client.mu.Lock()
	client.isAuthed = false
	client.mu.Unlock()
	assert.False(t, client.IsAuthenticated())

	// Complete auth with a new cookie
	err = client.CompleteAuth(testValidCookieAlt, "")
	assert.NoError(t, err)
	assert.True(t, client.IsAuthenticated())
}

func TestClient_CompleteAuth_EmptyCookie(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)

	// Complete auth with empty cookie should fail
	err = client.CompleteAuth("", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cookie is required")
}

func TestClient_SearchArtist_NotAuthenticated(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)

	// Set to unauthenticated
	client.mu.Lock()
	client.isAuthed = false
	client.mu.Unlock()

	artist, err := client.SearchArtist("test query")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")
	assert.Nil(t, artist)
}

func TestClient_GetArtistTopTracks_NotAuthenticated(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)

	client.mu.Lock()
	client.isAuthed = false
	client.mu.Unlock()

	tracks, err := client.GetArtistTopTracks("test-artist-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")
	assert.Nil(t, tracks)
}

func TestClient_GetUserPlaylists_NotAuthenticated(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)

	client.mu.Lock()
	client.isAuthed = false
	client.mu.Unlock()

	playlists, err := client.GetUserPlaylists("test-folder")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")
	assert.Nil(t, playlists)
}

func TestClient_AddTracksToPlaylist_NotAuthenticated(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)

	client.mu.Lock()
	client.isAuthed = false
	client.mu.Unlock()

	err = client.AddTracksToPlaylist("test-playlist", []string{"track1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")
}

func TestClient_CheckTracksInPlaylist_NotAuthenticated(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)

	client.mu.Lock()
	client.isAuthed = false
	client.mu.Unlock()

	// Note: CheckTracksInPlaylist checks len(trackIDs) first, so provide non-empty IDs
	results, err := client.CheckTracksInPlaylist("test-playlist", []string{"track1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")
	assert.Nil(t, results)
}

func TestClient_CheckTracksInPlaylist_EmptyTrackIDs(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)

	// Empty track IDs should return empty results without error (even when unauthenticated)
	client.mu.Lock()
	client.isAuthed = false
	client.mu.Unlock()

	results, err := client.CheckTracksInPlaylist("test-playlist", []string{})
	assert.NoError(t, err)
	assert.Equal(t, []bool{}, results)
}

func TestClient_CreatePlaylist_NotAuthenticated(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)

	client.mu.Lock()
	client.isAuthed = false
	client.mu.Unlock()

	playlist, err := client.CreatePlaylist("test-playlist", "test-desc", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")
	assert.Nil(t, playlist)
}

func TestClient_AddTracksToPlaylist_EmptyTrackIDs(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)

	// Empty track IDs should return error even when authenticated
	err = client.AddTracksToPlaylist("test-playlist", []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tracks provided")
}

func TestClient_NewContext(t *testing.T) {
	ctx := newContext()
	assert.Equal(t, clientName, ctx.Client.Name)
	assert.Equal(t, clientVersion, ctx.Client.Version)
	assert.Equal(t, "en", ctx.Client.HL)
	assert.Equal(t, "US", ctx.Client.GL)
}

func TestClient_ConcurrentIsAuthenticated(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        testValidCookie,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	client, err := NewClient(cfg, logger)
	assert.NoError(t, err)

	// Test concurrent reads don't cause race conditions
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.IsAuthenticated()
		}()
	}
	wg.Wait()
}
