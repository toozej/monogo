# Contributing to Podgrab

Thank you for your interest in contributing to Podgrab! This guide will help you
get started with development, testing, and submitting contributions.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Code Quality Standards](#code-quality-standards)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Coding Conventions](#coding-conventions)
- [Commit Message Guidelines](#commit-message-guidelines)

## Getting Started

### Prerequisites

- Go 1.24 or higher
- Docker and Docker Compose (optional, for containerized development)
- SQLite (automatically handled by Go)
- Chrome/Chromium (for E2E tests)

### Fork and Clone

1. Fork the repository on GitHub
1. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/podgrab.git
   cd podgrab
   ```
1. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/akhilrex/podgrab.git
   ```

## Development Setup

### Install Dependencies

```bash
# Download Go modules
go mod download

# Verify dependencies
go mod verify
```

### Run Locally

```bash
# Build the application
go build -o ./app ./main.go

# Run with default settings
./app

# Or run directly
go run main.go
```

Application will be available at http://localhost:8080

### Environment Variables

Create a `.env` file in the project root:

```env
# Optional configurations
CONFIG=/path/to/config    # Default: .
DATA=/path/to/data        # Default: ./assets
CHECK_FREQUENCY=30        # Minutes between RSS checks
PASSWORD=mysecret         # Enable basic auth (username: podgrab)
PORT=8080                # Override default port
GIN_MODE=release         # Set to release for production
```

### Development with Docker

```bash
# Build and run with docker-compose
docker-compose up -d

# View logs
docker-compose logs -f

# Rebuild after changes
docker-compose up -d --build
```

## Code Quality Standards

### Pre-commit Checks

Before submitting code, ensure all quality checks pass:

```bash
# Format code
go fmt ./...

# Run linters
golangci-lint run --timeout 5m

# Run security scanner
gosec ./...

# Lint Dockerfile
docker run --rm -i hadolint/hadolint < Dockerfile

# Check dependencies are tidy
go mod tidy
git diff --exit-code go.mod go.sum
```

### Installing Tools

```bash
# Install golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Install gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Install hadolint (macOS)
brew install hadolint
```

### Linter Configuration

See `.golangci.yml` for linter configuration. Key rules:

- **Formatting**: Code must pass `gofmt`
- **Vetting**: Code must pass `go vet`
- **Security**: No high-severity gosec issues
- **Complexity**: Keep cyclomatic complexity under 22
- **Duplication**: Minimize code duplication
- **Style**: Follow idiomatic Go conventions

## Testing Requirements

### Test Coverage Requirements

All PRs must maintain or improve test coverage:

- **Service Layer**: 85%+ coverage
- **Database Layer**: 90%+ coverage (goal)
- **Controllers**: 80%+ coverage
- **Integration**: 80%+ coverage

### Running Tests

```bash
# Run all unit tests
go test -v ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run integration tests
go test -tags=integration -v ./integration_test/...

# Run E2E tests (requires Chrome)
go test -tags=e2e -v ./e2e_test/...

# Run specific test
go test -v -run TestFunctionName ./package/...
```

### Writing Tests

#### Unit Tests

- Use table-driven tests for multiple scenarios
- Mock external dependencies
- Use `testify` for assertions
- Follow the Arrange-Act-Assert pattern

Example:

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "test", "expected", false},
        {"invalid input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)

            if tt.wantErr {
                require.Error(t, err)
                return
            }

            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

#### Integration Tests

- Use `//go:build integration` build tag
- Setup real database (in-memory SQLite)
- Clean up resources with defer
- Test complete workflows

#### E2E Tests

- Use `//go:build e2e` build tag
- Test critical user workflows
- Focus on happy paths
- Add screenshots on failure

See [docs/TESTING.md](docs/TESTING.md) for detailed testing guide.

## Pull Request Process

### Before Submitting

1. **Create a feature branch**:

   ```bash
   git checkout -b feat/my-new-feature
   # or
   git checkout -b fix/bug-description
   ```

1. **Make your changes**:

   - Write code following conventions
   - Add/update tests
   - Update documentation

1. **Commit your changes**:

   ```bash
   git add .
   git commit -m "feat: Add my new feature"
   ```

1. **Push to your fork**:

   ```bash
   git push origin feat/my-new-feature
   ```

1. **Sync with upstream** (if needed):

   ```bash
   git fetch upstream
   git rebase upstream/master
   git push --force-with-lease
   ```

### Submitting PR

1. **Open Pull Request** on GitHub

1. **Fill out PR template**:

   - Description of changes
   - Related issues
   - Testing performed
   - Screenshots (if UI changes)

1. **PR Title Format** (required):

   ```
   <type>: <description>
   ```

   Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert

   Examples:

   - `feat: Add podcast search functionality`
   - `fix: Resolve download concurrency issue`
   - `docs: Update installation instructions`

1. **Ensure CI Passes**:

   - Code quality checks
   - All tests pass
   - Coverage maintained

### Review Process

1. **Automated Checks**: All CI checks must pass
1. **Code Review**: At least one maintainer approval required
1. **Address Feedback**: Make requested changes
1. **Merge**: Maintainer will merge when approved

### PR Size Guidelines

Keep PRs focused and reasonably sized:

- **XS**: ≤10 lines (tiny fixes, typos)
- **S**: ≤100 lines (small features, bug fixes)
- **M**: ≤500 lines (moderate features)
- **L**: ≤1000 lines (large features)
- **XL**: >1000 lines (consider breaking into smaller PRs)

PRs are automatically labeled by size.

## Coding Conventions

### Go Style Guide

Follow the [Effective Go](https://golang.org/doc/effective_go.html) guidelines:

- **Naming**:

  - Use camelCase for variables and functions
  - Use PascalCase for exported types and functions
  - Use descriptive names (avoid abbreviations)

- **Error Handling**:

  - Always check errors
  - Return errors, don't panic
  - Wrap errors with context

- **Comments**:

  - Write comments for exported functions
  - Explain "why" not "what"
  - Use complete sentences

- **Code Organization**:

  - Keep functions small and focused
  - Group related functionality
  - Minimize package dependencies

### Project Structure

```
podgrab/
├── main.go              # Application entry point
├── controllers/         # HTTP handlers
├── service/             # Business logic
├── db/                  # Database models and operations
├── model/               # External data structures
├── client/              # HTML templates
├── integration_test/    # Integration tests
├── e2e_test/           # E2E tests
├── internal/           # Internal packages
│   └── testing/        # Test helpers
└── docs/               # Documentation
```

### File Naming

- Go files: `lowercase_with_underscores.go`
- Test files: `file_test.go`
- Build tags: `//go:build tag` at top of file

### Imports

Group imports in this order:

1. Standard library
1. External packages
1. Internal packages

Example:

```go
import (
    "fmt"
    "time"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"

    "github.com/akhilrex/podgrab/db"
    "github.com/akhilrex/podgrab/service"
)
```

## Commit Message Guidelines

Follow [Conventional Commits](https://www.conventionalcommits.org/):

### Format

```
<type>: <description>

[optional body]

[optional footer]
```

### Types

- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation changes
- **style**: Code style changes (formatting, missing semicolons, etc.)
- **refactor**: Code refactoring
- **perf**: Performance improvements
- **test**: Adding or updating tests
- **build**: Build system or dependencies
- **ci**: CI/CD changes
- **chore**: Other changes that don't modify src or test files
- **revert**: Revert a previous commit

### Examples

```
feat: Add iTunes API integration for podcast search

Implements iTunes search API to allow users to discover podcasts by keyword.
Includes error handling and rate limiting.

Closes #123
```

```
fix: Resolve race condition in download manager

The download counter was not properly synchronized, causing
incorrect concurrent download counts.

Fixes #456
```

```
docs: Update README with new environment variables

Added documentation for newly introduced CHECK_FREQUENCY
and MaxDownloadConcurrency settings.
```

### Best Practices

- Use imperative mood ("Add feature" not "Added feature")
- Don't end subject line with period
- Limit subject line to 50 characters
- Wrap body at 72 characters
- Reference issues and PRs liberally

## Getting Help

- **Documentation**: See [docs/](docs/) directory
- **Issues**: Check existing issues or create new one
- **Discussions**: Use GitHub Discussions for questions

## Code of Conduct

- Be respectful and inclusive
- Welcome newcomers
- Provide constructive feedback
- Focus on what is best for the community

## License

By contributing to Podgrab, you agree that your contributions will be licensed
under the project's license.

## Recognition

Contributors will be recognized in:

- GitHub contributors list
- Release notes
- Special thanks in major releases

Thank you for contributing to Podgrab! 🎉
