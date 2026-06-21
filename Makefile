# Set sane defaults for Make
SHELL = bash
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

# Set default goal such that `make` runs `make help`
.DEFAULT_GOAL := help

APPS := $(shell find apps -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort)
APP ?= monogo
PACKAGE ?=
APP_DIR = apps/$(APP)
APP_CONFIG = $(APP_DIR)/app.yaml
APP_BINARY = $(shell awk -F': *' '/^binary:/ {gsub(/"/, "", $$2); print $$2; exit}' $(APP_CONFIG) 2>/dev/null)
APP_MAIN_PATH = $(shell v=$$(awk -F': *' '/^mainPath:/ {gsub(/"/, "", $$2); print $$2; exit}' $(APP_CONFIG) 2>/dev/null); if test -n "$$v"; then echo "$$v"; else echo "$(APP_DIR)"; fi)
APP_CGO_ENABLED = $(shell v=$$(awk -F': *' '/^cgoEnabled:/ {gsub(/"/, "", $$2); print $$2; exit}' $(APP_CONFIG) 2>/dev/null); if [ "$$v" = "true" ] || [ "$$v" = "1" ]; then echo 1; else echo 0; fi)
APP_PACKAGES = ./$(APP_DIR)/... ./pkg/...
APP_ENV_FILE ?= $(APP_DIR)/.env
APP_COSIGN_KEY ?= $(APP_DIR)/$(APP_BINARY).key

# Build info
BUILDER = $(shell whoami)@$(shell hostname)
NOW = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Version control
VERSION = $(shell git describe --tags --exclude 'apps/*' --dirty --always 2>/dev/null || echo local)
COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BRANCH = $(shell branch=$$(git symbolic-ref --quiet --short HEAD 2>/dev/null || true); if test -n "$$branch"; then echo "$$branch"; else echo unknown; fi)

# Linker flags
PKG = $(shell head -n 1 go.mod | cut -c 8-)
VERSION_PACKAGE = $(PKG)/pkg/version
LDFLAGS = -s -w
LDFLAGS += \
	-X $(VERSION_PACKAGE).Version=$(or $(VERSION),unknown) \
	-X $(VERSION_PACKAGE).Commit=$(or $(COMMIT),unknown) \
	-X $(VERSION_PACKAGE).Branch=$(or $(BRANCH),unknown) \
	-X $(VERSION_PACKAGE).BuiltAt=$(NOW) \
	-X $(VERSION_PACKAGE).Builder=$(BUILDER)

# Docker image info
IMAGE_AUTHOR = toozej
IMAGE_NAME = $(APP_BINARY)
IMAGE_TAG = latest

# Detect the OS and architecture
OS := $(shell uname -s)
ARCH := $(shell uname -m)
LATEST_RELEASE_URL := https://github.com/toozej/monogo/releases/latest/download/$(APP_BINARY)_$(OS)_$(ARCH).tar.gz

ifeq ($(OS), Linux)
	OPENER=xdg-open
else
	OPENER=open
endif

COSIGN_IDENTITY_REGEXP := '^https://github.com/toozej/monogo/.github/workflows/(release|weekly-docker-refresh).yaml@refs/(tags/.*|heads/main)$$'
COSIGN_OIDC_ISSUER := 'https://token.actions.githubusercontent.com'

.PHONY: all list-apps import migrate-internal-package app-check common-generate app-generate generate generate-all app-templates-check templates-check vet test build release verify verify-docker verify-docker-all-registries run up down docker-vet docker-test docker-build distroless-build distroless-run install local local-all local-update-deps local-vet local-test local-cover local-build local-run local-kill local-iterate local-release-test local-release local-sign local-verify local-release-verify local-install docker-login pre-commit-install pre-commit-run pre-commit pre-reqs update-golang-version upload-secrets-to-gh upload-secrets-envfile-to-1pass docs diagrams mutation-test test-changed watch-test profile-cpu profile-mem profile-all benchmark clean clean-all help

