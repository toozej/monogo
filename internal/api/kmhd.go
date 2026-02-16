// Package api provides JSON API client functionality for the kmhd2spotify application.
//
// This package contains the KMHDAPIClient which is responsible for fetching
// playlist data from the KMHD JSON API and parsing it into song information.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/kmhd2spotify/internal/types"
	"github.com/toozej/kmhd2spotify/pkg/config"
	"github.com/toozej/kmhd2spotify/pkg/useragent"
)

// KMHDAPIClient handles fetching and parsing of KMHD JSON API data.
type KMHDAPIClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *log.Entry
}

// CompleteTrack represents a track object with full iTunes metadata from the JSON API.
type CompleteTrack struct {
	ID             string `json:"_id"`
	Duration       int    `json:"_duration"`
	StartTime      string `json:"_start_time"`
	ArtistName     string `json:"artistName"`
	TrackName      string `json:"trackName"`
	CollectionName string `json:"collectionName,omitempty"`
	// Additional iTunes metadata fields that may be present
	ArtistID      int    `json:"artistId,omitempty"`
	CollectionID  int    `json:"collectionId,omitempty"`
	TrackID       int    `json:"trackId,omitempty"`
	PreviewURL    string `json:"previewUrl,omitempty"`
	ArtworkURL30  string `json:"artworkUrl30,omitempty"`
	ArtworkURL60  string `json:"artworkUrl60,omitempty"`
	ArtworkURL100 string `json:"artworkUrl100,omitempty"`
	ReleaseDate   string `json:"releaseDate,omitempty"`
	Country       string `json:"country,omitempty"`
	Currency      string `json:"currency,omitempty"`
	PrimaryGenre  string `json:"primaryGenreName,omitempty"`
}

// MinimalTrack represents a track object with only basic fields from the JSON API.
type MinimalTrack struct {
	ID             string `json:"_id"`
	Duration       int    `json:"_duration"`
	StartTime      string `json:"_start_time"`
	ArtistName     string `json:"artistName"`
	TrackName      string `json:"trackName"`
	CollectionName string `json:"collectionName,omitempty"`
}

// APIResponse represents the raw JSON array response from the KMHD API.
type APIResponse []json.RawMessage

// NewKMHDAPIClient creates a new KMHD API client instance.
func NewKMHDAPIClient(cfg config.KMHDConfig) *KMHDAPIClient {
	// Use the API endpoint from config
	apiEndpoint := cfg.APIEndpoint
	if apiEndpoint == "" {
		apiEndpoint = "https://www.kmhd.org/pf/api/v3/content/fetch/playlist"
	}

	// Use HTTP timeout from config
	timeout := time.Duration(cfg.HTTPTimeout) * time.Second
	if cfg.HTTPTimeout <= 0 {
		timeout = 30 * time.Second
	}

	return &KMHDAPIClient{
		baseURL: apiEndpoint,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: log.WithField("component", "kmhd_api_client"),
	}
}

