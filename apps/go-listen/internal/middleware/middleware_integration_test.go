package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/pkg/config"
	"github.com/toozej/go-listen/pkg/logging"
)

// TestMiddlewareIntegration tests the integration of all middleware components
func TestMiddlewareIntegration(t *testing.T) {
	// Set up logger with buffer to capture output
	var logBuffer bytes.Buffer
	logger := logging.NewLogger(config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})
	logger.SetOutput(&logBuffer)

	// Set up middleware components
	rateLimiter := NewRateLimiter(5, 10) // 5 requests per second, burst of 10
	securityMiddleware := NewSecurityMiddleware(logger.Logger, rateLimiter)
	loggingMiddleware := NewLoggingMiddleware(logger)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	})

	// Chain all middleware
	handler := loggingMiddleware.LogRequests(
		securityMiddleware.SecurityHeaders(
			securityMiddleware.RateLimit(
				securityMiddleware.InputValidation(
					securityMiddleware.CSRFProtection(testHandler),
				),
			),
		),
	)

	tests := []struct {
		name           string
		method         string
		path           string
		headers        map[string]string
		expectedStatus int
		checkLogs      bool
		checkHeaders   bool
	}{
		{
			name:           "successful_get_request",
			method:         "GET",
			path:           "/api/test",
			expectedStatus: http.StatusOK,
			checkLogs:      true,
			checkHeaders:   true,
		},
		{
			name:   "successful_post_with_csrf",
			method: "POST",
			path:   "/api/test",
			headers: map[string]string{
				"X-CSRF-Token": securityMiddleware.GenerateCSRFToken(),
			},
			expectedStatus: http.StatusOK,
			checkLogs:      true,
			checkHeaders:   true,
		},
		{
			name:           "blocked_post_without_csrf",
			method:         "POST",
			path:           "/api/test",
			expectedStatus: http.StatusForbidden,
			checkLogs:      true,
		},
		{
			name:           "blocked_malicious_input",
			method:         "GET",
			path:           "/api/test?param=<script>alert('xss')</script>",
			expectedStatus: http.StatusBadRequest,
			checkLogs:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear log buffer
			logBuffer.Reset()

			req := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			req.RemoteAddr = "192.168.1.1:12345"

			// Add headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check security headers
			if tt.checkHeaders {
				expectedHeaders := map[string]string{
					"X-Content-Type-Options": "nosniff",
					"X-Frame-Options":        "DENY",
					"X-XSS-Protection":       "1; mode=block",
				}

				for header, expectedValue := range expectedHeaders {
					if got := w.Header().Get(header); got != expectedValue {
						t.Errorf("Expected header %s to be %s, got %s", header, expectedValue, got)
					}
				}

				// Check correlation ID header
				if correlationID := w.Header().Get("X-Correlation-ID"); correlationID == "" {
					t.Error("Expected X-Correlation-ID header to be set")
				}
			}

			// Check logs
			if tt.checkLogs {
				logOutput := logBuffer.String()
				if logOutput == "" {
					t.Error("Expected log output but got none")
				}

				// Parse log entries
				logLines := strings.Split(strings.TrimSpace(logOutput), "\n")
				if len(logLines) < 1 {
					t.Error("Expected at least one log entry")
				}

				// Check that logs contain expected fields
				for _, line := range logLines {
					var logEntry map[string]interface{}
					if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
						continue // Skip non-JSON lines
					}

					// Verify common log fields
					if logEntry["component"] == nil {
						t.Error("Expected 'component' field in log entry")
					}
					if logEntry["timestamp"] == nil {
						t.Error("Expected 'timestamp' field in log entry")
					}
				}
			}
		})
	}
}

// TestMiddlewareRateLimitingIntegration tests rate limiting across multiple requests
func TestMiddlewareRateLimitingIntegration(t *testing.T) {
	logger := log.New()
	logger.SetLevel(log.ErrorLevel)

	// Very restrictive rate limiting for testing
	rateLimiter := NewRateLimiter(2, 2)
	securityMiddleware := NewSecurityMiddleware(logger, rateLimiter)

	handler := securityMiddleware.RateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make requests from the same IP
	clientIP := "192.168.1.1:12345"

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.RemoteAddr = clientIP
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should have succeeded, got status %d", i+1, w.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.RemoteAddr = clientIP
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected rate limit status %d, got %d", http.StatusTooManyRequests, w.Code)
	}

	// Request from different IP should succeed
	req = httptest.NewRequest("GET", "/test", http.NoBody)
	req.RemoteAddr = "192.168.1.2:12345"
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Request from different IP should have succeeded, got status %d", w.Code)
	}
}

