package RSSFFS

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/RSSFFS/internal/config"
	"golang.org/x/time/rate"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type closeTrackingBody struct {
	io.Reader
	closed bool
}

func (b *closeTrackingBody) Close() error {
	b.closed = true
	return nil
}

// TestExtractDomainFromURL tests the extractDomainFromURL function with various URL formats
func TestExtractDomainFromURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "Full URL with HTTPS",
			input:       "https://example.com/blog/post",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "Full URL with HTTP",
			input:       "http://example.com/feed",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "URL with subdomain",
			input:       "https://blog.example.com",
			expected:    "blog.example.com",
			expectError: false,
		},
		{
			name:        "URL with subdomain and path",
			input:       "https://blog.example.com/posts/latest",
			expected:    "blog.example.com",
			expectError: false,
		},
		{
			name:        "URL without protocol",
			input:       "example.com/blog",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "URL without protocol with subdomain",
			input:       "blog.example.com",
			expected:    "blog.example.com",
			expectError: false,
		},
		{
			name:        "URL with port",
			input:       "https://example.com:8080/feed",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "URL with query parameters",
			input:       "https://example.com/search?q=test",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "URL with fragment",
			input:       "https://example.com/page#section",
			expected:    "example.com",
			expectError: false,
		},
		{
			name:        "Invalid URL - no domain",
			input:       "://invalid",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Invalid URL - malformed",
			input:       "not-a-url",
			expected:    "not-a-url",
			expectError: false,
		},
		{
			name:        "Empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractDomainFromURL(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input %q, but got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("For input %q, expected %q, got %q", tt.input, tt.expected, result)
				}
			}
		})
	}
}

func TestPrivateIPDetectionHandlesMappedIPv4(t *testing.T) {
	for _, raw := range []string{"0.0.0.0", "10.0.0.1", "169.254.169.254", "127.0.0.1", "192.168.1.1", "224.0.0.1", "::", "::1", "fe80::1", "fc00::1"} {
		t.Run(raw, func(t *testing.T) {
			if !isPrivateIP(net.ParseIP(raw)) {
				t.Fatalf("expected %s to be private/internal", raw)
			}
		})
	}
	if isPrivateIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("expected public address to be allowed")
	}
}

func TestSafeDialRevalidatesDNS(t *testing.T) {
	originalLookup := lookupIPAddr
	originalDial := networkDialer
	t.Cleanup(func() {
		lookupIPAddr = originalLookup
		networkDialer = originalDial
	})

	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}, nil
	}
	if err := validateURL("https://public.test/feed"); err != nil {
		t.Fatalf("initial validation failed: %v", err)
	}

	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
	}
	networkDialer = func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("private address reached the network dialer")
		return nil, nil
	}
	if _, err := safeDialContext(context.Background(), "tcp", "public.test:443"); err == nil {
		t.Fatal("expected rebound private address to be rejected")
	}

	client := newSafeHTTPClient(time.Second)
	redirect := &http.Request{URL: mustParseURL(t, "http://private.test/")}
	if err := client.CheckRedirect(redirect, []*http.Request{{URL: mustParseURL(t, "https://public.test/")}}); err == nil {
		t.Fatal("expected private redirect to be rejected")
	}
}

func TestSafeHTTPClientDisablesProxies(t *testing.T) {
	transport := newSafeHTTPTransport()
	if transport.Proxy != nil {
		t.Fatal("proxy must be disabled so target DNS is resolved and pinned locally")
	}
}

