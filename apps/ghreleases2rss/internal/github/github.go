package github

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

var repoPartPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// GetReleaseFeedURL takes a GitHub repo name or URL and returns the RSS feed URL for the releases.
// It supports full URLs, username/repoName, and GHCR container image URLs.
func GetReleaseFeedURL(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	switch {
	// Case for GHCR container image URLs
	case strings.HasPrefix(repo, "ghcr.io/") || strings.HasPrefix(repo, "https://ghcr.io/"):
		parsed := repo
		if !strings.Contains(parsed, "://") {
			parsed = "https://" + parsed
		}
		parsedURL, err := url.ParseRequestURI(parsed)
		if err != nil || parsedURL.Hostname() != "ghcr.io" {
			return "", errors.New("invalid GHCR URL")
		}
		repoParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
		if len(repoParts) != 2 {
			return "", errors.New("invalid GHCR URL format")
		}
		// Remove any image tag (e.g., ":latest")
		repoName := strings.Split(repoParts[1], ":")[0]
		repo = fmt.Sprintf("%s/%s", repoParts[0], repoName)

	// Case for full GitHub URLs
	case strings.HasPrefix(repo, "https://") || strings.HasPrefix(repo, "http://"):
		parsedURL, err := url.ParseRequestURI(repo)
		if err != nil || parsedURL.Scheme != "https" || parsedURL.Hostname() != "github.com" {
			return "", errors.New("invalid GitHub URL")
		}
		parts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
		if len(parts) != 2 {
			return "", errors.New("GitHub URL must identify exactly one owner and repository")
		}
		repo = strings.Join(parts, "/")

	// Case for username/reponame
	case strings.Contains(repo, "/"):
		repoParts := strings.Split(strings.Trim(repo, "/"), "/")
		if len(repoParts) != 2 {
			return "", errors.New("invalid username/reponame format")
		}
		repo = fmt.Sprintf("%s/%s", repoParts[0], repoParts[1])

	// Case for invalid formats or incomplete input
	default:
		return "", errors.New("invalid GitHub repo format, expected username/repoName")
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || !repoPartPattern.MatchString(parts[0]) || !repoPartPattern.MatchString(parts[1]) {
		return "", errors.New("GitHub owner and repository contain invalid characters")
	}

	log.Info("Repo is set to: ", repo)

	// Construct the GitHub releases RSS feed URL
	return fmt.Sprintf("https://github.com/%s/releases.atom", repo), nil
}
