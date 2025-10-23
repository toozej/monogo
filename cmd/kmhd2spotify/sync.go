// Package cmd provides the sync command implementation for kmhd2spotify.
package cmd

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/kmhd2spotify/internal/search"
	"github.com/toozej/kmhd2spotify/internal/types"
)

// newSyncCmd creates the sync command for synchronizing KMHD playlist with Spotify.
func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync KMHD playlist to Spotify",
		Long: `Sync the latest songs from KMHD jazz radio to your Spotify playlist.
This command fetches songs from the KMHD JSON API and adds them
to your specified Spotify playlist using fuzzy matching.`,
		Run: runSync,
	}

	cmd.Flags().BoolP("continuous", "c", false, "Run continuously, checking for new songs every hour with randomized timing")
	cmd.Flags().DurationP("interval", "i", time.Hour, "Base interval between checks in continuous mode (randomization will be added)")

	return cmd
}

// runSync executes the sync command.
func runSync(cmd *cobra.Command, args []string) {
	continuous, _ := cmd.Flags().GetBool("continuous")
	interval, _ := cmd.Flags().GetDuration("interval")

	if continuous {
		log.WithField("interval", interval).Info("Starting continuous KMHD to Spotify sync operation")
	} else {
		log.Info("Starting single KMHD to Spotify sync operation")
	}

	// Initialize services using configuration
	kmhdScraper, spotifyService, fuzzySongSearcher, err := initializeAllServices()
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize services")
		return
	}

	// Check if Spotify is authenticated, if not, start auth flow
	if !spotifyService.IsAuthenticated() {
		log.Info("Spotify authentication required. Starting authentication flow...")

		err := authenticateSpotify(spotifyService)
		if err != nil {
			log.WithError(err).Fatal("Failed to authenticate with Spotify")
			return
		}

		log.Info("Spotify authentication completed successfully")
	}

	// Get or create target playlist for current month
	targetPlaylist, err := getOrCreateMonthlyPlaylist(spotifyService, conf.Spotify.PlaylistNamePrefix)
	if err != nil {
		log.WithError(err).Error("Failed to get or create monthly playlist")
		return
	}

	log.WithField("playlist", targetPlaylist.Name).Info("Using playlist as sync target")

	// For radio monitoring, we don't need to track "seen songs" across cycles
	// since the same song can legitimately play multiple times and users might want it added each time
	// The Spotify duplicate checking will handle preventing actual duplicates in the playlist

	// Run sync operation
	if continuous {
		runContinuousSync(kmhdScraper, spotifyService, fuzzySongSearcher, targetPlaylist, interval)
	} else {
		// For single sync, use a seenSongs map to avoid processing the same song multiple times within one batch
		seenSongs := make(map[string]bool)
		runSingleSync(kmhdScraper, spotifyService, fuzzySongSearcher, targetPlaylist, seenSongs)
	}
}

// initializeAllServices creates and initializes all required services using configuration
// This function is shared between search and sync commands

// runContinuousSync runs the sync operation continuously at the specified interval with randomization
func runContinuousSync(kmhdScraper types.KMHDScraper, spotifyService types.SpotifyService, fuzzySongSearcher *search.FuzzySongSearcher, targetPlaylist types.Playlist, interval time.Duration) {
	log.Info("🎵 Starting continuous sync mode - monitoring KMHD for new songs...")
	fmt.Printf("🎵 Monitoring KMHD every %v (with randomization) for new songs...\n", interval)
	fmt.Printf("Press Ctrl+C to stop\n\n")

	// Run initial sync
	cycleSeen := make(map[string]bool)
	runSingleSync(kmhdScraper, spotifyService, fuzzySongSearcher, targetPlaylist, cycleSeen)

	// Continue monitoring with randomized intervals
	for {
		// Calculate next sync time with randomization
		nextSyncDuration := calculateNextSyncTime(interval)
		nextSyncTime := time.Now().Add(nextSyncDuration)

		log.WithFields(log.Fields{
			"next_sync_duration": nextSyncDuration,
			"next_sync_time":     nextSyncTime.Format("2006-01-02 15:04:05"),
		}).Info("Scheduled next sync")

		fmt.Printf("⏰ Next sync scheduled for: %s (in %v)\n",
			nextSyncTime.Format("2006-01-02 15:04:05"), nextSyncDuration)

		// Wait for the calculated duration
		time.Sleep(nextSyncDuration)

		log.Debug("Running scheduled sync check")
		// Create a fresh cycle-specific seenSongs map for each cycle
		// Global tracking prevents cross-day duplicates in long-running sessions
		cycleSeen := make(map[string]bool)
		runSingleSync(kmhdScraper, spotifyService, fuzzySongSearcher, targetPlaylist, cycleSeen)
	}
}

