#!/usr/bin/env bash
set -euo pipefail

# Check the CLI without downloading upstream releases.
BIN="${BIN:-out/go-get-releases}"
"${BIN}" --help
"${BIN}" version
