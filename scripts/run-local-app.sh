#!/usr/bin/env bash

set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_BINARY="$1"

if [[ ! -f "$ROOT/.env" ]]; then
	echo "No environment variables found at $ROOT/.env. Cannot run." >&2
	exit 1
fi

set -a
# shellcheck disable=SC1091
source "$ROOT/.env"
set +a
"$ROOT/out/$APP_BINARY"
