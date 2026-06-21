// Package scraper provides web scraping functionality for artist discovery.
//
// This package implements the core web scraping service that fetches HTML content
// from URLs, parses it using CSS selectors, extracts artist names, and integrates
// with the existing Spotify fuzzy matching and playlist management components.
package scraper

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/internal/types"
)

// ScraperService defines the interface for web scraping operations.
type ScraperService interface {
	// ScrapeArtists fetches a URL and extracts potential artist names from the content.
	ScrapeArtists(url, cssSelector string) ([]string, error)

	// ScrapeAndAddToPlaylist performs a complete scraping workflow: fetch URL,
	// extract artists, fuzzy match against Spotify, and add to playlist.
	ScrapeAndAddToPlaylist(url, cssSelector, playlistID string, force bool) (*ScrapeResult, error)
}

// WebScraper implements the ScraperService interface.
type WebScraper struct {
	httpClient       *http.Client
	parser           HTMLParser
	extractor        ArtistExtractor
	searcher         types.ArtistSearcher
	playlist         types.PlaylistManager
	logger           *logrus.Logger
	config           ScraperConfig
	duplicateChecker DuplicateChecker
	trackAdder       TrackAdder
}

// DuplicateChecker is a function type for checking duplicates (allows testing override)
type DuplicateChecker func(playlistID, artistID string) (*types.DuplicateResult, error)

// TrackAdder is a function type for adding tracks to playlist (allows testing override)
type TrackAdder func(playlistID string, trackIDs []string) error

// ScraperConfig holds configuration for the web scraper.
type ScraperConfig struct {
	Timeout        time.Duration
	MaxRetries     int
	RetryBackoff   time.Duration
	UserAgent      string
	MaxContentSize int64
}

// DefaultScraperConfig returns the default scraper configuration.
func DefaultScraperConfig() ScraperConfig {
	return ScraperConfig{
		Timeout:        30 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   2 * time.Second,
		UserAgent:      "go-listen/1.0 (Web Scraper)",
		MaxContentSize: 10 * 1024 * 1024, // 10MB
	}
}

// NewWebScraper creates a new WebScraper instance with the provided dependencies.
func NewWebScraper(
	config ScraperConfig,
	parser HTMLParser,
	extractor ArtistExtractor,
	searcher types.ArtistSearcher,
	playlist types.PlaylistManager,
	logger *logrus.Logger,
) *WebScraper {
	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
			DisableKeepAlives:  false,
		},
	}

	ws := &WebScraper{
		httpClient: httpClient,
		parser:     parser,
		extractor:  extractor,
		searcher:   searcher,
		playlist:   playlist,
		logger:     logger,
		config:     config,
	}

	// Set default implementations
	ws.duplicateChecker = ws.checkDuplicateDefault
	ws.trackAdder = ws.addTracksToPlaylistDefault

	return ws
}

// MinConfidenceThreshold is the minimum confidence score required for a fuzzy match.
const MinConfidenceThreshold = 0.5

