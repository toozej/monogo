#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $# -ne 1 || ! $1 =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "usage: $0 <Go version, for example 1.27.2>" >&2
    exit 2
fi

if ! command -v go >/dev/null 2>&1; then
    echo "Go is required to update go.mod" >&2
    exit 1
fi

repo_root=$(git rev-parse --show-toplevel)
cd "${repo_root}"

new_version=$1
old_version=$(awk '$1 == "go" { print $2; exit }' go.mod)
if [[ -z ${old_version} ]]; then
    echo "go.mod has no Go version" >&2
    exit 1
fi

# GNU sed uses the same in-place syntax on Linux and macOS.
if [[ $(uname -s) == Darwin ]]; then
    sed_command="gsed"
else
    sed_command="sed"
fi

if [[ ${old_version} != "${new_version}" ]]; then
    go mod edit -go="${new_version}"
fi

# Change tracked active files. Preserve historical imports under .monogo and
# tool versions that happen to contain the old Go version number.
old_pattern=${old_version//./[.]}
while IFS= read -r -d '' file; do
    case ${file} in
        apps/*/.monogo/*) continue ;;
        *.yaml|*.yml|*Dockerfile*|*.md|*.sh) ;;
        *) continue ;;
    esac

    if grep -Fq "${old_version}" "${file}"; then
        "${sed_command}" -i -e "s/${old_pattern}/${new_version}/g" "${file}"
    fi

    # Some active guides have an older minimum version than go.mod.
    if [[ ${file} == *.md ]]; then
        "${sed_command}" -i -E \
            -e "s/(Go[[:space:]]+)[0-9]+[.][0-9]+([.][0-9]+)?/\1${new_version}/g" \
            -e "s/(Go[[:space:]]+\(version[[:space:]]+)[0-9]+[.][0-9]+([.][0-9]+)?/\1${new_version}/g" \
            -e "s/(Go[[:space:]]+[Vv]ersion:[*]*[[:space:]]*)[0-9]+[.][0-9]+([.][0-9]+)?/\1${new_version}/g" \
            "${file}"
    fi
done < <(git ls-files -z)

# The dev container can have an older Go image than go.mod.
devcontainer_config=.devcontainer/devcontainer.json
if [[ -f ${devcontainer_config} ]]; then
    "${sed_command}" -i -E \
        "s#(\"GO_IMAGE\"[[:space:]]*:[[:space:]]*\"golang:)[0-9]+[.][0-9]+([.][0-9]+)?(-[^\"]+\")#\1${new_version}\3#" \
        "${devcontainer_config}"
fi
