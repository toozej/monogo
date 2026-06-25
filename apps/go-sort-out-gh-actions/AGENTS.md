# AGENTS.md

## Project Overview

CLI tool that scans `.github/workflows/` YAML files and detects archived, outdated, stale, and end-of-life (EOL) GitHub Actions. Resolves action references against the GitHub API and optionally auto-updates workflow files, creates GitHub issues, and sends notifications.

**Module:** `github.com/toozej/monogo/apps/go-sort-out-gh-actions`
**Go Version:** 1.26
**Build:** `CGO_ENABLED=0` always

## Commands

### Build & Run

```bash
make build                                      # Build binary with ldflags
make run                                        # Run the built binary
make local-run                                  # Build and run locally
make demo                                       # Run with demo workflows
```

### Test

```bash
make test                                       # go test -race -count=1 ./...
make local-cover                                # Tests with coverage
make test-changed                               # Tests for changed files only
make watch-test                                 # Watch and rerun tests on file changes
make mutation-test                              # Mutation testing
make benchmark                                  # Run benchmarks
```

### Lint & Vet

```bash
make vet                                        # go vet ./...
make all                                        # Run vet + pre-commit + test
make pre-commit-run                             # Run all pre-commit hooks
```

### Docker

```bash
docker compose up                               # Run via docker-compose
```

### Hot Reload

```bash
make local-iterate                              # Hot-reload via air (.air.toml)
```

## Repository Structure

```
main.go                           # Entry point, delegates to cmd.Execute()
cmd/go-sort-out-gh-actions/       # Cobra CLI subcommands
  root.go                         # Root command, persistent flags, resolveToken(), resolveWorkflowFiles()
  archived.go                     # `archived` subcommand
  outdated.go                     # `outdated` subcommand
  eol.go                          # `eol` subcommand
  check.go                        # `check` subcommand (runs all checks)
  version.go                      # `version` subcommand
  man.go                          # `man` subcommand
  completion.go                   # `completion` subcommand
internal/                         # Core business logic (not importable externally)
  github/                         # GitHub API client with caching & rate limit logging
    types.go                      # Client, RepoInfo, ReleaseInfo, RefInfo, StaleInfo
    github.go                     # API methods, goroutine semaphore
    github_test.go                # Table-driven tests with httptest mock server
  workflow/                       # YAML parser for workflow files, finds `uses:` refs
    types.go                      # ActionRef, WorkflowFile, WorkflowParser
    workflow.go                   # FindWorkflowFiles, ParseWorkflowFile, GetAllUsesFromRepoWithVersions
    workflow_test.go
  actioninfo/                     # Utility funcs
    types.go                      # OutdatedActionInfo, StaleActionInfo, RuntimeEOLActionInfo, FileUpdate + constants
    actioninfo.go                 # GetOwnerRepos, ExpandPath, SanitizeStaleDays, WriteActionOutput, Emoji
  checkrunner/                    # Detection logic and orchestration
    types.go                      # CheckResult, RunContext, ProcessFunc
    detect.go                     # DetectArchived, DetectStale, DetectRuntimeEOL, DetectOutdated
    runcontext.go                 # NewRunContext constructor
    notify.go                     # SendArchivedNotifications, CreateArchivedIssues
    display.go                    # WriteResult helper
    reposmode.go                  # RunReposMode for scanning multiple repos
  notification/                   # Multi-provider notifications
    types.go                      # Notifier interface, NotificationManager, GotifyNotifier, NikoksrNotifier
    notification.go               # Implementations wrapping nikoksr/notify
  issue/                          # GitHub issue creation
    types.go                      # IssueCreator, ArchivedActionInfo
    issue.go                      # IssueCreator using go-github
  output/                         # Output formatting
    format.go                     # Format enum (text/json), Writer, CheckOutput
  runtime/                        # EOL runtime detection via endoflife.date API
    types.go                      # RuntimeEOLInfo, EOLClient, ProductRelease
    runtime.go                    # EOLClient fetching
  version/                        # Semver logic
    version.go                    # IsVersionOutdated, IsMajorVersionTag, SameMajorVersion
pkg/                              # Public packages (importable)
  config/                         # Configuration from env vars with path traversal protection
    config.go                     # Config struct with env tags, GetEnvVars, NotificationConfig
  version/                        # Build metadata vars
    version.go                    # Version, Commit, Branch, BuiltAt, Builder, Get(), Command()
  man/                            # Man page generation
    man.go                        # Man page via mango-cobra/roff
 scripts/ # Build and utility scripts
 check-archived/ # Reusable GitHub Action: check for archived actions
 check-archived/action.yml
 check-outdated/ # Reusable GitHub Action: check for outdated actions
 check-outdated/action.yml
 check/ # Reusable GitHub Action: run all checks
 check/action.yml
 eol/ # Reusable GitHub Action: check for EOL runtimes
 eol/action.yml
 go-sort-out-gh-actions/ # Reusable GitHub Action: default archived check (composite)
 go-sort-out-gh-actions/action.yml
 examples/workflows/ # Example workflow YAML files
 examples/pre-commit/ # Example pre-commit config file
```