// ScrapeArtists fetches a URL and extracts potential artist names.
func (w *WebScraper) ScrapeArtists(url, cssSelector string) ([]string, error) {
	w.logger.WithFields(logrus.Fields{
		"component":    "scraper",
		"operation":    "scrape_start",
		"url":          url,
		"css_selector": cssSelector,
	}).Info("Starting web scraping operation")

	// Fetch HTML content with retry logic
	htmlContent, err := w.fetchWithRetry(url)
	if err != nil {
		w.logger.WithError(err).WithField("url", url).Error("Failed to fetch URL")
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}

	// Parse HTML content
	doc, err := w.parser.Parse(htmlContent)
	if err != nil {
		w.logger.WithError(err).Error("Failed to parse HTML content")
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract text using CSS selector
	text, err := w.parser.ExtractText(doc, cssSelector)
	if err != nil {
		w.logger.WithError(err).WithField("css_selector", cssSelector).Error("Failed to extract text")
		return nil, fmt.Errorf("failed to extract text: %w", err)
	}

	// Extract artist names from text
	artists, err := w.extractor.ExtractArtists(text)
	if err != nil {
		w.logger.WithError(err).Error("Failed to extract artists")
		return nil, fmt.Errorf("failed to extract artists: %w", err)
	}

	w.logger.WithFields(logrus.Fields{
		"component":     "scraper",
		"operation":     "extract_artists",
		"artists_found": len(artists),
		"artists":       artists,
	}).Info("Artists extracted from content")

	return artists, nil
}

// ScrapeAndAddToPlaylist performs the complete scraping workflow.
func (w *WebScraper) ScrapeAndAddToPlaylist(url, cssSelector, playlistID string, force bool) (*ScrapeResult, error) {
	startTime := time.Now()

	w.logger.WithFields(logrus.Fields{
		"component":    "scraper",
		"operation":    "scrape_and_add_start",
		"url":          url,
		"css_selector": cssSelector,
		"playlist_id":  playlistID,
		"force":        force,
	}).Info("Starting complete scraping workflow")

	result := &ScrapeResult{
		URL:         url,
		CSSSelector: cssSelector,
		Errors:      []string{},
	}

	// Step 1: Scrape artists from URL
	artists, err := w.ScrapeArtists(url, cssSelector)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to scrape artists: %v", err)
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}

	result.ArtistsFound = artists

	if len(artists) == 0 {
		result.Message = "No artists found in the scraped content"
		w.logger.Info("No artists found in scraped content")
		return result, nil
	}

	// Step 2: Fuzzy match artists against Spotify
	matchResults := w.matchArtists(artists)
	result.MatchResults = matchResults

	// Step 3: Add matched artists to playlist
	for i := range matchResults {
		matchResult := &matchResults[i]

		// Skip if not matched
		if !matchResult.Matched {
			result.FailureCount++
			continue
		}

		// Check for duplicates unless force flag is set
		if !force && w.playlist != nil {
			dupResult, err := w.duplicateChecker(playlistID, matchResult.Artist.ID)
			if err != nil {
				w.logger.WithError(err).WithFields(logrus.Fields{
					"artist_id":   matchResult.Artist.ID,
					"artist_name": matchResult.Artist.Name,
				}).Warn("Failed to check for duplicates, continuing anyway")
			} else if dupResult != nil && dupResult.HasDuplicates {
				// Mark as duplicate and skip
				matchResult.WasDuplicate = true
				matchResult.Error = "Artist already in playlist"
				result.DuplicateCount++
				w.logger.WithFields(logrus.Fields{
					"artist_id":   matchResult.Artist.ID,
					"artist_name": matchResult.Artist.Name,
					"playlist_id": playlistID,
				}).Info("Skipping duplicate artist")
				continue
			}
		}

		// Get top 5 tracks for the artist
		tracks, err := w.playlist.GetTop5Tracks(matchResult.Artist.ID)
		if err != nil {
			matchResult.Error = fmt.Sprintf("Failed to get tracks: %v", err)
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Artist %s: %v", matchResult.Artist.Name, err))
			w.logger.WithError(err).WithField("artist_id", matchResult.Artist.ID).Error("Failed to get top tracks")
			continue
		}

		// Add tracks to playlist
		trackIDs := make([]string, len(tracks))
		for i, track := range tracks {
			trackIDs[i] = track.ID
		}

		err = w.trackAdder(playlistID, trackIDs)
		if err != nil {
			matchResult.Error = fmt.Sprintf("Failed to add tracks: %v", err)
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Artist %s: %v", matchResult.Artist.Name, err))
			w.logger.WithError(err).WithFields(logrus.Fields{
				"artist_id":   matchResult.Artist.ID,
				"playlist_id": playlistID,
			}).Error("Failed to add tracks to playlist")
			continue
		}

		// Success!
		matchResult.TracksAdded = len(tracks)
		result.SuccessCount++
		result.TotalTracksAdded += len(tracks)

		w.logger.WithFields(logrus.Fields{
			"artist_id":    matchResult.Artist.ID,
			"artist_name":  matchResult.Artist.Name,
			"tracks_added": len(tracks),
			"playlist_id":  playlistID,
		}).Info("Successfully added artist tracks to playlist")
	}

	// Build summary message
	duration := time.Since(startTime)
	result.Message = fmt.Sprintf("Scraping complete: %d artists found, %d matched, %d added, %d duplicates, %d failed",
		len(result.ArtistsFound), w.countMatched(result.MatchResults), result.SuccessCount, result.DuplicateCount, result.FailureCount)

	w.logger.WithFields(logrus.Fields{
		"component":       "scraper",
		"operation":       "scrape_complete",
		"url":             url,
		"artists_found":   len(result.ArtistsFound),
		"success_count":   result.SuccessCount,
		"failure_count":   result.FailureCount,
		"duplicate_count": result.DuplicateCount,
		"total_tracks":    result.TotalTracksAdded,
		"duration_ms":     duration.Milliseconds(),
	}).Info("Web scraping operation completed")

	return result, nil
}