// FetchPlaylist fetches playlist data from the KMHD JSON API for the specified date.
func (c *KMHDAPIClient) FetchPlaylist(date time.Time) (*types.SongCollection, error) {
	c.logger.Info("Fetching playlist from KMHD JSON API")

	// Build the API URL with date parameter
	apiURL := c.buildAPIURL(date)
	c.logger.Debugf("API URL: %s", apiURL)

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		c.logger.WithError(err).Error("Failed to create HTTP request")
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers to mimic a browser request
	userAgentString := useragent.GetLatestChromeUserAgent()
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("User-Agent", userAgentString)
	req.Header.Set("Referer", c.baseURL)

	c.logger.WithFields(log.Fields{
		"user_agent": userAgentString,
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}).Debug("Making API request with platform-appropriate user agent")

	// Make the HTTP request with retry logic for better Docker container reliability
	var resp *http.Response
	var requestDuration time.Duration
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		attemptStart := time.Now()
		resp, err = c.httpClient.Do(req) // #nosec G704 -- URL is hardcoded KMHD API endpoint
		requestDuration = time.Since(attemptStart)

		if err == nil && resp.StatusCode != http.StatusBadGateway && resp.StatusCode != http.StatusGatewayTimeout {
			// Success or non-retryable error
			break
		}

		if resp != nil {
			_ = resp.Body.Close()
		}

		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * 2 * time.Second // Exponential backoff: 2s, 4s
			c.logger.WithFields(log.Fields{
				"attempt":     attempt,
				"max_retries": maxRetries,
				"wait_time":   waitTime,
				"error":       err,
				"status_code": func() int {
					if resp != nil {
						return resp.StatusCode
					}
					return 0
				}(),
			}).Warn("API request failed, retrying...")
			time.Sleep(waitTime)
		}
	}

	if err != nil {
		c.logger.WithFields(log.Fields{
			"duration_ms": requestDuration.Milliseconds(),
			"attempts":    maxRetries,
			"error":       err.Error(),
		}).Error("API request failed after all retries")
		return nil, fmt.Errorf("failed to make HTTP request after %d attempts: %w", maxRetries, err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		c.logger.WithFields(log.Fields{
			"status_code": resp.StatusCode,
			"status":      resp.Status,
			"duration_ms": requestDuration.Milliseconds(),
		}).Error("API returned non-200 status code")
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, resp.Status)
	}

	// Parse the JSON response
	var apiResponse APIResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&apiResponse); err != nil {
		c.logger.WithFields(log.Fields{
			"duration_ms": requestDuration.Milliseconds(),
			"error":       err.Error(),
		}).Error("Failed to decode JSON response from API")
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	c.logger.WithFields(log.Fields{
		"track_count": len(apiResponse),
		"duration_ms": requestDuration.Milliseconds(),
	}).Info("Successfully received API response")

	// Parse the response into a song collection
	collection, err := c.parseResponse(apiResponse)
	if err != nil {
		c.logger.WithError(err).Error("Failed to parse API response into song collection")
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	c.logger.Infof("Successfully parsed %d songs from KMHD API", len(collection.Songs))
	return collection, nil
}

// buildAPIURL constructs the API URL with the date parameter.
func (c *KMHDAPIClient) buildAPIURL(date time.Time) string {
	// Convert to Pacific Time (KMHD's timezone) for consistent API queries
	// This ensures the correct date is used regardless of container timezone
	pacificTZ, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		c.logger.WithError(err).Warn("Failed to load Pacific timezone, using local time")
		pacificTZ = time.Local
	}

	// Convert the date to Pacific Time
	pacificDate := date.In(pacificTZ)

	// Format the date as ISO 8601 with timezone offset
	dateStr := pacificDate.Format("2006-01-02T15:04:05.000-07:00")

	// Create the query parameter
	query := fmt.Sprintf(`{"day":"%s"}`, dateStr)

	// Build the full URL (baseURL now contains the full API endpoint)
	fullURL := fmt.Sprintf("%s?query=%s", c.baseURL, url.QueryEscape(query))

	c.logger.WithFields(log.Fields{
		"original_date": date.Format(time.RFC3339),
		"pacific_date":  pacificDate.Format(time.RFC3339),
		"query_date":    dateStr,
		"full_url":      fullURL,
	}).Debug("Built API URL with Pacific timezone")

	return fullURL
}

// parseResponse parses the JSON API response into a song collection.
func (c *KMHDAPIClient) parseResponse(apiResponse APIResponse) (*types.SongCollection, error) {
	collection := &types.SongCollection{
		Songs:       make([]types.Song, 0, len(apiResponse)),
		LastUpdated: time.Now(),
		Source:      "kmhd_api",
	}

	for i, rawTrack := range apiResponse {
		song, err := c.parseTrackObject(rawTrack)
		if err != nil {
			c.logger.WithError(err).Warnf("Failed to parse track object at index %d, skipping", i)
			continue
		}

		if song != nil && song.IsValid() {
			collection.AddSong(*song)
			c.logger.Debugf("Parsed song: %s", song.String())
		} else if song != nil {
			c.logger.Warnf("Invalid song parsed at index %d: missing required fields (artist: %q, title: %q)",
				i, song.Artist, song.Title)
		}
	}

	return collection, nil
}

