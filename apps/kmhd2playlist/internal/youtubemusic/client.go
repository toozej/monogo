package youtubemusic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/config"
)

const (
	baseURL       = "https://music.youtube.com/youtubei/v1/"
	apiKey        = "REDACTED_API_KEY" // #nosec G101 -- nosemgrep:generic.secrets.security.detected-generic-api-key.detected-generic-api-key YouTube Music public API key, not a secret
	userAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0"
	contentType   = "application/json; charset=utf-8"
	origin        = "https://music.youtube.com"
	referer       = "https://music.youtube.com/"
	clientName    = "WEB_REMIX"
	clientVersion = "1.20240101.00.00"
)

type Client struct {
	config     config.YouTubeMusicConfig
	logger     *logrus.Logger
	cookie     string
	isAuthed   bool
	httpClient *http.Client
	tokenFile  string
	mu         sync.RWMutex
}

type contextPayload struct {
	Client struct {
		Name    string `json:"clientName"`
		Version string `json:"clientVersion"`
		HL      string `json:"hl"`
		GL      string `json:"gl"`
	} `json:"client"`
}

func newContext() contextPayload {
	ctx := contextPayload{}
	ctx.Client.Name = clientName
	ctx.Client.Version = clientVersion
	ctx.Client.HL = "en"
	ctx.Client.GL = "US"
	return ctx
}

func NewClient(cfg config.YouTubeMusicConfig, logger *logrus.Logger) (*Client, error) {
	if cfg.Cookie == "" {
		return nil, fmt.Errorf("youtube music cookie is required")
	}

	tokenFile, err := cfg.GetTokenFilePath()
	if err != nil {
		logger.WithError(err).Warn("Could not determine token file path")
	}

	client := &Client{
		config:     cfg,
		logger:     logger,
		cookie:     cfg.Cookie,
		isAuthed:   cfg.Cookie != "",
		httpClient: &http.Client{},
		tokenFile:  tokenFile,
	}

	client.loadAuthState()

	logger.WithFields(logrus.Fields{
		"has_cookie": cfg.Cookie != "",
		"token_file": tokenFile,
	}).Info("YouTube Music client initialized")

	return client, nil
}

func (c *Client) doRequest(endpoint string, body any) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	url := baseURL + endpoint + "?key=" + apiKey
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", referer)
	req.Header.Set("Cookie", c.cookie)
	req.Header.Set("X-Goog-AuthUser", "0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		c.mu.Lock()
		c.isAuthed = false
		c.mu.Unlock()
		return nil, fmt.Errorf("unauthorized: cookie may be invalid or expired")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, endpoint)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

func (c *Client) IsAuthenticated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isAuthed
}

func (c *Client) GetAuthURL() string {
	return "https://music.youtube.com"
}

