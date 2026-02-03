# Configuration Guide

Complete reference for configuring Podgrab through environment variables and
application settings.

## Configuration Levels

Podgrab has two configuration levels:

1. **Environment Variables**: Set at deployment/startup (Docker, system
   environment)
1. **Application Settings**: Configured through web UI or API

## Environment Variables

Set before starting Podgrab. Changes require restart.

### Required Variables

#### DATA

Downloaded episodes and media storage directory.

```bash
DATA=/path/to/assets
```

**Docker:**

```yaml
environment:
  - DATA=/assets
volumes:
  - /host/path/data:/assets
```

**Default:** `/assets` (inside container)

**Requirements:**

- Read/write permissions
- Sufficient storage space (podcasts can be large)
- Persistent storage (not tmpfs)

**Recommendations:**

- SSD not required (episodes are large files)
- HDD acceptable for cost/capacity
- Network storage (NAS) supported

#### CONFIG

Database and configuration storage directory.

```bash
CONFIG=/path/to/config
```

**Docker:**

```yaml
environment:
  - CONFIG=/config
volumes:
  - /host/path/config:/config
```

**Default:** `/config` (inside container)

**Contains:**

- `podgrab.db` - SQLite database
- `backups/` - Automatic database backups

**Requirements:**

- Read/write permissions
- Persistent storage (critical)
- Regular backups recommended

**Recommendations:**

- **SSD highly recommended** for database performance
- Keep on fast, reliable storage
- Include in backup strategy

### Optional Variables

#### PASSWORD

HTTP Basic Authentication password.

```bash
PASSWORD=mysecurepassword
```

**Docker:**

```yaml
environment:
  - PASSWORD=${PODGRAB_PASSWORD}
```

**Default:** None (no authentication)

**Behavior:**

- If set: All routes require authentication
- Username: Always `podgrab` (fixed)
- Password: Value of this variable
- Applies to: Web UI, API, WebSocket, RSS feeds

**Security Notes:**

- **Production deployments**: Strongly recommended
- **Private networks**: Optional but recommended
- **Public internet**: Required
- **Password strength**: Use strong, unique passwords

**Password in RSS Feeds:**

```
https://podgrab:password@podgrab.example.com/podcasts/id/rss
```

**Recommendation:**

```bash
# Generate strong password
openssl rand -base64 32
```

#### CHECK_FREQUENCY

Minutes between background job executions.

```bash
CHECK_FREQUENCY=30
```

**Docker:**

```yaml
environment:
  - CHECK_FREQUENCY=30
```

**Default:** `30` (minutes)

**Range:** 1-1440 (1 minute to 24 hours)

**Affected Jobs:**

- RSS feed refresh: Every `CHECK_FREQUENCY` minutes
- Download queue processing: Every `CHECK_FREQUENCY` minutes
- File verification: Every `CHECK_FREQUENCY` minutes
- Image downloads: Every `CHECK_FREQUENCY` minutes
- File size updates: Every `CHECK_FREQUENCY × 2` minutes
- Backup creation: Every 2 days (independent)
- Lock cleanup: Every `CHECK_FREQUENCY × 2` minutes

**Recommended Values:**

| Use Case         | Minutes | Reasoning                   |
| ---------------- | ------- | --------------------------- |
| Active listening | 15      | Fast new episode detection  |
| Normal use       | 30      | Balanced (default)          |
| Light use        | 60      | Lower resource usage        |
| Archive only     | 120+    | Minimal background activity |
| Development      | 5       | Quick testing               |

**Considerations:**

- **Lower values**:

  - ✅ Faster episode discovery
  - ✅ More responsive
  - ❌ Higher CPU usage
  - ❌ More network requests
  - ❌ Potential rate limiting from feeds

- **Higher values**:

  - ✅ Lower resource usage
  - ✅ Less network traffic
  - ❌ Delayed episode discovery
  - ❌ Delayed downloads

**Impact on specific jobs:**