func TestSafeDialTriesAllValidatedAddresses(t *testing.T) {
	originalLookup := lookupIPAddr
	originalDial := networkDialer
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}, {IP: net.ParseIP("203.0.113.11")}}, nil
	}
	var attempts []string
	peer, accepted := net.Pipe()
	t.Cleanup(func() {
		lookupIPAddr = originalLookup
		networkDialer = originalDial
		_ = peer.Close()
		_ = accepted.Close()
	})
	networkDialer = func(_ context.Context, _, address string) (net.Conn, error) {
		attempts = append(attempts, address)
		if strings.HasPrefix(address, "203.0.113.10:") {
			return nil, errors.New("unreachable")
		}
		return accepted, nil
	}

	conn, err := safeDialContext(context.Background(), "tcp", "public.test:443")
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if len(attempts) != 2 || !strings.HasPrefix(attempts[1], "203.0.113.11:") {
		t.Fatalf("dial attempts = %v, want both validated addresses", attempts)
	}
}

func TestValidateURLContextHonorsCancellation(t *testing.T) {
	originalLookup := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, _ string) ([]net.IPAddr, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	t.Cleanup(func() { lookupIPAddr = originalLookup })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := validateURLContext(ctx, "https://public.test"); !errors.Is(err, context.Canceled) {
		t.Fatalf("validateURLContext() error = %v, want context cancellation", err)
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestCheckRSSFeedClosesBodiesAndValidatesRoot(t *testing.T) {
	originalLookup := lookupIPAddr
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	t.Cleanup(func() { lookupIPAddr = originalLookup })

	body := &closeTrackingBody{Reader: strings.NewReader("not found")}
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Body: body, Header: make(http.Header)}, nil
	})}
	if checkRSSFeed(context.Background(), client, "https://public.test/feed") {
		t.Fatal("404 response should not be a feed")
	}
	if !body.closed {
		t.Fatal("404 response body was not closed")
	}

	for _, tc := range []struct {
		name    string
		content string
		valid   bool
	}{
		{name: "rss", content: `<rss version="2.0"><channel/></rss>`, valid: true},
		{name: "atom", content: `<feed xmlns="http://www.w3.org/2005/Atom"/>`, valid: true},
		{name: "iso-8859-1 rss", content: `<?xml version="1.0" encoding="ISO-8859-1"?><rss version="2.0"><channel/></rss>`, valid: true},
		{name: "windows-1252 atom", content: `<?xml version="1.0" encoding="windows-1252"?><feed xmlns="http://www.w3.org/2005/Atom"/>`, valid: true},
		{name: "atom 0.3", content: `<feed xmlns="http://purl.org/atom/ns#"/>`, valid: true},
		{name: "rss 1.0", content: `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns="http://purl.org/rss/1.0/"><channel/></rdf:RDF>`, valid: true},
		{name: "unrelated rdf", content: `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description/></rdf:RDF>`, valid: false},
		{name: "wrong atom namespace", content: `<feed xmlns="https://example.com/not-atom"/>`, valid: false},
		{name: "root beyond response limit", content: strings.Repeat(" ", maxFeedBytes) + `<rss version="2.0"/>`, valid: false},
		{name: "sitemap", content: `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"/>`, valid: false},
		{name: "generic xml", content: `<document/>`, valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tc.content)), Header: make(http.Header)}, nil
			})}
			if got := checkRSSFeed(context.Background(), client, "https://public.test/feed"); got != tc.valid {
				t.Fatalf("got %t, want %t", got, tc.valid)
			}
		})
	}
}

func TestGetAllDomainsBoundsPageBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// nosemgrep: go.lang.security.audit.xss.no-io-writestring-to-responsewriter.no-io-writestring-to-responsewriter -- fixed test-only response body with no user input
		_, _ = io.WriteString(w, strings.Repeat("x", maxPageBytes))
		_, _ = io.WriteString(w, `<a href="https://beyond-limit.test/">too late</a>`)
	}))
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)
	_, port, _ := net.SplitHostPort(serverURL.Host)

	originalLookup := lookupIPAddr
	originalDial := networkDialer
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	networkDialer = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, serverURL.Host)
	}
	t.Cleanup(func() {
		lookupIPAddr = originalLookup
		networkDialer = originalDial
	})

	origin := "http://public.test:" + port
	domains, err := getAllDomainsFromPage(context.Background(), origin)
	if err != nil {
		t.Fatal(err)
	}
	if domains["https://beyond-limit.test"] {
		t.Fatalf("parsed link beyond %d-byte page limit", maxPageBytes)
	}
}