func (c *Client) CompleteAuth(cookie, _ string) error {
	if cookie == "" {
		return fmt.Errorf("cookie is required for YouTube Music authentication")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.cookie = cookie
	c.isAuthed = true

	if err := c.saveAuthState(); err != nil {
		c.logger.WithError(err).Warn("Failed to save YouTube Music auth state")
	}

	c.logger.Info("YouTube Music authentication completed via cookie")
	return nil
}

func (c *Client) SearchArtist(query string) (*Artist, error) {
	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated to YouTube Music")
	}

	c.logger.WithField("query", query).Debug("Searching for artist on YouTube Music")

	reqBody := struct {
		Context contextPayload `json:"context"`
		Query   string         `json:"query"`
		Params  string         `json:"params,omitempty"`
	}{
		Context: newContext(),
		Query:   query,
	}

	data, err := c.doRequest("search", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to search for artist: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	contents, ok := result["contents"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("no artists found for query: %s", query)
	}

	tabResults, ok := contents["tabbedSearchResultsRenderer"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("no artists found for query: %s", query)
	}

	tabs, ok := tabResults["tabs"].([]any)
	if !ok || len(tabs) == 0 {
		return nil, fmt.Errorf("no artists found for query: %s", query)
	}

	for _, tab := range tabs {
		tabMap, ok := tab.(map[string]any)
		if !ok {
			continue
		}
		tabContent, ok := tabMap["tabRenderer"].(map[string]any)
		if !ok {
			continue
		}
		content, ok := tabContent["content"].(map[string]any)
		if !ok {
			continue
		}
		sectionList, ok := content["sectionListRenderer"].(map[string]any)
		if !ok {
			continue
		}
		sections, ok := sectionList["contents"].([]any)
		if !ok {
			continue
		}
		for _, section := range sections {
			sectionMap, ok := section.(map[string]any)
			if !ok {
				continue
			}
			musicShelf, ok := sectionMap["musicShelfRenderer"].(map[string]any)
			if !ok {
				continue
			}
			contents, ok := musicShelf["contents"].([]any)
			if !ok || len(contents) == 0 {
				continue
			}
			firstItem, ok := contents[0].(map[string]any)
			if !ok {
				continue
			}
			flexColumns, ok := firstItem["musicResponsiveListItemRenderer"].(map[string]any)
			if !ok {
				continue
			}
			fc, ok := flexColumns["flexColumns"].([]any)
			if !ok || len(fc) == 0 {
				continue
			}
			firstCol, ok := fc[0].(map[string]any)
			if !ok {
				continue
			}
			textRuns, ok := firstCol["musicResponsiveListItemFlexColumnRenderer"].(map[string]any)
			if !ok {
				continue
			}
			textObj, ok := textRuns["text"].(map[string]any)
			if !ok {
				continue
			}
			runs, ok := textObj["runs"].([]any)
			if !ok || len(runs) == 0 {
				continue
			}
			firstRun, ok := runs[0].(map[string]any)
			if !ok {
				continue
			}
			artistName, _ := firstRun["text"].(string)

			navigationEndpoint, ok := firstRun["navigationEndpoint"].(map[string]any)
			if !ok {
				continue
			}
			browseEndpoint, ok := navigationEndpoint["browseEndpoint"].(map[string]any)
			if !ok {
				continue
			}
			artistID, _ := browseEndpoint["browseId"].(string)

			if artistID != "" && strings.HasPrefix(artistID, "UC") {
				artist := &Artist{
					ID:     artistID,
					Name:   artistName,
					URI:    fmt.Sprintf("https://music.youtube.com/channel/%s", artistID),
					Genres: []string{},
				}
				c.logger.WithFields(logrus.Fields{
					"query":       query,
					"artist_id":   artist.ID,
					"artist_name": artist.Name,
				}).Debug("Artist found on YouTube Music")
				return artist, nil
			}
		}
	}

	return nil, fmt.Errorf("no artists found for query: %s", query)
}

func (c *Client) GetArtistTopTracks(artistID string) ([]Track, error) {
	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated to YouTube Music")
	}

	c.logger.WithField("artist_id", artistID).Debug("Getting artist top tracks from YouTube Music")

	reqBody := struct {
		Context  contextPayload `json:"context"`
		BrowseID string         `json:"browseId"`
	}{
		Context:  newContext(),
		BrowseID: artistID,
	}

	data, err := c.doRequest("browse", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to browse artist: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse browse response: %w", err)
	}

	var tracks []Track

	contents, ok := result["contents"].(map[string]any)
	if !ok {
		return tracks, nil
	}

	singleColumn, ok := contents["singleColumnBrowseResultsRenderer"].(map[string]any)
	if !ok {
		return tracks, nil
	}

	tabs, ok := singleColumn["tabs"].([]any)
	if !ok || len(tabs) == 0 {
		return tracks, nil
	}

	for _, tab := range tabs {
		tabMap, ok := tab.(map[string]any)
		if !ok {
			continue
		}
		tabContent, ok := tabMap["tabRenderer"].(map[string]any)
		if !ok {
			continue
		}
		content, ok := tabContent["content"].(map[string]any)
		if !ok {
			continue
		}
		sectionList, ok := content["sectionListRenderer"].(map[string]any)
		if !ok {
			continue
		}
		sections, ok := sectionList["contents"].([]any)
		if !ok {
			continue
		}
		for _, section := range sections {
			sectionMap, ok := section.(map[string]any)
			if !ok {
				continue
			}
			shelf, ok := sectionMap["musicShelfRenderer"].(map[string]any)
			if !ok {
				continue
			}
			shelfContents, ok := shelf["contents"].([]any)
			if !ok {
				continue
			}
			for _, item := range shelfContents {
				track := c.parseTrackFromRenderer(item)
				if track != nil {
					tracks = append(tracks, *track)
				}
				if len(tracks) >= 5 {
					break
				}
			}
			if len(tracks) >= 5 {
				break
			}
		}
		if len(tracks) >= 5 {
			break
		}
	}

	c.logger.WithFields(logrus.Fields{
		"artist_id":    artistID,
		"tracks_found": len(tracks),
	}).Debug("Retrieved artist top tracks from YouTube Music")

	return tracks, nil
}

