package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toozej/monogo/apps/go-listen/internal/config"
	"github.com/toozej/monogo/apps/go-listen/internal/types"
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
	refreshErr error
}

func (f *fakeAuth) AuthURL(state string, opts ...oauth2.AuthCodeOption) string {
	return "http://localhost/auth?state=" + state
}

func (f *fakeAuth) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeAuth) RefreshToken(ctx context.Context, token *oauth2.Token) (*oauth2.Token, error) {
	return nil, f.refreshErr
}

func (f *fakeAuth) Client(ctx context.Context, token *oauth2.Token) *http.Client {
	return &http.Client{}
}

// TestRefreshToken_InvalidGrantDiscardsToken verifies that when the Spotify
// token endpoint returns an invalid_grant error during refresh, the client
// discards the stored token (so a failed refresh is never retried) and returns
// an error wrapping types.ErrReauthenticationRequired so callers can redirect
// the user to sign in again.
func TestRefreshToken_InvalidGrantDiscardsToken(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

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
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

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
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

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
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

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
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

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
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

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
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

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
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

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
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

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
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

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
