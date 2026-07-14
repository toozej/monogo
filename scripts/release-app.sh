#!/usr/bin/env bash

set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="$1"
TYPE="$2"
VERSION="$3"
if [[ -z "$TYPE" ]]; then TYPE="patch"; fi

if [[ -z "$APP" ]]; then
	echo "usage: release-app.sh APP [TYPE] [VERSION]" >&2
	exit 2
fi

cd "$ROOT"

if [[ -n "$VERSION" ]]; then
	if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
		echo "VERSION must look like vX.Y.Z (got '$VERSION')." >&2
		exit 1
	fi
	new_version="$VERSION"
	release_reason="version $VERSION provided"
else
	case "$TYPE" in
	major | minor | patch) ;;
	*)
		echo "TYPE must be major, minor, or patch (got '$TYPE')." >&2
		exit 1
		;;
	esac

	latest="$({
		git tag --list "apps/$APP/v*"
		git ls-remote --tags --refs origin "apps/$APP/v*" 2>/dev/null | sed 's#.*refs/tags/##'
	} | grep -E "^apps/$APP/v[0-9]+\.[0-9]+\.[0-9]+$" | sort -V | tail -n 1 || true)"
	if [[ -n "$latest" ]]; then
		version_tag="$(basename "$latest")"
		base="$(printf '%s' "$version_tag" | sed 's/^v//')"
	else
		base="0.0.0"
	fi
	IFS=. read -r major minor patch <<<"$base"
	case "$TYPE" in
	major)
		major=$((major + 1))
		minor=0
		patch=0
		;;
	minor)
		minor=$((minor + 1))
		patch=0
		;;
	patch) patch=$((patch + 1)) ;;
	esac
	new_version="v$major.$minor.$patch"
	if [[ -n "$latest" ]]; then
		release_reason="$TYPE bump from $latest"
	else
		release_reason="$TYPE bump from <none>"
	fi
fi

new_tag="apps/$APP/$new_version"
if git rev-parse -q --verify "refs/tags/$new_tag" >/dev/null 2>&1 ||
	git ls-remote --exit-code --tags origin "refs/tags/$new_tag" >/dev/null 2>&1; then
	echo "$new_tag already exists locally or on origin; aborting." >&2
	exit 1
fi

echo "Releasing $APP $new_version ($release_reason) at commit $(git rev-parse --short HEAD)..."
git tag -a "$new_tag" -m "$APP $new_version"
git push origin "refs/tags/$new_tag"
echo "Pushed $new_tag; the Release workflow will build, sign, and publish $APP."