func (c *Client) parseTrackFromRenderer(item any) *Track {
	itemMap, ok := item.(map[string]any)
	if !ok {
		return nil
	}

	renderer, ok := itemMap["musicResponsiveListItemRenderer"].(map[string]any)
	if !ok {
		return nil
	}

	flexColumns, ok := renderer["flexColumns"].([]any)
	if !ok || len(flexColumns) < 1 {
		return nil
	}

	trackName := c.extractTextFromFlexColumn(flexColumns, 0)
	artistName := c.extractTextFromFlexColumn(flexColumns, 1)
	albumName := c.extractTextFromFlexColumn(flexColumns, 2)

	trackID := ""
	if playlistItemData, ok := renderer["playlistItemData"].(map[string]any); ok {
		trackID, _ = playlistItemData["videoId"].(string)
	}

	if trackID == "" || trackName == "" {
		return nil
	}

	return &Track{
		ID:   trackID,
		Name: trackName,
		URI:  fmt.Sprintf("https://music.youtube.com/watch?v=%s", trackID),
		Artists: []Artist{
			{Name: artistName},
		},
		Album: Album{
			Name: albumName,
		},
	}
}

func (c *Client) extractTextFromFlexColumn(flexColumns []any, index int) string {
	if index >= len(flexColumns) {
		return ""
	}

	col, ok := flexColumns[index].(map[string]any)
	if !ok {
		return ""
	}

	renderer, ok := col["musicResponsiveListItemFlexColumnRenderer"].(map[string]any)
	if !ok {
		return ""
	}

	text, ok := renderer["text"].(map[string]any)
	if !ok {
		return ""
	}

	runs, ok := text["runs"].([]any)
	if !ok || len(runs) == 0 {
		return ""
	}

	firstRun, ok := runs[0].(map[string]any)
	if !ok {
		return ""
	}

	result, _ := firstRun["text"].(string)
	return result
}

func (c *Client) GetUserPlaylists(_ string) ([]Playlist, error) {
	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated to YouTube Music")
	}

	c.logger.Debug("Getting user playlists from YouTube Music")

	reqBody := struct {
		Context  contextPayload `json:"context"`
		BrowseID string         `json:"browseId"`
	}{
		Context:  newContext(),
		BrowseID: "FEmusic_liked_playlists",
	}

	data, err := c.doRequest("browse", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to get user playlists: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse playlists response: %w", err)
	}

	var playlists []Playlist

	contents, ok := result["contents"].(map[string]any)
	if !ok {
		return playlists, nil
	}

	singleColumn, ok := contents["singleColumnBrowseResultsRenderer"].(map[string]any)
	if !ok {
		return playlists, nil
	}

	tabs, ok := singleColumn["tabs"].([]any)
	if !ok || len(tabs) == 0 {
		return playlists, nil
	}

	for _, tab := range tabs {
		tabMap, ok := tab.(map[string]any)
		if !ok {
			continue
		}
		tabContent, ok := tabMap["tabRenderer"].(map[string]any)
		if !ok {
			continue
		}
		content, ok := tabContent["content"].(map[string]any)
		if !ok {
			continue
		}
		sectionList, ok := content["sectionListRenderer"].(map[string]any)
		if !ok {
			continue
		}
		sections, ok := sectionList["contents"].([]any)
		if !ok {
			continue
		}
		for _, section := range sections {
			sectionMap, ok := section.(map[string]any)
			if !ok {
				continue
			}
			shelf, ok := sectionMap["gridRenderer"].(map[string]any)
			if !ok {
				shelf, ok = sectionMap["musicShelfRenderer"].(map[string]any)
				if !ok {
					continue
				}
			}
			items, ok := shelf["items"].([]any)
			if !ok {
				continue
			}
			for _, item := range items {
				playlist := c.parsePlaylistFromRenderer(item)
				if playlist != nil {
					playlists = append(playlists, *playlist)
				}
			}
		}
	}

	c.logger.WithField("playlist_count", len(playlists)).Debug("Retrieved user playlists from YouTube Music")
	return playlists, nil
}

