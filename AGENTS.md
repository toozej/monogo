# Repository Guidelines

## Project Structure

- **Module:** `github.com/toozej/monogo`
- **Apps:** `apps/<app>` contains app code, such as `main.go`, `cmd/<app>`, and `internal`.
- **Shared packages:** root `pkg/` contains public packages that apps share.
- **Generated config:** each app directory contains Dockerfiles, `docker-compose.yml`, `.air.toml`, and `.goreleaser.yml`.
- **Shared tools:** the root contains `Makefile`, `.pre-commit-config.yaml`, scripts, and GitHub Actions workflows.

Templates in `templates/app/*.tmpl` generate Docker and GoReleaser files.
Templates in `templates/common/*.tmpl` generate app-local Air and Compose files and root files such as `.env.sample`.
Keep `.devcontainer/` as root config that Git tracks.

The manifest in `tools/go-tools.tsv` pins the versions of Go tools.
Make targets install each tool independently into `.tools/bin`.

## Build and Test Commands

- Use `mktemp -d` with `trap` to automatically remove temporary directories after you build and test this repo.
- Generate configs: `make generate-all`
- List apps: `make list-apps`
- Test the default app: `make test`
- Test a specific app: `make test APP=golang-starter`
- Mutation-test a specific app: `make mutation-test APP=golang-starter`
- Build a local binary: `make local-build APP=golang-starter`
- Check GoReleaser config and build a snapshot: `make release-test APP=golang-starter`
- Run pre-commit: `make pre-commit-run`
- Update pinned Go tools and pre-commit hooks: `make pre-commit-update`

`APP` defaults to `golang-starter`.
**Use `golang-starter` to validate any change that affects all apps.**
This requirement applies to `templates/app/*.tmpl`, `templates/common/*.tmpl`, root `scripts/`, the `Makefile`, and the release workflow.
The starter app has no app-specific dependencies and builds fastest, so it gives the quickest signal about shared changes.

1. Validate the starter app first, for example with `make release-test APP=golang-starter`.
2. For changes to packages or releases, also build a snapshot with `goreleaser release --snapshot`.
3. Apply the change to all apps with `make generate-all`.

## Add or Update Apps

1. Add app code under `apps/<app>`.
2. Add `apps/<app>/app.yaml` with the app's name, binary, source paths, build images, and description.
3. Run `make app-generate APP=<app>`.
4. Add the app to the matrices in `.github/workflows/ci.yaml` and `weekly-docker-refresh.yaml`.
5. Run `make test APP=<app>`.
6. Run `make release-test APP=<app>`.

See [docs/app-configuration.md](docs/app-configuration.md) for the full field reference.
The release workflow uses tags and has no app matrix.
Prefer app-specific metadata in `app.yaml` over direct edits to generated files.
Put shared Go packages in root `pkg/`.
Put app-private code in `apps/<app>/internal`.
Put changes to shared builds or root tools in `templates/app/*.tmpl` or `templates/common/*.tmpl`.

### `app.yaml`

App metadata and build options live in `apps/<app>/app.yaml`.
The [field reference](docs/app-configuration.md) describes the optional `cgoEnabled`, `runtimeImage`, and `port` fields.
It also explains how CGO changes runtime images and GoReleaser config.

When you change the schema, keep these files consistent:

- The gomplate templates: `templates/app/Dockerfile*.tmpl` and `.goreleaser.yml.tmpl`.
- The `Makefile`, particularly `APP_CGO_ENABLED`.
- The `.devcontainer/Dockerfile`.

## Import Apps

Import a service with `make import APP=owner/repo`.
The `owner/repo` form defaults to `github.com/owner/repo`.
Use `APP=vcs.example.com/owner/repo` for a different VCS hostname.
The import target uses `git subtree add` without a squash, so it requires a clean, committed monorepo baseline.

The importer puts source tags under `refs/tags/apps/<app>/<source-tag>`.
It writes GitHub release metadata under `apps/<app>/.monogo/releases`.
It downloads release assets only with `IMPORT_RELEASES=assets`.

The importer moves imported `pkg/` contents to root `pkg/`.
If a root package name already exists, the importer puts that package under `pkg/<app>/...` and rewrites imports to that path.
After import, inspect `apps/<app>/app.yaml` for the selected `mainPath`, `binary`, and description.

## Releases

Each app has independent releases.
Push an `apps/<app>/vX.Y.Z` tag to trigger a release.
Use `make release APP=<app> TYPE=<major|minor|patch>` to increase the version and push the tag.
The CI workflow in `.github/workflows/release.yaml` builds, signs, and publishes binaries, archives, Docker images, and the Homebrew cask.

### Delete a bad release

Use `make delete-release APP=<app> VERSION=vX.Y.Z` to remove the GitHub release and delete its tag locally and on `origin`.
You must pass `VERSION` explicitly.

### Reproducible archives

Archives for each architecture are byte-for-byte reproducible across builds.
The build uses `-trimpath` and sets `mod_timestamp` to the commit timestamp.
The `scripts/normalize-archive-mtimes.sh` script sets the mtimes of bundled files to that timestamp.
This GoReleaser `before` hook must run last.
The build also uses `gzip -n` for manpages.

The `Darwin_all` universal binary is the only exception.
GoReleaser merges architectures in nondeterministic order, so its content is not reproducible.
The build still pins its mtime.
This exception is harmless because each release comes from a single build.

### Homebrew cask

The same GoReleaser build produces the archives and renders the cask.
The `skip_upload: "true"` option stops that build from a push to the tap.
After the GitHub release assets exist, the `publish_homebrew_cask` job commits the rendered cask to `toozej/homebrew-tap`.
The job uses `scripts/publish-homebrew-cask.sh`.

Because the cask and uploaded archives come from one build, the cask's SHA-256 always matches the downloaded asset.
Do **not** run GoReleaser again in a second job to publish the cask.
A new build on another runner reintroduces checksum mismatches.

## Style and Tools

- Format Go code with `go fmt`.
- Organize imports with `goimports`.
- Regenerate files with gomplate before you commit template or `app.yaml` changes.
- Use Make targets from the repository root instead of direct tool invocations when possible.
- Do not commit `.env`, Cosign private keys, generated release artifacts, coverage output, or local binaries.