// calculateNextSyncTime calculates the next sync duration with randomization
// Adds 0-3600 seconds (0-60 minutes) to the base interval to prevent predictable patterns
func calculateNextSyncTime(baseInterval time.Duration) time.Duration {
	// Add randomization: 0-3600 seconds (0-60 minutes) using crypto/rand
	randomOffset := time.Duration(generateSecureRandomInt(3600)) * time.Second
	return baseInterval + randomOffset
}

// globalSeenSongs tracks songs across all sync cycles to prevent cross-day duplicates
// in long-running sessions. Key format: "artist - title"
var globalSeenSongs = make(map[string]time.Time)

// generateSecureRandomInt generates a cryptographically secure random integer in the range [0, max)
func generateSecureRandomInt(max int64) int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		// Fallback to current time-based offset if crypto/rand fails
		return time.Now().UnixNano() % max
	}
	return n.Int64()
}

// runSingleSync runs a single sync operation
func runSingleSync(kmhdScraper types.KMHDScraper, spotifyService types.SpotifyService, fuzzySongSearcher *search.FuzzySongSearcher, targetPlaylist types.Playlist, seenSongs map[string]bool) {
	// Fetch KMHD playlist from JSON API
	log.Debug("Fetching KMHD playlist from JSON API...")
	songCollection, err := kmhdScraper.ScrapePlaylist()
	if err != nil {
		log.WithError(err).Error("Failed to fetch KMHD playlist from API")
		return
	}

	if len(songCollection.Songs) == 0 {
		log.Debug("No songs found in KMHD playlist")
		return
	}

	// Check for invalid songs
	validSongs := 0
	for _, song := range songCollection.Songs {
		if song.IsValid() {
			validSongs++
		} else {
			log.WithFields(log.Fields{
				"artist":   song.Artist,
				"title":    song.Title,
				"raw_text": song.RawText,
			}).Warn("Invalid song found in API data")
		}
	}

	log.WithFields(log.Fields{
		"total_songs": len(songCollection.Songs),
		"valid_songs": validSongs,
	}).Debug("Song validation completed")

	log.WithField("song_count", len(songCollection.Songs)).Debug("Successfully fetched KMHD playlist")

	// Log details of songs for debugging
	for i, song := range songCollection.Songs {
		log.WithFields(log.Fields{
			"song_index": i,
			"artist":     song.Artist,
			"title":      song.Title,
			"song_key":   fmt.Sprintf("%s - %s", song.Artist, song.Title),
			"played_at":  song.PlayedAt,
		}).Debug("Song details")
	}

	// Process new songs
	newSongs := filterNewSongs(songCollection.Songs, seenSongs)
	if len(newSongs) == 0 {
		log.Debug("No new songs found")
		return
	}

	log.WithField("new_song_count", len(newSongs)).Info("Found new songs to sync")

	// Sync new songs to Spotify
	syncSongsToSpotify(newSongs, spotifyService, fuzzySongSearcher, targetPlaylist, seenSongs)
}