func (c *Client) parsePlaylistFromRenderer(item any) *Playlist {
	itemMap, ok := item.(map[string]any)
	if !ok {
		return nil
	}

	renderer, ok := itemMap["musicTwoRowItemRenderer"].(map[string]any)
	if !ok {
		renderer, ok = itemMap["musicResponsiveListItemRenderer"].(map[string]any)
		if !ok {
			return nil
		}
	}

	title := ""
	playlistID := ""

	if titleObj, ok := renderer["title"].(map[string]any); ok {
		if runs, ok := titleObj["runs"].([]any); ok && len(runs) > 0 {
			if firstRun, ok := runs[0].(map[string]any); ok {
				title, _ = firstRun["text"].(string)
				if navEndpoint, ok := firstRun["navigationEndpoint"].(map[string]any); ok {
					if browseEP, ok := navEndpoint["browseEndpoint"].(map[string]any); ok {
						playlistID, _ = browseEP["browseId"].(string)
					} else if watchEP, ok := navEndpoint["watchEndpoint"].(map[string]any); ok {
						playlistID, _ = watchEP["playlistId"].(string)
					}
				}
			}
		}
	}

	if playlistID == "" || title == "" {
		return nil
	}

	return &Playlist{
		ID:         playlistID,
		Name:       title,
		URI:        fmt.Sprintf("https://music.youtube.com/playlist?list=%s", playlistID),
		EmbedURL:   fmt.Sprintf("https://music.youtube.com/embed/playlist?list=%s", playlistID),
		IsIncoming: false,
	}
}

func (c *Client) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	if !c.IsAuthenticated() {
		return fmt.Errorf("not authenticated to YouTube Music")
	}

	if len(trackIDs) == 0 {
		return fmt.Errorf("no tracks provided to add")
	}

	c.logger.WithFields(logrus.Fields{
		"playlist_id": playlistID,
		"track_count": len(trackIDs),
	}).Debug("Adding tracks to playlist on YouTube Music")

	actions := make([]map[string]any, len(trackIDs))
	for i, trackID := range trackIDs {
		actions[i] = map[string]any{
			"action":       "ACTION_ADD_VIDEO",
			"addedVideoId": trackID,
		}
	}

	reqBody := struct {
		Context    contextPayload   `json:"context"`
		PlaylistID string           `json:"playlistId"`
		Actions    []map[string]any `json:"actions"`
	}{
		Context:    newContext(),
		PlaylistID: playlistID,
		Actions:    actions,
	}

	_, err := c.doRequest("browse/edit_playlist", reqBody)
	if err != nil {
		return fmt.Errorf("failed to add tracks to playlist %s: %w", playlistID, err)
	}

	c.logger.WithFields(logrus.Fields{
		"playlist_id": playlistID,
		"track_count": len(trackIDs),
	}).Debug("Successfully added tracks to playlist on YouTube Music")

	return nil
}

