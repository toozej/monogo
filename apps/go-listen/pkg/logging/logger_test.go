package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/pkg/config"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name     string
		config   config.LoggingConfig
		wantJSON bool
	}{
		{
			name: "JSON format configuration",
			config: config.LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
			wantJSON: true,
		},
		{
			name: "Text format configuration",
			config: config.LoggingConfig{
				Level:  "debug",
				Format: "text",
				Output: "stderr",
			},
			wantJSON: false,
		},
		{
			name: "Default configuration",
			config: config.LoggingConfig{
				Level:  "",
				Format: "",
				Output: "",
			},
			wantJSON: true, // Default is JSON
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.config)

			if logger == nil {
				t.Fatal("NewLogger returned nil")
			}

			// Test that the logger is properly configured
			if logger.Logger == nil {
				t.Fatal("Logger.Logger is nil")
			}

			// Check formatter type
			isJSON := false
			isText := false
			switch logger.Logger.Formatter.(type) {
			case *logrus.JSONFormatter:
				isJSON = true
			case *logrus.TextFormatter:
				isText = true
			}

			if tt.wantJSON && !isJSON {
				t.Error("Expected JSON formatter but got different type")
			}
			if !tt.wantJSON && !isText {
				t.Error("Expected Text formatter but got different type")
			}
		})
	}
}

func TestLogger_WithCorrelationID(t *testing.T) {
	logger := NewLogger(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	correlationID := "test-correlation-id"
	entry := logger.WithCorrelationID(correlationID)

	if entry.Data["correlation_id"] != correlationID {
		t.Errorf("Expected correlation_id %s, got %v", correlationID, entry.Data["correlation_id"])
	}
}

func TestLogger_WithContext(t *testing.T) {
	logger := NewLogger(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	tests := []struct {
		name              string
		ctx               context.Context
		wantCorrelationID string
	}{
		{
			name:              "Context with correlation ID",
			ctx:               context.WithValue(context.Background(), CorrelationIDKey, "test-id"),
			wantCorrelationID: "test-id",
		},
		{
			name:              "Context without correlation ID",
			ctx:               context.Background(),
			wantCorrelationID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := logger.WithContext(tt.ctx)

			correlationID, exists := entry.Data["correlation_id"]
			if tt.wantCorrelationID != "" {
				if !exists {
					t.Error("Expected correlation_id in entry data but not found")
				}
				if correlationID != tt.wantCorrelationID {
					t.Errorf("Expected correlation_id %s, got %v", tt.wantCorrelationID, correlationID)
				}
			} else if exists && correlationID != "" {
				t.Errorf("Expected no correlation_id but got %v", correlationID)
			}
		})
	}
}

func TestLogger_WithComponent(t *testing.T) {
	logger := NewLogger(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	component := "test-component"
	entry := logger.WithComponent(component)

	if entry.Data["component"] != component {
		t.Errorf("Expected component %s, got %v", component, entry.Data["component"])
	}
}

func TestLogger_WithOperation(t *testing.T) {
	logger := NewLogger(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	operation := "test-operation"
	entry := logger.WithOperation(operation)

	if entry.Data["operation"] != operation {
		t.Errorf("Expected operation %s, got %v", operation, entry.Data["operation"])
	}
}

func TestLogger_LogArtistSearch(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})
	logger.SetOutput(&buf)

	ctx := context.WithValue(context.Background(), CorrelationIDKey, "test-correlation")
	searchTerm := "test artist"
	matchedArtist := "Test Artist"
	matchScore := 0.95

	logger.LogArtistSearch(ctx, searchTerm, matchedArtist, matchScore)

	// Parse the logged JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}

	// Verify log fields
	expectedFields := map[string]interface{}{
		"correlation_id": "test-correlation",
		"component":      "artist_search",
		"operation":      "search",
		"search_term":    searchTerm,
		"matched_artist": matchedArtist,
		"match_score":    matchScore,
		"level":          "info",
		"message":        "Artist search performed",
	}

	for key, expectedValue := range expectedFields {
		if logEntry[key] != expectedValue {
			t.Errorf("Expected %s to be %v, got %v", key, expectedValue, logEntry[key])
		}
	}
}

func TestLogger_LogTrackAddition(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})
	logger.SetOutput(&buf)

	ctx := context.WithValue(context.Background(), CorrelationIDKey, "test-correlation")
	artistName := "Test Artist"
	playlistName := "Test Playlist"
	trackCount := 3
	trackNames := []string{"Track 1", "Track 2", "Track 3"}

	logger.LogTrackAddition(ctx, artistName, playlistName, trackCount, trackNames)

	// Parse the logged JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}

	// Verify log fields
	if logEntry["correlation_id"] != "test-correlation" {
		t.Errorf("Expected correlation_id to be test-correlation, got %v", logEntry["correlation_id"])
	}
	if logEntry["component"] != "playlist" {
		t.Errorf("Expected component to be playlist, got %v", logEntry["component"])
	}
	if logEntry["operation"] != "add_tracks" {
		t.Errorf("Expected operation to be add_tracks, got %v", logEntry["operation"])
	}
	if logEntry["artist_name"] != artistName {
		t.Errorf("Expected artist_name to be %s, got %v", artistName, logEntry["artist_name"])
	}
	if logEntry["playlist_name"] != playlistName {
		t.Errorf("Expected playlist_name to be %s, got %v", playlistName, logEntry["playlist_name"])
	}
	if logEntry["track_count"] != float64(trackCount) { // JSON numbers are float64
		t.Errorf("Expected track_count to be %d, got %v", trackCount, logEntry["track_count"])
	}
}

