# Scripts Directory

This directory contains utility scripts for maintaining and managing the Podgrab
project.

## Available Scripts

### `update-project.sh`

Comprehensive project update script that handles:

- Go module dependencies (go.mod/go.sum)
- GitHub Actions workflow versions
- Docker configuration (Dockerfile, docker-compose.yml)
- Go version directives across all files

#### Usage

```bash
# Update everything with minor version updates (default, safest)
./scripts/update-project.sh

# Dry run to see what would be updated
./scripts/update-project.sh --dry-run

# Update only Go dependencies
./scripts/update-project.sh --go-only

# Update only GitHub Actions workflows
./scripts/update-project.sh --workflows-only

# Update only Docker files
./scripts/update-project.sh --docker-only

# Update to latest patch versions only (most conservative)
./scripts/update-project.sh --patch

# Update to latest major versions (may introduce breaking changes)
./scripts/update-project.sh --major

# Skip backup creation
./scripts/update-project.sh --no-backup
```

#### Update Levels

- **`--patch`**: Updates to latest patch versions (e.g., 1.2.3 → 1.2.4)

  - Safest option, only bug fixes
  - Recommended for production

- **`--minor`**: Updates to latest minor versions (e.g., 1.2.3 → 1.3.0)
  **[DEFAULT]**

  - Includes new features but maintains backward compatibility
  - Balanced approach

- **`--major`**: Updates to latest major versions (e.g., 1.2.3 → 2.0.0)

  - May include breaking changes
  - Requires thorough testing
  - Use with caution

#### What Gets Updated

**Go Dependencies:**

- All direct and indirect dependencies in go.mod
- Uses `go get -u` with appropriate flags
- Automatically runs `go mod tidy` and `go mod verify`

**GitHub Actions:**

- actions/checkout (v2/v3 → v4)
- actions/cache (v2/v3 → v4)
- actions/upload-artifact (v2/v3 → v4)
- actions/download-artifact (v2/v3 → v4)
- actions/setup-go (v2/v3/v4 → v5)
- actions/setup-node (v2/v3 → v4)
- actions/setup-python (v2/v3/v4 → v5)
- docker/setup-qemu-action (v1/v2 → v3)
- docker/setup-buildx-action (v1/v2 → v3)
- docker/login-action (v1/v2 → v3)
- docker/build-push-action (v2/v3/v4 → v5)
- docker/metadata-action (v3/v4 → v5)

**Docker Files:**

- **Dockerfile**: Updates `ARG GO_VERSION` to latest stable Go
- **go.mod**: Updates `go` directive to match Dockerfile version
- **docker-compose.yml**: Updates compose file format version (2.x → 3.8)
- Warns about unpinned base images (alpine:latest)

**Pre-commit Hooks:**

- Automatically updates all pre-commit hook versions
- Uses `--freeze` to pin specific commit SHAs
- Updates: Go linters, formatters, security tools, file validators

#### Backups

By default, the script creates timestamped backups:

- `go.mod.backup.YYYYMMDD_HHMMSS`
- `go.sum.backup.YYYYMMDD_HHMMSS`
- `.github/workflows/*.yml.backup.YYYYMMDD_HHMMSS`

To skip backups: `./scripts/update-project.sh --no-backup`

To clean up backups after testing:

```bash
rm -f *.backup.* .github/workflows/*.backup.*
```

#### Recommended Workflow

1. **Before updating:**

   ```bash
   # Ensure you're on a clean branch
   git status

   # Create a feature branch
   git checkout -b update-project
   ```

1. **Ensure pre-commit is installed** (for hook updates):

   ```bash
   pip install pre-commit
   ```

1. **Run dry-run first:**

   ```bash
   ./scripts/update-project.sh --dry-run
   ```

1. **Update dependencies:**

   ```bash
   # For regular updates
   ./scripts/update-project.sh --minor

   # For conservative updates
   ./scripts/update-project.sh --patch
   ```

1. **Review changes:**

   ```bash
   git diff
   ```

1. **Test locally:**

   ```bash
   # Build the application
   go build -o ./app ./main.go

   # Run it
   ./app

   # Or test with Docker
   docker build -t podgrab:test .
   docker run -p 8080:8080 podgrab:test
   ```

1. **Test with docker-compose:**

   ```bash
   docker-compose up --build
   ```

1. **Commit if successful:**

   ```bash
   git add go.mod go.sum .github/workflows/
   git commit -m "Update dependencies to latest versions"

   # Clean up backups
   rm -f *.backup.* .github/workflows/*.backup.*
   ```

1. **Or rollback if issues found:**

   ```bash
   # Restore from backups
   mv go.mod.backup.* go.mod
   mv go.sum.backup.* go.sum
   mv .github/workflows/hub.yml.backup.* .github/workflows/hub.yml
   ```

#### Important Notes

- **Always test after updating** - Run the application and verify functionality
- **Check breaking changes** - Review release notes for major dependency updates
- **Security updates** - Use `go list -m -u all` to check for security
  vulnerabilities
- **Docker builds** - Test multi-platform builds if modifying Dockerfile
- **GitHub Actions** - Monitor workflow runs after updating actions

#### Troubleshooting

**Dependencies fail to build:**

```bash
# Check for incompatibilities
go mod verify

# See what changed
git diff go.mod go.sum

# Restore from backup and update incrementally
```

**GitHub Actions fail:**

```bash
# Check syntax
yamllint .github/workflows/*.yml

# Review action documentation for breaking changes
# Visit: https://github.com/actions/<action-name>/releases
```

**Docker build fails:**

```bash
# Test build locally
docker build --no-cache -t podgrab:test .

# Check specific platform
docker build --platform linux/amd64 -t podgrab:test .
```

#### Security Scanning

After updating dependencies, scan for known vulnerabilities:

```bash
# Using Go's built-in vulnerability scanner (Go 1.18+)
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

# Or use go list to check for updates with security fixes
go list -m -u all
```

## Adding New Scripts

When adding new scripts to this directory:

1. Make them executable: `chmod +x scripts/your-script.sh`
1. Add usage documentation at the top of the script
1. Update this README with script description and usage
1. Follow the existing patterns for error handling and output formatting
1. Include a `--help` flag
1. Use color output for better UX (see update-project.sh for examples)

## Script Standards

All scripts in this directory should:

- Include a shebang: `#!/bin/bash`
- Use `set -e` for fail-fast behavior
- Provide `--help` documentation
- Use colored output for clarity (GREEN for success, YELLOW for warnings, RED
  for errors)
- Support `--dry-run` for safe testing where applicable
- Create backups of modified files
- Exit with appropriate status codes (0 for success, non-zero for failure)
