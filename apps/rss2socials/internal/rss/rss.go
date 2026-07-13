// Package rss provides functionality for fetching, parsing, and processing RSS feeds.
// It defines structures for RSS feed data and utilities for HTTP requests and content hashing.
package rss

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	maxFeedBytes = 5 << 20
	maxFeedItems = 1000
)

type RSSFeed struct {
	// RSSFeed represents the structure of an RSS feed as parsed from XML.
	Channel struct {
		Title string    `xml:"title"`
		Items []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	// RSSItem represents a single item from an RSS feed, containing title, link, content, and pubDate.
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	Content string `xml:"description"`
	PubDate string `xml:"pubDate"`
}

// ParsePubDate attempts to parse the item's PubDate field into a time.Time value.
// It tries common RSS date formats including RFC 822 (with and without timezone)
// and RFC 1123. Returns the zero time and an error if parsing fails.
func (item RSSItem) ParsePubDate() (time.Time, error) {
	if item.PubDate == "" {
		return time.Time{}, fmt.Errorf("pubDate is empty")
	}
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, _2 Jan 2006 15:04:05 -0700",
		"Mon, _2 Jan 2006 15:04:05 MST",
		"2 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 MST",
		time.RFC850,
	}
	for _, format := range formats {
		if t, err := time.Parse(format, item.PubDate); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("failed to parse pubDate: %q", item.PubDate)
}

// CheckRSSFeed fetches and parses the RSS feed from the provided URL
func CheckRSSFeed(feedURL string) ([]RSSItem, error) {
	return CheckRSSFeedContext(context.Background(), feedURL)
}

func CheckRSSFeedContext(ctx context.Context, feedURL string) ([]RSSItem, error) {
	// CheckRSSFeed fetches the RSS feed from the given URL, parses it into RSSItems, and returns them.
	// It handles HTTP requests with timeout and XML decoding.
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	parsedURL, err := url.Parse(feedURL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, fmt.Errorf("feed URL must be an absolute HTTP(S) URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create RSS request: %w", err)
	}
	resp, err := client.Do(req) // #nosec G704 -- feed URL is operator configuration validated by the CLI
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read RSS feed: %w", err)
	}
	if len(body) > maxFeedBytes {
		return nil, fmt.Errorf("RSS feed exceeds %d-byte limit", maxFeedBytes)
	}

	var feed RSSFeed
	if err := xml.NewDecoder(bytes.NewReader(body)).Decode(&feed); err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed: %w", err)
	}
	if len(feed.Channel.Items) > maxFeedItems {
		return nil, fmt.Errorf("RSS feed contains more than %d items", maxFeedItems)
	}
	for i, item := range feed.Channel.Items {
		link, err := url.Parse(item.Link)
		if err != nil || link.Host == "" || (link.Scheme != "http" && link.Scheme != "https") {
			return nil, fmt.Errorf("RSS item %d has invalid HTTP(S) link %q", i+1, item.Link)
		}
	}

	return feed.Channel.Items, nil
}

// HashContent creates a SHA-256 hash of the post content
func HashContent(content string) [32]byte {
	// HashContent computes the SHA-256 hash of the provided content string.
	// Returns the hash as a 32-byte array.
	return sha256.Sum256([]byte(content))
}
