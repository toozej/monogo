#!/usr/bin/env bash
set -Eeuo pipefail

# Helper function for error handling
function handle_error {
	echo "Error: $1" >&2
	exit 1
}

usage() {
	echo "Usage: $0 <secrets|envfile> <project_name> [env_file] [github_username]" >&2
	exit 2
}

dotenv_value() {
	local key="$1"
	local env_file="$2"
	local line name value

	while IFS= read -r line || [[ -n "${line}" ]]; do
		line="${line%$'\r'}"
		[[ "${line}" =~ ^[[:space:]]*($|#) ]] && continue

		if [[ "${line}" =~ ^[[:space:]]*(export[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*=(.*)$ ]]; then
			name="${BASH_REMATCH[2]}"
			value="${BASH_REMATCH[3]}"

			if [[ "${name}" == "${key}" ]]; then
				value="${value#"${value%%[![:space:]]*}"}"
				value="${value%"${value##*[![:space:]]}"}"

				if [[ "${value}" == \"*\" && "${value}" == *\" ]]; then
					value="${value:1:${#value}-2}"
				elif [[ "${value}" == \'*\' && "${value}" == *\' ]]; then
					value="${value:1:${#value}-2}"
				fi

				printf '%s\n' "${value}"
				return 0
			fi
		fi
	done <"${env_file}"

	return 1
}

ensure_env_file() {
	if [[ ! -f "${ENV_FILE}" ]]; then
		handle_error "Env file not found: ${ENV_FILE}"
	fi
}

ensure_1password_item() {
	if ! op item get "${NEW_PROJECT_NAME}" &>/dev/null; then
		op item create --category login --title "${NEW_PROJECT_NAME}" \
			--url "https://github.com/${GITHUB_USERNAME}/${NEW_PROJECT_NAME}" \
			--tags "Projects/${NEW_PROJECT_NAME}" || handle_error "Failed to create 1Password item."
	fi
}

store_envfile_in_1password() {
	echo "Storing ${ENV_FILE} in 1Password..."

	ensure_env_file
	ensure_1password_item

	op item edit "${NEW_PROJECT_NAME}" \
		"EnvFile[file]=${ENV_FILE}" \
		|| handle_error "Failed to update 1Password item with envfile."

	echo "EnvFile successfully stored in 1Password."
}

# Helper function to store secrets in 1Password
store_secrets_in_1password() {
	local fields

	echo "Storing secrets from ${ENV_FILE} in 1Password..."

	ensure_env_file
	ensure_1password_item

	if [[ -z "${GITHUB_TOKEN}" ]]; then
		handle_error "GITHUB_TOKEN is not set in ${ENV_FILE}."
	fi

	fields=("GH PAT[password]=${GITHUB_TOKEN}")
	if [[ -n "${COSIGN_PASSWORD}" ]]; then
		fields+=("COSIGN_PASSWORD[password]=${COSIGN_PASSWORD}")
	fi

	op item edit "${NEW_PROJECT_NAME}" "${fields[@]}" \
		|| handle_error "Failed to update 1Password item with secrets."

	echo "Secrets successfully stored in 1Password."
}


[[ $# -ge 2 && $# -le 4 ]] || usage

MODE="$1"
NEW_PROJECT_NAME="$2"
ENV_FILE="${3:-.env}"
GITHUB_USERNAME="${4:-}"

[[ -n "${NEW_PROJECT_NAME}" ]] || usage

case "${MODE}" in
	secrets | envfile)
		;;
	*)
		usage
		;;
esac

ensure_env_file

GITHUB_USERNAME="${GITHUB_USERNAME:-$(dotenv_value GITHUB_USERNAME "${ENV_FILE}" || true)}"
GITHUB_USERNAME="${GITHUB_USERNAME:-toozej}"
GITHUB_TOKEN="$(dotenv_value GITHUB_TOKEN "${ENV_FILE}" || true)"
if [[ -z "${GITHUB_TOKEN}" ]]; then
	GITHUB_TOKEN="$(dotenv_value GH_TOKEN "${ENV_FILE}" || true)"
fi
COSIGN_PASSWORD="$(dotenv_value COSIGN_PASSWORD "${ENV_FILE}" || true)"

case "${MODE}" in
	secrets)
		store_secrets_in_1password
		;;
	envfile)
		store_envfile_in_1password
		;;
	*)
		usage
		;;
esac
