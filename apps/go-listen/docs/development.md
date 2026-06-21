# Development

## Using the Makefile (Recommended)

The Makefile provides the primary method for developing and installing the application locally. It includes all necessary tools and checks:

```bash
# Install all development dependencies and run the full workflow
make local

# Individual commands:
make local-build          # Build the binary locally
make local-test           # Run all tests with coverage
make local-run            # Run the built binary with .env file
make pre-commit           # Run all code quality checks
make local-cover          # View test coverage in browser

# Development iteration (rebuilds and restarts on file changes)
make local-iterate

# Clean up
make clean
```

## Manual Building (Alternative)

```bash
# Build binary
go build -o go-listen .

# Run tests
go test ./...

# Run with development settings
cp configs/development.env .env
./go-listen serve --debug
```

## Project Structure

```
├── cmd/go-listen/          # CLI commands
│   ├── root.go            # Root command
│   ├── serve.go           # Web server command
│   └── scrape.go          # Web scraping command
├── internal/
│   ├── middleware/         # HTTP middleware (security, logging, rate limiting)
│   ├── server/            # HTTP server and handlers
│   ├── services/          # Business logic services
│   │   ├── duplicate/     # Duplicate detection
│   │   ├── playlist/      # Playlist management
│   │   ├── scraper/       # Web scraping and artist extraction
│   │   ├── search/        # Fuzzy artist search
│   │   └── spotify/       # Spotify API integration
│   └── types/             # Type definitions and interfaces
├── pkg/
│   ├── config/            # Configuration management
│   └── logging/           # Structured logging
├── docs/                  # Documentation
├── configs/               # Example configurations
└── internal/server/static/ # Web interface assets
```

## Testing

### Running Tests

```bash
# Run all tests
make local-test

# Run tests with coverage
go test -cover ./...

# Run tests for specific package
go test ./internal/services/scraper/...

# Run with race detection
go test -race ./...

# View coverage in browser
make local-cover
```

### Testing Web Scraping

The scraper service includes comprehensive tests:

**Unit Tests:**
- HTML parsing with various structures
- CSS selector validation and extraction
- Artist name extraction strategies
- Error handling and retry logic

**Property-Based Tests:**
- HTML parsing robustness across random inputs
- CSS selector extraction correctness
- Artist deduplication
- Fuzzy matching integration
- Batch processing fault tolerance

**Integration Tests:**
- End-to-end scraping workflow
- Reddit example URL validation
- CLI command execution

Run scraper-specific tests:
```bash
go test ./internal/services/scraper/... -v
go test ./cmd/go-listen/scrape_test.go -v
```

### Manual Testing

Test the scraper with a real URL:

```bash
# Build and run
make local-build

# Test scraping (requires valid Spotify credentials)
./go-listen scrape https://example.com/artists \
  --playlist YOUR_PLAYLIST_ID \
  --selector "div.content"
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass: `make local-test`
6. Run code quality checks: `make pre-commit`
7. Submit a pull request

## Update Golang Version

```bash
make update-golang-version
```