func (c *Client) CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error) {
	if len(trackIDs) == 0 {
		return []bool{}, nil
	}

	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated to YouTube Music")
	}

	c.logger.WithFields(logrus.Fields{
		"playlist_id": playlistID,
		"track_count": len(trackIDs),
	}).Debug("Checking tracks in playlist on YouTube Music")

	reqBody := struct {
		Context  contextPayload `json:"context"`
		BrowseID string         `json:"browseId"`
	}{
		Context:  newContext(),
		BrowseID: playlistID,
	}

	data, err := c.doRequest("browse", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist tracks: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse playlist response: %w", err)
	}

	existingTracks := make(map[string]bool)

	contents, ok := result["contents"].(map[string]any)
	if ok {
		if singleColumn, ok := contents["singleColumnBrowseResultsRenderer"].(map[string]any); ok {
			if tabs, ok := singleColumn["tabs"].([]any); ok && len(tabs) > 0 {
				if tab, ok := tabs[0].(map[string]any); ok {
					if tabContent, ok := tab["tabRenderer"].(map[string]any); ok {
						if content, ok := tabContent["content"].(map[string]any); ok {
							if sectionList, ok := content["sectionListRenderer"].(map[string]any); ok {
								if sections, ok := sectionList["contents"].([]any); ok {
									for _, section := range sections {
										if sectionMap, ok := section.(map[string]any); ok {
											if shelf, ok := sectionMap["musicShelfRenderer"].(map[string]any); ok {
												if items, ok := shelf["contents"].([]any); ok {
													for _, item := range items {
														if track := c.parseTrackFromRenderer(item); track != nil {
															existingTracks[track.ID] = true
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	results := make([]bool, len(trackIDs))
	for i, trackID := range trackIDs {
		results[i] = existingTracks[trackID]
	}

	return results, nil
}

func (c *Client) CreatePlaylist(name, description string, public bool) (*Playlist, error) {
	if !c.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated to YouTube Music")
	}

	c.logger.WithFields(logrus.Fields{
		"playlist_name": name,
		"description":   description,
		"public":        public,
	}).Debug("Creating playlist on YouTube Music")

	privacyStatus := "PRIVATE"
	if public {
		privacyStatus = "PUBLIC"
	}

	reqBody := struct {
		Context       contextPayload `json:"context"`
		Title         string         `json:"title"`
		Description   string         `json:"description,omitempty"`
		PrivacyStatus string         `json:"privacyStatus"`
	}{
		Context:       newContext(),
		Title:         name,
		Description:   description,
		PrivacyStatus: privacyStatus,
	}

	data, err := c.doRequest("playlist/create", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create playlist %s: %w", name, err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse create playlist response: %w", err)
	}

	playlistID, _ := result["playlistId"].(string)
	if playlistID == "" {
		return nil, fmt.Errorf("failed to create playlist: no playlist ID in response")
	}

	playlist := &Playlist{
		ID:         playlistID,
		Name:       name,
		URI:        fmt.Sprintf("https://music.youtube.com/playlist?list=%s", playlistID),
		EmbedURL:   fmt.Sprintf("https://music.youtube.com/embed/playlist?list=%s", playlistID),
		IsIncoming: false,
	}

	c.logger.WithFields(logrus.Fields{
		"playlist_id":   playlist.ID,
		"playlist_name": playlist.Name,
	}).Info("Successfully created playlist on YouTube Music")

	return playlist, nil
}

type authState struct {
	Cookie string `json:"cookie"`
}

func (c *Client) saveAuthState() error {
	if c.tokenFile == "" {
		return nil
	}

	state := authState{Cookie: c.cookie}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal auth state: %w", err)
	}

	tempFile := c.tokenFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write auth state file: %w", err)
	}

	if err := os.Rename(tempFile, c.tokenFile); err != nil {
		_ = os.Remove(tempFile)
		return fmt.Errorf("failed to rename auth state file: %w", err)
	}

	c.logger.Debug("Successfully saved auth state to file")
	return nil
}

func (c *Client) loadAuthState() {
	if c.tokenFile == "" {
		return
	}

	data, err := os.ReadFile(c.tokenFile)
	if err != nil {
		if !os.IsNotExist(err) {
			c.logger.WithError(err).Debug("Failed to read auth state file")
		}
		return
	}

	var state authState
	if err := json.Unmarshal(data, &state); err != nil {
		c.logger.WithError(err).Debug("Failed to parse auth state file")
		return
	}

	if state.Cookie != "" && c.cookie == "" {
		c.cookie = state.Cookie
		c.isAuthed = true
		c.logger.Debug("Loaded auth state from file")
	}
}
