# go-get-releases examples

Use a YAML configuration to store the repository list.
Use a wrapper script to load tokens before cron starts the tool.
The examples below use the current user's crontab and home directory.

## 1. Install the binary and create directories

Run these commands from the monorepo root:

```sh
make local-build APP=go-get-releases
mkdir -p "$HOME/.local/bin" \
  "$HOME/.config/go-get-releases" \
  "$HOME/.local/share/go-get-releases" \
  "$HOME/.local/state/go-get-releases"
install -m 755 out/go-get-releases "$HOME/.local/bin/go-get-releases"
```

Run each setup command as the user whose crontab will run the tool.
That user must have write access to the archive and log directories.

## 2. Create the repository list

Save this configuration as `~/.config/go-get-releases/config.yaml`:

```yaml
---
keep: 3
retries: 2
all_assets: true
repositories:
  - name: keepassxreboot/keepassxc
  - name: laurent22/joplin
  # Replace these examples with your repository paths before enabling them.
  # - name: gitlab.com/group/subgroup/repo
  # - name: codeberg.org/owner/repo
  # - name: tangled.sh/user.tngl.sh/repo
```

This configuration keeps the three latest stable releases for each repository.
`all_assets: true` downloads every uploaded asset, including packages, signatures, and metadata.
The tool also downloads recognized checksum files and checks available checksums.
It excludes prereleases and automatically generated source archives.

For platform selection, use the existing [config.example.yaml](config.example.yaml).
That file includes platform targets and Joplin filename rules.
Leave `all_assets` false when you use those targets.

Unqualified names use GitHub.
Use `gitlab.com`, `codeberg.org`, or `tangled.sh` to select another forge.
GitLab paths can include subgroups.
The tool also accepts HTTPS repository URLs and the `tangled.org` alias.

## 3. Configure tokens

Create `~/.config/go-get-releases/tokens.env`:

```sh
touch "$HOME/.config/go-get-releases/tokens.env"
chmod 600 "$HOME/.config/go-get-releases/tokens.env"
```

Edit that file to add the tokens for your configured forges:

```sh
GH_TOKEN='replace-with-your-github-token'
# GITLAB_TOKEN='replace-with-your-gitlab-token'
# FORGEJO_TOKEN='replace-with-your-codeberg-token'
```

Replace each placeholder with a token before you enable its assignment.
Remove the assignment if you want anonymous access to that forge.
Tangled's public read endpoints require no token.

The tool also accepts these environment variables:

| Forge | Token lookup order |
| --- | --- |
| GitHub | `GH_TOKEN`, `GITHUB_TOKEN`, `GITHUB_API_TOKEN`, then the GitHub CLI |
| GitLab | `GITLAB_TOKEN`, `GITLAB_ACCESS_TOKEN`, `OAUTH_TOKEN` |
| Codeberg | `FORGEJO_TOKEN`, `CODEBERG_TOKEN` |

The wrapper below exports the assignments from `tokens.env`.
The tool does not load this file itself.
Use shell assignment syntax in this file.
See [authentication details](README.md#authentication-and-rate-limits) for token precedence and API headers.

## 4. Create the cron wrapper

Save this script as `~/.local/bin/go-get-releases-cron`:

```sh
#!/bin/sh
set -eu

releases_config_dir="$HOME/.config/go-get-releases"
releases_archive_dir="$HOME/.local/share/go-get-releases"

set -a
. "$releases_config_dir/tokens.env"
set +a

exec "$HOME/.local/bin/go-get-releases" \
  --config "$releases_config_dir/config.yaml" \
  --directory "$releases_archive_dir" \
  --timeout 2h
```

Make the script executable:

```sh
chmod 700 "$HOME/.local/bin/go-get-releases-cron"
```

The script uses absolute paths and supplies the archive directory through `--directory`.
The timeout limits the complete run to two hours.
Change `--timeout` if your repository list needs more time.

Run the wrapper once before adding the cron entry:

```sh
"$HOME/.local/bin/go-get-releases-cron"
```

The tool returns a nonzero exit status if any repository fails.
It continues to process the other repositories.
It preserves the previous release directories when a download or checksum check fails.

## 5. Schedule the job

Open the current user's crontab:

```sh
crontab -e
```

Add this entry to run the wrapper daily at 04:00:

```cron
0 4 * * * "$HOME/.local/bin/go-get-releases-cron" >> "$HOME/.local/state/go-get-releases/cron.log" 2>&1
```

Cron uses the scheduler's configured timezone.
If your cron implementation supports `CRON_TZ`, add this line before the entry to select Pacific time:

```cron
CRON_TZ=America/Los_Angeles
```

Read the log after a run:

```sh
tail -n 100 "$HOME/.local/state/go-get-releases/cron.log"
```

The entry appends standard output and error messages to the log.
Configure log rotation if you keep the job enabled.
The tool locks the archive directory during each run to prevent overlapping runs.
See [retention and lock behavior](README.md#stable-releases-and-retention) for recovery instructions.

## Alternative: a plain text repository list

Save one repository per line as `~/.config/go-get-releases/repos.txt`:

```text
keepassxreboot/keepassxc
laurent22/joplin
# gitlab.com/group/subgroup/repo
# codeberg.org/owner/repo
# tangled.sh/user.tngl.sh/repo
```

Start comment lines with `#` in the first column.
The loop below ignores comments and empty lines.

In the wrapper, replace the final `exec` command with this block:

```sh
set --
releases_repo=''
while IFS= read -r releases_repo || [ -n "$releases_repo" ]; do
  case "$releases_repo" in
    ''|\#*) continue ;;
  esac
  set -- "$@" --repo "$releases_repo"
done < "$releases_config_dir/repos.txt"

exec "$HOME/.local/bin/go-get-releases" \
  --directory "$releases_archive_dir" \
  --all-assets \
  --keep 3 \
  --retries 2 \
  --timeout 2h \
  "$@"
```

This version reads the repository list from `repos.txt`.
The existing wrapper still loads tokens from `tokens.env`.
The same cron entry runs either version.
