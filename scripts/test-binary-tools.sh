#!/usr/bin/env bash

# Test release installation without network access or changes to .tools/bin.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
temporary="$(mktemp -d)"
trap 'rm -rf "${temporary}"' EXIT
export BINARY_TOOL_MANIFEST="${temporary}/tools.tsv"
export TOOLS_BIN="${temporary}/bin"
export BINARY_TEST_ASSETS="${temporary}/assets"
export BINARY_TEST_LOG="${temporary}/requests"
mkdir -p "${temporary}/mock" "${BINARY_TEST_ASSETS}" "${temporary}/package/nested"

cat >"${temporary}/mock/curl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
url= output=
while (( $# )); do
	case "$1" in
		-o) output="$2"; shift 2 ;;
		https://*) url="$1"; shift ;;
		*) shift ;;
	esac
done
printf '%s\n' "${url}" >>"${BINARY_TEST_LOG}"
if [[ "${url}" == */releases/latest ]]; then
	printf '{"tag_name": "v2.0.0"}\n'
else
	cp "${BINARY_TEST_ASSETS}/${url##*/}" "${output}"
fi
MOCK
cat >"${temporary}/mock/uname" <<'MOCK'
#!/usr/bin/env bash
case "$1" in
	-s) echo "${BINARY_TEST_OS:-Linux}" ;;
	-m) echo "${BINARY_TEST_ARCH:-x86_64}" ;;
esac
MOCK
cat >"${temporary}/package/tool" <<'BINARY'
#!/usr/bin/env bash
echo 'fixture version 1.0.0'
BINARY
chmod +x "${temporary}/mock/"* "${temporary}/package/tool"
export PATH="${temporary}/mock:${PATH}"
cp "${temporary}/package/tool" "${temporary}/package/nested/tool"
cp "${temporary}/package/tool" "${BINARY_TEST_ASSETS}/raw"
tar -czf "${BINARY_TEST_ASSETS}/tool_1.0.0.tar.gz" -C "${temporary}/package" tool
(cd "${temporary}/package" && zip -q "${BINARY_TEST_ASSETS}/nested.zip" nested/tool)

row() {
	printf '%s\ttest/repo\tv1.0.0\t%s\t%s\t%s\t%s' "$1" "$2" "$2" "$2" "$2"
	if (( $# == 3 )); then printf '\t%s' "$3"; fi
	printf '\n'
}
{
	# The legacy seven-column format must remain valid.
	row raw raw
	row tar 'tool_{version}.tar.gz' tool
	row zip nested.zip '{archive}/tool'
} >"${BINARY_TOOL_MANIFEST}"

"${ROOT}/scripts/manage-binary-tools.sh" install
for tool in raw tar zip; do
	cmp "${temporary}/package/tool" "${TOOLS_BIN}/${tool}"
done
requests="$(wc -l <"${BINARY_TEST_LOG}")"
"${ROOT}/scripts/manage-binary-tools.sh" install
test "$(wc -l <"${BINARY_TEST_LOG}")" -eq "${requests}"

# Check every platform mapping, including a changed asset at the same version.
for asset in linux64 linuxarm mac64 macarm; do
	cp "${temporary}/package/tool" "${BINARY_TEST_ASSETS}/${asset}"
done
printf 'platform\ttest/repo\tv1.0.0\tlinux64\tlinuxarm\tmac64\tmacarm\t-\n' >"${BINARY_TOOL_MANIFEST}"
while read -r os arch asset; do
	BINARY_TEST_OS="${os}" BINARY_TEST_ARCH="${arch}" "${ROOT}/scripts/manage-binary-tools.sh" install platform
	[[ "$(tail -n 1 "${BINARY_TEST_LOG}")" == */"${asset}" ]]
done <<'PLATFORMS'
Linux x86_64 linux64
Linux aarch64 linuxarm
Darwin x86_64 mac64
Darwin arm64 macarm
PLATFORMS

# A failed replacement must retain the old binary and stamp and remove scratch files.
cp "${TOOLS_BIN}/.stamps/tar" "${temporary}/old-stamp"
for asset in missing.tar.gz corrupt.tar.gz wrong-member.tar.gz bad-version.tar.gz; do
	printf 'invalid archive\n' >"${BINARY_TEST_ASSETS}/corrupt.tar.gz"
	tar -czf "${BINARY_TEST_ASSETS}/wrong-member.tar.gz" -C "${temporary}/package" nested/tool
	printf '#!/usr/bin/env bash\nexit 1\n' >"${temporary}/package/tool"
	tar -czf "${BINARY_TEST_ASSETS}/bad-version.tar.gz" -C "${temporary}/package" tool
	row tar "${asset}" tool >"${BINARY_TOOL_MANIFEST}"
	if "${ROOT}/scripts/manage-binary-tools.sh" install tar >"${temporary}/failure.log" 2>&1; then
		echo "Expected installation failure for ${asset}" >&2
		exit 1
	fi
	cmp "${BINARY_TEST_ASSETS}/raw" "${TOOLS_BIN}/tar"
	cmp "${temporary}/old-stamp" "${TOOLS_BIN}/.stamps/tar"
	if compgen -G "${TOOLS_BIN}/.tar.*" >/dev/null; then
		echo 'Installer left temporary files after failure.' >&2
		exit 1
	fi
done

row tar 'tool_{version}.tar.gz' tool >"${BINARY_TOOL_MANIFEST}"
"${ROOT}/scripts/manage-binary-tools.sh" update
test "$(cut -f 3 "${BINARY_TOOL_MANIFEST}")" = v2.0.0
test "$(cut -f 4 "${BINARY_TOOL_MANIFEST}")" = 'tool_{version}.tar.gz'
test "$(cut -f 8 "${BINARY_TOOL_MANIFEST}")" = tool
cp "${BINARY_TEST_ASSETS}/tool_1.0.0.tar.gz" "${BINARY_TEST_ASSETS}/tool_2.0.0.tar.gz"
"${ROOT}/scripts/manage-binary-tools.sh" install tar
[[ "$(tail -n 1 "${BINARY_TEST_LOG}")" == */v2.0.0/tool_2.0.0.tar.gz ]]
echo 'Binary installer tests passed.'
