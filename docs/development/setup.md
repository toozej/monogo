# Development Environment Setup

Complete guide for setting up a Podgrab development environment.

## Prerequisites

### Required Software

- **Go**: 1.15 or later
- **Git**: For version control
- **Make**: For build automation (optional)
- **Docker**: For containerized development (optional)
- **Chrome/Chromium**: For E2E tests (optional)

### Installation

#### Go

**Ubuntu/Debian:**

```bash
sudo apt-get update
sudo apt-get install golang-go

# Verify installation
go version
```

**macOS:**

```bash
brew install go

# Verify installation
go version
```

**Manual Installation:**

```bash
# Download from https://golang.org/dl/
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz

# Add to PATH in ~/.bashrc or ~/.zshrc
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

# Reload shell
source ~/.bashrc

# Verify
go version
```

#### Git

**Ubuntu/Debian:**

```bash
sudo apt-get install git
```

**macOS:**

```bash
brew install git
```

## Project Setup

### Clone Repository

```bash
# Clone from GitHub
git clone https://github.com/akhilrex/podgrab.git
cd podgrab
```

### Install Dependencies

```bash
# Download Go dependencies
go mod download

# Verify dependencies
go mod verify
```

### Project Structure

```
podgrab/
├── client/               # HTML templates
│   ├── index.html
│   ├── episodes.html
│   └── ...
├── controllers/          # HTTP handlers
│   ├── pages.go         # Page rendering
│   ├── podcast.go       # API endpoints
│   └── websockets.go    # WebSocket handler
├── db/                   # Database layer
│   ├── db.go            # Connection
│   ├── dbfunctions.go   # CRUD operations
│   ├── migrations.go    # Schema migrations
│   └── podcast.go       # Data models
├── model/                # External models
│   ├── podcastModels.go # RSS parsing
│   ├── itunesModel.go   # iTunes API
│   └── opmlModels.go    # OPML import/export
├── service/              # Business logic
│   ├── podcastService.go # Podcast operations
│   ├── fileService.go    # File management
│   └── itunesService.go  # iTunes search
├── webassets/            # Static assets
│   ├── css/
│   ├── js/
│   └── images/
├── main.go               # Application entry point
├── go.mod                # Go module definition
├── go.sum                # Dependency checksums
├── Dockerfile            # Container build
└── docker-compose.yml    # Docker setup
```

### Environment Configuration

Create `.env` file in project root:

```env
# Required: Data directory
DATA=/tmp/podgrab/assets

# Required: Config directory
CONFIG=/tmp/podgrab/config

# Optional: Password protection
PASSWORD=development

# Optional: Check frequency (minutes)
CHECK_FREQUENCY=30

# Optional: Logging level (debug, info, warn, error)
LOG_LEVEL=debug
```

**Create directories:**

```bash
mkdir -p /tmp/podgrab/config
mkdir -p /tmp/podgrab/assets
```

## Running in Development

### Method 1: Direct Go Execution

```bash
# Run with auto-reload using air (optional)
go install github.com/cosmtrek/air@latest
air

# Or run directly
go run main.go
```

**Expected output:**

```
Config Dir:  /tmp/podgrab/config
Assets Dir:  /tmp/podgrab/assets
Check Frequency (mins):  30
[GIN-debug] Listening and serving HTTP on :8080
```

Access application: http://localhost:8080

### Method 2: Build and Run

```bash
# Build binary
go build -o podgrab .

# Run binary
./podgrab
```

### Method 3: Docker Development

```bash
# Build development image
docker build -t podgrab:dev .

# Run with volume mounts for hot-reload
docker run -it --rm \
  -p 8080:8080 \
  -v $(pwd):/app \
  -v /tmp/podgrab/config:/config \
  -v /tmp/podgrab/assets:/assets \
  -e PASSWORD=development \
  podgrab:dev
```

## Development Tools

### Hot Reload with Air

#### Install Air

```bash
go install github.com/cosmtrek/air@latest
```

#### Configure Air

Create `.air.toml`:

```toml
root = "."
tmp_dir = "tmp"

[build]
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ."
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_error = true

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false

[misc]
  clean_on_exit = false
```

#### Run with Air

```bash
air
```

### Code Formatting

```bash
# Format all Go files
go fmt ./...

# Check formatting
gofmt -l .

# Auto-format with goimports (recommended)
go install golang.org/x/tools/cmd/goimports@latest
goimports -w .
```

