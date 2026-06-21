# GitHub Actions Version Caching in go-sort-out-gh-actions

## Overview
The `go-sort-out-gh-actions` tool caches data retrieved from the GitHub API to reduce the number of HTTP requests and avoid hitting rate limits. The caching is implemented in the `internal/github` package.

## Cache Storage
The cache consists of four in-memory maps stored as fields of the `github.Client` struct:

1. `archivedCache map[string]bool`  
   - Tracks whether a repository is archived.
2. `releaseCache map[string]*ReleaseInfo`  
   - Stores the latest release (tag) information for a repository.
3. `refSHACache map[string]string`  
   - Maps a reference (tag or branch) to its SHA commit hash.
4. `repoInfoCache map[string]*RepoInfo`  
   - Holds the full repository metadata (used alongside the archived cache).

All maps are protected by a `sync.RWMutex` named `cacheMu` to allow safe concurrent access from multiple goroutines (the GitHub client uses a semaphore‑limited worker pool with `maxConcurrency = 10`).

## Cache Lifetime
- The caches have **no expiration or TTL**.
- An entry remains cached for the lifetime of the `Client` instance.
- Caches are populated **on‑demand**: the first request for a given key (e.g., owner/repo or owner/repo@ref) triggers a GitHub API call, and the result is stored in the appropriate map.
- Subsequent requests for the same key return the cached value without another HTTP request.

## Scope Across Invocations
- The cache is **not persisted** and does **not survive** across different runs of the binary.
- Each command (e.g., `archived`, `outdated`, `check`, `eol`) creates its own `RunContext` (see `internal/checkrunner/runcontext.go`), which instantiates a fresh `github.Client` via `github.NewClient(token)`.
- Therefore, caches are **isolated to a single invocation** of the tool. When the process ends, all in‑memory maps are garbage‑collected.

## Typical Flow
1. A CLI command resolves the GitHub token and loads configuration.
2. `NewRunContext` → `github.NewClient(token)` → a new `Client` with empty maps.
3. As the workflow parser discovers `uses:` references, the client’s methods (`IsRepoArchived`, `GetLatestRelease`, `GetRefSHA`, etc.) are called.
4. Each method first checks its respective map (under a read lock):
   - **Cache hit**: returns the cached value.
   - **Cache miss**: makes the GitHub API call, stores the result in the map (under a write lock), then returns it.
5. When the command finishes, the `Client` (and its caches) are discarded.

## Code References
- **Client struct definition**: `internal/github/types.go`
- **Cache getters/setters**: `internal/github/github.go` (functions like `getCachedArchived`, `setCachedArchived`, `getCachedRelease`, `setCachedRelease`, `getCachedRefSHA`, `setCachedRefSHA`)
- **Client construction**: `internal/github/github.go` (`NewClient` and `NewClientWithHTTP`)
- **Per‑run client creation**: `internal/checkrunner/runcontext.go` (`NewRunContext`)

## Benefits
- Reduces GitHub API call volume, helping to stay within rate limits.
- Improves performance by avoiding repeated requests for the same data during a single run.
- Thread‑safe due to `sync.RWMutex` protection.

## Limitations
- No automatic cache invalidation; if data changes on GitHub during a long-running invocation, the tool may see stale data until the process restarts.
- No cross‑run caching; each invocation starts with a clean cache.