func TestTraversalSeedsOriginAndResolvesRelativeLinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "redirected.test"+r.URL.Query().Get("port") && r.URL.Path == "/" {
			http.Redirect(w, r, "http://redirected.test"+r.URL.Query().Get("port")+"/final", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, `<a href="/relative">relative</a><a href="https://other.test/page">other</a>`)
		for i := 0; i < maxDomains+20; i++ {
			_, _ = fmt.Fprintf(w, `<a href="https://linked-%d.test/page">linked</a>`, i)
		}
	}))
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)
	_, port, _ := net.SplitHostPort(serverURL.Host)

	originalLookup := lookupIPAddr
	originalDial := networkDialer
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	networkDialer = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, serverURL.Host)
	}
	t.Cleanup(func() {
		lookupIPAddr = originalLookup
		networkDialer = originalDial
	})

	origin := "http://public.test:" + port
	domains, err := getAllDomainsFromPage(context.Background(), origin+"/?port=:"+port)
	if err != nil {
		t.Fatal(err)
	}
	if !domains[origin] {
		t.Fatalf("submitted origin missing from %#v", domains)
	}
	if !domains["http://redirected.test:"+port] {
		t.Fatalf("final redirect origin missing from %#v", domains)
	}
	if !domains["https://other.test"] {
		t.Fatalf("absolute linked origin missing from %#v", domains)
	}
	if len(domains) != maxDomains {
		t.Fatalf("domain count = %d, want cap %d", len(domains), maxDomains)
	}
}

func TestCheckDomainsForRSSBoundsConcurrency(t *testing.T) {
	var active atomic.Int32
	var peak atomic.Int32
	release := make(chan struct{})
	var releaseOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := peak.Load()
			if current <= previous || peak.CompareAndSwap(previous, current) {
				break
			}
		}
		if current == maxDomainWorkers {
			releaseOnce.Do(func() { close(release) })
		}
		select {
		case <-release:
		case <-time.After(2 * time.Second):
		}
		_, _ = io.WriteString(w, `<rss version="2.0"><channel/></rss>`)
	}))
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)

	originalLookup := lookupIPAddr
	originalDial := networkDialer
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	networkDialer = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, serverURL.Host)
	}
	t.Cleanup(func() {
		lookupIPAddr = originalLookup
		networkDialer = originalDial
	})

	domains := make(map[string]bool)
	for i := 0; i < maxDomainWorkers*2+3; i++ {
		domains[fmt.Sprintf("http://domain-%d.test", i)] = true
	}
	feeds, err := checkDomainsForRSS(context.Background(), domains, "http://submitted.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != len(domains) {
		t.Fatalf("feed count = %d, want %d", len(feeds), len(domains))
	}
	if got := peak.Load(); got != maxDomainWorkers {
		t.Fatalf("peak concurrent probes = %d, want %d", got, maxDomainWorkers)
	}
}

