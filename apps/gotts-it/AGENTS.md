# AGENTS.md

## Project overview

CLI tool that extracts readable article text from a URL or local HTML file and synthesizes speech via a self-hosted Speaches TTS server (OpenAI-compatible API). It uses Cobra for CLI commands and Logrus for logs.
GoDotEnv and `caarlos0/env` handle configuration, and go-readability extracts articles.
The app uses openai-go for TTS and GoReleaser for cross-platform builds and Docker images.

- **Module:** `github.com/toozej/monogo/apps/gotts-it`
- **Go version:** 1.27.3
- **License:** GPLv3

## Setup commands

- Install Go toolchain: https://go.dev/dl/
- Install the pre-commit hooks and linting tools: `make pre-commit-install`
- Install the dependencies and vendor them: `go mod tidy && go mod vendor`
- Update deps: `make local-update-deps`
- Install from latest release: `make install`
- Start the Speaches TTS server: `make speaches-up`
- Run server mode: `make server-up`

## Build commands

- Build locally: `make local-build` (outputs binary to `out/`)
- Build via Docker: `make build`
- Build the distroless Docker image: `make distroless-build`
- Run locally built binary: `make local-run` (requires `.env` file)
- Hot-reload on file changes: `make local-iterate` (uses Air)
- Run via Docker: `make run`
- Run via Docker Compose: `make up`

## Test commands

- Run all tests locally with race detection and coverage: `make local-test`
- Run tests in Docker: `make test`
- View coverage report in browser: `make local-cover`
- Run tests for changed packages only: `make test-changed`
- Watch and re-test on changes: `make watch-test`
- Run mutation tests: `make mutation-test` (requires gremlins)
- Run benchmarks: `make benchmark`
- Profile CPU: `make profile-cpu`
- Profile memory: `make profile-mem`

## Lint and format commands

- Run all pre-commit hooks: `make pre-commit` (installs + runs)
- Run pre-commit hooks only: `make pre-commit-run`
- Run go vet locally: `make local-vet` (also runs `go fmt`)
- Run go vet in Docker: `make vet`
- Check Goreleaser config: `goreleaser check`
- Run govulncheck: `govulncheck ./...`
- Run go-licenses report: `go-licenses report github.com/toozej/monogo/apps/gotts-it/cmd/gotts-it`

Pre-commit hooks include: golangci-lint, gosec, staticcheck, go-critic, gofmt, goimports, shellcheck, hadolint (Dockerfiles), checkmake (Makefile), actionlint (GitHub Actions), goreleaser-check, OpenGrep, and private key detection.

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

The project uses these layout conventions for Go:
- `cmd/` — main applications; each subdirectory is a separate binary
- `internal/` — private application code not importable by other packages
- `pkg/` — library code safe to import by external packages

## Code style

- Follow the standard Go format: `gofmt` / `goimports`
- Package-level doc comments on every package in the form `// Package <name> ...`
- Exported functions and types must have godoc comments
- Table-driven tests with `t.Run` for subtests
- Errors use `fmt.Errorf` / `os.Exit(1)` for fatal startup errors
- Configuration via environment variables with `env` struct tags
- Logs via logrus. Use `--debug` or `-d` to toggle debug mode

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

- Conventional commits for changelog generation, such as `feat:`, `fix:`, `docs:`, and `chore:`
- CI runs on all branches via `.github/workflows/ci.yaml`
- CI includes: pre-commit hooks, go tests, gitleaks, CodeQL, Trivy, Snyk
- The release workflow: `.github/workflows/release.yaml` via Goreleaser
- Pre-commit hooks must pass before you commit

## Build and release

- Goreleaser builds binaries for Linux, macOS, Windows across amd64, arm64, 386, arm
- Docker images published to DockerHub, GHCR, and Quay.io
- Distroless variant also available
- Signatures via Cosign (requires `gotts-it.key` and `.env`)
- ldflags inject version info at build time: `Version`, `Commit`, `Branch`, `BuiltAt`, `Builder`
- Manpages and shell completions auto-generated via scripts in `scripts/`

## Boundaries

### Always

- Run `make local-test` before you commit.
- Run `make pre-commit-run` before you commit.
- Write tests for new code.
- Follow the current package layout conventions.
- Add godoc comments to exported symbols.
- Vendor dependencies after module changes.

### Ask first

- Ask before you add dependencies.
- Ask before you modify `.goreleaser.yml`.
- Ask before you change Dockerfiles or CI workflows.
- Ask before you modify release configuration or signature configuration.

### Never

- Do not commit secrets, API keys, private keys, or `.env` files.
- Do not edit `vendor/` files directly. Use `go mod vendor`.
- Do not remove tests that fail without justification.
- Do not modify `go.mod` without a subsequent `go mod tidy` run.
