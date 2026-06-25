# Podgrab

Self-hosted podcast manager for automatically downloading and managing podcast
episodes.

[![Build Status](https://img.shields.io/github/actions/workflow/status/toozej/podgrab/build.yml?branch=main)](https://github.com/toozej/monogo/apps/podgrab/actions)
[![Code Quality](https://img.shields.io/github/actions/workflow/status/toozej/podgrab/code-quality.yml?branch=main&label=code%20quality)](https://github.com/toozej/monogo/apps/podgrab/actions)
[![Tests](https://img.shields.io/github/actions/workflow/status/toozej/podgrab/test.yml?branch=main&label=tests)](https://github.com/toozej/monogo/apps/podgrab/actions)
[![E2E Tests](https://img.shields.io/github/actions/workflow/status/toozej/podgrab/e2e-test.yml?branch=main&label=e2e)](https://github.com/toozej/monogo/apps/podgrab/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/toozej/monogo/apps/podgrab)](https://goreportcard.com/report/github.com/toozej/monogo/apps/podgrab)
[![codecov](https://codecov.io/gh/toozej/podgrab/branch/main/graph/badge.svg)](https://codecov.io/gh/toozej/podgrab)
[![License](https://img.shields.io/github/license/toozej/podgrab)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/toozej/podgrab)](https://hub.docker.com/r/toozej/podgrab)
[![Go Version](https://img.shields.io/github/go-mod/go-version/toozej/podgrab)](go.mod)

## Features

- 🎙️ **Automatic Downloads**: Subscribe to podcasts and automatically download
  new episodes
- 🔍 **iTunes Search**: Search and subscribe to podcasts from iTunes directory
- 📱 **Web Interface**: Clean, responsive web UI for managing your podcasts
- 🎧 **Built-in Player**: Stream episodes directly from the web interface
- 📊 **Episode Management**: Mark episodes as played, bookmark favorites
- 🏷️ **Tagging System**: Organize podcasts with custom tags
- ⚙️ **Flexible Settings**: Customize download behavior, file naming, and more
- 🔄 **OPML Import/Export**: Easy migration from other podcast managers
- 🐳 **Docker Support**: Simple deployment with Docker or Docker Compose
- 🌐 **Multi-platform**: Supports amd64, arm64, arm/v6, and arm/v7

## Quick Start

### Docker

```bash
docker run -d \
  -p 8080:8080 \
  -v podgrab-config:/config \
  -v podgrab-data:/assets \
  --name=podgrab \
  toozej/podgrab
```

### Docker Compose

```yaml
services:
  podgrab:
    image: toozej/podgrab
    container_name: podgrab
    environment:
      - CHECK_FREQUENCY=240
    volumes:
      - ./config:/config
      - ./assets:/assets
    ports:
      - 8080:8080
```

### From Source

**Prerequisites**: Go 1.26+, Make

```bash
# Clone repository
git clone https://github.com/toozej/podgrab.git
cd podgrab

# Install dependencies and build
make local-update-deps local-vendor local-build

# Run (requires .env file)
make local-run
```

The compiled binary is output to `out/podgrab`.

Access the web interface at `http://localhost:8080`

### Make Targets

The Makefile provides comprehensive build and development commands:

```bash
make help                    # Show all available targets
make local                   # Full local workflow (deps, vet, test, build, run)
make local-build             # Build binary to out/podgrab
make local-test              # Run tests with race detection
make local-cover             # View coverage report in browser
make pre-commit              # Install and run pre-commit hooks
make docker-login            # Login to container registries
make install                 # Install from latest GitHub release
```

For a complete list of targets, run `make help` or see the [Makefile](Makefile).

## Configuration

### Environment Variables

- `PORT`: HTTP port (default: `8080`)
- `DATA`: Directory for downloaded episodes (default: `./assets`)
- `CONFIG`: Directory for database and backups (default: `.`)
- `CHECK_FREQUENCY`: Minutes between RSS feed checks (default: `30`)
- `PASSWORD`: Enable basic authentication (username: `podgrab`)
- `LOG_LEVEL`: Logging verbosity - `debug`, `info`, `warn`, `error` (default:
  `info`)
- `PUID`: Sets the UID of the container user (default: `1000`)
- `PGID`: Sets the GID of the container user (default: `1000`)

### Application Settings

Configure via the web UI at `http://localhost:8080/settings`:

- **Download on Add**: Automatically download when adding a podcast
- **Initial Download Count**: Number of episodes to download initially
- **Auto Download**: Automatically download new episodes
- **Max Concurrency**: Concurrent download limit
- **Append Date**: Add date prefix to episode filenames
- **Filename Format**: Customize episode file naming

## Development

### Prerequisites

- Go 1.26 or higher
- Make (build automation)
- Chrome/Chromium (for E2E tests)
- pre-commit (for code quality hooks)

### Setup

```bash
# Install pre-commit hooks and development tools
make pre-commit

# Run all tests
make local-test

# View coverage report
make local-cover

# Run integration tests
go test ./integration_test/... -v -tags=integration

# Run E2E tests (requires Chrome)
go test ./e2e_test/... -v -tags=e2e
```

### Project Structure

```
podgrab/
├── main.go                 # Application entry point
├── controllers/            # HTTP request handlers
├── service/               # Business logic
├── db/                    # Database layer (GORM + SQLite)
├── model/                 # Data models
├── client/                # HTML templates
├── webassets/             # Static files (CSS, JS, images)
├── internal/
│   ├── database/          # Repository interface & implementations
│   └── testing/           # Test helpers and mocks
├── integration_test/      # Integration tests
├── e2e_test/              # End-to-end tests
└── docs/                  # Documentation
```

### Testing

Podgrab has comprehensive test coverage:

- **Unit Tests**: 60+ tests for service, database, and controller layers
- **Integration Tests**: 19 tests for complete workflows
- **E2E Tests**: 21 browser-based tests

For detailed testing documentation, see [testing.md](docs/testing.md).

### Code Quality

All code changes must pass:

- `golangci-lint` (comprehensive linting)
- `gosec` (security scanning)
- `pre-commit` hooks (25 automated checks)
- Test coverage thresholds (85%+ overall)

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## Documentation

- **[Documentation Index](docs/readme.md)**: Complete documentation navigation
- **[Testing Guide](docs/testing.md)**: Comprehensive testing documentation
- **[CI/CD Guide](docs/ci-cd.md)**: GitHub Actions workflows and automation
- **[Contributing](CONTRIBUTING.md)**: How to contribute to Podgrab
- **[Logging Guide](internal/logger/README.md)**: Structured logging
  documentation
- **[API Documentation](docs/api/)**: REST API reference
- **[Architecture](docs/architecture/)**: System design and architecture

## Technology Stack

- **Backend**: Go 1.26+
- **Web Framework**: Gin
- **Database**: GORM with SQLite
- **Templating**: Go HTML templates
- **RSS Parsing**: gofeed
- **Background Jobs**: gocron
- **Real-time Updates**: WebSockets
- **Logging**: Uber Zap (structured logging)
- **Testing**: chromedp (E2E), testify (assertions)

## Support

- **Issues**: [GitHub Issues](https://github.com/toozej/monogo/apps/podgrab/issues)
- **Discussions**:
  [GitHub Discussions](https://github.com/toozej/monogo/apps/podgrab/discussions)

## Credits

### Original Author

**Podgrab** was created and is maintained by
**[Akhil Gupta](https://github.com/akhilrex)** (akhilrex).

- **Original Repository**: <https://github.com/akhilrex/podgrab>
- **Docker Hub**: <https://hub.docker.com/r/akhilrex/podgrab>

## Acknowledgments

Built with ❤️ using Go and open source libraries. Special thanks to:

- [Akhil Gupta](https://github.com/akhilrex) for creating the original podgrab
- [Robert Wlodarczyk](https://github.com/SimplicityGuy) for creating an updated fork with many modernizations
- The Podgrab community for feedback and contributions
- The Go community for excellent tooling and libraries
- Contributors to gofeed, gin, gorm, and other dependencies
