#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="$1"
VERSION="$2"

if [[ -z "$APP" || -z "$VERSION" ]]; then
	echo "usage: delete-app-release.sh APP VERSION" >&2
	exit 2
fi
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "VERSION must look like vX.Y.Z (got '$VERSION')." >&2
	exit 1
fi

cd "$ROOT"
tag="apps/$APP/$VERSION"
echo "Deleting release $tag (GitHub release + local/origin tags)..."

if command -v gh >/dev/null 2>&1; then
	if gh release view "$tag" >/dev/null 2>&1; then
		gh release delete "$tag" --yes
		echo "  deleted GitHub release $tag"
	else
		echo "  no GitHub release $tag (skipping)"
	fi
else
	echo "  gh not installed (skipping GitHub release deletion)"
fi

if git rev-parse -q --verify "refs/tags/$tag" >/dev/null 2>&1; then
	git tag -d "$tag" >/dev/null
	echo "  deleted local tag $tag"
else
	echo "  no local tag $tag (skipping)"
fi

if git ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1; then
	git push origin --delete "refs/tags/$tag"
	echo "  deleted origin tag $tag"
else
	echo "  no origin tag $tag (skipping)"
fi

echo "Done."
