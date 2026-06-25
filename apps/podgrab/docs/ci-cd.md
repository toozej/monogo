# CI/CD Documentation

Enterprise-grade CI/CD pipeline for Podgrab with comprehensive testing, quality
gates, and automated deployment.

## Architecture Overview

```
build.yml (Main Orchestrator)
├── code-quality.yml (Pre-flight Gate)
│   ├── gofmt - Code formatting
│   ├── go vet - Static analysis
│   ├── golangci-lint - Comprehensive linting
│   ├── gosec - Security scanning
│   └── hadolint - Dockerfile linting
│
├── test.yml (Parallel Testing) [after quality]
│   ├── test-service - Service layer tests + coverage
│   ├── test-db - Database layer tests + coverage
│   ├── test-controllers - Controller tests + coverage
│   ├── test-integration - Integration tests + coverage
│   └── aggregate-results - Verify all passed
│
├── e2e-test.yml (Browser Automation) [after quality]
│   ├── Install Chrome dependencies
│   ├── Install Google Chrome
│   ├── Run E2E tests (chromedp)
│   └── Upload screenshots on failure
│
└── build-podgrab (Docker Build) [after tests, main only]
    ├── Setup QEMU + Buildx
    ├── Configure build cache
    ├── Build multi-platform images
    └── Push to DockerHub + GHCR
```

## Pipeline Execution Flow

### On Pull Request

1. **Code Quality** (10 min) - Blocks everything if fails
1. **Tests** (15 min) - Run in parallel after quality passes
1. **E2E Tests** (25 min) - Run in parallel after quality passes
1. **Build Summary** - Aggregate status report

**Total**: ~25-30 minutes (tests run in parallel) **Docker Build**: Skipped on
PRs

### On Push to Main

1. **Code Quality** (10 min)
1. **Tests + E2E** (25 min, parallel)
1. **Docker Build** (30 min) - Multi-platform build and push
1. **Build Summary**

**Total**: ~35 minutes **Result**: Docker images pushed to DockerHub and GHCR

## Core Workflows

### build.yml - Main Orchestrator

**Triggers**:

- Push to main
- Pull requests to main

**Jobs**:

1. `code-quality` - Calls code-quality.yml workflow
1. `tests` - Calls test.yml workflow (needs: code-quality)
1. `e2e-tests` - Calls e2e-test.yml workflow (needs: code-quality)
1. `build-podgrab` - Docker build (needs: tests, e2e-tests, if: main)
1. `build-summary` - Status aggregation (needs: all, if: always)

**Features**:

- Workflow composition via `uses`
- Parallel test execution
- Conditional Docker build (main only)
- Build summary in GitHub Actions UI

### code-quality.yml - Pre-flight Gate

**Purpose**: Enforce code quality standards before testing

**Steps**:

1. Checkout code
1. Setup Go with caching (composite action)
1. Run gofmt - Verify code formatting
1. Run go vet - Go static analysis
1. Install golangci-lint v1.63.4
1. Run golangci-lint (5m timeout, GitHub Actions format)
1. Install gosec
1. Run gosec (JSON output, continue on error)
1. Upload gosec results (artifact, 30 days)
1. Lint Dockerfile with hadolint (fail on warnings)
1. Check go.mod/go.sum are tidy

**Configuration**: See `.golangci.yml` for linter configuration

**Failure Impact**: Blocks all downstream jobs

### test.yml - Parallel Test Execution

**Purpose**: Run all unit and integration tests with coverage

**Jobs**:

1. **test-service**: Service layer

   - Run: `go test -v -coverprofile=service-coverage.out ./service/...`
   - Upload to Codecov (flag: service)

1. **test-db**: Database layer

   - Run: `go test -v -coverprofile=db-coverage.out ./db/...`
   - Upload to Codecov (flag: db)

1. **test-controllers**: Controller layer

   - Run: `go test -v -coverprofile=controllers-coverage.out ./controllers/...`
   - Upload to Codecov (flag: controllers)

1. **test-integration**: Integration tests

   - Run:
     `go test -tags=integration -v -coverprofile=integration-coverage.out ./integration_test/...`
   - Upload to Codecov (flag: integration)

1. **aggregate-results**: Verify all tests passed

   - Checks: All jobs succeeded
   - Fails if any test job failed

**Parallelization**: All 4 test jobs run simultaneously

### e2e-test.yml - Browser Automation

**Purpose**: End-to-end testing with Chrome and chromedp

**Steps**:

1. Checkout code
1. Setup Go with caching
1. Install Chrome dependencies (30+ packages)
1. Install Google Chrome stable
1. Verify Chrome installation
1. Run E2E tests: `go test -tags=e2e -v -timeout 10m ./e2e_test/...`
1. Upload test artifacts on failure (screenshots)

**Environment**: Ubuntu with Chrome headless **Tests**: 21 E2E tests covering
critical workflows **Artifacts**: Screenshots saved to /tmp/podgrab-e2e-\*.png

### pr-validation.yml - PR Quality

**Triggers**: PR opened, edited, synchronized, reopened

**Validations**:

1. **Semantic PR Title** (amannn/action-semantic-pull-request@v5)

   - Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore,
     revert
   - Subject pattern: Must start with uppercase
   - Example: `feat: Add podcast search` ✅
   - Example: `add podcast search` ❌

1. **PR Size Labeling** (codelytv/pr-size-labeler@v1)

   - XS: ≤10 lines
   - S: ≤100 lines
   - M: ≤500 lines
   - L: ≤1000 lines
   - XL: >1000 lines (warning message)
   - Ignores: go.sum, go.mod, \*.json, \*.md

### cleanup-cache.yml - Cache Management

**Trigger**: PR closed

**Purpose**: Clean up GitHub Actions cache for merged/closed PRs

**Process**:

1. Install gh-actions-cache extension
1. List all caches for PR branch
1. Delete each cache entry
1. Prevents cache bloat and storage waste

**Permissions**: `actions: write`

### cleanup-images.yml - Image Management

**Trigger**:

- Schedule: Monthly on 15th at midnight UTC
- Manual: workflow_dispatch

**Purpose**: Clean up old Docker images from GHCR

**Strategy** (dataaxiom/ghcr-cleanup-action@v1):

- Delete untagged images
- Delete partial/broken images
- Keep last 3 tagged versions
- Delete images older than 30 days
- Dry-run: false (actually delete)

**Permissions**: `packages: write`

## Composite Actions

### setup-go (`.github/actions/setup-go/action.yml`)

**Purpose**: Reusable Go environment setup with intelligent caching

**Inputs**:

- `go-version`: Go version (default: '1.24')
- `cache-dependency-path`: Path to go.sum (default: 'go.sum')

**Caching Strategy**:

- Level 1: go.mod and go.sum cache (actions/setup-go)
- Level 2: Go modules cache (~go/pkg/mod)
- Level 3: Build cache (~/.cache/go-build)

**Steps**:

1. Set up Go with built-in caching
1. Cache Go modules and build artifacts
1. Download dependencies (`go mod download`)
1. Verify dependencies (`go mod verify`)

**Usage**:

```yaml
- uses: ./.github/actions/setup-go
  with:
    go-version: '1.24'
```

### docker-build-cache (`.github/actions/docker-build-cache/action.yml`)

**Purpose**: Docker BuildKit cache management

**Inputs**:

- `cache-key`: Cache key (required)
- `cache-paths`: Cache paths (default: /tmp/.buildx-cache)

**Outputs**:

- `cache-hit`: Whether cache was restored

**Steps**:

1. Create cache directory
1. Restore cache with restore-keys fallback
1. Report cache hit status

**Usage**:

```yaml
- name: Generate cache key
  id: cache-key
  run: |
    DOCKERFILE_HASH=$(sha256sum Dockerfile | cut -c1-8)
    GOSUM_HASH=$(sha256sum go.sum | cut -c1-8)
    echo "key=${DOCKERFILE_HASH}-${GOSUM_HASH}" >> $GITHUB_OUTPUT

- uses: ./.github/actions/docker-build-cache
  with:
    cache-key: ${{ steps.cache-key.outputs.key }}
```

## Docker Build Process

**Platforms**:

- linux/amd64
- linux/arm64
- linux/arm/v6
- linux/arm/v7

**Build Steps**:

1. Setup QEMU for cross-platform emulation
1. Setup Docker Buildx
1. Generate cache key (Dockerfile + go.sum hashes)
1. Restore BuildKit cache
1. Login to DockerHub and GHCR
1. Extract metadata (tags, labels)
1. Build and push images
1. Move cache (prevents unbounded growth)
1. Save cache for next build

**Tags Generated**:

- `akhilrex/podgrab:latest`
- `akhilrex/podgrab:1.0.0`
- `ghcr.io/akhilrex/podgrab:latest`
- `ghcr.io/akhilrex/podgrab:1.0.0`
- `akhilrex/podgrab:main-<sha>` (branch-sha format)

**Cache Strategy**:

- Cache key: `${OS}-buildx-${Dockerfile hash}-${go.sum hash}`
- Restore keys: `${OS}-buildx-`
- Mode: max (cache all layers)
- Rotation: Old cache replaced with new

## Required Secrets

Configure in repository settings (Settings → Secrets and variables → Actions):

**DockerHub**:

- `DOCKER_USERNAME` - DockerHub username
- `DOCKERHUB_TOKEN` - DockerHub access token (not password)

**GitHub** (auto-provided):

- `GITHUB_TOKEN` - Automatically provided by GitHub Actions
- Permissions: Read contents, write packages

## Configuration Files

### .golangci.yml

Comprehensive linter configuration with 20+ enabled linters:

**Categories**:

- Error detection: errcheck, govet, staticcheck, unused
- Security: gosec
- Performance: ineffassign, bodyclose, prealloc
- Code quality: dupl, gocyclo, gocritic, revive
- Style: misspell, unconvert, unparam, whitespace

**Key Settings**:

- Timeout: 5 minutes
- Tests: Included
- Skip directories: vendor, webassets, client
- Exclude rules: Test files, generated code, legacy exceptions

## Performance Metrics

