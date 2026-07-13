package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// parseLine decodes a single JSON log line into a map for assertions.
func parseLine(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(b), &entry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v (output: %q)", err, string(b))
	}
	return entry
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{name: "JSON format", config: Config{Level: "info", Format: "json", Output: "stdout"}},
		{name: "Text format", config: Config{Level: "debug", Format: "text", Output: "stderr"}},
		{name: "Default (empty) config", config: Config{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if logger := NewLogger(tt.config); logger == nil {
				t.Fatal("NewLogger returned nil")
			}
		})
	}
}

func TestNewLogger_JSONSchema(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, Config{Level: "info", Format: "json"})

	logger.Info("hello", "component", "test", "count", 3)

	entry := parseLine(t, buf.Bytes())
	if entry["message"] != "hello" {
		t.Errorf("expected message 'hello', got %v", entry["message"])
	}
	if entry["level"] != "info" {
		t.Errorf("expected lower-case level 'info', got %v", entry["level"])
	}
	if _, ok := entry["timestamp"]; !ok {
		t.Error("expected a 'timestamp' key")
	}
	if entry["component"] != "test" {
		t.Errorf("expected component 'test', got %v", entry["component"])
	}
	if entry["count"] != float64(3) {
		t.Errorf("expected count 3, got %v", entry["count"])
	}
}

func TestNewLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, Config{Level: "warn", Format: "json"})

	logger.Info("suppressed")
	if buf.Len() != 0 {
		t.Errorf("expected info to be filtered at warn level, got %q", buf.String())
	}

	logger.Warn("emitted")
	entry := parseLine(t, buf.Bytes())
	if entry["level"] != "warn" {
		t.Errorf("expected level 'warn', got %v", entry["level"])
	}
	if entry["message"] != "emitted" {
		t.Errorf("expected message 'emitted', got %v", entry["message"])
	}
}

func TestNewLogger_InvalidLevelDefaultsToInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, Config{Level: "not-a-level", Format: "json"})

	logger.Debug("suppressed")
	if buf.Len() != 0 {
		t.Errorf("expected debug to be filtered at default info level, got %q", buf.String())
	}
	logger.Info("emitted")
	if buf.Len() == 0 {
		t.Error("expected info to be emitted at default level")
	}
}

func TestContextWithCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, Config{Level: "info", Format: "json"})

	ctx := ContextWithCorrelationID(context.Background(), "corr-123")
	logger.InfoContext(ctx, "with correlation")

	entry := parseLine(t, buf.Bytes())
	if entry["correlation_id"] != "corr-123" {
		t.Errorf("expected correlation_id 'corr-123', got %v", entry["correlation_id"])
	}
}

func TestContextWithoutCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, Config{Level: "info", Format: "json"})

	logger.InfoContext(context.Background(), "no correlation")

	entry := parseLine(t, buf.Bytes())
	if _, ok := entry["correlation_id"]; ok {
		t.Errorf("expected no correlation_id, got %v", entry["correlation_id"])
	}
}

func TestWithComponent(t *testing.T) {
	var buf bytes.Buffer
	logger := WithComponent(NewLoggerWithWriter(&buf, Config{Level: "info", Format: "json"}), "server")

	// The correlation-ID handler must survive a With* derivation.
	ctx := ContextWithCorrelationID(context.Background(), "corr-xyz")
	logger.InfoContext(ctx, "component message")

	entry := parseLine(t, buf.Bytes())
	if entry["component"] != "server" {
		t.Errorf("expected component 'server', got %v", entry["component"])
	}
	if entry["correlation_id"] != "corr-xyz" {
		t.Errorf("expected correlation_id to survive With(), got %v", entry["correlation_id"])
	}
}

func TestReplaceAttr_ErrorRenderedAsString(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, Config{Level: "info", Format: "json"})

	logger.Error("failed", "error", errors.New("boom"))

	entry := parseLine(t, buf.Bytes())
	if entry["error"] != "boom" {
		t.Errorf("expected error rendered as 'boom', got %v", entry["error"])
	}
	if entry["level"] != "error" {
		t.Errorf("expected level 'error', got %v", entry["level"])
	}
}

func TestReplaceAttr_ReservedFieldsCannotShadowSchema(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, Config{Level: "info", Format: "json"})

	logger.Info("real message",
		"timestamp", "caller timestamp",
		"level", "caller level",
		"message", "caller message",
	)

	entry := parseLine(t, buf.Bytes())
	if entry["message"] != "real message" {
		t.Errorf("expected record message to remain authoritative, got %v", entry["message"])
	}
	if entry["level"] != "info" {
		t.Errorf("expected record level to remain authoritative, got %v", entry["level"])
	}
	if entry["timestamp"] == "caller timestamp" {
		t.Error("expected record timestamp to remain authoritative")
	}

	expectedCallerFields := map[string]any{
		"fields.timestamp": "caller timestamp",
		"fields.level":     "caller level",
		"fields.message":   "caller message",
	}
	for key, want := range expectedCallerFields {
		if got := entry[key]; got != want {
			t.Errorf("expected %s=%q, got %v", key, want, got)
		}
	}
}

func TestNewLogger_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, Config{Level: "info", Format: "text"})

	logger.Info("text message", "key", "value")

	out := buf.String()
	if !strings.Contains(out, "text message") {
		t.Errorf("expected text output to contain the message, got %q", out)
	}
	if !strings.Contains(out, "key=value") {
		t.Errorf("expected text output to contain 'key=value', got %q", out)
	}
}