func TestLogger_LogDuplicateDetection(t *testing.T) {
	tests := []struct {
		name          string
		hasDuplicates bool
		overrideUsed  bool
		expectedLevel string
	}{
		{
			name:          "No duplicates",
			hasDuplicates: false,
			overrideUsed:  false,
			expectedLevel: "debug",
		},
		{
			name:          "Duplicates with override",
			hasDuplicates: true,
			overrideUsed:  true,
			expectedLevel: "warning",
		},
		{
			name:          "Duplicates without override",
			hasDuplicates: true,
			overrideUsed:  false,
			expectedLevel: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(config.LoggingConfig{
				Level:  "debug", // Set to debug to capture all levels
				Format: "json",
				Output: "stdout",
			})
			logger.SetOutput(&buf)

			ctx := context.WithValue(context.Background(), CorrelationIDKey, "test-correlation")
			artistName := "Test Artist"
			playlistName := "Test Playlist"

			logger.LogDuplicateDetection(ctx, artistName, playlistName, tt.hasDuplicates, tt.overrideUsed)

			// Parse the logged JSON
			var logEntry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Fatalf("Failed to parse log output as JSON: %v", err)
			}

			// Verify log fields
			if logEntry["level"] != tt.expectedLevel {
				t.Errorf("Expected level to be %s, got %v", tt.expectedLevel, logEntry["level"])
			}
			if logEntry["component"] != "duplicate_detection" {
				t.Errorf("Expected component to be duplicate_detection, got %v", logEntry["component"])
			}
			if logEntry["has_duplicates"] != tt.hasDuplicates {
				t.Errorf("Expected has_duplicates to be %v, got %v", tt.hasDuplicates, logEntry["has_duplicates"])
			}
			if logEntry["override_used"] != tt.overrideUsed {
				t.Errorf("Expected override_used to be %v, got %v", tt.overrideUsed, logEntry["override_used"])
			}
		})
	}
}

func TestLogger_LogSecurityEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})
	logger.SetOutput(&buf)

	ctx := context.WithValue(context.Background(), CorrelationIDKey, "test-correlation")
	eventType := "rate_limit_exceeded"
	clientIP := "192.168.1.1"
	userAgent := "test-agent"
	details := "Too many requests"

	logger.LogSecurityEvent(ctx, eventType, clientIP, userAgent, details)

	// Parse the logged JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output as JSON: %v", err)
	}

	// Verify log fields
	expectedFields := map[string]interface{}{
		"correlation_id": "test-correlation",
		"component":      "security",
		"operation":      "security_event",
		"event_type":     eventType,
		"client_ip":      clientIP,
		"user_agent":     userAgent,
		"details":        details,
		"level":          "warning",
	}

	for key, expectedValue := range expectedFields {
		if logEntry[key] != expectedValue {
			t.Errorf("Expected %s to be %v, got %v", key, expectedValue, logEntry[key])
		}
	}
}

func TestLogger_LogAPIRequest(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		expectedLevel string
	}{
		{
			name:          "Successful request",
			statusCode:    200,
			expectedLevel: "info",
		},
		{
			name:          "Client error",
			statusCode:    400,
			expectedLevel: "warning",
		},
		{
			name:          "Server error",
			statusCode:    500,
			expectedLevel: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(config.LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			})
			logger.SetOutput(&buf)

			ctx := context.WithValue(context.Background(), CorrelationIDKey, "test-correlation")
			method := "POST"
			path := "/api/test"
			clientIP := "192.168.1.1"
			userAgent := "test-agent"
			duration := int64(150)

			logger.LogAPIRequest(ctx, method, path, clientIP, userAgent, tt.statusCode, duration)

			// Parse the logged JSON
			var logEntry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Fatalf("Failed to parse log output as JSON: %v", err)
			}

			// Verify log fields
			if logEntry["level"] != tt.expectedLevel {
				t.Errorf("Expected level to be %s, got %v", tt.expectedLevel, logEntry["level"])
			}
			if logEntry["component"] != "http" {
				t.Errorf("Expected component to be http, got %v", logEntry["component"])
			}
			if logEntry["status_code"] != float64(tt.statusCode) {
				t.Errorf("Expected status_code to be %d, got %v", tt.statusCode, logEntry["status_code"])
			}
			if logEntry["duration_ms"] != float64(duration) {
				t.Errorf("Expected duration_ms to be %d, got %v", duration, logEntry["duration_ms"])
			}
		})
	}
}

func TestLogger_SetOutput(t *testing.T) {
	logger := NewLogger(config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	logger.Info("test message")

	if buf.Len() == 0 {
		t.Error("Expected log output but buffer is empty")
	}

	if !strings.Contains(buf.String(), "test message") {
		t.Error("Expected log output to contain 'test message'")
	}
}
