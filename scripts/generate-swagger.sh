#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="${1:-${APP:-}}"

if [ -z "${APP}" ]; then
	echo "usage: $0 <app-name>" >&2
	exit 1
fi

APP_DIR="${ROOT}/apps/${APP}"
APP_CONFIG="${APP_DIR}/app.yaml"

if [ ! -f "${APP_CONFIG}" ]; then
	echo "missing app config: ${APP_CONFIG}" >&2
	exit 1
fi

SWAGGER_ENABLED="${APP_SWAGGER_ENABLED:-}"
if [ -z "${SWAGGER_ENABLED}" ]; then
	SWAGGER_ENABLED="$(awk -F': *' '/^swaggerEnabled:/ {gsub(/"/, "", $2); print $2; exit}' "${APP_CONFIG}")"
fi

case "${SWAGGER_ENABLED}" in
1 | true) ;;
"" | 0 | false) exit 0 ;;
*)
	echo "swaggerEnabled must be true or false in ${APP_CONFIG}" >&2
	exit 1
	;;
esac

GENERAL_INFO="${APP_SWAGGER_GENERAL_INFO:-}"
if [ -z "${GENERAL_INFO}" ]; then
	GENERAL_INFO="$(awk -F': *' '/^swaggerGeneralInfo:/ {gsub(/"/, "", $2); print $2; exit}' "${APP_CONFIG}")"
fi
if [ -z "${GENERAL_INFO}" ]; then
	echo "swaggerGeneralInfo is required when swaggerEnabled is true in ${APP_CONFIG}" >&2
	exit 1
fi
case "${GENERAL_INFO}" in
/* | ../* | */../* | */..)
	echo "swaggerGeneralInfo must stay within ${APP_DIR}: ${GENERAL_INFO}" >&2
	exit 1
	;;
esac
if [ ! -f "${APP_DIR}/${GENERAL_INFO}" ]; then
	echo "Swagger general info file does not exist: ${APP_DIR}/${GENERAL_INFO}" >&2
	exit 1
fi

# `make generate-all` runs on every pre-commit and CI step, so avoid the full
# `swag init` AST parse when nothing swag reads has changed. swag pulls
# annotations from non-test .go files, so if the generated docs already exist and
# no such file is newer than them, they are current and we can skip. Any missing
# output or newer source falls through and regenerates. Set APP_SWAGGER_FORCE=1
# to always regenerate (e.g. after a swag version bump).
DOCS_DIR="${APP_DIR}/docs"
if [ "${APP_SWAGGER_FORCE:-}" != "1" ] &&
	[ -f "${DOCS_DIR}/docs.go" ] &&
	[ -f "${DOCS_DIR}/swagger.json" ] &&
	[ -f "${DOCS_DIR}/swagger.yaml" ]; then
	NEWER_SOURCE="$(find "${APP_DIR}" -type f -name '*.go' \
		! -name '*_test.go' \
		! -path "${DOCS_DIR}/*" \
		-newer "${DOCS_DIR}/docs.go" -print -quit 2>/dev/null || true)"
	if [ -z "${NEWER_SOURCE}" ]; then
		exit 0
	fi
fi

if ! command -v swag >/dev/null 2>&1; then
	echo "swag is required. Install it with: make swag-install" >&2
	exit 1
fi

(
	cd "${APP_DIR}"
	swag init \
		--quiet \
		--generalInfo "${GENERAL_INFO}" \
		--dir . \
		--output ./docs \
		--outputTypes go,json,yaml \
		--parseInternal
)
