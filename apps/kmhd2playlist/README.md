# 🎵 kmhd2playlist

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/monogo)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/kmhd2playlist)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/monogo/total)

<img src="img/avatar.png" alt="kmhd2playlist avatar" style="background-color: #FFFFFF;" />

> 🎷 Automatically sync songs from KMHD jazz radio to your Spotify or YouTube Music playlists

**kmhd2playlist** fetches the playlist from KMHD jazz radio through a JSON API.
It automatically adds newly played songs to your Spotify or YouTube Music playlist. It uses fuzzy comparison to find the best artist matches and can run continuously to keep your playlist up-to-date with the latest jazz discoveries.

## ✨ Features

- 🎯 **Smart Matching**: Uses fuzzy search to find the best artist matches on Spotify or YouTube Music
- 🔄 **Continuous Sync**: Monitor KMHD in real-time with configurable intervals
- 🎵 **Duplicate Prevention**: Automatically skips songs already in your playlist
- 🔐 **OAuth Integration**: Secure Spotify authentication with local callback server
- 📊 **Detailed Logs**: sync summaries and progress tracking
- 🐳 **Docker Support**: Run anywhere with Docker or Docker Compose
- 🛠️ **Make-Driven**: All operations managed through simple `make` commands

## 🚀 Quick Start

### Use Make (Recommended)

```bash
# Install from latest release
make install

# Or build and run locally
make local
```

