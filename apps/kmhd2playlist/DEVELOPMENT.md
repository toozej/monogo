# 🏗️ Development

## Prerequisites

- Go 1.25+
- Task 3.52+
- Docker (optional)

## Setup Development Environment

```bash
# Install development dependencies
task prereqs

# Set up pre-commit hooks
task pre-commit:install

# Run the full development workflow
task local
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
task local:test

# Run tests with coverage
task local:cover

# Run mutation tests
task test:mutation

# Watch for changes and test
task test:watch
```

## Development Workflow


```bash
# Set up development environment
task prereqs

# Make changes and test continuously
task local:iterate

# Run full test suite before committing
task local:test local:cover pre-commit
```
