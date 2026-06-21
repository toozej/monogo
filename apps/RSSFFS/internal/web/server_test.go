package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/toozej/RSSFFS/pkg/config"
)

func TestNewServer(t *testing.T) {
	conf := config.Config{
		RSSReaderEndpoint: "https://test.example.com",
		RSSReaderAPIKey:   "test-key",
	}

	server := NewServer(conf, true)

	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.config.RSSReaderEndpoint != conf.RSSReaderEndpoint {
		t.Errorf("Expected endpoint %s, got %s", conf.RSSReaderEndpoint, server.config.RSSReaderEndpoint)
	}

	if server.config.RSSReaderAPIKey != conf.RSSReaderAPIKey {
		t.Errorf("Expected API key %s, got %s", conf.RSSReaderAPIKey, server.config.RSSReaderAPIKey)
	}

	if !server.debug {
		t.Error("Expected debug mode to be true")
	}

	if server.rateLimiter == nil {
		t.Error("Expected rate limiter to be initialized")
	}
}

func TestSetupRoutes(t *testing.T) {
	conf := config.Config{}
	server := NewServer(conf, false)
	mux := server.SetupRoutes()

	if mux == nil {
		t.Fatal("SetupRoutes returned nil")
	}

	// Test that routes are properly registered by making test requests
	testCases := []struct {
		path           string
		method         string
		expectedStatus int
	}{
		{"/", "GET", http.StatusOK},
		{"/", "POST", http.StatusMethodNotAllowed},
		{"/submit", "POST", http.StatusForbidden}, // Will fail due to missing CSRF cookie/header
		{"/submit", "GET", http.StatusMethodNotAllowed},
		{"/categories", "GET", http.StatusOK}, // Will use fallback categories when RSS reader not accessible
		{"/categories", "POST", http.StatusMethodNotAllowed},
		{"/static/style.css", "GET", http.StatusOK},
		{"/style.css", "GET", http.StatusOK},   // Direct asset route
		{"/script.js", "GET", http.StatusOK},   // Direct asset route
		{"/favicon.svg", "GET", http.StatusOK}, // Direct asset route
		{"/nonexistent", "GET", http.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.method+"_"+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d for %s %s", tc.expectedStatus, w.Code, tc.method, tc.path)
			}
		})
	}
}

func TestWithMiddleware(t *testing.T) {
	conf := config.Config{}
	server := NewServer(conf, true)

	// Create a test handler that we can verify was called
	handlerCalled := false
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}

	wrappedHandler := server.withMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler(w, req)

	if !handlerCalled {
		t.Error("Expected wrapped handler to be called")
	}

	// Check that security headers are set
	expectedHeaders := map[string]string{
		"X-Content-Type-Options":      "nosniff",
		"X-Frame-Options":             "DENY",
		"X-XSS-Protection":            "1; mode=block",
		"Referrer-Policy":             "strict-origin-when-cross-origin",
		"Content-Security-Policy":     "default-src 'self';",
		"Permissions-Policy":          "geolocation=(), microphone=(), camera=()",
		"Access-Control-Allow-Origin": "*",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := w.Header().Get(header)
		if !strings.Contains(actualValue, expectedValue) {
			t.Errorf("Expected header %s to contain %s, got %s", header, expectedValue, actualValue)
		}
	}
}

func TestWithMiddlewareRateLimit(t *testing.T) {
	conf := config.Config{}
	server := NewServer(conf, false)

	// Override rate limiter with a more restrictive one for testing
	server.rateLimiter = NewRateLimiter(1, time.Minute)

	testHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	wrappedHandler := server.withMiddleware(testHandler)

	// First POST request should succeed
	req1 := httptest.NewRequest("POST", "/test", nil)
	w1 := httptest.NewRecorder()
	wrappedHandler(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected first request to succeed, got status %d", w1.Code)
	}

	// Second POST request should be rate limited
	req2 := httptest.NewRequest("POST", "/test", nil)
	w2 := httptest.NewRecorder()
	wrappedHandler(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Expected second request to be rate limited, got status %d", w2.Code)
	}

	// GET requests should not be rate limited
	req3 := httptest.NewRequest("GET", "/test", nil)
	w3 := httptest.NewRecorder()
	wrappedHandler(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("Expected GET request to succeed, got status %d", w3.Code)
	}
}