// fetchWithRetry fetches a URL with exponential backoff retry logic.
func (w *WebScraper) fetchWithRetry(url string) (string, error) {
	var lastErr error
	backoff := w.config.RetryBackoff

	for attempt := 0; attempt <= w.config.MaxRetries; attempt++ {
		if attempt > 0 {
			w.logger.WithFields(logrus.Fields{
				"attempt": attempt,
				"backoff": backoff,
				"url":     url,
			}).Info("Retrying HTTP request")
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}

		htmlContent, err := w.fetchURL(url)
		if err == nil {
			return htmlContent, nil
		}

		lastErr = err
		w.logger.WithError(err).WithFields(logrus.Fields{
			"attempt": attempt + 1,
			"max":     w.config.MaxRetries + 1,
			"url":     url,
		}).Warn("HTTP request failed")
	}

	return "", fmt.Errorf("failed after %d attempts: %w", w.config.MaxRetries+1, lastErr)
}

// fetchURL fetches HTML content from a URL.
func (w *WebScraper) fetchURL(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", w.config.UserAgent)

	startTime := time.Now()
	resp, err := w.httpClient.Do(req) // #nosec G704 -- URL is from config, not user input
	duration := time.Since(startTime)

	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	w.logger.WithFields(logrus.Fields{
		"component":      "scraper",
		"operation":      "http_fetch",
		"url":            url,
		"status_code":    resp.StatusCode,
		"content_length": resp.ContentLength,
		"duration_ms":    duration.Milliseconds(),
	}).Info("HTML content fetched")

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request returned status %d", resp.StatusCode)
	}

	// Read response body with size limit
	body := http.MaxBytesReader(nil, resp.Body, w.config.MaxContentSize)
	content, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(content), nil
}

// checkDuplicateDefault is the default implementation for checking duplicates.
func (w *WebScraper) checkDuplicateDefault(playlistID, artistID string) (*types.DuplicateResult, error) {
	// Get the artist's top tracks
	tracks, err := w.playlist.GetTop5Tracks(artistID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tracks for duplicate check: %w", err)
	}

	if len(tracks) == 0 {
		return &types.DuplicateResult{
			HasDuplicates: false,
		}, nil
	}

	// Extract track IDs
	trackIDs := make([]string, len(tracks))
	for i, track := range tracks {
		trackIDs[i] = track.ID
	}

	// Use the playlist manager to check for duplicates
	duplicateResult, err := w.playlist.CheckForDuplicates(playlistID, trackIDs)
	if err != nil {
		w.logger.WithError(err).WithFields(logrus.Fields{
			"playlist_id": playlistID,
			"artist_id":   artistID,
		}).Warn("Failed to check for duplicates, assuming no duplicates")

		// If we can't check, assume no duplicates to allow the operation to continue
		return &types.DuplicateResult{
			HasDuplicates: false,
		}, nil
	}

	return duplicateResult, nil
}

// addTracksToPlaylistDefault is the default implementation for adding tracks to a playlist.
func (w *WebScraper) addTracksToPlaylistDefault(playlistID string, trackIDs []string) error {
	if len(trackIDs) == 0 {
		return fmt.Errorf("no tracks provided to add")
	}

	// Use the playlist manager to add tracks to the playlist
	err := w.playlist.AddTracksToPlaylist(playlistID, trackIDs)
	if err != nil {
		w.logger.WithError(err).WithFields(logrus.Fields{
			"playlist_id": playlistID,
			"track_count": len(trackIDs),
			"track_ids":   trackIDs,
		}).Error("Failed to add tracks to playlist via playlist manager")
		return fmt.Errorf("failed to add tracks to playlist: %w", err)
	}

	w.logger.WithFields(logrus.Fields{
		"playlist_id": playlistID,
		"track_count": len(trackIDs),
		"track_ids":   trackIDs,
	}).Info("Successfully added tracks to playlist via playlist manager")

	return nil
}

