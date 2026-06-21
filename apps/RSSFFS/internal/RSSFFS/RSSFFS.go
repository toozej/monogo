package RSSFFS

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"net/url"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/RSSFFS/pkg/config"
	"golang.org/x/net/html"
)

// Category struct to unmarshal the JSON response
type Category struct {
	Title  string `json:"title"`
	UserID int    `json:"user_id"`
	ID     int    `json:"id"`
}

var (
	apiEndpoint string
	apiKey      string
)

var commonPatterns = []string{"/index.xml", "/feed", "/feed.xml", "/rss", "/rss.xml", "/atom.xml", "/?format=rss"}

const maxRedirects = 10
const timeoutSeconds = 10

// validateURL validates that a URL is safe to request and not targeting internal networks
func validateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Parse the URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %v", err)
	}

	// Only allow HTTP and HTTPS schemes
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only HTTP and HTTPS schemes are allowed, got: %s", u.Scheme)
	}

	// Get the hostname
	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("no hostname found in URL")
	}

	// Resolve the hostname to IP addresses
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %s: %v", hostname, err)
	}

	// Check if any resolved IP is in a private/internal range
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("requests to private/internal IP addresses are not allowed: %s resolves to %s", hostname, ip.String())
		}
	}

	return nil
}

// isPrivateIP checks if an IP address is in a private/internal range
func isPrivateIP(ip net.IP) bool {
	// Check for IPv4 private ranges
	if ip.To4() != nil {
		// 10.0.0.0/8
		if ip[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip[0] == 192 && ip[1] == 168 {
			return true
		}
		// 127.0.0.0/8 (localhost)
		if ip[0] == 127 {
			return true
		}
		// 169.254.0.0/16 (link-local)
		if ip[0] == 169 && ip[1] == 254 {
			return true
		}
	}

	// Check for IPv6 private ranges
	if ip.To16() != nil {
		// ::1 (localhost)
		if ip.Equal(net.IPv6loopback) {
			return true
		}
		// fe80::/10 (link-local)
		if ip[0] == 0xfe && (ip[1]&0xc0) == 0x80 {
			return true
		}
		// fc00::/7 (unique local)
		if (ip[0] & 0xfe) == 0xfc {
			return true
		}
	}

	return false
}

// extractDomainFromURL extracts the domain from a URL, handling various formats and edge cases
func extractDomainFromURL(pageURL string) (string, error) {
	if pageURL == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	// Handle URLs without protocol
	originalURL := pageURL
	if !strings.HasPrefix(pageURL, "http://") && !strings.HasPrefix(pageURL, "https://") {
		pageURL = "https://" + pageURL
	}

	u, err := url.Parse(pageURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format '%s': %v", originalURL, err)
	}

	hostname := u.Hostname()
	if hostname == "" {
		return "", fmt.Errorf("no valid hostname found in URL '%s'", originalURL)
	}

	// Additional validation for common issues
	if strings.Contains(hostname, " ") {
		return "", fmt.Errorf("hostname contains spaces: '%s'", hostname)
	}

	if len(hostname) > 253 {
		return "", fmt.Errorf("hostname too long (max 253 characters): '%s'", hostname)
	}

	return hostname, nil
}

// getAllDomainsFromPage retrieves all unique domain names from a webpage
func getAllDomainsFromPage(pageURL string) (map[string]bool, error) {
	// Validate the URL before making the request
	if err := validateURL(pageURL); err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	client := &http.Client{
		Timeout: time.Second * timeoutSeconds,
	}

	resp, err := client.Get(pageURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Errorf("Error closing response body: %v", err)
		}
	}()

	tokenizer := html.NewTokenizer(resp.Body)
	domains := make(map[string]bool)

	// Parse HTML and extract URLs
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return domains, nil
		case html.StartTagToken:
			t := tokenizer.Token()
			if t.Data == "a" {
				for _, attr := range t.Attr {
					if attr.Key == "href" {
						u, err := url.Parse(attr.Val)
						if err == nil && u.Host != "" {
							domain := u.Hostname()
							domains[domain] = true
						}
					}
				}
			}
		}
	}
}

