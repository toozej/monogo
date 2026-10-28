# CLAUDE.md

Use this guidance for Claude Code (claude.ai/code) tasks in this repository.

## Project Overview

Podgrab is a self-hosted podcast manager written in Go that automatically
downloads podcast episodes. It uses a lightweight web application with a Go backend and HTML template frontend.

**Stack**: Go 1.27.3+, Gin web framework, GORM with SQLite, HTML templates,
WebSockets for real-time updates, Uber Zap structured logs

## Development Commands

### Build and Run

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

**Note**: This project has test coverage (100+ tests). Run the test suite that covers your changes.

## Architecture Overview

### Application Structure

The codebase follows a layered architecture with clear separation of concerns:

**Entry Point** (`main.go`):

- Initializes database and runs migrations
- Sets up Gin router with middleware
- Configures template functions to render HTML
- Registers all HTTP routes for the REST API and pages
- Starts background jobs via gocron (episode refresh, downloads, backups)
- Launches WebSocket server for real-time updates

**Core Layers**:

1. **Controllers** (`controllers/`): HTTP request handlers

   - `podcast.go`: REST API endpoints for podcasts, episodes, tags
   - `pages.go`: Endpoints to render HTML pages
   - `websockets.go`: WebSocket handler for real-time client updates

1. **Service** (`service/`): Business logic

   - `podcastService.go`: Podcast and episode CRUD, RSS parsing, download
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

   - Models to parse RSS feeds
   - Models for iTunes API responses
   - OPML import and export models
   - Requests and responses DTOs

1. **Client** (`client/`): HTML templates

   - Go templates with custom functions defined in main.go
   - No separate JavaScript build process - templates served directly

1. **Internal** (`internal/`): Internal packages

   - `logger/`: Centralized structured logs with Uber Zap
   - `database/`: Repository interfaces and implementations
   - `testing/`: Test helpers and mocks

### Key Architectural Patterns

**Background Jobs**: The application uses `gocron` to schedule repeated tasks:

- `RefreshEpisodes()`: Checks RSS feeds for new episodes (every CHECK_FREQUENCY
  mins)
- `DownloadMissingEpisodes()`: Downloads queued episodes
- `CheckMissingFiles()`: Detects deleted files to update database
- `CreateBackup()`: Database backups (every 2 days)

**Download Flow**:

1. The user adds a podcast URL.
1. The controller parses RSS.
1. The service creates `Podcast` and `PodcastItem` records.
1. Background job detects new episodes → Queues downloads based on settings
1. `fileService.Download()` handles concurrent downloads (max controlled by
   `MaxDownloadConcurrency` option)
1. WebSocket broadcasts download progress to connected clients

**Real-time Updates**: WebSocket connection (`/ws`) broadcasts events:

- Download progress
- Episode status changes
- Playlist updates
- Clients subscribe and update UI reactively

**File Management**:

- Episodes stored in `/assets/{podcast-name}/` with sanitized filenames
- Optional date or episode number prefixes based on settings
- Cached images to reduce external requests
- File existence checking to prevent re-downloads

### Database Schema

**Core Models** (all use UUID primary keys via `db.Base`):

- `Podcast`: RSS feed metadata, relationship to episodes and tags
- `PodcastItem`: Individual episodes with download state tracking
- `Tag`: Labels to organize podcasts (many-to-many)
- `Setting`: Global app configuration (singleton)
- `JobLock`: Prevents duplicate background job execution

**Key Relationships**:

- Podcast → PodcastItems (one-to-many)
- Podcast ↔ Tags (many-to-many via `podcast_tags`)

**Download States** (enum in `podcast.go`):

- `NotDownloaded`: Queued for download
- `Downloading`: Download in progress
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
- `LOG_LEVEL`: Logs verbosity (`debug`, `info`, `warn`, `error`) - default:
  `info`

### Settings Database

Runtime settings stored in database (editable via `/settings` page):

- `DownloadOnAdd`: Auto-download after a podcast addition
- `InitialDownloadCount`: How many episodes to download initially
- `AutoDownload`: Enable automatic downloads for new episodes
- `AppendDateToFileName`: Add date prefix to filenames
- `MaxDownloadConcurrency`: Concurrent download limit (default: 5)

## Important Implementation Details

### Structured Logs

The application uses Uber Zap for structured logs via
`internal/logger/logger.go`:

- Global logger instance: `logger.Log` (SugaredLogger)
- Log levels controlled by `LOG_LEVEL` environment variable
- Console format in development, JSON in production (based on `GIN_MODE`)
- Always use structured logs with context:
  `logger.Log.Errorw("message", "key", value)`

For detailed log output documentation, see `internal/logger/README.md`.

### Template Functions

Custom template helpers defined in `main.go`:

- `formatDate`, `naturalDate`: Formats for times
- `formatFileSize`, `formatDuration`: Human-readable sizes and durations
- `downloadedEpisodes`, `downloadingEpisodes`: Episode stats
- `intRange`: Pagination helpers

When you modify templates, reference these functions in `main.go:35-128`.

### RSS Feed Parse

The application uses `encoding/xml` with custom models in `model/rssModels.go`.
When you handle new podcast sources:

- RSS parsers run in `service.FetchURL()` and `service/podcastService.go`
- XML namespaces through `xmlquery` for complex feeds
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

### Add Support for New Podcast Sources

1. Extend models to parse RSS in `model/rssModels.go` if needed
1. Update `service.FetchURL()` for custom parsing logic
1. Test with `service.AddPodcast()` which handles the full flow
1. Consider user-agent requirements in `fileService.httpClient()`

### Modify Download Behavior

1. Check the current logic in `service/fileService.go` (Download function)
1. Access settings through `db.GetOrCreateSetting()`
1. Filename generation in `getFileName()` considers multiple settings
1. Update WebSocket broadcasts if you add progress records

### Database Migrations

1. Add new migration in `db/migrations.go`
1. Migration runs automatically on startup via `db.Migrate()`
1. Use GORM auto-migration for simple schema changes
1. Complex migrations require explicit SQL in migration records

## File Organization Principles

- **No shared state**: Prefer packages that you can test independently
- **DB layer never calls service layer**: Prevents circular dependencies
- **Controllers are thin**: Business logic belongs in service layer
- **Models are data-only**: No business logic in model structs

## Known Gotchas

- Template parsing happens at startup - restart required for template changes
- SQLite database can lock under high concurrency - consider adjustments to the connection pool
- Background jobs use simple locks in the `JobLock` table - not
  distributed-system safe
- The application sanitizes file paths but keeps a flat folder structure (one folder per podcast)
- WebSocket connections do not persist across server restarts
