package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/toozej/RSSFFS/pkg/config"
)

// Helper to extract cookie value from a response
func getCSRFCookie(w *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "csrf_token" {
			return cookie
		}
	}
	return nil
}

// TestWebServerIntegration tests the complete web server workflow
func TestWebServerIntegration(t *testing.T) {
	// Create a test configuration
	conf := config.Config{
		RSSReaderEndpoint: "https://test.example.com",
		RSSReaderAPIKey:   "test-key",
	}

	// Create server instance
	server := NewServer(conf, true)
	mux := server.SetupRoutes()

	// Test 1: GET index page
	t.Run("GET index page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			t.Errorf("Expected HTML content type, got %s", contentType)
		}

		// Check for CSRF cookie
		csrfCookie := getCSRFCookie(w)
		if csrfCookie == nil {
			t.Fatal("CSRF cookie not found in response")
		}
		if csrfCookie.Value == "" {
			t.Error("CSRF cookie value should not be empty")
		}

		body := w.Body.String()
		if !strings.Contains(body, "<form") {
			t.Error("Expected page to contain a form")
		}
		if strings.Contains(body, "csrf_token") {
			t.Error("Expected page to not contain CSRF token field")
		}
	})

	// Test 2: Static asset serving
	t.Run("Static asset serving", func(t *testing.T) {
		assets := []string{"/static/style.css", "/script.js"}

		for _, asset := range assets {
			req := httptest.NewRequest("GET", asset, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200 for %s, got %d", asset, w.Code)
			}
		}
	})

	// Test 3: Form submission workflow with validation errors
	t.Run("Form submission with validation errors", func(t *testing.T) {
		// First get CSRF cookie from index page
		indexReq := httptest.NewRequest("GET", "/", nil)
		indexW := httptest.NewRecorder()
		mux.ServeHTTP(indexW, indexReq)
		csrfCookie := getCSRFCookie(indexW)
		if csrfCookie == nil {
			t.Fatal("CSRF cookie not found")
		}

		// Test invalid URL submission
		formData := url.Values{
			"url":      {"invalid-url"},
			"category": {"test"},
		}

		req := httptest.NewRequest("POST", "/submit", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-CSRF-Token", csrfCookie.Value)
		req.AddCookie(csrfCookie)

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid URL, got %d", w.Code)
		}

		var response SubmitResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if response.Success {
			t.Error("Expected validation error response to indicate failure")
		}
	})

	// Test 4: CSRF protection
	t.Run("CSRF protection", func(t *testing.T) {
		// Submit form without CSRF header/cookie
		formData := url.Values{
			"url":      {"https://example.com"},
			"category": {"test"},
		}

		req := httptest.NewRequest("POST", "/submit", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403 for missing CSRF token, got %d", w.Code)
		}
	})

	// Test 5: Rate limiting
	t.Run("Rate limiting", func(t *testing.T) {
		// Create a new server with restrictive rate limiting for this test
		testServer := NewServer(conf, false)
		testServer.rateLimiter = NewRateLimiter(1, time.Minute)
		testMux := testServer.SetupRoutes()

		// Get CSRF cookie
		indexReq := httptest.NewRequest("GET", "/", nil)
		indexW := httptest.NewRecorder()
		testMux.ServeHTTP(indexW, indexReq)
		csrfCookie := getCSRFCookie(indexW)
		if csrfCookie == nil {
			t.Fatal("CSRF cookie not found")
		}

		formData := url.Values{
			"url":      {"https://example.com"},
			"category": {"test"},
		}

		// First request should be processed
		req1 := httptest.NewRequest("POST", "/submit", strings.NewReader(formData.Encode()))
		req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req1.Header.Set("X-CSRF-Token", csrfCookie.Value)
		req1.AddCookie(csrfCookie)
		w1 := httptest.NewRecorder()
		testMux.ServeHTTP(w1, req1)

		if w1.Code == http.StatusTooManyRequests {
			t.Error("First request should not be rate limited")
		}

		// Second request should be rate limited
		req2 := httptest.NewRequest("POST", "/submit", strings.NewReader(formData.Encode()))
		req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req2.Header.Set("X-CSRF-Token", csrfCookie.Value)
		req2.AddCookie(csrfCookie)
		w2 := httptest.NewRecorder()
		testMux.ServeHTTP(w2, req2)

		if w2.Code != http.StatusTooManyRequests {
			t.Errorf("Expected second request to be rate limited, got status %d", w2.Code)
		}
	})
}

// TestEndToEndWorkflow tests the complete user workflow
func TestEndToEndWorkflow(t *testing.T) {
	conf := config.Config{
		RSSReaderEndpoint: "https://test.example.com",
		RSSReaderAPIKey:   "test-key",
	}

	server := NewServer(conf, false)
	mux := server.SetupRoutes()

	// Step 1: User visits the homepage and gets a CSRF cookie
	var csrfCookie *http.Cookie
	t.Run("Step 1: Visit homepage", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Failed to load homepage: status %d", w.Code)
		}

		csrfCookie = getCSRFCookie(w)
		if csrfCookie == nil {
			t.Fatal("CSRF cookie not set on homepage load")
		}
	})

	// Step 2: User submits form with validation errors
	t.Run("Step 2: Submit invalid form", func(t *testing.T) {
		if csrfCookie == nil {
			t.Fatal("CSRF cookie not available for test")
		}
		// Submit form with empty URL
		formData := url.Values{"url": {""}, "category": {"test"}}

		req := httptest.NewRequest("POST", "/submit", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-CSRF-Token", csrfCookie.Value)
		req.AddCookie(csrfCookie)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected validation error, got status %d", w.Code)
		}
	})

	// Step 3: User corrects form and submits valid data
	t.Run("Step 3: Submit valid form", func(t *testing.T) {
		if csrfCookie == nil {
			t.Fatal("CSRF cookie not available for test")
		}
		formData := url.Values{
			"url":      {"https://test-success.example.com"},
			"category": {"test"},
		}

		req := httptest.NewRequest("POST", "/submit", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-CSRF-Token", csrfCookie.Value)
		req.AddCookie(csrfCookie)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK for valid submission, got %d", w.Code)
		}

		var response SubmitResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to parse JSON response: %v", err)
		}

		if !response.Success {
			t.Error("Expected success for valid submission")
		}
	})
}

// TestCategoriesEndpoint tests the categories API endpoint
func TestCategoriesEndpoint(t *testing.T) {
	t.Run("Test environment categories", func(t *testing.T) {
		// Create test server with test configuration
		conf := config.Config{
			RSSReaderEndpoint: "https://test.example.com",
			RSSReaderAPIKey:   "test-key",
		}
		server := NewServer(conf, false)
		mux := server.SetupRoutes()

		testServer := httptest.NewServer(mux)
		defer testServer.Close()

		resp, err := http.Get(testServer.URL + "/categories")
		if err != nil {
			t.Fatalf("Failed to get categories: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		var response CategoryResponse
		if err := json.Unmarshal(body, &response); err != nil {
			t.Fatalf("Failed to parse JSON response: %v", err)
		}

		if !response.Success {
			t.Error("Categories response should be successful")
		}

		if len(response.Categories) == 0 {
			t.Error("Should have test categories")
		}
	})
}