## Tech Stack

| Component | Technology |
|---|---|
| Language | Go 1.26 (`CGO_ENABLED=0`) |
| CLI | `github.com/spf13/cobra` |
| Logging | `github.com/sirupsen/logrus` (aliased as `log`) |
| GitHub API | `github.com/google/go-github/v85` + `golang.org/x/oauth2` |
| Semver | `github.com/Masterminds/semver/v3` |
| YAML | `gopkg.in/yaml.v3` |
| Config | `github.com/caarlos0/env/v11` + `github.com/joho/godotenv` |
| Notifications | `github.com/nikoksr/notify` (Gotify, Slack, Telegram, Discord, Pushover, Pushbullet) |
| TTY Detection | `golang.org/x/term` |
| Man Pages | `github.com/mango-cobra/roff` |

## Testing

- **Framework:** Go standard `testing` package
- **Style:** Table-driven tests with `t.Parallel()` where appropriate
- **HTTP Mocking:** `net/http/httptest.NewServer` for mocking GitHub API responses
- **Test Colocation:** `*_test.go` files colocated with their source files
- **Run:** `make test` or `go test -race -count=1 ./...`
- **Coverage:** `make local-cover`
- **Changed files only:** `make test-changed`

### Test example (follow this pattern)

```go
func TestClient_GetActionYML(t *testing.T) {
    tests := []struct {
        name        string
        ownerRepo   string
        ref         string
        statusCode  int
        wantUsing   string
        wantError   bool
    }{
        {
            name:       "action.yml found with node20",
            ownerRepo:  "owner/repo",
            ref:        "v2",
            statusCode: 200,
            wantUsing:  "node20",
            wantError:  false,
        },
        {
            name:       "action.yml not found",
            ownerRepo:  "owner/repo",
            ref:        "v1",
            statusCode: 404,
            wantError:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                // handle request
            }))
            defer server.Close()

            client := newTestClient(server)
            // call client method and assert
        })
    }
}
```

## Code Style

### Package doc comments

Every package must have a godoc-style doc comment:

```go
// Package config provides secure configuration management for the go-sort-out-gh-actions application.
//
// This package handles loading configuration from environment variables and .env files
// with built-in security measures to prevent path traversal attacks.
package config
```

### Types in separate files

Types are defined in dedicated `types.go` files within each package. Logic goes in `<name>.go`.

### Error wrapping

Always use `fmt.Errorf` with `%w` for error chaining:

```go
// ✅ Good
return conf, fmt.Errorf("error loading .env file: %w", err)

// ❌ Bad
return conf, fmt.Errorf("error loading .env file: %v", err)
```

### Logging

Use logrus with the `log` alias. Use `log.WithFields()` for structured logging:

```go
log "github.com/sirupsen/logrus"

log.WithFields(log.Fields{
    "repo":  ownerRepo,
    "error": err,
}).Error("Failed to check repo")
```

### Cobra commands

Command constructors return `*cobra.Command`. Bind flags in the constructor:

```go
func newArchivedCmd() *cobra.Command {
    var staleDays int

    cmd := &cobra.Command{
        Use:   "archived",
        Short: "Display archived GitHub Actions",
        Long:  `Scan workflow files and display GitHub Actions that have been archived upstream.`,
        Args:  cobra.NoArgs,
        Run: func(cmd *cobra.Command, args []string) {
            runArchived(staleDays)
        },
    }

    cmd.Flags().IntVar(&staleDays, "stale-days", actioninfo.DefaultStaleDays, "Number of days after which an action is considered stale")

    return cmd
}
```

