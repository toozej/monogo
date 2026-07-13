package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/go-listen/internal/services/playlist"
	"github.com/toozej/monogo/apps/go-listen/internal/services/scraper"
	"github.com/toozej/monogo/apps/go-listen/internal/services/search"
	"github.com/toozej/monogo/apps/go-listen/internal/services/spotify"
)

var (
	scrapeURL   string
	cssSelector string
	playlistID  string
	forceAdd    bool
)

var scrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "Scrape artists from a website and add to playlist",
	Long: `Scrape artist names from a website URL and add their top 5 songs to a playlist.
Optionally use a CSS selector to target specific page sections.

Examples:
  # Scrape from a Reddit post
  go-listen scrape --url "https://reddit.com/r/music/..." --playlist "playlist_id"

  # Scrape with CSS selector
  go-listen scrape --url "https://example.com" --selector "div.content" --playlist "playlist_id"

  # Force add even if duplicates exist
  go-listen scrape --url "https://example.com" --playlist "playlist_id" --force`,
	RunE: runScrapeCommand,
}

func runScrapeCommand(cmd *cobra.Command, args []string) error {
	// Validate required flags
	if scrapeURL == "" {
		return fmt.Errorf("--url flag is required")
	}

	if playlistID == "" {
		return fmt.Errorf("--playlist flag is required")
	}

	// Initialize logger
	logger := slog.Default()

	// Initialize Spotify service
	spotifyService := spotify.NewService(conf.Spotify, logger)

	// Check if authenticated
	if !spotifyService.IsAuthenticated() {
		return fmt.Errorf("not authenticated with Spotify; run 'go-listen serve' and authenticate first")
	}

	// Initialize playlist manager
	playlistManager := playlist.NewService(spotifyService, logger)

	// Initialize fuzzy artist searcher
	fuzzySearcher := search.NewFuzzyArtistSearcher(spotifyService, logger)

	// Initialize scraper components
	parser := scraper.NewGoqueryParser(logger)
	extractor := scraper.NewPatternArtistExtractor(logger)

	// Create scraper service
	scraperConfig := configuredScraper(conf.Scraper)
	scraperService := scraper.NewWebScraper(
		scraperConfig,
		parser,
		extractor,
		fuzzySearcher,
		playlistManager,
		logger,
	)

	// Perform scraping operation
	logger.Info("Starting scraping operation",
		"url", scrapeURL,
		"css_selector", cssSelector,
		"playlist_id", playlistID,
		"force", forceAdd,
	)

	result, err := scraperService.ScrapeAndAddToPlaylist(scrapeURL, cssSelector, playlistID, forceAdd)
	if err != nil {
		logger.Error("Scraping operation failed", "error", err)
		return fmt.Errorf("scraping operation failed: %w", err)
	}

	// Display results
	displayScrapeResults(result)

	// Exit with appropriate code
	if result.FailureCount > 0 && result.SuccessCount == 0 {
		return fmt.Errorf("all artist operations failed")
	}
	return nil
}

func displayScrapeResults(result *scraper.ScrapeResult) {
	fmt.Println("\n=== Scraping Results ===")
	fmt.Printf("URL: %s\n", result.URL)
	if result.CSSSelector != "" {
		fmt.Printf("CSS Selector: %s\n", result.CSSSelector)
	}
	fmt.Println()

	// Summary
	fmt.Printf("Artists Found: %d\n", len(result.ArtistsFound))
	fmt.Printf("Successfully Matched: %d\n", countMatched(result.MatchResults))
	fmt.Printf("Successfully Added: %d\n", result.SuccessCount)
	fmt.Printf("Duplicates Skipped: %d\n", result.DuplicateCount)
	fmt.Printf("Failed: %d\n", result.FailureCount)
	fmt.Printf("Total Tracks Added: %d\n", result.TotalTracksAdded)
	fmt.Println()

	// Detailed results
	if len(result.MatchResults) > 0 {
		fmt.Println("=== Detailed Results ===")
		for _, match := range result.MatchResults {
			displayMatchResult(match)
		}
	}

	// Errors
	if len(result.Errors) > 0 {
		fmt.Println("\n=== Errors ===")
		for _, err := range result.Errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	fmt.Printf("\n%s\n", result.Message)
}

func displayMatchResult(match scraper.ArtistMatchResult) {
	status := "✗ FAILED"

	switch {
	case match.WasDuplicate:
		status = "⊘ DUPLICATE"
	case match.Matched && match.TracksAdded > 0:
		status = "✓ SUCCESS"
	case match.Matched:
		status = "⚠ MATCHED"
	}

	fmt.Printf("[%s] %s", status, match.Query)

	if match.Artist != nil {
		fmt.Printf(" → %s", match.Artist.Name)
		if match.Confidence > 0 {
			fmt.Printf(" (confidence: %.2f)", match.Confidence)
		}
	}

	if match.TracksAdded > 0 {
		fmt.Printf(" - %d tracks added", match.TracksAdded)
	}

	if match.Error != "" {
		fmt.Printf(" - Error: %s", match.Error)
	}

	fmt.Println()
}

func countMatched(results []scraper.ArtistMatchResult) int {
	count := 0
	for _, r := range results {
		if r.Matched {
			count++
		}
	}
	return count
}

func init() {
	// Add flags
	scrapeCmd.Flags().StringVarP(&scrapeURL, "url", "u", "", "Website URL to scrape (required)")
	scrapeCmd.Flags().StringVarP(&cssSelector, "selector", "s", "", "CSS selector for content extraction (optional)")
	scrapeCmd.Flags().StringVarP(&playlistID, "playlist", "p", "", "Playlist ID to add artists to (required)")
	scrapeCmd.Flags().BoolVarP(&forceAdd, "force", "f", false, "Force add even if duplicates exist")

	// Mark required flags
	_ = scrapeCmd.MarkFlagRequired("url")
	_ = scrapeCmd.MarkFlagRequired("playlist")

	// Add to root command
	rootCmd.AddCommand(scrapeCmd)
}
