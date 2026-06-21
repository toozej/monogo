package ghreleases2rss

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/toozej/ghreleases2rss/internal/github"
	"github.com/toozej/ghreleases2rss/internal/miniflux"
	"github.com/toozej/ghreleases2rss/pkg/config"
)

func Run(cmd *cobra.Command, args []string, conf config.Config) {
	// Get Miniflux API URL endpoint and API Key from config
	minifluxAPIKey := conf.MinifluxAPIKey
	minifluxURL := conf.MinifluxURL

	// Get input file from flag
	filePath, _ := cmd.Flags().GetString("file")

	// Get category from flag
	category, _ := cmd.Flags().GetString("category")

	// Get clearCategoryFeeds from flag
	clearCategoryFeeds, _ := cmd.Flags().GetBool("clearCategoryFeeds")

	// Get debug from flag
	debug, _ := cmd.Flags().GetBool("debug")

	// Validate the category if provided
	var categoryID int
	if category != "" {
		var err error
		categoryID, err = miniflux.GetCategoryID(minifluxURL, minifluxAPIKey, category)
		if err != nil {
			log.Fatalf("Error validating category: %v", err)
		}
	}

	// delete all feeds within categoryId if user requested it
	if clearCategoryFeeds {
		feedIds, err := miniflux.GetCategoryFeeds(minifluxURL, minifluxAPIKey, categoryID)
		if err != nil {
			log.Fatalf("Error getting feeds in categoryId %d: %v\n", categoryID, err)
		}
		log.Info("Deleting feeds from categoryId: ", categoryID)
		for _, feedId := range feedIds {
			log.Debug("Deleting feedId ", feedId)
			err := miniflux.DeleteFeed(minifluxURL, minifluxAPIKey, feedId)
			if err != nil {
				log.Errorf("Error deleting feedId %d: %\n ", feedId, err)
			}
		}
	}

	// Open the input file securely (prevent directory traversal)
	file, err := openFileSecurely(filePath)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		repo := strings.TrimSpace(scanner.Text())
		if repo == "" {
			continue
		}

		// Validate and parse the GitHub repository
		releaseFeed, err := github.GetReleaseFeedURL(repo)
		if err != nil {
			log.Printf("Error processing repo '%s': %v", repo, err)
			continue
		}

		// Subscribe to the feed in Miniflux with optional category
		if debug {
			log.Debug("Pretending to subscribe to feed: ", releaseFeed)
		} else {
			err = miniflux.SubscribeToFeed(minifluxURL, minifluxAPIKey, categoryID, releaseFeed)
			if err != nil {
				log.Printf("Failed to subscribe to feed %s: %v", releaseFeed, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error("Error reading file: ", err)
	}
}

// openFileSecurely opens a file with path traversal protection
func openFileSecurely(filePath string) (*os.File, error) {
	// Get current working directory for secure file operations
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting current working directory: %w", err)
	}

	// Resolve absolute path for the file
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("error resolving file path: %w", err)
	}

	// Get absolute path for current directory
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return nil, fmt.Errorf("error resolving current directory: %w", err)
	}

	// Check if the file is within allowed directories (current directory or subdirectories)
	relPath, err := filepath.Rel(absCwd, absFilePath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return nil, fmt.Errorf("file path traversal detected or file outside allowed directory")
	}

	// Open the file - gosec G304 is acceptable here as we have directory traversal protection above
	file, err := os.Open(absFilePath) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}

	return file, nil
}
