// Package useragent provides utilities for generating and managing user agent strings.
//
// This package contains functionality for fetching the latest browser versions
// and constructing appropriate user agent strings for web scraping and HTTP requests.
package useragent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// ChromeVersionResponse represents the response from Chrome version API
type ChromeVersionResponse struct {
	Versions []ChromeVersion `json:"versions"`
}

// ChromeVersion represents a Chrome version entry
type ChromeVersion struct {
	Version string `json:"version"`
}

// GetLatestChromeUserAgent fetches the latest Chrome version and returns a macOS user agent string.
//
// This function queries the Google Chrome Version History API to get the latest stable
// Chrome version for macOS and constructs a proper user agent string. If the API call
// fails for any reason, it returns a fallback user agent with a recent Chrome version.
//
// The function includes:
//   - 5-second timeout to prevent hanging
//   - Comprehensive error handling
//   - Fallback to a recent Chrome version if API fails
//   - Debug logging for troubleshooting
//
// Returns:
//   - string: A properly formatted Chrome on macOS user agent string
//
// Example:
//
//	userAgent := useragent.GetLatestChromeUserAgent()
//	// Returns: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"
func GetLatestChromeUserAgent() string {
	// Fallback user agent with a recent Chrome version
	fallback := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Query Chrome version API
	resp, err := client.Get("https://versionhistory.googleapis.com/v1/chrome/platforms/mac/channels/stable/versions?fields=versions(version)&filter=endtime=none")
	if err != nil {
		log.WithError(err).Debug("Failed to fetch Chrome version, using fallback")
		return fallback
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debugf("Chrome version API returned status %d, using fallback", resp.StatusCode)
		return fallback
	}

	var versionResp ChromeVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&versionResp); err != nil {
		log.WithError(err).Debug("Failed to decode Chrome version response, using fallback")
		return fallback
	}

	if len(versionResp.Versions) == 0 {
		log.Debug("No Chrome versions found in API response, using fallback")
		return fallback
	}

	// Get the latest version (first in the list)
	latestVersion := versionResp.Versions[0].Version

	// Construct user agent with latest Chrome version
	userAgent := fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", latestVersion)

	log.Debugf("Using Chrome user agent with version %s", latestVersion)
	return userAgent
}

// GetChromeUserAgentWithVersion constructs a Chrome user agent string with a specific version.
//
// This function allows you to create a Chrome user agent string with a custom version
// while maintaining the proper format for macOS Chrome browsers.
//
// Parameters:
//   - version: The Chrome version string (e.g., "119.0.0.0")
//
// Returns:
//   - string: A properly formatted Chrome on macOS user agent string
//
// Example:
//
//	userAgent := useragent.GetChromeUserAgentWithVersion("120.0.0.0")
//	// Returns: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
func GetChromeUserAgentWithVersion(version string) string {
	return fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", version)
}
