# 🎵 kmhd2playlist

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/kmhd2playlist)
[![Go Report Card](https://goreportcard.com/badge/github.com/toozej/monogo/apps/kmhd2playlist)](https://goreportcard.com/report/github.com/toozej/monogo/apps/kmhd2playlist)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/kmhd2playlist/cicd.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/kmhd2playlist)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/kmhd2playlist/total)

> 🎷 Automatically sync songs from KMHD jazz radio to your Spotify playlists

**kmhd2playlist** is a Go application that fetches the KMHD jazz radio playlist via JSON API and automatically adds newly played songs to your Spotify playlist. It uses fuzzy matching to find the best artist matches and can run continuously to keep your playlist up-to-date with the latest jazz discoveries.

## ✨ Features

- 🎯 **Smart Matching**: Uses fuzzy search to find the best artist matches on Spotify
- 🔄 **Continuous Sync**: Monitor KMHD in real-time with configurable intervals
- 🎵 **Duplicate Prevention**: Automatically skips songs already in your playlist
- 🔐 **OAuth Integration**: Secure Spotify authentication with local callback server
- 📊 **Detailed Logging**: Comprehensive sync summaries and progress tracking
- 🐳 **Docker Support**: Run anywhere with Docker or Docker Compose
- 🛠️ **Make-Driven**: All operations managed through simple `make` commands

## 🚀 Quick Start

### Using Make (Recommended)

```bash
# Install from latest release
make install

# Or build and run locally
make local
```

### Using Docker

```bash
# Build and run with Docker
make build run

# Or use Docker Compose
make up
```

## ⚙️ Configuration

1. **Copy the sample environment file:**
   ```bash
   cp .env.sample .env
   ```

2. **Configure your Spotify app:**
   - Visit [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
   - Create a new app and get your Client ID and Secret
   - Set redirect URI to `http://localhost:8080/callback`

3. **Update `.env` with your credentials:**
   ```bash
   # Required Spotify Configuration
   SPOTIFY_CLIENT_ID=your_client_id_here
   SPOTIFY_CLIENT_SECRET=your_client_secret_here
   SPOTIFY_REDIRECT_URI=http://localhost:8080/callback
   SPOTIFY_PLAYLIST_NAME_PREFIX=KMHD

   # Optional KMHD Configuration (uses defaults if not set)
   KMHD_API_ENDPOINT=https://www.kmhd.org/pf/api/v3/content/fetch/playlist
   KMHD_HTTP_TIMEOUT=30
   ```

   **Monthly Playlist Feature**: The app creates monthly playlists automatically based on your prefix configuration:
   - Set `SPOTIFY_PLAYLIST_NAME_PREFIX=KMHD` to create playlists like "KMHD-2025-10", "KMHD-2025-11", etc.
   - Each month gets its own playlist to keep them manageable (recommended)
   - Leave the prefix empty to use your first existing playlist (legacy behavior)
   
   **Important: Manual Folder Organization Required**
   - **Spotify's API limitation**: Folders cannot be created or managed programmatically
   - **What the app does**: Creates playlists with consistent naming and provides organization instructions
   - **What you must do**: Manually organize playlists into folders using Spotify Desktop
   - **Recommendation**: Create a folder named after your prefix (e.g., "KMHD") and drag monthly playlists into it

## 🎮 Usage

### Understanding the Workflow

1. **First Run**: App creates a new monthly playlist (e.g., "KMHD-2025-10")
2. **Playlist Population**: Songs from KMHD are automatically added to the current month's playlist
3. **Monthly Rotation**: Each month, a new playlist is created automatically
4. **Manual Organization**: You organize playlists into folders using Spotify Desktop (optional but recommended)

### Sync Commands

```bash
# Single sync operation
kmhd2playlist sync

# Continuous monitoring (checks every hour with randomization)
kmhd2playlist sync --continuous

# Custom interval
kmhd2playlist sync --continuous --interval 30m
```

### Make Commands

```bash
# 🔧 Development
make local-build          # Build binary locally
make local-test           # Run tests
make local-run            # Run with environment variables
make local-iterate        # Auto-rebuild on file changes

# 🐳 Docker Operations  
make build                # Build Docker image
make run                  # Run Docker container
make up                   # Start with Docker Compose
make down                 # Stop Docker Compose

# 🧪 Testing & Quality
make test                 # Run tests in Docker
make local-cover          # View coverage report
make pre-commit           # Run all quality checks

# 📦 Release & Deploy
make local-release        # Build release artifacts
make install              # Install from GitHub releases
```



## 🐳 Docker Deployment

### Docker Compose (Recommended)

```bash
# Deploy with Docker Compose
make up
```

### Standalone Docker

```bash
# Build and run
make build run

# Or pull from registry with volume mount for token persistence
mkdir -p ./data
docker run --rm --env-file .env -v ./data:/app/data toozej/kmhd2playlist:latest sync --continuous
```

### Token Persistence in Docker

When running in Docker, the Spotify authentication token needs to be persisted between container restarts. The application supports configuring the token file path via the `SPOTIFY_TOKEN_FILE_PATH` environment variable:

```bash
# For Docker Compose (already configured in docker-compose.yml)
SPOTIFY_TOKEN_FILE_PATH=/app/data/spotify_token.json

# For standalone Docker runs
docker run --rm \
  --env-file .env \
  -e SPOTIFY_TOKEN_FILE_PATH=/app/data/spotify_token.json \
  -v ./data:/app/data \
  toozej/kmhd2playlist:latest sync --continuous
```

This ensures your authentication persists between container restarts and you won't need to re-authenticate every time.

## 🔧 Advanced Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SPOTIFY_CLIENT_ID` | Spotify app client ID | Required |
| `SPOTIFY_CLIENT_SECRET` | Spotify app client secret | Required |
| `SPOTIFY_REDIRECT_URI` | OAuth redirect URI | `http://localhost:8080/callback` |
| `SPOTIFY_PLAYLIST_NAME_PREFIX` | Prefix for monthly playlists (creates "{prefix}-YYYY-MM" format) | Uses first existing playlist |
| `SPOTIFY_TOKEN_FILE_PATH` | Path to store Spotify auth token | `~/.config/kmhd2playlist/spotify_token.json` |
| `KMHD_API_ENDPOINT` | KMHD JSON API endpoint | `https://www.kmhd.org/pf/api/v3/content/fetch/playlist` |
| `KMHD_HTTP_TIMEOUT` | API request timeout (seconds) | `30` |
| `SERVER_HOST` | OAuth callback host | `127.0.0.1` |
| `SERVER_PORT` | OAuth callback port | `8080` |

### Continuous Mode Options

```bash
# Check every 30 minutes (minimum recommended interval)
kmhd2playlist sync --continuous --interval 30m

# Check every hour (default)
kmhd2playlist sync --continuous --interval 1h

# Check every 2 hours
kmhd2playlist sync --continuous --interval 2h
```

## 📊 Monitoring & Logging

The application provides detailed logging and sync summaries:

```
🎵 Found on KMHD: Miles Davis / Kind of Blue
   🎯 Found artist: Miles Davis (confidence: 0.95)
   ✅ Added to playlist: Kind of Blue

📊 Sync Summary:
   • Songs processed: 5
   • Songs synced: 3
   • Songs skipped: 2
   • Target playlist: Jazz Discoveries
```

## 📁 Organizing Playlists into Folders

**Important**: Spotify's Web API does not support automatic folder creation or playlist organization. This is a platform limitation, not an application limitation. All folder management must be done manually through the Spotify Desktop application.

### How the Playlist Prefix System Works

1. **Automatic Playlist Creation**: When you set `SPOTIFY_PLAYLIST_NAME_PREFIX=KMHD`, the app creates monthly playlists:
   - October 2025: `KMHD-2025-10`
   - November 2025: `KMHD-2025-11`
   - December 2025: `KMHD-2025-12`

2. **Folder Organization Reminders**: Each time a new playlist is created, the app will:
   - Display a console message suggesting folder organization
   - Include folder organization hints in the playlist description
   - Log recommendations for manual organization

### Manual Organization Steps (Spotify Desktop Required)

1. **Create a Folder**: Right-click in your playlist sidebar → "Create folder"
2. **Name the Folder**: Use your configured prefix (e.g., "KMHD")
3. **Drag Playlists**: Drag your monthly playlists into the folder
4. **Collapse for Organization**: Collapse the folder to keep your sidebar clean

### Platform Limitations

- **Spotify Desktop**: Full folder management capabilities
- **Spotify Mobile**: Folders are visible but cannot be created or managed
- **Spotify Web Player**: Limited folder functionality
- **API Limitation**: No programmatic folder creation or playlist organization

The app provides clear instructions and reminders, but folder organization remains a manual process due to Spotify's API restrictions.

## 🏗️ Development

### Prerequisites

- Go 1.25+
- Make
- Docker (optional)

### Setup Development Environment

```bash
# Install development dependencies
make pre-reqs

# Set up pre-commit hooks
make pre-commit-install

# Run the full development workflow
make local
```

### Project Structure

```
├── cmd/kmhd2playlist/     # CLI application entry point
├── internal/
│   ├── api/              # KMHD JSON API integration
│   ├── spotify/          # Spotify API integration  
│   ├── search/           # Fuzzy artist matching
│   └── types/            # Shared data structures
├── pkg/
│   ├── config/           # Configuration management
│   └── version/          # Version information
└── scripts/              # Build and deployment scripts
```

### Testing

```bash
# Run all tests
make local-test

# Run tests with coverage
make local-cover

# Run mutation tests
make mutation-test

# Watch for changes and test
make watch-test
```

### Development Workflow


```bash
# Set up development environment
make pre-reqs

# Make changes and test continuously
make local-iterate

# Run full test suite before committing
make local-test local-cover pre-commit
```

## 🙏 Acknowledgments

- 🎷 **KMHD Jazz Radio** for providing excellent jazz programming
- 🎵 **Spotify** for their comprehensive music API
- 🧠 [**KMHD Fetcher**](https://github.com/mccutchen/kmhd-playlist-fetcher/) for alerting me to the available KMHD radio playlist API
- 🛠️ **Go Community** for excellent tooling and libraries

---

**Made with ❤️ for jazz lovers everywhere** 🎺

