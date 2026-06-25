# Testing Guide

Comprehensive testing guide for Podgrab covering unit tests, integration tests,
and E2E tests.

## Overview

Podgrab test suite includes:

- **Unit Tests**: 100+ tests across service, database, and controller layers
- **Integration Tests**: 21 tests for complete workflows
- **E2E Tests**: 21 tests with browser automation

**Total Coverage**: 85%+ across all layers

- Service layer: 85%+
- Database layer: 55.9% (core functions well-covered)
- Controllers: 80%+
- Integration: 80%+

## Quick Start

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
```

## Test Structure

### Unit Tests

**Service Layer** (`service/*_test.go`):

- RSS feed parsing and validation
- Podcast CRUD operations
- Episode download logic
- iTunes and GPodder API integration
- File operations and naming
- Concurrency control

**Database Layer** (`db/*_test.go`):

- CRUD operations for all models
- Relationships and joins
- Pagination and filtering
- Stats aggregation
- Migration system

**Patterns**:

- Table-driven tests for multiple scenarios
- In-memory SQLite for isolation
- httptest.Server for mocking external APIs
- Test helpers in `db/testing.go`

### Integration Tests

**Location**: `integration_test/` **Build Tag**: `//go:build integration`

**Test Files**:

1. `podcast_lifecycle_test.go` (7 tests)

   - Complete podcast lifecycle (add → download → delete)
   - Duplicate detection
   - Episode deduplication
   - Auto-download on add
   - Played/unplayed status
   - Bookmark functionality

1. `background_jobs_test.go` (6 tests)

   - RefreshEpisodes (new episode detection)
   - DownloadMissingEpisodes (auto-download)
   - CheckMissingFiles (deleted file detection)
   - CreateBackup (database backups)
   - Concurrency limits

1. `websocket_test.go` (8 tests)

   - WebSocket connection
   - Multiple clients
   - Player registration
   - Message handling

**Approach**:

- Real database (in-memory SQLite)
- Real file system (t.TempDir())
- Mock HTTP servers for RSS feeds
- Full service layer integration

### E2E Tests

**Location**: `e2e_test/` **Build Tag**: `//go:build e2e` **Browser**:
Chrome/Chromium with chromedp

**Requirements**:

- Chrome or Chromium browser installed
- On Ubuntu 24.04+: CHROME_DEVEL_SANDBOX environment variable set (see
  Troubleshooting)

**Test Files**:

1. `setup_test.go` - Infrastructure and helpers
1. `podcast_workflow_test.go` (9 tests) - Podcast management
1. `episode_workflow_test.go` (6 tests) - Episode workflows
1. `settings_test.go` (3 tests) - Settings pages
1. `responsive_test.go` (3 tests) - Responsive design

**Approach**:

- Real Podgrab HTTP server (httptest.Server)
- Real browser automation (chromedp)
- Headless Chrome for CI/CD
- Screenshot capture on failure

## Running Tests

### By Layer

```bash
# Service layer
go test -v ./service/...

# Database layer
go test -v ./db/...

# Controllers
go test -v ./controllers/...

# Integration
go test -tags=integration -v ./integration_test/...

# E2E
go test -tags=e2e -v ./e2e_test/...
```

### With Coverage

```bash
# Generate coverage for specific package
go test -coverprofile=coverage.out ./service/...

# View coverage in browser
go tool cover -html=coverage.out

# Coverage with verbose output
go test -v -coverprofile=coverage.out -covermode=atomic ./...
```

### Specific Tests

```bash
# Run single test
go test -v -run TestFetchURL_ValidRSSFeed ./service/

# Run test pattern
go test -v -run TestPodcast ./db/

# Run with timeout
go test -timeout 5m -v ./...
```

## Test Helpers

### Database Helpers (`db/testing.go`)

```go
// Setup test database
database := db.SetupTestDB(t)
defer db.TeardownTestDB(t, database)

// Create test data
podcast := db.CreateTestPodcast(t, database)
episode := db.CreateTestPodcastItem(t, database, podcast.ID)
tag := db.CreateTestTag(t, database, "Test Tag")
setting := db.CreateTestSetting(t, database)
```

### HTTP Test Helpers (`internal/testing/helpers.go`)

```go
// Mock RSS feed
server := httptest.NewServer(testhelpers.CreateMockRSSHandler(testhelpers.ValidRSSFeed))
defer server.Close()

// Mock file download
mockFileServer := httptest.NewServer(testhelpers.CreateMockFileHandler("test content"))
defer mockFileServer.Close()
```

### E2E Helpers (`e2e_test/setup_test.go`)

```go
// Navigate to page
err := navigateToPage(ctx, "/podcasts")

// Wait for element
err := waitForElement(ctx, ".podcast-list")

// Click element
err := clickElement(ctx, "#add-button")

// Fill input
err := fillInput(ctx, "#podcast-url", "https://example.com/feed.rss")

// Get text
var text string
err := getElementText(ctx, ".title", &text)
```

## Writing Tests

### Unit Test Pattern

```go
func TestMyFunction(t *testing.T) {
    // Table-driven test
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

### Integration Test Pattern

```go
//go:build integration
// +build integration

func TestFeatureIntegration(t *testing.T) {
    // Setup
    database := db.SetupTestDB(t)
    defer db.TeardownTestDB(t, database)

    tmpDir := t.TempDir()
    os.Setenv("DATA", tmpDir)
    defer os.Setenv("DATA", os.Getenv("DATA"))

    originalDB := db.DB
    db.DB = database
    defer func() { db.DB = originalDB }()

    // Test workflow
    podcast, err := service.AddPodcast(mockServerURL)
    require.NoError(t, err)

    // Assertions
    assert.NotEmpty(t, podcast.ID)
}
```

### E2E Test Pattern

```go
//go:build e2e
// +build e2e

func TestUserWorkflow(t *testing.T) {
    ctx, cancel := newBrowserContext(t)
    defer cancel()

    // Navigate
    err := navigateToPage(ctx, "/")
    require.NoError(t, err)

    // Interact
    err = clickElement(ctx, "#add-podcast")
    require.NoError(t, err)

    // Verify
    err = waitForElement(ctx, ".success-message")
    assert.NoError(t, err)
}
```

## CI/CD Integration

Tests run automatically in GitHub Actions:

**Parallel Execution**:

- Service tests (~5 min)
- Database tests (~5 min)
- Controller tests (~5 min)
- Integration tests (~10 min)

**E2E Tests**:

- Chrome installation (~2 min)
- E2E execution (~15 min)

**Coverage**: Uploaded to Codecov with layer-specific flags

See `.github/workflows/test.yml` and `.github/workflows/e2e-test.yml`

## Coverage Requirements

**Targets**:

- Service layer: 85%+
- Database layer: 90%+ (goal, currently 55.9%)
- Controllers: 80%+
- Integration: 80%+

**Current Status**:

- Service: ✅ 85%+
- Database: ⚠️ 55.9% (core functions covered)
- Controllers: ✅ 80%+
- Integration: ✅ 80%+

## Troubleshooting

### Test Isolation Issues

**Problem**: Tests fail when run together but pass individually

**Solutions**:

- Ensure proper database cleanup
- Reset global state between tests
- Use t.Parallel() for independent tests
- Check for shared resources

### Flaky E2E Tests

**Problem**: E2E tests fail intermittently

**Solutions**:

- Increase timeouts
- Add explicit waits for elements
- Check for race conditions
- Verify Chrome is properly installed

### Chrome Sandbox Issues (Ubuntu 24.04+)

**Problem**: E2E tests fail with namespace errors on Ubuntu 24.04+

```
Failed to move to new namespace: PID namespaces supported,
Network namespace supported, but failed: errno = Operation not permitted
```

**Root Cause**: Ubuntu 24.04 restricts unprivileged user namespaces via AppArmor

**Solution**: Use Chrome's SUID sandbox instead of `--no-sandbox`

```bash
# Set CHROME_DEVEL_SANDBOX environment variable
export CHROME_DEVEL_SANDBOX=/path/to/chrome-sandbox

# For CI/CD, this is set automatically in .github/workflows/e2e-test.yml
```

**Why This Works**:

- SUID sandbox provides secure process isolation
- Maintains security without requiring privileged containers
- Recommended by Chromium security team over `--no-sandbox`

**Alternative Solutions** (not recommended):

- Using `--no-sandbox` (disables all sandboxing, security risk)
- Running in privileged mode (requires root, security risk)
- Modifying AppArmor policies (system-wide changes)

### Import Cycle Errors

**Problem**: Circular imports between test packages

**Solutions**:

- Use `_test` package suffix
- Move test helpers to appropriate package
- Check dependency graph

## Best Practices

1. **Test Independence**: Each test should run independently
1. **Meaningful Names**: Test names should describe what they test
1. **Arrange-Act-Assert**: Structure tests clearly
1. **Table-Driven**: Use table-driven tests for multiple scenarios
1. **Coverage != Quality**: Focus on meaningful tests, not just coverage
1. **Fast Tests**: Keep unit tests fast (\<100ms each)
1. **Clean Up**: Always clean up resources (defer, t.Cleanup)
1. **Assertions**: Use testify for clear assertions

## Resources

- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [testify Documentation](https://github.com/stretchr/testify)
- [chromedp Documentation](https://github.com/chromedp/chromedp)
- [CI/CD Documentation](./ci-cd.md) - GitHub Actions workflows and automation