### Linting

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run

# Run with auto-fix
golangci-lint run --fix
```

### Static Analysis

```bash
# Install staticcheck
go install honnef.co/go/tools/cmd/staticcheck@latest

# Run static analysis
staticcheck ./...
```

## Testing

Podgrab has comprehensive test coverage with unit, integration, and E2E tests.

### Running Tests

```bash
# Run all unit tests
go test -v ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run integration tests
go test -tags=integration -v ./integration_test/...

# Run E2E tests (requires Chrome/Chromium)
go test -tags=e2e -v ./e2e_test/...

# Run specific test
go test -v -run TestFetchURL_ValidRSSFeed ./service/
```

See [Testing Guide](../testing.md) for comprehensive testing documentation.

### Manual Testing Checklist

- [ ] Add podcast from RSS URL
- [ ] Import OPML file
- [ ] Download episodes
- [ ] Play episodes in browser
- [ ] Create and manage tags
- [ ] Update settings
- [ ] Export OPML
- [ ] WebSocket connectivity
- [ ] Background job execution

## Database Development

### SQLite CLI

```bash
# Install SQLite
sudo apt-get install sqlite3

# Open database
sqlite3 /tmp/podgrab/config/podgrab.db

# Common commands
.tables          # List tables
.schema          # Show schema
.dump podcasts   # Dump table
.exit            # Exit
```

### Database Migrations

Migrations are in `db/migrations.go`. New migrations:

```go
func Migrate() {
    db := DB

    // Add migration
    migration := Migration{
        Name: "add_user_agent_setting",
        Date: time.Now(),
    }

    // Check if already run
    var existingMigration Migration
    if err := db.Where("name = ?", migration.Name).First(&existingMigration).Error; err != nil {
        // Run migration
        db.Model(&Setting{}).Where("1=1").Update("user_agent", "")

        // Record migration
        db.Create(&migration)
    }
}
```

### Database Reset

```bash
# WARNING: Deletes all data
rm /tmp/podgrab/config/podgrab.db

# Restart application to recreate
go run main.go
```

## Debugging

### VS Code Configuration

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Podgrab",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/main.go",
      "env": {
        "DATA": "/tmp/podgrab/assets",
        "CONFIG": "/tmp/podgrab/config",
        "PASSWORD": "development",
        "CHECK_FREQUENCY": "30",
        "LOG_LEVEL": "debug"
      },
      "args": []
    }
  ]
}
```

### GoLand/IntelliJ Configuration

1. Run → Edit Configurations
1. Add new Go Build configuration
1. Set environment variables
1. Run in debug mode (Shift+F9)

### Delve Debugger (CLI)

```bash
# Install Delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Run with debugger
dlv debug main.go

# Commands
break main.main    # Set breakpoint
continue          # Continue execution
print variable    # Print variable
step             # Step into
next             # Step over
```

### Logging

Podgrab uses structured logging with Uber Zap. Use the global logger instance:

```go
// Import logger
import "github.com/akhilrex/podgrab/internal/logger"

// Use structured logging
logger.Log.Infow("Processing podcast", "podcast_id", podcastID)
logger.Log.Errorw("Failed to download", "error", err, "url", url)
logger.Log.Debug("Debug message")
```

**Set log level via environment:**

```bash
# Enable debug logging
export LOG_LEVEL=debug
go run main.go

# Production logging
export LOG_LEVEL=warn
export GIN_MODE=release
go run main.go
```

See [Logging Guide](../../internal/logger/README.md) for detailed documentation.

### HTTP Request Debugging

```bash
# Test API endpoints
curl -v http://localhost:8080/podcasts

# Test with authentication
curl -v -u podgrab:development http://localhost:8080/podcasts

# Test POST request
curl -v -X POST http://localhost:8080/podcasts \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/feed.rss"}'
```

## Frontend Development

### Template Development

Templates are in `client/` directory. Changes require server restart (or use Air
for auto-reload).

**Template syntax:**

```html
<!-- Accessing data -->
{{ .title }}

<!-- Conditionals -->
{{ if .setting.DarkMode }}
  <body class="dark">
{{ else }}
  <body>
{{ end }}

<!-- Loops -->
{{ range .podcasts }}
  <div>{{ .Title }}</div>
{{ end }}

<!-- Custom functions -->
{{ formatDate .PubDate }}
{{ formatFileSize .FileSize }}
```

