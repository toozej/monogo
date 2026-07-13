// Package logging provides a small, dependency-free wrapper around the standard
// library's log/slog for configuring structured application logging.
//
// Apps map their own configuration onto Config and call NewLogger to obtain a
// *slog.Logger, which they typically install process-wide via slog.SetDefault
// and/or thread through their components. The logger emits a stable schema whose
// top-level keys are "timestamp", "level" (lower-case), and "message", followed
// by any structured attributes.
//
// Correlation IDs stored in a context under CorrelationIDKey are automatically
// attached to any record logged through the context-aware methods (for example
// logger.InfoContext(ctx, ...)). Use ContextWithCorrelationID to populate the
// context.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// ContextKey is a type for context keys to avoid collisions.
type ContextKey string

const (
	// CorrelationIDKey is the context key for correlation IDs.
	CorrelationIDKey ContextKey = "correlation_id"
)

// timestampFormat matches the RFC3339 millisecond layout previously emitted by
// the logging stack, keeping the on-the-wire log schema stable for consumers.
const timestampFormat = "2006-01-02T15:04:05.000Z07:00"

// Config holds the logging settings consumed by NewLogger. It is intentionally
// small and self-contained so this package does not depend on any app-specific
// configuration type. Apps map their own config onto this struct.
type Config struct {
	// Level is the minimum log level ("debug", "info", "warn", "error").
	Level string
	// Format selects the output encoding: "json" or "text". Defaults to JSON.
	Format string
	// Output selects the destination: "stdout" or "stderr". Defaults to stdout.
	Output string
}

// NewLogger creates a new *slog.Logger configured from cfg, writing to stdout or
// stderr per cfg.Output.
func NewLogger(cfg Config) *slog.Logger {
	var out io.Writer = os.Stdout
	if strings.EqualFold(cfg.Output, "stderr") {
		out = os.Stderr
	}
	return NewLoggerWithWriter(out, cfg)
}

// NewLoggerWithWriter is like NewLogger but writes to w, ignoring cfg.Output. It
// is primarily useful in tests that need to capture log output.
func NewLoggerWithWriter(w io.Writer, cfg Config) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:       parseLevel(cfg.Level),
		ReplaceAttr: replaceAttr,
	}

	var handler slog.Handler
	if strings.EqualFold(cfg.Format, "text") {
		handler = slog.NewTextHandler(w, opts)
	} else {
		// Default to JSON for structured logging.
		handler = slog.NewJSONHandler(w, opts)
	}

	return slog.New(&contextHandler{Handler: handler})
}

// ContextWithCorrelationID returns a copy of ctx carrying the given correlation
// ID. NewLogger's handler attaches that ID to any record logged with the
// returned context via the context-aware logging methods.
func ContextWithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// WithComponent returns a child logger tagged with a "component" attribute, a
// common way to categorize log lines by subsystem.
func WithComponent(l *slog.Logger, component string) *slog.Logger {
	return l.With("component", component)
}

// parseLevel maps a textual level onto a slog.Level, defaulting to info.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// replaceAttr normalizes the record's built-in attributes to a stable schema
// (timestamp/level/message with a lower-case level) and renders error values as
// their message string, which slog's JSON handler would otherwise marshal to an
// empty object.
func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 {
		switch a.Key {
		case slog.TimeKey:
			a.Key = "timestamp"
			if t, ok := a.Value.Any().(time.Time); ok {
				a.Value = slog.StringValue(t.Format(timestampFormat))
			}
			return a
		case slog.MessageKey:
			a.Key = "message"
			return a
		case slog.LevelKey:
			if lvl, ok := a.Value.Any().(slog.Level); ok {
				a.Value = slog.StringValue(strings.ToLower(lvl.String()))
				return a
			}
		}

		// Keep application attributes from shadowing the stable top-level
		// schema. logrus handled these collisions by prefixing the application
		// field with "fields."; preserve that behavior so JSON consumers cannot
		// mistake a caller-supplied value for the record's metadata.
		switch a.Key {
		case "timestamp", "level", "message":
			a.Key = "fields." + a.Key
		}
	}

	if err, ok := a.Value.Any().(error); ok {
		a.Value = slog.StringValue(err.Error())
	}
	return a
}

// contextHandler wraps a slog.Handler to attach the correlation ID carried by
// the log record's context (see CorrelationIDKey).
type contextHandler struct {
	slog.Handler
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok && id != "" {
		r.AddAttrs(slog.String(string(CorrelationIDKey), id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name)}
}
