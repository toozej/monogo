# Integration Tests

This directory contains integration tests for Podgrab that test full workflows
with real database and file system interactions.

## Run Integration Tests

Integration tests use the `integration` build tag to separate them from unit tests:

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

- Add podcasts from RSS feeds
- Download episodes
- Delete podcasts
- Duplicate detection
- Episode deduplication
- Auto-download on add
- Played or unplayed status
- Bookmark status

### background_jobs_test.go

Tests background job functionality:

- RefreshEpisodes (detect new episodes)
- DownloadMissingEpisodes (auto-download queue)
- CheckMissingFiles (detect deleted files)
- CreateBackup (database backups)
- Concurrency limits

### websocket_test.go

Tests WebSocket real-time communication:

- Connection establishment
- Multiple concurrent clients
- Player registration protocol
- Enqueue message management
- Connection persistence
- Orderly disconnection
- Reconnection after server restart

## Test Approach

Integration tests use:

- Real SQLite in-memory databases for isolation
- Real file system with t.TempDir() for cleanup
- Mock HTTP servers for external feeds
- Full integration of the service layer
- WebSocket protocol tests

## Known Issues

Integration tests are currently in development and possibly have compilation errors.
They serve as documentation of intended integration test coverage for Phase 4.
