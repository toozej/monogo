#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="${1:-${APP:-}}"
PACKAGE="${2:-${PACKAGE:-}}"

usage() {
	cat >&2 <<EOF
usage: $0 <app-name> <package-name>

Moves apps/<app-name>/internal/<package-name>/ to pkg/<package-name>/,
rewrites references from the old import path to the new shared import path,
then runs task test and task local:build for each app whose imports changed.

Example:
  task app:migrate-internal-package APP=monogo PACKAGE=starter
EOF
}

die() {
	echo "error: $*" >&2
	exit 1
}

require_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

validate_inputs() {
	if [ "${APP}" = "-h" ] || [ "${APP}" = "--help" ]; then
		usage
		exit 0
	fi

	case "$APP" in
		""|.|..|*/*|*[!A-Za-z0-9._-]*)
			die "APP must be an existing apps/<app-name> directory name"
			;;
	esac

	case "$PACKAGE" in
		""|.|./*|../*|*/.|*/..|*/./*|*/../*|/*|*/|*//*|*[!A-Za-z0-9._/-]*)
			die "PACKAGE must be a relative internal package path such as starter or foo/bar"
			;;
	esac

	[ -f "${ROOT}/go.mod" ] || die "missing go.mod at ${ROOT}/go.mod"
	[ -d "${ROOT}/apps/${APP}" ] || die "missing app directory: apps/${APP}"
	[ -d "${ROOT}/apps/${APP}/internal/${PACKAGE}" ] || die "missing internal package: apps/${APP}/internal/${PACKAGE}"
	[ ! -e "${ROOT}/pkg/${PACKAGE}" ] || die "destination already exists: pkg/${PACKAGE}"
}

move_path() {
	local src="$1"
	local dst="$2"
	local rel_src

	rel_src="${src#"${ROOT}"/}"
	mkdir -p "$(dirname "$dst")"

	if command -v git >/dev/null 2>&1 &&
		git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1 &&
		git -C "$ROOT" ls-files --error-unmatch "$rel_src" >/dev/null 2>&1; then
		git -C "$ROOT" mv "$src" "$dst"
	else
		mv "$src" "$dst"
	fi
}

rewrite_imports() {
	local rg_status=0
	local file

	: >"$updated_files_tmp"
	rg -F -l --hidden -g '!vendor/**' -g '!**/.git/**' -- "$OLD_IMPORT" apps "$DEST_REL" >"$updated_files_tmp" || rg_status=$?
	if [ "$rg_status" -gt 1 ]; then
		die "failed searching for references to ${OLD_IMPORT}"
	fi

	while IFS= read -r file; do
		[ -f "$file" ] || continue
		OLD_IMPORT="$OLD_IMPORT" NEW_IMPORT="$NEW_IMPORT" perl -0pi -e 's/\Q$ENV{OLD_IMPORT}\E(?![\w.-])/$ENV{NEW_IMPORT}/g' "$file"
	done <"$updated_files_tmp"
}

format_touched_go_files() {
	local file

	: >"$go_files_tmp"
	if [ -s "$updated_files_tmp" ]; then
		awk '/\.go$/ {print}' "$updated_files_tmp" >>"$go_files_tmp"
	fi
	find "$DEST_REL" -type f -name '*.go' >>"$go_files_tmp"
	sort -u "$go_files_tmp" -o "$go_files_tmp"

	while IFS= read -r file; do
		[ -f "$file" ] || continue
		gofmt -w "$file"
	done <"$go_files_tmp"
}

collect_affected_apps() {
	: >"$affected_apps_tmp"
	if [ -s "$updated_files_tmp" ]; then
		awk -F/ '$1 == "apps" && NF >= 3 {print $2}' "$updated_files_tmp" | sort -u >"$affected_apps_tmp"
	fi
}

cleanup_empty_internal_dirs() {
	if [ -d "apps/${APP}/internal" ]; then
		find "apps/${APP}/internal" -depth -type d -empty -delete
	fi
}

run_task_check() {
	local app="$1"
	local target="$2"
	local log_file="$3"
	local status=0

	printf '  - task %s APP=%s: ' "$target" "$app"
	if task --dir "$ROOT" "$target" APP="$app" >"$log_file" 2>&1; then
		echo "ok"
		return 0
	else
		status=$?
	fi

	echo "failed (exit ${status}, log: ${log_file#"${ROOT}"/})"
	printf '%s\t%s\t%s\t%s\n' "$app" "$target" "$status" "${log_file#"${ROOT}"/}" >>"$failures_tmp"
	return 1
}

