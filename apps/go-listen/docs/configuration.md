# Configuration Guide

This document describes how to configure the go-listen application for deployment and operation.

## Configuration Methods

The application supports configuration through:
1. Environment variables (recommended for production)
2. `.env` file (recommended for development)
3. Command-line flags (limited options)

## Environment Variables

### Required Configuration

#### Spotify API Configuration
These are required for the application to function:

```bash
# Spotify API credentials (required)
SPOTIFY_CLIENT_ID=your_spotify_client_id_here
SPOTIFY_CLIENT_SECRET=your_spotify_client_secret_here
SPOTIFY_REDIRECT_URL=http://localhost:8080/callback
```

**How to get Spotify credentials:**
1. Go to [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
2. Create a new app
3. Copy the Client ID and Client Secret
4. Add `http://localhost:8080/callback` to the Redirect URIs
   - See [Spotify redirect URL documentation](https://developer.spotify.com/documentation/web-api/concepts/redirect_uri) for more info

### Optional Configuration

#### Server Configuration
```bash
# Server settings (optional, defaults shown)
SERVER_HOST=127.0.0.1             # Server bind address
SERVER_PORT=8080                  # Server port
SERVER_READ_TIMEOUT_SECONDS=30    # HTTP read timeout in seconds
SERVER_WRITE_TIMEOUT_SECONDS=60   # HTTP write timeout in seconds
SERVER_IDLE_TIMEOUT_SECONDS=120   # HTTP idle timeout in seconds
```

**Server Timeout Configuration Details:**

- `SERVER_READ_TIMEOUT_SECONDS`: Maximum time to read the entire request, including body
  - Increase for large file uploads or slow clients
  - Default: 30 seconds

- `SERVER_WRITE_TIMEOUT_SECONDS`: Maximum time to write the response
  - **Important**: Increase this for long-running operations like web scraping
  - Should be longer than your longest expected operation
  - Default: 60 seconds (increased from 15s to handle scraping operations)

- `SERVER_IDLE_TIMEOUT_SECONDS`: Maximum time to wait for the next request when keep-alives are enabled
  - Controls connection reuse efficiency
  - Default: 120 seconds

#### Scraper Configuration
```bash
# Web scraping settings (optional, defaults shown)
SCRAPER_TIMEOUT=30s                    # HTTP timeout for web requests
SCRAPER_MAX_RETRIES=3                  # Maximum retry attempts for failed requests
SCRAPER_RETRY_BACKOFF=2s               # Initial backoff delay for retries (exponential)
SCRAPER_USER_AGENT=go-listen/1.0       # User agent string for web requests
SCRAPER_MAX_CONTENT_SIZE=10485760      # Maximum content size in bytes (10MB)
```

**Scraper Configuration Details:**

- `SCRAPER_TIMEOUT`: How long to wait for a web page to respond before timing out
  - Increase for slow websites
  - Decrease to fail faster on unresponsive sites
  - Default: 30 seconds

- `SCRAPER_MAX_RETRIES`: Number of retry attempts for network errors
  - Uses exponential backoff (2s, 4s, 8s with default settings)
  - Set to 0 to disable retries
  - Default: 3 retries

- `SCRAPER_RETRY_BACKOFF`: Initial delay before first retry
  - Subsequent retries use exponential backoff (delay × 2^attempt)
  - Example: 2s → 4s → 8s
  - Default: 2 seconds

- `SCRAPER_USER_AGENT`: User agent string sent with HTTP requests
  - Some websites may block requests without a proper user agent
  - Customize if needed for specific websites
  - Default: "go-listen/1.0 (Web Scraper)"

- `SCRAPER_MAX_CONTENT_SIZE`: Maximum size of web page content to download
  - Prevents memory issues with very large pages
  - Pages exceeding this size will be rejected
  - Default: 10MB (10485760 bytes)

#### Security Configuration
```bash
# Rate limiting (optional, defaults shown)
SECURITY_RATE_LIMIT_REQUESTS_PER_SECOND=10  # Requests per second per IP
SECURITY_RATE_LIMIT_BURST=20                # Burst capacity per IP
```

#### Logging Configuration
```bash
# Logging settings (optional, defaults shown)
LOGGING_LEVEL=info            # Log level: debug, info, warn, error
LOGGING_FORMAT=json           # Log format: json, text
LOGGING_OUTPUT=stdout         # Log output: stdout, stderr, file path
LOGGING_ENABLE_HTTP=true      # Enable HTTP request logging
```


## Troubleshooting

### Common Issues

1. **"spotify client ID and secret are required"**
   - Set `SPOTIFY_CLIENT_ID` and `SPOTIFY_CLIENT_SECRET` environment variables
   - Verify credentials are correct in Spotify Developer Dashboard

2. **"Server failed to start: bind: address already in use"**
   - Change `SERVER_PORT` to an available port
   - Check if another service is using the port: `lsof -i :8080`

3. **Rate limiting too aggressive**
   - Increase `SECURITY_RATE_LIMIT_REQUESTS_PER_SECOND`
   - Increase `SECURITY_RATE_LIMIT_BURST` for burst capacity

4. **Too much/little logging**
   - Adjust `LOGGING_LEVEL` (debug, info, warn, error)
   - Set `LOGGING_ENABLE_HTTP=false` to disable HTTP request logging

5. **Web scraping timeouts**
   - Increase `SCRAPER_TIMEOUT` for slow websites
   - Check network connectivity to target website
   - Verify the URL is accessible from your server

6. **No artists found when scraping**
   - Try without a CSS selector first (scrape entire page)
   - Use browser developer tools to find the correct CSS selector
   - Check if the page requires JavaScript (not supported)
   - Verify the page actually contains artist names

7. **Scraping fails with "connection refused"**
   - Verify the URL is correct and accessible
   - Check if the website blocks automated requests
   - Try adjusting `SCRAPER_USER_AGENT` to a browser user agent
   - Some websites may require authentication or cookies

8. **Too many retry attempts**
   - Reduce `SCRAPER_MAX_RETRIES` to fail faster
   - Increase `SCRAPER_RETRY_BACKOFF` to wait longer between retries
   - Check if the target website is experiencing issues

9. **"Failed to encode JSON response" with I/O timeout**
   - This occurs when scraping operations take longer than the server write timeout
   - Increase `SERVER_WRITE_TIMEOUT_SECONDS` to be longer than your scraping operations
   - For long scraping operations (>30 seconds), set to 120 or higher
   - Example: `SERVER_WRITE_TIMEOUT_SECONDS=120` for operations up to 2 minutes

### Debug Mode

Enable debug logging for troubleshooting:

```bash
# Via environment variable
LOGGING_LEVEL=debug ./go-listen serve

# Via command line flag
./go-listen serve --debug
```

## Security Considerations

### Production Security

1. **Never commit credentials**: Keep `.env` files out of version control
2. **Use environment variables**: Set credentials via environment in production
3. **Restrict network access**: Bind to specific interfaces if needed
4. **Monitor rate limits**: Adjust based on expected usage patterns
5. **Enable HTTPS**: Use a reverse proxy (nginx, Caddy) for TLS termination

### Network Security

The application includes built-in security features:
- CSRF protection for state-changing operations
- Rate limiting per IP address
- Input validation and sanitization
- Security headers (HSTS, CSP, etc.)

For production deployment, consider:
- Running behind a reverse proxy
- Using TLS/HTTPS
- Implementing additional authentication if needed
- Monitoring and alerting on security events

## Web Scraping Configuration Examples

### Fast, Aggressive Scraping

For reliable, fast websites:

```bash
SCRAPER_TIMEOUT=10s
SCRAPER_MAX_RETRIES=1
SCRAPER_RETRY_BACKOFF=1s
```

### Slow, Unreliable Websites

For slow or unreliable websites:

```bash
SCRAPER_TIMEOUT=60s
SCRAPER_MAX_RETRIES=5
SCRAPER_RETRY_BACKOFF=5s
```

### Conservative (Default)

Balanced settings for most use cases:

```bash
SCRAPER_TIMEOUT=30s
SCRAPER_MAX_RETRIES=3
SCRAPER_RETRY_BACKOFF=2s
```

### Custom User Agent

Some websites may require a browser-like user agent:

```bash
SCRAPER_USER_AGENT="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
```

### Large Content Pages

For websites with very large pages:

```bash
SCRAPER_MAX_CONTENT_SIZE=52428800  # 50MB
```

## Performance Tuning

### Rate Limiting

Adjust based on your usage patterns:

```bash
# For high-traffic scenarios
SECURITY_RATE_LIMIT_REQUESTS_PER_SECOND=50
SECURITY_RATE_LIMIT_BURST=100

# For low-traffic scenarios
SECURITY_RATE_LIMIT_REQUESTS_PER_SECOND=5
SECURITY_RATE_LIMIT_BURST=10
```

### Logging

For high-traffic production:

```bash
# Reduce logging overhead
LOGGING_LEVEL=warn
LOGGING_ENABLE_HTTP=false
```

For development and debugging:

```bash
# Verbose logging
LOGGING_LEVEL=debug
LOGGING_ENABLE_HTTP=true
```