### Timeline (Parallelized)

```
0:00 - Workflow starts
0:00 - Code quality starts
0:10 - Code quality completes
0:10 - Tests start (4 jobs parallel)
0:10 - E2E tests start
0:25 - Tests complete
0:35 - E2E tests complete
0:35 - Docker build starts (main only)
1:05 - Docker build completes
```

**Total Duration**:

- Pull Request: ~30 minutes (no Docker build)
- Master Push: ~65 minutes (with Docker build)

**Optimization Strategies**:

1. **Parallelization**: Tests run in 4 parallel jobs
1. **Smart Caching**: Go modules, build cache, Docker layers
1. **Early Failure**: Code quality blocks everything
1. **Selective Build**: Docker only on main

### Resource Usage

**Cache Storage**:

- Go modules: ~100-500 MB
- Build cache: ~50-200 MB
- Docker cache: ~500-1000 MB
- **Total**: ~650-1700 MB per branch

**Runner Minutes** (per run):

- Code quality: 10 min × 1 = 10 min
- Tests: 15 min × 4 = 60 min
- E2E: 25 min × 1 = 25 min
- Docker: 30 min × 1 = 30 min (main only)
- **Total**: 95 min (PR), 125 min (main)

## Branch Protection

Recommended settings for `main` branch:

**Required Status Checks**:

- ✅ Code Quality Gate
- ✅ Unit & Integration Tests
- ✅ E2E Tests

**Pull Request Requirements**:

- Require pull request reviews: 1 approval
- Dismiss stale reviews when new commits pushed
- Require review from code owners: Optional
- Require conversation resolution: Enabled

**Additional Settings**:

- Require branches to be up to date: Enabled
- Require status checks to pass: Enabled
- Require deployments to succeed: Disabled
- Lock branch: Disabled
- Do not allow bypassing: Enabled

## Troubleshooting

### Code Quality Failures

**gofmt fails**:

```bash
# Fix locally
go fmt ./...
git add .
git commit -m "fix: Format code"
```

**golangci-lint fails**:

```bash
# Fix automatically where possible
golangci-lint run --fix

# Check specific issues
golangci-lint run --disable-all --enable=errcheck
```

**gosec fails**:

```bash
# Review security issues
gosec ./...

# Suppress false positives (add to .golangci.yml)
```

### Test Failures

**Tests pass locally but fail in CI**:

- Check for race conditions (`go test -race`)
- Verify test isolation
- Check environment dependencies
- Review test logs in GitHub Actions

**Integration tests fail**:

- Check database initialization
- Verify mock HTTP servers
- Review global state management
- Check file system operations

**E2E tests fail**:

- Verify Chrome installation
- Check timeout values
- Review screenshot artifacts
- Test locally: `go test -tags=e2e -v ./e2e_test/...`

### Docker Build Failures

**Multi-platform build fails**:

```bash
# Test locally
docker buildx build --platform linux/amd64 -t test .
```

**Push fails**:

- Verify DockerHub credentials
- Check GHCR permissions
- Review registry authentication logs

**Cache issues**:

- Clear cache and rebuild
- Check cache key generation
- Verify cache path permissions

### Cache Issues

**Cache not restoring**:

- Check cache key format
- Verify restore-keys fallback
- Review cache size limits (10 GB per repository)
- Check cache age (7 days for unused caches)

**Cache too large**:

- Review what's being cached
- Implement cache rotation
- Use cache-to with mode=max selectively

## Monitoring and Observability

### GitHub Actions UI

**Workflow Runs**:

- Actions tab → All workflows
- Filter by workflow, branch, event
- View run duration, status, logs

**Artifacts**:

- Downloadable from workflow run
- gosec results (JSON)
- E2E screenshots (on failure)
- Retention: 7-30 days

**Caches**:

- Actions tab → Caches
- View size, age, last used
- Manual deletion available

### Codecov Integration

**Coverage Reports**:

- Uploaded per test job with flags
- Flags: service, db, controllers, integration
- View trends over time
- PR comments with coverage diff

**Configuration**: See codecov.yml (if created)

## Migration from hub.yml

**Old Workflow**: Single workflow with Docker build only

**New Workflows**: Multi-workflow pipeline with testing

**Migration Status**:

- ✅ New workflows created alongside hub.yml
- ⏳ Testing on feature branches
- ⏳ Update branch protection rules
- ⏳ Archive hub.yml after validation
- ⏳ Monitor first production runs

**Rollback Plan**: hub.yml remains active during migration

## Future Enhancements

Potential improvements:

- Release workflow with semantic versioning and changelogs
- Performance benchmarking job with historical comparison
- Mutation testing for test quality validation
- SBOM generation and provenance
- Slack/Discord notifications for failures
- Automated dependency updates (Dependabot/Renovate)
- Blue-green deployment strategy
- Automated rollback on deployment failure

## Resources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Docker Buildx Documentation](https://docs.docker.com/buildx/working-with-buildx/)
- [golangci-lint Documentation](https://golangci-lint.run/)
- [Codecov Documentation](https://docs.codecov.com/)
- [Workflows README](./../.github/workflows/README.md)
