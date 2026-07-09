#!/usr/bin/env bash
# Publish a Homebrew cask for a monogo app-tagged release into the tap repo.
#
# GoReleaser already renders the cask file into the release build's dist/ (its
# Run phase writes dist/<app>/homebrew/Casks/<binary>.rb with skip_upload set,
# so it is never pushed from the goreleaser job). This script commits that
# already-rendered file to the tap. Because the cask and the archives users
# download come from the same single build, the cask's SHA-256 always matches
# the uploaded release asset -- no rebuild, no reproducibility assumptions.
#
# Run this only after the GitHub release assets the cask links to have been
# uploaded, so the tap can never point at a 404.
#
# Usage: publish-homebrew-cask.sh APP DIST_DIR VERSION_TAG
#   APP          app name (e.g. go-sort-out-gh-actions)
#   DIST_DIR     GoReleaser dist directory for the app (e.g. dist/<app>)
#   VERSION_TAG  clean version tag (e.g. v1.6.0), used in the commit message
#
# Requires TAP_GITHUB_TOKEN in the environment with push access to the tap.
# Optional overrides: TAP_OWNER (toozej), TAP_REPO (homebrew-tap),
# TAP_CASK_DIR (Casks), GIT_AUTHOR_NAME/GIT_AUTHOR_EMAIL.
set -euo pipefail

APP="${1:?usage: publish-homebrew-cask.sh APP DIST_DIR VERSION_TAG}"
DIST_DIR="${2:?missing DIST_DIR (dist/<app>)}"
VERSION_TAG="${3:?missing VERSION_TAG (vX.Y.Z)}"

: "${TAP_GITHUB_TOKEN:?TAP_GITHUB_TOKEN must be set}"
TAP_OWNER="${TAP_OWNER:-toozej}"
TAP_REPO="${TAP_REPO:-homebrew-tap}"
TAP_CASK_DIR="${TAP_CASK_DIR:-Casks}"
GIT_AUTHOR_NAME="${GIT_AUTHOR_NAME:-github-actions[bot]}"
GIT_AUTHOR_EMAIL="${GIT_AUTHOR_EMAIL:-41898282+github-actions[bot]@users.noreply.github.com}"

# GoReleaser writes exactly one cask under dist/<app>/homebrew/<Directory>/.
cask_file="$(find "${DIST_DIR}/homebrew" -type f -name '*.rb' | head -n 1 || true)"
if [ -z "${cask_file}" ] || [ ! -f "${cask_file}" ]; then
	echo "::error::No rendered cask (.rb) found under ${DIST_DIR}/homebrew. Did the goreleaser job run the homebrew_casks pipe?" >&2
	exit 1
fi
cask_name="$(basename "${cask_file}")"
echo "Found rendered cask: ${cask_file}"

workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT

git clone --depth 1 \
	"https://x-access-token:${TAP_GITHUB_TOKEN}@github.com/${TAP_OWNER}/${TAP_REPO}.git" \
	"${workdir}/tap"

dest_dir="${workdir}/tap/${TAP_CASK_DIR}"
mkdir -p "${dest_dir}"
cp "${cask_file}" "${dest_dir}/${cask_name}"

cd "${workdir}/tap"
git config user.name "${GIT_AUTHOR_NAME}"
git config user.email "${GIT_AUTHOR_EMAIL}"
git add "${TAP_CASK_DIR}/${cask_name}"

if git diff --cached --quiet; then
	echo "Cask ${cask_name} is already up to date in ${TAP_OWNER}/${TAP_REPO}; nothing to publish."
	exit 0
fi

git commit -m "Brew cask update for ${APP} version ${VERSION_TAG}"
git push origin HEAD
echo "Published ${cask_name} (${APP} ${VERSION_TAG}) to ${TAP_OWNER}/${TAP_REPO}."
