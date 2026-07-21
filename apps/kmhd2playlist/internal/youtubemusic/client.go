package youtubemusic

import (
	"bytes"
	"crypto/sha1" // #nosec G505 -- SAPISIDHASH requires SHA-1 to match YouTube Music's auth scheme
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/config"
)

const (
	baseURL     = "https://music.youtube.com/youtubei/v1/"
	apiKey      = "REDACTED_API_KEY" // #nosec G101 -- nosemgrep:generic.secrets.security.detected-generic-api-key.detected-generic-api-key YouTube Music public API key, not a secret
	userAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0"
	contentType = "application/json; charset=utf-8"
	origin      = "https://music.youtube.com"
	referer     = "https://music.youtube.com/"
	clientName  = "WEB_REMIX"
)

type Client struct {
	config        config.YouTubeMusicConfig
	logger        *logrus.Logger
	cookie        string
	authHeaders   http.Header
	clientVersion string
	authSource    string
	isAuthed      bool
	httpClient    *http.Client
	tokenFile     string
	mu            sync.RWMutex
}

type contextPayload struct {
	Client struct {
		Name    string `json:"clientName"`
		Version string `json:"clientVersion"`
		HL      string `json:"hl"`
		GL      string `json:"gl"`
	} `json:"client"`
}

func newContext(clientVersion string) contextPayload {
	ctx := contextPayload{}
	ctx.Client.Name = clientName
	ctx.Client.Version = clientVersion
	ctx.Client.HL = "en"
	ctx.Client.GL = "US"
	return ctx
}

func NewClient(cfg config.YouTubeMusicConfig, logger *logrus.Logger) (*Client, error) {
	authHeaders, err := loadAuthHeaders(cfg.AuthFilePath)
	if err != nil {
		return nil, err
	}

	cookie := cfg.Cookie
	authSource := "legacy_cookie"
	if authHeaders != nil {
		cookie = authHeaders.Get("Cookie")
		authSource = "auth_file"
	}
	if cookie == "" {
		return nil, fmt.Errorf("youtube music cookie is required")
	}

	if _, err := extractSAPISID(cookie); err != nil {
		return nil, fmt.Errorf("invalid youtube music cookie: %w", err)
	}

	tokenFile, err := cfg.GetTokenFilePath()
	if err != nil {
		logger.WithError(err).Warn("Could not determine token file path")
	}
	configuredClientVersion := clientVersionFromHeaders(authHeaders)

	client := &Client{
		config:        cfg,
		logger:        logger,
		cookie:        cookie,
		authHeaders:   authHeaders,
		clientVersion: configuredClientVersion,
		authSource:    authSource,
		isAuthed:      true,
		httpClient:    &http.Client{},
		tokenFile:     tokenFile,
	}

	client.loadAuthState()

	logger.WithFields(logrus.Fields{
		"auth_source":        client.authSource,
		"has_cookie":         cookie != "",
		"cookie_length":      len(cookie),
		"sapisid_found":      cookieHasKey(cookie, "SAPISID"),
		"sec3papisid":        cookieHasKey(cookie, "__Secure-3PAPISID"),
		"client_version":     client.clientVersion,
		"visitor_id_present": client.authHeaders.Get("X-Goog-Visitor-Id") != "",
		"token_file":         tokenFile,
		"token_writable":     tokenFileWritable(tokenFile),
	}).Info("YouTube Music client initialized")

	return client, nil
}

func loadAuthHeaders(path string) (http.Header, error) {
	if path == "" {
		return nil, nil
	}

	data, err := readAuthFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read YouTube Music auth file %q: %w", path, err)
	}

	var rawHeaders map[string]string
	if err := json.Unmarshal(data, &rawHeaders); err != nil {
		return nil, fmt.Errorf("failed to parse YouTube Music auth file %q: %w", path, err)
	}

	headers := make(http.Header, len(rawHeaders))
	for name, value := range rawHeaders {
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name != "" && value != "" && isForwardedAuthHeader(name) {
			headers.Set(name, value)
		}
	}

	if headers.Get("Cookie") == "" {
		return nil, fmt.Errorf("YouTube Music auth file %q does not contain a Cookie header", path)
	}
	if headers.Get("X-Goog-AuthUser") == "" {
		return nil, fmt.Errorf("YouTube Music auth file %q does not contain an X-Goog-AuthUser header", path)
	}

	return headers, nil
}

// readAuthFile reads a single auth file through an os.Root scoped to its
// containing directory. Root-relative reads prevent a symlink or path change
// from escaping that directory between path resolution and file access.
func readAuthFile(path string) ([]byte, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve auth file path: %w", err)
	}

	root, err := os.OpenRoot(filepath.Dir(absPath))
	if err != nil {
		return nil, fmt.Errorf("open auth file directory: %w", err)
	}
	defer func() { _ = root.Close() }()

	data, err := root.ReadFile(filepath.Base(absPath))
	if err != nil {
		return nil, err
	}

	return data, nil
}

// isForwardedAuthHeader excludes hop-by-hop and browser-only headers that are
// tied to the original request body or transport. The remaining headers retain
// the browser identity that YouTube Music requires for account mutations.
func isForwardedAuthHeader(name string) bool {
	canonicalName := http.CanonicalHeaderKey(name)
	if strings.HasPrefix(canonicalName, "Sec-") {
		return false
	}

	switch canonicalName {
	case "Host", "Content-Length", "Accept-Encoding":
		return false
	default:
		return true
	}
}

