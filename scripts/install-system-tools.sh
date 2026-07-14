#!/usr/bin/env bash

set -euo pipefail

run_as_root() {
	if [[ ${EUID} -eq 0 ]]; then
		"$@"
	elif command -v sudo >/dev/null 2>&1; then
		sudo "$@"
	else
		echo "Cannot run $1 as root: sudo is not installed." >&2
		return 1
	fi
}

install_system_package() {
	local brew_package=$1
	local apt_package=$2
	local dnf_package=$3

	if command -v brew >/dev/null 2>&1; then
		brew install "${brew_package}"
	elif command -v apt-get >/dev/null 2>&1; then
		run_as_root apt-get install -y "${apt_package}"
	elif command -v dnf >/dev/null 2>&1; then
		run_as_root dnf install -y "${dnf_package}"
	else
		echo "No supported package manager found to install ${brew_package}." >&2
		return 1
	fi
}

if command -v apt-get >/dev/null 2>&1; then
	run_as_root apt-get update
fi

if ! command -v uv >/dev/null 2>&1; then
	if command -v brew >/dev/null 2>&1; then
		brew install uv
	elif command -v pipx >/dev/null 2>&1; then
		pipx install uv >/dev/null 2>&1 || pipx install uv || pipx upgrade uv || true
	else
		python3 -m pip install --break-system-packages --upgrade uv ||
			python3 -m pip install --user --upgrade uv ||
			echo "uv not found; install from https://docs.astral.sh/uv/"
	fi
fi

command -v shellcheck >/dev/null 2>&1 || install_system_package shellcheck shellcheck ShellCheck
command -v dot >/dev/null 2>&1 || install_system_package graphviz graphviz graphviz
command -v semgrep >/dev/null 2>&1 || brew install semgrep || python3 -m pip install --break-system-packages --upgrade semgrep

if [[ -f /etc/os-release ]] && grep --silent 'VERSION="12 (bookworm)"' /etc/os-release; then
	run_as_root apt-get install -y --no-install-recommends python3-pip
	python3 -m pip install --break-system-packages --upgrade pre-commit
fi
command -v pre-commit >/dev/null 2>&1 || install_system_package pre-commit pre-commit pre-commit
