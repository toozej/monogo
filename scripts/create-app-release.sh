#!/usr/bin/env bash
# Create (or update) the GitHub release for a monogo app-tagged release.
#
# Generates release notes from the app's commit range and uploads the
# GoReleaser dist assets. Used by the CI Release workflow so release notes and
# assets are produced from a single, testable place.
#
# Usage: create-app-release.sh APP APP_TAG VERSION_TAG [DIST_DIR]
#   APP          app name (e.g. url2anki)
#   APP_TAG      release tag (e.g. apps/url2anki/v1.2.3)
#   VERSION_TAG  clean version tag (e.g. v1.2.3)
#   DIST_DIR     GoReleaser dist directory (default: dist/<app>)
#
# Requires `gh` authenticated via GH_TOKEN, and a full-history checkout with
# tags fetched (for previous-tag discovery and release notes).
set -euo pipefail

APP="${1:?usage: create-app-release.sh APP APP_TAG VERSION_TAG [DIST_DIR]}"
APP_TAG="${2:?missing APP_TAG (apps/<app>/vX.Y.Z)}"
VERSION_TAG="${3:?missing VERSION_TAG (vX.Y.Z)}"
DIST_DIR="${4:-dist/${APP}}"

notes_file="$(mktemp)"
trap 'rm -f "${notes_file}"' EXIT

# Most-recent prior release tag for this app, if any.
previous_tag="$(git for-each-ref \
	--sort=-version:refname \
	--format='%(refname:strip=2)' \
	"refs/tags/apps/${APP}/v*" \
	| grep -E "^apps/${APP}/v[0-9]+\.[0-9]+\.[0-9]+$" \
	| grep -Fxv "${APP_TAG}" \
	| head -n 1 || true)"

{
	echo "## Changes"
	echo
	if [ -n "${previous_tag}" ]; then
		echo "Changes since ${previous_tag}:"
		echo
		# Bounded by the tag range, so it is safe to include the shared paths
		# that affect this app's build.
		git log --no-merges --pretty='- %s (%h)' "${previous_tag}..${APP_TAG}" \
			-- "apps/${APP}" pkg templates Makefile .github/workflows || true
	else
		echo "Initial ${APP} release from ${APP_TAG}."
		echo
		# A first release has no lower bound, so scope to the app's own
		# directory; including the shared paths here would replay every
		# infrastructure commit across all apps.
		git log --no-merges --pretty='- %s (%h)' "${APP_TAG}" -- "apps/${APP}" || true
	fi
} >"${notes_file}"

assets=()
while IFS= read -r -d '' asset; do
	assets+=("${asset}")
done < <(find "${DIST_DIR}" -maxdepth 1 -type f \
	! -name config.yaml ! -name metadata.json ! -name artifacts.json \
	-print0 | sort -z)

if [ "${#assets[@]}" -eq 0 ]; then
	echo "::error::No top-level release assets found in ${DIST_DIR}." >&2
	exit 1
fi

if gh release view "${APP_TAG}" >/dev/null 2>&1; then
	gh release edit "${APP_TAG}" \
		--title "${APP} ${VERSION_TAG}" \
		--notes-file "${notes_file}"
	gh release upload "${APP_TAG}" "${assets[@]}" --clobber
else
	gh release create "${APP_TAG}" "${assets[@]}" \
		--title "${APP} ${VERSION_TAG}" \
		--notes-file "${notes_file}" \
		--verify-tag
fi