all: local-all ## Run default workflow for every app using locally installed tools
local: app-check generate local-update-deps local-vet pre-commit clean local-test local-build local-release-test ## Run default workflow for APP using locally installed tools
local-all: generate-all ## Run local vet, test, build, and release checks for every app
	@for app in $(APPS); do $(MAKE) local APP=$$app; done
local-release-verify: local-release local-sign local-verify ## Release and verify APP using locally installed tools (keyless)

list-apps: ## List monorepo apps
	@printf '%s\n' $(APPS)

import: ## Import a Go service repo into apps/, preserving history and release metadata; usage: make import APP=[vcs-host/]owner/repo
	$(CURDIR)/scripts/import-app.sh "$(APP)"

migrate-internal-package: ## Move apps/APP/internal/PACKAGE to pkg/PACKAGE and verify affected apps; usage: make migrate-internal-package APP=monogo PACKAGE=starter
	$(CURDIR)/scripts/migrate-internal-package.sh "$(APP)" "$(PACKAGE)"

app-check: ## Validate APP points at a configured app
	@test -n "$(APP)" || (echo "APP is required, e.g. make test APP=monogo" && exit 1)
	@test -f "$(APP_CONFIG)" || (echo "No app config found at $(APP_CONFIG)" && exit 1)
	@test -n "$(APP_BINARY)" || (echo "No binary configured in $(APP_CONFIG)" && exit 1)

common-generate: pre-reqs app-check ## Generate root shared configs from templates/common
	$(CURDIR)/scripts/render-common-configs.sh $(APP)

app-generate: pre-reqs app-check ## Generate APP Docker, GoReleaser, Compose, and Air configs with gomplate
	$(CURDIR)/scripts/render-app-configs.sh $(APP)

generate: common-generate app-generate ## Generate root shared configs and APP configs

generate-all: pre-reqs ## Generate configs for every app
	$(MAKE) common-generate APP=$(APP)
	@for app in $(APPS); do $(MAKE) app-generate APP=$$app; done


app-templates-check: app-generate ## Render and check generated config for APP
	goreleaser check --config $(APP_DIR)/.goreleaser.yml

templates-check: pre-reqs ## Render and check generated config for every app
	@for app in $(APPS); do $(MAKE) app-templates-check APP=$$app; done

vet: local-vet ## Run go vet for APP

test: local-test ## Run go test for APP

build: docker-build ## Build Docker image for APP, including running tests

release: app-check generate docker-login ## Release APP assets using GoReleaser OSS
	if test -e $(CURDIR)/$(APP_ENV_FILE); then \
		export `cat $(CURDIR)/$(APP_ENV_FILE) | xargs`; \
		goreleaser release --clean --config $(APP_DIR)/.goreleaser.yml; \
	else \
		echo "No environment variables found at $(CURDIR)/$(APP_ENV_FILE). Cannot release."; \
	fi

verify: app-check ## Verify APP Docker image with Cosign (keyless)
	cosign verify \
		--certificate-identity-regexp $(COSIGN_IDENTITY_REGEXP) \
		--certificate-oidc-issuer $(COSIGN_OIDC_ISSUER) \
		$(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG)

verify-docker: verify ## Alias for verify

verify-docker-all-registries: app-check ## Verify APP Docker images on all registries with Cosign (keyless)
	@for registry in "" "ghcr.io/" "quay.io/"; do \
		echo "=== Verifying $${registry:-DockerHub} ===" && \
		cosign verify \
			--certificate-identity-regexp $(COSIGN_IDENTITY_REGEXP) \
			--certificate-oidc-issuer $(COSIGN_OIDC_ISSUER) \
			$${registry}$(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG); \
	done

docker-vet: app-check generate ## Run go vet for APP in Docker
	docker build --target vet -f $(CURDIR)/$(APP_DIR)/Dockerfile -t $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG) .

docker-test: app-check generate ## Run go test for APP in Docker
	docker build --progress=plain --target test -f $(CURDIR)/$(APP_DIR)/Dockerfile -t $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG) .