```
CHECK_FREQUENCY=30

RSS Refresh:       Every 30 min
Downloads:         Every 30 min
File Check:        Every 30 min
Image Downloads:   Every 30 min
File Size Update:  Every 60 min (30×2)
Lock Cleanup:      Every 60 min (30×2)
Backups:           Every 2 days (independent)
```

#### LOG_LEVEL

Logging verbosity level for application output.

```bash
LOG_LEVEL=info
```

**Docker:**

```yaml
environment:
  - LOG_LEVEL=debug
```

**Default:** `info`

**Valid Values:**

| Level   | Description                                         | Use Case    |
| ------- | --------------------------------------------------- | ----------- |
| `debug` | Detailed information for diagnosing problems        | Development |
| `info`  | General informational messages                      | Production  |
| `warn`  | Warning messages for potentially harmful situations | Production  |
| `error` | Error messages for serious problems                 | Production  |

**Output Format:**

- **Development** (`GIN_MODE` != `release`):
  - Human-readable console output
  - Colored log levels
  - ISO8601 timestamps
- **Production** (`GIN_MODE` = `release`):
  - JSON formatted output
  - Machine-parseable
  - Optimized for log aggregation

**Recommendations:**

- **Development**: Use `debug` for detailed troubleshooting
- **Production**: Use `info` or `warn` to reduce log volume
- **Performance debugging**: Use `debug` temporarily, then revert
- **Containerized deployments**: JSON format integrates with log aggregators
  (ELK, Splunk, etc.)

**Example Output:**

```bash
# debug level (development)
2024-01-15T10:30:45Z  DEBUG  Processing podcast  podcast_id=abc123 name="Tech Podcast"
2024-01-15T10:30:46Z  INFO   Download started    episode_id=xyz789 size=50MB

# info level (production JSON)
{"level":"info","ts":"2024-01-15T10:30:46Z","msg":"Download started","episode_id":"xyz789","size":"50MB"}
```

**Structured Logging:**

Podgrab uses Uber Zap for structured logging with key-value pairs. For detailed
documentation, see [Logging Guide](../../internal/logger/README.md).

## Application Settings

Configured through Settings page (`/settings`) or API endpoint
(`POST /settings`).

Access Settings:

1. Navigate to `http://localhost:8080/settings`
1. Modify settings
1. Click "Save"
1. Changes take effect immediately (no restart required)

### Download Settings

#### Download on Add

Auto-download episodes when adding new podcast.

**Setting:** `downloadOnAdd` **Type:** Boolean **Default:** `true`

**Behavior:**

- `true`: Downloads episodes immediately after adding podcast
- `false`: Only fetches episode metadata, no downloads

**Works with:** `initialDownloadCount`

**Use Cases:**

- `true`: Want episodes ready to listen immediately
- `false`: Preview podcast before downloading, limited storage

**Example:**

```json
{
  "downloadOnAdd": true,
  "initialDownloadCount": 5
}
```

**Result:** Downloads 5 most recent episodes when adding podcast

#### Initial Download Count

Number of episodes to download when adding podcast.

**Setting:** `initialDownloadCount` **Type:** Integer **Default:** `5`
**Range:** 0-100 (practical range: 0-20)

**Behavior:**

- Downloads N most recent episodes
- Only applies if `downloadOnAdd` is `true`
- Counted from newest to oldest

**Recommendations:**

| Scenario            | Count | Reasoning                   |
| ------------------- | ----- | --------------------------- |
| New podcast         | 1-3   | Test before committing      |
| Known good podcast  | 5-10  | Catch up on recent episodes |
| Archive entire feed | 0     | Manually select episodes    |
| Binge listening     | 10-20 | Queue up content            |

**Storage Impact:**

```
Average episode: 50 MB
Initial count: 5
New podcast: 250 MB
10 podcasts: 2.5 GB
```

#### Auto Download

Automatically download new episodes from existing podcasts.

**Setting:** `autoDownload` **Type:** Boolean **Default:** `true`

**Behavior:**

- `true`: New episodes download automatically during RSS refresh
- `false`: New episodes detected but not downloaded

**Job:** Runs every `CHECK_FREQUENCY` minutes

