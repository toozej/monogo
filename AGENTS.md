# Repository Guidelines

## Project Structure

- **Module:** `github.com/toozej/monogo`
- **Apps:** `apps/<app>` contains app code, including `main.go`, `cmd/<app>`, and `internal`.
- **Shared packages:** root `pkg/` contains public packages shared across apps.
- **Generated config:** Dockerfiles, `docker-compose.yml`, `.air.toml`, and `.goreleaser.yml` live in each app directory. Docker and GoReleaser files are generated from `templates/app/*.tmpl`; app-local Air and Compose files plus root shared files such as `.env.sample` are generated from `templates/common/*.tmpl`. Keep `.devcontainer/` as checked-in root config.
- **Shared tooling:** root `Makefile`, `go.mod`, `go.sum`, `.pre-commit-config.yaml`, scripts, and GitHub Actions workflows.

## Build and Test Commands

- Generate configs: `make generate-all`
- List apps: `make list-apps`
- Test default app: `make test`
- Test a specific app: `make test APP=monogo`
- Build a local binary: `make local-build APP=monogo`
- Check GoReleaser config and snapshot build: `make local-release-test APP=monogo`
- Run pre-commit: `make pre-commit-run`

## Adding or Updating Apps

1. Add app code under `apps/<app>`.
2. Add `apps/<app>/app.yaml` with the app's name, binary, source paths, build images, and description (see [docs/app-configuration.md](docs/app-configuration.md) for the full field reference).
3. Run `make app-generate APP=<app>`.
4. Add the app to the matrices in `.github/workflows/ci.yaml`, `release.yaml`, and `weekly-docker-refresh.yaml`.
5. Run `make test APP=<app>` and `make local-release-test APP=<app>`.

Prefer app-specific metadata in `app.yaml` over editing generated files directly. Shared Go packages belong in root `pkg/`; app-private code belongs in `apps/<app>/internal`. Shared build and root tooling changes belong in `templates/app/*.tmpl` or `templates/common/*.tmpl`.

### `app.yaml`

App metadata and build options live in `apps/<app>/app.yaml`. See [docs/app-configuration.md](docs/app-configuration.md) for the full field reference, including the optional `cgoEnabled`, `runtimeImage`, and `port` fields and how CGO changes the runtime images and GoReleaser config. When changing the schema, keep the gomplate templates (`templates/app/Dockerfile*.tmpl`, `.goreleaser.yml.tmpl`), the `Makefile` (`APP_CGO_ENABLED`), and `.devcontainer/Dockerfile` in sync.

## Importing Apps

- Import an existing service with `make import APP=owner/repo`; `owner/repo` defaults to `github.com/owner/repo`, and `APP=vcs.example.com/owner/repo` sets a different VCS hostname.
- The import target uses a non-squashed `git subtree add`, so the monorepo must have a clean committed baseline before importing.
- Imported source tags are namespaced as `refs/tags/apps/<app>/<source-tag>`.
- GitHub release metadata is written under `apps/<app>/.monogo/releases`; release assets are downloaded only with `IMPORT_RELEASES=assets`.
- Imported `pkg/` contents are moved to root `pkg/`. If a root package name already exists, the imported package is placed under `pkg/<app>/...` and imports are rewritten to that path.
- After import, inspect `apps/<app>/app.yaml` for the selected `mainPath`, `binary`, and description.

## Style and Tooling

- Keep Go code formatted with `go fmt` and imports organized by `goimports`.
- Keep generated files current with gomplate before committing template or `app.yaml` changes.
- Use root Make targets instead of direct tool invocations when possible.
- Do not commit `.env`, Cosign private keys, generated release artifacts, coverage output, or local binaries.
