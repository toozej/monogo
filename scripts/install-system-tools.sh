#!/usr/bin/env bash

set -euo pipefail

if command -v apt >/dev/null 2>&1; then
	apt-get update
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

command -v shellcheck >/dev/null 2>&1 || brew install shellcheck || apt install -y shellcheck || sudo dnf install -y ShellCheck || sudo apt install -y shellcheck
command -v dot >/dev/null 2>&1 || brew install graphviz || sudo apt install -y graphviz || sudo dnf install -y graphviz
command -v semgrep >/dev/null 2>&1 || brew install semgrep || python3 -m pip install --break-system-packages --upgrade semgrep

if [[ -f /etc/os-release ]] && grep --silent 'VERSION="12 (bookworm)"' /etc/os-release; then
	apt install -y --no-install-recommends python3-pip
	python3 -m pip install --break-system-packages --upgrade pre-commit
fi
command -v pre-commit >/dev/null 2>&1 || brew install pre-commit || sudo dnf install -y pre-commit || sudo apt install -y pre-commit