func TestWithMiddlewareOptionsRequest(t *testing.T) {
	conf := config.Config{}
	server := NewServer(conf, false)

	handlerCalled := false
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	}

	wrappedHandler := server.withMiddleware(testHandler)

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler(w, req)

	if handlerCalled {
		t.Error("Expected handler not to be called for OPTIONS request")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected OPTIONS request to return 200, got %d", w.Code)
	}
}

func TestSetSecurityHeaders(t *testing.T) {
	conf := config.Config{}
	server := NewServer(conf, false)

	w := httptest.NewRecorder()
	server.setSecurityHeaders(w)

	expectedHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
		"Content-Security-Policy",
		"Permissions-Policy",
		"Cache-Control",
		"Pragma",
		"Expires",
	}

	for _, header := range expectedHeaders {
		if w.Header().Get(header) == "" {
			t.Errorf("Expected header %s to be set", header)
		}
	}

	// Test specific header values
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Expected X-Content-Type-Options to be 'nosniff', got %s", w.Header().Get("X-Content-Type-Options"))
	}

	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("Expected X-Frame-Options to be 'DENY', got %s", w.Header().Get("X-Frame-Options"))
	}
}

func TestHandleIndex(t *testing.T) {
	conf := config.Config{}
	server := NewServer(conf, false)

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{"Valid GET request", "GET", "/", http.StatusOK},
		{"Invalid method", "POST", "/", http.StatusMethodNotAllowed},
		{"Invalid path", "GET", "/invalid", http.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			server.handleIndex(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedStatus == http.StatusOK {
				contentType := w.Header().Get("Content-Type")
				if !strings.Contains(contentType, "text/html") {
					t.Errorf("Expected Content-Type to contain 'text/html', got %s", contentType)
				}

				// Check for CSRF cookie
				csrfCookie := w.Header().Get("Set-Cookie")
				if !strings.Contains(csrfCookie, "csrf_token=") {
					t.Error("Expected Set-Cookie header with csrf_token")
				}

				body := w.Body.String()
				if !strings.Contains(body, "RSSFFS") {
					t.Error("Expected response body to contain 'RSSFFS'")
				}
			}
		})
	}
}

func TestHandleStatic(t *testing.T) {
	conf := config.Config{}
	server := NewServer(conf, false)

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{"Valid CSS file", "GET", "/static/style.css", http.StatusOK},
		{"Valid JS file", "GET", "/static/script.js", http.StatusOK},
		{"HTML file should not be served as static asset", "GET", "/static/index.html", http.StatusForbidden},
		{"Invalid method", "POST", "/static/style.css", http.StatusMethodNotAllowed},
		{"Empty asset path", "GET", "/static/", http.StatusNotFound},
		{"Nonexistent asset", "GET", "/static/nonexistent.css", http.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			server.handleStatic(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedStatus == http.StatusOK {
				// Check that appropriate headers are set
				contentType := w.Header().Get("Content-Type")
				if contentType == "" {
					t.Error("Expected Content-Type header to be set")
				}

				// Check security headers
				if w.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Error("Expected X-Content-Type-Options header to be set")
				}
			}
		})
	}
}
func TestHandleDirectAsset(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		method         string
		expectedStatus int
		expectedType   string
	}{
		{
			name:           "Valid CSS file",
			path:           "/style.css",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedType:   "text/css",
		},
		{
			name:           "Valid JS file",
			path:           "/script.js",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedType:   "application/javascript",
		},
		{
			name:           "Valid SVG file",
			path:           "/favicon.svg",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedType:   "image/svg+xml",
		},
		{
			name:           "Valid HEAD request",
			path:           "/style.css",
			method:         "HEAD",
			expectedStatus: http.StatusOK,
			expectedType:   "text/css",
		},
		{
			name:           "Invalid method",
			path:           "/style.css",
			method:         "POST",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Nonexistent asset",
			path:           "/nonexistent.css",
			method:         "GET",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{}
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			server.handleDirectAsset(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("handleDirectAsset() status = %v, want %v", w.Code, tt.expectedStatus)
			}

			if tt.expectedType != "" {
				contentType := w.Header().Get("Content-Type")
				if !strings.Contains(contentType, tt.expectedType) {
					t.Errorf("handleDirectAsset() Content-Type = %v, want to contain %v", contentType, tt.expectedType)
				}
			}
		})
	}
}
