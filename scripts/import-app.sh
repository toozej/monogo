#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="${1:-}"

DEFAULT_VCS_HOST="github.com/"
IMPORT_REF="${IMPORT_REF:-}"
IMPORT_NAME="${IMPORT_NAME:-}"
IMPORT_RELEASES="${IMPORT_RELEASES:-metadata}"
IMPORT_SKIP_VERIFY="${IMPORT_SKIP_VERIFY:-0}"
IMPORT_TAG_PREFIX="${IMPORT_TAG_PREFIX:-}"

die() {
	echo "error: $*" >&2
	exit 1
}

require_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

yaml_escape() {
	printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

normalize_vcs_host() {
	local host="$1"

	host="${host#http://}"
	host="${host#https://}"
	host="${host%/}"
	printf '%s/' "$host"
}

move_path() {
	local src="$1"
	local dst="$2"

	mkdir -p "$(dirname "$dst")"
	if git -C "$ROOT" ls-files --error-unmatch "${src#"$ROOT"/}" >/dev/null 2>&1; then
		git -C "$ROOT" mv "$src" "$dst"
	else
		mv "$src" "$dst"
	fi
}

rewrite_imports() {
	local old="$1"
	local new="$2"
	local scope="${3:-$ROOT}"
	local tmp

	tmp="$(mktemp)"
	if rg -F -l --hidden -g '!**/.monogo/**' -g '!vendor/**' "$old" "$scope" >"$tmp" 2>/dev/null; then
		xargs perl -pi -e "s#\\Q${old}\\E(?![\\w.-])#${new}#g" <"$tmp"
	fi
	rm -f "$tmp"
}

relocate_imported_pkg() {
	local app_pkg="${APP_DIR}/pkg"
	local source_import_prefix="${ROOT_MODULE}/apps/${APP_NAME}/pkg"
	local relocation_log="${IMPORT_METADATA_DIR}/pkg-relocations.tsv"

	: >"$relocation_log"

	if [ ! -d "$app_pkg" ]; then
		return 0
	fi

	mkdir -p "${ROOT}/pkg" "${APP_METADATA_DIR}/original-pkg"

	shopt -s nullglob dotglob
	for entry in "$app_pkg"/*; do
		local base
		local target
		local old_import
		local new_import

		base="$(basename "$entry")"

		if [ -d "$entry" ]; then
			old_import="${source_import_prefix}/${base}"
			if [ -e "${ROOT}/pkg/${base}" ]; then
				target="${ROOT}/pkg/${APP_NAME}/${base}"
				new_import="${ROOT_MODULE}/pkg/${APP_NAME}/${base}"
				echo "root pkg/${base} already exists; moving imported pkg/${base} to pkg/${APP_NAME}/${base}"
			else
				target="${ROOT}/pkg/${base}"
				new_import="${ROOT_MODULE}/pkg/${base}"
			fi

			move_path "$entry" "$target"
			rewrite_imports "$old_import" "$new_import" "$ROOT"
			printf '%s\t%s\n' "$old_import" "$new_import" >>"$relocation_log"

			continue
		fi

		if [ -e "${ROOT}/pkg/${base}" ]; then
			target="${APP_METADATA_DIR}/original-pkg/${base}"
		else
			target="${ROOT}/pkg/${base}"
		fi
		move_path "$entry" "$target"
		printf '%s\t%s\n' "${app_pkg#"$ROOT"/}/${base}" "${target#"$ROOT"/}" >>"$relocation_log"
	done
	shopt -u nullglob dotglob

	rmdir "$app_pkg" 2>/dev/null || true
}

add_app_to_matrix() {
	local file="$1"
	local tmp

	tmp="$(mktemp)"
	awk -v app="$APP_NAME" '
		BEGIN {
			in_app = 0
			found = 0
			indent = ""
		}
		{
			if (in_app && $0 !~ /^[[:space:]]+- /) {
				if (!found) {
					print indent "- " app
				}
				in_app = 0
				found = 0
			}
			print
			if ($0 ~ /^[[:space:]]+app:[[:space:]]*$/) {
				in_app = 1
				found = 0
				indent = substr($0, 1, index($0, "a") - 1) "  "
				next
			}
			if (in_app && $0 ~ "^[[:space:]]+- " app "$") {
				found = 1
			}
		}
		END {
			if (in_app && !found) {
				print indent "- " app
			}
		}
	' "$file" >"$tmp"
	mv "$tmp" "$file"
}

if [ -z "$SOURCE" ] || [ "$SOURCE" = "monogo" ]; then
	die "usage: make import APP=[vcs-host/]owner/repo [IMPORT_REF=main] [IMPORT_NAME=app-name]"
fi

require_cmd git
require_cmd go
require_cmd jq
require_cmd gomplate
require_cmd rg

if [ "$IMPORT_RELEASES" != "none" ]; then
	require_cmd gh
fi

if [ "$IMPORT_SKIP_VERIFY" != "1" ]; then
	require_cmd goreleaser
fi

case "$SOURCE" in
	http://*/*)
		SOURCE_WITHOUT_SCHEME="${SOURCE#http://}"
		VCS_HOST="$(normalize_vcs_host "${SOURCE_WITHOUT_SCHEME%%/*}")"
		OWNER_REPO="${SOURCE_WITHOUT_SCHEME#*/}"
		OWNER_REPO="${OWNER_REPO%.git}"
		REPO_URL="http://${VCS_HOST}${OWNER_REPO}.git"
		;;
	https://*/*)
		SOURCE_WITHOUT_SCHEME="${SOURCE#https://}"
		VCS_HOST="$(normalize_vcs_host "${SOURCE_WITHOUT_SCHEME%%/*}")"
		OWNER_REPO="${SOURCE_WITHOUT_SCHEME#*/}"
		OWNER_REPO="${OWNER_REPO%.git}"
		REPO_URL="https://${VCS_HOST}${OWNER_REPO}.git"
		;;
	git@*:*)
		SOURCE_WITHOUT_USER="${SOURCE#git@}"
		VCS_HOST="$(normalize_vcs_host "${SOURCE_WITHOUT_USER%%:*}")"
		OWNER_REPO="${SOURCE_WITHOUT_USER#*:}"
		OWNER_REPO="${OWNER_REPO%.git}"
		REPO_URL="$SOURCE"
		;;
	*/*/*)
		VCS_HOST="$(normalize_vcs_host "${SOURCE%%/*}")"
		OWNER_REPO="${SOURCE#*/}"
		OWNER_REPO="${OWNER_REPO%.git}"
		REPO_URL="https://${VCS_HOST}${OWNER_REPO}.git"
		;;
	*/*)
		VCS_HOST="$DEFAULT_VCS_HOST"
		OWNER_REPO="${SOURCE%.git}"
		REPO_URL="https://${VCS_HOST}${OWNER_REPO}.git"
		;;
	*)
		die "APP must be [vcs-host/]owner/repo or a VCS URL, got: $SOURCE"
		;;
esac

VCS_HOST="$(normalize_vcs_host "$VCS_HOST")"
GH_REPO="$OWNER_REPO"
if [ "$VCS_HOST" != "$DEFAULT_VCS_HOST" ]; then
	GH_REPO="${VCS_HOST}${OWNER_REPO}"
fi

OWNER="${OWNER_REPO%%/*}"
REPO="${OWNER_REPO#*/}"
APP_NAME="${IMPORT_NAME:-$REPO}"
APP_NAME="${APP_NAME%.git}"
APP_DIR="${ROOT}/apps/${APP_NAME}"
ROOT_MODULE="$(awk '/^module / {print $2; exit}' "${ROOT}/go.mod")"