docker-build: app-check generate ## Build APP Docker image, including running tests
	docker build -f $(CURDIR)/$(APP_DIR)/Dockerfile \
		--build-arg VERSION=$(or $(VERSION),unknown) \
		--build-arg COMMIT=$(or $(COMMIT),unknown) \
		--build-arg BRANCH=$(or $(BRANCH),unknown) \
		--build-arg BUILT_AT=$(NOW) \
		--build-arg BUILDER=$(BUILDER) \
		-t $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG) .

run: app-check ## Run built APP Docker image
	-docker kill $(IMAGE_NAME)
	docker run --rm --name $(IMAGE_NAME) --env-file $(CURDIR)/.env $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG)

up: docker-test docker-build ## Run APP Docker Compose project with built image
	docker compose -f $(APP_DIR)/docker-compose.yml down --remove-orphans
	docker compose -f $(APP_DIR)/docker-compose.yml pull
	docker compose -f $(APP_DIR)/docker-compose.yml up -d

down: app-check ## Stop APP Docker Compose project
	docker compose -f $(APP_DIR)/docker-compose.yml down --remove-orphans

distroless-build: app-check generate ## Build APP Docker image using distroless as final base
	docker build -f $(CURDIR)/$(APP_DIR)/Dockerfile.distroless \
		--build-arg VERSION=$(or $(VERSION),unknown) \
		--build-arg COMMIT=$(or $(COMMIT),unknown) \
		--build-arg BRANCH=$(or $(BRANCH),unknown) \
		--build-arg BUILT_AT=$(NOW) \
		--build-arg BUILDER=$(BUILDER) \
		-t $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG)-distroless .

distroless-run: app-check ## Run built APP Docker image using distroless as final base
	docker run --rm --name $(IMAGE_NAME) -v $(CURDIR)/config:/config $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG)-distroless

install: app-check ## Install APP from latest GitHub release
	if command -v go; then \
		go install $(PKG)/$(APP_MAIN_PATH)@latest ; \
	else \
		echo "Downloading $(APP_BINARY) binary for $(OS)-$(ARCH)..."; \
		mkdir -p $(CURDIR)/tmp; \
		curl --silent -L -o $(CURDIR)/tmp/$(APP_BINARY).tgz $(LATEST_RELEASE_URL); \
		tar -xzf $(CURDIR)/tmp/$(APP_BINARY).tgz -C $(CURDIR)/tmp/; \
		chmod +x $(CURDIR)/tmp/$(APP_BINARY); \
		sudo mv $(CURDIR)/tmp/$(APP_BINARY) /usr/local/bin/$(APP_BINARY); \
		rm -rf $(CURDIR)/tmp; \
	fi

local-update-deps: ## Run `go get -t -u ./...` to update Go module dependencies
	go get -t -u ./...

local-vet: app-check ## Run goimports and go vet for APP
	command -v goimports || go install golang.org/x/tools/cmd/goimports@latest
	goimports -w $(APP_DIR) pkg
	go vet $(APP_PACKAGES)

local-vendor: ## Run `go mod tidy & vendor` using locally installed golang toolchain
	go mod tidy
	go mod vendor

local-test: app-check ## Run go test for APP
	go test -race -coverprofile $(APP_DIR)/c.out -v $(APP_PACKAGES)
	@echo -e "\nStatements missing coverage"
	@grep -e " 0$$" $(APP_DIR)/c.out || true

local-cover: app-check ## View APP coverage report in web browser
	go tool cover -html=$(APP_DIR)/c.out

local-build: app-check ## Build APP using locally installed golang toolchain
	CGO_ENABLED=$(APP_CGO_ENABLED) go build -o $(CURDIR)/out/$(APP_BINARY) -ldflags="$(LDFLAGS)" ./$(APP_MAIN_PATH)

local-run: app-check ## Run locally built APP binary
	if test -e $(CURDIR)/.env; then \
		export `cat $(CURDIR)/.env | xargs` && $(CURDIR)/out/$(APP_BINARY); \
	else \
		echo "No environment variables found at $(CURDIR)/.env. Cannot run."; \
	fi

