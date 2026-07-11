package spotify

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toozej/monogo/apps/go-listen/internal/config"
	"github.com/toozej/monogo/apps/go-listen/internal/types"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

// isInvalidGrantError reports whether err is (or wraps) an OAuth2 "invalid_grant"
// error returned by the Spotify token endpoint. Per Spotify's refresh token
// documentation, the token endpoint returns invalid_grant when a refresh token
// is expired, revoked, or otherwise invalid, and the app must discard the
// refresh token and restart the authorization flow rather than retrying.
func isInvalidGrantError(err error) bool {
	if err == nil {
		return false
	}
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) {
		return retrieveErr.ErrorCode == "invalid_grant"
	}
	// Fall back to a substring check for wrapped error strings from
	// alternative transport implementations.
	return strings.Contains(err.Error(), "invalid_grant")
}

// authenticator abstracts the spotifyauth.Authenticator methods the Client
// depends on, allowing the refresh/reauth path to be unit-tested with a fake
// implementation instead of hitting the live Spotify token endpoint.
type authenticator interface {
	AuthURL(state string, opts ...oauth2.AuthCodeOption) string
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	RefreshToken(ctx context.Context, token *oauth2.Token) (*oauth2.Token, error)
	Client(ctx context.Context, token *oauth2.Token) *http.Client
}

// Client wraps the Spotify client with authentication and configuration
type Client struct {
	client     *spotify.Client
	config     config.SpotifyConfig
	logger     *logrus.Logger
	token      *oauth2.Token
	tokenMu    sync.RWMutex
	ctx        context.Context
	auth       authenticator
	isUserAuth bool
	states     map[string]time.Time
	tokenFile  string
}

// NewClient creates a new Spotify client with user authentication flow
func NewClient(cfg config.SpotifyConfig, logger *logrus.Logger) (*Client, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("spotify client ID and secret are required")
	}

	ctx := context.Background()
	if cfg.RedirectURL == "" {
		return nil, fmt.Errorf("redirect URL is required but not configured")
	}

	// Set up Authorization Code flow for user authentication following the library examples
	auth := spotifyauth.New(
		spotifyauth.WithRedirectURL(cfg.RedirectURL),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopePlaylistReadPrivate,
			spotifyauth.ScopePlaylistModifyPrivate,
			spotifyauth.ScopePlaylistModifyPublic,
		),
		spotifyauth.WithClientID(cfg.ClientID),
		spotifyauth.WithClientSecret(cfg.ClientSecret),
	)

	logger.WithFields(logrus.Fields{
		"client_id":    cfg.ClientID,
		"redirect_url": cfg.RedirectURL,
	}).Info("Initialized Spotify OAuth client")

	client := &Client{
		client:     nil, // Will be set after authentication
		config:     cfg,
		logger:     logger,
		token:      nil,
		ctx:        ctx,
		auth:       auth,
		isUserAuth: false,
		states:     make(map[string]time.Time),
		tokenFile:  spotifyTokenFile(cfg.TokenFile),
	}
	if token, err := client.loadPersistedToken(); err == nil {
		client.token = token
		client.client = spotify.New(auth.Client(ctx, token))
		client.isUserAuth = true
		logger.Info("Loaded persisted Spotify authentication")
	} else if !errors.Is(err, os.ErrNotExist) {
		logger.WithError(err).Warn("Ignoring unusable persisted Spotify token")
	}

	logger.Info("Spotify client initialized")

	return client, nil
}

// GetAuthURL returns the URL for user authentication
func (c *Client) GetAuthURL() string {
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		c.logger.WithError(err).Error("Failed to generate Spotify OAuth state")
		return ""
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	c.tokenMu.Lock()
	now := time.Now()
	for candidate, expiry := range c.states {
		if now.After(expiry) {
			delete(c.states, candidate)
		}
	}
	c.states[state] = now.Add(10 * time.Minute)
	c.tokenMu.Unlock()
	return c.auth.AuthURL(state)
}

// IsAuthenticated returns whether the user is authenticated
func (c *Client) IsAuthenticated() bool {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.isUserAuth && c.client != nil
}

