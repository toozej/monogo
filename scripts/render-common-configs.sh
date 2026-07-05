#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="${1:-${APP:-golang-starter}}"

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
	".env.sample"
)

for template in "${templates[@]}"; do
	mkdir -p "$(dirname "${ROOT}/${template}")"
	gomplate \
		--left-delim '[[' \
		--right-delim ']]' \
		-d "app=${APP_CONFIG}" \
		-f "${ROOT}/templates/common/${template}.tmpl" \
		-o "${ROOT}/${template}"
done
