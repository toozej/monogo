package web

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGenerateCSRFToken(t *testing.T) {
	token1, err := GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken returned error: %v", err)
	}
	if token1 == "" {
		t.Error("Expected token to be non-empty")
	}

	token2, err := GenerateCSRFToken()
	if err != nil {
		t.Fatalf("Second GenerateCSRFToken returned error: %v", err)
	}
	if token1 == token2 {
		t.Error("Expected different tokens to be generated")
	}

	if strings.Contains(token1, " ") {
		t.Error("Token should not contain spaces")
	}
}

func TestNewRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(5, time.Minute)

	if limiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	if limiter.limit != 5 {
		t.Errorf("Expected limit to be 5, got %d", limiter.limit)
	}

	if limiter.window != time.Minute {
		t.Errorf("Expected window to be 1 minute, got %v", limiter.window)
	}

	if limiter.requests == nil {
		t.Error("Expected requests map to be initialized")
	}
}

func TestRateLimiterBasicFunctionality(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	ip := "192.168.1.1"

	// First request should be allowed
	if !limiter.IsAllowed(ip) {
		t.Error("Expected first request to be allowed")
	}

	// Second request should be allowed
	if !limiter.IsAllowed(ip) {
		t.Error("Expected second request to be allowed")
	}

	// Third request should be blocked (limit is 2)
	if limiter.IsAllowed(ip) {
		t.Error("Expected third request to be blocked")
	}
}

func TestRateLimiterDifferentIPs(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)

	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"

	// First IP should be allowed
	if !limiter.IsAllowed(ip1) {
		t.Error("Expected first IP to be allowed")
	}

	// Second IP should also be allowed (different IP)
	if !limiter.IsAllowed(ip2) {
		t.Error("Expected second IP to be allowed")
	}

	// First IP should now be blocked (already used its quota)
	if limiter.IsAllowed(ip1) {
		t.Error("Expected first IP to be blocked on second request")
	}
}

func TestRateLimiterTimeWindow(t *testing.T) {
	// Use a very short window for testing
	limiter := NewRateLimiter(1, 100*time.Millisecond)
	ip := "192.168.1.1"

	// First request should be allowed
	if !limiter.IsAllowed(ip) {
		t.Error("Expected first request to be allowed")
	}

	// Second request should be blocked
	if limiter.IsAllowed(ip) {
		t.Error("Expected second request to be blocked")
	}

	// Wait for the window to expire
	time.Sleep(150 * time.Millisecond)

	// Request should be allowed again after window expires
	if !limiter.IsAllowed(ip) {
		t.Error("Expected request to be allowed after window expiry")
	}
}

func TestGetClientIP(t *testing.T) {
	testCases := []struct {
		description string
		headers     map[string]string
		remoteAddr  string
		expected    string
	}{
		{
			description: "X-Forwarded-For single IP",
			headers:     map[string]string{"X-Forwarded-For": "1.1.1.1"},
			remoteAddr:  "10.0.0.1:12345",
			expected:    "1.1.1.1",
		},
		{
			description: "X-Forwarded-For multiple IPs",
			headers:     map[string]string{"X-Forwarded-For": "1.1.1.1, 2.2.2.2"},
			remoteAddr:  "10.0.0.1:12345",
			expected:    "1.1.1.1",
		},
		{
			description: "X-Forwarded-For with spaces",
			headers:     map[string]string{"X-Forwarded-For": "  1.1.1.1  , 2.2.2.2"},
			remoteAddr:  "10.0.0.1:12345",
			expected:    "1.1.1.1",
		},
		{
			description: "X-Real-IP header",
			headers:     map[string]string{"X-Real-IP": "3.3.3.3"},
			remoteAddr:  "10.0.0.1:12345",
			expected:    "3.3.3.3",
		},
		{
			description: "X-Real-IP with spaces",
			headers:     map[string]string{"X-Real-IP": "  3.3.3.3  "},
			remoteAddr:  "10.0.0.1:12345",
			expected:    "3.3.3.3",
		},
		{
			description: "X-Forwarded-For takes precedence over X-Real-IP",
			headers:     map[string]string{"X-Forwarded-For": "1.1.1.1", "X-Real-IP": "3.3.3.3"},
			remoteAddr:  "10.0.0.1:12345",
			expected:    "1.1.1.1",
		},
		{
			description: "RemoteAddr as fallback",
			headers:     map[string]string{},
			remoteAddr:  "4.4.4.4:12345",
			expected:    "4.4.4.4",
		},
		{
			description: "RemoteAddr without port",
			headers:     map[string]string{},
			remoteAddr:  "4.4.4.4",
			expected:    "4.4.4.4",
		},
		{
			description: "IPv6 in X-Forwarded-For",
			headers:     map[string]string{"X-Forwarded-For": "2001:db8::1"},
			remoteAddr:  "10.0.0.1:12345",
			expected:    "2001:db8::1",
		},
		{
			description: "IPv6 in RemoteAddr",
			headers:     map[string]string{},
			remoteAddr:  "[2001:db8::1]:12345",
			expected:    "2001:db8::1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}
			req.RemoteAddr = tc.remoteAddr

			ip := getClientIP(req)
			if ip != tc.expected {
				t.Errorf("Expected IP %q, got %q", tc.expected, ip)
			}
		})
	}
}

func TestRateLimiterConcurrency(t *testing.T) {
	limiter := NewRateLimiter(5, time.Minute)
	ip := "192.168.1.1"

	// Test concurrent requests
	resultChan := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			resultChan <- limiter.IsAllowed(ip)
		}()
	}

	// Collect results
	allowedCount := 0
	for i := 0; i < 10; i++ {
		select {
		case allowed := <-resultChan:
			if allowed {
				allowedCount++
			}
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for rate limit check")
		}
	}

	// Should allow exactly 5 requests (the limit)
	if allowedCount != 5 {
		t.Errorf("Expected 5 requests to be allowed, got %d", allowedCount)
	}
}
