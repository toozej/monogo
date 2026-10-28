# Migration Guide

Migrate podcast files and database from
[`akhilrex/podgrab`](https://github.com/akhilrex/podgrab) to
[Podgrab in monogo](https://github.com/toozej/monogo/tree/main/apps/podgrab).

## Overview

The migration script (`scripts/migrate_to_fork.go`) updates episode file paths
in the database to match the fork's naming conventions. It is **idempotent**
and safe to re-run.

**What it does:**

- Renames episode files to match current filename format settings
- Updates database paths to reflect new file locations
- Creates a database backup before it makes changes
- Reports a summary of all actions taken

## Prerequisites

- Current Podgrab database (`podgrab.db`) with downloaded episodes
- Read and write access to the config and assets directories
- Go 1.27.3+ toolchain (for local execution) **or** Docker (for container
  execution)

## Run Locally (Go Toolchain)

Requires Go installed on the host machine.

### Basic Usage

```bash
CONFIG=/path/to/config DATA=/path/to/assets go run scripts/migrate_to_fork.go
```

### Dry Run

Preview changes without any modifications:

```bash
CONFIG=/path/to/config DATA=/path/to/assets go run scripts/migrate_to_fork.go --dry-run
```

### Verbose Logs

Enable debug-level output:

```bash
CONFIG=/path/to/config DATA=/path/to/assets go run scripts/migrate_to_fork.go --verbose
```

### Flags

| Flag          | Description                       | Default |
| ------------- | --------------------------------- | ------- |
| `--dry-run`   | Preview changes without execution | `false` |
| `--verbose`   | Enable verbose (debug) log output    | `false` |

### Environment Variables

| Variable | Description                      | Default       |
| -------- | -------------------------------- | ------------- |
| `CONFIG` | Path to config directory (DB)    | `.`           |
| `DATA`   | Path to assets directory (files) | `./assets`    |

## Run with Docker Compose

The `docker-compose.yml` includes a `podgrab-migrate` service that runs the
migration script inside a container with the same volume mounts as the main
Podgrab service.

The migration service uses `profiles: [migration]`, so it does
**not** start with normal `docker compose up` commands.

### Dry Run

```bash
docker compose --profile migration run --rm podgrab-migrate --dry-run
```

### With Verbose Logs

```bash
docker compose --profile migration run --rm podgrab-migrate --dry-run --verbose
```

### Execute the Migration

```bash
docker compose --profile migration run --rm podgrab-migrate
```

### Pass Flags

Append all flags after the service name:

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

## Run with Docker (Standalone)

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
| Files not found          | Files absent from disk (database updated)   |
| Errors                   | Episodes that failed to migrate            |

## Safety

- **Automatic backup**: The migration creates `CONFIG/backups/podgrab_migration_backup_<timestamp>.tar.gz` before any changes.
- **Rollback on failure**: After a database update failure, the script attempts to restore the moved file's original location.
- **No overwrites**: If the destination file already exists, the migration skips the episode and reports an error.
- **Idempotent**: Safe to repeat. The migration skips episodes already at the correct path.

## Resolve Problems

### "database not found" error

Ensure `CONFIG` points to the directory with `podgrab.db`.

### "Failed to initialize database" error

The database file possibly has corruption or a lock. Stop Podgrab before you run the migration.

### Files not found

The migration updates database paths for episodes whose files no longer exist on disk without any file moves.
This behavior follows the design, and the summary reports it.

### Permission errors in Docker

Ensure the container user has read and write access to both volumes. The migration
service inherits the same volume paths as the main `podgrab` service.

## Related Documentation

- [Docker Deployment](../deployment/docker.md) - Docker and Docker Compose
  setup
- [Configuration Guide](configuration.md) - Filename format settings
- [Scripts README](../../../../scripts/README.md) - Project utility scripts
