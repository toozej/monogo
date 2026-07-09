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

mkdir -p "${ROOT}/manpages"
# `gzip -n` omits the original filename and modification timestamp from the
# gzip header. Without it, gzip stamps the current wall-clock time into every
# .1.gz, so the manpage bundled inside the release archive differs byte-for-byte
# on each run, breaking reproducible archives (and Homebrew cask checksums).
go run "./apps/${APP}" man | gzip -n -c -9 >"${ROOT}/manpages/${BINARY}.1.gz"
