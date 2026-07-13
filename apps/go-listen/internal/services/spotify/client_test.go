package spotify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/go-listen/internal/config"
	"github.com/toozej/monogo/apps/go-listen/internal/types"
	spotifyapi "github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"
)

func TestIsInvalidGrantError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "direct RetrieveError with invalid_grant",
			err:  &oauth2.RetrieveError{ErrorCode: "invalid_grant"},
			want: true,
		},
		{
			name: "wrapped RetrieveError with invalid_grant",
			err:  fmt.Errorf("refresh failed: %w", &oauth2.RetrieveError{ErrorCode: "invalid_grant"}),
			want: true,
		},
		{
			name: "RetrieveError with different error code",
			err:  &oauth2.RetrieveError{ErrorCode: "invalid_client"},
			want: false,
		},
		{
			name: "plain error containing invalid_grant substring",
			err:  errors.New("oauth2: \"invalid_grant\" \"refresh token expired\""),
			want: true,
		},
		{
			name: "unrelated error",
			err:  errors.New("connection reset by peer"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInvalidGrantError(tt.err); got != tt.want {
				t.Errorf("isInvalidGrantError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// fakeAuth is a test authenticator that simulates the Spotify token endpoint
// returning an invalid_grant error during refresh.
type fakeAuth struct {
	refreshErr    error
	exchangeErr   error
	exchangeToken *oauth2.Token
	httpClient    *http.Client
}

func (f *fakeAuth) AuthURL(state string, opts ...oauth2.AuthCodeOption) string {
	return "http://localhost/auth?state=" + state
}

func (f *fakeAuth) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	if f.exchangeErr != nil {
		return nil, f.exchangeErr
	}
	if f.exchangeToken != nil {
		return f.exchangeToken, nil
	}
	return nil, errors.New("not implemented")
}

func (f *fakeAuth) RefreshToken(ctx context.Context, token *oauth2.Token) (*oauth2.Token, error) {
	return nil, f.refreshErr
}

func (f *fakeAuth) Client(ctx context.Context, token *oauth2.Token) *http.Client {
	if f.httpClient != nil {
		return f.httpClient
	}
	return &http.Client{}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestOAuthStateIsRandomAndSingleUse(t *testing.T) {
	client := &Client{
		logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		auth:   &fakeAuth{},
		states: make(map[string]time.Time),
	}
	firstURL := client.GetAuthURL()
	secondURL := client.GetAuthURL()
	if firstURL == secondURL || firstURL == "" || secondURL == "" {
		t.Fatalf("OAuth URLs must contain distinct state values: %q %q", firstURL, secondURL)
	}

	state := strings.TrimPrefix(firstURL, "http://localhost/auth?state=")
	if err := client.CompleteAuth("code", state); err == nil {
		t.Fatal("first callback should reach the fake exchange and fail")
	}
	if err := client.CompleteAuth("code", state); !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("replayed state error = %v", err)
	}
}

func TestOAuthStateExpires(t *testing.T) {
	client := &Client{
		logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		auth:   &fakeAuth{},
		states: map[string]time.Time{"expired": time.Now().Add(-time.Second)},
	}
	if err := client.CompleteAuth("code", "expired"); !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("expired state error = %v", err)
	}
	if _, exists := client.states["expired"]; exists {
		t.Fatal("expired state was not consumed")
	}
}

func TestCompleteAuthPersistsOnlyVerifiedToken(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "spotify-token.json")
	token := &oauth2.Token{AccessToken: "access", RefreshToken: "refresh", Expiry: time.Now().Add(time.Hour)}
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id":"user","display_name":"Test User"}`)),
		}, nil
	})}
	client := &Client{
		logger:    slog.New(slog.NewJSONHandler(io.Discard, nil)),
		ctx:       context.Background(),
		auth:      &fakeAuth{exchangeToken: token, httpClient: httpClient},
		states:    make(map[string]time.Time),
		tokenFile: tokenFile,
	}
	authURL := client.GetAuthURL()
	state := strings.TrimPrefix(authURL, "http://localhost/auth?state=")
	if err := client.CompleteAuth("code", state); err != nil {
		t.Fatalf("CompleteAuth() error = %v", err)
	}
	if !client.IsAuthenticated() {
		t.Fatal("client should be authenticated after verification")
	}
	info, err := os.Stat(tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("token permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestNewClientLoadsPersistedTokenForCLIReuse(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "spotify-token.json")
	if err := os.WriteFile(tokenFile, []byte(`{"access_token":"persisted","token_type":"Bearer","refresh_token":"refresh"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(config.SpotifyConfig{
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "http://localhost/callback",
		TokenFile:    tokenFile,
	}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if !client.IsAuthenticated() {
		t.Fatal("persisted token should authenticate a new CLI client")
	}
}

func TestCheckTracksInPlaylistReadsEveryPage(t *testing.T) {
	requestCount := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		body := `{"items":[{"track":{"type":"track","id":"first","name":"First"}}],"next":"https://api.spotify.com/v1/next"}`
		if strings.HasSuffix(req.URL.Path, "/next") {
			body = `{"items":[{"track":{"type":"track","id":"second","name":"Second"}}],"next":null}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	client := &Client{
		client:     spotifyapi.New(httpClient),
		logger:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
		token:      &oauth2.Token{AccessToken: "access", Expiry: time.Now().Add(time.Hour)},
		ctx:        context.Background(),
		isUserAuth: true,
	}

	results, err := client.CheckTracksInPlaylist("playlist", []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	if requestCount != 2 || len(results) != 2 || !results[0] || !results[1] {
		t.Fatalf("requests=%d results=%v", requestCount, results)
	}
}

func TestGetUserPlaylistsReadsEveryPage(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body string
		switch {
		case strings.Contains(req.URL.Path, "next-playlists"):
			body = `{"items":[{"id":"p2","name":"Second","uri":"spotify:playlist:p2","owner":{"id":"user"},"tracks":{"total":1}}],"next":null}`
		case strings.HasSuffix(req.URL.Path, "/playlists"):
			body = `{"items":[{"id":"p1","name":"First","uri":"spotify:playlist:p1","owner":{"id":"user"},"tracks":{"total":1}}],"next":"https://api.spotify.com/v1/next-playlists"}`
		default: // current user lookup
			body = `{"id":"user","display_name":"Test User"}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	client := &Client{
		client:     spotifyapi.New(httpClient),
		logger:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
		token:      &oauth2.Token{AccessToken: "access", Expiry: time.Now().Add(time.Hour)},
		ctx:        context.Background(),
		isUserAuth: true,
	}

	playlists, err := client.GetUserPlaylists("")
	if err != nil {
		t.Fatal(err)
	}
	if len(playlists) != 2 {
		t.Fatalf("expected playlists from every page, got %d: %+v", len(playlists), playlists)
	}
}

// TestRefreshToken_InvalidGrantDiscardsToken verifies that when the Spotify
// token endpoint returns an invalid_grant error during refresh, the client
// discards the stored token (so a failed refresh is never retried) and returns
// an error wrapping types.ErrReauthenticationRequired so callers can redirect
// the user to sign in again.
func TestRefreshToken_InvalidGrantDiscardsToken(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/callback",
	}

	// Token is already expired so RefreshToken proceeds to the refresh call.
	expiredToken := &oauth2.Token{
		AccessToken:  "expired-access",
		TokenType:    "Bearer",
		RefreshToken: "expired-refresh",
		Expiry:       time.Now().Add(-1 * time.Hour),
	}

	client := &Client{
		config:     cfg,
		logger:     logger,
		token:      expiredToken,
		ctx:        context.Background(),
		auth:       &fakeAuth{refreshErr: &oauth2.RetrieveError{ErrorCode: "invalid_grant", ErrorDescription: "Refresh token expired"}},
		isUserAuth: true,
	}

	err := client.RefreshToken()
	if err == nil {
		t.Fatal("RefreshToken() expected error for invalid_grant but got none")
	}
	if !errors.Is(err, types.ErrReauthenticationRequired) {
		t.Errorf("RefreshToken() error does not wrap ErrReauthenticationRequired: %v", err)
	}

	// The stored token and authenticated client must be discarded so the
	// failed refresh is not retried with the same bad token.
	if client.token != nil {
		t.Errorf("RefreshToken() should discard stored token, got %v", client.token)
	}
	if client.isUserAuth {
		t.Error("RefreshToken() should clear isUserAuth after invalid_grant")
	}
	if client.client != nil {
		t.Error("RefreshToken() should discard authenticated spotify client after invalid_grant")
	}
}

// TestRefreshToken_TransientErrorKeepsToken verifies that on a non-invalid_grant
// refresh failure the client keeps its auth state so the operation can be
// retried later (the discard rule applies specifically to invalid grants).
func TestRefreshToken_TransientErrorKeepsToken(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/callback",
	}

	expiredToken := &oauth2.Token{
		AccessToken:  "expired-access",
		TokenType:    "Bearer",
		RefreshToken: "expired-refresh",
		Expiry:       time.Now().Add(-1 * time.Hour),
	}

	client := &Client{
		config:     cfg,
		logger:     logger,
		token:      expiredToken,
		ctx:        context.Background(),
		auth:       &fakeAuth{refreshErr: errors.New("connection reset by peer")},
		isUserAuth: true,
	}

	err := client.RefreshToken()
	if err == nil {
		t.Fatal("RefreshToken() expected error for transient failure but got none")
	}
	if errors.Is(err, types.ErrReauthenticationRequired) {
		t.Errorf("RefreshToken() should not report ErrReauthenticationRequired for transient errors: %v", err)
	}

	// Auth state must be retained so a transient failure can be retried.
	if client.token == nil {
		t.Error("RefreshToken() should retain stored token on transient error")
	}
	if !client.isUserAuth {
		t.Error("RefreshToken() should retain isUserAuth on transient error")
	}
}

func TestNewClient(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	tests := []struct {
		name      string
		config    config.SpotifyConfig
		wantErr   bool
		errString string
	}{
		{
			name: "missing client ID",
			config: config.SpotifyConfig{
				ClientID:     "",
				ClientSecret: "test-secret",
			},
			wantErr:   true,
			errString: "spotify client ID and secret are required",
		},
		{
			name: "missing client secret",
			config: config.SpotifyConfig{
				ClientID:     "test-id",
				ClientSecret: "",
			},
			wantErr:   true,
			errString: "spotify client ID and secret are required",
		},
		{
			name: "invalid credentials",
			config: config.SpotifyConfig{
				ClientID:     "invalid-id",
				ClientSecret: "invalid-secret",
			},
			wantErr: true, // Will fail when trying to get token
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config, logger)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewClient() expected error but got none")
					return
				}
				if tt.errString != "" && err.Error() != tt.errString {
					t.Errorf("NewClient() error = %v, want %v", err.Error(), tt.errString)
				}
				return
			}

			if err != nil {
				t.Errorf("NewClient() unexpected error = %v", err)
				return
			}

			if client == nil {
				t.Error("NewClient() returned nil client")
			}
		})
	}
}

func TestClient_RefreshToken(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	// Test with invalid credentials to test error handling
	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	client := &Client{
		config: cfg,
		logger: logger,
		token:  nil, // No token, should trigger refresh
		ctx:    context.Background(),
	}

	err := client.RefreshToken()
	if err == nil {
		t.Error("RefreshToken() expected error with invalid credentials but got none")
	}
}

func TestClient_RefreshToken_NotNeeded(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	// Create a token that doesn't need refresh (expires in 1 hour)
	futureTime := time.Now().Add(1 * time.Hour)

	client := &Client{
		config: cfg,
		logger: logger,
		token: &oauth2.Token{
			AccessToken: "test-token",
			TokenType:   "Bearer",
			Expiry:      futureTime,
		},
		ctx:        context.Background(),
		isUserAuth: true, // Set to true to simulate authenticated state
	}

	err := client.RefreshToken()
	if err != nil {
		t.Errorf("RefreshToken() unexpected error when token doesn't need refresh: %v", err)
	}
}

func TestClient_SearchArtist_NoToken(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	client := &Client{
		config: cfg,
		logger: logger,
		token:  nil,
		ctx:    context.Background(),
	}

	_, err := client.SearchArtist("test artist")
	if err == nil {
		t.Error("SearchArtist() expected error when no valid token but got none")
	}
}

func TestClient_GetArtistTopTracks_NoToken(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	client := &Client{
		config: cfg,
		logger: logger,
		token:  nil,
		ctx:    context.Background(),
	}

	_, err := client.GetArtistTopTracks("test-artist-id")
	if err == nil {
		t.Error("GetArtistTopTracks() expected error when no valid token but got none")
	}
}

func TestClient_GetUserPlaylists_NoToken(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	client := &Client{
		config: cfg,
		logger: logger,
		token:  nil,
		ctx:    context.Background(),
	}

	_, err := client.GetUserPlaylists("Incoming")
	if err == nil {
		t.Error("GetUserPlaylists() expected error when no valid token but got none")
	}
}

func TestClient_AddTracksToPlaylist_NoTracks(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	client := &Client{
		config: cfg,
		logger: logger,
		ctx:    context.Background(),
	}

	err := client.AddTracksToPlaylist("test-playlist-id", []string{})
	if err == nil {
		t.Error("AddTracksToPlaylist() expected error when no tracks provided but got none")
	}

	expectedErr := "no tracks provided to add"
	if err.Error() != expectedErr {
		t.Errorf("AddTracksToPlaylist() error = %v, want %v", err.Error(), expectedErr)
	}
}

func TestClient_CheckTracksInPlaylist_NoTracks(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := config.SpotifyConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}

	client := &Client{
		config: cfg,
		logger: logger,
		ctx:    context.Background(),
	}

	results, err := client.CheckTracksInPlaylist("test-playlist-id", []string{})
	if err != nil {
		t.Errorf("CheckTracksInPlaylist() unexpected error with empty tracks: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("CheckTracksInPlaylist() expected empty results but got %v", results)
	}
}