// TestMiddlewareConcurrentAccess tests middleware behavior under concurrent load
func TestMiddlewareConcurrentAccess(t *testing.T) {
	logger := log.New()
	logger.SetLevel(log.ErrorLevel)

	rateLimiter := NewRateLimiter(100, 200) // Higher limits for concurrent testing
	securityMiddleware := NewSecurityMiddleware(logger, rateLimiter)
	loggingMiddleware := NewLoggingMiddleware(&logging.Logger{Logger: logger})

	handler := loggingMiddleware.LogRequests(
		securityMiddleware.SecurityHeaders(
			securityMiddleware.RateLimit(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Simulate some processing time
					time.Sleep(1 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
				}),
			),
		),
	)

	// Test concurrent requests
	numGoroutines := 50
	numRequestsPerGoroutine := 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numRequestsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numRequestsPerGoroutine; j++ {
				req := httptest.NewRequest("GET", "/test", http.NoBody)
				req.RemoteAddr = "192.168.1." + string(rune(goroutineID%255)) + ":12345"
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				// Most requests should succeed (some might be rate limited)
				if w.Code != http.StatusOK && w.Code != http.StatusTooManyRequests {
					errors <- fmt.Errorf("unexpected status code: %d", w.Code)
				}

				// Check that correlation ID is set
				if w.Header().Get("X-Correlation-ID") == "" {
					errors <- fmt.Errorf("missing correlation ID")
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Errorf("Got %d errors out of %d total requests", errorCount, numGoroutines*numRequestsPerGoroutine)
	}
}

// TestMiddlewareSecurityScenarios tests various security attack scenarios
func TestMiddlewareSecurityScenarios(t *testing.T) {
	logger := log.New()
	logger.SetLevel(log.ErrorLevel)

	rateLimiter := NewRateLimiter(10, 20)
	securityMiddleware := NewSecurityMiddleware(logger, rateLimiter)

	handler := securityMiddleware.InputValidation(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	attackScenarios := []struct {
		name           string
		path           string
		expectedStatus int
		description    string
	}{
		{
			name:           "xss_script_tag",
			path:           "/test?param=<script>alert('xss')</script>",
			expectedStatus: http.StatusBadRequest,
			description:    "XSS attempt with script tag",
		},
		{
			name:           "sql_injection_union",
			path:           "/test?id=1%27%20UNION%20SELECT%20*%20FROM%20users--",
			expectedStatus: http.StatusBadRequest,
			description:    "SQL injection with UNION",
		},
		{
			name:           "path_traversal_dots",
			path:           "/test/../../../etc/passwd",
			expectedStatus: http.StatusBadRequest,
			description:    "Path traversal with dots",
		},
		{
			name:           "javascript_protocol",
			path:           "/test?callback=javascript:alert(1)",
			expectedStatus: http.StatusBadRequest,
			description:    "JavaScript protocol injection",
		},
		{
			name:           "php_code_injection",
			path:           "/test?code=%3C%3Fphp%20system%28%27rm%20-rf%20/%27%29%3B%20%3F%3E",
			expectedStatus: http.StatusBadRequest,
			description:    "PHP code injection",
		},
		{
			name:           "command_injection",
			path:           "/test?cmd=%3B%20rm%20-rf%20/",
			expectedStatus: http.StatusBadRequest,
			description:    "Command injection attempt",
		},
		{
			name:           "legitimate_request",
			path:           "/test?name=john&age=25",
			expectedStatus: http.StatusOK,
			description:    "Legitimate request should pass",
		},
	}

	for _, scenario := range attackScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", scenario.path, http.NoBody)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != scenario.expectedStatus {
				t.Errorf("%s: expected status %d, got %d",
					scenario.description, scenario.expectedStatus, w.Code)
			}
		})
	}
}