**Use Cases:**

- `true`: Stay current automatically
- `false`: Selective downloading, limited storage

**Paused Podcasts:**

- Setting has no effect on paused podcasts
- Paused podcasts never auto-download

#### Max Download Concurrency

Maximum simultaneous episode downloads.

**Setting:** `maxDownloadConcurrency` **Type:** Integer **Default:** `5`
**Range:** 1-20 (practical range: 1-10)

**Behavior:**

- Limits concurrent HTTP downloads
- Queued downloads wait for slot availability
- Applies to all downloads (auto and manual)

**Recommendations by System:**

| System           | Cores | RAM   | Recommended |
| ---------------- | ----- | ----- | ----------- |
| Raspberry Pi 3   | 4     | 1GB   | 2-3         |
| Raspberry Pi 4   | 4     | 4GB   | 3-5         |
| NAS (2 core)     | 2     | 2GB   | 3           |
| Desktop (4 core) | 4     | 8GB   | 5-7         |
| Server (8+ core) | 8+    | 16GB+ | 10+         |

**Considerations:**

- **Network bandwidth**: Don't saturate connection
- **Storage I/O**: Don't overwhelm disk writes
- **CPU**: Each download uses some processing
- **Memory**: Each connection uses memory

**Performance Impact:**

```
Concurrency 1:  Sequential (slowest, lowest resource)
Concurrency 5:  Balanced (default)
Concurrency 10: Fast (higher resource usage)
Concurrency 20: Maximum (potential instability)
```

### File Naming Settings

#### Append Date to Filename

Add episode publication date to downloaded filename.

**Setting:** `appendDateToFileName` **Type:** Boolean **Default:** `false`

**Behavior:**

- `false`: `episode-title.mp3`
- `true`: `episode-title_2024-01-15.mp3`

**Format:** `YYYY-MM-DD`

**Use Cases:**

- Chronological organization
- Multiple episodes with same name
- Sorting by date in file browser

**Example:**

```
false: The Daily Show.mp3
true:  The Daily Show_2024-01-15.mp3
```

#### Append Episode Number to Filename

Add episode number to downloaded filename.

**Setting:** `appendEpisodeNumberToFileName` **Type:** Boolean **Default:**
`false`

**Behavior:**

- `false`: `episode-title.mp3`
- `true`: `001_episode-title.mp3` (if feed provides episode number)

**Requirements:**

- Feed must include episode number in RSS
- Not all podcasts provide this

**Use Cases:**

- Serialized podcasts (numbered series)
- Audiobooks
- Sequential content

**Example:**

```
false: Episode Title.mp3
true:  042_Episode Title.mp3
```

**Combined with Date:**

```
Both enabled: 042_Episode Title_2024-01-15.mp3
```

### User Interface Settings

#### Dark Mode

Enable dark color scheme.

**Setting:** `darkMode` **Type:** Boolean **Default:** `false`

**Behavior:**

- `false`: Light theme
- `true`: Dark theme

**Persistence:**

- Saved per browser/device
- Syncs across tabs
- Persists across sessions

**Recommendation:**

- Personal preference
- Reduces eye strain in low light
- Saves battery on OLED screens

#### Base URL

Custom base URL for generated RSS feeds.

**Setting:** `baseUrl` **Type:** String **Default:** Empty (auto-detect from
request)

**Behavior:**

- **Empty**: Uses request hostname
- **Set**: Uses configured value

**Use Cases:**

**Behind reverse proxy:**

```
baseUrl: https://podgrab.example.com
```

**Custom domain:**

```
baseUrl: https://podcasts.mydomain.com
```

**Local network:**

```
baseUrl: http://192.168.1.100:8080
```

**Example Impact:**

Without baseUrl:

```xml
<enclosure url="http://localhost:8080/podcastitems/123/file"/>
```

With baseUrl set to `https://podgrab.example.com`:

```xml
<enclosure url="https://podgrab.example.com/podcastitems/123/file"/>
```

**Important:**

- Required for RSS feeds to work externally
- Critical for sharing feeds
- Must include protocol (http/https)
- No trailing slash