verify_affected_apps() {
	local app
	local app_failed
	local package_log_name
	local timestamp

	: >"$manual_apps_tmp"
	: >"$failures_tmp"

	if [ ! -s "$affected_apps_tmp" ]; then
		echo "No app import paths changed; skipping app build/test checks."
		return 0
	fi

	timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
	package_log_name="${PACKAGE//\//__}"
	LOG_DIR="${MIGRATE_LOG_DIR:-${ROOT}/out/internal-package-migrations/${timestamp}-${APP}-${package_log_name}}"
	mkdir -p "$LOG_DIR"

	echo "Verifying affected apps; logs will be written under ${LOG_DIR#"${ROOT}"/}"
	while IFS= read -r app; do
		[ -n "$app" ] || continue
		app_failed=0
		echo "Checking ${app}"
		run_task_check "$app" test "${LOG_DIR}/${app}-test.log" || app_failed=1
		run_task_check "$app" local:build "${LOG_DIR}/${app}-local-build.log" || app_failed=1
		if [ "$app_failed" -ne 0 ]; then
			echo "$app" >>"$manual_apps_tmp"
		fi
	done <"$affected_apps_tmp"
}

print_summary() {
	local updated_count
	local app_count
	local go_count
	local app
	local target
	local status
	local log_file

	updated_count="$(wc -l <"$updated_files_tmp" | tr -d ' ')"
	app_count="$(wc -l <"$affected_apps_tmp" | tr -d ' ')"
	go_count="$(wc -l <"$go_files_tmp" | tr -d ' ')"

	cat <<EOF

Migration summary
- moved apps/${APP}/internal/${PACKAGE} -> pkg/${PACKAGE}
- rewrote ${updated_count} file(s) from ${OLD_IMPORT} to ${NEW_IMPORT}
- formatted ${go_count} Go file(s)
- affected app(s): ${app_count}
EOF

	if [ -s "$affected_apps_tmp" ]; then
		while IFS= read -r app; do
			echo "  - ${app}"
		done <"$affected_apps_tmp"
	fi

	if [ ! -s "$failures_tmp" ]; then
		if [ -s "$affected_apps_tmp" ]; then
			echo
			echo "All affected apps passed task test and task local:build."
		fi
		return 0
	fi

	echo
	echo "Apps needing manual intervention:"
	sort -u "$manual_apps_tmp" | while IFS= read -r app; do
		[ -n "$app" ] || continue
		echo "  - ${app}"
	done

	echo
	echo "Failed checks:"
	while IFS=$'\t' read -r app target status log_file; do
		echo "  - ${app}: task ${target} failed with exit ${status} (${log_file})"
	done <"$failures_tmp"

	return 1
}

validate_inputs
require_cmd gofmt
require_cmd task
require_cmd perl
require_cmd rg

cd "$ROOT"

ROOT_MODULE="$(awk '/^module / {print $2; exit}' go.mod)"
[ -n "$ROOT_MODULE" ] || die "could not read module path from go.mod"

SOURCE_DIR="${ROOT}/apps/${APP}/internal/${PACKAGE}"
DEST_DIR="${ROOT}/pkg/${PACKAGE}"
DEST_REL="pkg/${PACKAGE}"
OLD_IMPORT="${ROOT_MODULE}/apps/${APP}/internal/${PACKAGE}"
NEW_IMPORT="${ROOT_MODULE}/pkg/${PACKAGE}"

updated_files_tmp="$(mktemp)"
go_files_tmp="$(mktemp)"
affected_apps_tmp="$(mktemp)"
manual_apps_tmp="$(mktemp)"
failures_tmp="$(mktemp)"
trap 'rm -f "$updated_files_tmp" "$go_files_tmp" "$affected_apps_tmp" "$manual_apps_tmp" "$failures_tmp"' EXIT

echo "Promoting ${OLD_IMPORT} to ${NEW_IMPORT}"
move_path "$SOURCE_DIR" "$DEST_DIR"
rewrite_imports
format_touched_go_files
collect_affected_apps
cleanup_empty_internal_dirs
verify_affected_apps
print_summary
