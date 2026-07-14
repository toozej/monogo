#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="${ROOT}/tools/go-tools.tsv"
TOOLS_BIN="${TOOLS_BIN:-${ROOT}/.tools/bin}"

usage() {
	echo "usage: $0 <install [tool ...]|update>" >&2
	exit 2
}

require_tool() {
	local requested="$1"
	if ! awk -F '\t' -v name="${requested}" '$1 == name { found = 1 } END { exit !found }' "${MANIFEST}"; then
		echo "unknown pinned Go tool: ${requested}" >&2
		exit 2
	fi
}

install_tools() {
	local requested name package _module version
	for requested in "$@"; do
		require_tool "${requested}"
	done

	mkdir -p "${TOOLS_BIN}"
	# `|| [[ -n "${name}" ]]` so a final manifest line lacking a trailing
	# newline is still processed rather than silently dropped by `read`.
	while IFS=$'\t' read -r name package _module version || [[ -n "${name}" ]]; do
		if [[ -z "${name}" || "${name}" == \#* ]]; then
			continue
		fi
		if (( $# > 0 )); then
			local selected=false
			for requested in "$@"; do
				if [[ "${requested}" == "${name}" ]]; then
					selected=true
					break
				fi
			done
			if [[ "${selected}" != true ]]; then
				continue
			fi
		fi

		# Skip the (potentially slow, from-source) `go install` when the pinned
		# binary is already present. A stamp records package@version so a manifest
		# bump forces a rebuild, while an unchanged tool (e.g. from a restored
		# cache) is reused.
		local stamp="${TOOLS_BIN}/.stamps/${name}"
		if [[ -x "${TOOLS_BIN}/${name}" && -f "${stamp}" \
			&& "$(cat "${stamp}")" == "${package}@${version}" ]]; then
			echo "Skipping ${name} (${package}@${version} already installed)"
			continue
		fi

		echo "Installing ${name} (${package}@${version})"
		GOBIN="${TOOLS_BIN}" go install "${package}@${version}"
		mkdir -p "${TOOLS_BIN}/.stamps"
		printf '%s@%s\n' "${package}" "${version}" >"${stamp}"
	done <"${MANIFEST}"
}

update_tools() {
	local name package module version latest updated=false
	local temp
	temp="$(mktemp)"
	trap 'rm -f "${temp}"' EXIT

	while IFS=$'\t' read -r name package module version || [[ -n "${name}" ]]; do
		if [[ -z "${name}" || "${name}" == \#* ]]; then
			printf '%s\t%s\t%s\t%s\n' "${name}" "${package}" "${module}" "${version}" >>"${temp}"
			continue
		fi

		# `-mod=mod` so the network `@latest` query still works when a vendor/
		# directory is present (Go otherwise defaults to -mod=vendor and refuses).
		latest="$(go list -mod=mod -m -f '{{.Version}}' "${module}@latest")"
		if [[ -z "${latest}" ]]; then
			echo "could not resolve latest version for ${module}" >&2
			exit 1
		fi
		if [[ "${latest}" != "${version}" ]]; then
			echo "Updating ${name}: ${version} -> ${latest}"
			version="${latest}"
			updated=true
		fi
		printf '%s\t%s\t%s\t%s\n' "${name}" "${package}" "${module}" "${version}" >>"${temp}"
	done <"${MANIFEST}"

	mv "${temp}" "${MANIFEST}"
	trap - EXIT
	if [[ "${updated}" != true ]]; then
		echo "Pinned Go tools are already current"
	fi
}

case "${1:-}" in
install)
	shift
	install_tools "$@"
	;;
update)
	if (( $# != 1 )); then
		usage
	fi
	update_tools
	;;
*)
	usage
	;;
esac
