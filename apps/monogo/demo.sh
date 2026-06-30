#!/usr/bin/env bash
set -euo pipefail

# Demo for monogo: the starter/template app. It greets a username and reports its
# build metadata. This is intentionally the smallest possible demo and doubles as
# a template for new apps scaffolded from monogo — copy it to apps/<app>/demo.sh
# and replace the commands. Invoked by `make APP=monogo demo`, which builds the
# binary and exports BIN (path to the built binary).

BIN="${BIN:-out/monogo}"

echo "=== 1. greet a username (-u overrides the USERNAME env var) ==="
"${BIN}" --username "monogo demo"

echo "=== 2. version: print build metadata (version, commit, branch, build time) ==="
"${BIN}" version

echo
echo "=== Demo complete ==="