### Use Docker

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
    - Visit the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
    - Create a new app and get your Client ID and Secret
    - Set the redirect URI to `http://localhost:8080/callback`

    ### YouTube Music Setup
    - Log into YouTube Music in your browser.
    - Copy the complete headers from an authenticated `POST` request to `music.youtube.com/youtubei/v1/browse`.
    - Store the headers in `auth/youtubemusic-headers.json`. Docker Compose mounts this directory read-only at `/app/auth`.

    **Get browser headers from YouTube Music:**

    1. Open the [YouTube Music site](https://music.youtube.com). Log in
    2. Open the browser's developer tools:
       - **Firefox**: Press `Ctrl+Shift+I` or `Cmd+Opt+I`. Select the `Network` tab
       - **Chrome**: Press `Ctrl+Shift+I` or `Cmd+Opt+I`. Select the `Network` tab
       - **Safari**: Press `Cmd+Opt+I`. Select the `Network` tab
    3. Open `Library` or refresh the page. Select a successful `POST` request whose URL contains `/browse`.
    4. Copy its request headers. The request must be from `music.youtube.com` while your session is active.
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
    - YouTube requires `Cookie` and `X-Goog-AuthUser`.

    - Strong recommendation: Include `X-Goog-Visitor-Id`, `X-YouTube-Client-Name`, and `X-YouTube-Client-Version` for browser context that YouTube requires for playlist mutations.
    - Do not copy a stale `Authorization` header; the application derives a current `SAPISIDHASH` for every request.
    - Git ignores `auth/`. The container runs as UID 65532.
    - On a rootful Linux Docker host, restrict the bind-mounted credential and retain container access:

      ```bash
      sudo chown 65532:65532 auth/youtubemusic-headers.json && sudo chmod 600 auth/youtubemusic-headers.json
      ```
    - Refresh the file after you sign out, change Google accounts, or see a YouTube authorization error.
    - `YOUTUBEMUSIC_COOKIE` remains a legacy fallback. It lacks the complete browser request context and possibly fails authorization for playlist mutations.

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
    - Set `SPOTIFY_PLAYLIST_NAME_PREFIX=KMHD` or `YOUTUBEMUSIC_PLAYLIST_NAME_PREFIX=KMHD` to create playlists such as `KMHD-2025-10` and `KMHD-2025-11`
    - Each month gets its own playlist to keep them manageable (recommended)
    - Leave the prefix empty to use your first current playlist (legacy behavior)
    
    **Important: Manual Folder Organization Required**
    - **Spotify's API limitation**: The API cannot create or manage folders programmatically
    - **What the app does**: Creates playlists with consistent naming and provides organization instructions
    - **What you must do**: Manually organize playlists into folders with Spotify Desktop
    - **Recommendation**: Create a folder named after your prefix (for example `KMHD`) and drag monthly playlists into it

## 🎮 Usage

### Workflow

1. **First Run**: App creates a new monthly playlist (for example `KMHD-2025-10`) on your selected provider
2. **Playlist Population**: The app automatically adds KMHD songs to the current month's playlist
3. **Monthly Rotation**: The app automatically creates a new playlist each month
4. **Manual Organization**: You organize playlists into folders with Spotify Desktop (optional but recommended, Spotify only)

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

> The container uses `read_only: true` and runs as the distroless `nonroot` user (UID 65532).
> Compose mounts the named `kmhd2playlist-data` volume at `/app/data` with nonroot ownership from the image.
> It mounts browser headers read-only from `./auth` and stores token state separately at
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

When you run in Docker, persist the authentication token for the music provider between container restarts.
Configure the token file path through environment variables:

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

This ensures your authentication persists between container restarts and you will not need to re-authenticate every time.

## 🔧 Advanced Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MUSIC_CLIENT` | Music provider to use (`spotify` or `youtube`) | `spotify` |
| `SPOTIFY_CLIENT_ID` | Spotify app client ID | Required when `MUSIC_CLIENT=spotify` |
| `SPOTIFY_CLIENT_SECRET` | Spotify app client secret | Required when `MUSIC_CLIENT=spotify` |
| `SPOTIFY_REDIRECT_URI` | OAuth redirect URI | Required when `MUSIC_CLIENT=spotify` |
| `SPOTIFY_PLAYLIST_NAME_PREFIX` | Prefix for monthly playlists (creates "{prefix}-YYYY-MM" format) | Uses first current playlist |
| `SPOTIFY_TOKEN_FILE_PATH` | Path to store Spotify auth token | `~/.config/kmhd2playlist/spotify_token.json` |
| `YOUTUBEMUSIC_AUTH_FILE_PATH` | JSON file with full headers from an authenticated `POST /browse` request to YouTube Music. Preferred for playlist creation. | Required for YouTube Music unless you use the legacy cookie |
| `YOUTUBEMUSIC_COOKIE` | Legacy raw value of the `Cookie` header from YouTube Music. It lacks browser request context and possibly does not authorize playlist mutations. | Legacy fallback |
| `YOUTUBEMUSIC_PLAYLIST_NAME_PREFIX` | Prefix for monthly playlists (creates "{prefix}-YYYY-MM" format) | Uses first current playlist |
| `YOUTUBEMUSIC_TOKEN_FILE_PATH` | Path to store YouTube Music auth token | `~/.config/kmhd2playlist/youtubemusic_token.json` |
| `KMHD_API_ENDPOINT` | Endpoint for the KMHD JSON API | `https://www.kmhd.org/pf/api/v3/content/fetch/playlist` |
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

## 📊 Monitor & Logs

The application provides detailed log output and sync summaries:

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

## 📁 Organize Playlists into Folders

**Important**: Spotify's Web API does not support automatic folder creation or playlist organization. This is a platform limitation, not an application limitation. You must manage folders manually through the Spotify Desktop application.

### How Playlist Prefixes Work

1. **Automatic Playlist Creation**: When you set `SPOTIFY_PLAYLIST_NAME_PREFIX=KMHD`, the app creates monthly playlists:
   - October 2025: `KMHD-2025-10`
   - November 2025: `KMHD-2025-11`
   - December 2025: `KMHD-2025-12`

2. **Folder Organization Reminders**: Each time the app creates a playlist, it will:
   - Display a console message with suggestions for folder organization
   - Include hints for folder organization in the playlist description
   - Log recommendations for manual organization

### Manual Organization Steps (Spotify Desktop Required)

1. **Create a Folder**: Right-click in your playlist sidebar. Select `Create folder`
2. **Name the Folder**: Use your configured prefix (for example `KMHD`)
3. **Drag Playlists**: Drag your monthly playlists into the folder
4. **Collapse for Organization**: Collapse the folder to keep your sidebar clean

### Platform Limitations

- **Spotify Desktop**: All capabilities to manage folders
- **Spotify Mobile**: You can view folders but cannot create or manage them
- **Spotify Web Player**: Limited folder functionality
- **API Limitation**: No programmatic folder creation or playlist organization

The app provides clear instructions and reminders, but folder organization remains a manual process due to Spotify's API restrictions.


## 🙏 Acknowledgments

- 🎷 **KMHD Jazz Radio** for excellent jazz programs
- 🎵 **Spotify** for their music API
- 🎵 **YouTube Music** for their music platform
- 🧠 [**KMHD Fetcher**](https://github.com/mccutchen/kmhd-playlist-fetcher/) for the information about the available API for KMHD radio playlists
- 🛠️ **Go Community** for excellent tooling and libraries

---

**Made with ❤️ for jazz lovers everywhere** 🎺