// matchArtists performs batch fuzzy matching of artist names against Spotify.
// It filters out low confidence matches and selects the best match for each query.
func (w *WebScraper) matchArtists(artistNames []string) []ArtistMatchResult {
	if len(artistNames) == 0 {
		return []ArtistMatchResult{}
	}

	w.logger.WithField("artist_count", len(artistNames)).Info("Starting batch artist matching")

	results := make([]ArtistMatchResult, 0, len(artistNames))

	for _, query := range artistNames {
		result := w.matchSingleArtist(query)
		results = append(results, result)
	}

	w.logger.WithFields(logrus.Fields{
		"total_queries":  len(artistNames),
		"matched":        w.countMatched(results),
		"low_confidence": w.countLowConfidence(results),
		"failed":         w.countFailed(results),
	}).Info("Completed batch artist matching")

	return results
}

// matchSingleArtist matches a single artist query against Spotify with confidence filtering.
func (w *WebScraper) matchSingleArtist(query string) ArtistMatchResult {
	result := ArtistMatchResult{
		Query:   query,
		Matched: false,
	}

	// Use the fuzzy searcher to find the best match
	artist, confidence, err := w.searcher.FindBestMatch(query)
	if err != nil {
		w.logger.WithError(err).WithField("query", query).Warn("Failed to find artist match")
		result.Error = err.Error()
		return result
	}

	result.Artist = artist
	result.Confidence = confidence

	// Apply confidence threshold filtering
	if confidence < MinConfidenceThreshold {
		w.logger.WithFields(logrus.Fields{
			"component":  "scraper",
			"operation":  "skip_low_confidence",
			"query":      query,
			"artist":     artist.Name,
			"confidence": confidence,
			"threshold":  MinConfidenceThreshold,
		}).Warn("Skipping artist due to low confidence match")
		result.Error = fmt.Sprintf("confidence %.2f below threshold %.2f", confidence, MinConfidenceThreshold)
		return result
	}

	// Log successful match
	w.logger.WithFields(logrus.Fields{
		"component":  "scraper",
		"operation":  "fuzzy_match",
		"query":      query,
		"artist":     artist.Name,
		"confidence": confidence,
	}).Info("Artist matched")

	result.Matched = true
	return result
}

// countMatched counts the number of successfully matched artists.
func (w *WebScraper) countMatched(results []ArtistMatchResult) int {
	count := 0
	for _, r := range results {
		if r.Matched {
			count++
		}
	}
	return count
}

// countLowConfidence counts the number of low confidence matches that were filtered out.
func (w *WebScraper) countLowConfidence(results []ArtistMatchResult) int {
	count := 0
	for _, r := range results {
		if !r.Matched && r.Artist != nil && r.Confidence < MinConfidenceThreshold {
			count++
		}
	}
	return count
}

// countFailed counts the number of failed matches (errors during search).
func (w *WebScraper) countFailed(results []ArtistMatchResult) int {
	count := 0
	for _, r := range results {
		if !r.Matched && r.Artist == nil {
			count++
		}
	}
	return count
}

// ScrapeResult contains the results of a scraping operation.
type ScrapeResult struct {
	URL              string              `json:"url"`
	CSSSelector      string              `json:"css_selector,omitempty"`
	ArtistsFound     []string            `json:"artists_found"`
	MatchResults     []ArtistMatchResult `json:"match_results"`
	SuccessCount     int                 `json:"success_count"`
	FailureCount     int                 `json:"failure_count"`
	DuplicateCount   int                 `json:"duplicate_count"`
	TotalTracksAdded int                 `json:"total_tracks_added"`
	Message          string              `json:"message"`
	Errors           []string            `json:"errors,omitempty"`
}

// ArtistMatchResult contains the result of matching a single artist.
type ArtistMatchResult struct {
	Query        string        `json:"query"`
	Matched      bool          `json:"matched"`
	Artist       *types.Artist `json:"artist,omitempty"`
	Confidence   float64       `json:"confidence"`
	TracksAdded  int           `json:"tracks_added"`
	WasDuplicate bool          `json:"was_duplicate"`
	Error        string        `json:"error,omitempty"`
}

// HTMLParser defines the interface for HTML parsing operations.
type HTMLParser interface {
	Parse(htmlContent string) (*ParsedDocument, error)
	ExtractText(doc *ParsedDocument, cssSelector string) (string, error)
	ValidateSelector(cssSelector string) error
}

// ParsedDocument represents a parsed HTML document.
type ParsedDocument struct {
	// Document holds the goquery document
	Document *goquery.Document
	URL      string
}

// ArtistExtractor defines the interface for extracting artist names from text.
type ArtistExtractor interface {
	ExtractArtists(text string) ([]string, error)
	CleanArtistName(name string) string
}

