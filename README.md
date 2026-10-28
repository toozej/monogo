# MonoGo

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/monogo)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml)
![GitHub Actions Release Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/release.yaml)
![GitHub Actions Weekly Docker Refresh Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/weekly-docker-refresh.yaml)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/monogo/total)

<img src="img/avatar.png" alt="monogo avatar" style="background-color: #FFFFFF;" />

MonoGo is a [Golang](https://go.dev/) [monorepo](https://en.wikipedia.org/wiki/Monorepo) with a common pipeline to update, build, test, and release applications.
It also provides a scaffold to quickly create a new application.

## Install an App with Homebrew

Released MonoGo apps are available as Homebrew casks from the
[`toozej/homebrew-tap`](https://github.com/toozej/homebrew-tap) repository. For
example, install `files2prompt` with:

```bash
brew install --cask toozej/tap/files2prompt
```

Replace `files2prompt` with the app's binary name (for example, use
`readme-gen` for the `github-readme-gen` app). Homebrew automatically adds the
tap when you use the fully qualified cask name.

| App | Badges | Description |
| --- | --- | --- |
| [`golang-starter`](apps/golang-starter/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/golang-starter) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Starter app for this Go monorepo. |
| [`files2prompt`](apps/files2prompt/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/files2prompt) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | LLM prompt generator from local files. |
| [`ghreleases2rss`](apps/ghreleases2rss/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/ghreleases2rss) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Subscribe to GitHub project releases in an RSS reader. |
| [`go-castctl`](apps/go-castctl/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/go-castctl) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Cast web video to a TV or watch it in a local browser. |
| [`go-find-liquor`](apps/go-find-liquor/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/go-find-liquor) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Go Find Liquor in Oregon. |
| [`go-get-releases`](apps/go-get-releases/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/go-get-releases) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Archive and verify stable release packages from GitHub, GitLab, Codeberg, and Tangled. |
| [`go-listen`](apps/go-listen/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/go-listen) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Web app that searches for artists and adds their top songs to Spotify playlists. |
| [`go-sort-out-gh-actions`](apps/go-sort-out-gh-actions/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/go-sort-out-gh-actions) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Finds outdated GitHub Actions runtimes and can notify, update, pin, or create issues. |
| [`gphotos-lister`](apps/gphotos-lister/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) | List filenames of photos with album hearts and copy local originals. |
| [`gotts-it`](apps/gotts-it/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/gotts-it) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Go-based text-to-speech tool. |
| [`kmhd2playlist`](apps/kmhd2playlist/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/kmhd2playlist) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Syncs songs from KMHD jazz radio to Spotify playlists. |
| [`lego-stego`](apps/lego-stego/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/lego-stego) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Go-based CLI tool for steganography. |
| [`notes-courier`](apps/notes-courier/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/notes-courier) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Moves notes between Simplenote, Memos, Obsidian, Hugo, and go-vite. |
| [`photos2map`](apps/photos2map/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/photos2map) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Generates GPX maps from photo EXIF data. |
| [`podgrab`](apps/podgrab/README.md) | ![Build Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/podgrab) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Manages podcasts on your server. Downloads and archives episodes and includes a player. |
| [`RSSFFS`](apps/RSSFFS/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/rssffs) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | RSS Feed Finder and Subscriber. |
| [`rss2socials`](apps/rss2socials/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/rss2socials) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Watches RSS feeds for new posts and announces them on Mastodon. |
| [`terranotate`](apps/terranotate/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/terranotate) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Terraform comment parser and validator. |
| [`trails-completionist`](apps/trails-completionist/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/trails-completionist) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Searchable trail completion tracker. |
| [`url2anki`](apps/url2anki/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/url2anki) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Generate Anki flashcards from a URL. |

`golang-starter` is the starter app in this Go monorepo. It keeps shared repository concerns at the root while each app lives under `apps/<app>`.

Apps share the root `go.mod`, `go.sum`, `.pre-commit-config.yaml`, `.devcontainer/`, workflows, scripts, Makefile, and `pkg/` packages.
The manifests in `tools/go-tools.tsv` and `tools/binary-tools.tsv` pin the versions of development tools.
Templates in `templates/app` generate app-local Docker and GoReleaser files.
Templates in `templates/common` generate app-local Air and Compose files and root files such as `.env.sample`.

## Common Commands

```bash
make list-apps
make generate-all
make test APP=golang-starter
make mutation-test APP=golang-starter
make local-build APP=golang-starter
make release-test APP=golang-starter
make pre-commit-install
```

`APP` defaults to `golang-starter`, so `make test` and `make local-build` work for the starter app.

## Documentation Checks

Vale checks prose in Markdown files. The `tools/go-tools.tsv` manifest pins its Go package version.
The shared installer for Go tools installs Vale.

Lychee checks links in Markdown files. The `tools/binary-tools.tsv` manifest pins its release binary.
The shared installer for binaries installs Lychee.
Both installers place their tools in `.tools/bin`. Make, pre-commit, and the CI
`checks` job use these repository-managed tools and the same configuration.

```bash
make docs-check
```

This command installs both pinned tools when they are absent or differ from the pinned versions.
It synchronizes Vale styles and runs both documentation checks.
It runs the link check even when Vale reports errors.
The command fails if either check fails.
You do not need separate setup targets.
Use `make vale` or `make lychee` to run one check with its required setup.

To check one Markdown file, set `FILE` to a relative or absolute file path:

```bash
make docs-check FILE=README.md
make docs-check FILE="docs/my guide.md"
```

If `FILE` is empty, unset, or does not name a file that exists, each check prints a
message and checks all documentation files. The same exclusions still apply.

Vale synchronizes
[`Syntaf/vale-llm-slop` v0.1.0](https://github.com/Syntaf/vale-llm-slop/releases/tag/v0.1.0)
before every check. Its downloaded `Slop` and `STE` styles reside in `.vale/styles`,
which Git and documentation checks ignore.

`.vale.ini` reports suggestions and warnings as advice. Only errors fail the
prose check. Contractions produce errors. The configuration disables the dictionary rule.
Package download failures and configuration errors also fail the check.

`.lychee.toml` sets a 20-second timeout, two retries per request, and bounded concurrency.

The shared script also retries the complete check twice after link failures,
such as HTTP errors and timeouts. It waits 5 seconds before the first retry
and 10 seconds before the second retry. Configuration and installation errors
fail immediately. These retries apply to Make, pre-commit, and CI.

Set `LYCHEE_CHECK_RETRIES` to change the retry count. Set
`LYCHEE_CHECK_RETRY_WAIT` to change the initial delay in seconds.
Both values must be nonnegative integers. The delay doubles after each retry.
For example, `make lychee LYCHEE_CHECK_RETRIES=0` disables retries of the complete
check. Lychee still uses the per-request retries from `.lychee.toml`.

Lychee runs with `-v -v` to show checked URLs and the redirects it follows.
The pinned version requires two verbosity flags to display successful redirects.
The pre-commit hook also displays this output when the check passes.
Broken local links, HTTP failures, and timeouts fail the link check.

The configuration excludes loopback URLs because documentation uses local server
examples. It does not check email addresses or links inside code examples.

It retains lychee's default exclusions for reserved example domains.
The configuration excludes two exact URLs in `apps/RSSFFS/CREDITS.md` because they require browser challenges.
One URL points to answer `60675491` on Stack Overflow. The other points to the Golang Cafe article about dependency upgrades.
The source check took place on 2026-10-13.

The configuration also excludes the exact Substack and Datadog article URLs in
`apps/go-sort-out-gh-actions/SECURITY_SCAN.md`. Both load
locally. Substack returns HTTP 403 to GitHub Actions, and Datadog connections
failed in two GitHub Actions runs recorded in `.lychee.toml`.
The exclusions preserve attribution and do not accept HTTP failures for other
URLs. Review these exceptions when either site changes its access policy.
It does not accept HTTP error codes or exclude entire public hosts.

Checks cover tracked Markdown and untracked Markdown that Git does not ignore.
They exclude downloaded tools and styles, dependencies, build output, and
archived import metadata under `.monogo`. These exclusions also apply when a
hook receives an explicit file list.
Resolve documentation findings separately from tool installation or configuration failures.

For specific files, use the shared check script or the local pre-commit hooks:

```bash
scripts/check-docs.sh vale README.md
scripts/check-docs.sh lychee README.md
pre-commit run vale --files README.md
pre-commit run lychee --files README.md
```

The hooks check the Markdown files that pre-commit selects. CI runs both full
documentation checks as separate Make steps and skips duplicate hook runs.
Network access is necessary for the first installation and each synchronization of Vale styles.

## Dev Container

Open the repository in a Dev Container to get the pinned Go toolchain and Node and Python support for pre-commit.
The container provides an isolated Docker daemon for the repository's Docker targets. The first creation runs `make pre-commit-install`, which installs
the repository-managed tools and Git hook.

Use the **MonoGo** tasks in VS Code to build, test, develop with Air, or build a Docker image for any app.
Each task prompts for its `apps/<app>` directory name.
The same commands are available in the container terminal, for example:

```bash
make local-iterate APP=golang-starter
make docker-build APP=golang-starter
```

## Generated Configs

Install the prerequisites. Regenerate the configs:

```bash
make pre-reqs
make generate-all
```

To add or customize an app:

1. Create `apps/<app>/app.yaml`.
2. Put the app's Go code under `apps/<app>`.
3. Run `make app-generate APP=<app>`.
4. Add the app to the GitHub Actions matrices in `.github/workflows/ci.yaml` and `.github/workflows/weekly-docker-refresh.yaml`.

For normal per-app differences, change values in `apps/<app>/app.yaml`. Shared Go packages belong in root `pkg/`; app-private code belongs in `apps/<app>/internal`. For shared build or root tooling behavior, update `templates/app/*.tmpl` or `templates/common/*.tmpl` and run `make generate-all`.

See [docs/app-configuration.md](docs/app-configuration.md) for the full `app.yaml` field reference.
The reference covers CGO builds (`cgoEnabled`), runtime image selection (`runtimeImage`), and exposed ports (`port`).

## Repository Tooling

The `tools/go-tools.tsv` manifest declares Go-based development tools and pins their versions.
The installer puts each tool independently into `.tools/bin` as `package@version`.
This separation prevents tool dependencies from changes to apps or other tools. `make pre-commit-install` additionally installs the local Git hook.
Normal installation never changes dependency pins.

Run `make pre-commit-update` to upgrade the pinned Go tools and pre-commit hook
revisions, regenerate app configs, and validate the updated toolchain. The
scheduled tool autoupdate workflow runs the same target and opens a PR only when
the update produces repository changes.

## Create a New App

Run the scaffold script to clone the `golang-starter` app into a new app directory and wire it into CI:

```bash
make new-app APP=mytool
```

The `scripts/create-new-app.py` script copies `apps/golang-starter` to `apps/mytool`, renames command packages, and rewrites imports.
It updates `app.yaml`, amends the CI app matrix, and appends the Dependabot Docker entry.
It finishes with `go mod tidy` and `make app-generate APP=mytool`.
Commit the generated configs with the new app.

After scaffold:

1. Edit `apps/mytool/app.yaml` to describe the service and tweak build settings.
2. Replace `internal/starter` and extend `cmd/mytool` with real functionality.
3. Run `make test APP=mytool` and `make local-build APP=mytool` to verify everything compiles.
4. Commit the new files; CI and Dependabot already include the app once the script completes.

### Remove an App

```bash
make delete-app APP=mytool
```

The `scripts/delete-app.py` helper removes `apps/mytool` and its generated artifacts.
It updates shared automation, such as the CI matrix and Dependabot Docker entries, before it runs `go mod tidy`.
Review `git status` afterward.
Remove the extra references from documentation, dashboards, and secret stores.

## Import Current Services

Import a Go service with its Git history. `APP=owner/repo` defaults to `github.com/owner/repo`; include a hostname to import from another VCS host:

```bash
make import APP=toozej/go-listen
make import APP=github.com/toozej/go-listen
```

The import target:

1. Imports the source repository with `git subtree add` without a squash of its history.
2. Moves the imported app under `apps/<repo>`.
3. Merges the imported `go.mod` requirements into the root `go.mod`.
4. Rewrites imports from the old module path to `github.com/toozej/monogo/apps/<repo>`.
5. Moves imported `pkg/` packages to root `pkg/` and keeps app-private code under `apps/<repo>/internal`.
6. Stores the original app `go.mod`, `go.sum`, and build config under `apps/<repo>/.monogo/`.
7. Imports source tags under `refs/tags/apps/<repo>/`.
8. Imports metadata for GitHub releases under `apps/<repo>/.monogo/releases/`.
9. Generates app-local Docker, Compose, Air, and GoReleaser config with gomplate.
10. Updates the app matrices for CI and Docker refresh in GitHub Actions, plus Dependabot Docker config.
11. Runs tests, local build, and GoReleaser snapshot validation.

Options:

```bash
make import APP=toozej/go-listen IMPORT_REF=main
make import APP=toozej/go-listen IMPORT_NAME=listen
make import APP=toozej/go-listen IMPORT_RELEASES=assets
make import APP=toozej/go-listen IMPORT_RELEASES=none
make import APP=toozej/go-listen IMPORT_ARGS=--metadata-only
make import APP=toozej/go-listen IMPORT_SKIP_VERIFY=1
make import APP=github.example.com/owner/repo
```

`make import` requires a clean, committed baseline of the starter app.
Git history does not include GitHub release records, so the target uses `gh` to import metadata for releases.
Use `IMPORT_RELEASES=assets` to download release assets too.

Use `IMPORT_ARGS=--metadata-only` if a separate step already migrated the app code into `apps/<app>`.
This option tells the importer to collect release metadata, import source tags, and write `app.yaml`.
It also updates workflow and Dependabot entries and regenerates app-local config.

## Release Model

This repository uses GoReleaser OSS with independent release tags for each app: `apps/<app>/vX.Y.Z`.
A push of this tag triggers the [release workflow](.github/workflows/release.yaml).
The workflow builds, signs, and publishes **only** that app's binaries, archives, Docker images, and Homebrew cask.
It publishes Docker images to Docker Hub, GHCR, and Quay.

The workflow creates a clean `vX.Y.Z` tag locally so GoReleaser can parse the version.
It disables GoReleaser's SCM release creation.
It creates the real GitHub release at `apps/<app>/vX.Y.Z` with `gh`.

### Reproducible builds and the Homebrew cask

Each app's archives are byte-for-byte identical across independent builds for each architecture.
The build uses `-trimpath` and sets `mod_timestamp` to the commit timestamp.
The `scripts/normalize-archive-mtimes.sh` script sets the mtimes of all files in an archive to that timestamp.
This GoReleaser `before` hook covers README files, LICENSE files, completions, and manpages.
The build uses `gzip -n` for manpages.

The `Darwin_all` universal binary is the only exception because GoReleaser merges the two architectures in nondeterministic order.
This difference is cosmetic because each release comes from a single build.

The **same** GoReleaser build renders the Homebrew cask and produces the archives.
The `skip_upload: "true"` option stops the build job from a push to the tap.
The build computes the cask's `sha256` values from the exact archives it produces.
After the GitHub release assets exist, the `publish_homebrew_cask` job commits the rendered cask to [`toozej/homebrew-tap`](https://github.com/toozej/homebrew-tap).
This job uses `scripts/publish-homebrew-cask.sh`.

Because the cask and uploaded archives come from one build, the cask's checksum always matches the file `brew` downloads.
Do not add a second GoReleaser run to publish the cask.
A new build on another runner previously caused `brew upgrade` SHA-256 mismatches.

### Trigger a release of a specific app

The recommended path computes the next version and pushes the tag for you:

```bash
make release APP=url2anki TYPE=patch   # TYPE = major | minor | patch (default: patch)
```

`make release` runs the app's tests and computes the next `apps/<app>/vX.Y.Z` tag.
It increases the version by `TYPE` from the app's latest tag and checks both local tags and `origin`.
It creates and pushes the new tag.
CI builds, signs, and publishes the rest.

Equivalently, tag and push by hand:

```bash
git tag -a apps/url2anki/v0.1.0 -m "url2anki v0.1.0"
git push origin apps/url2anki/v0.1.0
```

Before a release, validate the GoReleaser config and build a snapshot locally with `make release-test APP=url2anki`.

### Automatic releases for Go dependency updates

When you push root `go.mod` or `go.sum` changes to `main`, the
`Release Go dependency updates` workflow runs `make release-all TYPE=patch`.
The target tests each previously released app and increases its patch version.
It pushes the app tag to trigger the release workflow.
The target skips apps with no previous release, consistent with the current `release-all` behavior.

The workflow reuses `AUTOUPDATE_APP_ID` and `AUTOUPDATE_APP_PRIVATE_KEY`. That
GitHub App needs **Contents: write** on this repository so its tag pushes trigger
release builds. The workflow serializes runs and does not cancel an active release.
Tags point to the commit with dependency updates.
If you repeat a completed run, it creates another patch release for each app.

### Delete a bad release

To remove a release published in error, such as a wrong version, broken assets, or an old cask with a checksum mismatch:

```bash
make delete-release APP=url2anki VERSION=v1.6.0
```

This command deletes the GitHub release for `apps/<app>/<version>` and removes its tag **locally and on `origin`**.
Each step checks for the release or tag and skips it if it is already absent.
You must pass `VERSION` explicitly as `vX.Y.Z`. The command does not compute a default value.
Afterward, create a fresh release with `make release`, or recreate and push the same tag to rebuild that version.

### Weekly Docker refresh

The [workflow for weekly Docker refresh](.github/workflows/weekly-docker-refresh.yaml) runs on a schedule or on demand through **Run workflow** and `workflow_dispatch`.
For each app with a published `apps/<app>/vX.Y.Z` tag, it checks out that exact released commit.
It rebuilds the `latest` and `distroless` images to include security patches from upstream distroless and Debian base images.
It preserves the released source and version. It does not create a GitHub release or move versioned image tags.

The workflow skips apps whose latest tag predates the monorepo cutover and reports an error.
Release these apps again from the monorepo before the workflow can refresh them.

### First-time monorepo cutover

Apps imported from their own repos (via `make import`) still carry only their original-repo tags, which point at pre-monorepo commits. To publish the first monorepo release for every imported app in one pass, use the idempotent helper:

```bash
scripts/cutover-release-apps.py            # dry run: print the per-app plan
scripts/cutover-release-apps.py --execute  # create and push the cutover tags
```

For each imported app, the helper computes a **minor version increase above its most recent tag from the original repo**.
For example, `v1.4.2` becomes `v1.5.0`. An app with no prior tag starts at `v0.1.0`.
The helper pushes `apps/<app>/v<bumped>` at `HEAD` to trigger the release workflow.
It skips the in-repo starter, `golang-starter`.

You can safely repeat the command because it skips apps whose target release already exists.
This behavior lets you iterate and fix release issues.
Use `--retry-failed` to trigger apps again when their tags exist but their releases did not complete.
Use `--preflight` to run `make release-test APP=<app>` before the helper tags each app.
Run `scripts/cutover-release-apps.py --help` for all options.