// parseTrackObject parses a single track object from the JSON response.
func (c *KMHDAPIClient) parseTrackObject(rawTrack json.RawMessage) (*types.Song, error) {
	// First, try to unmarshal as a complete track object
	var completeTrack CompleteTrack
	if err := json.Unmarshal(rawTrack, &completeTrack); err == nil {
		// Validate required fields
		if completeTrack.ArtistName != "" && completeTrack.TrackName != "" {
			return c.mapTrackToSong(completeTrack.ArtistName, completeTrack.TrackName,
				completeTrack.CollectionName, completeTrack.StartTime, string(rawTrack))
		}
	}

	// If that fails, try as a minimal track object
	var minimalTrack MinimalTrack
	if err := json.Unmarshal(rawTrack, &minimalTrack); err == nil {
		// Validate required fields
		if minimalTrack.ArtistName != "" && minimalTrack.TrackName != "" {
			return c.mapTrackToSong(minimalTrack.ArtistName, minimalTrack.TrackName,
				minimalTrack.CollectionName, minimalTrack.StartTime, string(rawTrack))
		}
	}

	// If both fail, try to extract fields manually from the raw JSON
	var rawMap map[string]interface{}
	if err := json.Unmarshal(rawTrack, &rawMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal track object: %w", err)
	}

	// Extract required fields with type checking
	artistName, _ := rawMap["artistName"].(string)
	trackName, _ := rawMap["trackName"].(string)
	collectionName, _ := rawMap["collectionName"].(string)
	startTime, _ := rawMap["_start_time"].(string)

	// Validate required fields
	if artistName == "" || trackName == "" {
		return nil, fmt.Errorf("missing required fields: artistName=%q, trackName=%q", artistName, trackName)
	}

	return c.mapTrackToSong(artistName, trackName, collectionName, startTime, string(rawTrack))
}

// mapTrackToSong converts JSON track data to the existing types.Song structure.
func (c *KMHDAPIClient) mapTrackToSong(artistName, trackName, collectionName, startTime, rawJSON string) (*types.Song, error) {
	// Create the song object with mapped fields
	song := &types.Song{
		Artist:  strings.TrimSpace(artistName),
		Title:   strings.TrimSpace(trackName),
		Album:   strings.TrimSpace(collectionName),
		RawText: rawJSON,
	}

	// Parse the ISO 8601 timestamp from _start_time field
	if startTime != "" {
		parsedTime, err := c.parseISO8601Timestamp(startTime)
		if err != nil {
			c.logger.WithError(err).Warnf("Failed to parse timestamp %q, using current time", startTime)
			song.PlayedAt = time.Now()
		} else {
			song.PlayedAt = parsedTime
		}
	} else {
		// Use current time if no timestamp is provided
		song.PlayedAt = time.Now()
	}

	return song, nil
}

// parseISO8601Timestamp parses an ISO 8601 timestamp string into time.Time.
func (c *KMHDAPIClient) parseISO8601Timestamp(timestamp string) (time.Time, error) {
	// Common ISO 8601 formats that might be used by the KMHD API
	formats := []string{
		time.RFC3339,                    // "2006-01-02T15:04:05Z07:00"
		time.RFC3339Nano,                // "2006-01-02T15:04:05.999999999Z07:00"
		"2006-01-02T15:04:05.000Z07:00", // With milliseconds
		"2006-01-02T15:04:05.000-07:00", // With milliseconds and timezone offset
		"2006-01-02T15:04:05-07:00",     // Without milliseconds but with timezone
		"2006-01-02T15:04:05Z",          // UTC format
		"2006-01-02T15:04:05.000Z",      // UTC with milliseconds
	}

	// Try each format until one works
	for _, format := range formats {
		if parsedTime, err := time.Parse(format, timestamp); err == nil {
			return parsedTime, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp %q with any known ISO 8601 format", timestamp)
}

// ScrapePlaylist implements the KMHDScraper interface by fetching today's playlist from the JSON API.
// This method provides compatibility with the existing scraper interface.
func (c *KMHDAPIClient) ScrapePlaylist() (*types.SongCollection, error) {
	return c.FetchPlaylist(time.Now())
}

// GetCurrentlyPlaying fetches the currently playing song from the KMHD JSON API.
// This method fetches the current day's playlist and returns the most recent song.
func (c *KMHDAPIClient) GetCurrentlyPlaying() (*types.Song, error) {
	c.logger.Info("Fetching currently playing song from KMHD JSON API")

	// Fetch today's playlist
	collection, err := c.FetchPlaylist(time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current playlist: %w", err)
	}

	// If no songs found, return error
	if len(collection.Songs) == 0 {
		return nil, fmt.Errorf("no songs found in current playlist")
	}

	// Find the most recent song (closest to current time)
	var currentSong *types.Song
	now := time.Now()
	minTimeDiff := time.Duration(24 * time.Hour) // Start with a large duration

	for i := range collection.Songs {
		song := &collection.Songs[i]
		timeDiff := now.Sub(song.PlayedAt)

		// Only consider songs that were played in the past (not future)
		if timeDiff >= 0 && timeDiff < minTimeDiff {
			minTimeDiff = timeDiff
			currentSong = song
		}
	}

	if currentSong == nil {
		// If no past songs found, return the first song as fallback
		currentSong = &collection.Songs[0]
	}

	c.logger.Infof("Found currently playing song: %s", currentSong.String())
	return currentSong, nil
}
