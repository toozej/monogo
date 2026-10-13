#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="${BINARY_TOOL_MANIFEST:-${ROOT}/tools/binary-tools.tsv}"
TOOLS_BIN="${TOOLS_BIN:-${ROOT}/.tools/bin}"

usage() {
	echo "usage: $0 <install [tool ...]|update>" >&2
	exit 2
}

require_tool() {
	local requested="$1"
	if ! awk -F '\t' -v name="${requested}" '$1 == name { found = 1 } END { exit !found }' "${MANIFEST}"; then
		echo "unknown configured binary tool: ${requested}" >&2
		exit 2
	fi
}

is_selected() {
	local name="$1"
	shift
	if (( $# == 0 )); then
		return 0
	fi
	local requested
	for requested in "$@"; do
		if [[ "${requested}" == "${name}" ]]; then
			return 0
		fi
	done
	return 1
}

asset_for_platform() {
	local linux_amd64="$1" linux_arm64="$2" darwin_amd64="$3" darwin_arm64="$4"
	case "$(uname -s)/$(uname -m)" in
		Linux/x86_64|Linux/amd64) printf '%s\n' "${linux_amd64}" ;;
		Linux/aarch64|Linux/arm64) printf '%s\n' "${linux_arm64}" ;;
		Darwin/x86_64|Darwin/amd64) printf '%s\n' "${darwin_amd64}" ;;
		Darwin/arm64|Darwin/aarch64) printf '%s\n' "${darwin_arm64}" ;;
		*) echo "unsupported platform: $(uname -s)/$(uname -m)" >&2; exit 1 ;;
	esac
}

install_tools() {
	local requested name repository version linux_amd64 linux_arm64 darwin_amd64 darwin_arm64 member
	for requested in "$@"; do
		require_tool "${requested}"
	done

	mkdir -p "${TOOLS_BIN}" "${TOOLS_BIN}/.stamps"
	while IFS=$'\t' read -r name repository version linux_amd64 linux_arm64 darwin_amd64 darwin_arm64 member || [[ -n "${name}" ]]; do
		if [[ -z "${name}" || "${name}" == \#* ]] || ! is_selected "${name}" "$@"; then
			continue
		fi

		local asset archive stamp_contents
		asset="$(asset_for_platform "${linux_amd64}" "${linux_arm64}" "${darwin_amd64}" "${darwin_arm64}")"
		# Release filenames can contain the version without its leading v.
		asset="${asset//\{version\}/${version#v}}"
		archive="${asset%.tar.gz}"
		archive="${archive%.tgz}"
		archive="${archive%.zip}"
		member="${member//\{archive\}/${archive}}"
		local stamp="${TOOLS_BIN}/.stamps/${name}"
		stamp_contents="${repository}@${version} ${asset} ${member:--}"
		if [[ -x "${TOOLS_BIN}/${name}" && -f "${stamp}" \
			&& "$(cat "${stamp}")" == "${stamp_contents}" ]]; then
			echo "Skipping ${name} (${repository}@${version} already installed)"
			continue
		fi

		echo "Installing ${name} (${repository}@${version})"
		# A subshell gives each download its own cleanup trap. Extract only the
		# configured member to stdout, never arbitrary archive paths to disk.
		(
			set -euo pipefail
			temporary="$(mktemp -d "${TOOLS_BIN}/.${name}.XXXXXX")"
			trap 'rm -rf "${temporary}"' EXIT
			curl --fail --silent --show-error --location --retry 3 --connect-timeout 20 \
				"https://github.com/${repository}/releases/download/${version}/${asset}" -o "${temporary}/download"
			if [[ -z "${member}" || "${member}" == - ]]; then
				mv "${temporary}/download" "${temporary}/binary"
			else
				case "${asset}" in
					*.tar.gz|*.tgz) tar -xzOf "${temporary}/download" "${member}" >"${temporary}/binary" ;;
					*.zip) unzip -p "${temporary}/download" "${member}" >"${temporary}/binary" ;;
					*) echo "unsupported release archive: ${asset}" >&2; exit 1 ;;
				esac
			fi
			chmod 0755 "${temporary}/binary"
			"${temporary}/binary" --version
			mv "${temporary}/binary" "${TOOLS_BIN}/${name}"
			printf '%s\n' "${stamp_contents}" >"${stamp}"
		)
	done <"${MANIFEST}"
}

update_tools() {
	local name repository version linux_amd64 linux_arm64 darwin_amd64 darwin_arm64 member latest updated=false
	temporary="$(mktemp)"
	trap 'rm -f "${temporary}"' EXIT

	while IFS=$'\t' read -r name repository version linux_amd64 linux_arm64 darwin_amd64 darwin_arm64 member || [[ -n "${name}" ]]; do
		if [[ -z "${name}" || "${name}" == \#* ]]; then
			printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "${name}" "${repository}" "${version}" "${linux_amd64}" "${linux_arm64}" "${darwin_amd64}" "${darwin_arm64}" "${member}" >>"${temporary}"
			continue
		fi

		latest="$(curl --fail --silent --show-error "https://api.github.com/repos/${repository}/releases/latest" | awk -F '"' '
			/"tag_name":/ && !found {
				print $4
				found = 1
			}
		')"
		if [[ -z "${latest}" ]]; then
			echo "could not resolve latest release for ${repository}" >&2
			exit 1
		fi
		if [[ "${latest}" != "${version}" ]]; then
			echo "Updating ${name}: ${version} -> ${latest}"
			version="${latest}"
			updated=true
		fi
		printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "${name}" "${repository}" "${version}" "${linux_amd64}" "${linux_arm64}" "${darwin_amd64}" "${darwin_arm64}" "${member}" >>"${temporary}"
	done <"${MANIFEST}"

	mv "${temporary}" "${MANIFEST}"
	trap - EXIT
	if [[ "${updated}" != true ]]; then
		echo "Configured binary tools are already current"
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
