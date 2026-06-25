// Package logger provides centralized structured logging for Podgrab.
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Log is the global logger instance
	Log *zap.SugaredLogger
)

func init() {
	Initialize()
}

// Initialize creates and configures the global logger
func Initialize() {
	config := zap.NewProductionConfig()

	// Set log level from environment or default to info
	logLevel := os.Getenv("LOG_LEVEL")
	switch logLevel {
	case "debug", "DEBUG":
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info", "INFO":
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn", "WARN", "warning", "WARNING":
		config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error", "ERROR":
		config.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	// Use a more human-readable output format in development
	if os.Getenv("GIN_MODE") != "release" {
		config.Encoding = "console"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	logger, err := config.Build()
	if err != nil {
		// Fallback to no-op logger if initialization fails
		logger = zap.NewNop()
	}

	Log = logger.Sugar()
}

// Sync flushes any buffered log entries
func Sync() {
	if Log != nil {
		// Sync errors are ignored because they're expected when stdout/stderr are not syncable
		_ = Log.Sync() //nolint:errcheck // sync errors expected in some environments
	}
}
