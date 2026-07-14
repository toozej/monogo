#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="$1"
APP_DIR="$2"
APP_BINARY="$3"
APP_DEMO="$4"

if [[ ! -f "$ROOT/$APP_DEMO" ]]; then
	echo "No demo script for $APP (expected $APP_DEMO)." >&2
	echo "Add an executable Bash script there to enable task demo APP=$APP." >&2
	exit 1
fi

echo "=== Running $APP demo ($APP_DEMO) ==="
env APP="$APP" \
	APP_DIR="$ROOT/$APP_DIR" \
	APP_BINARY="$APP_BINARY" \
	BIN="$ROOT/out/$APP_BINARY" \
	REPO_ROOT="$ROOT" \
	bash "$ROOT/$APP_DEMO"
