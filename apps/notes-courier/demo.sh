#!/usr/bin/env bash
set -euo pipefail

# Demo for notes-courier. Invoked by `make APP=notes-courier demo`, which builds the
# binary and exports BIN (path to the built binary).

BIN="${BIN:-out/notes-courier}"

echo "=== 1. show command help ==="
"${BIN}" --help

echo "=== 2. version: print build metadata (version, commit, branch, build time) ==="
"${BIN}" version

echo
echo "=== Demo complete ==="
