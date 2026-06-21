# Web UI Logs Feature

The RSSFFS web UI now includes a comprehensive logging system that allows users to view real-time logs of RSS feed processing requests directly in the browser.

## Features

### Log Display Panel
- **Toggle Visibility**: Click "Show Logs" to expand the logs panel
- **Real-time Updates**: Logs are automatically refreshed every 2 seconds when visible
- **Auto-show**: Logs panel automatically opens when a form submission starts
- **Clear Functionality**: Clear the displayed logs with the "Clear" button

### Log Information
Each log entry displays:
- **Timestamp**: When the log entry was created
- **Level**: Log level (ERROR, WARN, INFO, DEBUG) with color coding
- **Message**: The actual log message
- **Fields**: Additional structured data (if any)

### Color Coding
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

### GET /logs/stream
Server-Sent Events (SSE) endpoint for real-time log streaming.

**Headers:**
- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`

**Events:**
- `log`: New log entry data
- `heartbeat`: Keep-alive signal

## Implementation Details

### Log Capture System
- Uses a custom logrus hook (`WebUIHook`) to capture log entries
- Maintains a circular buffer of recent log entries (default: 100 entries)
- Thread-safe with proper mutex locking
- Automatically captures all log levels (DEBUG, INFO, WARN, ERROR, FATAL, PANIC)

### Web UI Integration
- JavaScript automatically polls the `/logs` endpoint when logs are visible
- Responsive design that works on mobile and desktop
- Proper error handling for network issues
- Automatic scrolling to show latest logs

### Security Considerations
- Logs are only available to users who can access the web UI
- No sensitive information is logged (API keys, passwords, etc.)
- Rate limiting applies to log endpoints like other API endpoints
- Proper CORS headers for cross-origin requests
- XSS protection: Uses safe DOM methods instead of innerHTML
- Error handling: All JSON encoding errors are properly handled
- Input sanitization: Log messages are safely displayed using textContent

## Usage Examples

### Viewing Logs During RSS Processing
1. Open the RSSFFS web UI
2. Enter a URL and submit the form
3. The logs panel will automatically open showing:
   - Request processing start
   - Domain extraction and validation
   - RSS feed discovery attempts
   - Subscription results or errors
   - Request completion timing

### Debugging Connection Issues
When RSS reader connectivity fails, logs will show:
```
ERROR: Error processing RSSFFS request: error getting categoryId from category Test: Get "https://rss.example.com/v1/categories": dial tcp: connect: no route to host
```

### Monitoring Server Activity
Server startup logs show:
```
INFO: Starting RSSFFS web server...
INFO: Server will be available at: http://0.0.0.0:8080
DEBUG: Debug mode enabled
DEBUG: RSS Reader Endpoint: https://rss.example.com
```

## Configuration

The logs feature is automatically enabled when running in serve mode. No additional configuration is required.

### Log Buffer Size
The default buffer size is 100 entries. This can be modified in the server initialization code if needed.

### Polling Interval
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