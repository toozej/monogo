#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="$1"
APP_NAME="$2"
APP_BINARY="$3"
APP_MAIN_PATH="$4"

if [[ -z "$APP" || -z "$APP_NAME" || -z "$APP_BINARY" || -z "$APP_MAIN_PATH" ]]; then
	echo "usage: install-app.sh APP APP_NAME APP_BINARY APP_MAIN_PATH" >&2
	exit 2
fi

MODULE="$(awk '/^module / {print $2; exit}' "$ROOT/go.mod")"
cd "$ROOT"
app_tag="$(git ls-remote --tags --refs origin "apps/$APP/v*" 2>/dev/null |
	awk '{sub(/^refs\/tags\//, "", $2); print $2}' |
	grep -E "^apps/$APP/v[0-9]+\.[0-9]+\.[0-9]+$" |
	sort -V | tail -n 1 || true)"
if [[ -z "$app_tag" ]]; then
	echo "No apps/$APP/vX.Y.Z release tag found on origin. Cannot resolve latest release." >&2
	exit 1
fi

version_tag="$(basename "$app_tag")"
if command -v go >/dev/null 2>&1; then
	commit="$(git ls-remote origin "refs/tags/$app_tag" "refs/tags/$app_tag^{}" | awk 'END {print $1}')"
	echo "Installing $APP_BINARY via go install from $app_tag ($commit)..."
	go install "$MODULE/$APP_MAIN_PATH@$commit"
else
	goos="$(uname -s)"
	goarch="$(uname -m)"
	case "$goarch" in
	x86_64 | amd64) goarch=x86_64 ;;
	i386 | i686) goarch=i386 ;;
	aarch64 | arm64) goarch=arm64 ;;
	armv7* | armv6* | armhf | arm) goarch=arm ;;
	esac
	url="https://github.com/toozej/monogo/releases/download/$app_tag/$APP_NAME""_""$goos""_""$goarch.tar.gz"
	echo "Downloading $APP_BINARY $version_tag for $goos""_""$goarch from $url..."
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	curl --fail --silent --show-error -L -o "$tmp/$APP_BINARY.tgz" "$url"
	tar -xzf "$tmp/$APP_BINARY.tgz" -C "$tmp/"
	chmod +x "$tmp/$APP_BINARY"
	sudo mv "$tmp/$APP_BINARY" "/usr/local/bin/$APP_BINARY"
fi
