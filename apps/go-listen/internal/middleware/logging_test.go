package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/toozej/go-listen/pkg/config"
	"github.com/toozej/go-listen/pkg/logging"
)

func TestNewLoggingMiddleware(t *testing.T) {
	logger := logging.NewLogger(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	middleware := NewLoggingMiddleware(logger)

	if middleware == nil {
		t.Fatal("NewLoggingMiddleware returned nil")
	}

	if middleware.logger != logger {
		t.Error("LoggingMiddleware logger not set correctly")
	}
}

func TestLoggingMiddleware_LogRequests(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewLogger(config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})
	logger.SetOutput(&buf)

	middleware := NewLoggingMiddleware(logger)

	// Create a test handler that returns 200 OK
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify correlation ID is in context
		correlationID := r.Context().Value(logging.CorrelationIDKey)
		if correlationID == nil {
			t.Error("Expected correlation ID in request context")
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap the handler with logging middleware
	wrappedHandler := middleware.LogRequests(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/test?param=value", http.NoBody)
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "192.168.1.1:12345"

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Execute the request
	wrappedHandler.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", rr.Code)
	}

	// Verify correlation ID header is set
	correlationID := rr.Header().Get("X-Correlation-ID")
	if correlationID == "" {
		t.Error("Expected X-Correlation-ID header to be set")
	}

	// Parse log output - should have multiple log entries
	logOutput := buf.String()
	logLines := strings.Split(strings.TrimSpace(logOutput), "\n")

	if len(logLines) < 2 {
		t.Fatalf("Expected at least 2 log entries, got %d", len(logLines))
	}

	// Check the request completion log (last entry)
	var completionLogEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logLines[len(logLines)-1]), &completionLogEntry); err != nil {
		t.Fatalf("Failed to parse completion log entry as JSON: %v", err)
	}

	// Verify completion log fields
	expectedFields := map[string]interface{}{
		"component":   "http",
		"operation":   "api_request",
		"method":      "GET",
		"path":        "/test",
		"status_code": float64(200),
		"level":       "info",
	}

	for key, expectedValue := range expectedFields {
		if completionLogEntry[key] != expectedValue {
			t.Errorf("Expected %s to be %v, got %v", key, expectedValue, completionLogEntry[key])
		}
	}

	// Verify correlation ID is consistent
	if completionLogEntry["correlation_id"] != correlationID {
		t.Errorf("Expected correlation_id in log to match header: %s != %s",
			completionLogEntry["correlation_id"], correlationID)
	}

	// Verify duration is present and reasonable
	duration, ok := completionLogEntry["duration_ms"].(float64)
	if !ok {
		t.Error("Expected duration_ms to be a number")
	}
	if duration < 0 || duration > 1000 { // Should be very fast for test
		t.Errorf("Expected reasonable duration, got %f ms", duration)
	}
}

func TestLoggingMiddleware_LogRequests_WithError(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewLogger(config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})
	logger.SetOutput(&buf)

	middleware := NewLoggingMiddleware(logger)

	// Create a test handler that returns 500 error
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	})

	// Wrap the handler with logging middleware
	wrappedHandler := middleware.LogRequests(testHandler)

	// Create a test request
	req := httptest.NewRequest("POST", "/api/test", http.NoBody)
	rr := httptest.NewRecorder()

	// Execute the request
	wrappedHandler.ServeHTTP(rr, req)

	// Parse log output
	logOutput := buf.String()
	logLines := strings.Split(strings.TrimSpace(logOutput), "\n")

	// Check the completion log entry
	var completionLogEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logLines[len(logLines)-1]), &completionLogEntry); err != nil {
		t.Fatalf("Failed to parse completion log entry as JSON: %v", err)
	}

	// Verify error-level logging for 500 status
	if completionLogEntry["level"] != "error" {
		t.Errorf("Expected error level for 500 status, got %v", completionLogEntry["level"])
	}

	if completionLogEntry["status_code"] != float64(500) {
		t.Errorf("Expected status_code 500, got %v", completionLogEntry["status_code"])
	}
}

