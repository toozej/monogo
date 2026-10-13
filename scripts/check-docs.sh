#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLS_BIN="${TOOLS_BIN:-${ROOT}/.tools/bin}"
cd "${ROOT}"

checker="${1:-}"
case "${checker}" in
	vale|lychee) shift ;;
	*) echo "usage: $0 <vale|lychee> [Markdown file ...]" >&2; exit 2 ;;
esac

# docs-check exports FILE as data, so spaces and shell characters stay literal.
# Explicit hook or script arguments take precedence over the Make selection.
if (( $# == 0 )) && [[ "${DOCS_CHECK_FILE+x}" == x ]]; then
	if [[ -z "${DOCS_CHECK_FILE}" ]]; then
		echo 'No FILE specified. Checking all documentation files.'
	elif [[ -f "${DOCS_CHECK_FILE}" ]]; then
		set -- "${DOCS_CHECK_FILE}"
	else
		printf 'File not found: %s. Checking all documentation files.\n' "${DOCS_CHECK_FILE}"
	fi
fi

files=()
add_file() {
	local file="${1#"${ROOT}/"}"
	file="${file#./}"
	case "/${file}" in
		*/.vale/*|*/.tools/*|*/.monogo/*|*/vendor/*|*/node_modules/*|*/dist/*) return ;;
	esac
	case "${file}" in
		*.md|*.markdown)
			# Prefix paths so a filename cannot become a command-line option.
			if [[ "${file}" == /* ]]; then
				files+=("${file}")
			else
				files+=("./${file}")
			fi
			;;
	esac
}

if (( $# > 0 )); then
	for file in "$@"; do add_file "${file}"; done
else
	# Include new documentation, but exclude ignored downloads and build output.
	# NUL delimiters preserve spaces and newlines in filenames.
	while IFS= read -r -d '' file; do
		[[ -f "${file}" ]] && add_file "${file}"
	done < <(git ls-files --cached --others --exclude-standard --deduplicate -z -- '*.md' '*.markdown')
fi

if (( ${#files[@]} == 0 )); then
	echo "No Markdown files to check."
	exit 0
fi

case "${checker}" in
	vale)
		make --no-print-directory vale-sync TOOLS_BIN="${TOOLS_BIN}"
		# Vale reports all configured levels, but only errors change its exit code.
		exec "${TOOLS_BIN}/vale" --config="${ROOT}/.vale.ini" --no-global "${files[@]}"
		;;
	lychee)
		make --no-print-directory lychee-install TOOLS_BIN="${TOOLS_BIN}"
		# Two verbosity flags show successful links and their redirect chains.
		exec "${TOOLS_BIN}/lychee" -v -v --config "${ROOT}/.lychee.toml" "${files[@]}"
		;;
esac
