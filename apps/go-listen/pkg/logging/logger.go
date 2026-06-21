package logging

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/toozej/go-listen/pkg/config"
)

// ContextKey is a type for context keys to avoid collisions
type ContextKey string

const (
	// CorrelationIDKey is the context key for correlation IDs
	CorrelationIDKey ContextKey = "correlation_id"
)

// Logger wraps logrus.Logger with additional functionality
type Logger struct {
	*logrus.Logger
}

// NewLogger creates a new configured logger instance
func NewLogger(cfg config.LoggingConfig) *Logger {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel // Default to info level
	}
	logger.SetLevel(level)

	// Set log format
	switch strings.ToLower(cfg.Format) {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	default:
		// Default to JSON for structured logging
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	}

	// Set output destination
	switch strings.ToLower(cfg.Output) {
	case "stdout":
		logger.SetOutput(os.Stdout)
	case "stderr":
		logger.SetOutput(os.Stderr)
	default:
		logger.SetOutput(os.Stdout) // Default to stdout
	}

	return &Logger{Logger: logger}
}

// WithCorrelationID adds a correlation ID to the logger context
func (l *Logger) WithCorrelationID(correlationID string) *logrus.Entry {
	return l.WithField("correlation_id", correlationID)
}

// WithContext extracts correlation ID from context and adds it to the logger
func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
	if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok && correlationID != "" {
		return l.WithCorrelationID(correlationID)
	}
	return l.WithFields(logrus.Fields{})
}

// WithComponent adds a component field to the logger for better categorization
func (l *Logger) WithComponent(component string) *logrus.Entry {
	return l.WithField("component", component)
}

// WithOperation adds an operation field to the logger for tracking specific operations
func (l *Logger) WithOperation(operation string) *logrus.Entry {
	return l.WithField("operation", operation)
}

// LogArtistSearch logs artist search operations
func (l *Logger) LogArtistSearch(ctx context.Context, searchTerm, matchedArtist string, matchScore float64) {
	l.WithContext(ctx).WithFields(logrus.Fields{
		"component":      "artist_search",
		"operation":      "search",
		"search_term":    searchTerm,
		"matched_artist": matchedArtist,
		"match_score":    matchScore,
	}).Info("Artist search performed")
}

// LogTrackAddition logs track addition operations
func (l *Logger) LogTrackAddition(ctx context.Context, artistName, playlistName string, trackCount int, trackNames []string) {
	l.WithContext(ctx).WithFields(logrus.Fields{
		"component":     "playlist",
		"operation":     "add_tracks",
		"artist_name":   artistName,
		"playlist_name": playlistName,
		"track_count":   trackCount,
		"track_names":   trackNames,
	}).Info("Tracks added to playlist")
}

// LogDuplicateDetection logs duplicate detection events
func (l *Logger) LogDuplicateDetection(ctx context.Context, artistName, playlistName string, hasDuplicates, overrideUsed bool) {
	entry := l.WithContext(ctx).WithFields(logrus.Fields{
		"component":      "duplicate_detection",
		"operation":      "check_duplicates",
		"artist_name":    artistName,
		"playlist_name":  playlistName,
		"has_duplicates": hasDuplicates,
		"override_used":  overrideUsed,
	})

	if hasDuplicates {
		if overrideUsed {
			entry.Warn("Duplicate tracks detected but override used")
		} else {
			entry.Info("Duplicate tracks detected, addition blocked")
		}
	} else {
		entry.Debug("No duplicate tracks found")
	}
}

// LogSecurityEvent logs security-related events
func (l *Logger) LogSecurityEvent(ctx context.Context, eventType, clientIP, userAgent, details string) {
	l.WithContext(ctx).WithFields(logrus.Fields{
		"component":  "security",
		"operation":  "security_event",
		"event_type": eventType,
		"client_ip":  clientIP,
		"user_agent": userAgent,
		"details":    details,
	}).Warn("Security event detected")
}

// LogAPIRequest logs API request details
func (l *Logger) LogAPIRequest(ctx context.Context, method, path, clientIP, userAgent string, statusCode int, duration int64) {
	entry := l.WithContext(ctx).WithFields(logrus.Fields{
		"component":   "http",
		"operation":   "api_request",
		"method":      method,
		"path":        path,
		"client_ip":   clientIP,
		"user_agent":  userAgent,
		"status_code": statusCode,
		"duration_ms": duration,
	})

	switch {
	case statusCode >= 500:
		entry.Error("API request completed with server error")
	case statusCode >= 400:
		entry.Warn("API request completed with client error")
	default:
		entry.Info("API request completed successfully")
	}
}

// SetOutput allows changing the output destination (useful for testing)
func (l *Logger) SetOutput(output io.Writer) {
	l.Logger.SetOutput(output)
}