// filterNewSongs returns only songs that haven't been seen in this cycle or globally
// This prevents duplicate processing within the same batch and across multiple days
// in long-running sessions. Spotify duplicate checking provides additional protection.
func filterNewSongs(songs []types.Song, seenSongs map[string]bool) []types.Song {
	var newSongs []types.Song

	log.WithFields(log.Fields{
		"total_songs":       len(songs),
		"cycle_seen_songs":  len(seenSongs),
		"global_seen_songs": len(globalSeenSongs),
	}).Debug("Filtering songs for processing")

	for i, song := range songs {
		// Skip invalid songs (missing artist or title)
		if !song.IsValid() {
			log.WithFields(log.Fields{
				"song_index": i,
				"artist":     song.Artist,
				"title":      song.Title,
				"raw_text":   song.RawText,
			}).Debug("Skipping invalid song")
			continue
		}

		// Create a unique key for the song (artist + title)
		songKey := fmt.Sprintf("%s - %s", song.Artist, song.Title)

		// Check if song was seen in current cycle
		cycleSeenBefore := seenSongs[songKey]

		// Check if song was seen globally (across all cycles)
		globalSeenTime, globalSeenBefore := globalSeenSongs[songKey]

		log.WithFields(log.Fields{
			"song_index":         i,
			"song_key":           songKey,
			"artist":             song.Artist,
			"title":              song.Title,
			"cycle_seen_before":  cycleSeenBefore,
			"global_seen_before": globalSeenBefore,
			"global_seen_time":   globalSeenTime,
		}).Debug("Checking song for filtering")

		// Skip if seen in current cycle or globally
		if cycleSeenBefore {
			log.WithField("song_key", songKey).Debug("Song already seen in this cycle, skipping")
			continue
		}

		if globalSeenBefore {
			log.WithFields(log.Fields{
				"song_key":        songKey,
				"first_seen_time": globalSeenTime,
			}).Debug("Song already seen in previous session, skipping")
			continue
		}

		// Song is new - add to both tracking maps
		newSongs = append(newSongs, song)
		seenSongs[songKey] = true
		globalSeenSongs[songKey] = time.Now()

		log.WithFields(log.Fields{
			"song_key":   songKey,
			"added_time": time.Now(),
		}).Debug("Song marked as new for processing")
	}

	log.WithFields(log.Fields{
		"total_songs": len(songs),
		"new_songs":   len(newSongs),
	}).Debug("Song filtering completed")

	return newSongs
}

// syncSongsToSpotify syncs the API-fetched songs to the specified Spotify playlist
func syncSongsToSpotify(songs []types.Song, spotifyService types.SpotifyService, fuzzySongSearcher *search.FuzzySongSearcher, targetPlaylist types.Playlist, seenSongs map[string]bool) {
	log.WithField("playlist", targetPlaylist.Name).Debug("Starting sync to Spotify playlist")

	syncedCount := 0
	skippedCount := 0

	for i, song := range songs {
		// Log the song found on KMHD before processing
		fmt.Printf("🎵 Found on KMHD: %s\n", song.String())
		log.WithFields(log.Fields{
			"song_number": i + 1,
			"total_songs": len(songs),
			"kmhd_song":   song.String(),
		}).Info("Processing song from KMHD")

		// Search for artist, song, and album on Spotify using the enhanced fuzzy song searcher
		songMatch, err := fuzzySongSearcher.FindBestSongMatchWithAlbum(song.Artist, song.Title, song.Album)
		if err != nil {
			log.WithFields(log.Fields{
				"kmhd_song": song.String(),
				"error":     err.Error(),
			}).Warn("Failed to find song match, skipping song")
			fmt.Printf("   ❌ Could not find song on Spotify: %s\n", err.Error())
			skippedCount++
			continue
		}

		// Skip low confidence matches
		if songMatch.OverallConfidence < 0.5 {
			log.WithFields(log.Fields{
				"kmhd_song":          song.String(),
				"overall_confidence": songMatch.OverallConfidence,
				"artist_confidence":  songMatch.ArtistConfidence,
				"song_confidence":    songMatch.SongConfidence,
			}).Debug("Low confidence match, skipping song")
			fmt.Printf("   ❌ Low confidence match (%.2f), skipping\n", songMatch.OverallConfidence)
			skippedCount++
			continue
		}

		fmt.Printf("   🎯 Found match: %s - %s (artist: %.2f, song: %.2f, overall: %.2f)\n",
			songMatch.Artist.Name, songMatch.Track.Name,
			songMatch.ArtistConfidence, songMatch.SongConfidence, songMatch.OverallConfidence)

		// Use the matched track
		trackIDs := []string{songMatch.Track.ID}

		// Check if tracks are already in playlist
		existing, err := spotifyService.CheckTracksInPlaylist(targetPlaylist.ID, trackIDs)
		if err != nil {
			log.WithFields(log.Fields{
				"playlist": targetPlaylist.Name,
				"error":    err.Error(),
			}).Warn("Failed to check existing tracks, attempting to add anyway")
		} else if len(existing) > 0 && existing[0] {
			log.WithFields(log.Fields{
				"kmhd_song": song.String(),
				"playlist":  targetPlaylist.Name,
			}).Debug("Track already exists in playlist, skipping")
			fmt.Printf("   ⏭️  Track already in playlist: %s\n", songMatch.Track.Name)
			skippedCount++
			continue
		}

		// Add track to playlist
		err = spotifyService.AddTracksToPlaylist(targetPlaylist.ID, trackIDs)
		if err != nil {
			log.WithFields(log.Fields{
				"kmhd_song": song.String(),
				"playlist":  targetPlaylist.Name,
				"error":     err.Error(),
			}).Warn("Failed to add track to playlist")
			fmt.Printf("   ❌ Failed to add to playlist: %s\n", err.Error())
			skippedCount++
			continue
		}

		log.WithFields(log.Fields{
			"kmhd_song": song.String(),
			"playlist":  targetPlaylist.Name,
			"track":     songMatch.Track.Name,
		}).Info("Successfully synced song to Spotify")

		fmt.Printf("   ✅ Added to playlist: %s\n", songMatch.Track.Name)
		syncedCount++
	}

	// Display sync summary
	if len(songs) > 0 {
		fmt.Printf("\n📊 Sync Summary:\n")
		fmt.Printf("   • Songs processed: %d\n", len(songs))
		fmt.Printf("   • Songs synced: %d\n", syncedCount)
		fmt.Printf("   • Songs skipped: %d\n", skippedCount)
		fmt.Printf("   • Target playlist: %s\n", targetPlaylist.Name)
		fmt.Println()
	}
}

