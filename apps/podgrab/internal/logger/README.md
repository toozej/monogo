# Structured Logging with Zap

This package provides centralized structured logging for Podgrab using
[Uber's Zap](https://github.com/uber-go/zap) logger.

## Usage

Import the logger package:

```go
import "github.com/akhilrex/podgrab/internal/logger"
```

### Logging Levels

Use the appropriate method based on the severity:

```go
// Debug - detailed information for diagnosing problems
logger.Log.Debug("Processing started")
logger.Log.Debugw("Processing item", "id", itemID, "name", name)

// Info - general informational messages
logger.Log.Info("Server started successfully")
logger.Log.Infow("Configuration loaded", "port", 8080, "env", "production")

// Warn - warning messages for potentially harmful situations
logger.Log.Warn("Connection pool nearly exhausted")
logger.Log.Warnw("Invalid configuration", "setting", "CHECK_FREQUENCY", "value", "abc")

// Error - error messages for serious problems
logger.Log.Error("Failed to connect to database")
logger.Log.Errorw("Download failed", "error", err, "url", downloadURL)

// Fatal - critical errors that require program termination
logger.Log.Fatal("Cannot initialize database")
logger.Log.Fatalw("Critical configuration missing", "error", err)
```

### Structured Logging (Recommended)

Use the `*w` variants (Debugw, Infow, Warnw, Errorw, Fatalw) for structured
logging with key-value pairs:

```go
logger.Log.Infow("Episode downloaded",
    "podcast_id", podcastID,
    "episode_id", episodeID,
    "file_size", fileSize,
    "duration", duration)
```

This creates structured log entries that are easy to parse and query.

### Simple Logging

For simple messages without structured data:

```go
logger.Log.Info("Application starting")
logger.Log.Error("Connection timeout")
```

## Configuration

### Log Level

Set the `LOG_LEVEL` environment variable to control logging verbosity:

```bash
# Show only errors
export LOG_LEVEL=error

# Show warnings and errors (default)
export LOG_LEVEL=warn

# Show info, warnings, and errors
export LOG_LEVEL=info

# Show everything including debug messages
export LOG_LEVEL=debug
```

Default: `info`

### Output Format

The logger automatically adjusts output format based on the `GIN_MODE`
environment variable:

- **Development** (GIN_MODE != "release"):

  - Human-readable console output
  - Colored log levels
  - ISO8601 timestamps

- **Production** (GIN_MODE = "release"):

  - JSON formatted output
  - Optimized for log aggregation systems
  - Machine-parseable

## Migration from fmt/log

### Before (Console Output)

```go
fmt.Printf("Error downloading file: %v\n", err)
fmt.Println("Processing complete")
log.Printf("User logged in: %s", username)
```

### After (Structured Logging)

```go
logger.Log.Errorw("Error downloading file", "error", err)
logger.Log.Info("Processing complete")
logger.Log.Infow("User logged in", "username", username)
```

## Best Practices

1. **Use structured logging** - Always prefer `*w` variants with key-value pairs
1. **Choose appropriate levels** - Debug for development, Info for important
   events, Error for failures
1. **Include context** - Add relevant context as key-value pairs (IDs,
   filenames, URLs, etc.)
1. **Avoid sensitive data** - Never log passwords, API keys, or personal
   information
1. **Be consistent** - Use consistent key names across your application (e.g.,
   always use "user_id" not "userID" or "userId")

## Example

```go
func DownloadEpisode(episodeID string) error {
    logger.Log.Infow("Starting episode download",
        "episode_id", episodeID)

    // ... download logic ...

    if err != nil {
        logger.Log.Errorw("Download failed",
            "error", err,
            "episode_id", episodeID,
            "retry_count", retryCount)
        return err
    }

    logger.Log.Infow("Download completed",
        "episode_id", episodeID,
        "file_size", fileSize,
        "duration_ms", duration.Milliseconds())

    return nil
}
```
