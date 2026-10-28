---
name: backfill-template
description: Back-fills template updates and improvements from the golang-starter template repository into target projects, keeping them consistent, updated, and passing builds.
---

# Apply Template Updates from golang-starter

This skill transfers scaffold changes, build pipelines, and configuration updates from the `golang-starter` template repository to downstream projects.
Keep downstream projects consistent with the latest optimizations, security practices, and workflow changes.
Preserve all downstream-specific functionality.

## Core Guidelines

1. Inspect the diffs carefully.
2. Merge template improvements into current files.
3. Preserve customized behavior in the target repository.
4. Never rename target Dockerfiles.
5. Match each target Dockerfile to the closest template by content structure.
6. Review differences before you apply improvements.
7. Preserve custom packages, ports, and entrypoints.
8. Verify that the project builds and passes checks after your changes.
9. Validate with `make local` for local Go checks.
10. Validate with `make all` for Docker workflow checks.
11. Stage changes with `git add`.
12. Write a clear commit message that describes the transferred changes.
13. Commit locally.
14. Do **not** push to the remote.

Target repositories possibly renamed, added, or deleted Dockerfiles.
Template improvements can include version updates, extra lint checks, newer workflow actions, nonroot users, and shell flags.

## Step 1: Compare the Repositories

Run the helper Python script from the target repository's root:

```bash
python3 <path_to_skill_folder>/scripts/compare.py
```

The script fetches the latest `golang-starter` template and compares it with the current target repository.
It substitutes the placeholders `golang-starter` and `toozej` with the target project's name and owner.
It also computes structural similarities between Dockerfiles.

For a template test or a local template clone, pass `--template-dir <path_to_template_dir>`.

The script produces these results:

- **[ADD]**: Files in the template that are absent from the target, such as new linter config or pre-commit hooks.
- **[MODIFIED]**: Unified diffs between target files and customized template files.
- **Dockerfile Matches**: The most similar template for each target Dockerfile, with a similarity ratio and diff.

## Step 2: Apply Scaffold and Configuration Diffs

For `.github/workflows/`:

- Apply the new steps, updated action versions such as `actions/checkout@v4`, and optimizations.
- Preserve target-specific environment variables, secrets, and steps.
- Ignore minor variations in cron minutes. Random minute selection distributes load.

For `Makefile`:

- Merge new targets, build arguments, and linter integrations.
- Preserve custom targets, build tags, and special compiler flags.

For `.goreleaser.yml`:

- Merge improved release stages, SBOM generation, and signature logic.
- Preserve binary names, custom archive formats, and tap configuration.

For config and version boilerplate in `pkg/version/` and `pkg/config/`:

- Update logic to parse versions or standard config loaders.
- Check that Go package paths match the target module path.

## Step 3: Update Dockerfiles

1. Identify the matched template for each target Dockerfile, for example `Dockerfile.distroless`.
2. Review the diff.
3. Apply the template improvements to the target Dockerfile.
4. Preserve the target Dockerfile's filename.
5. Preserve the custom `apt-get` packages, ports, volume mounts, entrypoints, and arguments.

Typical improvements include:

- Updates to the builder Go version, for example `FROM golang:1.26-bookworm`.
- Updates to runtime base images, for example `FROM gcr.io/distroless/static-debian13`.
- Security configuration, such as `SHELL ["/bin/bash", "-o", "pipefail", "-c"]` or `USER nonroot`.
- Improved caches and builds with multiple stages.

## Step 4: Verify the Changes

Run the local workflow:

```bash
make local
```

This target runs `go mod tidy`, `go mod vendor`, `go vet`, and pre-commit hooks.
It runs tests with race detection, generates coverage, builds the local binary, and checks GoReleaser snapshots.
Check that all code compiles and all tests pass.

Run the Docker workflow:

```bash
make all
```

This target runs vet, tests, and builds with Docker.
It verifies image signatures with Cosign and starts the container.

A local `make all` builds a fresh, unsigned Docker image.
The `cosign verify` step will therefore fail with `Error: no signatures found`.
This failure follows from the unsigned local build.
Docker verification succeeds for this workflow if the `vet`, `pre-commit`, `test`, and `build` stages complete successfully.
To test the container manually, run:

```bash
docker run --rm --name <project_name> --env-file .env <owner>/<project_name>:latest
```

If any check fails, diagnose the build or test issue.
Refine the changes until the checks pass.

## Step 5: Commit Locally

After builds and checks succeed, stage the modified files:

```bash
git add .
```

Commit the changes locally:

```bash
git commit -m "chore: backfill updates from golang-starter template

- Bumped Go and base image versions in Dockerfiles
- Updated GitHub Action workflows to latest versions
- Applied Makefile improvements and linter updates
- Synced pre-commit hooks and goreleaser configuration"
```

Do **not** push changes to the remote repository.
Leave the result staged for manual review and a push by the developer.