// PatternArtistExtractor implements the ArtistExtractor interface using multiple extraction strategies.
type PatternArtistExtractor struct {
	logger     *logrus.Logger
	strategies []ExtractionStrategy
}

// ExtractionStrategy defines the interface for different artist extraction strategies.
type ExtractionStrategy interface {
	Extract(text string) []string
}

// NewPatternArtistExtractor creates a new PatternArtistExtractor with all strategies.
func NewPatternArtistExtractor(logger *logrus.Logger) *PatternArtistExtractor {
	return &PatternArtistExtractor{
		logger: logger,
		strategies: []ExtractionStrategy{
			&CommaListStrategy{},
			&QuotedNamesStrategy{},
			&BulletListStrategy{},
			&LineByLineStrategy{},
		},
	}
}

// ExtractArtists extracts potential artist names from text using multiple strategies.
func (p *PatternArtistExtractor) ExtractArtists(text string) ([]string, error) {
	if text == "" {
		return []string{}, nil
	}

	// Collect artists from all strategies
	artistMap := make(map[string]bool)

	for _, strategy := range p.strategies {
		artists := strategy.Extract(text)
		for _, artist := range artists {
			// Clean the artist name
			cleaned := p.CleanArtistName(artist)
			if cleaned != "" {
				artistMap[cleaned] = true
			}
		}
	}

	// Convert map to slice for deduplication
	var uniqueArtists []string
	for artist := range artistMap {
		uniqueArtists = append(uniqueArtists, artist)
	}

	p.logger.WithFields(logrus.Fields{
		"text_length":   len(text),
		"artists_found": len(uniqueArtists),
		"artists":       uniqueArtists,
	}).Debug("Extracted artists from text")

	return uniqueArtists, nil
}

// CleanArtistName removes common non-artist words and cleans up the artist name.
func (p *PatternArtistExtractor) CleanArtistName(name string) string {
	// Trim whitespace
	name = strings.TrimSpace(name)

	// Remove common prefixes and suffixes
	name = strings.TrimPrefix(name, "-")
	name = strings.TrimPrefix(name, "*")
	name = strings.TrimPrefix(name, "•")
	name = strings.TrimPrefix(name, "·")
	name = strings.TrimSpace(name)

	// Filter out common non-artist words
	commonWords := map[string]bool{
		"the":     true,
		"a":       true,
		"an":      true,
		"of":      true,
		"in":      true,
		"at":      true,
		"on":      true,
		"for":     true,
		"is":      true,
		"are":     true,
		"was":     true,
		"were":    true,
		"be":      true,
		"band":    true,
		"bands":   true,
		"music":   true,
		"song":    true,
		"songs":   true,
		"album":   true,
		"albums":  true,
		"track":   true,
		"tracks":  true,
		"local":   true,
		"new":     true,
		"best":    true,
		"top":     true,
		"artist":  true,
		"artists": true,
	}

	// Check if the entire name is a common word
	lowerName := strings.ToLower(name)
	if commonWords[lowerName] {
		return ""
	}

	// If name is too short (likely not an artist)
	if len(name) < 2 {
		return ""
	}

	return name
}

// CommaListStrategy extracts artists from comma-separated lists.
type CommaListStrategy struct{}

// Extract implements the ExtractionStrategy interface for comma-separated lists.
func (c *CommaListStrategy) Extract(text string) []string {
	var artists []string

	// Split by commas
	parts := strings.Split(text, ",")
	for _, part := range parts {
		cleaned := strings.TrimSpace(part)
		if cleaned != "" {
			artists = append(artists, cleaned)
		}
	}

	return artists
}

// LineByLineStrategy extracts artists from line-separated text.
type LineByLineStrategy struct{}

// Extract implements the ExtractionStrategy interface for line-by-line extraction.
func (l *LineByLineStrategy) Extract(text string) []string {
	var artists []string

	// Split by newlines
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		cleaned := strings.TrimSpace(line)
		if cleaned != "" {
			artists = append(artists, cleaned)
		}
	}

	return artists
}

// QuotedNamesStrategy extracts artists from quoted text.
type QuotedNamesStrategy struct{}