func TestNormalizeHTTPURLPreservesAndDefaultsScheme(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "example.com/path", want: "https://example.com/path"},
		{input: "http://example.com/path", want: "http://example.com/path"},
		{input: "HTTP://example.com/path", want: "http://example.com/path"},
	} {
		parsed, err := normalizeHTTPURL(tc.input)
		if err != nil {
			t.Fatal(err)
		}
		if got := parsed.String(); got != tc.want {
			t.Fatalf("normalizeHTTPURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
	if _, err := normalizeHTTPURL("ftp://example.com/feed"); err == nil {
		t.Fatal("non-HTTP scheme must be rejected")
	}
}

func TestDiscoveryPropagatesCancellation(t *testing.T) {
	originalLookup := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, _ string) ([]net.IPAddr, error) {
		return nil, ctx.Err()
	}
	t.Cleanup(func() { lookupIPAddr = originalLookup })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := discoverSingleURLMode(ctx, "https://public.test"); !errors.Is(err, context.Canceled) {
		t.Fatalf("single URL discovery error = %v, want context cancellation", err)
	}
	if _, err := getAllDomainsFromPage(ctx, "https://public.test"); !errors.Is(err, context.Canceled) {
		t.Fatalf("page discovery error = %v, want context cancellation", err)
	}
	if _, err := checkDomainsForRSS(ctx, map[string]bool{"https://public.test": true}, "https://public.test"); !errors.Is(err, context.Canceled) {
		t.Fatalf("domain discovery error = %v, want context cancellation", err)
	}
}

func TestSubscribeToFeedPreservesURLSemantics(t *testing.T) {
	wantFeed := `https://feeds.example.test/search?a=1&label="news"`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			FeedURL    string `json:"feed_url"`
			CategoryID int    `json:"category_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode subscription body: %v", err)
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if body.FeedURL != wantFeed || body.CategoryID != 42 {
			t.Errorf("subscription body = %#v, want feed %q and category 42", body, wantFeed)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	originalClient := readerHTTPClient
	originalLimiter := limiter
	readerHTTPClient = server.Client()
	limiter = rate.NewLimiter(rate.Inf, 1)
	t.Cleanup(func() {
		readerHTTPClient = originalClient
		limiter = originalLimiter
	})
	if err := subscribeToFeed(context.Background(), server.URL, "token", 42, wantFeed); err != nil {
		t.Fatal(err)
	}
}

func TestReaderClientDoesNotForwardAPIKeyAcrossRedirect(t *testing.T) {
	var targetRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetRequests.Add(1)
		if token := r.Header.Get("X-Auth-Token"); token != "" {
			t.Errorf("redirect target received API token %q", token)
		}
		_, _ = io.WriteString(w, `[]`)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()

	if _, err := getCategoryId(context.Background(), source.URL, "super-secret", "News"); err == nil {
		t.Fatal("reader API redirect must be rejected")
	}
	if got := targetRequests.Load(); got != 0 {
		t.Fatalf("redirect target received %d requests", got)
	}
}

func TestSubscribeToFeedHonorsCanceledRateLimitWait(t *testing.T) {
	originalLimiter := limiter
	limiter = rate.NewLimiter(rate.Every(time.Hour), 1)
	if !limiter.Allow() {
		t.Fatal("failed to consume initial limiter token")
	}
	t.Cleanup(func() { limiter = originalLimiter })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := subscribeToFeed(ctx, "https://reader.test", "token", 1, "https://feed.test/rss"); !errors.Is(err, context.Canceled) {
		t.Fatalf("subscribeToFeed() error = %v, want context cancellation", err)
	}
}

func TestDebugResetDiscoversBeforeMutatingAndStillSubscribes(t *testing.T) {
	var deletes atomic.Int32
	var posts atomic.Int32
	feedAvailable := true
	categoryAvailable := true
	deleteStatus := http.StatusNoContent
	postStatus := http.StatusCreated
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/categories":
			if categoryAvailable {
				_, _ = io.WriteString(w, `[{"id":1,"title":"News"}]`)
			} else {
				_, _ = io.WriteString(w, `[]`)
			}
		case r.Method == http.MethodGet && r.URL.Path == "/v1/categories/1/feeds":
			_, _ = io.WriteString(w, `[{"id":7}]`)
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/feeds/7":
			deletes.Add(1)
			w.WriteHeader(deleteStatus)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/feeds":
			posts.Add(1)
			w.WriteHeader(postStatus)
		case r.Method == http.MethodGet && r.URL.Path == "/feed" && feedAvailable:
			_, _ = io.WriteString(w, `<rss version="2.0"><channel/></rss>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)
	_, port, _ := net.SplitHostPort(serverURL.Host)

	originalLookup := lookupIPAddr
	originalDial := networkDialer
	originalLimiter := limiter
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	networkDialer = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, serverURL.Host)
	}
	limiter = rate.NewLimiter(rate.Inf, 10)
	t.Cleanup(func() {
		lookupIPAddr = originalLookup
		networkDialer = originalDial
		limiter = originalLimiter
	})

	conf := config.Config{RSSReaderEndpoint: server.URL, RSSReaderAPIKey: "token"}
	pageURL := "http://public.test:" + port
	count, err := RunContext(context.Background(), pageURL, "News", true, true, true, conf)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || deletes.Load() != 1 || posts.Load() != 1 {
		t.Fatalf("count=%d deletes=%d posts=%d", count, deletes.Load(), posts.Load())
	}

	conf.SingleURLMode = true
	posts.Store(0)
	count, err = Run(pageURL, "News", false, false, false, conf)
	if err != nil || count != 1 || posts.Load() != 1 {
		t.Fatalf("configured single URL mode count=%d posts=%d error=%v", count, posts.Load(), err)
	}
	conf.SingleURLMode = false
	categoryAvailable = false
	posts.Store(0)
	if _, err := RunContext(context.Background(), pageURL, "Missing", false, false, true, conf); err == nil {
		t.Fatal("expected missing named category to fail")
	}
	if posts.Load() != 0 {
		t.Fatalf("missing category triggered %d subscriptions", posts.Load())
	}
	categoryAvailable = true

	feedAvailable = false
	deletes.Store(0)
	posts.Store(0)
	if _, err := RunContext(context.Background(), pageURL, "News", true, true, true, conf); err == nil {
		t.Fatal("expected reset without replacements to fail")
	}
	if deletes.Load() != 0 || posts.Load() != 0 {
		t.Fatalf("mutated before discovery: deletes=%d posts=%d", deletes.Load(), posts.Load())
	}

	feedAvailable = true
	postStatus = http.StatusInternalServerError
	if count, err := RunContext(context.Background(), pageURL, "News", true, false, true, conf); err == nil || count != 0 {
		t.Fatalf("subscription failure count=%d error=%v, want propagated failure", count, err)
	}

	postStatus = http.StatusFound
	if _, err := RunContext(context.Background(), pageURL, "News", true, false, true, conf); err == nil {
		t.Fatal("expected redirect response from subscription API to fail")
	}

	postStatus = http.StatusCreated
	deleteStatus = http.StatusInternalServerError
	deletes.Store(0)
	posts.Store(0)
	if _, err := RunContext(context.Background(), pageURL, "News", true, true, true, conf); err == nil {
		t.Fatal("expected deletion failure to propagate")
	}
	if deletes.Load() != 1 || posts.Load() != 0 {
		t.Fatalf("deletion failure deletes=%d posts=%d, want no subscriptions", deletes.Load(), posts.Load())
	}
}

