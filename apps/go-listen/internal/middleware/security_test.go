package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestSecurityHeaders(t *testing.T) {
	logger := log.New()
	logger.SetLevel(log.ErrorLevel) // Reduce noise in tests
	rateLimiter := NewRateLimiter(10, 20)
	sm := NewSecurityMiddleware(logger, rateLimiter)

	handler := sm.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check security headers
	expectedHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"X-XSS-Protection":        "1; mode=block",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Content-Security-Policy": "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self' https://api.spotify.com; frame-src https://open.spotify.com;",
	}

	for header, expectedValue := range expectedHeaders {
		if got := w.Header().Get(header); got != expectedValue {
			t.Errorf("Expected header %s to be %s, got %s", header, expectedValue, got)
		}
	}

	// HSTS should not be set for HTTP requests
	if hsts := w.Header().Get("Strict-Transport-Security"); hsts != "" {
		t.Errorf("Expected HSTS header to be empty for HTTP request, got %s", hsts)
	}
}

func TestRateLimit(t *testing.T) {
	logger := log.New()
	logger.SetLevel(log.ErrorLevel)
	rateLimiter := NewRateLimiter(2, 2) // Very low limits for testing
	sm := NewSecurityMiddleware(logger, rateLimiter)

	handler := sm.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", http.NoBody)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should have succeeded, got status %d", i+1, w.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/", http.NoBody)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected rate limit status %d, got %d", http.StatusTooManyRequests, w.Code)
	}

	if retryAfter := w.Header().Get("Retry-After"); retryAfter != "60" {
		t.Errorf("Expected Retry-After header to be 60, got %s", retryAfter)
	}
}

func TestInputValidation(t *testing.T) {
	logger := log.New()
	logger.SetLevel(log.ErrorLevel)
	rateLimiter := NewRateLimiter(10, 20)
	sm := NewSecurityMiddleware(logger, rateLimiter)

	handler := sm.InputValidation(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name           string
		path           string
		query          string
		expectedStatus int
	}{
		{
			name:           "normal request",
			path:           "/api/test",
			query:          "name=john&age=25",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "path traversal attempt",
			path:           "/api/../../../etc/passwd",
			query:          "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "XSS attempt in query",
			path:           "/api/test",
			query:          "name=<script>alert('xss')</script>",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "SQL injection attempt",
			path:           "/api/test",
			query:          "id=1%27%20OR%20%271%27%3D%271", // URL encoded: id=1' OR '1'='1
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "JavaScript injection",
			path:           "/api/test",
			query:          "callback=javascript:alert(1)",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.path
			if tt.query != "" {
				url += "?" + tt.query
			}

			req := httptest.NewRequest("GET", url, http.NoBody)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestCSRFProtection(t *testing.T) {
	logger := log.New()
	logger.SetLevel(log.ErrorLevel)
	rateLimiter := NewRateLimiter(10, 20)
	sm := NewSecurityMiddleware(logger, rateLimiter)

	handler := sm.CSRFProtection(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// GET request should not require CSRF token
	req := httptest.NewRequest("GET", "/", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET request should not require CSRF token, got status %d", w.Code)
	}

	// POST request without CSRF token should be rejected
	req = httptest.NewRequest("POST", "/", http.NoBody)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("POST request without CSRF token should be forbidden, got status %d", w.Code)
	}

	// POST request with valid CSRF token should succeed
	token := sm.GenerateCSRFToken()
	if token == "" {
		t.Fatal("Failed to generate CSRF token")
	}

	req = httptest.NewRequest("POST", "/", http.NoBody)
	req.Header.Set("X-CSRF-Token", token)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST request with valid CSRF token should succeed, got status %d", w.Code)
	}

	// POST request with invalid CSRF token should be rejected
	req = httptest.NewRequest("POST", "/", http.NoBody)
	req.Header.Set("X-CSRF-Token", "invalid-token")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("POST request with invalid CSRF token should be forbidden, got status %d", w.Code)
	}
}

func TestCSRFTokenGeneration(t *testing.T) {
	logger := log.New()
	logger.SetLevel(log.ErrorLevel)
	rateLimiter := NewRateLimiter(10, 20)
	sm := NewSecurityMiddleware(logger, rateLimiter)

	// Generate multiple tokens
	tokens := make([]string, 10)
	for i := 0; i < 10; i++ {
		tokens[i] = sm.GenerateCSRFToken()
		if tokens[i] == "" {
			t.Errorf("Failed to generate CSRF token %d", i)
		}
	}

	// All tokens should be unique
	for i := 0; i < len(tokens); i++ {
		for j := i + 1; j < len(tokens); j++ {
			if tokens[i] == tokens[j] {
				t.Errorf("Generated duplicate CSRF tokens: %s", tokens[i])
			}
		}
	}

	// Tokens should be valid initially
	for i, token := range tokens {
		if !sm.validateCSRFToken(token) {
			t.Errorf("Token %d should be valid: %s", i, token)
		}
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "direct connection",
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{},
			expected:   "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For header",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.1, 10.0.0.1"},
			expected:   "203.0.113.1",
		},
		{
			name:       "X-Real-IP header",
			remoteAddr: "10.0.0.1:12345",
			headers:    map[string]string{"X-Real-IP": "203.0.113.2"},
			expected:   "203.0.113.2",
		},
		{
			name:       "X-Forwarded-For takes precedence",
			remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1",
				"X-Real-IP":       "203.0.113.2",
			},
			expected: "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", http.NoBody)
			req.RemoteAddr = tt.remoteAddr

			for header, value := range tt.headers {
				req.Header.Set(header, value)
			}

			ip := getClientIP(req)
			if ip != tt.expected {
				t.Errorf("Expected IP %s, got %s", tt.expected, ip)
			}
		})
	}
}

func TestContainsSuspiciousPatterns(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"normal text", false},
		{"user@example.com", false},
		{"<script>alert('xss')</script>", true},
		{"javascript:alert(1)", true},
		{"../../../etc/passwd", true},
		{"SELECT * FROM users", true},
		{"1' OR '1'='1", true},
		{"onload=malicious()", true},
		{"normal-file.txt", false},
		{"file with spaces.txt", false},
		{"DROP TABLE users", true},
		{"<?php echo 'test'; ?>", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := containsSuspiciousPatterns(tt.input)
			if result != tt.expected {
				t.Errorf("Input %q: expected %v, got %v", tt.input, tt.expected, result)
			}
		})
	}
}

func TestRateLimiterCleanup(t *testing.T) {
	// This test is more complex and would require mocking time
	// For now, we'll just test basic functionality
	rl := NewRateLimiter(10, 10)

	// Add some visitors
	rl.Allow("192.168.1.1")
	rl.Allow("192.168.1.2")

	if len(rl.visitors) != 2 {
		t.Errorf("Expected 2 visitors, got %d", len(rl.visitors))
	}

	// Reset one visitor
	rl.Reset("192.168.1.1")

	if len(rl.visitors) != 1 {
		t.Errorf("Expected 1 visitor after reset, got %d", len(rl.visitors))
	}
}