local-kill: app-check ## Kill any currently running locally built APP binary
	-pkill -f '$(CURDIR)/out/$(APP_BINARY)'

local-iterate: app-check generate ## Run APP local build and run via air when files change
	air -c $(APP_DIR)/.air.toml

local-release-test: app-templates-check ## Check GoReleaser config and build APP snapshot
	goreleaser build --clean --snapshot --config $(APP_DIR)/.goreleaser.yml

local-release: local-test docker-login ## Release APP assets using locally installed golang toolchain and goreleaser (keyless)
	if test -e $(CURDIR)/$(APP_ENV_FILE); then \
		export `cat $(CURDIR)/$(APP_ENV_FILE) | xargs` && goreleaser release --clean --config $(APP_DIR)/.goreleaser.yml; \
	else \
		echo "No environment variables found at $(CURDIR)/$(APP_ENV_FILE). Cannot release."; \
	fi

local-sign: app-check ## Sign APP checksums with Cosign (keyless, requires OIDC token)
	cosign sign-blob --bundle=$(CURDIR)/dist/$(APP)/checksums.txt.sigstore.json $(CURDIR)/dist/$(APP)/checksums.txt --yes

local-verify: app-check ## Verify APP checksums signature with Cosign (keyless)
	@if cosign verify-blob \
		--bundle $(CURDIR)/dist/$(APP)/checksums.txt.sigstore.json \
		--certificate-identity-regexp $(COSIGN_IDENTITY_REGEXP) \
		--certificate-oidc-issuer $(COSIGN_OIDC_ISSUER) \
		$(CURDIR)/dist/$(APP)/checksums.txt 2>/dev/null; then \
		echo "Verified: signed by CI workflow identity"; \
	else \
		echo "CI identity regexp did not match (expected for locally-signed bundles)."; \
		echo "Falling back to issuer-only verification..."; \
		cosign verify-blob \
			--bundle $(CURDIR)/dist/$(APP)/checksums.txt.sigstore.json \
			--certificate-oidc-issuer $(COSIGN_OIDC_ISSUER) \
			$(CURDIR)/dist/$(APP)/checksums.txt && \
		echo "Issuer validated. Inspect the certificate identity above to confirm the signer." || \
		(echo "Verification failed: issuer validation did not pass." && exit 1); \
	fi

local-install: local-build local-verify ## Install compiled APP binary to local machine
	sudo cp $(CURDIR)/out/$(APP_BINARY) /usr/local/bin/$(APP_BINARY)
	sudo chmod 0755 /usr/local/bin/$(APP_BINARY)

upload-secrets-to-gh: app-check ## Upload APP secrets from apps/APP/.env to GitHub Actions Secrets + Dependabot
	$(CURDIR)/scripts/upload_secrets_to_github.sh "$(APP_BINARY)" "$(APP_ENV_FILE)" "$(APP_COSIGN_KEY)"

upload-secrets-envfile-to-1pass: app-check ## Upload APP secrets and apps/APP/.env file to 1Password
	$(CURDIR)/scripts/upload_secrets_to_1password.sh secrets "$(APP_BINARY)" "$(APP_ENV_FILE)"
	$(CURDIR)/scripts/upload_secrets_to_1password.sh envfile "$(APP_BINARY)" "$(APP_ENV_FILE)"

docker-login: ## Login to Docker registries used to publish images to
	if test -e $(CURDIR)/.env; then \
		export `cat $(CURDIR)/.env | xargs`; \
		export DOCKER_CONFIG=$$(mktemp -d); \
		mkdir -p $${DOCKER_CONFIG}; \
		DOCKERHUB_AUTH=$$(echo -n "$${DOCKERHUB_USERNAME}:$${DOCKERHUB_TOKEN}" | base64); \
		QUAY_AUTH=$$(echo -n "$${QUAY_USERNAME}:$${QUAY_TOKEN}" | base64); \
		GHCR_AUTH=$$(echo -n "$${GITHUB_USERNAME}:$${GH_GHCR_TOKEN}" | base64); \
		printf '{"credsStore":"","credHelpers":{},"auths":{"index.docker.io":{"auth":"%s"},"quay.io":{"auth":"%s"},"ghcr.io":{"auth":"%s"}}}\n' "$$DOCKERHUB_AUTH" "$$QUAY_AUTH" "$$GHCR_AUTH" > $${DOCKER_CONFIG}/config.json; \
	else \
		echo "No container registry credentials found, need to add them to ./.env. See README.md for more info"; \
	fi