// checkDomainsForRSS checks for RSS feeds on the given domains with concurrency
func checkDomainsForRSS(domains map[string]bool, pageURL string) []string {
	var wg sync.WaitGroup
	feedChan := make(chan string)
	feedMap := make(map[string]bool)
	mu := sync.Mutex{}

	for domain := range domains {
		wg.Add(1)
		go func(domain string) {
			defer wg.Done()
			feed := findPreferredRSSFeed(domain, pageURL)
			if feed != "" {
				mu.Lock()
				if !feedMap[domain] {
					feedMap[domain] = true
					feedChan <- feed
				}
				mu.Unlock()
			}
		}(domain)
	}

	// Close channel when all goroutines are done
	go func() {
		wg.Wait()
		close(feedChan)
	}()

	var validFeeds []string
	for feed := range feedChan {
		validFeeds = append(validFeeds, feed)
	}

	return validFeeds
}

// findPreferredRSSFeed checks RSS patterns for a domain and returns the first valid one based on preference
func findPreferredRSSFeed(domain string, originalURL string) string {
	client := &http.Client{
		Timeout: time.Second * timeoutSeconds,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	log.Debugf("Checking RSS patterns for domain: %s", domain)
	for _, pattern := range commonPatterns {
		feedURL := "https://" + domain + pattern
		log.Debugf("Checking RSS feed URL: %s", feedURL)
		if checkRSSFeed(client, feedURL) {
			log.Debugf("Valid RSS feed found at: %s", feedURL)
			return feedURL
		}
	}

	// Special case for medium.com: if original URL is https://medium.com/$USERNAME, try https://medium.com/feed/$USERNAME
	if domain == "medium.com" && strings.HasPrefix(originalURL, "https://medium.com/") {
		path := strings.TrimPrefix(originalURL, "https://medium.com/")
		if path != "" && !strings.Contains(path, "/") {
			// Assuming it's a username, no further slashes
			specialURL := "https://medium.com/feed/" + path
			log.Debugf("Checking special Medium RSS feed URL: %s", specialURL)
			if checkRSSFeed(client, specialURL) {
				log.Debugf("Valid RSS feed found at special Medium URL: %s", specialURL)
				return specialURL
			}
		}
	}

	log.Debugf("No RSS feeds found for domain: %s", domain)
	return ""
}

// checkRSSFeed checks if the given URL is a valid RSS feed
func checkRSSFeed(client *http.Client, feedURL string) bool {
	// Validate the URL before making the request
	if err := validateURL(feedURL); err != nil {
		log.Debugf("Skipping invalid RSS feed URL %s: %v", feedURL, err)
		return false
	}

	resp, err := client.Get(feedURL)
	if err != nil || resp.StatusCode != 200 {
		return false
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Errorf("Error closing response body: %v", err)
		}
	}()

	// Check if the Content-Type header indicates an RSS feed
	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "xml") || strings.Contains(contentType, "rss")
}

func Run(pageURL string, category string, debug bool, clearCategoryFeeds bool, singleURLMode bool, conf config.Config) (int, error) {
	// Use configuration passed from caller
	apiEndpoint, apiKey = conf.RSSReaderEndpoint, conf.RSSReaderAPIKey

	// Get categoryId of user-input category if it exists
	categoryId, err := getCategoryId(apiEndpoint, apiKey, category)
	if err != nil {
		return 0, fmt.Errorf("error getting categoryId from category %s: %w", category, err)
	}

	// delete all feeds within categoryId if user requested it
	if clearCategoryFeeds {
		feedIds, err := getCategoryFeeds(apiEndpoint, apiKey, categoryId)
		if err != nil {
			return 0, fmt.Errorf("error getting feeds in categoryId %d: %w", categoryId, err)
		}
		log.Info("Deleting feeds from categoryId: ", categoryId)
		for _, feedId := range feedIds {
			log.Debug("Deleting feedId ", feedId)
			err := deleteFeed(apiEndpoint, apiKey, feedId)
			if err != nil {
				log.Errorf("Error deleting feedId %d: %v\n ", feedId, err)
			}
		}
	}

	// Mode selection logic based on CLI flag and environment variable precedence
	// CLI flag takes precedence over environment variable
	useSingleURLMode := singleURLMode || conf.SingleURLMode

	if useSingleURLMode {
		return runSingleURLMode(pageURL, categoryId, debug)
	}
	return runTraversalMode(pageURL, categoryId, debug)
}

