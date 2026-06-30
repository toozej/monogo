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
# Release bump type for `make release` (major | minor | patch)
TYPE ?= patch
APP_DIR = apps/$(APP)
APP_CONFIG = $(APP_DIR)/app.yaml
APP_BINARY = $(shell awk -F': *' '/^binary:/ {gsub(/"/, "", $$2); print $$2; exit}' $(APP_CONFIG) 2>/dev/null)
APP_NAME = $(shell awk -F': *' '/^name:/ {gsub(/"/, "", $$2); print $$2; exit}' $(APP_CONFIG) 2>/dev/null)
APP_MAIN_PATH = $(shell v=$$(awk -F': *' '/^mainPath:/ {gsub(/"/, "", $$2); print $$2; exit}' $(APP_CONFIG) 2>/dev/null); if test -n "$$v"; then echo "$$v"; else echo "$(APP_DIR)"; fi)
APP_CGO_ENABLED = $(shell v=$$(awk -F': *' '/^cgoEnabled:/ {gsub(/"/, "", $$2); print $$2; exit}' $(APP_CONFIG) 2>/dev/null); if [ "$$v" = "true" ] || [ "$$v" = "1" ]; then echo 1; else echo 0; fi)
APP_PACKAGES = ./$(APP_DIR)/... ./pkg/...
APP_ENV_FILE ?= $(APP_DIR)/.env
APP_COSIGN_KEY ?= $(APP_DIR)/$(APP_BINARY).key
# Per-app demo script run by `make APP=<app> demo`; each app stores its own.
APP_DEMO ?= $(APP_DIR)/demo.sh

# Build info
BUILDER = $(shell whoami)@$(shell hostname)
NOW = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Version control
VERSION = $(shell tag=$$(git describe --tags --match 'apps/$(APP)/v*' --dirty 2>/dev/null || true); if test -n "$$tag"; then basename "$$tag"; else echo local; fi)
APP_LATEST_TAG = $(shell git tag --list 'apps/$(APP)/v*' 2>/dev/null | grep -E '^apps/$(APP)/v[0-9]+\.[0-9]+\.[0-9]+$$' | sort -V | tail -n 1 || true)
APP_VERSION_TAG = $(shell tag='$(APP_LATEST_TAG)'; if test -n "$$tag"; then basename "$$tag"; else echo v0.0.0; fi)
COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BRANCH = $(shell branch=$$(git symbolic-ref --quiet --short HEAD 2>/dev/null || true); if test -n "$$branch"; then echo "$$branch"; else echo unknown; fi)

# Linker flags
PKG = $(shell head -n 1 go.mod | cut -c 8-)
VERSION_PACKAGE = $(PKG)/pkg/version
LDFLAGS = -s -w
LDFLAGS += \
	-X $(VERSION_PACKAGE).Version=$(VERSION) \
	-X $(VERSION_PACKAGE).Commit=$(COMMIT) \
	-X $(VERSION_PACKAGE).Branch=$(BRANCH) \
	-X $(VERSION_PACKAGE).BuiltAt=$(NOW) \
	-X $(VERSION_PACKAGE).Builder=$(BUILDER)

# Docker image info
IMAGE_AUTHOR = toozej
# Docker repository names must be lower-case, so lower-case the app name
# (e.g. RSSFFS -> rssffs). Derive from the app name (not the binary) to match
# the image published by CI and the GoReleaser/Compose templates, which key
# off app.yaml's `name` field.
IMAGE_NAME = $(shell echo '$(APP_NAME)' | tr '[:upper:]' '[:lower:]')
IMAGE_TAG = latest

COSIGN_IDENTITY_REGEXP := '^https://github.com/toozej/monogo/.github/workflows/(release|weekly-docker-refresh).yaml@refs/(tags/.*|heads/main)$$'
COSIGN_OIDC_ISSUER := 'https://token.actions.githubusercontent.com'

