# REST API Reference

Complete reference for Podgrab's REST API endpoints.

## Base URL

All API requests are relative to your Podgrab installation's base URL:

```
http://localhost:8080
```

If you've configured a `PASSWORD` environment variable, all endpoints require
HTTP Basic Authentication:

- Username: `podgrab`
- Password: `<your configured password>`

## Response Formats

- **Success**: HTTP 200/201/204 with JSON response body
- **Error**: HTTP 400/404/409 with JSON error message
- **Created**: HTTP 200 with created resource
- **No Content**: HTTP 204 (for deletions)

## Podcasts

### List All Podcasts

```http
GET /podcasts
```

**Query Parameters:**

- `sort` (optional): Sort field - `dateadded`, `name`, `lastepisode`
- `order` (optional): Sort order - `asc` or `desc`

**Response:**

```json
[
  {
    "id": "uuid",
    "title": "Podcast Title",
    "summary": "Description",
    "author": "Author Name",
    "image": "https://...",
    "url": "https://feed.url/rss",
    "lastEpisode": "2024-01-15T10:00:00Z",
    "downloadedEpisodesCount": 10,
    "downloadingEpisodesCount": 2,
    "allEpisodesCount": 25,
    "downloadedEpisodesSize": 524288000,
    "downloadingEpisodesSize": 104857600,
    "allEpisodesSize": 1073741824,
    "isPaused": false,
    "createdAt": "2024-01-01T00:00:00Z",
    "updatedAt": "2024-01-15T10:00:00Z"
  }
]
```

**Example:**

```bash
curl http://localhost:8080/podcasts?sort=name&order=asc
```

### Add Podcast

```http
POST /podcasts
Content-Type: application/json
```

**Request Body:**

```json
{
  "url": "https://example.com/feed.rss"
}
```

**Response:**

```json
{
  "id": "uuid",
  "title": "Podcast Title",
  "summary": "Description",
  "author": "Author Name",
  "image": "https://...",
  "url": "https://example.com/feed.rss",
  "createdAt": "2024-01-15T10:00:00Z"
}
```

**Error Responses:**

- `409 Conflict`: Podcast already exists
- `400 Bad Request`: Invalid RSS feed URL

**Example:**

```bash
curl -X POST http://localhost:8080/podcasts \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/feed.rss"}'
```

### Get Podcast by ID

```http
GET /podcasts/:id
```

**Response:**

```json
{
  "id": "uuid",
  "title": "Podcast Title",
  "summary": "Description",
  "author": "Author Name",
  "image": "https://...",
  "url": "https://feed.url/rss",
  "podcastItems": [...],
  "tags": [...]
}
```

### Get Podcast Cover Image

```http
GET /podcasts/:id/image
```

Returns the podcast cover image. Redirects to original URL if local copy doesn't
exist.

**Response:** Image file (JPEG/PNG)

### Delete Podcast

```http
DELETE /podcasts/:id
```

Deletes podcast and all associated episodes (files and database records).

**Response:** HTTP 204 No Content

### Delete Podcast Only

```http
DELETE /podcasts/:id/podcast
```

Deletes podcast database record but preserves downloaded episode files.

**Response:** HTTP 204 No Content

### Delete Podcast Episodes

```http
DELETE /podcasts/:id/items
```

Deletes all episode files and database records for a podcast, but keeps the
podcast itself.

**Response:** HTTP 204 No Content

### Pause Podcast

```http
GET /podcasts/:id/pause
```

Pauses automatic episode downloads for this podcast.

**Response:**

```json
{}
```

### Unpause Podcast

```http
GET /podcasts/:id/unpause
```

Resumes automatic episode downloads for this podcast.

**Response:**

```json
{}
```

### Get Podcast Episodes

```http
GET /podcasts/:id/items
```

**Response:**

```json
[
  {
    "id": "uuid",
    "podcastId": "podcast-uuid",
    "title": "Episode Title",
    "summary": "Description",
    "episodeType": "full",
    "duration": 3600,
    "pubDate": "2024-01-15T10:00:00Z",
    "fileURL": "https://...",
    "guid": "unique-guid",
    "image": "https://...",
    "downloadDate": "2024-01-15T11:00:00Z",
    "downloadPath": "/assets/podcast/episode.mp3",
    "downloadStatus": 2,
    "isPlayed": false,
    "bookmarkDate": "0001-01-01T00:00:00Z",
    "localImage": "/assets/images/episode.jpg",
    "fileSize": 52428800
  }
]
```