func TestLoggingMiddleware_LogRequests_WithClientError(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewLogger(config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})
	logger.SetOutput(&buf)

	middleware := NewLoggingMiddleware(logger)

	// Create a test handler that returns 400 error
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Bad Request"))
	})

	// Wrap the handler with logging middleware
	wrappedHandler := middleware.LogRequests(testHandler)

	// Create a test request
	req := httptest.NewRequest("POST", "/api/test", http.NoBody)
	rr := httptest.NewRecorder()

	// Execute the request
	wrappedHandler.ServeHTTP(rr, req)

	// Parse log output
	logOutput := buf.String()
	logLines := strings.Split(strings.TrimSpace(logOutput), "\n")

	// Check the completion log entry
	var completionLogEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logLines[len(logLines)-1]), &completionLogEntry); err != nil {
		t.Fatalf("Failed to parse completion log entry as JSON: %v", err)
	}

	// Verify warning-level logging for 400 status
	if completionLogEntry["level"] != "warning" {
		t.Errorf("Expected warning level for 400 status, got %v", completionLogEntry["level"])
	}

	if completionLogEntry["status_code"] != float64(400) {
		t.Errorf("Expected status_code 400, got %v", completionLogEntry["status_code"])
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rr,
		statusCode:     0,
		written:        false,
	}

	// Test writing header
	rw.WriteHeader(http.StatusCreated)

	if rw.statusCode != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, rw.statusCode)
	}

	if !rw.written {
		t.Error("Expected written to be true after WriteHeader")
	}

	// Test that subsequent WriteHeader calls don't change status
	rw.WriteHeader(http.StatusBadRequest)

	if rw.statusCode != http.StatusCreated {
		t.Errorf("Expected status code to remain %d, got %d", http.StatusCreated, rw.statusCode)
	}
}

func TestResponseWriter_Write(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rr,
		statusCode:     0,
		written:        false,
	}

	// Test writing data without explicit WriteHeader
	data := []byte("test data")
	n, err := rw.Write(data)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	if rw.statusCode != http.StatusOK {
		t.Errorf("Expected default status code %d, got %d", http.StatusOK, rw.statusCode)
	}

	if !rw.written {
		t.Error("Expected written to be true after Write")
	}
}

func TestGenerateCorrelationID(t *testing.T) {
	// Test that correlation IDs are generated
	id1 := generateCorrelationID()
	id2 := generateCorrelationID()

	if id1 == "" {
		t.Error("Expected non-empty correlation ID")
	}

	if id2 == "" {
		t.Error("Expected non-empty correlation ID")
	}

	// Test that IDs are unique
	if id1 == id2 {
		t.Error("Expected unique correlation IDs")
	}

	// Test that IDs are hex encoded (should be even length and contain only hex chars)
	if len(id1)%2 != 0 {
		t.Error("Expected correlation ID to be hex encoded (even length)")
	}

	for _, char := range id1 {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			t.Errorf("Expected correlation ID to contain only hex characters, found: %c", char)
		}
	}
}

func TestLoggingMiddleware_ContextPropagation(t *testing.T) {
	logger := logging.NewLogger(config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})

	middleware := NewLoggingMiddleware(logger)

	var capturedCorrelationID string

	// Create a test handler that captures the correlation ID from context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if correlationID, ok := r.Context().Value(logging.CorrelationIDKey).(string); ok {
			capturedCorrelationID = correlationID
		}
		w.WriteHeader(http.StatusOK)
	})

	// Wrap the handler with logging middleware
	wrappedHandler := middleware.LogRequests(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/test", http.NoBody)
	rr := httptest.NewRecorder()

	// Execute the request
	wrappedHandler.ServeHTTP(rr, req)

	// Verify correlation ID was captured
	if capturedCorrelationID == "" {
		t.Error("Expected correlation ID to be propagated to handler context")
	}

	// Verify correlation ID matches response header
	headerCorrelationID := rr.Header().Get("X-Correlation-ID")
	if capturedCorrelationID != headerCorrelationID {
		t.Errorf("Expected context correlation ID %s to match header %s",
			capturedCorrelationID, headerCorrelationID)
	}
}
