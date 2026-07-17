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
