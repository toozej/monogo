#!/usr/bin/env bash
set -Eeuo pipefail

# Helper function for error handling
function handle_error {
    echo "Error: $1"
    exit 1
}

# Source .env file if it exists
if [[ -f .env ]]; then
    # shellcheck disable=SC2046
    export $(grep -v '^#' .env | xargs)
fi

# Inputs (using positional arguments or environment variables)
REPO_NAME="${1:-}"
ROBOT_NAME="${2:-}"
VISIBILITY="${3:-private}" # public or private

# Extract namespace and robot name if QUAY_USERNAME contains a '+'
# E.g., if QUAY_USERNAME=toozej+github_builder
QUAY_NAMESPACE="${QUAY_NAMESPACE:-}"
if [[ -n "${QUAY_USERNAME:-}" ]]; then
    if [[ "$QUAY_USERNAME" == *"+"* ]]; then
        # Namespace is everything before '+'
        QUAY_NAMESPACE="${QUAY_NAMESPACE:-${QUAY_USERNAME%%+*}}"
        # Robot name defaults to everything after '+' if not explicitly provided
        ROBOT_NAME="${ROBOT_NAME:-${QUAY_USERNAME#*+}}"
    else
        QUAY_NAMESPACE="${QUAY_NAMESPACE:-$QUAY_USERNAME}"
    fi
fi

# Use QUAY_OAUTH_TOKEN if available, otherwise fallback to QUAY_TOKEN
QUAY_TOKEN="${QUAY_OAUTH_TOKEN:-${QUAY_TOKEN:-}}"

# Validation
if [[ -z "$REPO_NAME" ]]; then
    handle_error "Usage: $0 <repo_name> [robot_name] [visibility: public|private]"
fi

if [[ -z "$QUAY_NAMESPACE" ]]; then
    handle_error "QUAY_NAMESPACE (or QUAY_USERNAME) is not resolved. Please set it in your environment or .env file."
fi

if [[ -z "$QUAY_TOKEN" ]]; then
    handle_error "QUAY_TOKEN or QUAY_OAUTH_TOKEN is not set. Please set it in your environment or .env file. Note: This must be an OAuth 2.0 Access Token with 'repo:create' and 'repo:admin' permissions."
fi

if [[ "$VISIBILITY" != "public" && "$VISIBILITY" != "private" ]]; then
    handle_error "Visibility must be 'public' or 'private'. Got: $VISIBILITY"
fi

echo "=========================================="
echo "Creating Quay.io Repository"
echo "Repository: $QUAY_NAMESPACE/$REPO_NAME"
echo "Visibility: $VISIBILITY"
echo "=========================================="

# Create the repository via Quay.io REST API v1
# Endpoint: POST https://quay.io/api/v1/repository
# Payload: namespace, repository, visibility, description
CREATE_PAYLOAD=$(jq -n \
  --arg ns "$QUAY_NAMESPACE" \
  --arg repo "$REPO_NAME" \
  --arg vis "$VISIBILITY" \
  --arg desc "Repository for ${REPO_NAME}" \
  '{namespace: $ns, repository: $repo, visibility: $vis, description: $desc}')

echo "Sending request to create repository..."
CREATE_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
  -H "Authorization: Bearer ${QUAY_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "${CREATE_PAYLOAD}" \
  "https://quay.io/api/v1/repository")

# Extract response body and status code
CREATE_BODY=$(echo "$CREATE_RESPONSE" | sed '$d')
CREATE_STATUS=$(echo "$CREATE_RESPONSE" | tail -n1)

if [[ "$CREATE_STATUS" -eq 201 || "$CREATE_STATUS" -eq 200 ]]; then
    echo "Successfully created repository '$QUAY_NAMESPACE/$REPO_NAME'."
elif [[ "$CREATE_STATUS" -eq 400 ]] && echo "$CREATE_BODY" | grep -q "already exists"; then
    echo "Repository '$QUAY_NAMESPACE/$REPO_NAME' already exists. Proceeding to permissions configuration..."
else
    echo "Failed to create repository. HTTP Status: $CREATE_STATUS"
    echo "Response: $CREATE_BODY"
    exit 1
fi

# Set permissions for the robot user if provided
if [[ -n "$ROBOT_NAME" ]]; then
    # Standard Quay robot name format is namespace+robotname
    # Remove namespace prefix if the user passed it as 'namespace+robotname'
    CLEAN_ROBOT_NAME="${ROBOT_NAME#*+}"
    FULL_ROBOT_NAME="${QUAY_NAMESPACE}+${CLEAN_ROBOT_NAME}"

    # URL encode the '+' sign as '%2B' for the API path
    ENCODED_ROBOT_NAME="${QUAY_NAMESPACE}%2B${CLEAN_ROBOT_NAME}"

    echo ""
    echo "=========================================="
    echo "Configuring Permissions for Robot User"
    echo "Robot User: $FULL_ROBOT_NAME"
    echo "Permission: write"
    echo "=========================================="

    # Set permissions via Quay.io REST API v1
    # Endpoint: PUT /api/v1/repository/{repository}/permissions/robot/{robot}
    # Payload: {"role": "write"}
    PERM_PAYLOAD='{"role": "write"}'

    echo "Sending request to set write permissions for $FULL_ROBOT_NAME..."
    PERM_RESPONSE=$(curl -s -w "\n%{http_code}" -X PUT \
      -H "Authorization: Bearer ${QUAY_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "${PERM_PAYLOAD}" \
      "https://quay.io/api/v1/repository/${QUAY_NAMESPACE}/${REPO_NAME}/permissions/robot/${ENCODED_ROBOT_NAME}")

    PERM_BODY=$(echo "$PERM_RESPONSE" | sed '$d')
    PERM_STATUS=$(echo "$PERM_RESPONSE" | tail -n1)

    if [[ "$PERM_STATUS" -eq 200 ]]; then
        echo "Successfully granted 'write' permissions to $FULL_ROBOT_NAME."
    else
        echo "Failed to set permissions for robot user. HTTP Status: $PERM_STATUS"
        echo "Response: $PERM_BODY"
        exit 1
    fi
fi

echo ""
echo "Operation completed successfully!"
