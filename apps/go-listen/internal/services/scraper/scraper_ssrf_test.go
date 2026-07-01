package scraper

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestFetchURLRejectsSSRF verifies that fetchURL refuses non-HTTP(S) schemes and
// URLs resolving to private/internal ranges before issuing any request. The
// rejection happens prior to the HTTP call, so no httpClient is needed.
func TestFetchURLRejectsSSRF(t *testing.T) {
	w := &WebScraper{config: DefaultScraperConfig()}

	blocked := []string{
		"http://127.0.0.1/admin",
		"http://169.254.169.254/latest/meta-data/",
		"http://192.168.1.1/",
		"http://10.0.0.5/",
		"http://[::1]/",
		"ftp://example.com/feed",
		"",
	}

	for _, url := range blocked {
		if _, err := w.fetchURL(url); err == nil {
			t.Errorf("fetchURL(%q) = nil error, want SSRF/scheme rejection", url)
		} else if !strings.Contains(err.Error(), "refusing to scrape URL") {
			t.Errorf("fetchURL(%q) error = %q, want it to wrap the urlsafe rejection", url, err)
		}
	}
}

// TestFetchURLAllowPrivateBypassesValidation confirms the opt-out lets a
// private target pass the SSRF guard (the ensuing network error is expected and
// proves validation no longer short-circuits the call).
func TestFetchURLAllowPrivateBypassesValidation(t *testing.T) {
	cfg := DefaultScraperConfig()
	cfg.AllowPrivateNetwork = true
	w := &WebScraper{config: cfg, httpClient: &http.Client{Timeout: 2 * time.Second}}

	// 127.0.0.1:1 is almost certainly closed; the guard must not be what stops us.
	_, err := w.fetchURL("http://127.0.0.1:1/")
	if err != nil && strings.Contains(err.Error(), "refusing to scrape URL") {
		t.Fatalf("AllowPrivateNetwork should bypass the SSRF guard, got: %v", err)
	}
}
