# Integration Tests

This directory contains integration tests for Podgrab that test full workflows
with real database and file system interactions.

## Running Integration Tests

Integration tests are tagged with `integration` build tag to separate them from
unit tests:

```bash
# Run all integration tests
go test -tags=integration -v ./integration_test/...

# Run specific test file
go test -tags=integration -v ./integration_test/podcast_lifecycle_test.go

# Run with coverage
go test -tags=integration -coverprofile=integration_cover.out ./integration_test/...
```

## Test Files

### podcast_lifecycle_test.go

Tests complete podcast lifecycle workflows:

- Adding podcasts from RSS feeds
- Downloading episodes
- Deleting podcasts
- Duplicate detection
- Episode deduplication
- Auto-download on add
- Played/unplayed status
- Bookmark status

### background_jobs_test.go

Tests background job functionality:

- RefreshEpisodes (detecting new episodes)
- DownloadMissingEpisodes (auto-download queue)
- CheckMissingFiles (detecting deleted files)
- CreateBackup (database backups)
- Concurrency limits

### websocket_test.go

Tests WebSocket real-time communication:

- Connection establishment
- Multiple concurrent clients
- Player registration protocol
- Enqueue message handling
- Connection persistence
- Clean disconnection
- Reconnection after server restart

## Test Approach

Integration tests use:

- Real SQLite in-memory databases for isolation
- Real file system with t.TempDir() for cleanup
- Mock HTTP servers for external feeds
- Full service layer integration
- WebSocket protocol testing

## Known Issues

Integration tests are currently in development and may have compilation errors.
They serve as documentation of intended integration test coverage for Phase 4.
