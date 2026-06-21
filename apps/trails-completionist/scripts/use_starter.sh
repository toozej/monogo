#!/usr/bin/env bash
set -Eeuo pipefail

# --- Functions ---

# Helper function for error handling
function handle_error {
    echo "Error: $1"
    exit 1
}

# Helper function to fetch credentials from 1Password
fetch_credentials() {
    echo "Fetching credentials from 1Password..."

    GH_GHCR_TOKEN=$(op item get "github.com" --field ghcr_token --reveal) || handle_error "Failed to fetch GHCR GitHub token."
    TAP_GITHUB_TOKEN=$(op item get "github.com" --field tap_token --reveal) || handle_error "Failed to fetch Homebrew Tap GitHub token."
    DOCKERHUB_USERNAME=$(op item get "docker.com" --field username) || handle_error "Failed to fetch DockerHub username."
    DOCKERHUB_TOKEN=$(op item get "docker.com" --field token --reveal) || handle_error "Failed to fetch DockerHub token."
    QUAY_USERNAME=$(op item get "Quay.io" --field username) || handle_error "Failed to fetch Quay username."
    QUAY_TOKEN=$(op item get "Quay.io" --field password --reveal) || handle_error "Failed to fetch Quay password."
    SNYK_TOKEN=$(op item get "Snyk" --field password --reveal) || handle_error "Failed to fetch Snyk token."

    # Write environment variables to .env file
    cat <<EOF >> .env
GITHUB_USERNAME=${GITHUB_USERNAME}
GH_GHCR_TOKEN=${GH_GHCR_TOKEN}
TAP_GITHUB_TOKEN=${TAP_GITHUB_TOKEN}
DOCKERHUB_USERNAME=${DOCKERHUB_USERNAME}
DOCKERHUB_TOKEN=${DOCKERHUB_TOKEN}
QUAY_USERNAME=${QUAY_USERNAME}
QUAY_TOKEN=${QUAY_TOKEN}
SNYK_TOKEN=${SNYK_TOKEN}
EOF

    echo ".env file created successfully."
}

# --- Main Script ---

# Validate script arguments
if [[ $# -lt 1 ]]; then
    handle_error "Usage: $0 <new_project_name> [github_username]"
fi

OLD_PROJECT_NAME="trails-completionist"
NEW_PROJECT_NAME="${1}"
GITHUB_USERNAME="${2:-toozej}"

GIT_REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${GIT_REPO_ROOT}"

# Register new project's GitHub fine-grained token
read -r -s -p "Enter ${NEW_PROJECT_NAME}'s GH fine-grained PAT from webpage: " USER_INPUTTED_GITHUB_TOKEN
export GITHUB_TOKEN=${USER_INPUTTED_GITHUB_TOKEN}
cat <<EOF > .env
GH_TOKEN=${USER_INPUTTED_GITHUB_TOKEN}
EOF

# Update project files
echo "Updating project from ${OLD_PROJECT_NAME} to ${NEW_PROJECT_NAME}..."

# Truncate existing CREDITS.md file and replace its contents with link to template repo's CREDITS.md file
echo -e "# Credits and Acknowledgements\n\n- https://raw.githubusercontent.com/toozej/trails-completionist/main/CREDITS.md" > CREDITS.md

# Remove old public key if it exists
rm -f "./${OLD_PROJECT_NAME}.pub" || handle_error "Failed to remove ${OLD_PROJECT_NAME}.pub"

# Update go module name
# shellcheck disable=SC2086
go mod edit -module=github.com/${GITHUB_USERNAME}/${NEW_PROJECT_NAME} || handle_error "Failed to update go module name."

# Move directories to match new project name
mv "cmd/${OLD_PROJECT_NAME}" "cmd/${NEW_PROJECT_NAME}" || handle_error "Failed to move project directories."

# Replace old project name with the new project name across files
grep --exclude-dir=.git --exclude ./CREDITS.md -rl "${OLD_PROJECT_NAME}" . | xargs sed -i -e "s/${OLD_PROJECT_NAME}/${NEW_PROJECT_NAME}/g" || handle_error "Failed to rename instances of ${OLD_PROJECT_NAME} to ${NEW_PROJECT_NAME}."

# Randomize minute for CI/CD GitHub Actions pipeline executes on Sunday evenings
RAND_MIN=$((RANDOM % 60))
sed -i "s/0 1 \* \* 1/${RAND_MIN} 1 * * 1/" .github/workflows/cicd.yaml

# Show git diff to allow verification of changes
git diff || handle_error "Failed to show git diff."

# Initialize project environment
echo "Initializing project environment..."

# Fetch credentials from 1Password
fetch_credentials

# Store generated secrets in 1Password
./scripts/upload_secrets_to_1password.sh secrets "${NEW_PROJECT_NAME}"

# Store EnvFile in 1Password
./scripts/upload_secrets_to_1password.sh envfile "${NEW_PROJECT_NAME}"

# Call the external secrets upload script
./scripts/upload_secrets_to_github.sh "${NEW_PROJECT_NAME}"

# Setup necessary GitHub repo labels
gh label create dependencies --description "Dependencies" --repo "${GITHUB_USERNAME}/${NEW_PROJECT_NAME}"

echo "Project initialization complete! You can now verify and commit the changes."
