package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// LogEntry represents a single log entry for the web UI
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// LogBuffer manages a circular buffer of log entries for the web UI
type LogBuffer struct {
	entries []LogEntry
	size    int
	index   int
	mutex   sync.RWMutex
}

// NewLogBuffer creates a new log buffer with the specified size
func NewLogBuffer(size int) *LogBuffer {
	return &LogBuffer{
		entries: make([]LogEntry, size),
		size:    size,
		index:   0,
	}
}

// Add adds a new log entry to the buffer
func (lb *LogBuffer) Add(entry LogEntry) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	lb.entries[lb.index] = entry
	lb.index = (lb.index + 1) % lb.size
}

// GetRecent returns the most recent log entries (up to limit)
func (lb *LogBuffer) GetRecent(limit int) []LogEntry {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	if limit <= 0 || limit > lb.size {
		limit = lb.size
	}

	// codeql[go/uncontrolled-allocation-size] limit is capped at 200 by the caller before reaching this function
	result := make([]LogEntry, 0, limit)

	// Start from the most recent entry and work backwards
	for i := 0; i < limit; i++ {
		idx := (lb.index - 1 - i + lb.size) % lb.size
		entry := lb.entries[idx]

		// Skip empty entries (buffer not full yet)
		if entry.Timestamp.IsZero() {
			break
		}

		result = append(result, entry)
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// WebUIHook is a logrus hook that captures logs for the web UI
type WebUIHook struct {
	buffer *LogBuffer
}

// NewWebUIHook creates a new web UI log hook
func NewWebUIHook(bufferSize int) *WebUIHook {
	return &WebUIHook{
		buffer: NewLogBuffer(bufferSize),
	}
}

// Levels returns the log levels this hook should fire for
func (hook *WebUIHook) Levels() []log.Level {
	return []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
		log.DebugLevel,
	}
}

// Fire is called when a log entry is made
func (hook *WebUIHook) Fire(entry *log.Entry) error {
	// Convert logrus fields to map[string]interface{}
	fields := make(map[string]interface{})
	for k, v := range entry.Data {
		fields[k] = v
	}

	logEntry := LogEntry{
		Timestamp: entry.Time,
		Level:     entry.Level.String(),
		Message:   entry.Message,
		Fields:    fields,
	}

	hook.buffer.Add(logEntry)
	return nil
}

// GetBuffer returns the log buffer for external access
func (hook *WebUIHook) GetBuffer() *LogBuffer {
	return hook.buffer
}

// LogsResponse represents the JSON response for log entries
type LogsResponse struct {
	Success bool       `json:"success"`
	Logs    []LogEntry `json:"logs,omitempty"`
	Error   string     `json:"error,omitempty"`
}

// handleLogs serves recent log entries as JSON
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set response content type
	w.Header().Set("Content-Type", "application/json")

	// Check if we have a log hook installed
	if s.logHook == nil {
		response := LogsResponse{
			Success: false,
			Error:   "Log capture not available",
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Errorf("Error encoding logs error response: %v", err)
		}
		return
	}

	// Get recent logs (default to last 50 entries)
	limit := 50
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		if parsedLimit, err := fmt.Sscanf(limitParam, "%d", &limit); err != nil || parsedLimit != 1 {
			limit = 50 // fallback to default
		}
		if limit > 200 {
			limit = 200 // cap at 200 entries
		}
	}

	logs := s.logHook.GetBuffer().GetRecent(limit)

	response := LogsResponse{
		Success: true,
		Logs:    logs,
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("Error encoding logs response: %v", err)
	}
}

// handleLogsSSE serves log entries via Server-Sent Events for real-time updates
func (s *Server) handleLogsSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check if we have a log hook installed
	if s.logHook == nil {
		_, _ = fmt.Fprintf(w, "event: error\ndata: Log capture not available\n\n")
		return
	}

	// Send initial batch of recent logs
	recentLogs := s.logHook.GetBuffer().GetRecent(20)
	for _, logEntry := range recentLogs {
		data, err := json.Marshal(logEntry)
		if err != nil {
			continue
		}
		// nosemgrep: go.lang.security.audit.xss.no-fprintf-to-responsewriter.no-fprintf-to-responsewriter
		_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", data) // #nosec G705 -- data is JSON-marshaled log entry, safe for SSE
	}

	// Flush initial data
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Keep connection alive and send periodic heartbeats
	// In a production implementation, you'd want to implement a proper
	// pub/sub system to push new logs as they arrive
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send heartbeat
			// nosemgrep: go.lang.security.audit.xss.no-fprintf-to-responsewriter.no-fprintf-to-responsewriter
			_, _ = fmt.Fprintf(w, "event: heartbeat\ndata: {\"timestamp\":\"%s\"}\n\n", time.Now().Format(time.RFC3339))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		case <-r.Context().Done():
			// Client disconnected
			return
		}
	}
}
