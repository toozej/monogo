#!/bin/bash

#########################################################################
# Podgrab Project Update Script
#########################################################################
# Comprehensive update script for all project dependencies:
#   - Go dependencies (go.mod/go.sum)
#   - GitHub Actions workflow versions
#   - Docker configuration (Dockerfile, docker-compose.yml)
#   - Go version directives
#
# Usage:
#   ./scripts/update-project.sh [options]
#
# Options:
#   --go-only          Update only Go dependencies
#   --workflows-only   Update only GitHub Actions workflows
#   --docker-only      Update only Docker/docker-compose files
#   --patch            Update to latest patch versions only (safer)
#   --minor            Update to latest minor versions (default)
#   --major            Update to latest major versions (may break)
#   --dry-run          Show what would be updated without making changes
#   --no-backup        Skip creating backup files
#########################################################################

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default options
UPDATE_GO=true
UPDATE_WORKFLOWS=true
UPDATE_DOCKER=true
UPDATE_LEVEL="minor"
DRY_RUN=false
CREATE_BACKUP=true
SUGGESTED_GO_VERSION="1.21.6" # Latest stable Go version

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --go-only)
      UPDATE_GO=true
      UPDATE_WORKFLOWS=false
      shift
      ;;
    --workflows-only)
      UPDATE_GO=false
      UPDATE_WORKFLOWS=true
      UPDATE_DOCKER=false
      shift
      ;;
    --docker-only)
      UPDATE_GO=false
      UPDATE_WORKFLOWS=false
      UPDATE_DOCKER=true
      shift
      ;;
    --patch)
      UPDATE_LEVEL="patch"
      shift
      ;;
    --minor)
      UPDATE_LEVEL="minor"
      shift
      ;;
    --major)
      UPDATE_LEVEL="major"
      shift
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --no-backup)
      CREATE_BACKUP=false
      shift
      ;;
    -h | --help)
      echo "Usage: $0 [options]"
      echo ""
      echo "Options:"
      echo "  --go-only          Update only Go dependencies"
      echo "  --workflows-only   Update only GitHub Actions workflows"
      echo "  --docker-only      Update only Docker and docker-compose files"
      echo "  --patch            Update to latest patch versions only (safer)"
      echo "  --minor            Update to latest minor versions (default)"
      echo "  --major            Update to latest major versions (may break)"
      echo "  --dry-run          Show what would be updated without making changes"
      echo "  --no-backup        Skip creating backup files"
      echo "  -h, --help         Show this help message"
      exit 0
      ;;
    *)
      echo -e "${RED}Unknown option: $1${NC}"
      exit 1
      ;;
  esac
done

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo -e "${BLUE}╔════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   Podgrab Dependency Update Script        ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}Update level: ${UPDATE_LEVEL}${NC}"
echo -e "${YELLOW}Dry run: ${DRY_RUN}${NC}"
echo ""

#########################################################################
# Backup Function
#########################################################################
create_backup() {
  local file=$1
  if [ "$CREATE_BACKUP" = true ] && [ "$DRY_RUN" = false ]; then
    local backup_file
    backup_file="${file}.backup.$(date +%Y%m%d_%H%M%S)"
    cp "$file" "$backup_file"
    echo -e "${GREEN}  ✓ Backup created: $backup_file${NC}"
  fi
}

