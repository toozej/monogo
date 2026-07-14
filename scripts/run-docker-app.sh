#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE_NAME="$1"
IMAGE_REF="$2"

docker kill "$IMAGE_NAME" >/dev/null 2>&1 || true
if [[ ! -f "$ROOT/.env" ]]; then
	echo "No environment variables found at $ROOT/.env. Cannot run." >&2
	exit 1
fi

docker run --rm --name "$IMAGE_NAME" --env-file "$ROOT/.env" "$IMAGE_REF"
