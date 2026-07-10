package ghreleases2rss

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/ghreleases2rss/internal/config"
	"github.com/toozej/monogo/apps/ghreleases2rss/internal/github"
	"github.com/toozej/monogo/apps/ghreleases2rss/internal/miniflux"
)

func Run(cmd *cobra.Command, args []string, conf config.Config) error {
	// Get Miniflux API URL endpoint and API Key from config
	minifluxAPIKey := conf.MinifluxAPIKey
	minifluxURL := conf.MinifluxURL

	// Get input file from flag
	filePath, err := cmd.Flags().GetString("file")
	if err != nil {
		return err
	}

	// Get category from flag
	category, err := cmd.Flags().GetString("category")
	if err != nil {
		return err
	}

	// Get clearCategoryFeeds from flag
	clearCategoryFeeds, err := cmd.Flags().GetBool("clearCategoryFeeds")
	if err != nil {
		return err
	}

	// Get debug from flag
	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return err
	}
	if clearCategoryFeeds && category == "" {
		return fmt.Errorf("category is required when clearing feeds")
	}

	// Validate input before performing any destructive remote operation.
	file, err := openFileSecurely(filePath)
	if err != nil {
		return fmt.Errorf("open input file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Validate the category if provided
	var categoryID int
	if category != "" {
		categoryID, err = miniflux.GetCategoryID(minifluxURL, minifluxAPIKey, category)
		if err != nil {
			return fmt.Errorf("validate category %q: %w", category, err)
		}
	}
	var runErrors []error

	// delete all feeds within categoryId if user requested it
	if clearCategoryFeeds {
		feedIDs, err := miniflux.GetCategoryFeeds(minifluxURL, minifluxAPIKey, categoryID)
		if err != nil {
			return fmt.Errorf("get feeds in category %d: %w", categoryID, err)
		}
		log.Info("Deleting feeds from categoryId: ", categoryID)
		for _, feedID := range feedIDs {
			if debug {
				log.Debug("Pretending to delete feedId ", feedID)
				continue
			}
			log.Debug("Deleting feedId ", feedID)
			if err := miniflux.DeleteFeed(minifluxURL, minifluxAPIKey, feedID); err != nil {
				runErrors = append(runErrors, fmt.Errorf("delete feed %d: %w", feedID, err))
			}
		}
	}

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
			runErrors = append(runErrors, fmt.Errorf("process repo %q: %w", repo, err))
			continue
		}

		// Subscribe to the feed in Miniflux with optional category
		if debug {
			log.Debug("Pretending to subscribe to feed: ", releaseFeed)
		} else {
			err = miniflux.SubscribeToFeed(minifluxURL, minifluxAPIKey, categoryID, releaseFeed)
			if err != nil {
				log.Printf("Failed to subscribe to feed %s: %v", releaseFeed, err)
				runErrors = append(runErrors, fmt.Errorf("subscribe to %s: %w", releaseFeed, err))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		runErrors = append(runErrors, fmt.Errorf("read input file: %w", err))
	}
	return errors.Join(runErrors...)
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
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("file path traversal detected or file outside allowed directory")
	}

	root, err := os.OpenRoot(absCwd)
	if err != nil {
		return nil, fmt.Errorf("open current directory root: %w", err)
	}
	defer func() { _ = root.Close() }()
	file, err := root.Open(relPath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}

	return file, nil
}