// getOrCreateMonthlyPlaylist finds or creates a monthly playlist based on the configured prefix.
// Creates playlists with format: "{prefix}-YYYY-MM" (e.g., "KMHD-2025-10")
// If no prefix is configured, it returns the first existing playlist.
func getOrCreateMonthlyPlaylist(spotifyService types.SpotifyService, playlistNamePrefix string) (types.Playlist, error) {
	// Get user's playlists
	playlists, err := spotifyService.GetUserPlaylists("")
	if err != nil {
		return types.Playlist{}, fmt.Errorf("failed to get user playlists: %w", err)
	}

	// If no playlist name prefix is configured, use the first playlist (backward compatibility)
	if playlistNamePrefix == "" {
		if len(playlists) == 0 {
			return types.Playlist{}, fmt.Errorf("no playlists found and no prefix configured. Please create a playlist or set SPOTIFY_PLAYLIST_NAME_PREFIX")
		}
		log.Warn("No playlist name prefix configured (SPOTIFY_PLAYLIST_NAME_PREFIX), using first playlist")
		return playlists[0], nil
	}

	// Generate current month's playlist name
	now := time.Now()
	monthlyPlaylistName := fmt.Sprintf("%s-%04d-%02d", playlistNamePrefix, now.Year(), now.Month())

	log.WithFields(log.Fields{
		"prefix":                playlistNamePrefix,
		"monthly_playlist_name": monthlyPlaylistName,
		"year":                  now.Year(),
		"month":                 int(now.Month()),
	}).Debug("Generated monthly playlist name")

	// Search for existing monthly playlist
	for _, playlist := range playlists {
		if playlist.Name == monthlyPlaylistName {
			log.WithFields(log.Fields{
				"playlist_id":   playlist.ID,
				"playlist_name": playlist.Name,
			}).Info("Found existing monthly playlist")
			return playlist, nil
		}
	}

	// Monthly playlist doesn't exist, create it
	log.WithFields(log.Fields{
		"playlist_name": monthlyPlaylistName,
		"prefix":        playlistNamePrefix,
	}).Info("Monthly playlist not found, creating new one")

	description := fmt.Sprintf("KMHD jazz radio songs for %s %d. Organize into '%s' folder for better management.", now.Month().String(), now.Year(), playlistNamePrefix)
	newPlaylist, err := spotifyService.CreatePlaylist(monthlyPlaylistName, description, false)
	if err != nil {
		return types.Playlist{}, fmt.Errorf("failed to create monthly playlist '%s': %w", monthlyPlaylistName, err)
	}

	log.WithFields(log.Fields{
		"playlist_id":   newPlaylist.ID,
		"playlist_name": newPlaylist.Name,
		"description":   description,
		"folder_hint":   playlistNamePrefix,
	}).Info("Successfully created new monthly playlist")

	// Log instructions for manual folder organization
	log.WithFields(log.Fields{
		"playlist_name": newPlaylist.Name,
		"folder_name":   playlistNamePrefix,
	}).Info("💡 Tip: In Spotify Desktop, create a folder named '" + playlistNamePrefix + "' and drag this playlist into it for better organization")

	// Also print to console for user visibility
	fmt.Printf("📁 Organization Tip: In Spotify Desktop, create a folder named '%s' and drag the playlist '%s' into it for better organization.\n", playlistNamePrefix, newPlaylist.Name)

	return *newPlaylist, nil
}

