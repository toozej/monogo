package spotify

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/pkg/config"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

// Client wraps the Spotify client with authentication and configuration
type Client struct {
	client     *spotify.Client
	config     config.SpotifyConfig
	logger     *logrus.Logger
	token      *oauth2.Token
	tokenMu    sync.RWMutex
	ctx        context.Context
	auth       *spotifyauth.Authenticator
	isUserAuth bool
	authURL    string
	state      string
}

// NewClient creates a new Spotify client with user authentication flow
func NewClient(cfg config.SpotifyConfig, logger *logrus.Logger) (*Client, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("spotify client ID and secret are required")
	}

	ctx := context.Background()

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

	// Generate state for security
	state := "go-listen-auth-state"

	// Use the library's AuthURL method as shown in examples
	authURL := auth.AuthURL(state)

	logger.WithFields(logrus.Fields{
		"client_id":    cfg.ClientID,
		"redirect_url": cfg.RedirectURL,
		"state":        state,
		"auth_url":     authURL,
	}).Info("Generated Spotify auth URL using library method")

	// Validate redirect URL is configured
	if cfg.RedirectURL == "" {
		logger.Error("RedirectURL is empty! Check SPOTIFY_REDIRECT_URL environment variable")
		return nil, fmt.Errorf("redirect URL is required but not configured")
	}

	client := &Client{
		client:     nil, // Will be set after authentication
		config:     cfg,
		logger:     logger,
		token:      nil,
		ctx:        ctx,
		auth:       auth,
		isUserAuth: false,
		authURL:    authURL,
		state:      state,
	}

	logger.Info("Spotify client initialized, user authentication required")
	logger.WithField("auth_url", client.authURL).Info("Visit this URL to authenticate with Spotify")

	return client, nil
}

// GetAuthURL returns the URL for user authentication
func (c *Client) GetAuthURL() string {
	return c.authURL
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
	defer c.tokenMu.Unlock()

	if state != c.state {
		return fmt.Errorf("invalid state parameter")
	}

	c.logger.Debug("Completing Spotify authentication")

	// Exchange authorization code for token using the library method
	token, err := c.auth.Exchange(c.ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange code for token: %w", err)
	}

	c.token = token
	c.isUserAuth = true

	// Create authenticated Spotify client following library examples
	httpClient := c.auth.Client(c.ctx, token)
	c.client = spotify.New(httpClient)

	c.logger.Info("Spotify user authentication completed successfully")

	// Test the authentication by fetching current user info
	user, err := c.client.CurrentUser(c.ctx)
	if err != nil {
		c.logger.WithError(err).Error("Failed to verify authentication by getting current user")
		return fmt.Errorf("authentication verification failed: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"user_id":           user.ID,
		"user_display_name": user.DisplayName,
	}).Info("Authentication verified successfully")

	return nil
}

// RefreshToken refreshes the access token if needed
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
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	c.token = newToken

	// Update the client with new token following library examples
	httpClient := c.auth.Client(c.ctx, newToken)
	c.client = spotify.New(httpClient)

	c.logger.Info("Spotify access token refreshed successfully")
	return nil
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
	results, err := c.client.Search(c.ctx, query, spotify.SearchTypeArtist)
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
	topTracks, err := c.client.GetArtistsTopTracks(c.ctx, spotify.ID(artistID), spotify.CountryUSA)
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
	currentUser, err := c.client.CurrentUser(c.ctx)
	if err != nil {
		c.logger.WithError(err).Error("Failed to get current user - authentication may have failed")
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"user_id":           currentUser.ID,
		"user_display_name": currentUser.DisplayName,
	}).Info("Successfully validated user authentication")

	// Get all user playlists using the library method
	playlistPage, err := c.client.CurrentUsersPlaylists(c.ctx)
	if err != nil {
		c.logger.WithError(err).WithField("folder_name", folderName).Error("Failed to get user playlists")
		return nil, fmt.Errorf("failed to get user playlists: %w", err)
	}

	allPlaylists := playlistPage.Playlists

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
	_, err := c.client.AddTracksToPlaylist(c.ctx, spotify.ID(playlistID), spotifyIDs...)
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
	items, err := c.client.GetPlaylistItems(c.ctx, spotify.ID(playlistID))
	if err != nil {
		c.logger.WithError(err).WithField("playlist_id", playlistID).Error("Failed to get playlist items")
		return nil, fmt.Errorf("failed to get playlist items: %w", err)
	}

	// Create a set of existing track IDs for efficient lookup
	existingTracks := make(map[string]bool)
	existingTrackNames := make([]string, 0)

	for i := range items.Items {
		playlistItem := &items.Items[i]
		if playlistItem.Track.Track != nil && playlistItem.Track.Track.ID != "" {
			trackID := string(playlistItem.Track.Track.ID)
			existingTracks[trackID] = true
			existingTrackNames = append(existingTrackNames, playlistItem.Track.Track.Name)
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