.PHONY: all list-apps import migrate-internal-package app-check common-generate app-generate generate generate-all app-templates-check templates-check vet test build release verify verify-docker verify-docker-all-registries run up down docker-vet docker-test docker-build distroless-build distroless-run install local local-all local-update-deps local-vet local-vendor local-test local-cover local-build local-run local-kill local-iterate release-test local-install docker-login pre-commit-install pre-commit-run pre-commit pre-reqs licenses licenses-all update-golang-version upload-secrets-to-gh upload-secrets-envfile-to-1pass docs diagrams mutation-test test-changed watch-test profile-cpu profile-mem profile-all benchmark demo clean clean-all help
.PHONY: common-generate-no-prereqs app-generate-no-prereqs app-templates-check-no-generate docker-vet-no-generate docker-test-no-generate docker-build-no-generate release-test-no-generate pre-commit-install-no-prereqs pre-commit-run-no-generate

all: generate-all ## Run default workflow for every app using Docker where available
	@set -e; \
	$(MAKE) local-update-deps local-vendor pre-commit-install-no-prereqs pre-commit-run-no-generate licenses-all; \
	for app in $(APPS); do \
		$(MAKE) app-check docker-vet-no-generate clean docker-test-no-generate docker-build-no-generate release-test-no-generate APP=$$app; \
	done
local: app-check generate local-update-deps local-vendor local-vet pre-commit clean local-test local-build release-test ## Run default workflow for APP using locally installed tools
local-all: generate-all ## Run local vet, test, build, and release checks for every app
	@for app in $(APPS); do $(MAKE) local APP=$$app; done

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

common-generate: pre-reqs ## Generate root shared configs from templates/common
	$(MAKE) common-generate-no-prereqs APP=$(APP)

common-generate-no-prereqs: app-check
	$(CURDIR)/scripts/render-common-configs.sh $(APP)

app-generate: pre-reqs ## Generate APP Docker, GoReleaser, Compose, and Air configs with gomplate
	$(MAKE) app-generate-no-prereqs APP=$(APP)

app-generate-no-prereqs: app-check
	$(CURDIR)/scripts/render-app-configs.sh $(APP)

generate: common-generate app-generate ## Generate root shared configs and APP configs

generate-all: pre-reqs ## Generate root shared configs and configs for every app
	$(MAKE) common-generate-no-prereqs APP=$(APP)
	@for app in $(APPS); do $(MAKE) app-generate-no-prereqs APP=$$app; done


app-templates-check: app-generate ## Render and check generated config for APP
	$(MAKE) app-templates-check-no-generate APP=$(APP)

app-templates-check-no-generate: app-check
	goreleaser check --config $(APP_DIR)/.goreleaser.yml

templates-check: pre-reqs ## Render and check generated config for every app
	@for app in $(APPS); do $(MAKE) app-generate-no-prereqs app-templates-check-no-generate APP=$$app; done

vet: local-vet ## Run goimports and go vet for APP

test: local-test ## Run go test for APP

build: docker-build ## Build APP Docker image

