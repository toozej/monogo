#!/usr/bin/env bash
set -euo pipefail

# Demo for photos2map: extract GPS coordinates from the EXIF data of the bundled
# sample photos and render them as both a GPX track and an HTML map. Invoked by
# `task demo APP=photos2map`, which builds the binary and exports BIN (path to
# the built binary), APP_DIR (this app's absolute directory), and REPO_ROOT.
#
# Note: photos2map writes to the hardcoded paths out/output.gpx and out/map.html
# relative to the working directory, so the demo runs from REPO_ROOT and then
# moves the results into demo-output/ (gitignored, removed by `task clean`).

BIN="${BIN:-out/photos2map}"
APP_DIR="${APP_DIR:-.}"
REPO_ROOT="${REPO_ROOT:-.}"
TESTDATA="${APP_DIR}/internal/testdata"
DEMO_DIR="${APP_DIR}/demo-output"
OUT_DIR="${REPO_ROOT}/out"

cd "${REPO_ROOT}"
mkdir -p "${DEMO_DIR}" "${OUT_DIR}"

echo "=== Sample photos (with GPS EXIF) in ${TESTDATA}/ ==="
ls -1 "${TESTDATA}"

echo "=== 1. gpx: extract GPS waypoints from EXIF into a GPX file ==="
"${BIN}" -i "${TESTDATA}" -o gpx
mv "${OUT_DIR}/output.gpx" "${DEMO_DIR}/output.gpx"
echo "--- ${DEMO_DIR}/output.gpx ---"
cat "${DEMO_DIR}/output.gpx"
echo

echo "=== 2. html: render the same photos as an interactive HTML map ==="
"${BIN}" -i "${TESTDATA}" -o html
mv "${OUT_DIR}/map.html" "${DEMO_DIR}/map.html"
echo "Wrote $(wc -l < "${DEMO_DIR}/map.html") lines of HTML to ${DEMO_DIR}/map.html"

echo
echo "=== Demo complete ==="
echo "Generated files in ${DEMO_DIR}/:"
echo "  GPX track: ${DEMO_DIR}/output.gpx"
echo "  HTML map:  ${DEMO_DIR}/map.html (open in a browser)"