### Download All Episodes

```http
GET /podcasts/:id/download
```

Queues all episodes in the podcast for download.

**Response:**

```json
{}
```

### Get Podcast RSS Feed

```http
GET /podcasts/:id/rss
```

Generates a custom RSS feed for this podcast using Podgrab as the media source.

**Response:** XML RSS feed

## Episodes (Podcast Items)

### List All Episodes

```http
GET /podcastitems
```

**Query Parameters:**

- `page` (default: 1): Page number
- `count` (default: 10): Items per page
- `podcastId` (optional): Filter by podcast ID
- `tagId` (optional): Filter by tag ID
- `onlyDownloaded` (optional): Show only downloaded episodes
- `onlyBookmarked` (optional): Show only bookmarked episodes
- `sortBy` (default: release_desc): Sort order
  - `release_asc`: Release date ascending
  - `release_desc`: Release date descending
  - `duration_asc`: Duration ascending
  - `duration_desc`: Duration descending

**Response:**

```json
{
  "podcastItems": [...],
  "filter": {
    "page": 1,
    "count": 10,
    "totalCount": 100,
    "totalPages": 10,
    "podcastId": "",
    "tagId": "",
    "onlyDownloaded": false,
    "onlyBookmarked": false,
    "sortBy": "release_desc"
  }
}
```

### Get Episode by ID

```http
GET /podcastitems/:id
```

**Response:**

```json
{
  "id": "uuid",
  "podcastId": "podcast-uuid",
  "title": "Episode Title",
  "summary": "Description",
  "duration": 3600,
  "pubDate": "2024-01-15T10:00:00Z",
  "fileURL": "https://...",
  "downloadStatus": 2,
  "isPlayed": false,
  "fileSize": 52428800
}
```

### Get Episode Image

```http
GET /podcastitems/:id/image
```

Returns the episode artwork. Redirects to original URL if local copy doesn't
exist.

**Response:** Image file (JPEG/PNG)

### Get Episode File

```http
GET /podcastitems/:id/file
```

Downloads or streams the episode audio file. Redirects to original URL if not
downloaded locally.

**Response:** Audio file (MP3/M4A/etc.)

**Headers:**

- `Content-Description: File Transfer`
- `Content-Transfer-Encoding: binary`
- `Content-Disposition: attachment; filename=<filename>`
- `Content-Type: audio/mpeg` (or detected type)

### Download Episode

```http
GET /podcastitems/:id/download
```

Queues episode for download.

**Response:**

```json
{}
```

### Delete Episode

```http
GET /podcastitems/:id/delete
```

Deletes the downloaded episode file and updates database status.

**Response:**

```json
{}
```

### Mark Episode as Played

```http
GET /podcastitems/:id/markPlayed
```

Marks episode as played.

**Response:** HTTP 200 OK

### Mark Episode as Unplayed

```http
GET /podcastitems/:id/markUnplayed
```

Marks episode as unplayed.

**Response:** HTTP 200 OK

### Bookmark Episode

```http
GET /podcastitems/:id/bookmark
```

Adds episode to bookmarks with current timestamp.

**Response:** HTTP 200 OK

### Remove Bookmark

```http
GET /podcastitems/:id/unbookmark
```

Removes episode from bookmarks.

**Response:** HTTP 200 OK

### Update Episode

```http
PATCH /podcastitems/:id
Content-Type: application/json
```

**Request Body:**

```json
{
  "isPlayed": true,
  "title": "Updated Title"
}
```

**Response:**

```json
{
  "id": "uuid",
  "title": "Updated Title",
  "isPlayed": true,
  ...
}
```

## Tags

### List All Tags

```http
GET /tags
```

**Response:**

```json
[
  {
    "id": "uuid",
    "label": "Technology",
    "description": "Tech podcasts",
    "podcasts": [...],
    "createdAt": "2024-01-01T00:00:00Z"
  }
]
```

### Create Tag

```http
POST /tags
Content-Type: application/json
```

**Request Body:**

```json
{
  "label": "Technology",
  "description": "Tech podcasts"
}
```

**Response:**

