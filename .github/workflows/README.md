# GitHub Workflows Documentation

Enterprise-grade CI/CD workflows for Podgrab with comprehensive testing and
deployment.

## Workflow Architecture

```
build.yml (Orchestrator)
├── code-quality.yml → gofmt, go vet, golangci-lint, gosec, hadolint
├── test.yml → service, db, controllers, integration (parallel)
├── e2e-test.yml → chromedp browser automation
└── Docker build → multi-platform images (master only)
```

## Core Workflows

### `build.yml` - Main Orchestrator

- **Triggers**: Push to master, PRs
- **Flow**: Quality checks → Tests (parallel) → E2E → Docker build
- **Duration**: ~35 min (parallelized)

### `code-quality.yml` - Pre-flight Gate

- Go formatting, vetting, linting
- Security scanning (gosec)
- Dockerfile linting (hadolint)
- **Duration**: ~10 min

### `test.yml` - Parallel Testing

- Service layer tests
- Database layer tests
- Controller tests
- Integration tests
- Coverage uploaded to Codecov
- **Duration**: ~15 min

### `e2e-test.yml` - Browser Testing

- Chromedp with Chrome headless
- 21 E2E tests
- Screenshot capture on failure
- **Duration**: ~25 min

### `pr-validation.yml` - PR Quality

- Semantic PR title validation
- Auto-size labeling (XS/S/M/L/XL)

### `cleanup-cache.yml` - Cache Management

- Cleans PR caches when closed

### `cleanup-images.yml` - Image Management

- Monthly GHCR cleanup
- Keep last 3 tagged versions
- Delete images >30 days old

## Composite Actions

### `setup-go` - Go Environment

- Go installation + multi-level caching
- Dependency download and verification

### `docker-build-cache` - Docker Cache

- BuildKit cache management
- Intelligent cache keys (Dockerfile + go.sum)

## Required Secrets

- `DOCKER_USERNAME` - DockerHub username
- `DOCKERHUB_TOKEN` - DockerHub access token
- `GITHUB_TOKEN` - Auto-provided

## Running Locally

```bash
# Code quality
go fmt ./...
go vet ./...
golangci-lint run --timeout 5m
gosec ./...

# Tests
go test -v ./service/...
go test -v ./db/...
go test -tags=integration -v ./integration_test/...
go test -tags=e2e -v ./e2e_test/...

# Docker
docker build -t podgrab:local .
```

## Performance

- **Code Quality**: ~10 min
- **Tests (parallel)**: ~15 min
- **E2E Tests**: ~25 min
- **Docker Build**: ~30 min
- **Total**: ~35 min (with parallelization)

## Branch Protection

Recommended requirements for `master`:

- Code Quality Gate (required)
- Unit & Integration Tests (required)
- E2E Tests (required)
- 1 PR review approval
- Up-to-date branches

## Migration Status

- ✅ New workflows created
- ⏳ Testing on feature branches
- ⏳ Update branch protection
- ⏳ Archive old hub.yml
- ⏳ Monitor production runs

## Documentation

See individual workflow files for detailed documentation and configuration
options.
