#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="${1:-${APP:-}}"

if [ -z "${APP}" ]; then
	echo "usage: $0 <app-name>" >&2
	exit 1
fi

APP_DIR="${ROOT}/apps/${APP}"
APP_CONFIG="${APP_DIR}/app.yaml"

if [ ! -f "${APP_CONFIG}" ]; then
	echo "missing app config: ${APP_CONFIG}" >&2
	exit 1
fi

if ! command -v gomplate >/dev/null 2>&1; then
	echo "gomplate is required. Install it with: make pre-reqs" >&2
	exit 1
fi

templates=(
	".goreleaser.yml"
	"Dockerfile"
	"Dockerfile.distroless"
	"Dockerfile.goreleaser"
	"Dockerfile.goreleaser.distroless"
	"cmd/diagrams/main.go"
	"cmd/diagrams/main_test.go"
)

for template in "${templates[@]}"; do
	mkdir -p "$(dirname "${APP_DIR}/${template}")"
	gomplate \
		--left-delim '[[' \
		--right-delim ']]' \
		-d "app=${APP_CONFIG}" \
		-f "${ROOT}/templates/app/${template}.tmpl" \
		-o "${APP_DIR}/${template}"
done

common_templates=(
	".air.toml"
	"docker-compose.yml"
)

for template in "${common_templates[@]}"; do
	gomplate \
		--left-delim '[[' \
		--right-delim ']]' \
		-d "app=${APP_CONFIG}" \
		-f "${ROOT}/templates/common/${template}.tmpl" \
		-o "${APP_DIR}/${template}"
done
