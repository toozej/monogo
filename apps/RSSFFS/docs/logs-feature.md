# Logs in the Web UI

The RSSFFS web UI now displays real-time logs in the browser. These logs describe requests to process RSS feeds.

## Features

### Log Display Panel
- **Toggle Visibility**: Select `Show Logs` to expand the logs panel
- **Real-time Updates**: The panel automatically refreshes visible logs every 2 seconds
- **Auto-show**: Logs panel automatically opens when a form submission starts
- **Clear Functionality**: Select `Clear` to remove the displayed logs

### Log Information
Each log entry displays:
- **Timestamp**: The creation time of the log entry
- **Level**: Log level (ERROR, WARN, INFO, DEBUG) with color coding
- **Message**: The actual log message
- **Fields**: Additional structured data (if any)

### Colors for Log Levels
- **ERROR**: Red background with red border
- **WARN**: Yellow background with orange border  
- **INFO**: Blue background with blue border
- **DEBUG**: Green background with green border

## API Endpoints

### GET /logs
Returns recent log entries as JSON.

**Query Parameters:**
- `limit` (optional): Number of log entries to return (default: 50, max: 200)

**Response:**
```json
{
  "success": true,
  "logs": [
    {
      "timestamp": "2025-10-19T17:58:21.521979-07:00",
      "level": "info",
      "message": "Starting RSSFFS web server...",
      "fields": {}
    }
  ]
}
```

### `GET /logs/stream`
Server-Sent Events (SSE) endpoint for real-time log streaming.

**Headers:**
- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`

**Events:**
- `log`: Data for a new log entry
- `heartbeat`: Keep-alive signal

## Implementation Details

### Log Capture System
- Uses a custom logrus hook (`WebUIHook`) to capture log entries
- Maintains a circular buffer of recent log entries (default: 100 entries)
- Mutex locks protect concurrent access
- Automatically captures all log levels (DEBUG, INFO, WARN, ERROR, FATAL, PANIC)

### Web UI Integration
- JavaScript automatically polls the `/logs` endpoint when logs are visible
- Responsive design that works on mobile and desktop
- Proper error management for network issues
- Automatic scrolling to show latest logs

### Security Considerations
- Logs are only available to users who can access the web UI
- The server omits sensitive information, such as API keys and passwords, from logs
- Rate limits apply to log endpoints like other API endpoints
- Proper CORS headers for cross-origin requests
- XSS protection: Uses safe DOM methods instead of innerHTML
- Error management: The server handles all errors from JSON serialization
- Input sanitization: The UI safely displays log messages with `textContent`

## Usage Examples

### View Logs While RSSFFS Processes Feeds
1. Open the RSSFFS web UI
2. Enter a URL and submit the form
3. The logs panel will automatically open showing:
   - Start of a request
   - Domain extraction and validation
   - RSS feed discovery attempts
   - Subscription results or errors
   - Request completion timing

### Debug Connection Issues
When RSS reader connectivity fails, logs will show:
```
ERROR: Error processing RSSFFS request: error getting categoryId from category Test: Get "https://rss.example.com/v1/categories": dial tcp: connect: no route to host
```

### Monitor Server Activity
Server startup logs show:
```
INFO: Starting RSSFFS web server...
INFO: Server will be available at: http://0.0.0.0:8080
DEBUG: Debug mode enabled
DEBUG: RSS Reader Endpoint: https://rss.example.com
```

## Configuration

Serve mode automatically enables logs. You do not need additional configuration.

### Log Buffer Size
The default buffer size is 100 entries. You can change this value in the server initialization code if you need a different buffer size.

### Poll Interval
The web UI polls for new logs every 2 seconds when the logs panel is visible. This provides a good balance between responsiveness and server load.

## Browser Compatibility

The logs feature works in all modern browsers that support:
- Fetch API
- ES6 JavaScript features
- CSS Grid and Flexbox
- Server-Sent Events (for real-time streaming)

## Performance Impact

- Minimal memory usage (circular buffer with fixed size)
- No performance impact when logs panel is hidden
- Efficient JSON serialization for API responses
- Automatic cleanup of old log entries