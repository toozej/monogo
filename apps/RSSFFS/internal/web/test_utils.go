package web

import (
	"net/http/httptest"
	"strings"

	"github.com/toozej/RSSFFS/pkg/config"
)

// TestServer wraps the web server for testing purposes
type TestServer struct {
	*Server
	testMode bool
}

// NewTestServer creates a server instance configured for testing
func NewTestServer(conf config.Config, debug bool) *TestServer {
	server := NewServer(conf, debug)
	return &TestServer{
		Server:   server,
		testMode: true,
	}
}

// ExtractCSRFTokenFromHTML extracts CSRF token from HTML response (simplified)
func ExtractCSRFTokenFromHTML(html string) string {
	// This is a simplified extraction - in a real test we'd use proper HTML parsing
	start := strings.Index(html, `name="csrf_token" value="`)
	if start == -1 {
		return ""
	}
	start += len(`name="csrf_token" value="`)
	end := strings.Index(html[start:], `"`)
	if end == -1 {
		return ""
	}
	return html[start : start+end]
}

// CreateTestRequest creates a test HTTP request with proper headers
func CreateTestRequest(method, path, body string) *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

// AssertResponseContains checks if response body contains expected content
func AssertResponseContains(t interface{}, response *httptest.ResponseRecorder, expected string) {
	// This would be implemented with proper testing.T interface
	body := response.Body.String()
	if !strings.Contains(body, expected) {
		// In real implementation, we'd use t.Errorf
		panic("Response does not contain expected content: " + expected)
	}
}

// MockRSSFFSResponse represents a mock response from RSSFFS processing
type MockRSSFFSResponse struct {
	Success   bool
	FeedCount int
	Error     string
}

// MockRSSFFSResponses maps URLs to mock responses for testing
var MockRSSFFSResponses = map[string]MockRSSFFSResponse{
	"https://example.com": {
		Success:   true,
		FeedCount: 1,
	},
	"https://multi-feed.example.com": {
		Success:   true,
		FeedCount: 3,
	},
	"https://no-feeds.example.com": {
		Success:   true,
		FeedCount: 0,
	},
	"https://error.example.com": {
		Success: false,
		Error:   "Connection timeout",
	},
}
