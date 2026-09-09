#!/usr/bin/env bash
set -euo pipefail

# Demo for notes2ssg. Invoked by `make APP=notes2ssg demo`, which builds the
# binary and exports BIN (path to the built binary).

BIN="${BIN:-out/notes2ssg}"

echo "=== 1. show command help ==="
"${BIN}" --help

echo "=== 2. version: print build metadata (version, commit, branch, build time) ==="
"${BIN}" version

echo
echo "=== Demo complete ==="