case "$APP_NAME" in
	*[!A-Za-z0-9._-]*)
		die "derived app name '$APP_NAME' is not safe for apps/<name>; set IMPORT_NAME"
		;;
esac

if [ -e "$APP_DIR" ]; then
	die "$APP_DIR already exists"
fi

cd "$ROOT"

git rev-parse --verify HEAD >/dev/null 2>&1 || die "commit the monogo scaffold before importing; git subtree requires an existing HEAD"

if [ -n "$(git status --porcelain)" ]; then
	die "working tree must be clean before import"
fi

EXISTING_TAGS="$(mktemp)"
trap 'rm -f "$EXISTING_TAGS" "${SOURCE_TAGS:-}"' EXIT
git tag -l >"$EXISTING_TAGS"

DEFAULT_REF="$(git ls-remote --symref "$REPO_URL" HEAD | awk '/^ref:/ {ref=$2; sub("^refs/heads/", "", ref); print ref; exit}')"
REF="${IMPORT_REF:-$DEFAULT_REF}"

if [ -z "$REF" ]; then
	die "could not resolve default branch for $REPO_URL; set IMPORT_REF"
fi

echo "Importing ${VCS_HOST}${OWNER_REPO}@${REF} into apps/${APP_NAME}"
git subtree add \
	--prefix "apps/${APP_NAME}" \
	"$REPO_URL" \
	"$REF" \
	-m "chore(import): import ${VCS_HOST}${OWNER_REPO} into apps/${APP_NAME}"