### Advanced Settings

#### Download Episode Images

Download and store episode artwork locally.

**Setting:** `downloadEpisodeImages` **Type:** Boolean **Default:** `false`

**Behavior:**

- `false`: Reference original image URLs
- `true`: Download images to `/assets/images/`

**Storage Impact:**

```
Average image: 50-200 KB
100 episodes: 5-20 MB
1000 episodes: 50-200 MB
```

**Benefits:**

- Faster image loading
- Works offline
- Privacy (no external image requests)
- Consistent experience

**Drawbacks:**

- More storage usage
- Longer initial download time
- Bandwidth for image downloads

**Recommendation:**

- Enable if: Storage abundant, privacy important
- Disable if: Storage limited, bandwidth restricted

#### Generate NFO Files

Create metadata .nfo files alongside episodes.

**Setting:** `generateNFOFile` **Type:** Boolean **Default:** `false`

**Behavior:**

- `false`: No .nfo files
- `true`: Creates .nfo for podcast and episodes

**NFO Format:** XML metadata format

**Use Cases:**

- Media center integration (Kodi, Plex, Jellyfin)
- Metadata preservation
- Archive organization

**Files Created:**

```
/assets/podcast-name/
├── tvshow.nfo          # Podcast metadata
├── episode1.mp3
├── episode1.nfo        # Episode metadata
├── episode2.mp3
└── episode2.nfo
```

**NFO Content Example:**

```xml
<episodedetails>
  <title>Episode Title</title>
  <showtitle>Podcast Name</showtitle>
  <plot>Episode description</plot>
  <aired>2024-01-15</aired>
  <runtime>3600</runtime>
</episodedetails>
```

**Recommendation:**

- Enable if: Using media center software
- Disable if: Not needed (saves disk writes)

#### Don't Re-download Deleted Episodes

Skip re-downloading manually deleted episodes.

**Setting:** `dontDownloadDeletedFromDisk` **Type:** Boolean **Default:**
`false`

**Behavior:**

- `false`: Re-download deleted episodes if marked for download
- `true`: Never re-download episodes with status "Deleted"

**Use Cases:**

**Scenario 1: Free up space temporarily**

```
Setting: false (default)
1. Delete episodes to free space
2. Auto-download re-downloads them later
```

**Scenario 2: Permanently skip episodes**

```
Setting: true
1. Delete episodes you don't want
2. They stay deleted even if auto-download active
```

**Manual override:**

- Can always manually download deleted episodes
- This setting only affects automatic downloads

**Recommendation:**

- `false`: Default behavior, re-download is okay
- `true`: Intentional curation, keep deletions

#### User Agent

Custom HTTP User-Agent header for downloads.