// authenticateSpotify handles the OAuth authentication flow by starting a temporary server
func authenticateSpotify(spotifyService types.SpotifyService) error {
	authURL := spotifyService.GetAuthURL()

	log.WithField("auth_url", authURL).Info("Please visit this URL to authenticate with Spotify")
	fmt.Printf("\n🔐 Spotify Authentication Required\n")
	fmt.Printf("Please visit this URL to authenticate:\n%s\n\n", authURL)
	fmt.Printf("Waiting for authentication... (Press Ctrl+C to cancel)\n")

	// Create a channel to signal when authentication is complete
	authComplete := make(chan error, 1)

	// Set up HTTP server to handle the callback
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		handleSpotifyCallback(w, r, spotifyService, authComplete)
	})

	// Use the server configuration from config instead of parsing redirect URI
	// This allows the server to bind to the correct interface in Docker containers
	serverAddr := conf.Server.Address()

	server := &http.Server{
		Addr:              serverAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
	}

	// Start the server in a goroutine
	go func() {
		log.WithField("address", serverAddr).Info("Starting temporary server for OAuth callback")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			authComplete <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for authentication to complete or timeout
	select {
	case err := <-authComplete:
		// Shutdown the server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := server.Shutdown(ctx); shutdownErr != nil {
			log.WithError(shutdownErr).Warn("Error shutting down authentication server")
		}

		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		return nil

	case <-time.After(5 * time.Minute):
		// Timeout after 5 minutes
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := server.Shutdown(ctx); shutdownErr != nil {
			log.WithError(shutdownErr).Warn("Error shutting down authentication server")
		}

		return fmt.Errorf("authentication timeout after 5 minutes")
	}
}

// handleSpotifyCallback handles the OAuth callback from Spotify
func handleSpotifyCallback(w http.ResponseWriter, r *http.Request, spotifyService types.SpotifyService, authComplete chan<- error) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		log.WithField("error", errorParam).Error("Spotify authentication error")
		http.Error(w, "Authentication failed: "+errorParam, http.StatusBadRequest)
		authComplete <- fmt.Errorf("spotify authentication error: %s", errorParam)
		return
	}

	if code == "" {
		log.Error("No authorization code received")
		http.Error(w, "No authorization code received", http.StatusBadRequest)
		authComplete <- fmt.Errorf("no authorization code received")
		return
	}

	// Complete the authentication
	err := spotifyService.CompleteAuth(code, state)
	if err != nil {
		log.WithError(err).Error("Failed to complete Spotify authentication")
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		authComplete <- fmt.Errorf("failed to complete authentication: %w", err)
		return
	}

	// Send success response
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	successHTML := `
		<!DOCTYPE html>
		<html>
		<head>
			<title>Authentication Successful</title>
			<style>
				body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
				.success { color: #28a745; font-size: 24px; margin-bottom: 20px; }
				.message { color: #6c757d; font-size: 16px; }
			</style>
		</head>
		<body>
			<div class="success">✅ Authentication Successful!</div>
			<div class="message">You can now close this window and return to the terminal.</div>
		</body>
		</html>
	`

	if _, err := w.Write([]byte(successHTML)); err != nil {
		log.WithError(err).Warn("Failed to write success response")
	}

	log.Info("Spotify authentication completed successfully via callback")
	authComplete <- nil
}
