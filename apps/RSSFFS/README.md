# RSSFFS

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/RSSFFS)
[![Go Report Card](https://goreportcard.com/badge/github.com/toozej/RSSFFS)](https://goreportcard.com/report/github.com/toozej/RSSFFS)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/RSSFFS/cicd.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/RSSFFS)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/RSSFFS/total)

RSS Feed Finder [and] Subscriber finds RSS feeds on the user-inputted URL, and subscribes to them in your [Miniflux RSS feed reader](https://miniflux.app) instance.

RSSFFS provides both a command-line interface and a web-based interface for discovering and subscribing to RSS feeds.

## Usage

### Command Line Interface

RSSFFS operates in two modes:

- **Traversal Mode (default)**: Discovers RSS feeds on the provided URL and follows links to find feeds on other domains mentioned on the page
- **Single URL Mode**: Only searches for RSS feeds on the specific domain of the provided URL, without following links to other domains

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

The web interface will be available at `http://localhost:8080` (or your configured host/port). The web UI provides:

- **Mobile-friendly interface**: Responsive design optimized for mobile devices
- **Simple form**: Enter URL and optional category through an easy-to-use form
- **Real-time feedback**: Toast notifications show success/error messages
- **Input validation**: Client-side and server-side URL validation
- **Loading indicators**: Visual feedback during RSS feed processing

#### Web Server Configuration

The serve command supports the following options:

- `--host, -H`: Host address to bind the server to (default: 127.0.0.1)
- `--port, -p`: Port number to listen on (default: 8080)
- `--debug, -d`: Enable debug-level logging
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
   - `rssffs_single_url_mode` setting

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

The web interface uses the same configuration as the command-line interface, so no additional setup is required.

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
