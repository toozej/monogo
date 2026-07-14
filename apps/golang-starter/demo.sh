#!/usr/bin/env bash
set -euo pipefail

# Demo for golang-starter: the starter/template app. It greets a username and reports its
# build metadata. This is intentionally the smallest possible demo and doubles as
# a template for new apps scaffolded from golang-starter — copy it to apps/<app>/demo.sh
# and replace the commands. Invoked by `task demo APP=golang-starter`, which builds the
# binary and exports BIN (path to the built binary).

BIN="${BIN:-out/golang-starter}"

echo "=== 1. greet a username (-u overrides the USERNAME env var) ==="
"${BIN}" --username "golang-starter demo"

echo "=== 2. version: print build metadata (version, commit, branch, build time) ==="
"${BIN}" version

echo
echo "=== Demo complete ==="
