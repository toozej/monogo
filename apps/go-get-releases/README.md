# go-get-releases

Store the latest stable GitHub release packages for a list of repositories.
The default retention count is three releases per repository.
The tool runs once and returns a nonzero exit status if any repository fails.

This app uses the monorepo scaffold from `make new-app APP=go-get-releases`.

## Build and run

```sh
make local-build APP=go-get-releases
./out/go-get-releases --directory /srv/github-releases --keep 3 \
  keepassxreboot/keepassxc laurent22/joplin
```

Use the YAML configuration to specify repository rules:

```sh
./out/go-get-releases --config apps/go-get-releases/config.example.yaml
```

The tool accepts `owner/repo` and HTTPS GitHub repository URLs.
Use positional arguments or repeat `--repo` to add repositories.
CLI options override global values in the configuration.
Repository targets override global targets.

Use `--release-match` to select tags for one app in a monorepo:

```sh
./out/go-get-releases --keep 3 --all-assets \
  --release-match '^apps/go-sort-out-gh-actions/' toozej/monogo
```

The filter applies before the tool selects the latest releases.
Cleanup only removes older releases whose tags match the same filter.
YAML accepts `release_match` globally and per repository.
A repository filter overrides the global filter.

Use `--all-assets` or YAML `all_assets: true` to download every uploaded asset.
This includes packages without platform names, signatures, and metadata.
The tool still checks asset sizes, GitHub digests, and recognized checksum files.
This option bypasses platform targets and cannot be combined with `--require-all`.

## Asset selection

The default targets select all matching packages for these platforms:

| Platform | Architecture | Formats |
| --- | --- | --- |
| Windows | AMD64 | EXE, MSI, MSIX, ZIP, 7z, tar.gz |
| macOS | Intel AMD64 | DMG, PKG, ZIP, tar.gz, tar.xz |
| macOS | Apple Silicon ARM64 | DMG, PKG, ZIP, tar.gz, tar.xz |
| Linux | AMD64 | AppImage |
| Linux | ARM64 | AppImage |

Common aliases include `win`, `win64`, `mac`, `macos`, `osx`, `darwin`, `x64`,
`x86_64`, `x86-64`, `intel`, `aarch64`, and `apple-silicon`.
Architecture detection uses filename tokens.
An ARM64 asset does not match an AMD64 target.
A universal macOS asset satisfies both macOS targets and downloads once.
Platform-specific extensions can identify the OS.
The tool requires an architecture token unless you specify a filename rule.

For example, `App-x86_64.AppImage` matches Linux AMD64.
`App.dmg` needs a repository rule because its name does not identify the architecture.
The example configuration supplies rules for Joplin filenames that omit architecture tokens.
Inspect upstream filenames when you configure another repository.

Repeat `--target` to replace the global defaults:

```sh
./out/go-get-releases --directory /srv/github-releases \
  --target windows/amd64:exe+zip \
  --target macos/arm64:dmg+zip \
  --target linux/aarch64:appimage+deb+rpm owner/repo
```

Additional formats include MSIX bundles, tar.bz2, tar.zst, TGZ, TBZ2, TXZ,
compressed AppImages, GZ, XZ, ZST, BIN, and `binary` for names without an extension or names that end with an architecture token.
The tool stores packages without extraction.
It does not download the source archives that GitHub generates automatically.

Each YAML target can specify `match` and `exclude` regular expressions.
`match` assigns the configured OS and architecture to matching filenames.
The configured formats still apply.
`exclude` rejects matching filenames.
Rules use Go regular expressions and are case sensitive unless they contain `(?i)`.

A missing target produces a message by default.
A release with no matching package fails.
Use `--require-all` or `require_all: true` to require every target.
This option also prevents cleanup if an upstream release lacks a requested platform.

## Stable releases and retention

The tool verifies each repository through the GitHub REST API.
It reads every release page and sorts stable releases by publication time.
It excludes drafts, releases with the prerelease flag, semantic prerelease versions,
and names with alpha, beta, RC, preview, nightly, dev, or similar markers.
Tags can use semantic versions, dates, or other stable names.

Files use this directory structure:

