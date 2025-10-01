# golang-starter

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/golang-starter)
[![Go Report Card](https://goreportcard.com/badge/github.com/toozej/golang-starter)](https://goreportcard.com/report/github.com/toozej/golang-starter)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/golang-starter/cicd.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/golang-starter)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/golang-starter/total)

Golang starter template

## features of this starter template
- follows common Golang best practices in terms of repo/project layout, and includes explanations of what goes where in README files
- Cobra library for CLI handling, Logrus for logging, and GoDotEnv and Env libraries for reading config files already plugged in and ready to expand upon
- Goreleaser to build Docker images and most standard package types across Linux, MacOS and Windows
    - also includes auto-generated manpages and shell autocompletions
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
- set up new repository in quay.io web console
    - (DockerHub and GitHub Container Registry do this automatically on first push/publish)
    - name must match Git repo name
    - grant robot user with username stored in QUAY_USERNAME "write" permissions (your quay.io account should already have admin permissions)
- set built packages visibility in GitHub packages to public
    - navigate to https://github.com/users/$USERNAME/packages/container/$REPO/settings
    - scroll down to "Danger Zone"
    - change visibility to public

## changes required to update golang version
- `make update-golang-version`