### Static Assets

Located in `webassets/`:

- CSS: `webassets/css/`
- JavaScript: `webassets/js/`
- Images: `webassets/images/`

Changes to static files don't require restart.

### JavaScript Development

Main JS file: `webassets/js/app.js`

Key libraries:

- Alpine.js for reactivity
- Axios for HTTP requests

## Common Development Tasks

### Add New API Endpoint

1. **Define model** (`model/` or `db/`)
1. **Create database function** (`db/dbfunctions.go`)
1. **Add service logic** (`service/`)
1. **Create controller** (`controllers/podcast.go`)
1. **Register route** (`main.go`)

**Example:**

```go
// 1. Model (db/podcast.go)
type Podcast struct {
    Base
    Title string
    URL   string
}

// 2. Database function (db/dbfunctions.go)
func GetPodcasts() (*[]Podcast, error) {
    var podcasts []Podcast
    err := DB.Find(&podcasts).Error
    return &podcasts, err
}

// 3. Service (service/podcastService.go)
func GetAllPodcasts() *[]Podcast {
    podcasts, _ := db.GetPodcasts()
    return podcasts
}

// 4. Controller (controllers/podcast.go)
func GetAllPodcasts(c *gin.Context) {
    podcasts := service.GetAllPodcasts()
    c.JSON(200, podcasts)
}

// 5. Route (main.go)
router.GET("/podcasts", controllers.GetAllPodcasts)
```

### Add Background Job

In `main.go`, add to `intiCron()`:

```go
func intiCron() {
    checkFrequency, _ := strconv.Atoi(os.Getenv("CHECK_FREQUENCY"))

    // Add new job
    gocron.Every(uint64(checkFrequency)).Minutes().Do(service.MyNewJob)

    <-gocron.Start()
}
```

### Add Database Migration

In `db/migrations.go`:

```go
func Migrate() {
    // ... existing migrations

    // New migration
    migration := Migration{
        Name: "add_new_field",
        Date: time.Now(),
    }

    var existingMigration Migration
    if err := db.Where("name = ?", migration.Name).First(&existingMigration).Error; err != nil {
        // Add column
        db.Migrator().AddColumn(&Podcast{}, "NewField")

        // Record migration
        db.Create(&migration)
    }
}
```

## Performance Profiling

### CPU Profiling

```go
import (
    "runtime/pprof"
    "os"
)

// Start CPU profiling
f, _ := os.Create("cpu.prof")
pprof.StartCPUProfile(f)
defer pprof.StopCPUProfile()

// Your code here
```

Analyze:

```bash
go tool pprof cpu.prof
(pprof) top
(pprof) web  # Graphical view (requires graphviz)
```

### Memory Profiling

```bash
# Run with memory profiling
go run -memprofile mem.prof main.go

# Analyze
go tool pprof mem.prof
```

## Building for Production

### Build Binary

```bash
# Standard build
go build -o podgrab .

# Optimized build (smaller binary)
go build -ldflags="-s -w" -o podgrab .

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o podgrab-linux-amd64 .

# Cross-compile for ARM (Raspberry Pi)
GOOS=linux GOARCH=arm GOARM=7 go build -o podgrab-linux-arm .
```

### Build Docker Image

```bash
# Build image
docker build -t podgrab:latest .

# Multi-architecture build
docker buildx create --use
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t podgrab:latest .
```

## Troubleshooting

### Port Already in Use

```bash
# Find process using port 8080
lsof -i :8080

# Kill process
kill -9 <PID>
```

### Database Locked

```bash
# Remove lock files
rm /tmp/podgrab/config/*.db-shm
rm /tmp/podgrab/config/*.db-wal
```

### Module Download Issues

```bash
# Clear module cache
go clean -modcache

# Re-download modules
go mod download
```

### Build Errors

```bash
# Clean build cache
go clean -cache

# Update dependencies
go get -u ./...
go mod tidy
```

## Contributing Workflow

See [Contributing Guidelines](contributing.md) for detailed contribution
process.

### Quick Start

1. Fork repository
1. Create feature branch
1. Make changes
1. Test thoroughly
1. Submit pull request

## Related Documentation

- [Contributing Guidelines](contributing.md) - How to contribute
- [Testing Guide](testing.md) - Testing documentation
- [Architecture Overview](../architecture/overview.md) - System architecture
- [REST API](../api/rest-api.md) - API reference
