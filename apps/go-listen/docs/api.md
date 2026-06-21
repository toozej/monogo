# go-listen REST API Documentation

This document describes the REST API endpoints available in the go-listen application.

## Base URL

All API endpoints are relative to your server's base URL:
```
http://localhost:8080
```

## Authentication

The go-listen API does not require authentication for internal network usage. All endpoints are publicly accessible within your network.

## Content Type

All API endpoints expect and return JSON data with the content type:
```
Content-Type: application/json
```

## Rate Limiting

The API implements rate limiting to prevent abuse:
- **Default Limit**: 10 requests per second per IP address
- **Burst Capacity**: 20 requests
- **Response**: HTTP 429 (Too Many Requests) when limit exceeded

## Security Features

The API includes several security protections:
- **CSRF Protection**: Required for state-changing operations
- **Input Validation**: All inputs are validated and sanitized
- **Security Headers**: Standard security headers are applied
- **Rate Limiting**: Per-IP rate limiting prevents abuse

## Error Responses

All endpoints return errors in a consistent format:

```json
{
  "success": false,
  "error": "Error message describing what went wrong"
}
```

Common HTTP status codes:
- `200` - Success
- `400` - Bad Request (invalid input)
- `405` - Method Not Allowed
- `429` - Too Many Requests (rate limited)
- `500` - Internal Server Error

## Endpoints

### 1. Get CSRF Token

Get a CSRF token required for state-changing operations.

**Endpoint:** `GET /api/csrf-token`

**Response:**
```json
{
  "csrf_token": "generated-csrf-token-string"
}
```

**Example:**
```bash
curl -X GET http://localhost:8080/api/csrf-token
```

---

### 2. Get Playlists

Retrieve playlists from the "Incoming" folder with optional search filtering.

**Endpoint:** `GET /api/playlists`

**Query Parameters:**
- `search` (optional): Filter playlists by name containing this term

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "playlist_id_string",
      "name": "Playlist Name",
      "uri": "spotify:playlist:playlist_id",
      "track_count": 25,
      "embed_url": "https://open.spotify.com/embed/playlist/playlist_id",
      "is_incoming": true
    }
  ]
}
```

**Examples:**

Get all incoming playlists:
```bash
curl -X GET http://localhost:8080/api/playlists
```

Search for playlists containing "rock":
```bash
curl -X GET "http://localhost:8080/api/playlists?search=rock"
```

---

### 3. Scrape Artists from Web Page

Scrape artist names from a web page and add their top 5 tracks to a playlist.

**Endpoint:** `POST /api/scrape-artists`

**Request Headers:**
```
Content-Type: application/json
X-CSRF-Token: your-csrf-token (required)
```

**Request Body:**
```json
{
  "url": "https://example.com/artist-recommendations",
  "css_selector": "div.post-content",
  "playlist_id": "spotify_playlist_id",
  "force": false
}
```

**Request Parameters:**
- `url` (required): URL of the web page to scrape (must be valid URL)
- `css_selector` (optional): CSS selector to target specific page sections (max 500 characters)
- `playlist_id` (required): Spotify playlist ID where tracks should be added
- `force` (optional): Set to `true` to bypass duplicate detection (default: `false`)

**Success Response:**
```json
{
  "success": true,
  "data": {
    "url": "https://example.com/artist-recommendations",
    "css_selector": "div.post-content",
    "artists_found": ["Artist One", "Artist Two", "Artist Three"],
    "match_results": [
      {
        "query": "Artist One",
        "matched": true,
        "artist": {
          "id": "artist_id",
          "name": "Artist One",
          "uri": "spotify:artist:artist_id",
          "genres": ["rock", "alternative"]
        },
        "confidence": 0.95,
        "tracks_added": 5,
        "was_duplicate": false,
        "error": ""
      },
      {
        "query": "Artist Two",
        "matched": false,
        "confidence": 0.3,
        "tracks_added": 0,
        "was_duplicate": false,
        "error": "No matching artist found with sufficient confidence"
      }
    ],
    "success_count": 1,
    "failure_count": 1,
    "duplicate_count": 0,
    "total_tracks_added": 5,
    "message": "Successfully scraped and processed 3 artists",
    "errors": []
  }
}
```

**Error Response:**
```json
{
  "success": false,
  "error": "Failed to fetch URL: connection timeout"
}
```

**Common Error Scenarios:**
- Invalid URL format (400 Bad Request)
- URL unreachable (502 Bad Gateway)
- Request timeout after 30 seconds (504 Gateway Timeout)
- Invalid CSS selector (400 Bad Request)
- CSS selector matches no content (404 Not Found)
- HTML parsing failure (422 Unprocessable Entity)
- No artists found in content (200 OK with empty results)

**CSS Selector Examples:**

For Reddit posts:
```bash
# Post content only (excluding comments)
curl -X POST http://localhost:8080/api/scrape-artists \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -d '{
    "url": "https://www.reddit.com/r/Music/comments/...",
    "css_selector": "div[data-test-id=\"post-content\"]",
    "playlist_id": "your_playlist_id"
  }'
