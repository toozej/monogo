# RSSFFS Web UI Usage Guide

This guide provides comprehensive examples and usage instructions for the RSSFFS web interface.

## Quick Start

1. **Configure RSSFFS**: Set up your RSS reader configuration (see [Configuration](#configuration))
2. **Start the web server**: Run `./RSSFFS serve`
3. **Open your browser**: Navigate to `http://localhost:8080`
4. **Submit URLs**: Use the web form to discover and subscribe to RSS feeds

## Starting the Web Server

### Basic Usage

```bash
# Start on default host (127.0.0.1) and port (8080)
./RSSFFS serve
```

The server will display startup information:
```
INFO[2024-10-14T12:00:00Z] Starting RSSFFS web server...
INFO[2024-10-14T12:00:00Z] Server will be available at: http://127.0.0.1:8080
INFO[2024-10-14T12:00:00Z] Press Ctrl+C to stop the server
```

### Custom Host and Port

```bash
# Bind to all interfaces on port 3000
./RSSFFS serve --host 0.0.0.0 --port 3000

# Use short flags
./RSSFFS serve -H 0.0.0.0 -p 3000
```

### Debug Mode

```bash
# Enable debug logging for troubleshooting
./RSSFFS serve --debug

# Combine with custom host/port
./RSSFFS serve -d -H 0.0.0.0 -p 8080
```

## Web Interface Features

### Mobile-Friendly Design

The web interface is optimized for mobile devices with:
- **Responsive layout**: Adapts to different screen sizes
- **Touch-friendly controls**: Large buttons and input fields
- **Mobile keyboard support**: Appropriate input types for URLs
- **Readable fonts**: Optimized typography for mobile screens

### Single URL Mode

The web interface includes a checkbox to enable single URL mode:

**Single URL Mode checkbox:**
- **Checked**: Only searches for RSS feeds on the provided URL's domain
- **Unchecked**: Uses traversal mode (default) - searches the provided URL and follows links to other domains

**Usage examples:**
```
URL: https://blog.example.com/post/123
☑ Single URL Mode → Only checks blog.example.com for RSS feeds

URL: https://news.example.com
☐ Single URL Mode → Checks news.example.com and any linked domains
```

**When to use Single URL Mode:**
- You want feeds only from a specific website
- You're on a slow connection and want faster results
- You want to avoid feeds from linked/referenced sites
- You're targeting a specific domain for feed discovery

### Form Validation

The interface provides both client-side and server-side validation:

**Client-side validation:**
- Real-time URL format checking
- Visual feedback for invalid inputs
- Prevention of empty form submissions

**Server-side validation:**
- URL format verification
- Category name sanitization
- Input length limits
- XSS protection

### User Feedback

**Success notifications:**
```
✓ Success! Found and added 3 RSS feeds to category "Tech Blogs"
```

**Error notifications:**
```
✗ Error: Invalid URL format. Please check your URL and try again.
✗ Error: RSS reader API endpoint not configured.
```

**Loading indicators:**
- Form submission shows spinner
- Submit button is disabled during processing
- Visual feedback prevents duplicate submissions

## Configuration

### RSS Reader Setup

The web interface uses the same configuration as the command-line tool. You can configure RSSFFS using either a configuration file or environment variables.

#### Configuration File (config.yaml)

```yaml
rss_reader_endpoint: "https://your-miniflux-instance.com"
rss_reader_api_key: "your-api-token"

# Optional: Enable single URL mode by default for web interface
rssffs_single_url_mode: false
```

#### Environment Variables

```bash
export RSS_READER_ENDPOINT="https://your-miniflux-instance.com"
export RSS_READER_API_KEY="your-api-token"

# Optional: Enable single URL mode by default for web interface
export RSSFFS_SINGLE_URL_MODE="true"
```

**Note**: When `RSSFFS_SINGLE_URL_MODE` is set to "true", the single URL mode checkbox will be checked by default in the web interface, but users can still uncheck it to use traversal mode.

### Miniflux Configuration

For Miniflux users, ensure your instance is properly configured:

1. **API Access**: Enable API access in Miniflux settings
2. **User Permissions**: Ensure your user has feed management permissions
3. **HTTPS**: Use HTTPS for production deployments
4. **Categories**: Pre-create categories in Miniflux if desired

## Deployment Examples

### Local Development

```bash
# Start server for local testing
./RSSFFS serve --debug
```

### Local Network Access

```bash
# Allow access from other devices on your network
./RSSFFS serve --host 0.0.0.0 --port 8080
```

Access from other devices using your machine's IP address:
`http://192.168.1.100:8080`

### Docker Deployment

```bash
# Build and run with Docker
docker build -t rssffs .
docker run -p 8080:8080 --env-file .env rssffs serve --host 0.0.0.0
```

### Docker Compose

```yaml
version: '3.8'
services:
  rssffs:
    build: .
    ports:
      - "8080:8080"
    environment:
      - RSS_READER_ENDPOINT=https://your-miniflux.com
      - RSS_READER_API_KEY=your-api-token
    command: ["serve", "--host", "0.0.0.0"]
```

### Reverse Proxy Setup

#### Nginx

```nginx
server {
    listen 80;
    server_name rssffs.example.com;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

#### Apache

```apache
<VirtualHost *:80>
    ServerName rssffs.example.com
    ProxyPreserveHost On
    ProxyPass / http://localhost:8080/
    ProxyPassReverse / http://localhost:8080/
</VirtualHost>
```

## Troubleshooting

### Common Issues

**Port already in use:**
```bash
# Check what's using the port
lsof -i :8080

# Use a different port
./RSSFFS serve --port 8081
```

**Configuration not found:**
```bash
# Specify config file location
./RSSFFS serve --config /path/to/config.yaml

# Or use environment variables
export RSS_READER_ENDPOINT="https://your-miniflux.com"
./RSSFFS serve
```

**Permission denied (port < 1024):**
```bash
# Use a port >= 1024 or run with sudo
./RSSFFS serve --port 8080

# Or use sudo for privileged ports
sudo ./RSSFFS serve --port 80
```

### Debug Mode

Enable debug mode to see detailed logging:

```bash
./RSSFFS serve --debug
```

Debug output includes:
- HTTP request details
- RSS feed discovery process
- API communication with RSS reader
- Configuration loading information

### Log Analysis

**Successful feed discovery:**
```
DEBU[2024-10-14T12:01:00Z] Processing URL: https://example.com
DEBU[2024-10-14T12:01:01Z] Found RSS feed: https://example.com/feed.xml
INFO[2024-10-14T12:01:02Z] Successfully added 1 feed to category "News"
```

**Configuration errors:**
```
ERRO[2024-10-14T12:01:00Z] RSS reader endpoint not configured
ERRO[2024-10-14T12:01:00Z] Please set RSS_READER_ENDPOINT environment variable
```

## Security Considerations

### Input Validation

The web interface implements multiple layers of security:
- URL format validation
- Input sanitization to prevent XSS
- CSRF protection for form submissions
- Rate limiting to prevent abuse

### Network Security

For production deployments:
- Use HTTPS with a reverse proxy
- Implement proper firewall rules
- Consider IP-based access restrictions
- Use strong authentication for your RSS reader

### Configuration Security

- Store sensitive configuration in environment variables
- Use secure file permissions for config files (600)
- Avoid committing credentials to version control
- Consider using secrets management systems

## Performance Tips

### Resource Usage

The web server is lightweight and efficient:
- Minimal memory footprint
- Fast startup time
- Efficient static asset serving
- Concurrent request handling

### Optimization

For high-traffic deployments:
- Use a reverse proxy for static asset caching
- Implement connection pooling for RSS reader API
- Monitor resource usage with debug mode
- Consider horizontal scaling with load balancers

## Browser Compatibility

The web interface is tested and compatible with:
- **Chrome/Chromium**: Latest versions
- **Firefox**: Latest versions  
- **Safari**: Latest versions (iOS and macOS)
- **Edge**: Latest versions
- **Mobile browsers**: iOS Safari, Chrome Mobile, Firefox Mobile

### JavaScript Requirements

The interface requires JavaScript for:
- Form validation and submission
- Toast notifications
- Loading indicators
- Enhanced user experience

The interface gracefully degrades when JavaScript is disabled, but some features may not be available.