// CompleteAuth completes the authentication process with the authorization code
func (c *Client) CompleteAuth(code, state string) error {
	c.tokenMu.Lock()
	expires, validState := c.states[state]
	delete(c.states, state)
	c.tokenMu.Unlock()
	if !validState || time.Now().After(expires) {
		return fmt.Errorf("invalid state parameter")
	}

	c.logger.Debug("Completing Spotify authentication")

	// Exchange authorization code for token using the library method
	token, err := c.auth.Exchange(c.ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange code for token: %w", err)
	}

	// Create authenticated Spotify client following library examples
	httpClient := c.auth.Client(c.ctx, token)
	spotifyClient := spotify.New(httpClient)

	// Test the authentication by fetching current user info
	user, err := spotifyClient.CurrentUser(c.ctx)
	if err != nil {
		c.logger.WithError(err).Error("Failed to verify authentication by getting current user")
		return fmt.Errorf("authentication verification failed: %w", err)
	}
	if err := c.persistToken(token); err != nil {
		return fmt.Errorf("persisting Spotify token: %w", err)
	}

	c.tokenMu.Lock()
	c.token = token
	c.client = spotifyClient
	c.isUserAuth = true
	c.tokenMu.Unlock()
	c.logger.Info("Spotify user authentication completed successfully")

	c.logger.WithFields(logrus.Fields{
		"user_id":           user.ID,
		"user_display_name": user.DisplayName,
	}).Info("Authentication verified successfully")

	return nil
}

// RefreshToken refreshes the access token if needed.
//
// Spotify refresh tokens issued to apps have a 6-month lifetime and are not
// extended by refreshing. When the token endpoint responds with an
// "invalid_grant" error (refresh token expired, revoked, or otherwise
// invalid), this method discards the stored token and authenticated client so
// a failed refresh is never retried with the same token. The returned error
// wraps ErrReauthenticationRequired so callers can detect the need to send the
// user through the authorization flow again via errors.Is.
//
// This behavior only applies to user-issued tokens; Client Credentials flows
// are unaffected.
//
// See: https://developer.spotify.com/documentation/web-api/tutorials/refreshing-tokens
func (c *Client) RefreshToken() error {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if !c.isUserAuth {
		return fmt.Errorf("user not authenticated")
	}

	// Check if token needs refresh (refresh 5 minutes before expiry)
	if c.token != nil && time.Until(c.token.Expiry) > 5*time.Minute {
		return nil
	}

	c.logger.Debug("Refreshing Spotify access token")

	// Use the authenticator to refresh the token following library patterns
	newToken, err := c.auth.RefreshToken(c.ctx, c.token)
	if err != nil {
		// Per Spotify's guidance, do not retry a failed refresh. When the
		// refresh token is expired, revoked, or invalid the token endpoint
		// returns invalid_grant; discard the stored credentials first so the
		// user is forced to reauthorize rather than retrying with a bad token.
		if isInvalidGrantError(err) {
			c.logger.WithError(err).Warn("Spotify refresh token is no longer valid; discarding stored token and requiring reauthorization")
			c.discardAuthLocked()
			if removeErr := os.Remove(c.tokenFile); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				c.logger.WithError(removeErr).Warn("Failed to remove invalid Spotify token file")
			}
			return fmt.Errorf("%w: %v", types.ErrReauthenticationRequired, err)
		}

		return fmt.Errorf("failed to refresh token: %w", err)
	}

	// Update the client with new token following library examples
	httpClient := c.auth.Client(c.ctx, newToken)
	if err := c.persistToken(newToken); err != nil {
		return fmt.Errorf("persisting refreshed Spotify token: %w", err)
	}
	c.token = newToken
	c.client = spotify.New(httpClient)

	c.logger.Info("Spotify access token refreshed successfully")
	return nil
}

func spotifyTokenFile(configured string) string {
	if configured != "" {
		return configured
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".go-listen", "spotify-token.json")
	}
	return filepath.Join(configDir, "go-listen", "spotify-token.json")
}

func (c *Client) loadPersistedToken() (loadedToken *oauth2.Token, resultErr error) {
	file, err := os.Open(c.tokenFile) // #nosec G304 -- path is explicit application configuration
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			loadedToken = nil
			resultErr = errors.Join(resultErr, fmt.Errorf("close persisted token file: %w", closeErr))
		}
	}()
	if runtime.GOOS != "windows" {
		info, statErr := file.Stat()
		if statErr != nil {
			return nil, statErr
		}
		if info.Mode().Perm()&0o077 != 0 {
			return nil, fmt.Errorf("token file permissions must not allow group or other access")
		}
	}
	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode token: %w", err)
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("persisted token has no access token")
	}
	return &token, nil
}

