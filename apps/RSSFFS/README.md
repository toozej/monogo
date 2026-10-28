# RSSFFS

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/monogo)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/rssffs)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/monogo/total)

<img src="img/avatar.png" alt="RSSFFS avatar" style="background-color: #FFFFFF;" />

RSSFFS finds RSS feeds at the URL you enter and subscribes to them in your [Miniflux](https://miniflux.app) instance.

RSSFFS provides both a command-line interface and a web-based interface to discover and subscribe to RSS feeds.

## Usage

### Command Line Interface

RSSFFS operates in two modes:

- **Traversal Mode (default)**: Finds feeds at the input URL and follows page links to discover feeds on other domains
- **Single URL Mode**: Searches for feeds only on the input domain and does not follow links to other domains

Find and subscribe to RSS feeds from a URL:

```bash
# Basic usage (traversal mode - checks all domains found on page)
./RSSFFS https://example.com

# Single URL mode - only check the provided domain
./RSSFFS --single-url https://example.com/blog/post
./RSSFFS -s https://blog.example.com

# With category assignment
./RSSFFS -c "Tech Blogs" https://example.com

# Single URL mode with category
./RSSFFS -s -c "Tech Blogs" https://blog.example.com

# With debug logging
./RSSFFS -d https://example.com

# Clear existing feeds in category before adding new ones
./RSSFFS -r -c "News" https://example.com

# Combine single URL mode with other options
./RSSFFS -s -r -c "News" https://news.example.com
```

### Web Interface

Start the web server for browser-based RSS feed discovery:

```bash
# Start web server on default port (8080)
./RSSFFS serve

# Start on custom host and port
./RSSFFS serve --host 0.0.0.0 --port 3000

# Start with debug logging
./RSSFFS serve -d
```

The web interface will be available at `http://localhost:8080` (or your configured host and port). The web UI provides:

- **Mobile-friendly interface**: Responsive design optimized for mobile devices
- **Simple form**: Enter URL and optional category through an easy-to-use form
- **Real-time feedback**: Toast notifications show success or error messages
- **Input validation**: Client-side and server-side URL validation
- **Progress indicators**: Visual feedback while the application processes RSS feeds

Interactive Swagger API documentation is available at
`http://localhost:8080/swagger/index.html`. It follows the same configuration for HTTP Basic authentication as the web interface.

#### Web Server Configuration

The serve command supports the following options:

- `--host, -H`: Host address to bind the server to (default: 127.0.0.1)
- `--port, -p`: Port number to listen on (default: 8080)
- `--debug, -d`: Enable debug-level log output
- `--config`: Path to configuration file (inherits from root command)

Examples:

```bash
# Bind to all interfaces on port 8080
./RSSFFS serve --host 0.0.0.0

# Use custom port
./RSSFFS serve --port 9000

# Enable debug mode for troubleshooting
./RSSFFS serve --debug

# Combine options
./RSSFFS serve --host 0.0.0.0 --port 3000 --debug
```

To bind the web UI to a non-loopback address, set `WEB_USERNAME` and `WEB_PASSWORD`.
With these values, HTTP Basic authentication protects all UI, API, category, and log routes.
The sample environment file intentionally leaves both values empty.
Set the strong, deployment-specific credentials before you start the development Compose service.
This service binds the application to `0.0.0.0` inside the container and publishes the port only on the host's loopback interface.

For any remote deployment, put the service behind TLS.
HTTP Basic authentication does not encrypt credentials.

## Configuration

RSSFFS requires configuration for your RSS reader API. Create a `config.yaml` file or set environment variables:

### Configuration File (config.yaml)

```yaml
rss_reader_endpoint: "https://your-miniflux-instance.com"
rss_reader_api_key: "your-api-token"

# Optional: Enable single URL mode by default
rssffs_single_url_mode: false
```

### Environment Variables

```bash
export RSS_READER_ENDPOINT="https://your-miniflux-instance.com"
export RSS_READER_API_KEY="your-api-token"

# Optional: Enable single URL mode by default
export RSSFFS_SINGLE_URL_MODE="true"
```

### Configuration Precedence

RSSFFS follows this precedence order for configuration values (highest to lowest priority):

1. **CLI Flags**: Command-line flags always take highest precedence
   - `--single-url` / `-s` flag overrides all other single URL mode settings
2. **Environment Variables**: Environment variables override config file values
   - `RSSFFS_SINGLE_URL_MODE` environment variable
3. **Configuration File**: Values from `config.yaml` (lowest priority)
   - `rssffs_single_url_mode` option

Example precedence scenarios:
```bash
# CLI flag takes precedence over environment variable
export RSSFFS_SINGLE_URL_MODE="false"
./RSSFFS --single-url https://example.com  # Uses single URL mode

# Environment variable takes precedence over config file
# config.yaml: rssffs_single_url_mode: false
export RSSFFS_SINGLE_URL_MODE="true"
./RSSFFS https://example.com  # Uses single URL mode
```

The web interface uses the same configuration as the command-line interface, so you do not need additional setup.

## Development

For development and building from source:

```bash
# Full local development workflow
make local

# Build binary locally
make local-build

# Run with config.yaml
make local-run

# Run tests with coverage
make local-test

# Live reload development
make local-iterate
```
