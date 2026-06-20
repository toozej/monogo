#!/usr/bin/env bash
set -Eeuo pipefail

# Helper function for error handling
function handle_error {
	echo "Error: $1" >&2
	exit 1
}

usage() {
	echo "Usage: $0 <repo_name> [env_file] [cosign_key_file]" >&2
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

cleanup() {
	if [[ -n "${GITHUB_SECRET_ENV_FILE:-}" && -f "${GITHUB_SECRET_ENV_FILE}" ]]; then
		rm -f "${GITHUB_SECRET_ENV_FILE}"
	fi
}

build_github_secret_env_file() {
	local source_file="$1"
	local dest_file="$2"
	local line name
	local found_gh_token=0

	: >"${dest_file}"

	while IFS= read -r line || [[ -n "${line}" ]]; do
		line="${line%$'\r'}"

		if [[ "${line}" =~ ^[[:space:]]*(export[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*= ]]; then
			name="${BASH_REMATCH[2]}"

			if [[ "${name}" == "GITHUB_TOKEN" ]]; then
				continue
			fi

			if [[ "${name}" == "GH_TOKEN" ]]; then
				found_gh_token=1
			fi
		fi

		printf '%s\n' "${line}" >>"${dest_file}"
	done <"${source_file}"

	if [[ "${found_gh_token}" -eq 0 ]]; then
		printf 'GH_TOKEN=%s\n' "${GITHUB_TOKEN}" >>"${dest_file}"
	fi
}

[[ $# -ge 1 && $# -le 3 ]] || usage

# Main script arguments
REPO_NAME="$1"
ENV_FILE="${2:-.env}"
COSIGN_KEY_FILE="${3:-}"

[[ -n "${REPO_NAME}" ]] || usage

# Validate that .env exists
if [[ ! -f "${ENV_FILE}" ]]; then
	handle_error "Env file not found: ${ENV_FILE}"
fi

# Read GitHub username and token from the environment
GITHUB_USERNAME="$(dotenv_value GITHUB_USERNAME "${ENV_FILE}" || true)"
GITHUB_TOKEN="$(dotenv_value GITHUB_TOKEN "${ENV_FILE}" || true)"
if [[ -z "${GITHUB_TOKEN}" ]]; then
	GITHUB_TOKEN="$(dotenv_value GH_TOKEN "${ENV_FILE}" || true)"
fi

if [[ -z "${GITHUB_USERNAME}" ]]; then
	handle_error "GITHUB_USERNAME is not set in ${ENV_FILE}."
elif [[ -z "${GITHUB_TOKEN}" ]]; then
	handle_error "GITHUB_TOKEN is not set in ${ENV_FILE}."
fi

GITHUB_SECRET_ENV_FILE="$(mktemp)"
trap cleanup EXIT
chmod 0600 "${GITHUB_SECRET_ENV_FILE}"
build_github_secret_env_file "${ENV_FILE}" "${GITHUB_SECRET_ENV_FILE}"

# Helper function to upload secrets to GitHub Actions
upload_secrets_to_github() {
	echo "Pushing ${ENV_FILE} entries to GitHub Actions secrets for repo: ${GITHUB_USERNAME}/${REPO_NAME}..."
	GH_TOKEN="${GITHUB_TOKEN}" gh secret set --repo "${GITHUB_USERNAME}/${REPO_NAME}" --app actions --env-file "${GITHUB_SECRET_ENV_FILE}"

	if [[ -n "${COSIGN_KEY_FILE}" && -f "${COSIGN_KEY_FILE}" ]]; then
		echo "Pushing ${COSIGN_KEY_FILE} to GitHub Actions secret COSIGN_PRIVATE_KEY..."
		GH_TOKEN="${GITHUB_TOKEN}" gh secret set COSIGN_PRIVATE_KEY --repo "${GITHUB_USERNAME}/${REPO_NAME}" --app actions <"${COSIGN_KEY_FILE}"
	elif [[ -n "${COSIGN_KEY_FILE}" ]]; then
		echo "Cosign key file not found at ${COSIGN_KEY_FILE}; skipping COSIGN_PRIVATE_KEY upload."
	fi

	echo "Secrets successfully uploaded to GitHub Actions."
}

# Helper function to upload secrets to GitHub secrets for use by Dependabot
upload_secrets_to_dependabot() {
	echo "Pushing ${ENV_FILE} entries to GitHub secrets for use by Dependabot for repo: ${GITHUB_USERNAME}/${REPO_NAME}..."
	GH_TOKEN="${GITHUB_TOKEN}" gh secret set --repo "${GITHUB_USERNAME}/${REPO_NAME}" --app dependabot --env-file "${GITHUB_SECRET_ENV_FILE}"
	echo "Secrets successfully uploaded to GitHub Dependabot."
}

# Execute the functions to upload secrets
upload_secrets_to_github
upload_secrets_to_dependabot
