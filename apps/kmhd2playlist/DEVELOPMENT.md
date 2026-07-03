# 🏗️ Development

## Prerequisites

- Go 1.25+
- Make
- Docker (optional)

## Setup Development Environment

```bash
# Install development dependencies
make pre-reqs

# Set up pre-commit hooks
make pre-commit-install

# Run the full development workflow
make local
```

## Project Structure

```
├── cmd/kmhd2playlist/     # CLI application entry point
├── internal/
│   ├── api/              # KMHD JSON API integration
│   ├── spotify/          # Spotify API integration  
│   ├── youtubemusic/     # YouTube Music API integration
│   ├── search/           # Fuzzy artist matching
│   └── types/            # Shared data structures
├── pkg/
│   ├── config/           # Configuration management (supports spotify/youtube providers)
│   └── version/          # Version information
└── scripts/              # Build and deployment scripts
```

## Testing

```bash
# Run all tests
make local-test

# Run tests with coverage
make local-cover

# Run mutation tests
make mutation-test

# Watch for changes and test
make watch-test
```

## Development Workflow


```bash
# Set up development environment
make pre-reqs

# Make changes and test continuously
make local-iterate

# Run full test suite before committing
make local-test local-cover pre-commit
```