release: local-test ## Release APP: bump TYPE=<major|minor|patch> from the latest apps/APP/vX.Y.Z tag, push the new tag, and let CI publish
	@set -euo pipefail; \
	case "$(TYPE)" in major|minor|patch) ;; *) echo "TYPE must be major, minor, or patch (got '$(TYPE)')."; exit 1 ;; esac; \
	latest=$$( { git tag --list 'apps/$(APP)/v*'; git ls-remote --tags --refs origin 'apps/$(APP)/v*' 2>/dev/null | sed 's#.*refs/tags/##'; } \
		| grep -E '^apps/$(APP)/v[0-9]+\.[0-9]+\.[0-9]+$$' | sort -V | tail -n 1 || true); \
	if [ -n "$$latest" ]; then version_tag="$${latest##*/}"; base="$${version_tag#v}"; else base="0.0.0"; fi; \
	IFS=. read -r major minor patch <<< "$$base"; \
	case "$(TYPE)" in \
		major) major=$$((major + 1)); minor=0; patch=0 ;; \
		minor) minor=$$((minor + 1)); patch=0 ;; \
		patch) patch=$$((patch + 1)) ;; \
	esac; \
	new_version="v$${major}.$${minor}.$${patch}"; \
	new_tag="apps/$(APP)/$${new_version}"; \
	if git rev-parse -q --verify "refs/tags/$$new_tag" >/dev/null 2>&1 || git ls-remote --exit-code --tags origin "refs/tags/$$new_tag" >/dev/null 2>&1; then \
		echo "$$new_tag already exists locally or on origin; aborting."; exit 1; \
	fi; \
	echo "Releasing $(APP) $$new_version ($(TYPE) bump from $${latest:-<none>}) at commit $$(git rev-parse --short HEAD)..."; \
	git tag -a "$$new_tag" -m "$(APP) $$new_version"; \
	git push origin "refs/tags/$$new_tag"; \
	echo "Pushed $$new_tag; the Release workflow will build, sign, and publish $(APP)."

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

docker-vet: generate ## Run go vet for APP in Docker
	$(MAKE) docker-vet-no-generate APP=$(APP)

docker-vet-no-generate: app-check
	docker build --target vet -f $(CURDIR)/$(APP_DIR)/Dockerfile -t $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG) .

docker-test: generate ## Run go test for APP in Docker
	$(MAKE) docker-test-no-generate APP=$(APP)

docker-test-no-generate: app-check
	docker build --progress=plain --target test -f $(CURDIR)/$(APP_DIR)/Dockerfile -t $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG) .

docker-build: generate ## Build APP Docker image
	$(MAKE) docker-build-no-generate APP=$(APP)

docker-build-no-generate: app-check
	docker build -f $(CURDIR)/$(APP_DIR)/Dockerfile \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BRANCH=$(BRANCH) \
		--build-arg BUILT_AT=$(NOW) \
		--build-arg BUILDER=$(BUILDER) \
		-t $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG) .

run: app-check ## Run built APP Docker image
	-docker kill $(IMAGE_NAME)
	if test -e $(CURDIR)/.env; then \
		docker run --rm --name $(IMAGE_NAME) --env-file $(CURDIR)/.env $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG); \
	else \
		echo "No environment variables found at $(CURDIR)/.env. Cannot run."; \
	fi

up: docker-test docker-build ## Run APP Docker Compose project with built image
	docker compose -f $(APP_DIR)/docker-compose.yml down --remove-orphans
	docker compose -f $(APP_DIR)/docker-compose.yml up -d

down: app-check ## Stop APP Docker Compose project
	docker compose -f $(APP_DIR)/docker-compose.yml down --remove-orphans

distroless-build: app-check generate ## Build APP Docker image using distroless as final base
	docker build -f $(CURDIR)/$(APP_DIR)/Dockerfile.distroless \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BRANCH=$(BRANCH) \
		--build-arg BUILT_AT=$(NOW) \
		--build-arg BUILDER=$(BUILDER) \
		-t $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG)-distroless .

distroless-run: app-check ## Run built APP Docker image using distroless as final base
	docker run --rm --name $(IMAGE_NAME) -v $(CURDIR)/config:/config $(IMAGE_AUTHOR)/$(IMAGE_NAME):$(IMAGE_TAG)-distroless