```text
/srv/github-releases/
  owner/
    repo/
      v1.2.3/
        App-linux-amd64.AppImage
        checksums.txt
        .go-get-releases.json
      v1.2.2/
      v1.2.1/
```

The owner and repository each use a separate directory.
This prevents collisions between owners with repositories of the same name.
Ordinary version tags remain unchanged.
The tool percent-encodes path separators and leading dots in tags.
For example, `apps/tool/v1.2.3` becomes `apps%2Ftool%2Fv1.2.3`.

The tool downloads each release into a temporary directory.
It checks the asset size and any GitHub asset digest.
It also checks published MD5, SHA-1, SHA-256, and SHA-512 checksum files.
Supported checksum layouts include GNU, BSD, and bare per-asset sidecars.
Conflicting checksums fail verification.
Malformed supported checksum files also fail the run.
The tool limits each checksum file to 8 MiB.

If an asset has no upstream checksum, the tool reports that condition.
It checks the byte count and stores a local SHA-256 for later cache checks.
An upstream checksum is not always available.
The tool does not verify detached signatures or signature bundles.

A failed download or checksum check receives two retries by default.
Use `--retries` to specify between one and ten retries.
The tool waits between retries and obeys API rate limits.
It verifies cached package bytes before reuse.
It downloads corrupt or changed packages again.
It refreshes checksum files on each run.

The tool publishes a release directory only after all selected assets pass their checks.
It preserves the previous directory if a replacement fails.
It can recover a replacement interrupted between directory renames.
It removes older releases only after every retained release succeeds for that repository.
It only removes directories with a matching tool manifest.
It preserves unrelated directories and files.
A failure in one repository does not prevent downloads for another repository.

The tool creates `.go-get-releases.lock` in the destination during each run.
This lock prevents overlapping runs in the same destination.
If the process stops without cleanup, confirm that it stopped before you remove the stale lock.
The default total timeout is six hours.
Use `--timeout` to change it.

## Authentication and rate limits

The tool checks credentials in this order:

1. `GH_TOKEN`
2. `GITHUB_TOKEN`
3. `GITHUB_API_TOKEN`
4. `gh auth token --hostname github.com`, if the CLI is installed
5. Unauthenticated API access

An invalid configured token produces an API error.
The tool does not silently replace it with anonymous access.
The container does not include the GitHub CLI.
Supply a token through the environment for authenticated container runs.

The tool uses `github.com/google/go-github/v89` for API requests.
It sends API requests sequentially.
It waits until the primary limit resets or the secondary retry delay expires.
It also honors `Retry-After` for download errors.
It sends no GitHub token to asset redirect hosts.
See the [GitHub rate limit guidance](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)
and the [release asset API](https://docs.github.com/en/rest/releases/assets).

## Daily cron at 04:00

Build the binary before you add the cron entry.
Use absolute paths in the cron entry and the YAML configuration.
Ensure that the cron user can write the destination and log file.
Ensure that the cron environment contains a token or can access the user's GitHub CLI login.
The tool does not read `.env` files.

```cron
0 4 * * * /usr/local/bin/go-get-releases --config /etc/go-get-releases.yaml >> /var/log/go-get-releases.log 2>&1
```

Cron uses the scheduler's configured timezone.
If your cron implementation supports `CRON_TZ`, set it to the desired timezone.
For example, use `CRON_TZ=America/Los_Angeles` for local Pacific time.

## Container

```sh
make build APP=go-get-releases
docker run --rm -e GH_TOKEN -v github-releases:/releases \
  toozej/go-get-releases:latest --directory /releases --keep 3 owner/repo
```

The generated Compose config uses the example repository list and a named volume.
Edit the example configuration before you run it.

```sh
docker compose -f apps/go-get-releases/docker-compose.yml run --rm go-get-releases
```

The generated container runs as UID 65532.
A bind mount must permit that user to write files.

## Validation

```sh
make test APP=go-get-releases
make local-test APP=go-get-releases
make local-build APP=go-get-releases
make build APP=go-get-releases
make release-test APP=go-get-releases
make pre-commit
```

Unit tests use temporary directories and mock HTTP servers.
They cover selection, authentication, pagination, rate limits, retries, checksums,
cache repair, directory safety, interrupted replacements, and retention failures.