pre-reqs: ## Install repository prerequisites
	command -v gomplate || go install github.com/hairyhenderson/gomplate/v5/cmd/gomplate@latest

pre-commit: pre-commit-install pre-commit-run ## Install and run pre-commit hooks

pre-commit-install: pre-reqs ## Install pre-commit hooks and necessary binaries
	command -v apt && apt-get update || echo "package manager not apt"
	# golangci-lint
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	# goimports
	go install golang.org/x/tools/cmd/goimports@latest
	# gosec
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	# staticcheck
	go install honnef.co/go/tools/cmd/staticcheck@latest
	# go-critic
	go install github.com/go-critic/go-critic/cmd/go-critic@latest
	# shellcheck
	command -v shellcheck || brew install shellcheck || apt install -y shellcheck || sudo dnf install -y ShellCheck || sudo apt install -y shellcheck
	# checkmake
	go install github.com/checkmake/checkmake/cmd/checkmake@latest
	# goreleaser
	go install github.com/goreleaser/goreleaser/v2@latest
	# actionlint
	command -v actionlint || brew install actionlint || go install github.com/rhysd/actionlint/cmd/actionlint@latest
	# syft
	command -v syft || brew install syft || curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
	# cosign
	command -v cosign || brew install cosign || go install github.com/sigstore/cosign/v3/cmd/cosign@latest
	# go-licenses
	go install github.com/google/go-licenses@latest
	# go vuln check
	go install golang.org/x/vuln/cmd/govulncheck@latest
	# air
	go install github.com/air-verse/air@latest
	# graphviz for dot
	command -v dot || brew install graphviz || sudo apt install -y graphviz || sudo dnf install -y graphviz
	# semgrep
	command -v semgrep || brew install semgrep || python3 -m pip install --break-system-packages --upgrade semgrep
	# install and update pre-commits
	grep --silent "VERSION=\"12 (bookworm)\"" /etc/os-release && apt install -y --no-install-recommends python3-pip && python3 -m pip install --break-system-packages --upgrade pre-commit || echo "OS is not Debian 12 bookworm"
	command -v pre-commit || brew install pre-commit || sudo dnf install -y pre-commit || sudo apt install -y pre-commit
	pre-commit install
	pre-commit autoupdate

pre-commit-run: generate-all ## Run pre-commit hooks against all files
	pre-commit run --all-files
	# manually run the following checks since their pre-commits are not working or do not exist
	@for app in $(APPS); do \
		binary=$$(awk -F': *' '/^binary:/ {gsub(/"/, "", $$2); print $$2; exit}' apps/$$app/app.yaml); \
		go-licenses report $(PKG)/apps/$$app; \
	done
	govulncheck ./...

update-golang-version: ## Update to latest Golang version across the repo
	@VERSION=`curl -s "https://go.dev/dl/?mode=json" | jq -r '.[0].version' | sed 's/go//' | cut -d '.' -f 1,2`; \
	$(CURDIR)/scripts/update_golang_version.sh $$VERSION

docs: ## Serve Go documentation
	@echo "Starting Go documentation server on localhost"
	@echo "Use Ctrl+C to stop the server"
	go doc -http

diagrams: app-generate ## Generate APP architectural diagrams using go-diagrams
	@echo "Generating architectural diagrams for $(APP)..."
	go run ./$(APP_DIR)/cmd/diagrams
	cd ./docs/diagrams/$(APP)/go-diagrams && for i in $$(find . -name '*.dot'); do \
		dot -Tpng $$i > $${i%.dot}.png; \
	done
	@echo "Diagram PNGs generated in ./docs/diagrams/$(APP)/go-diagrams/"

