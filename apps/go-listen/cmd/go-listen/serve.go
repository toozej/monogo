package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/toozej/go-listen/internal/server"
	"github.com/toozej/go-listen/internal/services/scraper"
	"github.com/toozej/go-listen/internal/services/search"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the web server",
	Long:  `Start the HTTP web server for the go-listen application`,
	Run:   runServeCommand,
}

func runServeCommand(cmd *cobra.Command, args []string) {
	// Initialize logger
	logger := log.New()
	if debug {
		logger.SetLevel(log.DebugLevel)
	}

	// Create server instance
	srv := server.NewServer(&conf)

	// Use the server's authenticated Spotify service and playlist manager
	// instead of creating new instances that won't be authenticated
	spotifyService := srv.GetSpotifyService()
	playlistManager := srv.GetPlaylistManager()
	fuzzySearcher := search.NewFuzzyArtistSearcher(spotifyService, logger)

	// Initialize scraper components
	parser := scraper.NewGoqueryParser(logger)
	extractor := scraper.NewPatternArtistExtractor(logger)

	// Create scraper service using the server's authenticated services
	scraperConfig := scraper.DefaultScraperConfig()
	scraperService := scraper.NewWebScraper(
		scraperConfig,
		parser,
		extractor,
		fuzzySearcher,
		playlistManager,
		logger,
	)

	// Set the scraper service on the server
	srv.SetScraperService(scraperService)

	logger.Info("Server initialized with scraper service using authenticated Spotify service")

	// Start server in a goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Stop(ctx); err != nil {
		log.WithError(err).Error("Server forced to shutdown")
	}

	log.Info("Server exited")
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