install: app-check ## Install APP via `go install` from the latest release commit, or download the latest release binary
	@set -euo pipefail; \
	app_tag=$$(git ls-remote --tags --refs origin 'apps/$(APP)/v*' 2>/dev/null \
		| awk '{sub(/^refs\/tags\//, "", $$2); print $$2}' \
		| grep -E '^apps/$(APP)/v[0-9]+\.[0-9]+\.[0-9]+$$' \
		| sort -V | tail -n 1 || true); \
	if [ -z "$$app_tag" ]; then \
		echo "No apps/$(APP)/vX.Y.Z release tag found on origin. Cannot resolve latest release."; \
		exit 1; \
	fi; \
	version_tag="$${app_tag##*/}"; \
	if command -v go >/dev/null 2>&1; then \
		commit=$$(git ls-remote origin "refs/tags/$$app_tag" "refs/tags/$$app_tag^{}" | awk 'END {print $$1}'); \
		echo "Installing $(APP_BINARY) via go install from $$app_tag ($$commit)..."; \
		go install $(PKG)/$(APP_MAIN_PATH)@$$commit; \
	else \
		goos=$$(uname -s); goarch=$$(uname -m); \
		case "$$goarch" in \
			x86_64|amd64) goarch=x86_64 ;; \
			i386|i686) goarch=i386 ;; \
			aarch64|arm64) goarch=arm64 ;; \
			armv7*|armv6*|armhf|arm) goarch=arm ;; \
		esac; \
		url="https://github.com/toozej/monogo/releases/download/$$app_tag/$(APP_NAME)_$${goos}_$${goarch}.tar.gz"; \
		echo "Downloading $(APP_BINARY) $$version_tag for $${goos}_$${goarch} from $$url..."; \
		tmp=$$(mktemp -d); \
		curl --fail --silent --show-error -L -o "$$tmp/$(APP_BINARY).tgz" "$$url"; \
		tar -xzf "$$tmp/$(APP_BINARY).tgz" -C "$$tmp/"; \
		chmod +x "$$tmp/$(APP_BINARY)"; \
		sudo mv "$$tmp/$(APP_BINARY)" /usr/local/bin/$(APP_BINARY); \
		rm -rf "$$tmp"; \
	fi

local-update-deps: ## Run `go get -t -u ./...` to update Go module dependencies
	go get -t -u ./...

local-vet: app-check ## Run goimports and go vet for APP
	command -v goimports >/dev/null 2>&1 || go install golang.org/x/tools/cmd/goimports@latest
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
		set -a && . $(CURDIR)/.env && set +a && $(CURDIR)/out/$(APP_BINARY); \
	else \
		echo "No environment variables found at $(CURDIR)/.env. Cannot run."; \
	fi

local-kill: app-check ## Kill any currently running locally built APP binary
	-pkill -f '$(CURDIR)/out/$(APP_BINARY)'

local-iterate: app-check generate ## Run APP local build and run via air when files change
	air -c $(APP_DIR)/.air.toml

demo: app-check local-build ## Run APP demo script (apps/APP/demo.sh) against the freshly built binary
	@if test -f "$(APP_DEMO)"; then \
		echo "=== Running $(APP) demo ($(APP_DEMO)) ==="; \
		APP="$(APP)" \
		APP_DIR="$(CURDIR)/$(APP_DIR)" \
		APP_BINARY="$(APP_BINARY)" \
		BIN="$(CURDIR)/out/$(APP_BINARY)" \
		REPO_ROOT="$(CURDIR)" \
		bash "$(APP_DEMO)"; \
	else \
		echo "No demo script for $(APP) (expected $(APP_DEMO))."; \
		echo "Add an executable bash script there to enable 'make APP=$(APP) demo'."; \
	fi

release-test: app-generate ## Check GoReleaser config and build APP snapshot
	$(MAKE) release-test-no-generate APP=$(APP)

release-test-no-generate: app-templates-check-no-generate
	GORELEASER_CURRENT_TAG=$(APP_VERSION_TAG) goreleaser build --clean --snapshot --config $(APP_DIR)/.goreleaser.yml

local-install: local-build ## Install compiled APP binary to local machine
	sudo cp $(CURDIR)/out/$(APP_BINARY) /usr/local/bin/$(APP_BINARY)
	sudo chmod 0755 /usr/local/bin/$(APP_BINARY)

