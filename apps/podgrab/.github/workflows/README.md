# GitHub Workflows Documentation

Enterprise-grade CI/CD workflows for Podgrab with comprehensive testing and
deployment.

## Workflow Architecture

```
build.yml (Orchestrator)
├── code-quality.yml → gofmt, go vet, golangci-lint, gosec, hadolint
├── test.yml → service, db, controllers, integration (parallel)
├── e2e-test.yml → chromedp browser automation
└── Docker build → multi-platform images (main only)
```

## Core Workflows

### `build.yml` - Main Orchestrator

- **Triggers**: Push to main, PRs
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

### `cleanup-cache.yml` - Cache Management

- Cleans PR caches when closed

### `cleanup-images.yml` - Image Management

- Monthly GHCR cleanup
- Keep last 3 tagged versions
- Delete images >30 days old

## Required Secrets

- `DOCKER_USERNAME` - DockerHub username
- `DOCKERHUB_TOKEN` - DockerHub access token
- `GITHUB_TOKEN` - Auto-provided
