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
- Test a specific app: `make test APP=golang-starter`
- Build a local binary: `make local-build APP=golang-starter`
- Check GoReleaser config and snapshot build: `make release-test APP=golang-starter`
- Run pre-commit: `make pre-commit-run`

`APP` defaults to `golang-starter`. **Use `golang-starter` to validate any change that affects all apps** — edits to `templates/app/*.tmpl`, `templates/common/*.tmpl`, root `scripts/`, the `Makefile`, or the release workflow. It is the minimal in-repo starter app (no app-specific dependencies, fastest to build), so it is the quickest signal a repo-wide change is safe: exercise it first (e.g. `make release-test APP=golang-starter`, and for packaging/release changes a `goreleaser release --snapshot` build), then apply the change everywhere with `make generate-all`.

## Adding or Updating Apps

1. Add app code under `apps/<app>`.
2. Add `apps/<app>/app.yaml` with the app's name, binary, source paths, build images, and description (see [docs/app-configuration.md](docs/app-configuration.md) for the full field reference).
3. Run `make app-generate APP=<app>`.
4. Add the app to the matrices in `.github/workflows/ci.yaml` and `weekly-docker-refresh.yaml`. The release workflow is tag-driven and does not have an app matrix.
5. Run `make test APP=<app>` and `make release-test APP=<app>`.

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

## Releases

- Releases are per-app, triggered by pushing an `apps/<app>/vX.Y.Z` tag. Use `make release APP=<app> TYPE=<major|minor|patch>` to bump and push; CI (`.github/workflows/release.yaml`) builds, signs, and publishes binaries/archives, Docker images, and the Homebrew cask.
- **Delete a bad release:** `make delete-release APP=<app> VERSION=vX.Y.Z` removes the GitHub release and deletes the tag both locally and on `origin`. `VERSION` must be passed explicitly.
- **Reproducible archives:** per-arch archives are byte-for-byte reproducible across builds — `-trimpath` + commit-pinned `mod_timestamp`, `scripts/normalize-archive-mtimes.sh` (a GoReleaser `before` hook, which must run last, pinning bundled files' mtimes to the commit timestamp), and `gzip -n` manpages. The `Darwin_all` universal binary is the one exception (GoReleaser merges arches in nondeterministic order); its mtime is pinned but its content is not reproducible. This is harmless because each release comes from a single build.
- **Homebrew cask:** the cask is rendered by the same GoReleaser build that produces the archives — `skip_upload: "true"` keeps that build from pushing it — and committed to `toozej/homebrew-tap` by the `publish_homebrew_cask` job via `scripts/publish-homebrew-cask.sh`, only after the GitHub release assets exist. Because the cask and the uploaded archives come from one build, the cask's SHA-256 always matches the downloaded asset. Do **not** re-run GoReleaser in a second job to publish the cask — rebuilding on another runner reintroduces checksum mismatches.

## Style and Tooling

- Keep Go code formatted with `go fmt` and imports organized by `goimports`.
- Keep generated files current with gomplate before committing template or `app.yaml` changes.
- Use root Make targets instead of direct tool invocations when possible.
- Do not commit `.env`, Cosign private keys, generated release artifacts, coverage output, or local binaries.
