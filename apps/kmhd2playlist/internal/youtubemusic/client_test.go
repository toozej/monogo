package youtubemusic

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func writeAuthFile(t *testing.T, headers map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "youtubemusic-headers.json")
	data, err := json.Marshal(headers)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(path, data, 0600))
	return path
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

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

func TestNewClient_WithAuthFile(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	authFile := writeAuthFile(t, map[string]string{
		"Cookie":                   testValidCookie,
		"X-Goog-AuthUser":          "2",
		"X-Goog-Visitor-Id":        "visitor-id",
		"X-YouTube-Client-Version": "1.20260721.01.00",
	})

	client, err := NewClient(config.YouTubeMusicConfig{
		AuthFilePath:  authFile,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}, logger)

	assert.NoError(t, err)
	assert.Equal(t, "auth_file", client.authSource)
	assert.Equal(t, testValidCookie, client.cookie)
	assert.Equal(t, "2", client.authHeaders.Get("X-Goog-AuthUser"))
	assert.Equal(t, "visitor-id", client.authHeaders.Get("X-Goog-Visitor-Id"))
	assert.Equal(t, "1.20260721.01.00", client.clientVersion)
}

func TestNewClient_AuthFileRequiresCookieAndAuthUser(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name    string
		headers map[string]string
		wantErr string
	}{
		{
			name:    "missing cookie",
			headers: map[string]string{"X-Goog-AuthUser": "0"},
			wantErr: "does not contain a Cookie header",
		},
		{
			name:    "missing account index",
			headers: map[string]string{"Cookie": testValidCookie},
			wantErr: "does not contain an X-Goog-AuthUser header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(config.YouTubeMusicConfig{
				AuthFilePath:  writeAuthFile(t, tt.headers),
				TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
			}, logger)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestLoadAuthHeadersExcludesTransportHeaders(t *testing.T) {
	authFile := writeAuthFile(t, map[string]string{
		"Cookie":            testValidCookie,
		"X-Goog-AuthUser":   "0",
		"X-Goog-Visitor-Id": "visitor-id",
		"Host":              "music.youtube.com",
		"Content-Length":    "999",
		"Accept-Encoding":   "gzip, deflate, br",
		"Sec-Fetch-Site":    "same-origin",
	})

	headers, err := loadAuthHeaders(authFile)
	assert.NoError(t, err)
	assert.Equal(t, testValidCookie, headers.Get("Cookie"))
	assert.Equal(t, "visitor-id", headers.Get("X-Goog-Visitor-Id"))
	assert.Empty(t, headers.Get("Host"))
	assert.Empty(t, headers.Get("Content-Length"))
	assert.Empty(t, headers.Get("Accept-Encoding"))
	assert.Empty(t, headers.Get("Sec-Fetch-Site"))
}

func TestReadAuthFileRejectsSymlinkOutsideAuthDirectory(t *testing.T) {
	authDir := t.TempDir()
	externalFile := writeAuthFile(t, map[string]string{
		"Cookie":          testValidCookie,
		"X-Goog-AuthUser": "0",
	})
	symlinkPath := filepath.Join(authDir, "youtubemusic-headers.json")
	assert.NoError(t, os.Symlink(externalFile, symlinkPath))

	_, err := readAuthFile(symlinkPath)
	assert.Error(t, err)
}

func TestClient_CreatePlaylist_UsesBrowserAuthHeaders(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	authFile := writeAuthFile(t, map[string]string{
		"Cookie":                   testValidCookie,
		"X-Goog-AuthUser":          "2",
		"X-Goog-Visitor-Id":        "visitor-id",
		"X-YouTube-Client-Name":    "67",
		"X-YouTube-Client-Version": "1.20260721.01.00",
	})
	client, err := NewClient(config.YouTubeMusicConfig{
		AuthFilePath:  authFile,
		TokenFilePath: filepath.Join(t.TempDir(), "test_token.json"),
	}, logger)
	assert.NoError(t, err)

	client.httpClient = &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, "2", request.Header.Get("X-Goog-AuthUser"))
		assert.Equal(t, "visitor-id", request.Header.Get("X-Goog-Visitor-Id"))
		assert.Equal(t, "67", request.Header.Get("X-YouTube-Client-Name"))
		assert.Equal(t, "1.20260721.01.00", request.Header.Get("X-YouTube-Client-Version"))
		assert.NotEmpty(t, request.Header.Get("Authorization"))

		body, readErr := io.ReadAll(request.Body)
		assert.NoError(t, readErr)
		assert.Contains(t, string(body), `"clientVersion":"1.20260721.01.00"`)

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"playlistId":"playlist-id"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	playlist, err := client.CreatePlaylist("test playlist", "test description", false)
	assert.NoError(t, err)
	assert.Equal(t, "playlist-id", playlist.ID)
}

func TestAuthorizationHeaderUsesTimestampSAPISIDOrigin(t *testing.T) {
	client := &Client{
		cookie: testValidCookie,
		authHeaders: http.Header{
			"X-Origin": {origin},
		},
	}

	authHeader, err := client.authorizationHeader()
	assert.NoError(t, err)

	parts := strings.Fields(authHeader)
	if !assert.Len(t, parts, 2) {
		return
	}
	timestampAndHash := strings.SplitN(parts[1], "_", 2)
	if !assert.Len(t, timestampAndHash, 2) {
		return
	}
	timestamp, err := strconv.ParseInt(timestampAndHash[0], 10, 64)
	assert.NoError(t, err)

	payload := fmt.Sprintf("%d %s %s", timestamp, "test_cookie_value", origin)
	// nosemgrep: go.lang.security.audit.crypto.use_of_weak_crypto.use-of-sha1 -- test verifies the externally mandated SAPISIDHASH protocol; SHA-1 is not used as a signature or security primitive here.
	expectedHash := fmt.Sprintf("%x", sha1.Sum([]byte(payload))) // #nosec G401 -- validates YouTube's required SAPISIDHASH algorithm
	assert.Equal(t, "SAPISIDHASH", parts[0])
	assert.Equal(t, expectedHash, timestampAndHash[1])
}

func TestTokenFileWritable(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "writable directory",
			path: filepath.Join(t.TempDir(), "token.json"),
			want: true,
		},
		{
			name: "missing directory",
			path: filepath.Join(t.TempDir(), "missing", "token.json"),
			want: false,
		},
		{
			name: "empty path",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tokenFileWritable(tt.path))
		})
	}
}

func TestResponseErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "inner tube error response",
			body: `{"error":{"code":401,"message":"Request is missing valid authentication credentials.","status":"UNAUTHENTICATED"}}`,
			want: "Request is missing valid authentication credentials.",
		},
		{
			name: "malformed response",
			body: "not json",
			want: "",
		},
		{
			name: "missing error message",
			body: `{"error":{"code":401}}`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, responseErrorMessage([]byte(tt.body)))
		})
	}
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
	ctx := newContext("1.20260721.01.00")
	assert.Equal(t, clientName, ctx.Client.Name)
	assert.Equal(t, "1.20260721.01.00", ctx.Client.Version)
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