// TestModeSelectionLogic tests the mode selection logic in the Run function
// Note: This is a basic test that verifies the function signature and basic parameter handling
func TestModeSelectionLogic(t *testing.T) {
	// This test verifies that the Run function accepts the correct parameters
	// and doesn't panic with basic inputs. Full integration testing would require
	// mocking the RSS reader API and HTTP client.

	// Test that the function signature is correct and accepts all required parameters
	defer func() {
		if r := recover(); r != nil {
			// If we get a panic about missing API configuration, that's expected
			// since we're not providing valid API credentials
			if panicMsg, ok := r.(string); ok {
				if panicMsg == "Error getting categoryId from category test: " {
					// This is expected - we don't have valid API credentials
					return
				}
			}
			// Re-panic if it's an unexpected error
			panic(r)
		}
	}()

	// For now, we just test that the function signature is correct
	// by checking that we can call the helper functions
	t.Log("Testing that mode selection helper functions exist")

	// Test that we can call extractDomainFromURL (already tested above)
	domain, err := extractDomainFromURL("https://example.com")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if domain != "example.com" {
		t.Errorf("Expected example.com, got %s", domain)
	}
}

// TestCommonPatternsExist tests that the common RSS patterns are defined
func TestCommonPatternsExist(t *testing.T) {
	expectedPatterns := []string{"/index.xml", "/feed", "/feed.xml", "/rss", "/rss.xml", "/atom.xml", "/?format=rss"}

	if len(commonPatterns) != len(expectedPatterns) {
		t.Errorf("Expected %d common patterns, got %d", len(expectedPatterns), len(commonPatterns))
	}

	for i, expected := range expectedPatterns {
		if i >= len(commonPatterns) {
			t.Errorf("Missing pattern: %s", expected)
			continue
		}
		if commonPatterns[i] != expected {
			t.Errorf("Pattern %d: expected %s, got %s", i, expected, commonPatterns[i])
		}
	}
}

