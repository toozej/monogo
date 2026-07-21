# 🎵 kmhd2playlist

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/monogo)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/kmhd2playlist)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/monogo/total)

<img src="img/avatar.png" alt="kmhd2playlist avatar" style="background-color: #FFFFFF;" />

> 🎷 Automatically sync songs from KMHD jazz radio to your Spotify or YouTube Music playlists

**kmhd2playlist** is a Go application that fetches the KMHD jazz radio playlist via JSON API and automatically adds newly played songs to your Spotify or YouTube Music playlist. It uses fuzzy matching to find the best artist matches and can run continuously to keep your playlist up-to-date with the latest jazz discoveries.

## ✨ Features

- 🎯 **Smart Matching**: Uses fuzzy search to find the best artist matches on Spotify or YouTube Music
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

2. **Choose your music provider:**

    ### Spotify Setup (default)
    - Visit [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
    - Create a new app and get your Client ID and Secret
    - Set redirect URI to `http://localhost:8080/callback`

    ### YouTube Music Setup
    - Log into YouTube Music in your browser.
    - Copy the complete headers from an authenticated `POST` request to `music.youtube.com/youtubei/v1/browse`.
    - Store the headers in `auth/youtubemusic-headers.json`. Docker Compose mounts this directory read-only at `/app/auth`.

    **Getting YouTube Music browser headers:**

    1. Open [music.youtube.com](https://music.youtube.com) and ensure you're logged in
    2. Open your browser's developer tools:
       - **Firefox**: `Ctrl+Shift+I` / `Cmd+Opt+I` → Network tab
       - **Chrome**: `Ctrl+Shift+I` / `Cmd+Opt+I` → Network tab
       - **Safari**: `Cmd+Opt+I` → Network tab
    3. Open **Library** or refresh the page, then select a successful `POST` request whose URL contains `/browse`.
    4. Copy its request headers. The request must be from `music.youtube.com` while you are signed in.
    5. Create `auth/youtubemusic-headers.json` with the browser values below. Preserve the account index and visitor ID from your request.

    Example (placeholders — replace every value with your own):

    ```bash
    mkdir -p auth
    $EDITOR auth/youtubemusic-headers.json
    ```

    ```json
    {
      "Cookie": "SID=...; HSID=...; SSID=...; SAPISID=...; __Secure-3PAPISID=...",
      "X-Goog-AuthUser": "0",
      "X-Goog-Visitor-Id": "...",
      "X-YouTube-Client-Name": "67",
      "X-YouTube-Client-Version": "1.20260721.01.00",
      "Origin": "https://music.youtube.com",
      "Referer": "https://music.youtube.com/"
    }
    ```

    Notes:
    - `Cookie` and `X-Goog-AuthUser` are required. `X-Goog-Visitor-Id`, `X-YouTube-Client-Name`, and `X-YouTube-Client-Version` are strongly recommended because YouTube requires this browser context for playlist mutations.
    - Do not copy a stale `Authorization` header; the application derives a current `SAPISIDHASH` for every request.
    - `auth/` is ignored by Git. The container runs as UID 65532, so on a rootful Linux Docker host restrict the bind-mounted credential without blocking the container: `sudo chown 65532:65532 auth/youtubemusic-headers.json && sudo chmod 600 auth/youtubemusic-headers.json`.
    - Refresh the file after signing out, changing Google accounts, or seeing a YouTube authorization error.
    - `YOUTUBEMUSIC_COOKIE` remains a legacy fallback. It lacks the complete browser request context and may be rejected for playlist mutations.

3. **Update `.env` with your credentials:**
    ```bash
    # Music Provider Selection (spotify or youtube)
    MUSIC_CLIENT=spotify

    # Required only when MUSIC_CLIENT=spotify
    SPOTIFY_CLIENT_ID=your_client_id_here
    SPOTIFY_CLIENT_SECRET=your_client_secret_here
    SPOTIFY_REDIRECT_URI=http://localhost:8080/callback
    SPOTIFY_PLAYLIST_NAME_PREFIX=KMHD

    # Required only when MUSIC_CLIENT=youtube
    # Docker Compose mounts ./auth at /app/auth.
    YOUTUBEMUSIC_AUTH_FILE_PATH=/app/auth/youtubemusic-headers.json
    YOUTUBEMUSIC_PLAYLIST_NAME_PREFIX=KMHD

    # Optional KMHD Configuration (uses defaults if not set)
    KMHD_API_ENDPOINT=https://www.kmhd.org/pf/api/v3/content/fetch/playlist
    KMHD_HTTP_TIMEOUT=30
    ```

    **Monthly Playlist Feature**: The app creates monthly playlists automatically based on your prefix configuration:
    - Set `SPOTIFY_PLAYLIST_NAME_PREFIX=KMHD` or `YOUTUBEMUSIC_PLAYLIST_NAME_PREFIX=KMHD` to create playlists like "KMHD-2025-10", "KMHD-2025-11", etc.
    - Each month gets its own playlist to keep them manageable (recommended)
    - Leave the prefix empty to use your first existing playlist (legacy behavior)
    
    **Important: Manual Folder Organization Required**
    - **Spotify's API limitation**: Folders cannot be created or managed programmatically
    - **What the app does**: Creates playlists with consistent naming and provides organization instructions
    - **What you must do**: Manually organize playlists into folders using Spotify Desktop
    - **Recommendation**: Create a folder named after your prefix (e.g., "KMHD") and drag monthly playlists into it

## 🎮 Usage

### Understanding the Workflow

1. **First Run**: App creates a new monthly playlist (e.g., "KMHD-2025-10") on your selected provider
2. **Playlist Population**: Songs from KMHD are automatically added to the current month's playlist
3. **Monthly Rotation**: Each month, a new playlist is created automatically
4. **Manual Organization**: You organize playlists into folders using Spotify Desktop (optional but recommended, Spotify only)

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
# 1) Create your .env file and browser-header file for the selected provider
cp .env.sample .env
$EDITOR .env
mkdir -p auth
$EDITOR auth/youtubemusic-headers.json

# 2) Deploy with Docker Compose. It mounts ./auth read-only and uses a named
#    volume at /app/data for persisted token state.
make up

# 3) Tail the logs while you verify authentication
docker compose logs -f kmhd2playlist
```

> The container is configured `read_only: true` and runs as the distroless
> `nonroot` user (UID 65532). The named `kmhd2playlist-data` volume is mounted
> at `/app/data` with image-provided nonroot ownership. Browser headers are
> mounted read-only from `./auth`; token state is stored separately at
> `/app/data/youtubemusic_token.json`.

### Standalone Docker

```bash
# Build and run
make build run

# Or pull from registry with browser headers and a named data volume
docker volume create kmhd2playlist-data
docker run --rm --env-file .env \
  -v "$(pwd)/auth:/app/auth:ro" \
  -v kmhd2playlist-data:/app/data \
  toozej/kmhd2playlist:latest sync --continuous
```

### Token Persistence in Docker

When running in Docker, the music provider authentication token needs to be persisted between container restarts. The application supports configuring the token file path via environment variables:

```bash
# For Docker Compose (already configured in docker-compose.yml)
SPOTIFY_TOKEN_FILE_PATH=/app/data/spotify_token.json
YOUTUBEMUSIC_TOKEN_FILE_PATH=/app/data/youtubemusic_token.json

# For standalone Docker runs
docker run --rm \
  --env-file .env \
  -e YOUTUBEMUSIC_AUTH_FILE_PATH=/app/auth/youtubemusic-headers.json \
  -e SPOTIFY_TOKEN_FILE_PATH=/app/data/spotify_token.json \
  -e YOUTUBEMUSIC_TOKEN_FILE_PATH=/app/data/youtubemusic_token.json \
  -v "$(pwd)/auth:/app/auth:ro" \
  -v kmhd2playlist-data:/app/data \
  toozej/kmhd2playlist:latest sync --continuous
```

This ensures your authentication persists between container restarts and you won't need to re-authenticate every time.

## 🔧 Advanced Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MUSIC_CLIENT` | Music provider to use (`spotify` or `youtube`) | `spotify` |
| `SPOTIFY_CLIENT_ID` | Spotify app client ID | Required when `MUSIC_CLIENT=spotify` |
| `SPOTIFY_CLIENT_SECRET` | Spotify app client secret | Required when `MUSIC_CLIENT=spotify` |
| `SPOTIFY_REDIRECT_URI` | OAuth redirect URI | Required when `MUSIC_CLIENT=spotify` |
| `SPOTIFY_PLAYLIST_NAME_PREFIX` | Prefix for monthly playlists (creates "{prefix}-YYYY-MM" format) | Uses first existing playlist |
| `SPOTIFY_TOKEN_FILE_PATH` | Path to store Spotify auth token | `~/.config/kmhd2playlist/spotify_token.json` |
| `YOUTUBEMUSIC_AUTH_FILE_PATH` | JSON file containing full headers from an authenticated YouTube Music `POST /browse` request. Preferred for playlist creation. | Required for YouTube Music unless legacy cookie is used |
| `YOUTUBEMUSIC_COOKIE` | Legacy raw YouTube Music `Cookie` header value. It lacks browser request context and may not authorize playlist mutations. | Legacy fallback |
| `YOUTUBEMUSIC_PLAYLIST_NAME_PREFIX` | Prefix for monthly playlists (creates "{prefix}-YYYY-MM" format) | Uses first existing playlist |
| `YOUTUBEMUSIC_TOKEN_FILE_PATH` | Path to store YouTube Music auth token | `~/.config/kmhd2playlist/youtubemusic_token.json` |
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


## 🙏 Acknowledgments

- 🎷 **KMHD Jazz Radio** for providing excellent jazz programming
- 🎵 **Spotify** for their comprehensive music API
- 🎵 **YouTube Music** for their music platform
- 🧠 [**KMHD Fetcher**](https://github.com/mccutchen/kmhd-playlist-fetcher/) for alerting me to the available KMHD radio playlist API
- 🛠️ **Go Community** for excellent tooling and libraries

---

**Made with ❤️ for jazz lovers everywhere** 🎺
