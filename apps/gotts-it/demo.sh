#!/usr/bin/env bash
set -euo pipefail

# Demo for gotts-it. Invoked by `make APP=gotts-it demo`, which builds the
# binary and exports BIN (the built binary) and APP_DIR (this app's absolute
# directory). It creates an isolated local .env for the Google Translate TTS
# backend, converts two articles with the host binary, and removes that
# environment after the run. The generated MP3 files are retained.

BIN="${BIN:-out/gotts-it}"
APP_DIR="${APP_DIR:-.}"
DEMO_DIR="${APP_DIR}/demo-output"
DEMO_ENV_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gotts-it-demo.XXXXXX")"
DEMO_ENV_FILE="${DEMO_ENV_DIR}/.env"

SIMON_URL="https://simonwillison.net/2025/Dec/30/"
BUN_URL="https://bun.com/blog/bun-in-rust"
SIMON_OUTPUT="${DEMO_DIR}/simon-willison-2025-dec-30.mp3"
BUN_OUTPUT="${DEMO_DIR}/bun-in-rust.mp3"

teardown() {
	rm -f "${DEMO_ENV_FILE}"
	rmdir "${DEMO_ENV_DIR}" || true
}

cleanup() {
	local status=$?

	echo "=== Tearing down local demo environment ==="
	teardown
	exit "${status}"
}

mkdir -p "${DEMO_DIR}"
trap cleanup EXIT

echo "=== Setting up local demo environment ==="
printf '%s\n' \
	'TTS_BACKEND=google' \
	'GOOGLE_TRANSLATE_LANG=en' \
	'TTS_FORMAT=mp3' \
	'TTS_TIMEOUT=1m' \
	'FETCH_TIMEOUT=2m' > "${DEMO_ENV_FILE}"

run_gotts_it() {
	local url="$1"
	local output="$2"

	(
		cd "${DEMO_ENV_DIR}"
		"${BIN}" --url "${url}" --output "${output}"
	)
}

echo "=== 1. Convert Simon Willison's article ==="
run_gotts_it "${SIMON_URL}" "${SIMON_OUTPUT}"

echo "=== 2. Convert Bun's article ==="
run_gotts_it "${BUN_URL}" "${BUN_OUTPUT}"

echo
echo "=== Tearing down local demo environment ==="
teardown
trap - EXIT

echo "=== Demo complete ==="
echo "Listen to the generated audio files:"
printf '  %s\n' "${SIMON_OUTPUT}" "${BUN_OUTPUT}"
