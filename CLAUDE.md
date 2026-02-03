# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Project Overview

Podgrab is a self-hosted podcast manager written in Go that automatically
downloads podcast episodes. It's built as a lightweight web application with a
Go backend and HTML template frontend.

**Stack**: Go 1.15+, Gin web framework, GORM with SQLite, HTML templates,
WebSockets for real-time updates, Uber Zap structured logging

## Development Commands

### Building and Running

```bash
# Build the application
go build -o ./app ./main.go

# Run locally (requires .env file or environment variables)
./app

# Run with Go
go run main.go

# Build Docker image
docker build -t podgrab .

# Run with Docker
docker run -d -p 8080:8080 --name=podgrab \
  -v "$(pwd)/config:/config" \
  -v "$(pwd)/assets:/assets" \
  podgrab

# Run with docker-compose
docker-compose up -d
```

### Development Workflow

```bash
# Install dependencies
go mod download

# Format code
go fmt ./...

# Tidy dependencies
go mod tidy

# Run tests
go test ./... -v

# Run integration tests
go test ./integration_test/... -v -tags=integration

# Run E2E tests (requires Chrome/Chromium)
go test ./e2e_test/... -v -tags=e2e
```

**Note**: This project has comprehensive test coverage (100+ tests). Run the
appropriate test suite for your changes.

## Architecture Overview

### Application Structure

The codebase follows a layered architecture with clear separation of concerns:

**Entry Point** (`main.go`):

- Initializes database and runs migrations
- Sets up Gin router with middleware
- Configures template functions for HTML rendering
- Registers all HTTP routes (REST API + page routes)
- Starts background jobs via gocron (episode refresh, downloads, backups)
- Launches WebSocket server for real-time updates

**Core Layers**:

1. **Controllers** (`controllers/`): HTTP request handlers

   - `podcast.go`: REST API endpoints for podcasts, episodes, tags
   - `pages.go`: HTML page rendering endpoints
   - `websockets.go`: WebSocket handler for real-time client updates

1. **Service** (`service/`): Business logic

   - `podcastService.go`: Podcast/episode CRUD, RSS parsing, download
     orchestration
   - `fileService.go`: File downloads, episode naming, image handling
   - `itunesService.go`: iTunes API integration for podcast search
   - `gpodderService.go`: GPodder API integration

1. **DB** (`db/`): Data layer with GORM

   - `podcast.go`: Core data models (Podcast, PodcastItem, Setting, Tag)
   - `dbfunctions.go`: Database queries and operations
   - `db.go`: Database initialization
   - `migrations.go`: Schema migrations

1. **Model** (`model/`): External data structures

   - RSS feed parsing models
   - iTunes API response models
   - OPML import/export models
   - Request/response DTOs

1. **Client** (`client/`): HTML templates

   - Go templates with custom functions defined in main.go
   - No separate JavaScript build process - templates served directly

1. **Internal** (`internal/`): Internal packages

   - `logger/`: Centralized structured logging with Uber Zap
   - `database/`: Repository interfaces and implementations
   - `testing/`: Test helpers and mocks

### Key Architectural Patterns

**Background Jobs**: The application uses `gocron` to schedule recurring tasks:

- `RefreshEpisodes()`: Checks RSS feeds for new episodes (every CHECK_FREQUENCY
  mins)
- `DownloadMissingEpisodes()`: Downloads queued episodes
- `CheckMissingFiles()`: Detects deleted files to update database
- `CreateBackup()`: Database backups (every 2 days)

**Download Flow**:

1. User adds podcast URL → Controller parses RSS → Service creates Podcast +
   PodcastItem records
1. Background job detects new episodes → Queues downloads based on settings
1. `fileService.Download()` handles concurrent downloads (max controlled by
   `MaxDownloadConcurrency` setting)
1. WebSocket broadcasts download progress to connected clients

**Real-time Updates**: WebSocket connection (`/ws`) broadcasts events:

- Download progress
- Episode status changes
- Playlist updates
- Clients subscribe and update UI reactively

**File Management**:

- Episodes stored in `/assets/{podcast-name}/` with sanitized filenames
- Optional date/episode number prefixes based on settings
- Image caching to reduce external requests
- File existence checking to prevent re-downloads

### Database Schema

**Core Models** (all use UUID primary keys via `db.Base`):

- `Podcast`: RSS feed metadata, relationship to episodes and tags
- `PodcastItem`: Individual episodes with download state tracking
- `Tag`: Labels for organizing podcasts (many-to-many)
- `Setting`: Global app configuration (singleton)
- `JobLock`: Prevents duplicate background job execution