**Setting:** `userAgent` **Type:** String **Default:** Empty (uses Go's default)

**Format:** `"AppName/Version (Additional Info)"`

**Use Cases:**

**Identify your instance:**

```
Podgrab/1.0 (MyServer)
```

**Comply with feed requirements:**

```
Some feeds require specific user agents
```

**Analytics/Tracking:**

```
Identify downloads in feed analytics
```

**Example Values:**

```
Podgrab/1.0
PodgrabServer/1.0 (MyDomain)
Mozilla/5.0 (compatible; Podgrab/1.0)
```

**Best Practices:**

- Include "Podgrab" for identification
- Add version if tracking multiple instances
- Be respectful of feed provider requirements

**Note:**

- Empty value uses Go's default User-Agent
- Some CDNs/feeds may block default agents
- Use custom agent if downloads fail

## Configuration via API

Settings can be updated via REST API.

### Get Current Settings

```http
GET /settings
```

**Response:**

```json
{
  "downloadOnAdd": true,
  "initialDownloadCount": 5,
  "autoDownload": true,
  "appendDateToFileName": false,
  "appendEpisodeNumberToFileName": false,
  "darkMode": false,
  "downloadEpisodeImages": true,
  "generateNFOFile": false,
  "dontDownloadDeletedFromDisk": false,
  "baseUrl": "https://podgrab.example.com",
  "maxDownloadConcurrency": 5,
  "userAgent": "Podgrab/1.0"
}
```

### Update Settings

```http
POST /settings
Content-Type: application/json
```

**Request:**

```json
{
  "downloadOnAdd": false,
  "initialDownloadCount": 3,
  "maxDownloadConcurrency": 3
}
```

**Response:**

```json
{
  "message": "Success"
}
```

**Note:** Only include fields you want to change.

## Configuration Best Practices

### Optimal Configuration by Use Case

#### Personal Podcast Library

```yaml
# Environment
CHECK_FREQUENCY: 30

# Settings
downloadOnAdd: true
initialDownloadCount: 5
autoDownload: true
maxDownloadConcurrency: 5
appendDateToFileName: false
```

#### Archive/Collection

```yaml
# Environment
CHECK_FREQUENCY: 120

# Settings
downloadOnAdd: false
initialDownloadCount: 0
autoDownload: false
maxDownloadConcurrency: 3
appendDateToFileName: true
appendEpisodeNumberToFileName: true
generateNFOFile: true
```

#### Podcast Server (Multi-User)

```yaml
# Environment
PASSWORD: <strong-password>
CHECK_FREQUENCY: 15

# Settings
downloadOnAdd: true
initialDownloadCount: 10
autoDownload: true
maxDownloadConcurrency: 10
baseUrl: https://podcasts.example.com
downloadEpisodeImages: true
```

#### Low-Resource Device (Raspberry Pi)

```yaml
# Environment
CHECK_FREQUENCY: 60

# Settings
downloadOnAdd: true
initialDownloadCount: 2
autoDownload: true
maxDownloadConcurrency: 2
downloadEpisodeImages: false
generateNFOFile: false
```

### Security Hardening

```yaml
# Environment
PASSWORD: <generated-strong-password>

# Settings
baseUrl: https://secure.domain.com
userAgent: Podgrab/1.0 SecureInstance
```

**Additional steps:**

- Use HTTPS reverse proxy
- Firewall rules
- Regular updates
- Strong passwords

### Performance Tuning

**High Performance:**

```yaml
CHECK_FREQUENCY: 15
maxDownloadConcurrency: 10
```

**Balanced:**

```yaml
CHECK_FREQUENCY: 30
maxDownloadConcurrency: 5
```

**Low Resource:**

```yaml
CHECK_FREQUENCY: 120
maxDownloadConcurrency: 2
```

## Troubleshooting Configuration

### Downloads Not Working

**Check:**

1. `autoDownload` enabled?
1. `CHECK_FREQUENCY` reasonable?
1. Storage space available?
1. `maxDownloadConcurrency` not too low?
1. Podcast not paused?

### Performance Issues

**If slow:**

1. Reduce `maxDownloadConcurrency`
1. Increase `CHECK_FREQUENCY`
1. Disable `downloadEpisodeImages`
1. Disable `generateNFOFile`

### RSS Feeds Not Working Externally

**Check:**

1. `baseUrl` configured correctly?
1. Includes protocol (http/https)?
1. Accessible from external network?
1. `PASSWORD` included in feed URL if set?

## Configuration Migration

### Upgrading from Old Versions

Settings are stored in database and persist across upgrades.

**New settings** get default values automatically.

**Example:** Upgrading adds `userAgent` setting

```
Before upgrade: Setting doesn't exist
After upgrade: userAgent = "" (default)
```

### Exporting Configuration

**Database includes all settings:**

```bash
# Backup database
cp /config/podgrab.db /backup/podgrab.db

# Restore on new instance
cp /backup/podgrab.db /config/podgrab.db
```

**Settings persist** in database backup.

## Related Documentation

- [User Guide](user-guide.md) - Using Podgrab
- [Docker Deployment](../deployment/docker.md) - Environment setup
- [Production Guide](../deployment/production.md) - Production configuration
- [REST API](../api/rest-api.md) - API settings endpoint

______________________________________________________________________

**Configure Podgrab to match your workflow!** ⚙️