func clientVersionFromHeaders(headers http.Header) string {
	if version := strings.TrimSpace(headers.Get("X-YouTube-Client-Version")); version != "" {
		return version
	}

	return "1." + time.Now().UTC().Format("20060102") + ".01.00"
}

// cookieHasKey reports whether the supplied Cookie header string contains a
// `name=value` pair for the given cookie name.
func cookieHasKey(cookie, name string) bool {
	parts := strings.Split(cookie, ";")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == name && strings.TrimSpace(kv[1]) != "" {
			return true
		}
	}
	return false
}

// tokenFileWritable peforms a quick check for whether the token file path can
// be written by the running process. Returns false in production paths where
// the parent directory does not exist (e.g. read-only filesystem with no
// mounted volume inside the Docker container).
func tokenFileWritable(path string) bool {
	if path == "" {
		return false
	}

	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()

	// Attempt to create and remove a probe file (best effort).
	const probeFile = ".kmhd2playlist-write-probe"
	f, err := root.OpenFile(probeFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = root.Remove(probeFile)
	return true
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

	for name, values := range c.authHeaders {
		req.Header[name] = append([]string(nil), values...)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if req.Header.Get("Origin") == "" {
		req.Header.Set("Origin", origin)
	}
	if req.Header.Get("Referer") == "" {
		req.Header.Set("Referer", referer)
	}
	if req.Header.Get("Cookie") == "" {
		req.Header.Set("Cookie", c.cookie)
	}
	if req.Header.Get("X-Goog-AuthUser") == "" {
		req.Header.Set("X-Goog-AuthUser", "0")
	}
	if req.Header.Get("X-Origin") == "" {
		req.Header.Set("X-Origin", req.Header.Get("Origin"))
	}
	if authHeader, err := c.authorizationHeader(); err != nil {
		c.logger.WithError(err).Debug("Skipping Authorization header for YouTube Music request")
	} else {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		c.mu.Lock()
		c.isAuthed = false
		c.mu.Unlock()
		responseBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			c.logger.WithError(readErr).Warn("Could not read YouTube Music unauthorized response")
		}
		cookieLen := len(c.cookie)
		hasS := cookieHasKey(c.cookie, "SAPISID")
		has3P := cookieHasKey(c.cookie, "__Secure-3PAPISID")
		c.logger.WithFields(logrus.Fields{
			"endpoint":              endpoint,
			"cookie_set":            cookieLen > 0,
			"cookie_length":         cookieLen,
			"sapisid_present":       hasS,
			"sec3papisid":           has3P,
			"auth_source":           c.authSource,
			"client_name":           clientName,
			"client_version":        c.clientVersion,
			"youtube_error_message": responseErrorMessage(responseBody),
		}).Warn("YouTube Music rejected the request as unauthorized")
		advice := "cookie may be invalid or expired"
		if cookieLen == 0 {
			advice = "authentication cookie is empty; provide YOUTUBEMUSIC_AUTH_FILE_PATH with browser request headers or set the legacy YOUTUBEMUSIC_COOKIE value"
		} else if !hasS && !has3P {
			advice = "cookie string does not include SAPISID or __Secure-3PAPISID; YouTube Music requires one of these for authenticated requests"
		}
		return nil, fmt.Errorf("unauthorized: %s", advice)
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

// responseErrorMessage extracts the safe, server-supplied error message from a
// YouTube InnerTube response. It intentionally excludes the full body because
// diagnostic logs must never include authentication material.
func responseErrorMessage(body []byte) string {
	var response struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return ""
	}

	return strings.TrimSpace(response.Error.Message)
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

func (c *Client) authorizationHeader() (string, error) {
	sapisid, err := extractSAPISID(c.cookie)
	if err != nil {
		return "", err
	}

	timestamp := time.Now().Unix()
	requestOrigin := c.authHeaders.Get("X-Origin")
	if requestOrigin == "" {
		requestOrigin = c.authHeaders.Get("Origin")
	}
	if requestOrigin == "" {
		requestOrigin = origin
	}
	payload := fmt.Sprintf("%d %s %s", timestamp, sapisid, requestOrigin)
	sum := sha1.Sum([]byte(payload)) // #nosec G401 -- required by YouTube Music SAPISIDHASH auth // nosemgrep:go.lang.security.audit.crypto.use_of_weak_crypto.use-of-sha1
	return fmt.Sprintf("SAPISIDHASH %d_%x", timestamp, sum), nil
}

func extractSAPISID(cookie string) (string, error) {
	cookies := parseCookieString(cookie)
	if len(cookies) == 0 {
		return "", fmt.Errorf("cookie string is empty or malformed")
	}

	if value := cookies["SAPISID"]; value != "" {
		return value, nil
	}
	if value := cookies["__Secure-3PAPISID"]; value != "" {
		return value, nil
	}

	return "", fmt.Errorf("SAPISID or __Secure-3PAPISID not found")
}

func parseCookieString(cookie string) map[string]string {
	parts := strings.Split(cookie, ";")
	result := make(map[string]string, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		result[key] = value
	}

	return result
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
		Context: newContext(c.clientVersion),
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
		Context:  newContext(c.clientVersion),
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
		Context:  newContext(c.clientVersion),
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
		Context:    newContext(c.clientVersion),
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
		Context:  newContext(c.clientVersion),
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
		Context:       newContext(c.clientVersion),
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
