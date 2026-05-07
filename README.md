# go-find-archived-gh-actions

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/go-find-archived-gh-actions)
[![Go Report Card](https://goreportcard.com/badge/github/toozej/go-find-archived-gh-actions)](https://goreportcard.com/report/github/toozej/go-find-archived-gh-actions)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/go-find-archived-gh-actions/ci.yaml)
![GitHub Actions Release Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/go-find-archived-gh-actions/release.yaml)
![GitHub Actions Weekly Docker Refresh Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/go-find-archived-gh-actions/weekly-docker-refresh.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/go-find-archived-gh-actions)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/go-find-archived-gh-actions/total)

A tool to detect archived GitHub Actions in repository workflows.

## What it does

This tool scans your GitHub Actions workflows (`.github/workflows/**/*.yml` and `**/*.yaml`) and checks if any of the `uses:` actions have been archived by their maintainers on GitHub. Archived actions may contain security vulnerabilities, stop receiving updates, or cease working with future GitHub changes.

## Features

- 🔍 **Automatic Detection**: Scans all workflow files in your repository
- 🚨 **Exit Codes**: Returns error code when archived actions are found (CI/CD friendly)
- ⚠️ **Outdated Detection**: Optionally checks for outdated action versions
- ⏳ **Stale Detection**: Detects actions not updated in over a year or marked with GitHub deprecation warnings
- 📢 **Notifications**: Send alerts to configured webhooks when archived actions are detected
- 🐛 **Issue Creation**: Automatically create GitHub issues to track replacement tasks
- 🔧 **Flexible Configuration**: Environment variables, config files, and CLI flags
- 📊 **Verbose Output**: Detailed reporting of findings and API calls
- 🐳 **Docker Support**: Run via Docker or as native binary
- ⚡ **Concurrent API Calls**: Parallel repository checks with rate limit protection
- 💾 **Smart Caching**: Each action is looked up only once, even if used across multiple workflows

## Installation

### From GitHub Releases

Download the latest release from [GitHub Releases](https://github.com/toozej/go-find-archived-gh-actions/releases).

### Using Go

```bash
go install github.com/toozej/go-find-archived-gh-actions@latest
```

### Docker

```bash
# Mount your current working directory so the tool can scan your workflows
docker run --rm -v $(pwd):/workspace -w /workspace ghcr.io/toozej/go-find-archived-gh-actions:latest

# With a GitHub token
docker run --rm -v $(pwd):/workspace -w /workspace -e GH_TOKEN=your_token ghcr.io/toozej/go-find-archived-gh-actions:latest

# With verbose output and outdated checking
docker run --rm -v $(pwd):/workspace -w /workspace -e GH_TOKEN=your_token ghcr.io/toozej/go-find-archived-gh-actions:latest --verbose --check-outdated
```

## Usage

### Basic Usage

```bash
# Check all workflows in current repository
go-find-archived-gh-actions

# Check a specific workflow file
go-find-archived-gh-actions --workflow .github/workflows/ci.yml

# Check a specific directory of workflow files
go-find-archived-gh-actions --workflows-dir ~/src/github/username/repo/.github/workflows

# Check multiple repos in a base directory (bulk scanning)
go-find-archived-gh-actions --repos-dir ~/src/github

# Verbose output
go-find-archived-gh-actions --verbose

# Debug logging (includes rate limit info)
go-find-archived-gh-actions --debug

# Check for outdated actions (not archived, but not latest version)
go-find-archived-gh-actions --check-outdated

# Check for stale/deprecated actions
go-find-archived-gh-actions --stale

# Check for stale actions with custom threshold (180 days instead of default 365)
go-find-archived-gh-actions --stale --stale-days 180
```

### Path Expansion

All path inputs support `~` expansion to your home directory:

```bash
go-find-archived-gh-actions --workflows-dir ~/src/github/repo/.github/workflows
go-find-archived-gh-actions --repos-dir ~/src/github
```

### Authentication

Set your GitHub token using one of these methods (in order of priority):

1. `--token` flag
2. `GH_TOKEN` environment variable
3. `GITHUB_TOKEN` environment variable

```bash
# Using environment variable
export GH_TOKEN=your_github_token_here
go-find-archived-gh-actions

# Using CLI flag
go-find-archived-gh-actions --token your_github_token_here

# Using GitHub CLI (gh) to get a token automatically
export GH_TOKEN=$(gh auth token)
go-find-archived-gh-actions

# Or inline
go-find-archived-gh-actions --token $(gh auth token)
```

### Notifications

Configure one or more notification providers and enable them with the `--notify` flag:

```bash
# Example: Configuring Slack
export SLACK_TOKEN=xoxb-...
export SLACK_CHANNEL_ID=C12345678
go-find-archived-gh-actions --notify
```

### Issue Creation

Automatically create GitHub issues when archived actions are found:

```bash
go-find-archived-gh-actions --create-issue
```

### Configuration File

Create a `.env` file in your repository root:

```env
GH_TOKEN=your_github_token_here
SLACK_TOKEN=xoxb-...
SLACK_CHANNEL_ID=C12345678
CREATE_ISSUES=true
```

### GitHub Action

Use the provided GitHub Action in your workflows:

```yaml
name: Check for Archived Actions
on:
  schedule:
  - cron: '0 0 * * 0' # Weekly
  workflow_dispatch:

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
    - name: Checkout
      uses: actions/checkout@v4

    - name: Check archived actions
      id: check
      uses: toozej/go-find-archived-gh-actions@main
      with:
        token: ${{ secrets.GITHUB_TOKEN }}
        verbose: true
        create-issue: true

    - name: Fail if archived actions found
      if: steps.check.outputs.has-archived == 'true'
      run: exit 1
```

### Pre-commit Hook

Add to your `.pre-commit-config.yaml`:

```yaml
repos:
- repo: https://github.com/toozej/go-find-archived-gh-actions
  rev: main
  hooks:
  - id: go-find-archived-gh-actions
    name: Check for archived GitHub Actions
    args: [--verbose]
```

## Exit Codes

- `0`: Success - no archived, outdated, or stale actions found
- `1`: Error - archived, outdated, or stale actions found, or execution failed

## Example Output

### Archived Actions Only

```
$ go-find-archived-gh-actions --verbose

Found 3 workflow files
- .github/workflows/ci.yml (2 uses)
- .github/workflows/release.yml (1 uses)
Extracted 3 unique action references
- actions/checkout
- actions/setup-go
- docker/build-push-action

Checking 3 action repositories for archived status...

🚨 Found 1 archived GitHub Actions in 1 workflows:

📄 .github/workflows/ci.yml:
❌ actions/checkout

❌ Archived actions detected. Please replace them with actively maintained alternatives.
```

### With Outdated Checking

```
$ go-find-archived-gh-actions --verbose --check-outdated

Found 1 workflow files
- example/workflows/example-archived-actions.yaml (9 uses)
Extracted 9 unique action references
- actions-rs/toolchain@v1
- actions/cache@v2
- actions/checkout@v4
Checking 9 action repositories for archived status...
Checking 5 non-archived action repositories for latest versions...

🚨 Found 4 archived GitHub Actions in 4 workflows:

📄 example-archived-actions.yaml:
❌ actions-rs/audit-check@v1
❌ actions-rs/cargo@v1
❌ actions-rs/clippy-check@v1
❌ actions-rs/toolchain@v1


⚠️ Found 2 outdated GitHub Actions in 2 uses:

📄 example-archived-actions.yaml:
⚠️ actions/cache@v2 (latest: v4.0.0)
⚠️ actions/checkout@v4 (latest: v4.1.0)

❌ Archived actions detected. Please replace them with actively maintained alternatives.
```

### With Stale Checking

```
$ go-find-archived-gh-actions --verbose --stale

Checking 5 non-archived action repositories for stale/deprecated status...

⏳ Found 1 stale/deprecated GitHub Actions in 1 uses:

📄 ci.yml:
⏳ some-action/old-action@v1 (DEPRECATED: This action uses Node.js 16 which is deprecated)
⏳ another/stale-action@v2 (not updated since 2021-05-10)

⏳ Stale or deprecated actions detected. Consider replacing them with actively maintained alternatives.
```

### Major Version Tag Handling

When using `--check-outdated`, the tool intelligently handles major version tags (e.g., `v2`):

- If you're using `action@v2` and the latest release is `v2.3.3`, the tool compares the commit SHAs
- If the major version tag (`v2`) points to the same commit as the latest version (`v2.3.3`), it's **not** marked as outdated
- If the major version tag (`v2`) points to a different commit than `v2.3.3`, it **is** marked as outdated (a new patch exists)
- This allows you to use major version tags (recommended practice) without false positives

```
# action@v2 points to same commit as v2.3.3 (same SHA) - NOT outdated
# action@v2 but latest is v3.0.0 (different major) - IS outdated
```

### Debug Mode (Rate Limit Info)

When running with `--debug`, the tool logs GitHub API rate limit information:

```
$ go-find-archived-gh-actions --debug
DEBU[0000] GitHub API rate limit: limit=5000 remaining=4998 used=2 reset=2026-05-04T22:00:00Z resource=core
```

## Configuration

### Core Settings

| Environment Variable | CLI Flag | Description |
|---------------------|----------|-------------|
| `GH_TOKEN` | `--token`, `-t` | GitHub API token (preferred) |
| `GITHUB_TOKEN` | `--token`, `-t` | GitHub API token (fallback) |
| `CREATE_ISSUES` | `--create-issue` | Create GitHub issues (true/false) |
| `NOTIFY_CONDENSE` | - | Condense multiple notifications into one (true/false) |
| - | `--notify` | Enable notifications to configured endpoints |
| - | `--workflow`, `-w` | Path to specific workflow file to check |
| - | `--workflows-dir` | Path to directory containing workflow yaml files |
| - | `--repos-dir` | Path to base directory containing multiple repos to scan |
| - | `--check-outdated` | Check for outdated action versions |
| - | `--stale` | Check for stale/deprecated actions |
| - | `--stale-days` | Days before an action is considered stale (default 365) |
| - | `--verbose`, `-v` | Show detailed output |
| - | `--debug`, `-d` | Enable debug-level logging (includes rate limit info) |

### Notification Providers

Configure one or more of the following providers to receive alerts when archived actions are found. Use the `--notify` flag to enable notifications.

| Provider | Environment Variables |
|----------|-----------------------|
| **Gotify** | `GOTIFY_ENDPOINT`, `GOTIFY_TOKEN` |
| **Slack** | `SLACK_TOKEN`, `SLACK_CHANNEL_ID` |
| **Telegram** | `TELEGRAM_TOKEN`, `TELEGRAM_CHAT_ID` |
| **Discord** | `DISCORD_TOKEN`, `DISCORD_CHANNEL_ID` |
| **Pushover** | `PUSHOVER_TOKEN`, `PUSHOVER_RECIPIENT_ID` |
| **Pushbullet** | `PUSHBULLET_TOKEN`, `PUSHBULLET_DEVICE_NICKNAME` |

## Quick Demo

To quickly see how this tool works, run the demo which checks an example workflow containing archived and outdated actions:

```bash
# Build and run against example workflow (includes outdated checking)
make demo

# Or run manually after building
make local-build
./out/go-find-archived-gh-actions --workflow example/workflows/example-archived-actions.yaml --verbose --check-outdated

# Using GitHub CLI for authentication
./out/go-find-archived-gh-actions --workflow example/workflows/example-archived-actions.yaml --verbose --check-outdated --token $(gh auth token)
```

The example workflow at `example/workflows/example-archived-actions.yaml` contains:
- **Archived actions** (4 from `actions-rs/*` organization): `actions-rs/toolchain`, `actions-rs/cargo`, `actions-rs/clippy-check`, `actions-rs/audit-check`
- **Current actions** (GitHub official): `actions/checkout@v6`, `actions/setup-go@v6`, `github/codeql-action`, `actions/upload-artifact@v4`, `actions/download-artifact@v4`
- **Outdated but not archived**: `actions/cache@v2` (latest: v5.x), `actions/download-artifact@v4` (latest: v8.x), `actions/upload-artifact@v4` (latest: v7.x)
