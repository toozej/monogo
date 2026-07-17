package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/RSSFFS/internal/config"
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

func TestSwaggerRoute(t *testing.T) {
	for _, tc := range []struct {
		name       string
		conf       config.Config
		username   string
		password   string
		wantStatus int
	}{
		{
			name:       "available without configured authentication",
			wantStatus: http.StatusOK,
		},
		{
			name:       "protected by configured authentication",
			conf:       config.Config{WebUsername: "owner", WebPassword: "secret"},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "available with configured authentication",
			conf:       config.Config{WebUsername: "owner", WebPassword: "secret"},
			username:   "owner",
			password:   "secret",
			wantStatus: http.StatusOK,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
			if tc.username != "" {
				req.SetBasicAuth(tc.username, tc.password)
			}
			w := httptest.NewRecorder()

			NewServer(tc.conf, false).SetupRoutes().ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("got status %d, want %d", w.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusOK {
				if got := w.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
					t.Fatalf("Content-Type = %q, want HTML", got)
				}
				if !strings.Contains(w.Body.String(), "Swagger UI") {
					t.Fatal("response does not contain Swagger UI")
				}
			}
		})
	}
}

func TestSwaggerDocument(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	w := httptest.NewRecorder()

	NewServer(config.Config{}, false).SetupRoutes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var spec struct {
		Info struct {
			Title string `json:"title"`
		} `json:"info"`
		Paths               map[string]json.RawMessage `json:"paths"`
		SecurityDefinitions map[string]struct {
			Type string `json:"type"`
		} `json:"securityDefinitions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode Swagger document: %v", err)
	}
	if spec.Info.Title != "RSSFFS API" {
		t.Errorf("Swagger title = %q, want %q", spec.Info.Title, "RSSFFS API")
	}
	for _, path := range []string{"/categories", "/logs", "/logs/stream", "/submit"} {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("Swagger document is missing %s", path)
		}
	}
	if definition, ok := spec.SecurityDefinitions["BasicAuth"]; !ok {
		t.Error("Swagger document is missing the BasicAuth security definition")
	} else if definition.Type != "basic" {
		t.Errorf("BasicAuth type = %q, want %q", definition.Type, "basic")
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
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"X-XSS-Protection":        "1; mode=block",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Content-Security-Policy": "default-src 'self';",
		"Permissions-Policy":      "geolocation=(), microphone=(), camera=()",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := w.Header().Get(header)
		if !strings.Contains(actualValue, expectedValue) {
			t.Errorf("Expected header %s to contain %s, got %s", header, expectedValue, actualValue)
		}
	}
}

func TestWithMiddlewareRequiresConfiguredBasicAuth(t *testing.T) {
	server := NewServer(config.Config{WebUsername: "owner", WebPassword: "secret"}, false)
	handler := server.withMiddleware(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	for _, tc := range []struct {
		name       string
		username   string
		password   string
		wantStatus int
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "wrong", username: "owner", password: "wrong", wantStatus: http.StatusUnauthorized},
		{name: "valid", username: "owner", password: "secret", wantStatus: http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.username != "" {
				req.SetBasicAuth(tc.username, tc.password)
			}
			w := httptest.NewRecorder()
			handler(w, req)
			if w.Code != tc.wantStatus {
				t.Fatalf("got %d, want %d", w.Code, tc.wantStatus)
			}
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Fatalf("unexpected permissive CORS header %q", got)
			}
			if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("security header = %q, want nosniff", got)
			}
		})
	}
}

func TestValidateBindAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name    string
		host    string
		conf    config.Config
		wantErr bool
	}{
		{name: "loopback without auth", host: "127.0.0.1"},
		{name: "IPv6 loopback without auth", host: "::1"},
		{name: "public without auth", host: "0.0.0.0", wantErr: true},
		{name: "public with auth", host: "0.0.0.0", conf: config.Config{WebUsername: "owner", WebPassword: "secret"}},
		{name: "partial auth", host: "127.0.0.1", conf: config.Config{WebUsername: "owner"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(tc.conf, false)
			err := server.validateBindAuthentication(tc.host)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateBindAuthentication() error = %v, wantErr %t", err, tc.wantErr)
			}
		})
	}
}

func TestSubmissionTimeoutFitsServerWriteDeadline(t *testing.T) {
	if submissionTimeout >= serverWriteTimeout {
		t.Fatalf("submission timeout %s must be shorter than server write timeout %s", submissionTimeout, serverWriteTimeout)
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

	if !handlerCalled {
		t.Error("Expected handler to receive OPTIONS request without cross-origin middleware")
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
