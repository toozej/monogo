#!/usr/bin/env bash
set -euo pipefail

# Demo for terranotate: exercise parse, validate (passing + failing), generate,
# and the fix workflow against the bundled examples. Invoked by
# `make APP=terranotate demo`, which builds the binary and exports BIN (path to
# the built binary) and APP_DIR (this app's absolute directory).
#
# Note: the `fix` command rewrites files in place and leaves .bak backups, so it
# runs against a throwaway copy under demo-output/ (gitignored, removed by
# `make clean`) rather than the tracked examples.

BIN="${BIN:-out/terranotate}"
APP_DIR="${APP_DIR:-.}"
EXAMPLES="${APP_DIR}/examples"
DEMO_DIR="${APP_DIR}/demo-output"

separator() {
	printf '\n'
	printf '%*s' "$(tput cols 2>/dev/null || echo 80)" '' | tr ' ' '='
	printf '\n\n'
}

echo "=== 1. parse: display annotations parsed from a Terraform file ==="
"${BIN}" parse "${EXAMPLES}/example.tf"
separator

echo "=== 2. validate: AWS module with sub-modules (expected PASS) ==="
"${BIN}" validate "${EXAMPLES}/example1-aws-module/vpc" "${EXAMPLES}/example1-aws-module/schema.yaml"
separator

echo "=== 3. validate: AWS workspace, recursive (expected FAIL) ==="
# Validation exits non-zero when it finds schema violations; keep the demo going.
"${BIN}" validate "${EXAMPLES}/example2-aws-workspace/infrastructure" "${EXAMPLES}/example2-aws-workspace/schema.yaml" || true
separator

echo "=== 4. validate: GCP monorepo, all projects (expected FAIL) ==="
"${BIN}" validate "${EXAMPLES}/example3-gcp-monorepo" "${EXAMPLES}/example3-gcp-monorepo/schema.yaml" || true
separator

echo "=== 5. generate: render markdown docs from a module's annotations ==="
"${BIN}" generate "${EXAMPLES}/example1-aws-module/vpc" "${EXAMPLES}/example1-aws-module/schema.yaml"
separator

echo "=== 6. fix: auto-add missing annotations (runs on a disposable copy) ==="
FIX_SRC="${EXAMPLES}/example4-gcp-module"
FIX_WORK="${DEMO_DIR}/example4-fix"
rm -rf "${FIX_WORK}"
mkdir -p "${FIX_WORK}"
cp -R "${FIX_SRC}/storage" "${FIX_WORK}/storage"
cp "${FIX_SRC}/schema.yaml" "${FIX_WORK}/schema.yaml"

echo "--- 6a. validate before fix (expected FAIL) ---"
"${BIN}" validate "${FIX_WORK}/storage" "${FIX_WORK}/schema.yaml" || true
echo "--- 6b. fix: add missing comment blocks ---"
"${BIN}" fix "${FIX_WORK}/storage" "${FIX_WORK}/schema.yaml"
echo "--- 6c. validate after fix (fewer errors) ---"
"${BIN}" validate "${FIX_WORK}/storage" "${FIX_WORK}/schema.yaml" || true

echo
echo "=== Demo complete ==="
echo "Fix workflow artifacts (including .bak backups) left in ${FIX_WORK}/"
