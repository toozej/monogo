package RSSFFS

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/monogo/apps/RSSFFS/internal/config"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// Category struct to unmarshal the JSON response
type Category struct {
	Title  string `json:"title"`
	UserID int    `json:"user_id"`
	ID     int    `json:"id"`
}

var commonPatterns = []string{"/index.xml", "/feed", "/feed.xml", "/rss", "/rss.xml", "/atom.xml", "/?format=rss"}

const maxRedirects = 10
const timeoutSeconds = 10
const maxPageBytes = 2 << 20
const maxFeedBytes = 2 << 20
const maxDomains = 100
const maxDomainWorkers = 10

const (
	atomNamespace   = "http://www.w3.org/2005/Atom"
	atom03Namespace = "http://purl.org/atom/ns#"
	rdfNamespace    = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	rss1Namespace   = "http://purl.org/rss/1.0/"
)

var lookupIPAddr = net.DefaultResolver.LookupIPAddr
var networkDialer = (&net.Dialer{Timeout: timeoutSeconds * time.Second}).DialContext

// validateURL validates that a URL is safe to request and not targeting internal networks
func validateURL(rawURL string) error {
	return validateURLContext(context.Background(), rawURL)
}

func validateURLContext(ctx context.Context, rawURL string) error {
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
	ips, err := lookupIPAddr(ctx, hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %s: %w", hostname, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("no addresses found for %s", hostname)
	}

	// Check if any resolved IP is in a private/internal range
	for _, ip := range ips {
		if isPrivateIP(ip.IP) {
			return fmt.Errorf("requests to private/internal IP addresses are not allowed: %s resolves to %s", hostname, ip.IP.String())
		}
	}

	return nil
}

// isPrivateIP checks if an IP address is in a private/internal range
func isPrivateIP(ip net.IP) bool {
	return ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() ||
		ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("split dial address: %w", err)
	}
	ips, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	for _, candidate := range ips {
		if isPrivateIP(candidate.IP) {
			return nil, fmt.Errorf("refusing private/internal dial target %s for %s", candidate.IP, host)
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for %s", host)
	}
	var dialErrors []error
	for _, candidate := range ips {
		conn, dialErr := networkDialer(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		dialErrors = append(dialErrors, fmt.Errorf("%s: %w", candidate.IP, dialErr))
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("dial %s: %w", host, errors.Join(dialErrors...))
}

func newSafeHTTPTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// A proxy resolves and dials the target itself, which would bypass the
	// validated and pinned address selected by safeDialContext.
	transport.Proxy = nil
	transport.DialContext = safeDialContext
	return transport
}

func newSafeHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: newSafeHTTPTransport(),
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects")
			}
			return validateURLContext(req.Context(), req.URL.String())
		},
	}
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
func getAllDomainsFromPage(ctx context.Context, pageURL string) (map[string]bool, error) {
	// Validate the URL before making the request
	if err := validateURLContext(ctx, pageURL); err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	client := newSafeHTTPClient(time.Second * timeoutSeconds)
	defer client.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Errorf("Error closing response body: %v", err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch page: unexpected status %s", resp.Status)
	}
	submittedURL, err := url.Parse(pageURL)
	if err != nil {
		return nil, err
	}
	baseURL := submittedURL
	if resp.Request != nil && resp.Request.URL != nil {
		baseURL = resp.Request.URL
	}
	tokenizer := html.NewTokenizer(io.LimitReader(resp.Body, maxPageBytes))
	domains := make(map[string]bool)
	domains[submittedURL.Scheme+"://"+submittedURL.Host] = true
	if len(domains) < maxDomains {
		domains[baseURL.Scheme+"://"+baseURL.Host] = true
	}

	// Parse HTML and extract URLs
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			if tokenizer.Err() != nil && tokenizer.Err() != io.EOF {
				return nil, tokenizer.Err()
			}
			return domains, nil
		case html.StartTagToken:
			t := tokenizer.Token()
			if t.Data == "a" {
				for _, attr := range t.Attr {
					if attr.Key == "href" {
						u, parseErr := url.Parse(attr.Val)
						if parseErr == nil {
							u = baseURL.ResolveReference(u)
							if (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && len(domains) < maxDomains {
								domains[u.Scheme+"://"+u.Host] = true
							}
						}
					}
				}
			}
		}
	}
}

