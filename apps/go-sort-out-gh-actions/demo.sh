#!/usr/bin/env bash
set -euo pipefail

# Demo for go-sort-out-gh-actions: run the built binary against a set of example
# workflows that exercise each detection path. Invoked by
# `make APP=go-sort-out-gh-actions demo`, which builds the binary and exports
# BIN (path to the built binary) and APP_DIR (this app's absolute directory).

BIN="${BIN:-out/go-sort-out-gh-actions}"
WORKFLOWS="${APP_DIR:-.}/demo/workflows"

separator() {
	printf '\n\n'
	printf '%*s' "$(tput cols 2>/dev/null || echo 80)" '' | tr ' ' '='
	printf '\n\n'
}

run_check() {
	local title="$1" file="$2"
	echo "=== Demo: ${title} ==="
	# The tool exits non-zero when it finds issues; keep the demo going.
	"${BIN}" check --workflow "${WORKFLOWS}/${file}" --verbose || true
}

run_check "Passing (all current) actions"             passing-archived-actions.yaml; separator
run_check "Failing (archived, outdated, EOL) actions" failing-archived-actions.yaml; separator
run_check "Current and up-to-date actions"            current-actions.yaml; separator
run_check "All archived actions"                      archived-actions.yaml; separator
run_check "Outdated and EOL runtime actions"          outdated-eol-actions.yaml; separator
run_check "Invalid and non-existent actions"          invalid-actions.yaml; separator
run_check "Mixed archived, outdated, EOL, invalid"    mixed-issues.yaml; separator
run_check "SHA-pinned and version-pinned actions"     sha-pinned-actions.yaml; separator

echo "=== Demo: pinnable actions ==="
"${BIN}" pin --workflow "${WORKFLOWS}/pinnable-actions.yaml" --verbose || true
