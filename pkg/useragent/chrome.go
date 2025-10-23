// Package useragent provides utilities for generating and managing user agent strings.
//
// This package contains functionality for fetching the latest browser versions
// and constructing appropriate user agent strings for web scraping and HTTP requests.
package useragent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
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

// GetLatestChromeUserAgent fetches the latest Chrome version and returns a user agent string for the current OS.
//
// This function queries the Google Chrome Version History API to get the latest stable
// Chrome version for the current runtime OS and constructs a proper user agent string.
// If the API call fails for any reason, it returns a fallback user agent with a recent Chrome version.
//
// The function includes:
//   - 5-second timeout to prevent hanging
//   - Comprehensive error handling
//   - Fallback to a recent Chrome version if API fails
//   - Debug logging for troubleshooting
//   - OS-specific user agent strings
//
// Returns:
//   - string: A properly formatted Chrome user agent string for the current OS
//
// Example:
//
//	userAgent := useragent.GetLatestChromeUserAgent()
//	// On macOS: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"
//	// On Linux: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"
func GetLatestChromeUserAgent() string {
	return GetLatestChromeUserAgentForOS(runtime.GOOS)
}

// GetLatestChromeUserAgentForOS fetches the latest Chrome version and returns a user agent string for the specified OS.
//
// This function queries the Google Chrome Version History API to get the latest stable
// Chrome version for the specified OS and constructs a proper user agent string.
// If the API call fails for any reason, it returns a fallback user agent with a recent Chrome version.
//
// Parameters:
//   - goos: The target operating system (e.g., "darwin", "linux", "windows")
//
// Returns:
//   - string: A properly formatted Chrome user agent string for the specified OS
//
// Example:
//
//	userAgent := useragent.GetLatestChromeUserAgentForOS("linux")
//	// Returns: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"
func GetLatestChromeUserAgentForOS(goos string) string {
	// Get fallback user agent for the specified OS
	fallback := getFallbackUserAgent(goos)

	// Get the Chrome platform identifier for the API
	platform := getChromeAPIplatform(goos)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Query Chrome version API for the specific platform
	apiURL := fmt.Sprintf("https://versionhistory.googleapis.com/v1/chrome/platforms/%s/channels/stable/versions?fields=versions(version)&filter=endtime=none", platform)
	resp, err := client.Get(apiURL)
	if err != nil {
		log.WithError(err).Debugf("Failed to fetch Chrome version for %s, using fallback", goos)
		return fallback
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debugf("Chrome version API returned status %d for %s, using fallback", resp.StatusCode, goos)
		return fallback
	}

	var versionResp ChromeVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&versionResp); err != nil {
		log.WithError(err).Debugf("Failed to decode Chrome version response for %s, using fallback", goos)
		return fallback
	}

	if len(versionResp.Versions) == 0 {
		log.Debugf("No Chrome versions found in API response for %s, using fallback", goos)
		return fallback
	}

	// Get the latest version (first in the list)
	latestVersion := versionResp.Versions[0].Version

	// Construct user agent with latest Chrome version for the specified OS
	userAgent := GetChromeUserAgentWithVersionForOS(latestVersion, goos)

	log.Debugf("Using Chrome user agent for %s with version %s", goos, latestVersion)
	return userAgent
}

// GetChromeUserAgentWithVersion constructs a Chrome user agent string with a specific version for the current OS.
//
// This function allows you to create a Chrome user agent string with a custom version
// while maintaining the proper format for the current runtime OS.
//
// Parameters:
//   - version: The Chrome version string (e.g., "119.0.0.0")
//
// Returns:
//   - string: A properly formatted Chrome user agent string for the current OS
//
// Example:
//
//	userAgent := useragent.GetChromeUserAgentWithVersion("120.0.0.0")
//	// On macOS: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
func GetChromeUserAgentWithVersion(version string) string {
	return GetChromeUserAgentWithVersionForOS(version, runtime.GOOS)
}

// GetChromeUserAgentWithVersionForOS constructs a Chrome user agent string with a specific version for the specified OS.
//
// This function allows you to create a Chrome user agent string with a custom version
// while maintaining the proper format for the specified operating system.
//
// Parameters:
//   - version: The Chrome version string (e.g., "119.0.0.0")
//   - goos: The target operating system (e.g., "darwin", "linux", "windows")
//
// Returns:
//   - string: A properly formatted Chrome user agent string for the specified OS
//
// Example:
//
//	userAgent := useragent.GetChromeUserAgentWithVersionForOS("120.0.0.0", "linux")
//	// Returns: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
func GetChromeUserAgentWithVersionForOS(version, goos string) string {
	switch goos {
	case "linux":
		return fmt.Sprintf("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", version)
	case "darwin":
		return fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", version)
	case "windows":
		return fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", version)
	default:
		// Fallback to Linux user agent for unknown platforms
		return fmt.Sprintf("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", version)
	}
}

// getChromeAPIplatform returns the Chrome Version History API platform identifier for the given OS.
func getChromeAPIplatform(goos string) string {
	switch goos {
	case "linux":
		return "linux"
	case "darwin":
		return "mac"
	case "windows":
		return "win64"
	default:
		// Default to Linux for unknown platforms
		return "linux"
	}
}

// getFallbackUserAgent returns a fallback user agent string for the specified OS.
func getFallbackUserAgent(goos string) string {
	switch goos {
	case "linux":
		return "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"
	case "darwin":
		return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"
	case "windows":
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"
	default:
		// Fallback to Linux user agent for unknown platforms
		return "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"
	}
}
