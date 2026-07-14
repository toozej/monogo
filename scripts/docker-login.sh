#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ ! -f "$ROOT/.env" ]]; then
	echo "No container registry credentials found in $ROOT/.env. See README.md for details." >&2
	exit 1
fi

set -a
# shellcheck disable=SC1091
source "$ROOT/.env"
set +a

DOCKER_CONFIG="$(mktemp -d)"
export DOCKER_CONFIG
mkdir -p "$DOCKER_CONFIG"
dockerhub_auth="$(printf '%s' "$DOCKERHUB_USERNAME:$DOCKERHUB_TOKEN" | base64)"
quay_auth="$(printf '%s' "$QUAY_USERNAME:$QUAY_TOKEN" | base64)"
ghcr_auth="$(printf '%s' "$GITHUB_USERNAME:$GH_GHCR_TOKEN" | base64)"
printf '{"credsStore":"","credHelpers":{},"auths":{"index.docker.io":{"auth":"%s"},"quay.io":{"auth":"%s"},"ghcr.io":{"auth":"%s"}}}\n' \
	"$dockerhub_auth" "$quay_auth" "$ghcr_auth" >"$DOCKER_CONFIG/config.json"

echo "Docker credentials written to $DOCKER_CONFIG/config.json."
echo "Export DOCKER_CONFIG=$DOCKER_CONFIG in commands that should use them."