#########################################################################
# Update Go Dependencies
#########################################################################
update_go_dependencies() {
  echo -e "${BLUE}═══════════════════════════════════════════${NC}"
  echo -e "${BLUE}Updating Go Dependencies${NC}"
  echo -e "${BLUE}═══════════════════════════════════════════${NC}"
  echo ""

  # Check if go.mod exists
  if [ ! -f "go.mod" ]; then
    echo -e "${RED}Error: go.mod not found${NC}"
    return 1
  fi

  # Create backup
  create_backup "go.mod"
  create_backup "go.sum"

  if [ "$DRY_RUN" = true ]; then
    echo -e "${YELLOW}[DRY RUN] Would update Go dependencies${NC}"
    echo ""
    echo "Current dependencies:"
    go list -m -u all | grep -v "^github.com/akhilrex/podgrab"
    return 0
  fi

  # Update dependencies based on level
  case $UPDATE_LEVEL in
    patch)
      echo -e "${YELLOW}Updating to latest patch versions...${NC}"
      go get -u=patch ./...
      ;;
    minor)
      echo -e "${YELLOW}Updating to latest minor versions...${NC}"
      go get -u ./...
      ;;
    major)
      echo -e "${YELLOW}Updating to latest major versions...${NC}"
      echo -e "${RED}Warning: This may introduce breaking changes!${NC}"
      go get -u ./...
      # For major version updates, might need manual intervention
      ;;
  esac

  # Tidy up
  echo -e "${YELLOW}Running go mod tidy...${NC}"
  go mod tidy

  # Verify
  echo -e "${YELLOW}Verifying dependencies...${NC}"
  go mod verify

  echo ""
  echo -e "${GREEN}✓ Go dependencies updated successfully${NC}"
  echo ""
  echo "Updated dependencies:"
  go list -m all | grep -v "^github.com/akhilrex/podgrab"
  echo ""
}

#########################################################################
# Update GitHub Actions Workflows
#########################################################################
update_github_workflows() {
  echo -e "${BLUE}═══════════════════════════════════════════${NC}"
  echo -e "${BLUE}Updating GitHub Actions Workflows${NC}"
  echo -e "${BLUE}═══════════════════════════════════════════${NC}"
  echo ""

  # Check if workflows exist
  if [ ! -d ".github/workflows" ]; then
    echo -e "${YELLOW}No .github/workflows directory found, skipping${NC}"
    return 0
  fi

  # Find all workflow files
  workflow_files=$(find .github/workflows -name "*.yml" -o -name "*.yaml")

  if [ -z "$workflow_files" ]; then
    echo -e "${YELLOW}No workflow files found${NC}"
    return 0
  fi

  # Action version mappings (current -> latest as of 2024)
  # Using arrays compatible with Bash 3.2+
  action_updates=(
    # Format: "old_version|new_version"
    # Core GitHub Actions
    "actions/checkout@v2|actions/checkout@v4"
    "actions/checkout@v3|actions/checkout@v4"
    "actions/cache@v2|actions/cache@v4"
    "actions/cache@v3|actions/cache@v4"
    "actions/upload-artifact@v2|actions/upload-artifact@v4"
    "actions/upload-artifact@v3|actions/upload-artifact@v4"
    "actions/download-artifact@v2|actions/download-artifact@v4"
    "actions/download-artifact@v3|actions/download-artifact@v4"
    "actions/setup-go@v2|actions/setup-go@v5"
    "actions/setup-go@v3|actions/setup-go@v5"
    "actions/setup-go@v4|actions/setup-go@v5"
    "actions/setup-node@v2|actions/setup-node@v4"
    "actions/setup-node@v3|actions/setup-node@v4"
    "actions/setup-python@v2|actions/setup-python@v5"
    "actions/setup-python@v3|actions/setup-python@v5"
    "actions/setup-python@v4|actions/setup-python@v5"
    # Docker Actions
    "docker/setup-qemu-action@v1|docker/setup-qemu-action@v3"
    "docker/setup-qemu-action@v2|docker/setup-qemu-action@v3"
    "docker/setup-buildx-action@v1|docker/setup-buildx-action@v3"
    "docker/setup-buildx-action@v2|docker/setup-buildx-action@v3"
    "docker/login-action@v1|docker/login-action@v3"
    "docker/login-action@v2|docker/login-action@v3"
    "docker/build-push-action@v2|docker/build-push-action@v5"
    "docker/build-push-action@v3|docker/build-push-action@v5"
    "docker/build-push-action@v4|docker/build-push-action@v5"
    "docker/metadata-action@v3|docker/metadata-action@v5"
    "docker/metadata-action@v4|docker/metadata-action@v5"
  )

  for workflow_file in $workflow_files; do
    echo -e "${YELLOW}Processing: $workflow_file${NC}"

    # Create backup
    create_backup "$workflow_file"

    if [ "$DRY_RUN" = true ]; then
      echo -e "${YELLOW}[DRY RUN] Would update actions in $workflow_file${NC}"
      for mapping in "${action_updates[@]}"; do
        old_action="${mapping%%|*}"
        new_action="${mapping##*|}"
        if grep -q "$old_action" "$workflow_file"; then
          echo -e "  ${old_action} → ${new_action}"
        fi
      done
    else
      # Update each action
      updated=false
      for mapping in "${action_updates[@]}"; do
        old_action="${mapping%%|*}"
        new_action="${mapping##*|}"
        if grep -q "$old_action" "$workflow_file"; then
          sed -i.tmp "s|$old_action|$new_action|g" "$workflow_file"
          rm -f "${workflow_file}.tmp"
          echo -e "  ${GREEN}✓${NC} Updated: $old_action → $new_action"
          updated=true
        fi
      done

      if [ "$updated" = false ]; then
        echo -e "  ${GREEN}✓${NC} No updates needed"
      fi
    fi
    echo ""
  done

  echo -e "${GREEN}✓ GitHub Actions workflows processed${NC}"
  echo ""
}

