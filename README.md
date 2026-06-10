# golang-starter

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/golang-starter)
[![Go Report Card](https://goreportcard.com/badge/github.com/toozej/golang-starter)](https://goreportcard.com/report/github.com/toozej/golang-starter)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/golang-starter/ci.yaml)
![GitHub Actions Release Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/golang-starter/release.yaml)
![GitHub Actions Weekly Docker Refresh Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/golang-starter/weekly-docker-refresh.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/golang-starter)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/golang-starter/total)

Golang starter template

## features of this starter template
- follows common Golang best practices in terms of repo/project layout, and includes explanations of what goes where in README files
- Cobra library for CLI handling, Logrus for logging, and GoDotEnv and Env libraries for reading config files already plugged in and ready to expand upon
- Goreleaser to build Docker images and most standard package types across Linux, MacOS and Windows
    - also includes auto-generated manpages and shell autocompletions
    - Docker images are signed with Cosign v3 via GoReleaser `docker_signs` using GitHub Actions OIDC certificates (keyless), and should be verified with `cosign verify` using `--certificate-identity-regexp` and `--certificate-oidc-issuer https://token.actions.githubusercontent.com` (not `--key`)
- Makefile for easy building, deploying, testing, updating, etc. both Dockerized and using locally installed Golang toolchain
- docker-compose project for easily hosting built Dockerized Golang project, with optional support for Golang web services
- scripts to make using the starter template easy, and to update the Golang version when a new one comes out
- Dev Container with built in Go-related VSCode extensions, and [llm](https://llm.datasette.io/) tool + plugins pre-configured to use GitHub Copilot
- built-in security scans, vulnerability warnings and auto-updates via Dependabot and GitHub Actions
- auto-generated documentation
- pre-commit hooks for ensuring formatting, linting, security checks, etc.

## changes required to use this as a starter template
- generate a GitHub fine-grained access token from https://github.com/settings/tokens?type=beta (used in repo as "GITHUB_TOKEN" and in GitHub Actions Secrets as "GH_TOKEN") with the following read/write permissions:
    - actions
    - attestations
    - code scanning alerts
    - commit statuses
    - contents
    - dependabot alerts
    - dependabot secrets
    - deployments
    - environments
    - issues
    - pages
    - pull requests
    - repository security advisories
    - secret scanning alerts
    - secrets
    - webhooks
    - workflows
- run `use_starter.sh` script to rename project files, generate Cosign artifacts, gather and upload secrets to GitHub Actions, etc.
    - run `./scripts/use_starter.sh $NEW_PROJECT_NAME_GOES_HERE`
    - to rename with a different GitHub username `./scripts/use_starter.sh $NEW_PROJECT_NAME_GOES_HERE $GITHUB_USERNAME_GOES_HERE`
- set up new repository in quay.io
    - (DockerHub and GitHub Container Registry do this automatically on first push/publish)
    - `use_starter.sh` automates Quay.io repo creation and robot permissions via `scripts/create_quay_repo.sh`
    - this requires a Quay.io **OAuth 2.0 Access Token** (stored as `QUAY_OAUTH_TOKEN` in `.env` / GitHub Actions Secrets):
        1. Log in to [quay.io](https://quay.io) and navigate to your **Account Settings** (or **Organization Settings** for org-owned repos)
        2. Click **Applications** in the left sidebar
        3. Click **Create New Application** and give it a name (e.g. `repo-automation`)
        4. Click the newly created application, then click **Generate Token** in the left sidebar
        5. Select the following permission scopes:
            - `Create Repositories` (`repo:create`)
            - `Administer Repositories` (`repo:admin`)
        6. Click **Generate Access Token**, copy the token, and save it as `QUAY_OAUTH_TOKEN` in your `.env` file
        - **Note:** `QUAY_OAUTH_TOKEN` is distinct from `QUAY_TOKEN` (the robot account password used by Docker login). The OAuth token is a user/org-level token needed for API operations; the robot token is used for image push/pull auth.
        - **Note:** Robot account usernames follow the format `namespace+robotname` (e.g. `toozej+github_builder`). If your `QUAY_USERNAME` is in this format, `create_quay_repo.sh` will automatically extract the namespace and robot name.
- set built packages visibility in GitHub packages to public
    - navigate to https://github.com/users/$USERNAME/packages/container/$REPO/settings
    - scroll down to "Danger Zone"
    - change visibility to public

## changes required to update golang version
- `make update-golang-version`
