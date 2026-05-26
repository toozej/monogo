# Configuration

## Core Settings

| Environment Variable | CLI Flag | Sub-command(s) | Description |
|---------------------|----------|----------------|-------------|
| `GH_TOKEN` | `--token`, `-t` | all | GitHub API token (preferred) |
| `GITHUB_TOKEN` | `--token`, `-t` | all | GitHub API token (fallback) |
| `CREATE_ISSUES` | `--create-issue` | archived, check | Create GitHub issues (true/false) |
| `NOTIFY_CONDENSE` | - | all (with `--notify`) | Condense multiple notifications into one (true/false) |
| - | `--notify` | archived, eol, check | Enable notifications to configured endpoints |
| - | `--workflow` | all | Path to specific workflow file to check |
| - | `--workflows-dir` | all | Path to directory containing workflow yaml files |
| - | `--repos-dir` | all | Path to base directory containing multiple repos to scan |
| - | `--update` | outdated, eol | Write updated versions to affected workflow files |
| - | `--pin` | outdated | Pin actions to SHAs instead of semver version strings |
| - | `--semver` | outdated | Use semver version strings instead of SHAs when updating |
| - | `--write`, `-w` | check | Auto-apply updates for EOL and outdated actions |
| - | `--stale-days` | archived, eol, check | Days before an action is considered stale (default 365) |
| - | `--verbose`, `-v` | all | Show detailed output |
| - | `--debug`, `-d` | all | Enable debug-level logging (includes rate limit info) |

## Notification Providers

Configure one or more of the following providers to receive alerts when archived actions are found. Use the `--notify` flag to enable notifications.

| Provider | Environment Variables |
|----------|-----------------------|
| **Gotify** | `GOTIFY_ENDPOINT`, `GOTIFY_TOKEN` |
| **Slack** | `SLACK_TOKEN`, `SLACK_CHANNEL_ID` |
| **Telegram** | `TELEGRAM_TOKEN`, `TELEGRAM_CHAT_ID` |
| **Discord** | `DISCORD_TOKEN`, `DISCORD_CHANNEL_ID` |
| **Pushover** | `PUSHOVER_TOKEN`, `PUSHOVER_RECIPIENT_ID` |
| **Pushbullet** | `PUSHBULLET_TOKEN`, `PUSHBULLET_DEVICE_NICKNAME` |

