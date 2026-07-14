#!/usr/bin/env bash

set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE="$1"
if [[ -z "$BASE" ]]; then BASE=HEAD~1; fi

cd "$ROOT"
changed_packages="$(git diff --name-only "$BASE" |
	grep '\.go$' |
	xargs -I {} dirname {} |
	sort -u |
	xargs -I {} go list ./{}... 2>/dev/null |
	grep -v 'no Go files' || true)"

if [[ -n "$changed_packages" ]]; then
	echo "Testing packages: $changed_packages"
	# Package paths are intentionally word-split for go test.
	# shellcheck disable=SC2086
	go test -race -v $changed_packages
else
	echo "No changed Go packages found."
fi
