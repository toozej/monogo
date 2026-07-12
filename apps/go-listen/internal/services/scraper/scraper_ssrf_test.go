package scraper

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

type testRoundTripFunc func(*http.Request) (*http.Response, error)

func (f testRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// TestFetchURLRejectsSSRF verifies that fetchURL refuses non-HTTP(S) schemes and
// URLs resolving to private/internal ranges before issuing any request. The
// rejection happens prior to the HTTP call, so no httpClient is needed.
func TestFetchURLRejectsSSRF(t *testing.T) {
	w := &WebScraper{
		config: DefaultScraperConfig(),
		httpClient: &http.Client{Transport: testRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("outbound request should not be reached")
		})},
	}

	blocked := []string{
		"http://127.0.0.1/admin",
		"http://169.254.169.254/latest/meta-data/",
		"http://192.168.1.1/",
		"http://10.0.0.5/",
		"http://100.64.0.1/",
		"http://198.18.0.1/",
		"http://224.0.0.1/",
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

func TestSafeDialTriesEachPublicAddress(t *testing.T) {
	originalLookup := lookupIPAddr
	originalDial := networkDial
	t.Cleanup(func() {
		lookupIPAddr = originalLookup
		networkDial = originalDial
	})
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{
			{IP: net.ParseIP("93.184.216.34")},
			{IP: net.ParseIP("93.184.216.35")},
		}, nil
	}
	var addresses []string
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	networkDial = func(_ context.Context, _, address string) (net.Conn, error) {
		addresses = append(addresses, address)
		if len(addresses) == 1 {
			return nil, errors.New("first address unavailable")
		}
		return client, nil
	}

	conn, err := safeDialContext(false)(context.Background(), "tcp", "example.test:443")
	if err != nil {
		t.Fatalf("safeDialContext() error = %v", err)
	}
	if conn == nil || len(addresses) != 2 {
		t.Fatalf("dialed addresses = %v, connection = %v", addresses, conn)
	}
}

func TestFetchWithRetryDoesNotRetryInvalidURL(t *testing.T) {
	cfg := DefaultScraperConfig()
	cfg.MaxRetries = 1
	cfg.RetryBackoff = 200 * time.Millisecond
	w := &WebScraper{config: cfg, logger: logrus.New()}

	started := time.Now()
	_, err := w.fetchWithRetry("ftp://example.com/feed")
	if err == nil {
		t.Fatal("expected invalid scheme error")
	}
	if elapsed := time.Since(started); elapsed >= 100*time.Millisecond {
		t.Fatalf("permanent validation error was retried; elapsed %s", elapsed)
	}
}

func TestHTTPClientRejectsUnsafeRedirect(t *testing.T) {
	cfg := DefaultScraperConfig()
	req, err := http.NewRequest(http.MethodGet, "https://169.254.169.254/latest/meta-data/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	if err := newHTTPClient(cfg).CheckRedirect(req, nil); err == nil || !strings.Contains(err.Error(), "refusing redirect target") {
		t.Fatalf("unsafe redirect error = %v", err)
	}
}

func TestSafeDialRejectsReboundInternalAddress(t *testing.T) {
	originalLookup := lookupIPAddr
	originalDial := networkDial
	t.Cleanup(func() {
		lookupIPAddr = originalLookup
		networkDial = originalDial
	})
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	networkDial = func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("dial should not be reached")
	}

	_, err := safeDialContext(false)(context.Background(), "tcp", "example.test:80")
	if err == nil || !strings.Contains(err.Error(), "private/internal") {
		t.Fatalf("safeDialContext error = %v", err)
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