#########################################################################
# Update Docker Files
#########################################################################
update_docker_files() {
  echo -e "${BLUE}═══════════════════════════════════════════${NC}"
  echo -e "${BLUE}Updating Docker Configuration${NC}"
  echo -e "${BLUE}═══════════════════════════════════════════${NC}"
  echo ""

  local docker_updated=false

  # Update Dockerfile
  if [ -f "Dockerfile" ]; then
    echo -e "${YELLOW}Updating Dockerfile...${NC}"
    create_backup "Dockerfile"

    current_go_version=$(grep "ARG GO_VERSION=" Dockerfile | cut -d'=' -f2)

    if [ -n "$current_go_version" ]; then
      echo -e "  Current Go version: ${YELLOW}$current_go_version${NC}"
      echo -e "  Suggested Go version: ${GREEN}$SUGGESTED_GO_VERSION${NC}"

      if [ "$DRY_RUN" = true ]; then
        echo -e "  ${YELLOW}[DRY RUN] Would update:${NC}"
        echo -e "    ARG GO_VERSION=$current_go_version → ARG GO_VERSION=$SUGGESTED_GO_VERSION"
      else
        sed -i.tmp "s/ARG GO_VERSION=.*/ARG GO_VERSION=$SUGGESTED_GO_VERSION/" Dockerfile
        rm -f Dockerfile.tmp
        echo -e "  ${GREEN}✓${NC} Updated Go version to $SUGGESTED_GO_VERSION"
        docker_updated=true
      fi
    fi

    # Check Alpine base image
    if grep -q "FROM alpine:latest" Dockerfile; then
      echo -e "  ${YELLOW}Warning: Using alpine:latest (consider pinning to specific version)${NC}"
      if [ "$DRY_RUN" = false ]; then
        echo -e "  ${YELLOW}Recommend: FROM alpine:3.19 (or latest stable)${NC}"
      fi
    fi
    echo ""
  fi

  # Update go.mod Go version
  if [ -f "go.mod" ]; then
    echo -e "${YELLOW}Updating go.mod Go directive...${NC}"
    create_backup "go.mod"

    current_mod_version=$(grep "^go " go.mod | awk '{print $2}')

    if [ -n "$current_mod_version" ]; then
      # Extract major.minor from suggested version (e.g., 1.21.6 -> 1.21)
      suggested_mod_version=$(echo "$SUGGESTED_GO_VERSION" | cut -d'.' -f1-2)

      echo -e "  Current go.mod version: ${YELLOW}$current_mod_version${NC}"
      echo -e "  Suggested version: ${GREEN}$suggested_mod_version${NC}"

      if [ "$DRY_RUN" = true ]; then
        echo -e "  ${YELLOW}[DRY RUN] Would update:${NC}"
        echo -e "    go $current_mod_version → go $suggested_mod_version"
      else
        sed -i.tmp "s/^go .*/go $suggested_mod_version/" go.mod
        rm -f go.mod.tmp
        echo -e "  ${GREEN}✓${NC} Updated go.mod to Go $suggested_mod_version"
        docker_updated=true
      fi
    fi
    echo ""
  fi

  # Update docker-compose.yml
  if [ -f "docker-compose.yml" ]; then
    echo -e "${YELLOW}Updating docker-compose.yml...${NC}"
    create_backup "docker-compose.yml"

    current_compose_version=$(grep "^version:" docker-compose.yml | awk '{print $2}' | tr -d '"')

    if [ -n "$current_compose_version" ]; then
      echo -e "  Current docker-compose version: ${YELLOW}$current_compose_version${NC}"

      # Version 2.x should be updated to 3.x
      if [[ "$current_compose_version" == 2* ]]; then
        suggested_compose_version="3.8"
        echo -e "  Suggested version: ${GREEN}$suggested_compose_version${NC}"

        if [ "$DRY_RUN" = true ]; then
          echo -e "  ${YELLOW}[DRY RUN] Would update:${NC}"
          echo -e "    version: \"$current_compose_version\" → version: \"$suggested_compose_version\""
        else
          sed -i.tmp "s/^version: .*/version: \"$suggested_compose_version\"/" docker-compose.yml
          rm -f docker-compose.yml.tmp
          echo -e "  ${GREEN}✓${NC} Updated docker-compose version to $suggested_compose_version"
          docker_updated=true
        fi
      else
        echo -e "  ${GREEN}✓${NC} docker-compose version is up to date"
      fi
    fi
    echo ""
  fi

  if [ "$docker_updated" = true ]; then
    echo -e "${GREEN}✓ Docker files updated successfully${NC}"
  else
    echo -e "${GREEN}✓ Docker files processed (no changes needed)${NC}"
  fi
  echo ""
}