func (c *Client) persistToken(token *oauth2.Token) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}
	dir := filepath.Dir(c.tokenFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".spotify-token-*") // #nosec G304 -- directory is explicit application configuration
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() {
		if removeErr := os.Remove(tempName); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			c.logger.WithError(removeErr).Warn("Failed to remove temporary Spotify token file")
		}
	}()
	closeTemp := func(writeErr error) error {
		if closeErr := temp.Close(); closeErr != nil {
			return errors.Join(writeErr, fmt.Errorf("close temporary token file: %w", closeErr))
		}
		return writeErr
	}
	if err := temp.Chmod(0o600); err != nil {
		return closeTemp(err)
	}
	// #nosec G117 -- the OAuth token is intentionally persisted to this mode-0600 temporary file.
	if err := json.NewEncoder(temp).Encode(token); err != nil {
		return closeTemp(err)
	}
	if err := temp.Sync(); err != nil {
		return closeTemp(err)
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, c.tokenFile); err != nil && runtime.GOOS == "windows" {
		if removeErr := os.Remove(c.tokenFile); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		return os.Rename(tempName, c.tokenFile)
	} else {
		return err
	}
}

// discardAuthLocked clears all cached authentication state so that subsequent
// calls require the user to reauthorize. The caller must hold c.tokenMu.
func (c *Client) discardAuthLocked() {
	c.token = nil
	c.client = nil
	c.isUserAuth = false
}

func (c *Client) authenticatedClient() *spotify.Client {
	c.tokenMu.RLock()
	defer c.tokenMu.RUnlock()
	return c.client
}

// SearchArtist searches for an artist by name and returns the best match
func (c *Client) SearchArtist(query string) (*Artist, error) {
	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("user not authenticated to Spotify")
	}

	if err := c.RefreshToken(); err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	c.logger.WithField("query", query).Debug("Searching for artist using Spotify library")

	// Use the library's Search method following the examples
	spotifyClient := c.authenticatedClient()
	results, err := spotifyClient.Search(c.ctx, query, spotify.SearchTypeArtist)
	if err != nil {
		c.logger.WithError(err).WithField("query", query).Error("Failed to search for artist")
		return nil, fmt.Errorf("failed to search for artist: %w", err)
	}

	if results.Artists == nil || len(results.Artists.Artists) == 0 {
		c.logger.WithField("query", query).Warn("No artists found")
		return nil, fmt.Errorf("no artists found for query: %s", query)
	}

	// Return the first (most relevant) result following library patterns
	spotifyArtist := results.Artists.Artists[0]

	artist := &Artist{
		ID:     string(spotifyArtist.ID),
		Name:   spotifyArtist.Name,
		URI:    string(spotifyArtist.URI),
		Genres: spotifyArtist.Genres,
	}

	c.logger.WithFields(logrus.Fields{
		"query":       query,
		"artist_id":   artist.ID,
		"artist_name": artist.Name,
		"genres":      artist.Genres,
	}).Info("Artist found using Spotify library")

	return artist, nil
}

// GetArtistTopTracks retrieves the top tracks for an artist (limited to 5)
func (c *Client) GetArtistTopTracks(artistID string) ([]Track, error) {
	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("user not authenticated to Spotify")
	}

	if err := c.RefreshToken(); err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	c.logger.WithField("artist_id", artistID).Debug("Getting artist top tracks using Spotify library")

	// Get top tracks for the artist using library method with market parameter
	spotifyClient := c.authenticatedClient()
	topTracks, err := spotifyClient.GetArtistsTopTracks(c.ctx, spotify.ID(artistID), spotify.CountryUSA)
	if err != nil {
		c.logger.WithError(err).WithField("artist_id", artistID).Error("Failed to get artist top tracks")
		return nil, fmt.Errorf("failed to get top tracks for artist %s: %w", artistID, err)
	}

	if len(topTracks) == 0 {
		c.logger.WithField("artist_id", artistID).Warn("No top tracks found for artist")
		return []Track{}, nil
	}

	// Limit to top 5 tracks as per requirements
	maxTracks := 5
	if len(topTracks) < maxTracks {
		maxTracks = len(topTracks)
	}

	tracks := make([]Track, maxTracks)
	trackNames := make([]string, maxTracks)

	for i := range topTracks[:maxTracks] {
		spotifyTrack := &topTracks[i]
		// Convert Spotify artists to our Artist type
		artists := make([]Artist, len(spotifyTrack.Artists))
		for j, spotifyArtist := range spotifyTrack.Artists {
			artists[j] = Artist{
				ID:     string(spotifyArtist.ID),
				Name:   spotifyArtist.Name,
				URI:    string(spotifyArtist.URI),
				Genres: []string{}, // SimpleArtist doesn't include genres, would need full artist lookup
			}
		}

		tracks[i] = Track{
			ID:       string(spotifyTrack.ID),
			Name:     spotifyTrack.Name,
			URI:      string(spotifyTrack.URI),
			Artists:  artists,
			Duration: int(spotifyTrack.Duration),
		}
		trackNames[i] = spotifyTrack.Name
	}

	c.logger.WithFields(logrus.Fields{
		"artist_id":    artistID,
		"tracks_found": len(tracks),
		"track_names":  trackNames,
	}).Info("Retrieved artist top tracks using Spotify library")

	return tracks, nil
}