if [ ! -f "${APP_DIR}/go.mod" ]; then
	die "imported repository does not contain a root go.mod"
fi

OLD_MODULE="$(awk '/^module / {print $2; exit}' "${APP_DIR}/go.mod")"
APP_METADATA_DIR="${APP_DIR}/.monogo"
ORIGINAL_MODULE_DIR="${APP_METADATA_DIR}/original-module"
ORIGINAL_CONFIG_DIR="${APP_METADATA_DIR}/original-config"
IMPORT_METADATA_DIR="${APP_METADATA_DIR}/import"

mkdir -p "$APP_METADATA_DIR" "$IMPORT_METADATA_DIR"

go mod edit -json "${APP_DIR}/go.mod" >"${IMPORT_METADATA_DIR}/go-mod.json"

jq -r '.Require[]? | "\(.Path)@\(.Version)"' "${IMPORT_METADATA_DIR}/go-mod.json" | while read -r requirement; do
	[ -n "$requirement" ] || continue
	req_path="${requirement%@*}"
	if [ "$req_path" = "$OLD_MODULE" ] || [ "$req_path" = "$ROOT_MODULE" ]; then
		continue
	fi
	go mod edit -require="$requirement"
done

jq -c '.Replace[]?' "${IMPORT_METADATA_DIR}/go-mod.json" | while read -r replacement; do
	old_path="$(jq -r '.Old.Path' <<<"$replacement")"
	old_version="$(jq -r '.Old.Version // ""' <<<"$replacement")"
	new_path="$(jq -r '.New.Path' <<<"$replacement")"
	new_version="$(jq -r '.New.Version // ""' <<<"$replacement")"

	old="$old_path"
	if [ -n "$old_version" ]; then
		old="${old}@${old_version}"
	fi

	if [ -n "$new_version" ]; then
		new="${new_path}@${new_version}"
	elif [[ "$new_path" = ./* || "$new_path" = ../* ]]; then
		new="./apps/${APP_NAME}/${new_path}"
	else
		new="$new_path"
	fi

	go mod edit -replace="${old}=${new}"
done

mkdir -p "$ORIGINAL_MODULE_DIR"
move_path "${APP_DIR}/go.mod" "${ORIGINAL_MODULE_DIR}/go.mod"
if [ -f "${APP_DIR}/go.sum" ]; then
	move_path "${APP_DIR}/go.sum" "${ORIGINAL_MODULE_DIR}/go.sum"
fi

if [ -n "$OLD_MODULE" ] && [ "$OLD_MODULE" != "$ROOT_MODULE/apps/$APP_NAME" ]; then
	rewrite_imports "$OLD_MODULE" "${ROOT_MODULE}/apps/${APP_NAME}" "$APP_DIR"
fi

relocate_imported_pkg

shopt -s nullglob
for original in \
	"${APP_DIR}/.air.toml" \
	"${APP_DIR}/.goreleaser.yml" \
	"${APP_DIR}/.goreleaser.yaml" \
	"${APP_DIR}/.pre-commit-config.yaml" \
	"${APP_DIR}/docker-compose.yml" \
	"${APP_DIR}/Makefile" \
	"${APP_DIR}"/Dockerfile \
	"${APP_DIR}"/Dockerfile.*; do
	[ -e "$original" ] || continue
	relative="${original#"$APP_DIR"/}"
	move_path "$original" "${ORIGINAL_CONFIG_DIR}/${relative}"
done
shopt -u nullglob

go mod tidy

MAIN_PACKAGES="$(go list -f '{{if eq .Name "main"}}{{.ImportPath}}{{end}}' "./apps/${APP_NAME}/..." | sed '/^$/d')"
MAIN_COUNT="$(printf '%s\n' "$MAIN_PACKAGES" | sed '/^$/d' | wc -l | tr -d ' ')"

if [ "$MAIN_COUNT" = "0" ]; then
	die "could not find a main package under apps/${APP_NAME}"
fi

if [ "$MAIN_COUNT" = "1" ]; then
	MAIN_IMPORT="$MAIN_PACKAGES"
else
	MAIN_IMPORT="$(printf '%s\n' "$MAIN_PACKAGES" | grep -E "/cmd/${APP_NAME}$|/${APP_NAME}$" | head -n 1 || true)"
	if [ -z "$MAIN_IMPORT" ]; then
		MAIN_IMPORT="$(printf '%s\n' "$MAIN_PACKAGES" | head -n 1)"
	fi
	echo "Multiple main packages found; selected ${MAIN_IMPORT}. Override apps/${APP_NAME}/app.yaml if needed."
fi

MAIN_PATH="${MAIN_IMPORT#"${ROOT_MODULE}"/}"

DESCRIPTION="${APP_NAME} imported from ${REPO_URL}"
if command -v gh >/dev/null 2>&1; then
	GH_DESCRIPTION="$(gh repo view "$GH_REPO" --json description --jq '.description // ""' 2>/dev/null || true)"
	if [ -n "$GH_DESCRIPTION" ]; then
		DESCRIPTION="$GH_DESCRIPTION"
	fi
fi

cat >"${APP_DIR}/app.yaml" <<EOF
---
name: ${APP_NAME}
binary: ${APP_NAME}
path: apps/${APP_NAME}
mainPath: ${MAIN_PATH}
description: "$(yaml_escape "$DESCRIPTION")"
shortDescription: "${APP_NAME}"
goImage: golang:1.26-trixie
distrolessImage: gcr.io/distroless/static-debian13:nonroot
# cgoEnabled defaults to false. When true, builds use CGO_ENABLED=1, the runtime
# image becomes glibc-capable (debian:trixie-slim / distroless base-debian), and
# GoReleaser is trimmed to the native linux/amd64 platform.
cgoEnabled: false
# runtimeImage (optional) overrides the runtime base for the non-distroless
# Dockerfile. Defaults to "scratch", or "debian:trixie-slim" when cgoEnabled is true.
# runtimeImage: debian:trixie-slim
# port (optional) is EXPOSEd in the generated Dockerfiles when set.
# port: 8081
EOF

cat >"${IMPORT_METADATA_DIR}/source.json" <<EOF
{
  "source": "$(yaml_escape "$SOURCE")",
  "vcsHost": "$(yaml_escape "$VCS_HOST")",
  "owner": "$(yaml_escape "$OWNER")",
  "repo": "$(yaml_escape "$REPO")",
  "ownerRepo": "$(yaml_escape "$OWNER_REPO")",
  "repoURL": "$(yaml_escape "$REPO_URL")",
  "ref": "$(yaml_escape "$REF")",
  "oldModule": "$(yaml_escape "$OLD_MODULE")",
  "newModule": "$(yaml_escape "${ROOT_MODULE}/apps/${APP_NAME}")",
  "tagPrefix": "$(yaml_escape "${IMPORT_TAG_PREFIX:-apps/${APP_NAME}}")"
}
EOF

TAG_PREFIX="${IMPORT_TAG_PREFIX:-apps/${APP_NAME}}"
SOURCE_TAGS="$(mktemp)"
git ls-remote --tags --refs "$REPO_URL" >"$SOURCE_TAGS"
if grep -q . "$SOURCE_TAGS"; then
	echo "Importing tags under refs/tags/${TAG_PREFIX}/"
	git fetch --no-tags "$REPO_URL" "+refs/tags/*:refs/tags/${TAG_PREFIX}/*"
	awk '{sub("^refs/tags/", "", $2); print $2}' "$SOURCE_TAGS" | while read -r tag; do
		[ -n "$tag" ] || continue
		if grep -Fxq "$tag" "$EXISTING_TAGS"; then
			continue
		fi
		if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null &&
			git rev-parse -q --verify "refs/tags/${TAG_PREFIX}/${tag}" >/dev/null &&
			[ "$(git rev-parse "refs/tags/${tag}")" = "$(git rev-parse "refs/tags/${TAG_PREFIX}/${tag}")" ]; then
			git tag -d "$tag" >/dev/null
		fi
	done
else
	echo "No source tags found for ${OWNER_REPO}"
fi
rm -f "$SOURCE_TAGS"

case "$IMPORT_RELEASES" in
	none)
		echo "Skipping GitHub release metadata import"
		;;
	metadata|assets)
		RELEASE_DIR="${APP_METADATA_DIR}/releases"
		mkdir -p "$RELEASE_DIR"
		gh release list \
			--repo "$GH_REPO" \
			--limit 1000 \
			--json tagName,name,isDraft,isPrerelease,isLatest,publishedAt,createdAt \
			>"${RELEASE_DIR}/releases.json"
		jq -r '.[].tagName' "${RELEASE_DIR}/releases.json" | while read -r tag; do
			[ -n "$tag" ] || continue
			safe_tag="${tag//\//__}"
			mkdir -p "${RELEASE_DIR}/${safe_tag}"
			gh release view "$tag" \
				--repo "$GH_REPO" \
				--json tagName,name,body,isDraft,isPrerelease,createdAt,publishedAt,url,assets \
				>"${RELEASE_DIR}/${safe_tag}/release.json"
			if [ "$IMPORT_RELEASES" = "assets" ]; then
				mkdir -p "${RELEASE_DIR}/${safe_tag}/assets"
				gh release download "$tag" \
					--repo "$GH_REPO" \
					--dir "${RELEASE_DIR}/${safe_tag}/assets" \
					--clobber
			fi
		done
		;;
	*)
		die "IMPORT_RELEASES must be one of: none, metadata, assets"
		;;
esac

# weekly-docker-refresh.yaml derives its matrix from released apps/<app>/vX.Y.Z
# tags at run time, so only ci.yaml carries a static app matrix to update here.
add_app_to_matrix "${ROOT}/.github/workflows/ci.yaml"

if ! grep -q "directory: \"/apps/${APP_NAME}\"" "${ROOT}/.github/dependabot.yml"; then
	cat >>"${ROOT}/.github/dependabot.yml" <<EOF
  - package-ecosystem: "docker"
    directory: "/apps/${APP_NAME}"
    schedule:
      interval: "weekly"
    cooldown:
      default-days: 7
    labels:
      - "dependencies"
    commit-message:
      prefix: "chore"
      include: "scope"
    patterns:
      - "*"
    multi-ecosystem-group: "combined"
EOF
fi

make app-generate APP="$APP_NAME"
go mod tidy

if [ "$IMPORT_SKIP_VERIFY" != "1" ]; then
	make test APP="$APP_NAME"
	make local-build APP="$APP_NAME"
	make local-release-test APP="$APP_NAME"
else
	echo "Skipping verification because IMPORT_SKIP_VERIFY=1"
fi

cat <<EOF

Imported ${VCS_HOST}${OWNER_REPO} into apps/${APP_NAME}.

Preserved:
- Git history via non-squashed git subtree import
- Source tags under refs/tags/${TAG_PREFIX}/
EOF

if [ "$IMPORT_RELEASES" = "none" ]; then
	echo "- GitHub release metadata import skipped"
else
	echo "- GitHub release metadata under apps/${APP_NAME}/.monogo/releases"
fi

cat <<EOF

Review app metadata in apps/${APP_NAME}/app.yaml, then commit the import changes.
EOF
