# Migration Guide

Migrate podcast files and database from
[akhilrex/podgrab](https://github.com/akhilrex/podgrab) to
[toozej/podgrab](https://github.com/toozej/podgrab).

## Overview

The migration script (`scripts/migrate_to_fork.go`) updates episode file paths
in the database to match the fork's naming conventions. It is **idempotent**
and safe to re-run.

**What it does:**

- Renames episode files to match current filename format settings
- Updates database paths to reflect new file locations
- Creates a database backup before making changes
- Reports a summary of all actions taken

## Prerequisites

- Existing Podgrab database (`podgrab.db`) with downloaded episodes
- Read/write access to the config and assets directories
- Go 1.15+ toolchain (for local execution) **or** Docker (for container
  execution)

## Running Locally (Go Toolchain)

Requires Go installed on the host machine.

### Basic Usage

```bash
CONFIG=/path/to/config DATA=/path/to/assets go run scripts/migrate_to_fork.go
```

### Dry Run

Preview changes without modifying anything:

```bash
CONFIG=/path/to/config DATA=/path/to/assets go run scripts/migrate_to_fork.go --dry-run
```

### Verbose Logging

Enable debug-level output:

```bash
CONFIG=/path/to/config DATA=/path/to/assets go run scripts/migrate_to_fork.go --verbose
```

### Flags

| Flag          | Description                       | Default |
| ------------- | --------------------------------- | ------- |
| `--dry-run`   | Preview changes without executing | `false` |
| `--verbose`   | Enable verbose (debug) logging    | `false` |

### Environment Variables

| Variable | Description                      | Default       |
| -------- | -------------------------------- | ------------- |
| `CONFIG` | Path to config directory (DB)    | `.`           |
| `DATA`   | Path to assets directory (files) | `./assets`    |

## Running with Docker Compose

The `docker-compose.yml` includes a `podgrab-migrate` service that runs the
migration script inside a container with the same volume mounts as the main
Podgrab service.

The migration service is defined with `profiles: [migration]`, so it does
**not** start with normal `docker compose up` commands.

### Dry Run

```bash
docker compose --profile migration run --rm podgrab-migrate --dry-run
```

### With Verbose Logging

```bash
docker compose --profile migration run --rm podgrab-migrate --dry-run --verbose
```

### Execute the Migration

```bash
docker compose --profile migration run --rm podgrab-migrate
```

### Passing Flags

All flags are appended after the service name:

```bash
docker compose --profile migration run --rm podgrab-migrate --dry-run --verbose
```

### Custom Volume Paths

Edit the `podgrab-migrate` service volumes in `docker-compose.yml` to match
your deployment:

```yaml
podgrab-migrate:
  # ...
  volumes:
    - /your/config/path:/config
    - /your/data/path:/assets
```

## Running with Docker (Standalone)

Without Docker Compose, build and run a one-off container from the `init`
build stage:

```bash
docker build --target init -t podgrab-migrate .

docker run --rm \
  -v /path/to/config:/config \
  -v /path/to/data:/assets \
  -e CONFIG=/config \
  -e DATA=/assets \
  podgrab-migrate \
  go run scripts/migrate_to_fork.go --dry-run
```

## Recommended Workflow

1. **Stop Podgrab** to prevent concurrent database access:

   ```bash
   docker compose stop podgrab
   ```

2. **Run a dry run** to review what will change:

   ```bash
   docker compose --profile migration run --rm podgrab-migrate --dry-run --verbose
   ```

3. **Review the output** and confirm the changes look correct.

4. **Run the migration**:

   ```bash
   docker compose --profile migration run --rm podgrab-migrate
   ```

5. **Start Podgrab**:

   ```bash
   docker compose start podgrab
   ```

## Output

The script prints a summary after completion:

```
========================================
Migration Summary
========================================
Total episodes processed: 142
Files moved:              138
Files already migrated:   2
Files not found:          1
Errors:                   0
```

| Field                    | Description                                |
| ------------------------ | ------------------------------------------ |
| Total episodes processed | All downloaded episodes in the database    |
| Files moved              | Episodes renamed and database updated      |
| Files already migrated   | Episodes already at the correct path       |
| Files not found          | Files missing on disk (database updated)   |
| Errors                   | Episodes that failed to migrate            |

## Safety

- **Automatic backup**: A `podgrab_migration_backup_<timestamp>.tar.gz` file is
  created in `CONFIG/backups/` before any changes are made.
- **Rollback on failure**: If the database update fails after moving a file,
  the script attempts to move the file back to its original location.
- **No overwrites**: If a destination file already exists, the episode is
  skipped and reported as an error.
- **Idempotent**: Safe to re-run. Episodes already at the correct path are
  skipped.

## Troubleshooting

### "database not found" error

Ensure `CONFIG` points to the directory containing `podgrab.db`.

### "Failed to initialize database" error

The database file may be corrupted or locked. Ensure Podgrab is stopped before
running the migration.

### Files not found

Episodes that were deleted from disk will have their database paths updated
without moving files. This is expected and reported in the summary.

### Permission errors in Docker

Ensure the container user has read/write access to both volumes. The migration
service inherits the same volume paths as the main `podgrab` service.

## Related Documentation

- [Docker Deployment](../deployment/docker.md) - Docker and Docker Compose
  setup
- [Configuration Guide](configuration.md) - Filename format settings
- [Scripts README](../../scripts/README.md) - Project utility scripts