```

For music blogs:
```bash
# Article content
curl -X POST http://localhost:8080/api/scrape-artists \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -d '{
    "url": "https://musicblog.com/best-artists-2024",
    "css_selector": "article.post-content",
    "playlist_id": "your_playlist_id"
  }'
```

Without CSS selector (scrapes entire page):
```bash
curl -X POST http://localhost:8080/api/scrape-artists \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -d '{
    "url": "https://example.com/artists",
    "playlist_id": "your_playlist_id"
  }'
```

---

### 4. Add Artist to Playlist

Add an artist's top 5 tracks to a specified playlist with duplicate detection.

**Endpoint:** `POST /api/add-artist`

**Request Headers:**
```
Content-Type: application/json
X-CSRF-Token: your-csrf-token (required)
```

**Request Body:**
```json
{
  "artist_name": "Artist Name",
  "playlist_id": "spotify_playlist_id",
  "force": false
}
```

**Request Parameters:**
- `artist_name` (required): Name of the artist to search for (1-100 characters)
- `playlist_id` (required): Spotify playlist ID where tracks should be added
- `force` (optional): Set to `true` to bypass duplicate detection (default: `false`)

**Success Response:**
```json
{
  "success": true,
  "message": "Successfully added 5 tracks from Artist Name to Playlist Name",
  "data": {
    "success": true,
    "artist": {
      "id": "artist_spotify_id",
      "name": "Artist Name",
      "uri": "spotify:artist:artist_id",
      "genres": ["rock", "alternative"]
    },
    "tracks_added": [
      {
        "id": "track_id",
        "name": "Track Name",
        "uri": "spotify:track:track_id",
        "artists": [
          {
            "id": "artist_id",
            "name": "Artist Name",
            "uri": "spotify:artist:artist_id",
            "genres": []
          }
        ],
        "duration_ms": 240000
      }
    ],
    "playlist": {
      "id": "playlist_id",
      "name": "Playlist Name",
      "uri": "spotify:playlist:playlist_id",
      "track_count": 30,
      "embed_url": "https://open.spotify.com/embed/playlist/playlist_id",
      "is_incoming": true
    },
    "was_duplicate": false,
    "message": "Successfully added 5 tracks from Artist Name to Playlist Name"
  }
}
```

**Duplicate Detection Response:**
When tracks already exist and `force` is `false`:
```json
{
  "success": false,
  "message": "Artist Name's tracks were already added to Playlist Name on 2024-01-15T10:30:00Z. Use force=true to add anyway.",
  "is_duplicate": true,
  "last_added": "2024-01-15T10:30:00Z",
  "data": {
    "success": false,
    "was_duplicate": true,
    "message": "Duplicate tracks detected"
  }
}
```

**Examples:**

Add artist without force (will detect duplicates):
```bash
curl -X POST http://localhost:8080/api/add-artist \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: EXAMPLE" \
  -d '{
    "artist_name": "Radiohead",
    "playlist_id": "37i9dQZF1DX0XUsuxWHRQd"
  }' #gitleaks:allow
```

Add artist with force (bypass duplicate detection):
```bash
curl -X POST http://localhost:8080/api/add-artist \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: EXAMPLE" \
  -d '{
    "artist_name": "Radiohead",
    "playlist_id": "37i9dQZF1DX0XUsuxWHRQd",
    "force": true
  }'
```

## CSS Selector Guide

CSS selectors allow you to target specific sections of web pages for artist extraction. Here are examples for common websites:

### Reddit

**Post content (excluding comments):**
```css
div[data-test-id="post-content"]
```

**Post title and body:**
```css
div.Post
```

**Text content only:**
```css
div[data-click-id="text"]
```

**Example:**
```bash
# Reddit music recommendation thread
curl -X POST http://localhost:8080/api/scrape-artists \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -d '{
    "url": "https://www.reddit.com/r/Portland/comments/1owigfd/looking_for_local_bands_to_listen_to/",
    "css_selector": "div[data-test-id=\"post-content\"]",
    "playlist_id": "your_playlist_id"
  }'
