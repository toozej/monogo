package github

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"
)

// GetReleaseFeedURL takes a GitHub repo name or URL and returns the RSS feed URL for the releases.
// It supports full URLs, username/repoName, and GHCR container image URLs.
func GetReleaseFeedURL(repo string) (string, error) {
	var err error
	switch {
	// Case for GHCR container image URLs
	case strings.Contains(repo, "ghcr.io"):
		repoParts := strings.Split(repo, "/")
		if len(repoParts) < 3 {
			return "", errors.New("invalid GHCR URL format")
		}
		// Remove any image tag (e.g., ":latest")
		repoName := strings.Split(repoParts[2], ":")[0]
		repo = fmt.Sprintf("%s/%s", repoParts[1], repoName)

	// Case for full GitHub URLs
	case strings.Contains(repo, "github.com"):
		var parsedURL *url.URL
		parsedURL, err = url.Parse(repo)
		if err != nil || parsedURL.Host != "github.com" {
			return "", errors.New("invalid GitHub URL")
		}
		repo = strings.TrimPrefix(parsedURL.Path, "/")

	// Case for username/reponame
	case strings.Contains(repo, "/"):
		repoParts := strings.Split(repo, "/")
		if len(repoParts) != 2 {
			return "", errors.New("invalid username/reponame format")
		}
		repo = fmt.Sprintf("%s/%s", repoParts[0], repoParts[1])

	// Case for invalid formats or incomplete input
	default:
		parsedURL, _ := url.Parse(repo)
		if !strings.Contains(repo, "/") || parsedURL.Host != "" {
			return "", errors.New("invalid GitHub repo format, expected username/repoName")
		}
	}

	log.Info("Repo is set to: ", repo)

	// Construct the GitHub releases RSS feed URL
	return fmt.Sprintf("https://github.com/%s/releases.atom", repo), nil
}