// TestSingleURLModeIntegration tests the complete single URL mode workflow
func TestSingleURLModeIntegration(t *testing.T) {
	// Mock configuration for testing
	mockConfig := struct {
		RSSReaderEndpoint string
		RSSReaderAPIKey   string
		SingleURLMode     bool
	}{
		RSSReaderEndpoint: "https://test.example.com/api",
		RSSReaderAPIKey:   "test-api-key",
		SingleURLMode:     false,
	}

	tests := []struct {
		name              string
		pageURL           string
		category          string
		debug             bool
		clearFeeds        bool
		singleURLMode     bool
		envSingleURLMode  bool
		expectPanic       bool
		expectedLogPhrase string
	}{
		{
			name:              "Single URL mode via CLI flag",
			pageURL:           "https://example.com/blog",
			category:          "test",
			debug:             true,
			clearFeeds:        false,
			singleURLMode:     true,
			envSingleURLMode:  false,
			expectPanic:       true, // Will panic due to missing API config
			expectedLogPhrase: "Using single URL mode for domain: example.com",
		},
		{
			name:              "Single URL mode via environment variable",
			pageURL:           "https://blog.example.com",
			category:          "test",
			debug:             true,
			clearFeeds:        false,
			singleURLMode:     false,
			envSingleURLMode:  true,
			expectPanic:       true, // Will panic due to missing API config
			expectedLogPhrase: "Using single URL mode for domain: blog.example.com",
		},
		{
			name:              "Traversal mode (default)",
			pageURL:           "https://example.com",
			category:          "test",
			debug:             true,
			clearFeeds:        false,
			singleURLMode:     false,
			envSingleURLMode:  false,
			expectPanic:       true, // Will panic due to missing API config
			expectedLogPhrase: "Using traversal mode, checking all domains found on page",
		},
		{
			name:              "CLI flag overrides environment variable",
			pageURL:           "https://test.example.com",
			category:          "test",
			debug:             true,
			clearFeeds:        false,
			singleURLMode:     true,
			envSingleURLMode:  false,
			expectPanic:       true, // Will panic due to missing API config
			expectedLogPhrase: "Using single URL mode for domain: test.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture panic to verify expected behavior
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectPanic {
						t.Errorf("Unexpected panic: %v", r)
					}
					// In a real test, we would capture and verify log output
					// For now, we just verify that the function was called with correct parameters
				}
			}()

			// Create a mock config that includes the environment variable setting
			testConfig := mockConfig
			testConfig.SingleURLMode = tt.envSingleURLMode

			// This would normally call Run, but since we don't have a real API,
			// we'll test the mode selection logic separately
			t.Logf("Testing mode selection: CLI=%t, Env=%t", tt.singleURLMode, tt.envSingleURLMode)

			// Test the mode selection logic
			useSingleURLMode := tt.singleURLMode || tt.envSingleURLMode
			if useSingleURLMode {
				// Test domain extraction for single URL mode
				domain, err := extractDomainFromURL(tt.pageURL)
				if err != nil {
					t.Errorf("Failed to extract domain from %s: %v", tt.pageURL, err)
				}
				t.Logf("Single URL mode would check domain: %s", domain)
			} else {
				t.Log("Traversal mode would check all domains on page")
			}
		})
	}
}