```json
{
  "id": "uuid",
  "label": "Technology",
  "description": "Tech podcasts",
  "createdAt": "2024-01-15T10:00:00Z"
}
```

**Error Responses:**

- `409 Conflict`: Tag with this label already exists

### Get Tag by ID

```http
GET /tags/:id
```

**Response:**

```json
{
  "id": "uuid",
  "label": "Technology",
  "description": "Tech podcasts",
  "podcasts": [...]
}
```

### Delete Tag

```http
DELETE /tags/:id
```

**Response:** HTTP 204 No Content

### Add Tag to Podcast

```http
POST /podcasts/:id/tags/:tagId
```

Associates a tag with a podcast.

**Response:**

```json
{}
```

### Remove Tag from Podcast

```http
DELETE /podcasts/:id/tags/:tagId
```

Removes tag association from podcast.

**Response:**

```json
{}
```

### Get Tag RSS Feed

```http
GET /tags/:id/rss
```

Generates RSS feed containing all episodes from podcasts with this tag.

**Response:** XML RSS feed

## Search

### Search Podcasts

```http
GET /search
```

**Query Parameters:**

- `q` (required): Search query
- `searchSource` (required): `itunes` or `podcastindex`

**Response:**

```json
[
  {
    "url": "https://feed.url/rss",
    "title": "Podcast Title",
    "image": "https://...",
    "already_saved": false,
    "description": "Description",
    "categories": ["Technology", "News"]
  }
]
```

**Example:**

```bash
curl "http://localhost:8080/search?q=javascript&searchSource=itunes"
```

## OPML Import/Export

### Export OPML

```http
GET /opml
```

**Query Parameters:**

- `usePodgrabLink` (optional): Use Podgrab RSS URLs instead of original feed
  URLs (`true`/`false`, default: `false`)

**Response:** OPML XML file

**Headers:**

- `Content-Disposition: attachment; filename=podgrab-export.opml`

**Example:**

```bash
curl "http://localhost:8080/opml?usePodgrabLink=true" -o podcasts.opml
```

### Import OPML

```http
POST /opml
Content-Type: multipart/form-data
```

**Form Data:**

- `file`: OPML file

**Response:**

```json
{
  "success": "File uploaded"
}
```

**Example:**

```bash
curl -X POST http://localhost:8080/opml \
  -F "file=@podcasts.opml"
```

## Settings

### Update Settings

```http
POST /settings
Content-Type: application/json
```

**Request Body:**

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

**Response:**

```json
{
  "message": "Success"
}
```

## RSS Feeds

### Global RSS Feed

```http
GET /rss
```

Generates RSS feed containing all downloaded episodes from all podcasts.

**Response:** XML RSS feed

## Data Models

### Download Status

Episode download status enumeration:

| Value | Status        | Description                            |
| ----- | ------------- | -------------------------------------- |
| 0     | NotDownloaded | Episode not yet downloaded             |
| 1     | Downloading   | Download in progress                   |
| 2     | Downloaded    | Successfully downloaded                |
| 3     | Deleted       | Previously downloaded but file deleted |

### Episode Type

- `full`: Full episode
- `trailer`: Podcast trailer
- `bonus`: Bonus episode

## Rate Limiting

No built-in rate limiting. Consider implementing at reverse proxy level for
production deployments.

## Error Handling

### Standard Error Response

```json
{
  "error": "Error message description",
  "message": "Detailed error information"
}
```

### Common HTTP Status Codes

- `200 OK`: Successful request
- `204 No Content`: Successful deletion
- `400 Bad Request`: Invalid request data
- `404 Not Found`: Resource not found
- `409 Conflict`: Resource already exists
- `500 Internal Server Error`: Server error

## WebSocket API

See [WebSocket API](websocket.md) for real-time communication documentation.

## Notes

- All timestamps are in ISO 8601 format with UTC timezone
- UUIDs are used for all resource identifiers
- File sizes are in bytes
- Durations are in seconds
- Some endpoints use GET for state-changing operations (legacy design)
- Background operations (downloads, refreshes) return immediately and process
  asynchronously

## Related Documentation

- [WebSocket API](websocket.md) - Real-time updates
- [User Guide](../guides/user-guide.md) - Using the API
- [Architecture Overview](../architecture/overview.md) - System design