**Key Relationships**:

- Podcast → PodcastItems (one-to-many)
- Podcast ↔ Tags (many-to-many via `podcast_tags`)

**Download States** (enum in `podcast.go`):

- `NotDownloaded`: Queued for download
- `Downloading`: Currently downloading
- `Downloaded`: Successfully downloaded
- `Deleted`: File removed from disk

## Configuration

### Environment Variables

Required for deployment (set in `.env` or docker-compose):

- `CONFIG`: Path to config directory (database, backups) - default: `.`
- `DATA`: Path to episode storage - default: `./assets`
- `CHECK_FREQUENCY`: Minutes between RSS checks - default: `30`
- `PASSWORD`: Enable basic auth if set (username: `podgrab`)
- `PORT`: Override default port 8080
- `GIN_MODE`: Set to `release` for production
- `LOG_LEVEL`: Logging verbosity (`debug`, `info`, `warn`, `error`) - default:
  `info`

### Settings Database

Runtime settings stored in database (editable via `/settings` page):

- `DownloadOnAdd`: Auto-download when adding podcast
- `InitialDownloadCount`: How many episodes to download initially
- `AutoDownload`: Enable automatic downloads for new episodes
- `AppendDateToFileName`: Add date prefix to filenames
- `MaxDownloadConcurrency`: Concurrent download limit (default: 5)

## Important Implementation Details

### Structured Logging

The application uses Uber Zap for structured logging via
`internal/logger/logger.go`:

- Global logger instance: `logger.Log` (SugaredLogger)
- Log levels controlled by `LOG_LEVEL` environment variable
- Console format in development, JSON in production (based on `GIN_MODE`)
- Always use structured logging with context:
  `logger.Log.Errorw("message", "key", value)`

For detailed logging documentation, see `internal/logger/README.md`.

### Template Functions

Custom template helpers defined in `main.go`:

- `formatDate`, `naturalDate`: Time formatting
- `formatFileSize`, `formatDuration`: Human-readable sizes/durations
- `downloadedEpisodes`, `downloadingEpisodes`: Episode stats
- `intRange`: Pagination helpers

When modifying templates, reference these functions in `main.go:35-128`.

### RSS Feed Parsing

The application uses `encoding/xml` with custom models in `model/rssModels.go`.
When handling new podcast sources:

- RSS parsing happens in `service.FetchURL()` and `service/podcastService.go`
- XML namespace handling via `xmlquery` for complex feeds
- Some feeds require special user-agent headers (configurable in settings)

### Concurrent Downloads

Download concurrency managed by semaphore pattern in `fileService.go`:

- Controlled via `Setting.MaxDownloadConcurrency`
- Default is 5 concurrent downloads
- Each download spawns a goroutine that acquires a slot
- WebSocket broadcasts progress for each download

### WebSocket Protocol

WebSocket messages sent from `controllers/websockets.go`:

- Messages are JSON with `type` and `data` fields
- Broadcast to all connected clients
- No authentication on WebSocket (relies on HTTP basic auth)

## Common Workflows

### Adding Support for New Podcast Sources

1. Extend RSS parsing models in `model/rssModels.go` if needed
1. Update `service.FetchURL()` for custom parsing logic
1. Test with `service.AddPodcast()` which handles the full flow
1. Consider user-agent requirements in `fileService.httpClient()`

### Modifying Download Behavior

1. Check current logic in `service/fileService.go` (Download function)
1. Settings are accessed via `db.GetOrCreateSetting()`
1. Filename generation in `getFileName()` considers multiple settings
1. Update WebSocket broadcasts if adding progress tracking

### Database Migrations

1. Add new migration in `db/migrations.go`
1. Migration runs automatically on startup via `db.Migrate()`
1. Use GORM auto-migration for simple schema changes
1. Complex migrations require explicit SQL in migration records

## File Organization Principles

- **No shared state**: Each package should be independently testable
- **DB layer never calls service layer**: Prevents circular dependencies
- **Controllers are thin**: Business logic belongs in service layer
- **Models are data-only**: No business logic in model structs

## Known Gotchas

- Template parsing happens at startup - restart required for template changes
- SQLite database can lock under high concurrency - consider connection pool
  tuning
- Background jobs use simple locking via `JobLock` table - not
  distributed-system safe
- File paths are sanitized but folder structure is flat (one folder per podcast)
- WebSocket connections don't persist across server restarts