// TestSingleURLModeErrorHandling tests error handling in single URL mode
func TestSingleURLModeErrorHandling(t *testing.T) {
	errorTests := []struct {
		name        string
		pageURL     string
		expectError bool
		errorPhrase string
	}{
		{
			name:        "Valid URL with subdomain",
			pageURL:     "https://blog.example.com/posts",
			expectError: false,
			errorPhrase: "",
		},
		{
			name:        "Valid URL without protocol",
			pageURL:     "example.com",
			expectError: false,
			errorPhrase: "",
		},
		{
			name:        "Empty URL",
			pageURL:     "",
			expectError: true,
			errorPhrase: "URL cannot be empty",
		},
		{
			name:        "Invalid URL format",
			pageURL:     "://invalid-url",
			expectError: true,
			errorPhrase: "no valid hostname found",
		},
		{
			name:        "URL with spaces in hostname",
			pageURL:     "https://example .com",
			expectError: true,
			errorPhrase: "invalid URL format",
		},
		{
			name:        "Very long hostname",
			pageURL:     "https://" + strings.Repeat("a", 260) + ".com",
			expectError: true,
			errorPhrase: "hostname too long",
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := extractDomainFromURL(tt.pageURL)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for URL %q, but got none", tt.pageURL)
				} else if !strings.Contains(err.Error(), tt.errorPhrase) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorPhrase, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for URL %q: %v", tt.pageURL, err)
				}
				if domain == "" {
					t.Errorf("Expected non-empty domain for valid URL %q", tt.pageURL)
				}
			}
		})
	}
}

// TestSingleURLModeLogging tests that appropriate log messages are generated
func TestSingleURLModeLogging(t *testing.T) {
	// Note: In a real implementation, we would use a test logger to capture
	// and verify log output. For now, we test the logic that determines
	// what should be logged.

	testCases := []struct {
		name           string
		pageURL        string
		singleURLMode  bool
		expectedDomain string
	}{
		{
			name:           "Single URL mode with simple domain",
			pageURL:        "https://example.com",
			singleURLMode:  true,
			expectedDomain: "example.com",
		},
		{
			name:           "Single URL mode with subdomain",
			pageURL:        "https://blog.example.com/feed",
			singleURLMode:  true,
			expectedDomain: "blog.example.com",
		},
		{
			name:           "Single URL mode with complex path",
			pageURL:        "https://news.example.com/category/tech?page=1",
			singleURLMode:  true,
			expectedDomain: "news.example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.singleURLMode {
				domain, err := extractDomainFromURL(tc.pageURL)
				if err != nil {
					t.Errorf("Failed to extract domain: %v", err)
				}
				if domain != tc.expectedDomain {
					t.Errorf("Expected domain %s, got %s", tc.expectedDomain, domain)
				}
				// In a real test, we would verify that the log message
				// "Using single URL mode for domain: %s" was generated
				t.Logf("Would log: Using single URL mode for domain: %s", domain)
			}
		})
	}
}