// GetUserPlaylists retrieves playlists from a specific folder (for now, returns all user playlists)
func (c *Client) GetUserPlaylists(folderName string) ([]Playlist, error) {
	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("user not authenticated to Spotify")
	}

	if err := c.RefreshToken(); err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	c.logger.WithField("folder_name", folderName).Debug("Getting user playlists using Spotify library")

	// Get current user first to validate authentication and for filtering
	spotifyClient := c.authenticatedClient()
	currentUser, err := spotifyClient.CurrentUser(c.ctx)
	if err != nil {
		c.logger.WithError(err).Error("Failed to get current user - authentication may have failed")
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"user_id":           currentUser.ID,
		"user_display_name": currentUser.DisplayName,
	}).Info("Successfully validated user authentication")

	// Get all user playlists using the library method
	playlistPage, err := spotifyClient.CurrentUsersPlaylists(c.ctx)
	if err != nil {
		c.logger.WithError(err).WithField("folder_name", folderName).Error("Failed to get user playlists")
		return nil, fmt.Errorf("failed to get user playlists: %w", err)
	}

	allPlaylists := make([]spotify.SimplePlaylist, 0, len(playlistPage.Playlists))
	for {
		allPlaylists = append(allPlaylists, playlistPage.Playlists...)
		err = spotifyClient.NextPage(c.ctx, playlistPage)
		if errors.Is(err, spotify.ErrNoMorePages) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get next user playlists page: %w", err)
		}
	}

	c.logger.WithFields(logrus.Fields{
		"total_playlists": len(allPlaylists),
		"folder_name":     folderName,
	}).Info("Retrieved all user playlists using Spotify library")

	// Log playlist names for debugging
	playlistNames := make([]string, len(allPlaylists))
	for i := range allPlaylists {
		pl := &allPlaylists[i]
		playlistNames[i] = pl.Name
	}
	c.logger.WithFields(logrus.Fields{
		"playlist_names": playlistNames,
	}).Debug("Available playlists")

	var filteredPlaylists []Playlist
	for i := range allPlaylists {
		spotifyPlaylist := &allPlaylists[i]
		// Only include playlists owned by the user (not followed playlists)
		isUserOwned := spotifyPlaylist.Owner.ID == currentUser.ID

		// If a folder name is specified, try basic filtering
		// Since Spotify Web API doesn't provide direct folder access, we'll use multiple strategies:
		// 1. Check if playlist name contains folder name
		// 2. Check if playlist description contains folder name
		// 3. For "Incoming" specifically, look for playlists that start with "i" (common pattern)
		matchesFolder := folderName == ""

		if folderName != "" {
			folderLower := strings.ToLower(folderName)
			playlistNameLower := strings.ToLower(spotifyPlaylist.Name)
			playlistDescLower := strings.ToLower(spotifyPlaylist.Description)

			// Basic name/description matching
			matchesFolder = strings.Contains(playlistNameLower, folderLower) ||
				strings.Contains(playlistDescLower, folderLower)

			// Special case for "Incoming" folder - look for playlists starting with "i"
			if !matchesFolder && folderLower == "incoming" {
				matchesFolder = strings.HasPrefix(playlistNameLower, "i") && len(spotifyPlaylist.Name) > 1
			}
		}

		if isUserOwned && matchesFolder {
			playlist := Playlist{
				ID:         string(spotifyPlaylist.ID),
				Name:       spotifyPlaylist.Name,
				URI:        string(spotifyPlaylist.URI),
				TrackCount: int(spotifyPlaylist.Tracks.Total),
				EmbedURL:   fmt.Sprintf("https://open.spotify.com/embed/playlist/%s", spotifyPlaylist.ID),
				IsIncoming: true, // Mark as incoming since it passed our filter
			}
			filteredPlaylists = append(filteredPlaylists, playlist)
		}
	}

	c.logger.WithFields(logrus.Fields{
		"folder_name":     folderName,
		"filtered_count":  len(filteredPlaylists),
		"total_playlists": len(allPlaylists),
	}).Info("Filtered user playlists")

	// If no playlists match the folder filter but we have playlists,
	// return all user playlists as a fallback
	if len(filteredPlaylists) == 0 && len(allPlaylists) > 0 && folderName != "" {
		c.logger.WithField("folder_name", folderName).Warn("No playlists found matching folder name, returning all user playlists")

		for i := range allPlaylists {
			spotifyPlaylist := &allPlaylists[i]
			isUserOwned := spotifyPlaylist.Owner.ID == currentUser.ID

			if isUserOwned {
				playlist := Playlist{
					ID:         string(spotifyPlaylist.ID),
					Name:       spotifyPlaylist.Name,
					URI:        string(spotifyPlaylist.URI),
					TrackCount: int(spotifyPlaylist.Tracks.Total),
					EmbedURL:   fmt.Sprintf("https://open.spotify.com/embed/playlist/%s", spotifyPlaylist.ID),
					IsIncoming: true,
				}
				filteredPlaylists = append(filteredPlaylists, playlist)
			}
		}
	}

	return filteredPlaylists, nil
}

