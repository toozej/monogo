package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/RSSFFS/pkg/config"
)

func TestLogBuffer(t *testing.T) {
	buffer := NewLogBuffer(3)

	// Test empty buffer
	logs := buffer.GetRecent(10)
	if len(logs) != 0 {
		t.Errorf("Expected empty buffer, got %d entries", len(logs))
	}

	// Add some entries
	entry1 := LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "Test message 1",
	}
	entry2 := LogEntry{
		Timestamp: time.Now().Add(time.Second),
		Level:     "error",
		Message:   "Test message 2",
	}
	entry3 := LogEntry{
		Timestamp: time.Now().Add(2 * time.Second),
		Level:     "debug",
		Message:   "Test message 3",
	}

	buffer.Add(entry1)
	buffer.Add(entry2)
	buffer.Add(entry3)

	// Test getting recent entries
	logs = buffer.GetRecent(10)
	if len(logs) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(logs))
	}

	// Check order (should be chronological)
	if logs[0].Message != "Test message 1" {
		t.Errorf("Expected first message to be 'Test message 1', got '%s'", logs[0].Message)
	}
	if logs[2].Message != "Test message 3" {
		t.Errorf("Expected last message to be 'Test message 3', got '%s'", logs[2].Message)
	}

	// Test circular buffer behavior
	entry4 := LogEntry{
		Timestamp: time.Now().Add(3 * time.Second),
		Level:     "warn",
		Message:   "Test message 4",
	}
	buffer.Add(entry4)

	logs = buffer.GetRecent(10)
	if len(logs) != 3 {
		t.Errorf("Expected 3 entries after overflow, got %d", len(logs))
	}

	// First entry should now be "Test message 2" (entry1 was overwritten)
	if logs[0].Message != "Test message 2" {
		t.Errorf("Expected first message after overflow to be 'Test message 2', got '%s'", logs[0].Message)
	}
}

func TestWebUIHook(t *testing.T) {
	hook := NewWebUIHook(10)

	// Create a test log entry
	entry := &log.Entry{
		Time:    time.Now(),
		Level:   log.InfoLevel,
		Message: "Test log message",
		Data: log.Fields{
			"key1": "value1",
			"key2": 42,
		},
	}

	// Fire the hook
	err := hook.Fire(entry)
	if err != nil {
		t.Errorf("Hook.Fire() returned error: %v", err)
	}

	// Check that the entry was added to the buffer
	logs := hook.GetBuffer().GetRecent(10)
	if len(logs) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(logs))
	}

	logEntry := logs[0]
	if logEntry.Message != "Test log message" {
		t.Errorf("Expected message 'Test log message', got '%s'", logEntry.Message)
	}
	if logEntry.Level != "info" {
		t.Errorf("Expected level 'info', got '%s'", logEntry.Level)
	}
	if logEntry.Fields["key1"] != "value1" {
		t.Errorf("Expected field key1='value1', got '%v'", logEntry.Fields["key1"])
	}
}

func TestHandleLogs(t *testing.T) {
	conf := config.Config{
		RSSReaderEndpoint: "https://test.example.com",
		RSSReaderAPIKey:   "test-key",
	}
	server := NewServer(conf, false)

	testCases := []struct {
		name           string
		method         string
		expectedStatus int
		expectJSON     bool
	}{
		{
			name:           "Valid GET request",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectJSON:     true,
		},
		{
			name:           "Invalid method",
			method:         "POST",
			expectedStatus: http.StatusMethodNotAllowed,
			expectJSON:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/logs", nil)
			w := httptest.NewRecorder()

			server.handleLogs(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectJSON {
				var response LogsResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				if err != nil {
					t.Errorf("Failed to decode JSON response: %v", err)
				}

				if !response.Success {
					t.Errorf("Expected successful response, got error: %s", response.Error)
				}
			}
		})
	}
}

func TestHandleLogsWithEntries(t *testing.T) {
	conf := config.Config{
		RSSReaderEndpoint: "https://test.example.com",
		RSSReaderAPIKey:   "test-key",
	}
	server := NewServer(conf, false)

	// Add some test log entries
	testEntry := &log.Entry{
		Time:    time.Now(),
		Level:   log.InfoLevel,
		Message: "Test log for API",
		Data:    log.Fields{},
	}
	if err := server.logHook.Fire(testEntry); err != nil {
		t.Errorf("Failed to fire log hook: %v", err)
	}

	req := httptest.NewRequest("GET", "/logs", nil)
	w := httptest.NewRecorder()

	server.handleLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response LogsResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Errorf("Failed to decode JSON response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected successful response, got error: %s", response.Error)
	}

	if len(response.Logs) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(response.Logs))
	}

	if response.Logs[0].Message != "Test log for API" {
		t.Errorf("Expected message 'Test log for API', got '%s'", response.Logs[0].Message)
	}
}

func TestHandleLogsSSE(t *testing.T) {
	conf := config.Config{
		RSSReaderEndpoint: "https://test.example.com",
		RSSReaderAPIKey:   "test-key",
	}
	server := NewServer(conf, false)

	req := httptest.NewRequest("GET", "/logs/stream", nil)
	w := httptest.NewRecorder()

	// Create a context that we can cancel to stop the SSE handler
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)

	// Start the SSE handler in a goroutine
	done := make(chan bool)
	go func() {
		defer close(done)
		server.handleLogsSSE(w, req)
	}()

	// Wait a bit for the handler to start and set headers
	time.Sleep(50 * time.Millisecond)

	// Cancel the context to stop the handler
	cancel()

	// Wait for the handler to finish
	<-done

	// Check headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", w.Header().Get("Content-Type"))
	}

	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Expected Cache-Control 'no-cache', got '%s'", w.Header().Get("Cache-Control"))
	}
}