```

### Music Blogs

**Article content:**
```css
article.post-content
```

**Main content area:**
```css
main .content
```

**Specific sections:**
```css
.artist-list, .recommendations
```

**Example:**
```bash
# Music blog article
curl -X POST http://localhost:8080/api/scrape-artists \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -d '{
    "url": "https://pitchfork.com/features/lists-and-guides/...",
    "css_selector": "article.post-content",
    "playlist_id": "your_playlist_id"
  }'
```

### Forums

**Forum post body:**
```css
.post-body, .message-content
```

**First post only:**
```css
.post:first-child .post-body
```

**Example:**
```bash
# Forum discussion
curl -X POST http://localhost:8080/api/scrape-artists \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -d '{
    "url": "https://forum.example.com/thread/12345",
    "css_selector": ".post:first-child .post-body",
    "playlist_id": "your_playlist_id"
  }'
```

### General Tips

1. **Inspect the page**: Use browser developer tools (F12) to find the right selector
2. **Start broad**: If unsure, omit the CSS selector to scrape the entire page
3. **Test selectors**: Use browser console to test: `document.querySelectorAll("your-selector")`
4. **Avoid comments**: Target main content to avoid extracting artist names from comments
5. **Multiple elements**: The system combines text from all matching elements

### Common Patterns

| Website Type | Typical Selector | Description |
|--------------|------------------|-------------|
| Reddit | `div[data-test-id="post-content"]` | Post content only |
| Medium | `article` | Article content |
| WordPress | `.entry-content` | Post content |
| Generic blog | `article.post, .post-content` | Main article |
| Forum | `.post-body, .message-content` | Post body |

## Data Models

### Artist
```json
{
  "id": "string",           // Spotify artist ID
  "name": "string",         // Artist display name
  "uri": "string",          // Spotify URI
  "genres": ["string"]      // Array of genre strings
}
```

### Track
```json
{
  "id": "string",           // Spotify track ID
  "name": "string",         // Track title
  "uri": "string",          // Spotify URI
  "artists": [Artist],      // Array of artist objects
  "duration_ms": number     // Track duration in milliseconds
}
```

### Playlist
```json
{
  "id": "string",           // Spotify playlist ID
  "name": "string",         // Playlist name
  "uri": "string",          // Spotify URI
  "track_count": number,    // Number of tracks in playlist
  "embed_url": "string",    // Spotify embed URL
  "is_incoming": boolean    // Whether playlist is in "Incoming" folder
}
```

### Add Result
```json
{
  "success": boolean,       // Whether operation succeeded
  "artist": Artist,         // Artist that was processed
  "tracks_added": [Track],  // Array of tracks that were added
  "playlist": Playlist,     // Target playlist
  "was_duplicate": boolean, // Whether duplicates were detected
  "message": "string"       // Human-readable result message
}
```

### Scrape Result
```json
{
  "url": "string",                    // URL that was scraped
  "css_selector": "string",           // CSS selector used (if any)
  "artists_found": ["string"],        // Raw artist names extracted
  "match_results": [ArtistMatchResult], // Detailed results per artist
  "success_count": number,            // Number of successfully added artists
  "failure_count": number,            // Number of failed artists
  "duplicate_count": number,          // Number of duplicate artists skipped
  "total_tracks_added": number,       // Total tracks added across all artists
  "message": "string",                // Summary message
  "errors": ["string"]                // Array of error messages (if any)
}
```

### Artist Match Result
```json
{
  "query": "string",        // Original artist name from scraping
  "matched": boolean,       // Whether a Spotify match was found
  "artist": Artist,         // Matched Spotify artist (if found)
  "confidence": number,     // Match confidence score (0.0-1.0)
  "tracks_added": number,   // Number of tracks added for this artist
  "was_duplicate": boolean, // Whether artist was skipped as duplicate
  "error": "string"         // Error message (if failed)
}
```

## Error Handling

### Validation Errors

**Status Code:** `400 Bad Request`

Common validation errors:
- Missing required fields (`artist_name`, `playlist_id`)
- Invalid field lengths (artist name must be 1-100 characters)
- Invalid playlist ID format
- Missing CSRF token for POST requests

### Rate Limiting

**Status Code:** `429 Too Many Requests`

```json
{
  "success": false,
  "error": "Rate limit exceeded. Please try again later."
}
```

### Spotify API Errors

**Status Code:** `500 Internal Server Error`

Common scenarios:
- Artist not found
- Playlist not accessible
- Spotify API rate limits
- Network connectivity issues

```json
{
  "success": false,
  "error": "Failed to add artist: Artist 'Unknown Artist' not found"
}
```

## CLI Usage

The go-listen CLI provides a `scrape` command for web scraping operations.

### Scrape Command

```bash
go-listen scrape [URL] --playlist PLAYLIST_ID [flags]
```

**Flags:**
- `--playlist, -p`: Spotify playlist ID (required)
- `--selector, -s`: CSS selector for content extraction (optional)
- `--force, -f`: Force add even if duplicates exist (optional)

**Examples:**

Basic scraping (entire page):
```bash
go-listen scrape https://example.com/artists --playlist 37i9dQZF1DX0XUsuxWHRQd
```

With CSS selector:
```bash
go-listen scrape https://www.reddit.com/r/Music/comments/xyz \
  --selector "div[data-test-id='post-content']" \
  --playlist 37i9dQZF1DX0XUsuxWHRQd
