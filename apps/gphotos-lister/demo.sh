#!/usr/bin/env bash
set -euo pipefail

# Run from the repository root. The Make target builds the binary first.
#
# List and format filenames from an unlisted album:
#   GPHOTOS_ALBUM='https://photos.google.com/share/ALBUM?key=KEY' \
#     make demo APP=gphotos-lister
#
# Also copy local originals into the demo directory:
#   GPHOTOS_ALBUM='https://photos.app.goo.gl/SHARE_ID' \
#     GPHOTOS_SOURCE='/photos/originals' make demo APP=gphotos-lister
#
# Set GPHOTOS_PREFIX and GPHOTOS_SUFFIX to change the formatting examples.
# Set GPHOTOS_DEBUG=true to log request progress and heart counts to stderr.
# No Google account credentials are required.

BIN="${BIN:-out/gphotos-lister}"
APP_DIR="${APP_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
DEMO_DIR="${APP_DIR}/demo-output"

echo '=== 1. CLI help and version ==='
"${BIN}" --help
"${BIN}" version

# Print this example instead of starting a server that blocks the remaining examples.
echo
echo '=== Web UI example ==='
printf 'Run: %q serve --host 127.0.0.1 --port 8080\n' "${BIN}"
echo 'Open http://127.0.0.1:8080. Enter the album URL and optional prefix or suffix.'
echo 'Select "Copy matching local files" to enter source and destination paths on the server.'

if [[ -z "${GPHOTOS_ALBUM:-}" ]]; then
    cat <<'EOF'

Set GPHOTOS_ALBUM to run the list examples:
  GPHOTOS_ALBUM='https://photos.google.com/share/ALBUM?key=KEY' \
    make demo APP=gphotos-lister

Set GPHOTOS_SOURCE as well to run the copy examples:
  GPHOTOS_ALBUM='https://photos.app.goo.gl/SHARE_ID' \
    GPHOTOS_SOURCE='/photos/originals' make demo APP=gphotos-lister

The demo writes its lists and copies under apps/gphotos-lister/demo-output/.
EOF
    exit 0
fi

mkdir -p "${DEMO_DIR}"

echo
echo '=== 2. Plain original filenames with CLI flags ==='
"${BIN}" --album "${GPHOTOS_ALBUM}" \
    --output "${DEMO_DIR}/liked-photos.txt" \
    --prefix '' --suffix '' --copy=false
echo 'First five filenames:'
head -n 5 "${DEMO_DIR}/liked-photos.txt"

echo
echo '=== 3. Prefix and suffix with a positional album URL ==='
# The suffix follows the extension: /photos/IMG_1234.JPG.original.
"${BIN}" "${GPHOTOS_ALBUM}" \
    --output "${DEMO_DIR}/formatted-photos.txt" \
    --prefix "${GPHOTOS_PREFIX:-/photos/}" \
    --suffix "${GPHOTOS_SUFFIX:-.original}" \
    --copy=false
echo 'First five formatted lines:'
head -n 5 "${DEMO_DIR}/formatted-photos.txt"

echo
echo '=== 4. Configure the list through environment variables ==='
GPHOTOS_ALBUM="${GPHOTOS_ALBUM}" \
    GPHOTOS_OUTPUT="${DEMO_DIR}/environment-photos.txt" \
    GPHOTOS_PREFIX="${GPHOTOS_PREFIX:-/photos/}" \
    GPHOTOS_SUFFIX="${GPHOTOS_SUFFIX:-.original}" \
    GPHOTOS_COPY=false \
    "${BIN}"

if [[ -n "${GPHOTOS_SOURCE:-}" ]]; then
    echo
    echo '=== 5. Copy local originals and overwrite existing demo copies ==='
    # The app searches subdirectories. Copies keep the original filenames.
    "${BIN}" --album "${GPHOTOS_ALBUM}" \
        --output "${DEMO_DIR}/copied-photos.txt" \
        --prefix '' --suffix '' \
        --copy --source "${GPHOTOS_SOURCE}" \
        --destination "${DEMO_DIR}/copies" --no-overwrite=false

    echo
    echo '=== 6. Configure copies through environment variables and skip existing files ==='
    GPHOTOS_ALBUM="${GPHOTOS_ALBUM}" \
        GPHOTOS_OUTPUT="${DEMO_DIR}/skipped-photos.txt" \
        GPHOTOS_PREFIX='' \
        GPHOTOS_SUFFIX='' \
        GPHOTOS_COPY=true \
        GPHOTOS_SOURCE="${GPHOTOS_SOURCE}" \
        GPHOTOS_DESTINATION="${DEMO_DIR}/copies" \
        GPHOTOS_NO_OVERWRITE=true \
        "${BIN}"
else
    echo
    echo 'Set GPHOTOS_SOURCE to run the copy and no-overwrite examples.'
fi

echo
printf 'Demo output: %s\n' "${DEMO_DIR}"
