// Package cmd provides the search command implementation for kmhd2playlist.
package cmd

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/kmhd2playlist/internal/api"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/search"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/spotify"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/types"
	"github.com/toozej/monogo/apps/kmhd2playlist/internal/youtubemusic"
)

// newSearchCmd creates the search command for searching songs in KMHD playlist.
func newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search for songs in KMHD playlist",
		Long: `Search for songs in the KMHD playlist using fuzzy matching.
This command fetches the current KMHD playlist from the JSON API and searches for songs
matching the provided query using fuzzy string matching.`,
		Args: cobra.ExactArgs(1),
		RunE: runSearch,
	}

	return cmd
}

// runSearch executes the search command.
func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.TrimSpace(args[0])
	if query == "" {
		return fmt.Errorf("search query cannot be empty")
	}

	log.WithField("query", query).Info("Starting KMHD playlist search")

	// Initialize services using configuration
	kmhdAPIClient, err := initializeKMHDAPIClient()
	if err != nil {
		return fmt.Errorf("initialize KMHD API client: %w", err)
	}

	// Fetch KMHD playlist
	log.Info("Fetching KMHD playlist from API...")
	songCollection, err := kmhdAPIClient.ScrapePlaylist()
	if err != nil {
		return fmt.Errorf("fetch KMHD playlist: %w", err)
	}

	if len(songCollection.Songs) == 0 {
		log.Warn("No songs found in KMHD playlist")
		return nil
	}

	log.WithField("song_count", len(songCollection.Songs)).Info("Successfully fetched KMHD playlist")

	// Search for the query in scraped songs
	matches := searchSongs(songCollection.Songs, query)

	if len(matches) == 0 {
		log.WithField("query", query).Warn("No matching songs found")
		return nil
	}

	// Display results
	displaySearchResults(matches, query)
	return nil
}

// initializeKMHDAPIClient creates and initializes the KMHD API client using configuration
func initializeKMHDAPIClient() (*api.KMHDAPIClient, error) {
	// Initialize KMHD API client
	kmhdAPIClient := api.NewKMHDAPIClient(conf.KMHD)
	return kmhdAPIClient, nil
}

// initializeAllServices creates and initializes all required services using configuration
func initializeAllServices() (types.KMHDScraper, types.MusicService, *search.FuzzySongSearcher, error) {
	// Create logger
	logger := log.StandardLogger()

	// Initialize KMHD API client (replaces scraper)
	kmhdAPIClient := api.NewKMHDAPIClient(conf.KMHD)

	// Initialize music service based on MUSIC_CLIENT configuration
	var musicService types.MusicService
	var err error
	switch conf.MusicClient {
	case "youtube":
		musicService, err = youtubemusic.NewServiceWithError(conf.YouTubeMusic, logger)
	default:
		musicService, err = spotify.NewServiceWithError(conf.Spotify, logger)
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("initialize %s music client: %w", conf.MusicClient, err)
	}

	// Initialize fuzzy song searcher
	fuzzySongSearcher := search.NewFuzzySongSearcher(musicService, logger)

	return kmhdAPIClient, musicService, fuzzySongSearcher, nil
}

// searchSongs searches for songs matching the query in the song collection
func searchSongs(songs []types.Song, query string) []types.Song {
	var matches []types.Song
	queryLower := strings.ToLower(query)

	for _, song := range songs {
		// Search in artist name
		if strings.Contains(strings.ToLower(song.Artist), queryLower) {
			matches = append(matches, song)
			continue
		}

		// Search in song title
		if strings.Contains(strings.ToLower(song.Title), queryLower) {
			matches = append(matches, song)
			continue
		}

		// Search in album name
		if strings.Contains(strings.ToLower(song.Album), queryLower) {
			matches = append(matches, song)
			continue
		}

		// Search in raw text as fallback
		if strings.Contains(strings.ToLower(song.RawText), queryLower) {
			matches = append(matches, song)
		}
	}

	return matches
}

// displaySearchResults displays the search results in a formatted way
func displaySearchResults(matches []types.Song, query string) {
	fmt.Printf("\n🔍 Search Results for '%s':\n", query)
	fmt.Printf("Found %d matching song(s):\n\n", len(matches))

	for i, song := range matches {
		fmt.Printf("%d. 🎵 %s\n", i+1, song.String())
		if !song.PlayedAt.IsZero() {
			fmt.Printf("   📅 Played: %s\n", song.PlayedAt.Format("Jan 2, 2006 15:04"))
		}
		if song.RawText != "" {
			fmt.Printf("   📝 Raw: %s\n", song.RawText)
		}
		fmt.Println()
	}
}
