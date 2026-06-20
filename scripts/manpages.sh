#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="${1:-${APP:-monogo}}"
APP_CONFIG="${ROOT}/apps/${APP}/app.yaml"

if [ ! -f "${APP_CONFIG}" ]; then
	echo "missing app config: ${APP_CONFIG}" >&2
	exit 1
fi

BINARY="$(awk -F': *' '/^binary:/ {gsub(/"/, "", $2); print $2; exit}' "${APP_CONFIG}")"

mkdir -p "${ROOT}/manpages"
go run "./apps/${APP}" man | gzip -c -9 >"${ROOT}/manpages/${BINARY}.1.gz"
