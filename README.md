# golang-starter

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/monogo)
[![Go Report Card](https://goreportcard.com/badge/github.com/toozej/monogo)](https://goreportcard.com/report/github.com/toozej/monogo)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml)
![GitHub Actions Release Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/release.yaml)
![GitHub Actions Weekly Docker Refresh Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/weekly-docker-refresh.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/monogo)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/monogo/total)

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
