package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetAsset(t *testing.T) {
	testCases := []struct {
		assetPath   string
		expectError bool
		description string
	}{
		{"index.html", false, "Valid HTML asset"},
		{"style.css", false, "Valid CSS asset"},
		{"script.js", false, "Valid JS asset"},
		{"favicon.svg", false, "Valid SVG asset"},
		{"nonexistent.txt", true, "Nonexistent asset"},
		{"../../../etc/passwd", true, "Path traversal attempt"},
		{"", true, "Empty path"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			data, err := GetAsset(tc.assetPath)
			hasError := err != nil

			if hasError != tc.expectError {
				if tc.expectError {
					t.Errorf("Expected error for asset %q, but got none", tc.assetPath)
				} else {
					t.Errorf("Expected no error for asset %q, but got: %v", tc.assetPath, err)
				}
			}

			if !tc.expectError && len(data) == 0 {
				t.Errorf("Expected asset %q to have content", tc.assetPath)
			}
		})
	}
}

func TestGetAssetMimeType(t *testing.T) {
	testCases := []struct {
		assetPath    string
		expectedType string
	}{
		{"index.html", "text/html; charset=utf-8"},
		{"style.css", "text/css; charset=utf-8"},
		{"script.js", "application/javascript; charset=utf-8"},
		{"favicon.svg", "image/svg+xml"},
		{"icon.ico", "image/x-icon"},
		{"image.png", "image/png"},
		{"image.jpg", "image/jpeg"},
		{"image.jpeg", "image/jpeg"},
		{"image.gif", "image/gif"},
		{"data.json", "application/json"},
		{"readme.txt", "text/plain; charset=utf-8"},
		{"unknown.xyz", "application/octet-stream"},
	}

	for _, tc := range testCases {
		t.Run(tc.assetPath, func(t *testing.T) {
			mimeType := GetAssetMimeType(tc.assetPath)
			if mimeType != tc.expectedType {
				t.Errorf("Expected MIME type %q for %q, got %q", tc.expectedType, tc.assetPath, mimeType)
			}
		})
	}
}

func TestServeAsset(t *testing.T) {
	testCases := []struct {
		assetPath      string
		expectedStatus int
		checkContent   bool
		description    string
	}{
		{"index.html", http.StatusForbidden, false, "HTML templates should not be served as static assets"},
		{"style.css", http.StatusOK, true, "Valid CSS asset"},
		{"script.js", http.StatusOK, true, "Valid JS asset"},
		{"favicon.svg", http.StatusOK, true, "Valid SVG asset"},
		{"nonexistent.txt", http.StatusNotFound, false, "Nonexistent asset"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/static/"+tc.assetPath, nil)
			w := httptest.NewRecorder()

			ServeAsset(w, req, tc.assetPath)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.checkContent && tc.expectedStatus == http.StatusOK {
				// Check Content-Type header
				contentType := w.Header().Get("Content-Type")
				if contentType == "" {
					t.Error("Expected Content-Type header to be set")
				}

				// Check security headers
				if w.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Error("Expected X-Content-Type-Options header to be set to 'nosniff'")
				}

				if w.Header().Get("X-Frame-Options") != "DENY" {
					t.Error("Expected X-Frame-Options header to be set to 'DENY'")
				}

				// Check that content is not empty
				if w.Body.Len() == 0 {
					t.Error("Expected response body to have content")
				}

				// Check caching headers are set
				cacheControl := w.Header().Get("Cache-Control")
				if cacheControl == "" {
					t.Error("Expected Cache-Control header to be set")
				}
			}
		})
	}
}

func TestSetCachingHeaders(t *testing.T) {
	testCases := []struct {
		assetPath     string
		expectCaching bool
		description   string
	}{
		{"style.css", true, "CSS file should be cached"},
		{"script.js", true, "JS file should be cached"},
		{"favicon.svg", true, "SVG file should be cached"},
		{"icon.ico", true, "ICO file should be cached"},
		{"image.png", true, "PNG file should be cached"},
		{"index.html", false, "HTML file should not be cached"},
		{"unknown.txt", true, "Unknown file should have default caching"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			w := httptest.NewRecorder()
			setCachingHeaders(w, tc.assetPath)

			cacheControl := w.Header().Get("Cache-Control")
			if cacheControl == "" {
				t.Error("Expected Cache-Control header to be set")
			}

			if tc.expectCaching {
				if strings.Contains(cacheControl, "no-cache") {
					t.Errorf("Expected %s to be cached, but got no-cache", tc.assetPath)
				}
			} else {
				if !strings.Contains(cacheControl, "no-cache") {
					t.Errorf("Expected %s not to be cached, but got caching headers", tc.assetPath)
				}
			}
		})
	}
}

func TestSetSecurityHeadersAssets(t *testing.T) {
	w := httptest.NewRecorder()
	setSecurityHeaders(w)

	expectedHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
	}

	for _, header := range expectedHeaders {
		if w.Header().Get(header) == "" {
			t.Errorf("Expected header %s to be set", header)
		}
	}

	// Test specific values
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Expected X-Content-Type-Options to be 'nosniff', got %s", w.Header().Get("X-Content-Type-Options"))
	}

	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("Expected X-Frame-Options to be 'DENY', got %s", w.Header().Get("X-Frame-Options"))
	}
}

func TestListAssets(t *testing.T) {
	assets, err := ListAssets()
	if err != nil {
		t.Fatalf("ListAssets returned error: %v", err)
	}

	if len(assets) == 0 {
		t.Error("Expected ListAssets to return at least one asset")
	}

	// Check that expected assets are present
	expectedAssets := []string{
		"assets/index.html",
		"assets/style.css",
		"assets/script.js",
		"assets/favicon.svg",
	}

	for _, expected := range expectedAssets {
		found := false
		for _, asset := range assets {
			if asset == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected asset %s not found in list", expected)
		}
	}
}

func TestAssetExists(t *testing.T) {
	testCases := []struct {
		assetPath string
		exists    bool
	}{
		{"index.html", true},
		{"style.css", true},
		{"script.js", true},
		{"favicon.svg", true},
		{"nonexistent.txt", false},
		{"../../../etc/passwd", false},
	}

	for _, tc := range testCases {
		t.Run(tc.assetPath, func(t *testing.T) {
			exists := AssetExists(tc.assetPath)
			if exists != tc.exists {
				t.Errorf("Expected AssetExists(%q) to be %v, got %v", tc.assetPath, tc.exists, exists)
			}
		})
	}
}

func TestServeAssetWithFallback(t *testing.T) {
	testCases := []struct {
		assetPath      string
		fallbackPath   string
		expectedStatus int
		description    string
	}{
		{"style.css", "script.js", http.StatusOK, "Existing asset should be served"},
		{"nonexistent.txt", "style.css", http.StatusOK, "Fallback should be served for missing asset"},
		{"nonexistent.txt", "also-missing.txt", http.StatusNotFound, "Both missing should return 404"},
		{"nonexistent.txt", "", http.StatusNotFound, "Empty fallback should return 404"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/static/"+tc.assetPath, nil)
			w := httptest.NewRecorder()

			ServeAssetWithFallback(w, req, tc.assetPath, tc.fallbackPath)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}
