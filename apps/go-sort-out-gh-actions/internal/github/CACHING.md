# Version Cache for GitHub Actions in go-sort-out-gh-actions

## Overview

The `go-sort-out-gh-actions` tool caches data from the GitHub API to reduce HTTP requests and avoid rate limits.
The `internal/github` package implements the cache.

## Cache Storage

The `github.Client` struct stores four maps in memory:

1. `archivedCache map[string]bool` tracks whether a repository has archived status.
2. `releaseCache map[string]*ReleaseInfo` stores the latest release tag information for a repository.
3. `refSHACache map[string]string` maps a tag or branch reference to its SHA commit hash.
4. `repoInfoCache map[string]*RepoInfo` holds full repository metadata alongside the archived cache.

A `sync.RWMutex` named `cacheMu` protects all maps for safe concurrent access from multiple goroutines.
The GitHub client uses a worker pool with a semaphore and `maxConcurrency = 10`.

## Cache Lifetime

- The caches have **no expiration or TTL**.
- An entry remains cached for the lifetime of the `Client` instance.
- The client populates caches **on demand**.
- Subsequent requests for the same key return the cached value without another HTTP request.

The first request for a key, such as `owner/repo` or `owner/repo@ref`, triggers a GitHub API call.
The client stores its result in the map for that data type.

## Scope Across Invocations

The tool does **not persist** the cache across runs of the binary.
Each command, such as `archived`, `outdated`, `check`, or `eol`, creates its own `RunContext`.
See `internal/checkrunner/runcontext.go`.
The context creates a fresh `github.Client` with `github.NewClient(token)`.
Caches therefore belong to a single invocation.
Garbage collection removes all maps in memory when the process ends.

## Typical Flow

1. A CLI command resolves the GitHub token and loads configuration.
2. `NewRunContext` calls `github.NewClient(token)` to create a `Client` with empty maps.
3. The workflow parser finds `uses:` references and calls client methods such as `IsRepoArchived`, `GetLatestRelease`, and `GetRefSHA`.
4. Each method checks its map under a read lock.
5. A cache hit returns the cached value.
6. A cache miss calls the GitHub API and stores the result under a write lock before it returns the result.
7. The command discards the `Client` and its caches when it finishes.

## Code References

- **Client struct definition**: `internal/github/types.go`
- **Cache getters and setters**: `internal/github/github.go`
- **Client construction**: `internal/github/github.go`, with `NewClient` and `NewClientWithHTTP`
- **Client creation for each run**: `internal/checkrunner/runcontext.go`, with `NewRunContext`

Cache access functions include `getCachedArchived`, `setCachedArchived`, `getCachedRelease`, `setCachedRelease`, `getCachedRefSHA`, and `setCachedRefSHA`.

## Benefits

- Reduces the number of calls to the GitHub API to stay within rate limits.
- Avoids repeated requests for the same data during one run to improve performance.
- Provides thread safety through `sync.RWMutex` protection.

## Limitations

The tool does not automatically invalidate the cache.
If GitHub data changes during a long invocation, the tool possibly sees stale data until the process restarts.
Each invocation starts with a clean cache and does not share it with other runs.