// TestMiddlewareCSRFIntegration tests CSRF protection integration
func TestMiddlewareCSRFIntegration(t *testing.T) {
	logger := log.New()
	logger.SetLevel(log.ErrorLevel)

	rateLimiter := NewRateLimiter(10, 20)
	securityMiddleware := NewSecurityMiddleware(logger, rateLimiter)

	handler := securityMiddleware.CSRFProtection(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// Test CSRF token generation and validation
	token1 := securityMiddleware.GenerateCSRFToken()
	token2 := securityMiddleware.GenerateCSRFToken()

	if token1 == "" || token2 == "" {
		t.Fatal("Failed to generate CSRF tokens")
	}

	if token1 == token2 {
		t.Error("CSRF tokens should be unique")
	}

	tests := []struct {
		name           string
		method         string
		csrfToken      string
		expectedStatus int
	}{
		{
			name:           "get_request_no_token",
			method:         "GET",
			csrfToken:      "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "post_request_valid_token",
			method:         "POST",
			csrfToken:      token1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "post_request_no_token",
			method:         "POST",
			csrfToken:      "",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "post_request_invalid_token",
			method:         "POST",
			csrfToken:      "invalid-token",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "put_request_valid_token",
			method:         "PUT",
			csrfToken:      token2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "delete_request_no_token",
			method:         "DELETE",
			csrfToken:      "",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", http.NoBody)
			if tt.csrfToken != "" {
				req.Header.Set("X-CSRF-Token", tt.csrfToken)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestMiddlewareLoggingIntegration tests logging integration across middleware
func TestMiddlewareLoggingIntegration(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := logging.NewLogger(config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})
	logger.SetOutput(&logBuffer)

	rateLimiter := NewRateLimiter(10, 20)
	securityMiddleware := NewSecurityMiddleware(logger.Logger, rateLimiter)
	loggingMiddleware := NewLoggingMiddleware(logger)

	handler := loggingMiddleware.LogRequests(
		securityMiddleware.SecurityHeaders(
			securityMiddleware.InputValidation(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Log something from the handler
					logger.WithField("handler", "test").Info("Handler executed")
					w.WriteHeader(http.StatusOK)
				}),
			),
		),
	)

	req := httptest.NewRequest("GET", "/test?param=value", http.NoBody)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("User-Agent", "test-agent")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Analyze log output
	logOutput := logBuffer.String()
	logLines := strings.Split(strings.TrimSpace(logOutput), "\n")

	if len(logLines) < 2 {
		t.Errorf("Expected at least 2 log entries, got %d", len(logLines))
	}

	// Check for correlation ID consistency
	var correlationID string
	for _, line := range logLines {
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			continue
		}

		if cid, ok := logEntry["correlation_id"].(string); ok {
			if correlationID == "" {
				correlationID = cid
			} else if correlationID != cid {
				t.Error("Correlation ID should be consistent across log entries")
			}
		}
	}

	if correlationID == "" {
		t.Error("Expected correlation ID in log entries")
	}

	// Check that correlation ID matches response header
	responseCorrelationID := w.Header().Get("X-Correlation-ID")
	if responseCorrelationID != correlationID {
		t.Errorf("Response correlation ID %s doesn't match log correlation ID %s",
			responseCorrelationID, correlationID)
	}
}

// TestMiddlewareErrorHandling tests error handling across middleware chain
func TestMiddlewareErrorHandling(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := logging.NewLogger(config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})
	logger.SetOutput(&logBuffer)

	rateLimiter := NewRateLimiter(10, 20)
	securityMiddleware := NewSecurityMiddleware(logger.Logger, rateLimiter)
	loggingMiddleware := NewLoggingMiddleware(logger)

	// Handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Handler that returns error status
	errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	tests := []struct {
		name           string
		handler        http.Handler
		expectedStatus int
		shouldPanic    bool
	}{
		{
			name:           "panic_handler",
			handler:        panicHandler,
			expectedStatus: http.StatusInternalServerError,
			shouldPanic:    true,
		},
		{
			name:           "error_handler",
			handler:        errorHandler,
			expectedStatus: http.StatusInternalServerError,
			shouldPanic:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logBuffer.Reset()

			wrappedHandler := loggingMiddleware.LogRequests(
				securityMiddleware.SecurityHeaders(tt.handler),
			)

			req := httptest.NewRequest("GET", "/test", http.NoBody)
			w := httptest.NewRecorder()

			if tt.shouldPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("Expected panic but didn't get one")
					}
				}()
			}

			wrappedHandler.ServeHTTP(w, req)

			if !tt.shouldPanic && w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
