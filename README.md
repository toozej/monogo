# monogo

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/monogo)
[![Go Report Card](https://goreportcard.com/badge/github.com/toozej/monogo)](https://goreportcard.com/report/github.com/toozej/monogo)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml)
![GitHub Actions Release Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/release.yaml)
![GitHub Actions Weekly Docker Refresh Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/weekly-docker-refresh.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/monogo)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/monogo/total)

`monogo` is a Go monorepo based on the `golang-starter` template. It keeps shared repository concerns at the root while each app lives under `apps/<app>`.

The root `go.mod`, `go.sum`, `.pre-commit-config.yaml`, `.devcontainer/`, workflows, scripts, Makefile, and `pkg/` packages are shared. App-local Docker and GoReleaser files are generated from `templates/app`; app-local Air and Compose files plus root shared files such as `.env.sample` are generated from `templates/common`.

## Common Commands

```bash
make list-apps
make generate-all
make test APP=monogo
make local-build APP=monogo
make local-release-test APP=monogo
```

`APP` defaults to `monogo`, so `make test` and `make local-build` work for the initial app.

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
4. Add the app to the GitHub Actions matrices in `.github/workflows/*.yaml`.

For normal per-app differences, change values in `apps/<app>/app.yaml`. Shared Go packages belong in root `pkg/`; app-private code belongs in `apps/<app>/internal`. For shared build or root tooling behavior, update `templates/app/*.tmpl` or `templates/common/*.tmpl` and run `make generate-all`.

See [docs/app-configuration.md](docs/app-configuration.md) for the full `app.yaml` field reference, including CGO builds (`cgoEnabled`), runtime image selection (`runtimeImage`), and exposed ports (`port`).

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
10. Updates GitHub Actions app matrices and Dependabot Docker config.
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

`make import` requires a clean, committed monogo baseline before it runs. GitHub release records are not part of Git history, so the target imports release metadata with `gh`; use `IMPORT_RELEASES=assets` to download release assets too.

## Release Model

This repository uses GoReleaser OSS with lockstep repository tags:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release workflow runs one GoReleaser job per app from the generated app-local `.goreleaser.yml`. This avoids GoReleaser Pro monorepo tag-prefix features while preserving the starter repository's Make, Docker, signing, packaging, and GoReleaser patterns.
