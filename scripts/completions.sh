#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="${1:-${APP:-golang-starter}}"
APP_CONFIG="${ROOT}/apps/${APP}/app.yaml"

if [ ! -f "${APP_CONFIG}" ]; then
	echo "missing app config: ${APP_CONFIG}" >&2
	exit 1
fi

BINARY="$(awk -F': *' '/^binary:/ {gsub(/"/, "", $2); print $2; exit}' "${APP_CONFIG}")"

mkdir -p "${ROOT}/completions"
for sh in bash zsh fish; do
	go run "./apps/${APP}" completion "${sh}" >"${ROOT}/completions/${BINARY}.${sh}"
done