// checkDomainsForRSS checks for RSS feeds on the given domains with concurrency
func checkDomainsForRSS(ctx context.Context, domains map[string]bool, pageURL string) ([]string, error) {
	var wg sync.WaitGroup
	jobs := make(chan string)
	feedChan := make(chan string, len(domains))
	feedMap := make(map[string]bool)
	mu := sync.Mutex{}

	workerCount := maxDomainWorkers
	if len(domains) < workerCount {
		workerCount = len(domains)
	}
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for domain := range jobs {
				feed := findPreferredRSSFeed(ctx, domain, pageURL)
				if feed != "" {
					mu.Lock()
					if !feedMap[feed] {
						feedMap[feed] = true
						feedChan <- feed
					}
					mu.Unlock()
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for domain := range domains {
			select {
			case jobs <- domain:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(feedChan)
	}()

	var validFeeds []string
	for feed := range feedChan {
		validFeeds = append(validFeeds, feed)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return validFeeds, nil
}

// findPreferredRSSFeed checks RSS patterns for a domain and returns the first valid one based on preference
func findPreferredRSSFeed(ctx context.Context, origin string, originalURL string) string {
	client := newSafeHTTPClient(time.Second * timeoutSeconds)
	defer client.CloseIdleConnections()

	log.Debugf("Checking RSS patterns for origin: %s", origin)
	for _, pattern := range commonPatterns {
		feedURL := strings.TrimSuffix(origin, "/") + pattern
		log.Debugf("Checking RSS feed URL: %s", feedURL)
		if checkRSSFeed(ctx, client, feedURL) {
			log.Debugf("Valid RSS feed found at: %s", feedURL)
			return feedURL
		}
	}

	// Special case for medium.com: if original URL is https://medium.com/$USERNAME, try https://medium.com/feed/$USERNAME
	originURL, _ := url.Parse(origin)
	if originURL != nil && originURL.Hostname() == "medium.com" && strings.HasPrefix(originalURL, "https://medium.com/") {
		path := strings.TrimPrefix(originalURL, "https://medium.com/")
		if path != "" && !strings.Contains(path, "/") {
			// Assuming it's a username, no further slashes
			specialURL := "https://medium.com/feed/" + path
			log.Debugf("Checking special Medium RSS feed URL: %s", specialURL)
			if checkRSSFeed(ctx, client, specialURL) {
				log.Debugf("Valid RSS feed found at special Medium URL: %s", specialURL)
				return specialURL
			}
		}
	}

	log.Debugf("No RSS feeds found for origin: %s", origin)
	return ""
}

// checkRSSFeed checks if the given URL is a valid RSS feed
func checkRSSFeed(ctx context.Context, client *http.Client, feedURL string) bool {
	// Validate the URL before making the request
	if err := validateURLContext(ctx, feedURL); err != nil {
		log.Debugf("Skipping invalid RSS feed URL %s: %v", feedURL, err)
		return false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Errorf("Error closing response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return false
	}
	decoder := xml.NewDecoder(io.LimitReader(resp.Body, maxFeedBytes))
	// Honor non-UTF-8 encodings (e.g. ISO-8859-1, windows-1252) that real feeds
	// declare; without a CharsetReader the decoder errors on such declarations
	// and valid feeds would be silently missed during discovery.
	decoder.CharsetReader = charset.NewReaderLabel
	for {
		token, tokenErr := decoder.Token()
		if tokenErr != nil {
			return false
		}
		if start, ok := token.(xml.StartElement); ok {
			return isFeedRoot(decoder, start)
		}
	}
}

func isFeedRoot(decoder *xml.Decoder, root xml.StartElement) bool {
	switch root.Name.Local {
	case "rss":
		return root.Name.Space == ""
	case "feed":
		return root.Name.Space == atomNamespace || root.Name.Space == atom03Namespace
	case "RDF":
		if root.Name.Space != rdfNamespace {
			return false
		}
		for {
			token, err := decoder.Token()
			if err != nil {
				return false
			}
			if start, ok := token.(xml.StartElement); ok && start.Name.Local == "channel" && start.Name.Space == rss1Namespace {
				return true
			}
		}
	default:
		return false
	}
}

func Run(pageURL string, category string, debug bool, clearCategoryFeeds bool, singleURLMode bool, conf config.Config) (int, error) {
	return RunContext(context.Background(), pageURL, category, debug, clearCategoryFeeds, singleURLMode || conf.SingleURLMode, conf)
}

// RunContext discovers and subscribes to feeds while honoring caller cancellation.
// Debug controls logging only; it never changes mutation behavior.
func RunContext(ctx context.Context, pageURL string, category string, debug bool, clearCategoryFeeds bool, singleURLMode bool, conf config.Config) (int, error) {
	_ = debug
	// Get categoryId of user-input category if it exists
	categoryId, err := getCategoryId(ctx, conf.RSSReaderEndpoint, conf.RSSReaderAPIKey, category)
	if err != nil {
		return 0, fmt.Errorf("error getting categoryId from category %s: %w", category, err)
	}
	if category != "" && categoryId == 0 {
		return 0, fmt.Errorf("RSS reader category %q was not found", category)
	}

	feeds, err := discoverFeeds(ctx, pageURL, singleURLMode)
	if err != nil {
		return 0, err
	}
	if clearCategoryFeeds && len(feeds) == 0 {
		return 0, fmt.Errorf("refusing to clear category %q because no replacement feeds were discovered", category)
	}

	// Clear only after replacement discovery has completed successfully.
	if clearCategoryFeeds {
		feedIds, err := getCategoryFeeds(ctx, conf.RSSReaderEndpoint, conf.RSSReaderAPIKey, categoryId)
		if err != nil {
			return 0, fmt.Errorf("error getting feeds in categoryId %d: %w", categoryId, err)
		}
		log.Info("Deleting feeds from categoryId: ", categoryId)
		for _, feedId := range feedIds {
			log.Debug("Deleting feedId ", feedId)
			err := deleteFeed(ctx, conf.RSSReaderEndpoint, conf.RSSReaderAPIKey, feedId)
			if err != nil {
				return 0, fmt.Errorf("delete feedId %d: %w", feedId, err)
			}
		}
	}

	successCount := 0
	var subscriptionErrors []error
	for _, feed := range feeds {
		if err := subscribeToFeed(ctx, conf.RSSReaderEndpoint, conf.RSSReaderAPIKey, categoryId, feed); err != nil {
			subscriptionErrors = append(subscriptionErrors, fmt.Errorf("subscribe to %s: %w", feed, err))
			continue
		}
		successCount++
	}
	if len(subscriptionErrors) > 0 {
		return successCount, fmt.Errorf("%d subscription(s) failed: %w", len(subscriptionErrors), errors.Join(subscriptionErrors...))
	}
	return successCount, nil
}

func discoverFeeds(ctx context.Context, pageURL string, singleURLMode bool) ([]string, error) {
	if singleURLMode {
		return discoverSingleURLMode(ctx, pageURL)
	}
	return discoverTraversalMode(ctx, pageURL)
}

func discoverSingleURLMode(ctx context.Context, pageURL string) ([]string, error) {
	parsed, err := normalizeHTTPURL(pageURL)
	if err != nil {
		log.Errorf("Single URL mode: Failed to extract domain from URL '%s': %v", pageURL, err)
		log.Errorf("Single URL mode: Please ensure the URL is properly formatted (e.g., https://example.com)")
		return nil, err
	}
	domain := parsed.Hostname()

	log.Infof("Using single URL mode for domain: %s", domain)
	log.Debugf("Single URL mode: checking common RSS patterns on %s", domain)

	normalizedURL := parsed.String()
	origin := parsed.Scheme + "://" + parsed.Host
	feed := findPreferredRSSFeed(ctx, origin, normalizedURL)
	if feed != "" {
		log.Infof("Single URL mode: Found RSS feed on %s: %s", domain, feed)
		return []string{feed}, nil
	} else {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		log.Infof("Single URL mode: No RSS feeds found on domain %s", domain)
		log.Infof("Single URL mode: Checked common RSS patterns: %v", commonPatterns)
		log.Infof("Single URL mode: The website may not have RSS feeds, or they may be located at non-standard paths")
	}
	return nil, nil
}

func normalizeHTTPURL(rawURL string) (*url.URL, error) {
	normalized := rawURL
	if !strings.Contains(normalized, "://") {
		normalized = "https://" + normalized
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return nil, err
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("only HTTP and HTTPS schemes are allowed, got: %s", parsed.Scheme)
	}
	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("no hostname found in URL")
	}
	return parsed, nil
}

func discoverTraversalMode(ctx context.Context, pageURL string) ([]string, error) {
	log.Info("Using traversal mode, checking all domains found on page")

	// Get all unique domains from the page
	log.Infof("Traversal mode: Getting all unique domains from the URL: %s", pageURL)
	domains, err := getAllDomainsFromPage(ctx, pageURL)
	if err != nil {
		return nil, fmt.Errorf("traversal mode: error fetching page %s: %w", pageURL, err)
	}

	log.Infof("Traversal mode: Found %d unique domains to check for RSS feeds", len(domains))
	if len(domains) == 0 {
		log.Warnf("Traversal mode: No domains found on page %s", pageURL)
		return nil, nil
	}

	// Deduplicate valid RSS feeds
	validFeeds, err := checkDomainsForRSS(ctx, domains, pageURL)
	if err != nil {
		return nil, err
	}

	if len(validFeeds) == 0 {
		log.Infof("Traversal mode: No RSS feeds found across %d domains", len(domains))
		return nil, nil
	}

	log.Infof("Traversal mode: Found %d RSS feeds across %d domains", len(validFeeds), len(domains))

	return validFeeds, nil
}