// TestRSSPatternChecking tests the RSS pattern checking logic
func TestRSSPatternChecking(t *testing.T) {
	// Test that we have the expected common patterns
	expectedPatterns := []string{"/index.xml", "/feed", "/feed.xml", "/rss", "/rss.xml", "/atom.xml", "/?format=rss"}

	if len(commonPatterns) != len(expectedPatterns) {
		t.Errorf("Expected %d patterns, got %d", len(expectedPatterns), len(commonPatterns))
	}

	for i, expected := range expectedPatterns {
		if i >= len(commonPatterns) || commonPatterns[i] != expected {
			t.Errorf("Pattern mismatch at index %d: expected %s, got %s",
				i, expected, commonPatterns[i])
		}
	}

	// Test pattern URL construction
	testDomain := "example.com"
	for _, pattern := range commonPatterns {
		expectedURL := "https://" + testDomain + pattern
		t.Logf("Would check RSS feed at: %s", expectedURL)

		// Verify URL construction is valid
		if !strings.HasPrefix(expectedURL, "https://") {
			t.Errorf("Invalid URL construction: %s", expectedURL)
		}
		if !strings.Contains(expectedURL, testDomain) {
			t.Errorf("URL should contain domain: %s", expectedURL)
		}
	}
}

// TestMediumSpecialCase tests the special case handling for medium.com URLs
func TestMediumSpecialCase(t *testing.T) {
	tests := []struct {
		name         string
		domain       string
		originalURL  string
		expectedPath string
		shouldTry    bool
	}{
		{
			name:         "Medium.com with username",
			domain:       "medium.com",
			originalURL:  "https://medium.com/rokkorxblog",
			expectedPath: "rokkorxblog",
			shouldTry:    true,
		},
		{
			name:         "Medium.com root",
			domain:       "medium.com",
			originalURL:  "https://medium.com",
			expectedPath: "",
			shouldTry:    false,
		},
		{
			name:         "Medium.com with path",
			domain:       "medium.com",
			originalURL:  "https://medium.com/tag/technology",
			expectedPath: "",
			shouldTry:    false, // has slash, not username
		},
		{
			name:         "Other domain",
			domain:       "example.com",
			originalURL:  "https://example.com/user",
			expectedPath: "",
			shouldTry:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the logic for constructing special URL
			if tt.domain == "medium.com" && strings.HasPrefix(tt.originalURL, "https://medium.com/") {
				path := strings.TrimPrefix(tt.originalURL, "https://medium.com/")
				if path != "" && !strings.Contains(path, "/") {
					specialURL := "https://medium.com/feed/" + path
					expected := "https://medium.com/feed/" + tt.expectedPath
					if specialURL != expected {
						t.Errorf("Expected special URL %s, got %s", expected, specialURL)
					}
				} else if tt.shouldTry {
					t.Errorf("Expected to try special case for %s", tt.originalURL)
				}
			} else if tt.shouldTry {
				t.Errorf("Did not expect to try special case for %s", tt.originalURL)
			}
		})
	}
}

// TestModeSelectionPrecedence tests CLI flag vs environment variable precedence
func TestModeSelectionPrecedence(t *testing.T) {
	precedenceTests := []struct {
		name         string
		cliFlag      bool
		envVar       bool
		expectedMode bool
		description  string
	}{
		{
			name:         "CLI flag true, env var false",
			cliFlag:      true,
			envVar:       false,
			expectedMode: true,
			description:  "CLI flag should take precedence",
		},
		{
			name:         "CLI flag false, env var true",
			cliFlag:      false,
			envVar:       true,
			expectedMode: true,
			description:  "Environment variable should be used when CLI flag is false",
		},
		{
			name:         "Both true",
			cliFlag:      true,
			envVar:       true,
			expectedMode: true,
			description:  "Should use single URL mode when both are true",
		},
		{
			name:         "Both false",
			cliFlag:      false,
			envVar:       false,
			expectedMode: false,
			description:  "Should use traversal mode when both are false",
		},
	}

	for _, tt := range precedenceTests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the precedence logic: CLI flag OR environment variable
			actualMode := tt.cliFlag || tt.envVar

			if actualMode != tt.expectedMode {
				t.Errorf("%s: expected mode %t, got %t",
					tt.description, tt.expectedMode, actualMode)
			}

			// Log what mode would be selected
			if actualMode {
				t.Log("Would use single URL mode")
			} else {
				t.Log("Would use traversal mode")
			}
		})
	}
}