upload-secrets-to-gh: app-check ## Upload APP secrets from apps/APP/.env to GitHub Actions Secrets + Dependabot
	$(CURDIR)/scripts/upload_secrets_to_github.sh "$(APP_BINARY)" "$(APP_ENV_FILE)" "$(APP_COSIGN_KEY)"

upload-secrets-envfile-to-1pass: app-check ## Upload APP secrets and apps/APP/.env file to 1Password
	$(CURDIR)/scripts/upload_secrets_to_1password.sh secrets "$(APP_BINARY)" "$(APP_ENV_FILE)"
	$(CURDIR)/scripts/upload_secrets_to_1password.sh envfile "$(APP_BINARY)" "$(APP_ENV_FILE)"

docker-login: ## Login to Docker registries used to publish images to
	if test -e $(CURDIR)/.env; then \
		set -a && . $(CURDIR)/.env && set +a; \
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
	command -v gomplate >/dev/null 2>&1 || go install github.com/hairyhenderson/gomplate/v5/cmd/gomplate@latest

pre-commit: pre-commit-install pre-commit-run ## Install and run pre-commit hooks

pre-commit-install: pre-reqs ## Install pre-commit hooks and necessary binaries
	$(MAKE) pre-commit-install-no-prereqs

pre-commit-install-no-prereqs:
	command -v apt >/dev/null 2>&1 && apt-get update || echo "package manager not apt"
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
	command -v shellcheck >/dev/null 2>&1 || brew install shellcheck || apt install -y shellcheck || sudo dnf install -y ShellCheck || sudo apt install -y shellcheck
	# checkmake
	go install github.com/checkmake/checkmake/cmd/checkmake@latest
	# goreleaser
	go install github.com/goreleaser/goreleaser/v2@latest
	# actionlint
	command -v actionlint >/dev/null 2>&1 || brew install actionlint || go install github.com/rhysd/actionlint/cmd/actionlint@latest
	# syft
	command -v syft >/dev/null 2>&1 || brew install syft || curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
	# cosign
	command -v cosign >/dev/null 2>&1 || brew install cosign || go install github.com/sigstore/cosign/v3/cmd/cosign@latest
	# go-licenses
	go install github.com/google/go-licenses@latest
	# go vuln check
	go install golang.org/x/vuln/cmd/govulncheck@latest
	# air
	go install github.com/air-verse/air@latest
	# graphviz for dot
	command -v dot >/dev/null 2>&1 || brew install graphviz || sudo apt install -y graphviz || sudo dnf install -y graphviz
	# semgrep
	command -v semgrep >/dev/null 2>&1 || brew install semgrep || python3 -m pip install --break-system-packages --upgrade semgrep
	# install and update pre-commits
	grep --silent "VERSION=\"12 (bookworm)\"" /etc/os-release && apt install -y --no-install-recommends python3-pip && python3 -m pip install --break-system-packages --upgrade pre-commit || echo "OS is not Debian 12 bookworm"
	command -v pre-commit >/dev/null 2>&1 || brew install pre-commit || sudo dnf install -y pre-commit || sudo apt install -y pre-commit
	pre-commit install
	pre-commit autoupdate

pre-commit-run: generate-all ## Run pre-commit hooks, govulncheck, and license checks against all files
	$(MAKE) pre-commit-run-no-generate licenses-all

pre-commit-run-no-generate:
	pre-commit run --all-files
	# manually run govulncheck since it has no working pre-commit hook
	govulncheck ./...

licenses: app-check ## Report third-party licenses for APP
	command -v go-licenses >/dev/null 2>&1 || go install github.com/google/go-licenses@latest
	go-licenses report $(PKG)/apps/$(APP)

licenses-all: ## Report third-party licenses for every app
	@for app in $(APPS); do $(MAKE) licenses APP=$$app; done

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
	@rm -rf $(APP_DIR)/demo-output
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
	@grep -E '^[a-zA-Z0-9_-]+ ?:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