mutation-test: app-check ## Run APP mutation testing using go-gremlins
	@echo "Running mutation tests for $(APP)..."
	gremlins unleash -E "vendor/" $(APP_PACKAGES)
	@echo "Mutation testing completed"

test-changed: ## Run tests only for packages with changes since last commit
	@echo "Running tests for changed packages..."
	@CHANGED_PACKAGES=$$(git diff --name-only HEAD~1 | grep '\.go$$' | xargs -I {} dirname {} | sort -u | xargs -I {} go list ./{}... 2>/dev/null | grep -v 'no Go files'); \
	if [ -n "$$CHANGED_PACKAGES" ]; then \
		echo "Testing packages: $$CHANGED_PACKAGES"; \
		go test -race -v $$CHANGED_PACKAGES; \
	else \
		echo "No changed Go packages found"; \
	fi

watch-test: ## Watch for file changes and run tests for changed packages
	@echo "Watching for changes and running tests..."
	@while true; do \
		CHANGED_PACKAGES=$$(git diff --name-only HEAD | grep '\.go$$' | xargs -I {} dirname {} | sort -u | xargs -I {} go list ./{}... 2>/dev/null | grep -v 'no Go files'); \
		if [ -n "$$CHANGED_PACKAGES" ]; then \
			echo "Changed packages detected: $$CHANGED_PACKAGES"; \
			go test -race -v $$CHANGED_PACKAGES; \
		fi; \
		sleep 2; \
	done

profile-cpu: app-check ## Generate APP CPU performance profile
	@echo "Generating CPU profile for $(APP)..."
	mkdir -p $(CURDIR)/profiles/$(APP)
	go test -bench=. -cpuprofile=$(CURDIR)/profiles/$(APP)/cpu.prof $(APP_PACKAGES)
	@echo "CPU profile generated at $(CURDIR)/profiles/$(APP)/cpu.prof"
	go tool pprof -http $(CURDIR)/profiles/$(APP)/cpu.prof

profile-mem: app-check ## Generate APP memory performance profile
	@echo "Generating memory profile for $(APP)..."
	mkdir -p $(CURDIR)/profiles/$(APP)
	go test -bench=. -memprofile=$(CURDIR)/profiles/$(APP)/mem.prof $(APP_PACKAGES)
	@echo "Memory profile generated at $(CURDIR)/profiles/$(APP)/mem.prof"
	go tool pprof -http $(CURDIR)/profiles/$(APP)/mem.prof

profile-all: profile-cpu profile-mem ## Generate both CPU and memory profiles for APP

benchmark: app-check ## Run APP benchmarks
	@echo "Running benchmarks for $(APP)..."
	go test -bench=. -benchmem $(APP_PACKAGES)

clean: app-check ## Remove locally compiled binaries, profiles, generated docs, and built APP image
	@echo "=== Cleaning $(APP) artifacts ==="
	@rm -f $(CURDIR)/out/$(APP_BINARY)
	@rm -rf $(CURDIR)/profiles/$(APP)
	@rm -rf $(CURDIR)/docs/diagrams/$(APP)
	@rm -rf $(CURDIR)/dist/$(APP)
	@rm -rf $(APP_DIR)/c.out
	@rm -rf $(APP_DIR)/*.bundle
	@rm -rf $(CURDIR)/manpages/$(APP_BINARY).1.gz
	@rm -rf $(CURDIR)/completions/$(APP_BINARY).bash
	@rm -rf $(CURDIR)/completions/$(APP_BINARY).fish
	@rm -rf $(CURDIR)/completions/$(APP_BINARY).zsh
	-docker image rm $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG)

clean-all: ## Remove generated artifacts for every app
	@for app in $(APPS); do $(MAKE) clean APP=$$app; done
	@rm -rf $(CURDIR)/dist/
	@rm -rf $(CURDIR)/profiles/
	@rm -rf $(CURDIR)/completions/
	@rm -rf $(CURDIR)/manpages/

help: ## Display help text
	@grep -E '^[a-zA-Z_-]+ ?:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