```

Force add (bypass duplicate detection):
```bash
go-listen scrape https://example.com/artists \
  --playlist 37i9dQZF1DX0XUsuxWHRQd \
  --force
```

**Output:**

The CLI displays a summary of the scraping operation:

```
Scraping artists from: https://example.com/artists
CSS Selector: div.post-content
Target Playlist: My Incoming Playlist

Found 5 potential artists:
  ✓ Artist One (confidence: 0.95) - Added 5 tracks
  ✓ Artist Two (confidence: 0.87) - Added 5 tracks
  ⊘ Artist Three - Duplicate (skipped)
  ✗ Artist Four - No match found (confidence too low: 0.32)
  ✓ Artist Five (confidence: 0.91) - Added 5 tracks

Summary:
  Artists found: 5
  Successfully added: 3
  Duplicates skipped: 1
  Failed: 1
  Total tracks added: 15
```

**Exit Codes:**
- `0`: Success (at least one artist added)
- `1`: Failure (no artists added or error occurred)

## Usage Examples

### Complete Workflow Example

1. **Get CSRF Token:**
```bash
CSRF_TOKEN=$(curl -s http://localhost:8080/api/csrf-token | jq -r '.csrf_token')
```

2. **Get Available Playlists:**
```bash
curl -s http://localhost:8080/api/playlists | jq '.data[].name'
```

3. **Add Artist to Playlist:**
```bash
curl -X POST http://localhost:8080/api/add-artist \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -d '{
    "artist_name": "The Beatles",
    "playlist_id": "your_playlist_id_here"
  }' | jq '.'
```

### JavaScript Example

```javascript
// Get CSRF token
const csrfResponse = await fetch('/api/csrf-token');
const { csrf_token } = await csrfResponse.json();

// Add artist to playlist
const response = await fetch('/api/add-artist', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-CSRF-Token': csrf_token
  },
  body: JSON.stringify({
    artist_name: 'Pink Floyd',
    playlist_id: 'your_playlist_id',
    force: false
  })
});

const result = await response.json();
console.log(result);
```

## Configuration

API behavior can be configured through environment variables:

- `SECURITY_RATE_LIMIT_REQUESTS_PER_SECOND`: Rate limit per IP (default: 10)
- `SECURITY_RATE_LIMIT_BURST`: Burst capacity (default: 20)
- `SERVER_HOST`: Server bind address (default: localhost)
- `SERVER_PORT`: Server port (default: 8080)

## Troubleshooting

### Common Issues

1. **CSRF Token Missing/Invalid**
   - Ensure you get a fresh CSRF token before making POST requests
   - Include the token in the `X-CSRF-Token` header

2. **Rate Limited**
   - Reduce request frequency
   - Implement exponential backoff in your client

3. **Artist Not Found**
   - Check artist name spelling
   - Try variations of the artist name
   - The fuzzy matching handles some typos but may not catch all variations

4. **Playlist Access Issues**
   - Ensure the playlist exists in your "Incoming" folder
   - Verify Spotify credentials are configured correctly
   - Check that the playlist is not private or restricted

5. **Network Errors**
   - Verify server is running and accessible
   - Check firewall settings
   - Ensure Spotify API credentials are valid

For more detailed troubleshooting, check the server logs which include structured logging for all operations.