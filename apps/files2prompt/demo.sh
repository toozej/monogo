#!/usr/bin/env bash
set -euo pipefail

# Demo for files2prompt: crawl a small generated sample tree and show the
# filtering and output-format options. Invoked by `make APP=files2prompt demo`,
# which builds the binary and exports BIN (path to the built binary) and APP_DIR
# (this app's absolute directory).

BIN="${BIN:-out/files2prompt}"
APP_DIR="${APP_DIR:-.}"
DEMO_DIR="${APP_DIR}/demo-output"
SAMPLE="${DEMO_DIR}/sample"

separator() {
	printf '\n'
	printf '%*s' "$(tput cols 2>/dev/null || echo 80)" '' | tr ' ' '='
	printf '\n\n'
}

# Build a tiny, deterministic sample project to crawl.
echo "=== Preparing sample project under ${SAMPLE}/ ==="
rm -rf "${SAMPLE}"
mkdir -p "${SAMPLE}/sub"
printf 'package main\n\nfunc main() {\n\tprintln("hello from files2prompt demo")\n}\n' > "${SAMPLE}/main.go"
printf 'helper code\n' > "${SAMPLE}/sub/helper.go"
printf '# Sample\n\nA tiny project used by the files2prompt demo.\n' > "${SAMPLE}/README.md"
printf 'SECRET=do-not-include-me\n' > "${SAMPLE}/.env"
separator

echo "=== 1. Default: crawl everything (hidden files like .env are skipped) ==="
"${BIN}" "${SAMPLE}"
separator

echo "=== 2. -e .go: include only Go files ==="
"${BIN}" -e .go "${SAMPLE}"
separator

echo "=== 3. -m -n: Markdown fenced blocks with line numbers (.go only) ==="
"${BIN}" -m -n -e .go "${SAMPLE}"
separator

echo "=== 4. -c: Claude XML output (Markdown files only) ==="
"${BIN}" -c -e .md "${SAMPLE}"
separator

echo "=== 5. --include-hidden: now .env is included ==="
"${BIN}" --include-hidden "${SAMPLE}"
separator

echo "=== 6. --ignore '*.md': skip Markdown files ==="
"${BIN}" --ignore '*.md' "${SAMPLE}"
separator

echo "=== 7. -o: write the prompt to a file instead of stdout ==="
OUT_FILE="${DEMO_DIR}/prompt.txt"
"${BIN}" -e .go -o "${OUT_FILE}" "${SAMPLE}"
echo "Wrote $(wc -l < "${OUT_FILE}") lines to ${OUT_FILE}"

echo
echo "=== Demo complete ==="
echo "Sample project and generated prompt left in ${DEMO_DIR}/"