// Extract implements the ExtractionStrategy interface for quoted names.
func (q *QuotedNamesStrategy) Extract(text string) []string {
	var artists []string

	// Look for text within double quotes
	inQuote := false
	var current strings.Builder

	for _, char := range text {
		if char == '"' {
			if inQuote {
				// End of quoted section
				quoted := strings.TrimSpace(current.String())
				if quoted != "" {
					artists = append(artists, quoted)
				}
				current.Reset()
			}
			inQuote = !inQuote
		} else if inQuote {
			current.WriteRune(char)
		}
	}

	return artists
}

// BulletListStrategy extracts artists from markdown/HTML bullet lists.
type BulletListStrategy struct{}

// Extract implements the ExtractionStrategy interface for bullet lists.
func (b *BulletListStrategy) Extract(text string) []string {
	var artists []string

	// Split by newlines and look for bullet points
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for various bullet point markers
		switch {
		case strings.HasPrefix(trimmed, "- "):
			artists = append(artists, strings.TrimSpace(trimmed[2:]))
		case strings.HasPrefix(trimmed, "* "):
			artists = append(artists, strings.TrimSpace(trimmed[2:]))
		case strings.HasPrefix(trimmed, "• "):
			artists = append(artists, strings.TrimSpace(trimmed[len("• "):]))
		case strings.HasPrefix(trimmed, "· "):
			artists = append(artists, strings.TrimSpace(trimmed[len("· "):]))
		}
	}

	return artists
}

// GoqueryParser implements the HTMLParser interface using goquery.
type GoqueryParser struct {
	logger *logrus.Logger
}

// NewGoqueryParser creates a new GoqueryParser instance.
func NewGoqueryParser(logger *logrus.Logger) *GoqueryParser {
	return &GoqueryParser{
		logger: logger,
	}
}

// Parse converts an HTML string to a goquery document.
func (g *GoqueryParser) Parse(htmlContent string) (*ParsedDocument, error) {
	if htmlContent == "" {
		return nil, fmt.Errorf("HTML content is empty")
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		g.logger.WithError(err).Error("Failed to parse HTML content")
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	g.logger.Debug("Successfully parsed HTML document")
	return &ParsedDocument{
		Document: doc,
	}, nil
}

// ExtractText extracts text from the document using a CSS selector.
// If cssSelector is empty, extracts text from the entire body.
func (g *GoqueryParser) ExtractText(doc *ParsedDocument, cssSelector string) (string, error) {
	if doc == nil || doc.Document == nil {
		return "", fmt.Errorf("parsed document is nil")
	}

	// If no selector provided, extract from entire body
	if cssSelector == "" {
		text := doc.Document.Find("body").Text()
		g.logger.WithField("text_length", len(text)).Debug("Extracted text from entire body")
		return text, nil
	}

	// Validate the selector first
	if err := g.ValidateSelector(cssSelector); err != nil {
		return "", err
	}

	// Find all matching elements
	selection := doc.Document.Find(cssSelector)

	// Check if selector matched any elements
	if selection.Length() == 0 {
		g.logger.WithField("selector", cssSelector).Warn("CSS selector matched no elements")
		return "", fmt.Errorf("CSS selector '%s' matched no elements", cssSelector)
	}

	// Extract and combine text from all matching elements
	var textParts []string
	selection.Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			textParts = append(textParts, text)
		}
	})

	combinedText := strings.Join(textParts, "\n")
	g.logger.WithFields(logrus.Fields{
		"selector":       cssSelector,
		"elements_found": selection.Length(),
		"text_length":    len(combinedText),
	}).Debug("Extracted text from CSS selector")

	return combinedText, nil
}

// ValidateSelector validates a CSS selector by attempting to use it.
func (g *GoqueryParser) ValidateSelector(cssSelector string) error {
	if cssSelector == "" {
		return nil // Empty selector is valid (means use entire body)
	}

	// Try to parse the selector by creating a temporary document
	// and attempting to use the selector
	tempHTML := "<html><body><div></div></body></html>"
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(tempHTML))
	if err != nil {
		return fmt.Errorf("internal error validating selector: %w", err)
	}

	// Attempt to use the selector - if it's invalid, goquery will panic or return error
	defer func() {
		if r := recover(); r != nil {
			g.logger.WithFields(logrus.Fields{
				"selector": cssSelector,
				"panic":    r,
			}).Error("CSS selector caused panic")
		}
	}()

	// Try to use the selector
	selection := doc.Find(cssSelector)
	if selection == nil {
		return fmt.Errorf("invalid CSS selector: '%s'", cssSelector)
	}

	g.logger.WithField("selector", cssSelector).Debug("CSS selector is valid")
	return nil
}
