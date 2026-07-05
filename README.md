# golang-starter

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/monogo)
[![Go Report Card](https://goreportcard.com/badge/github.com/toozej/monogo)](https://goreportcard.com/report/github.com/toozej/monogo)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml)
![GitHub Actions Release Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/release.yaml)
![GitHub Actions Weekly Docker Refresh Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/weekly-docker-refresh.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/monogo)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/monogo/total)

<img src="img/avatar.png" alt="monogo avatar" style="background-color: #FFFFFF;" />

| App | Badges | Description |
| --- | --- | --- |
| [`golang-starter`](apps/golang-starter/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/monogo) ![Downloads](https://img.shields.io/github/downloads/toozej/monogo/total) | Starter app for this Go monorepo. |
| [`files2prompt`](apps/files2prompt/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/files2prompt/cicd.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/files2prompt) ![Downloads](https://img.shields.io/github/downloads/toozej/files2prompt/total) | LLM prompt generator from local files. |
| [`ghreleases2rss`](apps/ghreleases2rss/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/ghreleases2rss/cicd.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/ghreleases2rss) ![Downloads](https://img.shields.io/github/downloads/toozej/ghreleases2rss/total) | Subscribe to GitHub project releases in an RSS reader. |
| [`go-find-liquor`](apps/go-find-liquor/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/go-find-liquor/cicd.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/go-find-liquor) ![Downloads](https://img.shields.io/github/downloads/toozej/go-find-liquor/total) | Go Find Liquor in Oregon. |
| [`go-listen`](apps/go-listen/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/go-listen/cicd.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/go-listen) ![Downloads](https://img.shields.io/github/downloads/toozej/go-listen/total) | Web app that searches for artists and adds their top songs to Spotify playlists. |
| [`go-sort-out-gh-actions`](apps/go-sort-out-gh-actions/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/go-sort-out-gh-actions/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/go-sort-out-gh-actions) ![Downloads](https://img.shields.io/github/downloads/toozej/go-sort-out-gh-actions/total) | Finds outdated GitHub Actions runtimes and can notify, update, pin, or create issues. |
| [`gotts-it`](apps/gotts-it/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/gotts-it/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/gotts-it) ![Downloads](https://img.shields.io/github/downloads/toozej/gotts-it/total) | Go-based text-to-speech tool. |
| [`kmhd2playlist`](apps/kmhd2playlist/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/kmhd2playlist/cicd.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/kmhd2playlist) ![Downloads](https://img.shields.io/github/downloads/toozej/kmhd2playlist/total) | Syncs KMHD jazz radio songs to Spotify playlists. |
| [`lego-stego`](apps/lego-stego/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/lego-stego/ci.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/lego-stego) ![Downloads](https://img.shields.io/github/downloads/toozej/lego-stego/total) | Go-based CLI tool for steganography. |
| [`notes2ssg`](apps/notes2ssg/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/notes2ssg/cicd.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/notes2ssg) ![Downloads](https://img.shields.io/github/downloads/toozej/notes2ssg/total) | Converts Memos or Simplenote notes into Hugo-compatible files. |
| [`photos2map`](apps/photos2map/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/photos2map/cicd.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/photos2map) ![Downloads](https://img.shields.io/github/downloads/toozej/photos2map/total) | Generates GPX maps from photo EXIF data. |
| [`podgrab`](apps/podgrab/README.md) | ![Build Status](https://img.shields.io/github/actions/workflow/status/toozej/podgrab/build.yml?branch=main) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/podgrab) ![Downloads](https://img.shields.io/github/downloads/toozej/podgrab/total) | Self-hosted podcast manager, downloader, and archiver with an integrated player. |
| [`RSSFFS`](apps/RSSFFS/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/RSSFFS/cicd.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/rssffs) ![Downloads](https://img.shields.io/github/downloads/toozej/RSSFFS/total) | RSS Feed Finder and Subscriber. |
| [`rss2socials`](apps/rss2socials/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/rss2socials/cicd.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/rss2socials) ![Downloads](https://img.shields.io/github/downloads/toozej/rss2socials/total) | Watches RSS feeds for new posts and announces them on Mastodon. |
| [`terranotate`](apps/terranotate/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/terranotate/cicd.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/terranotate) ![Downloads](https://img.shields.io/github/downloads/toozej/terranotate/total) | Terraform comment parser and validator. |
| [`trails-completionist`](apps/trails-completionist/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/trails-completionist/cicd.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/trails-completionist) ![Downloads](https://img.shields.io/github/downloads/toozej/trails-completionist/total) | Searchable trail completion tracker. |
| [`url2anki`](apps/url2anki/README.md) | ![CI](https://img.shields.io/github/actions/workflow/status/toozej/url2anki/cicd.yaml) ![Docker Pulls](https://img.shields.io/docker/pulls/toozej/url2anki) ![Downloads](https://img.shields.io/github/downloads/toozej/url2anki/total) | Generate Anki flashcards from a URL. |

`golang-starter` is the starter app in this Go monorepo. It keeps shared repository concerns at the root while each app lives under `apps/<app>`.

The root `go.mod`, `go.sum`, `.pre-commit-config.yaml`, `.devcontainer/`, workflows, scripts, Makefile, and `pkg/` packages are shared. App-local Docker and GoReleaser files are generated from `templates/app`; app-local Air and Compose files plus root shared files such as `.env.sample` are generated from `templates/common`.

## Common Commands

```bash
make list-apps
make generate-all
make test APP=golang-starter
make local-build APP=golang-starter
make release-test APP=golang-starter
```

`APP` defaults to `golang-starter`, so `make test` and `make local-build` work for the starter app.

## Generated Configs

Install prerequisites and regenerate configs:

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

See [docs/app-configuration.md](docs/app-configuration.md) for the full `app.yaml` field reference, including CGO builds (`cgoEnabled`), runtime image selection (`runtimeImage`), and exposed ports (`port`).

## Create a New App

Run the scaffold script to clone the `golang-starter` app into a new app directory and wire it into CI:

```bash
make new-app APP=mytool
```

The script (`scripts/create-new-app.py`) copies `apps/golang-starter` to `apps/mytool`, renames command packages, rewrites imports, updates `app.yaml`, amends the CI app matrix, and appends the Dependabot Docker entry. It finishes by running `go mod tidy` and `make app-generate APP=mytool` so the generated configs are committed.

After scaffolding:

1. Edit `apps/mytool/app.yaml` to describe the service and tweak build settings.
2. Replace `internal/starter` and extend `cmd/mytool` with real functionality.
3. Run `make test APP=mytool` and `make local-build APP=mytool` to verify everything compiles.
4. Commit the new files; CI and Dependabot already include the app once the script completes.

### Remove an App

```bash
make delete-app APP=mytool
```

The deletion helper (`scripts/delete-app.py`) removes `apps/mytool`, prunes generated artefacts, and updates shared automation like the CI matrix and Dependabot Docker entries before running `go mod tidy`. Review `git status` afterward to ensure any extra references (docs, dashboards, secret stores) are cleaned up.

## Import Existing Services

Import an existing Go service with its Git history. `APP=owner/repo` defaults to `github.com/owner/repo`; include a hostname to import from another VCS host:

```bash
make import APP=toozej/go-listen
make import APP=github.com/toozej/go-listen
```

The import target:

1. Imports the source repository with `git subtree add` without squashing history.
2. Moves the imported app under `apps/<repo>`.
3. Merges the imported `go.mod` requirements into the root `go.mod`.
4. Rewrites imports from the old module path to `github.com/toozej/monogo/apps/<repo>`.
5. Moves imported `pkg/` packages to root `pkg/`, preserving app-private code under `apps/<repo>/internal`.
6. Stores the original app `go.mod`, `go.sum`, and build config under `apps/<repo>/.monogo/`.
7. Imports source tags under `refs/tags/apps/<repo>/`.
8. Imports GitHub release metadata under `apps/<repo>/.monogo/releases/`.
9. Generates app-local Docker, Compose, Air, and GoReleaser config with gomplate.
10. Updates GitHub Actions app matrices for CI and Docker refresh, plus Dependabot Docker config.
11. Runs tests, local build, and GoReleaser snapshot validation.

Options:

```bash
make import APP=toozej/go-listen IMPORT_REF=main
make import APP=toozej/go-listen IMPORT_NAME=listen
make import APP=toozej/go-listen IMPORT_RELEASES=assets
make import APP=toozej/go-listen IMPORT_RELEASES=none
make import APP=toozej/go-listen IMPORT_SKIP_VERIFY=1
make import APP=github.example.com/owner/repo
```

`make import` requires a clean, committed starter-app baseline before it runs. GitHub release records are not part of Git history, so the target imports release metadata with `gh`; use `IMPORT_RELEASES=assets` to download release assets too.

## Release Model

This repository uses GoReleaser OSS with independent app release tags:

```bash
git tag apps/url2anki/v0.1.0
git push origin apps/url2anki/v0.1.0
```

Only the tagged app is released. The workflow creates a local-only clean `vX.Y.Z` tag for GoReleaser version parsing, disables GoReleaser's SCM release creation, and then creates the real GitHub release at `apps/<app>/vX.Y.Z` with `gh`.

For local release-config testing, and to cut a release:

```bash
make release-test APP=url2anki
make release APP=url2anki TYPE=patch
```

`make release-test` checks the GoReleaser config and builds a snapshot locally. `make release` runs the app's tests, computes the next `apps/<app>/vX.Y.Z` tag by bumping `TYPE` (`major`, `minor`, or `patch`; defaults to `patch`) from the latest existing tag, then creates and pushes that tag; CI builds, signs, and publishes the release. Tagging and pushing by hand (`git tag -a apps/<app>/vX.Y.Z -m ... && git push origin apps/<app>/vX.Y.Z`) triggers the same workflow.
