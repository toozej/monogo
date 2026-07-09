#!/usr/bin/env bash
# Normalize the mtimes of every file bundled into the release archive to the
# released commit's timestamp, so the tarball is byte-for-byte reproducible
# across independent builds.
#
# GoReleaser copies each file's on-disk mtime into the tar header. The binary's
# mtime is already pinned via builds.mod_timestamp ({{ .CommitTimestamp }}), but
# the extra files are not: regenerated completions/manpages carry the wall-clock
# time of the `before` hooks, and a freshly-checked-out README/LICENSE carry the
# checkout time. Those vary run-to-run and change the archive checksum. Pinning
# them to the commit timestamp (the same instant GoReleaser uses for the binary)
# makes the archive deterministic. Paired with `gzip -n` in manpages.sh, which
# strips the gzip header's own embedded timestamp.
#
# Run as the last GoReleaser `before` hook, after completions.sh and manpages.sh
# have generated their files. Usage: normalize-archive-mtimes.sh <app-name>
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="${1:-${APP:-golang-starter}}"
APP_CONFIG="${ROOT}/apps/${APP}/app.yaml"

if [ ! -f "${APP_CONFIG}" ]; then
	echo "missing app config: ${APP_CONFIG}" >&2
	exit 1
fi

BINARY="$(awk -F': *' '/^binary:/ {gsub(/"/, "", $2); print $2; exit}' "${APP_CONFIG}")"

# HEAD's commit timestamp (epoch seconds) — the released commit, matching the
# value GoReleaser stamps onto the binary.
epoch="$(git -C "${ROOT}" log -1 --format=%ct)"

# Portable epoch -> `touch -t` stamp (YYYYMMDDhhmm.SS). GNU date uses -d @epoch;
# BSD/macOS date uses -r epoch. Computed in UTC and applied with TZ=UTC0 so the
# resulting mtime is identical regardless of the runner's local timezone.
stamp="$(date -u -d "@${epoch}" +%Y%m%d%H%M.%S 2>/dev/null || date -u -r "${epoch}" +%Y%m%d%H%M.%S)"

# The set of files listed under archives.files in the GoReleaser config.
files=(
	"${ROOT}/README.md"
	"${ROOT}/LICENSE"
	"${ROOT}/completions/${BINARY}.bash"
	"${ROOT}/completions/${BINARY}.fish"
	"${ROOT}/completions/${BINARY}.zsh"
	"${ROOT}/manpages/${BINARY}.1.gz"
)

for f in "${files[@]}"; do
	[ -e "${f}" ] || continue
	TZ=UTC0 touch -t "${stamp}" -- "${f}"
done