#########################################################################
# Update Pre-commit Hooks
#########################################################################
update_precommit_hooks() {
  echo -e "${BLUE}═══════════════════════════════════════════${NC}"
  echo -e "${BLUE}Updating Pre-commit Hooks${NC}"
  echo -e "${BLUE}═══════════════════════════════════════════${NC}"
  echo ""

  if [ ! -f ".pre-commit-config.yaml" ]; then
    echo -e "${YELLOW}No .pre-commit-config.yaml found, skipping${NC}"
    return 0
  fi

  # Check if pre-commit is installed
  if ! command -v pre-commit &>/dev/null; then
    echo -e "${YELLOW}pre-commit not installed, skipping${NC}"
    echo -e "${YELLOW}Install with: pip install pre-commit${NC}"
    return 0
  fi

  echo -e "${YELLOW}Updating pre-commit hooks...${NC}"
  create_backup ".pre-commit-config.yaml"

  if [ "$DRY_RUN" = true ]; then
    echo -e "${YELLOW}[DRY RUN] Would update pre-commit hooks${NC}"
  else
    # Update hooks with frozen versions
    echo -e "Running: pre-commit autoupdate --freeze"
    if pre-commit autoupdate --freeze; then
      echo -e "${GREEN}✓ Pre-commit hooks updated${NC}"
    else
      echo -e "${YELLOW}Warning: Some hooks may not have updated${NC}"
    fi
  fi
  echo ""
}

#########################################################################
# Main Execution
#########################################################################
#########################################################################
# Main Execution
#########################################################################

# Update Go dependencies
if [ "$UPDATE_GO" = true ]; then
  update_go_dependencies
fi

# Update GitHub workflows
if [ "$UPDATE_WORKFLOWS" = true ]; then
  update_github_workflows
fi

# Update Docker files
if [ "$UPDATE_DOCKER" = true ]; then
  update_docker_files
fi

# Update pre-commit hooks
update_precommit_hooks

echo -e "${BLUE}═══════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ Update process complete!${NC}"
echo -e "${BLUE}═══════════════════════════════════════════${NC}"
echo ""

if [ "$DRY_RUN" = false ]; then
  echo -e "${YELLOW}Next steps:${NC}"
  echo "  1. Review the changes: git diff"
  echo "  2. Test the application: go build && ./app"
  echo "  3. Run in Docker: docker-compose up --build"
  echo "  4. Commit if everything works: git add . && git commit -m 'Update dependencies'"
  echo ""

  if [ "$CREATE_BACKUP" = true ]; then
    echo -e "${YELLOW}Note: Backup files created with .backup.* extension${NC}"
    echo "  Remove backups after testing: rm -f *.backup.*"
    echo ""
  fi
fi

exit 0
