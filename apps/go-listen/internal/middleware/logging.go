package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/toozej/monogo/pkg/logging"
)

// LoggingMiddleware provides HTTP request logging with correlation IDs
type LoggingMiddleware struct {
	logger *slog.Logger
}

// NewLoggingMiddleware creates a new logging middleware instance
func NewLoggingMiddleware(logger *slog.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	if !rw.written {
		rw.statusCode = http.StatusOK
		rw.written = true
	}
	return rw.ResponseWriter.Write(data)
}

// LogRequests wraps an HTTP handler with request logging and correlation ID injection
func (lm *LoggingMiddleware) LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Generate correlation ID
		correlationID := generateCorrelationID()

		// Add correlation ID to request context
		ctx := logging.ContextWithCorrelationID(r.Context(), correlationID)
		r = r.WithContext(ctx)

		// Add correlation ID to response headers for client tracking
		w.Header().Set("X-Correlation-ID", correlationID)

		// Wrap response writer to capture status code
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Log request start
		lm.logger.DebugContext(ctx, "HTTP request started",
			"component", "http",
			"operation", "request_start",
			"method", r.Method,
			"path", r.URL.Path,
			"query_count", len(r.URL.Query()),
			"client_ip", getClientIP(r),
			"user_agent", r.UserAgent(),
		)

		// Call the next handler
		next.ServeHTTP(rw, r)

		// Calculate duration
		duration := time.Since(start)

		// Log request completion
		lm.logAPIRequest(
			ctx,
			r.Method,
			r.URL.Path,
			getClientIP(r),
			r.UserAgent(),
			rw.statusCode,
			duration.Milliseconds(),
		)
	})
}

// logAPIRequest logs API request completion, selecting the log level based on
// the response status code.
func (lm *LoggingMiddleware) logAPIRequest(ctx context.Context, method, path, clientIP, userAgent string, statusCode int, duration int64) {
	args := []any{
		"component", "http",
		"operation", "api_request",
		"method", method,
		"path", path,
		"client_ip", clientIP,
		"user_agent", userAgent,
		"status_code", statusCode,
		"duration_ms", duration,
	}

	switch {
	case statusCode >= 500:
		lm.logger.ErrorContext(ctx, "API request completed with server error", args...)
	case statusCode >= 400:
		lm.logger.WarnContext(ctx, "API request completed with client error", args...)
	default:
		lm.logger.InfoContext(ctx, "API request completed successfully", args...)
	}
}

// generateCorrelationID generates a random correlation ID
func generateCorrelationID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405")))
	}
	return hex.EncodeToString(bytes)
}
