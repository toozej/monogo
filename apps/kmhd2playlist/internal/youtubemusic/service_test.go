package youtubemusic

import (
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/config"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/types"
)

var _ types.MusicService = (*Service)(nil)

func TestNewService_WithValidConfig(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        "test_cookie",
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	service := NewService(cfg, logger)
	assert.NotNil(t, service)
}

func TestNewService_WithEmptyCookie(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := config.YouTubeMusicConfig{
		Cookie:        "",
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}

	service := NewService(cfg, logger)
	assert.NotNil(t, service)
	assert.Nil(t, service.client)
	assert.False(t, service.IsAuthenticated())
}

func TestService_IsAuthenticated_WithNilClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := &Service{client: nil, logger: logger}
	assert.False(t, service.IsAuthenticated())
}

func TestService_GetAuthURL_WithNilClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := &Service{client: nil, logger: logger}
	assert.Equal(t, "", service.GetAuthURL())
}

func TestService_CompleteAuth_WithNilClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := &Service{client: nil, logger: logger}
	err := service.CompleteAuth("code", "state")
	assert.Error(t, err)
	assert.Equal(t, "youtube music client not available", err.Error())
}

func TestService_SearchArtist_WithNilClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := &Service{client: nil, logger: logger}
	artist, err := service.SearchArtist("test query")
	assert.Error(t, err)
	assert.Equal(t, "youtube music client not available", err.Error())
	assert.Nil(t, artist)
}

func TestService_GetArtistTopTracks_WithNilClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := &Service{client: nil, logger: logger}
	tracks, err := service.GetArtistTopTracks("test-artist-id")
	assert.Error(t, err)
	assert.Equal(t, "youtube music client not available", err.Error())
	assert.Nil(t, tracks)
}

func TestService_GetUserPlaylists_WithNilClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := &Service{client: nil, logger: logger}
	playlists, err := service.GetUserPlaylists("test-folder")
	assert.Error(t, err)
	assert.Equal(t, "youtube music client not available", err.Error())
	assert.Nil(t, playlists)
}

func TestService_AddTracksToPlaylist_WithNilClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := &Service{client: nil, logger: logger}
	err := service.AddTracksToPlaylist("test-playlist", []string{"track1"})
	assert.Error(t, err)
	assert.Equal(t, "youtube music client not available", err.Error())
}

func TestService_CheckTracksInPlaylist_WithNilClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := &Service{client: nil, logger: logger}
	results, err := service.CheckTracksInPlaylist("test-playlist", []string{"track1"})
	assert.Error(t, err)
	assert.Equal(t, "youtube music client not available", err.Error())
	assert.Nil(t, results)
}

func TestService_CreatePlaylist_WithNilClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := &Service{client: nil, logger: logger}
	playlist, err := service.CreatePlaylist("test-playlist", "test-desc", false)
	assert.Error(t, err)
	assert.Equal(t, "youtube music client not available", err.Error())
	assert.Nil(t, playlist)
}
