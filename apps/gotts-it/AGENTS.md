# AGENTS.md

## Project overview

CLI tool that extracts readable article text from a URL or local HTML file and synthesizes speech via a self-hosted Speaches TTS server (OpenAI-compatible API). Uses Cobra for CLI handling, Logrus for logging, GoDotEnv and caarlos0/env for configuration, go-readability for article extraction, openai-go for TTS, and Goreleaser for cross-platform builds and Docker images.

- **Module:** `github.com/toozej/monogo/apps/gotts-it`
- **Go version:** 1.26
- **License:** GPLv3

## Setup commands

- Install Go toolchain: https://go.dev/dl/
- Install pre-commit hooks and linting tools: `task pre-commit:install`
- Install deps and vendor: `go mod tidy && go mod vendor`
- Update deps: `task deps:update`
- Install from latest release: `task install`
- Start a Speaches TTS server separately and configure `OPENAI_BASE_URL`
- Run server mode: `gotts-it server`

## Build commands

- Build locally: `task local:build` (outputs binary to `out/`)
- Build via Docker: `task build`
- Build distroless Docker image: `task docker:distroless-build`
- Run locally built binary: `task local:run` (requires `.env` file)
- Hot-reload on file changes: `task local:iterate` (uses Air)
- Run via Docker: `task docker:run`
- Run via Docker Compose: `task docker:up`

## Test commands

- Run all tests locally with race detection and coverage: `task local:test`
- Run tests in Docker: `task test`
- View coverage report in browser: `task local:cover`
- Run tests for changed packages only: `task test:changed`
- Watch and re-test on changes: `task test:watch`
- Run mutation tests: `task test:mutation` (requires gremlins)
- Run benchmarks: `task benchmark`
- Profile CPU: `task profile:cpu`
- Profile memory: `task profile:memory`

## Lint and format commands

- Run all pre-commit hooks: `task pre-commit` (installs + runs)
- Run pre-commit hooks only: `task pre-commit:run`
- Run go vet locally: `task local:vet` (also runs `go fmt`)
- Run go vet in Docker: `task vet`
- Check Goreleaser config: `goreleaser check`
- Run govulncheck: `govulncheck ./...`
- Run go-licenses report: `go-licenses report github.com/toozej/monogo/apps/gotts-it/cmd/gotts-it`

Pre-commit hooks include: golangci-lint, gosec, staticcheck, go-critic, gofmt, goimports, shellcheck, hadolint (Dockerfiles), actionlint (GitHub Actions), goreleaser-check, semgrep, and private key detection.

## Project structure

```
main.go # Entry point, delegates to cmd.Execute()
cmd/
  gotts-it/
    root.go # Root cobra command, flags, subcommands
    root_test.go # Root command tests
    server.go # Server mode entrypoint and flags
  diagrams/
    main.go # Architecture diagram generation
internal/
  article/
    article.go # Article text extraction from URL/file via go-readability
    article_test.go
  tts/
    tts.go # TTS synthesis via openai-go with sentence-boundary chunking
    tts_test.go
    gtranslate.go # Google Translate API implementation for speech engine
    gtranslate_test.go
  slug/
    slug.go # Slug generation for default output file names
    slug_test.go
pkg/
  config/
    config.go # Config loading from env vars + .env with path traversal protection
    config_test.go
  version/
    version.go # Build version info (injected via ldflags) + version cobra command
    version_test.go
  man/
    man.go # Man page generation cobra command
    man_test.go
scripts/ # Build, release, and setup scripts
docs/
  diagrams/ # Generated architecture diagrams
vendor/ # Vendored dependencies
```

Go project layout conventions used:
- `cmd/` — main applications; each subdirectory is a separate binary
- `internal/` — private application code not importable by other packages
- `pkg/` — library code safe to import by external packages

## Code style

- Follow standard Go formatting: `gofmt` / `goimports`
- Package-level doc comments on every package matching `// Package <name> ...`
- Exported functions and types must have godoc comments
- Table-driven tests using `t.Run` for subtests
- Errors use `fmt.Errorf` / `os.Exit(1)` for fatal startup errors
- Configuration via environment variables with `env` struct tags
- Logging via logrus; debug mode toggled with `--debug` / `-d` flag

**Good test example:**

```go
func TestRun(t *testing.T) {
  tests := []struct {
    name string
    input string
    expected string
  }{
    {"valid username", "Alice", "Hello from Alice\n"},
    {"empty username", "", "Hello from \n"},
  }
  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      // test logic
    })
  }
}
```

**Good function example:**

```go
// Run executes the main functionality by printing a greeting.
func Run(username string) {
  fmt.Println("Hello from", username)
}
```

## Git workflow

- Conventional commits for changelog generation: `feat:`, `fix:`, `docs:`, `chore:`, etc.
- CI runs on all branches via `.github/workflows/ci.yaml`
- CI includes: pre-commit hooks, go tests, gitleaks, CodeQL, Trivy, Snyk
- Release workflow: `.github/workflows/release.yaml` via Goreleaser
- Pre-commit hooks must pass before committing

## Build and release

- Goreleaser builds binaries for Linux, macOS, Windows across amd64, arm64, 386, arm
- Docker images published to DockerHub, GHCR, and Quay.io
- Distroless variant also available
- Signing via Cosign (requires `gotts-it.key` and `.env`)
- ldflags inject version info at build time: `Version`, `Commit`, `Branch`, `BuiltAt`, `Builder`
- Manpages and shell completions auto-generated via scripts in `scripts/`

## Boundaries

- ✅ **Always:** Run `task local:test` and `task pre-commit:run` before committing, write tests for new code, follow existing package layout conventions, add godoc comments to exported symbols, vendor dependencies after module changes
- ⚠️ **Ask first:** Adding new dependencies, modifying `.goreleaser.yml`, changing Dockerfile or CI workflows, modifying release or signing configuration
- 🚫 **Never:** Commit secrets, API keys, or private keys, edit files in `vendor/` directly (use `go mod vendor`), commit `.env` files, remove failing tests without justification, modify `go.mod` without running `go mod tidy`