### Emoji output

Never use raw emoji strings. Always use `actioninfo.Emoji()` which conditionally renders emoji on TTY and text markers otherwise:

```go
// ✅ Good
actioninfo.Emoji("✅ ", "[OK] ") + "No archived actions found!"

// ❌ Bad
"✅ No archived actions found!"
```

### Config via environment variables

Use `caarlos0/env` struct tags for env var mapping:

```go
type Config struct {
    GitHubToken         string `env:"GH_TOKEN"`
    GitHubTokenFallback string `env:"GITHUB_TOKEN"`
    CreateIssues        bool   `env:"CREATE_ISSUES" envDefault:"false"`
}
```

## Architecture

### Data flow

1. CLI command parses flags and env vars via `pkg/config`
2. Workflow files parsed by `internal/workflow` to extract `uses:` action references
3. Each action reference resolved via `internal/github` client (with caching)
4. Detection functions in `internal/checkrunner` classify actions (archived/outdated/stale/EOL)
5. Results formatted by `internal/output` (text or JSON)
6. Optionally: notifications via `internal/notification`, issues via `internal/issue`

### Concurrency

- GitHub API client uses `maxConcurrency=10` goroutines with a semaphore
- Four `sync.RWMutex`-protected caches: `archivedCache`, `releaseCache`, `refSHACache`, `repoInfoCache`
- Rate limit info logged from GitHub API response headers

### Output

- `Format` enum: `text` (default, emoji on TTY) or `json`
- `Writer` interface renders `CheckOutput` in selected format

## Common Tasks

### Add a new subcommand

1. Create `cmd/go-sort-out-gh-actions/<command>.go`
2. Define a constructor function returning `*cobra.Command`
3. Register the command in `root.go` via `rootCmd.AddCommand()`
4. Implement business logic in a new `internal/<package>/` directory
5. Add `types.go` for types, `<name>.go` for logic, `<name>_test.go` for tests

### Add a new notification provider

1. Implement the `Notifier` interface in `internal/notification/`
2. Add environment variable config to `pkg/config/config.go` `NotificationConfig` struct
3. Register the notifier in `internal/notification/notification.go`

### Update Go version

1. Run `make update-golang-version`
2. Run `go mod tidy && make test`

## Git Workflow

- **Branch:** `main` is the primary development branch
- **PRs:** Feature branches merged via pull requests
- **Commits:** Run `make pre-commit-run` before committing to ensure formatting, linting, and tests pass

### CI/CD

| Workflow | Trigger | Purpose |
|---|---|---|
| `ci.yaml` | PR/push to main, weekly Mon 01:18 UTC | Pre-commit + tests + demo; Slack on failure |
| `release.yaml` | Push `apps/<app>/vX.Y.Z` tags | GoReleaser build, cosign signing, SBOM, Docker publish, GitHub release, Homebrew cask |
| `weekly-docker-refresh.yaml` | Weekly Tue 03:18 UTC | Rebuild Docker for base image updates |
| `dependabot-pin-sha.yaml` | Dependabot PRs | Auto-pin action refs to SHA |

### Release

1. Run `make release APP=go-sort-out-gh-actions TYPE=<major|minor|patch>` (bumps and pushes the next `apps/go-sort-out-gh-actions/vX.Y.Z` tag), or tag and push it by hand
2. `release.yaml` is tag-driven (no app matrix) and builds, signs, and publishes only the tagged app
3. Builds for linux/darwin/windows, 386/amd64/arm/arm64
4. Docker images to DockerHub, GHCR, Quay; GitHub release created with `gh`; Homebrew cask published
5. Binaries signed with cosign; SBOM via syft

## Boundaries

- ✅ **Always do:** Run `make vet` and `make test` before committing, follow code style examples, write table-driven tests, wrap errors with `%w`, use `actioninfo.Emoji()` for emoji output, add package doc comments, put types in `types.go`
- ⚠️ **Ask first:** Adding new dependencies (check if already available in `go.mod`), modifying CI/CD workflows, changing the `Notifier` interface, modifying `pkg/config/config.go` struct fields
- 🚫 **Never do:** Commit secrets or API tokens, use `CGO_ENABLED=1`, use raw emoji strings instead of `actioninfo.Emoji()`, skip error wrapping with `%w`, put business logic in `pkg/` (that goes in `internal/`), remove failing tests to make the suite pass
