#!/usr/bin/env bash
set -euo pipefail

# Demo for lego-stego: exercise hide, info, reveal, embed, and extract against
# the freshly built binary. Invoked by `task demo APP=lego-stego`, which builds
# the binary and exports BIN (path to the built binary) and APP_DIR (this app's
# absolute directory).

BIN="${BIN:-out/lego-stego}"
APP_DIR="${APP_DIR:-.}"

DEMO_PASSWORD="${DEMO_PASSWORD:-lego-stego-demo}"
DEMO_DIR="${APP_DIR}/demo-output"
DEMO_CARRIER="${APP_DIR}/demo-assets/avatar.png"
DEMO_STEGO_HIDE="${DEMO_DIR}/demo_stego_hide.png"
DEMO_STEGO_EMBED="${DEMO_DIR}/demo_stego_embed.png"
DEMO_REVEALED_FILE="${DEMO_DIR}/demo_revealed_secret.txt"
DEMO_EXTRACTED_QR="${DEMO_DIR}/demo_extracted_qr.png"
DEMO_SECRET_FILE="${DEMO_DIR}/demo_secret.txt"
DEMO_QR_URL="https://github.com/toozej/monogo/apps/lego-stego"

show_image() {
	if command -v viu >/dev/null 2>&1; then
		viu "$1"
	else
		echo "  (install 'viu' to preview $1)"
	fi
}

mkdir -p "${DEMO_DIR}"
echo "=== Preparing demo secret file ==="
echo "This is a lego-stego demo secret message." > "${DEMO_SECRET_FILE}"

echo "=== 1. hide: Hide a secret file inside an image ==="
"${BIN}" hide -i "${DEMO_CARRIER}" -o "${DEMO_STEGO_HIDE}" -f "${DEMO_SECRET_FILE}" --password "${DEMO_PASSWORD}"

echo "=== 2. info: Inspect the stego image for hidden payload ==="
"${BIN}" info -i "${DEMO_STEGO_HIDE}"

echo "=== 3. reveal: Reveal the hidden file from the stego image ==="
"${BIN}" reveal -i "${DEMO_STEGO_HIDE}" -o "${DEMO_REVEALED_FILE}" --password "${DEMO_PASSWORD}"
echo "=== Revealed content ==="
cat "${DEMO_REVEALED_FILE}"

echo "=== 4. embed: Embed a QR code (repo URL) into an image ==="
"${BIN}" embed -i "${DEMO_CARRIER}" -o "${DEMO_STEGO_EMBED}" -u "${DEMO_QR_URL}" --password "${DEMO_PASSWORD}"

echo "=== 5. extract: Extract and decode the QR code from the stego image ==="
"${BIN}" extract -i "${DEMO_STEGO_EMBED}" -o "${DEMO_EXTRACTED_QR}" --password "${DEMO_PASSWORD}"

echo "=== Demo complete ==="
echo "Generated files in ${DEMO_DIR}/:"
echo "  Stego (hide): ${DEMO_STEGO_HIDE}"
show_image "${DEMO_STEGO_HIDE}"
echo "  Stego (embed): ${DEMO_STEGO_EMBED}"
show_image "${DEMO_STEGO_EMBED}"
echo "  Revealed file: ${DEMO_REVEALED_FILE}"
cat "${DEMO_REVEALED_FILE}"
echo "  Extracted QR:"
show_image "${DEMO_EXTRACTED_QR}"