// runSingleURLMode implements single URL mode that only checks the provided URL's domain
func runSingleURLMode(pageURL string, categoryId int, debug bool) (int, error) {
	domain, err := extractDomainFromURL(pageURL)
	if err != nil {
		log.Errorf("Single URL mode: Failed to extract domain from URL '%s': %v", pageURL, err)
		log.Errorf("Single URL mode: Please ensure the URL is properly formatted (e.g., https://example.com)")
		return 0, err
	}

	log.Infof("Using single URL mode for domain: %s", domain)
	log.Debugf("Single URL mode: checking common RSS patterns on %s", domain)

	// Use existing RSS detection logic for the target domain
	feed := findPreferredRSSFeed(domain, pageURL)
	if feed != "" {
		log.Infof("Single URL mode: Found RSS feed on %s: %s", domain, feed)
		if debug {
			log.Debugf("Single URL mode: Debug mode enabled - pretending to subscribe to feed: %s", feed)
			return 1, nil
		} else {
			if err := subscribeToFeed(apiEndpoint, apiKey, categoryId, feed); err != nil {
				log.Errorf("Single URL mode: Error subscribing to RSS feed %s: %v", feed, err)
				log.Errorf("Single URL mode: Please check your RSS reader configuration and network connectivity")
				return 0, err
			} else {
				log.Infof("Single URL mode: Successfully subscribed to RSS feed: %s", feed)
				return 1, nil
			}
		}
	} else {
		log.Infof("Single URL mode: No RSS feeds found on domain %s", domain)
		log.Infof("Single URL mode: Checked common RSS patterns: %v", commonPatterns)
		log.Infof("Single URL mode: The website may not have RSS feeds, or they may be located at non-standard paths")
	}
	return 0, nil
}

// runTraversalMode implements the existing traversal mode logic
func runTraversalMode(pageURL string, categoryId int, debug bool) (int, error) {
	log.Info("Using traversal mode, checking all domains found on page")

	// Get all unique domains from the page
	log.Infof("Traversal mode: Getting all unique domains from the URL: %s", pageURL)
	domains, err := getAllDomainsFromPage(pageURL)
	if err != nil {
		return 0, fmt.Errorf("traversal mode: Error fetching page %s: %w", pageURL, err)
	}

	log.Infof("Traversal mode: Found %d unique domains to check for RSS feeds", len(domains))
	if len(domains) == 0 {
		log.Warnf("Traversal mode: No domains found on page %s", pageURL)
		return 0, nil
	}

	// Deduplicate valid RSS feeds
	validFeeds := checkDomainsForRSS(domains, pageURL)

	if len(validFeeds) == 0 {
		log.Infof("Traversal mode: No RSS feeds found across %d domains", len(domains))
		return 0, nil
	}

	log.Infof("Traversal mode: Found %d RSS feeds across %d domains", len(validFeeds), len(domains))

	// Subscribe to valid RSS feeds
	successCount := 0
	for _, feed := range validFeeds {
		if debug {
			log.Debugf("Traversal mode: Debug mode enabled - pretending to subscribe to feed: %s", feed)
			successCount++
		} else {
			if err := subscribeToFeed(apiEndpoint, apiKey, categoryId, feed); err != nil {
				log.Errorf("Traversal mode: Error subscribing to RSS feed %s: %v", feed, err)
			} else {
				log.Infof("Traversal mode: Successfully subscribed to RSS feed: %s", feed)
				successCount++
			}
		}
	}

	log.Infof("Traversal mode: Successfully processed %d out of %d RSS feeds", successCount, len(validFeeds))
	return successCount, nil
}