// AddTracksToPlaylist adds tracks to a specified playlist
func (c *Client) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	if len(trackIDs) == 0 {
		return fmt.Errorf("no tracks provided to add")
	}

	if !c.IsAuthenticated() {
		return fmt.Errorf("user not authenticated to Spotify")
	}

	if err := c.RefreshToken(); err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"playlist_id": playlistID,
		"track_count": len(trackIDs),
		"track_ids":   trackIDs,
	}).Debug("Adding tracks to playlist using Spotify library")

	// Convert string IDs to Spotify IDs following library patterns
	spotifyIDs := make([]spotify.ID, len(trackIDs))
	for i, trackID := range trackIDs {
		spotifyIDs[i] = spotify.ID(trackID)
	}

	// Add tracks to playlist using library method
	spotifyClient := c.authenticatedClient()
	_, err := spotifyClient.AddTracksToPlaylist(c.ctx, spotify.ID(playlistID), spotifyIDs...)
	if err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"playlist_id": playlistID,
			"track_count": len(trackIDs),
			"track_ids":   trackIDs,
		}).Error("Failed to add tracks to playlist")
		return fmt.Errorf("failed to add tracks to playlist %s: %w", playlistID, err)
	}

	c.logger.WithFields(logrus.Fields{
		"playlist_id": playlistID,
		"track_count": len(trackIDs),
		"track_ids":   trackIDs,
	}).Info("Successfully added tracks to playlist using Spotify library")

	return nil
}

// CheckTracksInPlaylist checks if tracks already exist in a playlist
func (c *Client) CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error) {
	if len(trackIDs) == 0 {
		return []bool{}, nil
	}

	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("user not authenticated to Spotify")
	}

	if err := c.RefreshToken(); err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"playlist_id": playlistID,
		"track_count": len(trackIDs),
		"track_ids":   trackIDs,
	}).Debug("Checking tracks in playlist using Spotify library")

	// Get all tracks from the playlist using library method
	spotifyClient := c.authenticatedClient()
	items, err := spotifyClient.GetPlaylistItems(c.ctx, spotify.ID(playlistID))
	if err != nil {
		c.logger.WithError(err).WithField("playlist_id", playlistID).Error("Failed to get playlist items")
		return nil, fmt.Errorf("failed to get playlist items: %w", err)
	}

	// Create a set of existing track IDs for efficient lookup
	existingTracks := make(map[string]bool)
	existingTrackNames := make([]string, 0)

	for {
		for i := range items.Items {
			playlistItem := &items.Items[i]
			if playlistItem.Track.Track != nil && playlistItem.Track.Track.ID != "" {
				trackID := string(playlistItem.Track.Track.ID)
				existingTracks[trackID] = true
				existingTrackNames = append(existingTrackNames, playlistItem.Track.Track.Name)
			}
		}
		err = spotifyClient.NextPage(c.ctx, items)
		if errors.Is(err, spotify.ErrNoMorePages) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get next playlist items page: %w", err)
		}
	}

	c.logger.WithFields(logrus.Fields{
		"playlist_id":          playlistID,
		"existing_track_count": len(existingTracks),
		"existing_track_names": existingTrackNames,
	}).Debug("Retrieved existing tracks from playlist")

	// Check each provided track ID
	results := make([]bool, len(trackIDs))
	for i, trackID := range trackIDs {
		results[i] = existingTracks[trackID]
	}

	duplicateCount := 0
	for _, isDuplicate := range results {
		if isDuplicate {
			duplicateCount++
		}
	}

	c.logger.WithFields(logrus.Fields{
		"playlist_id":     playlistID,
		"track_count":     len(trackIDs),
		"duplicate_count": duplicateCount,
	}).Info("Checked tracks in playlist using Spotify library")

	return results, nil
}
