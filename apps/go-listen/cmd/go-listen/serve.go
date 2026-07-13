package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/go-listen/internal/config"
	"github.com/toozej/monogo/apps/go-listen/internal/server"
	"github.com/toozej/monogo/apps/go-listen/internal/services/scraper"
	"github.com/toozej/monogo/apps/go-listen/internal/services/search"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the web server",
	Long:  `Start the HTTP web server for the go-listen application`,
	RunE:  runServeCommand,
}

func runServeCommand(cmd *cobra.Command, args []string) error {
	if err := conf.ValidateServer(); err != nil {
		return err
	}
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
	scraperConfig := configuredScraper(conf.Scraper)
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
	serverErr := make(chan error, 1)
	go func() { serverErr <- srv.Start() }()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)
	select {
	case <-quit:
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server failed: %w", err)
		}
		return nil
	}

	log.Info("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Stop(ctx); err != nil {
		return fmt.Errorf("stopping server: %w", err)
	}

	log.Info("Server exited")
	return nil
}

func configuredScraper(cfg config.ScraperConfig) scraper.ScraperConfig {
	result := scraper.DefaultScraperConfig()
	if cfg.Timeout > 0 {
		result.Timeout = cfg.Timeout
	}
	if cfg.MaxRetries >= 0 {
		result.MaxRetries = cfg.MaxRetries
	}
	if cfg.RetryBackoff > 0 {
		result.RetryBackoff = cfg.RetryBackoff
	}
	if cfg.UserAgent != "" {
		result.UserAgent = cfg.UserAgent
	}
	if cfg.MaxContentSize > 0 {
		result.MaxContentSize = cfg.MaxContentSize
	}
	result.AllowPrivateNetwork = cfg.AllowPrivateNetwork
	return result
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
