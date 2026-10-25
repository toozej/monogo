#!/usr/bin/env bash
set -euo pipefail

demo_app_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
demo_repo_root="$(cd -- "${demo_app_dir}/../.." && pwd -P)"
demo_output_dir="${demo_repo_root}/out/trails-completionist-demo"
demo_tracks_dir="${demo_app_dir}/tracks"
demo_serve=true
demo_refresh=false
demo_listen_address="${SERVE_ADDRESS:-0.0.0.0:3000}"

usage() {
    cat <<'HELP'
Usage: apps/trails-completionist/demo.sh [options]

Build the app, download Portland data, process both example inputs, and serve the page.
The script processes GPX and TCX files recursively in apps/trails-completionist/tracks.
The demo listens on all IPv4 interfaces at port 3000 by default.
Open http://127.0.0.1:3000 locally or http://<this-computer-LAN-IP>:3000 on your LAN.
Press Ctrl+C to stop the server.

Options:
  --tracks-dir DIR  Read activity files from DIR instead of the app's tracks directory.
  --output-dir DIR  Store data and results in DIR instead of out/trails-completionist-demo.
  --listen-address ADDR
                    Set the listen address (host:port). Overrides SERVE_ADDRESS.
  --refresh         Download Portland OSM data and the city boundary again.
  --no-serve        Generate the demo files without starting the web server.
  -h, --help        Show this help.

Downloads remain cached between runs. The first OSM download is approximately 153 MiB.
Required tools: Go, make, curl, gzip, and Python 3.
The Make demo target supplies BIN, so the script uses its freshly built binary.
HELP
}

fail() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

while (($#)); do
    case "$1" in
        --tracks-dir|--output-dir|--listen-address)
            if (($# < 2)) || [[ -z "${2-}" || "${2-}" == --* ]]; then
                fail "$1 requires a value."
            fi
            case "$1" in
                --tracks-dir) demo_tracks_dir="$2" ;;
                --output-dir) demo_output_dir="$2" ;;
                --listen-address) demo_listen_address="$2" ;;
            esac
            shift 2
            ;;
        --refresh) demo_refresh=true; shift ;;
        --no-serve) demo_serve=false; shift ;;
        -h|--help) usage; exit 0 ;;
        *) fail "Unknown option: $1. Use --help to list options." ;;
    esac
done

for demo_tool in curl gzip python3; do
    command -v "${demo_tool}" >/dev/null 2>&1 || fail "Required tool is missing: ${demo_tool}."
done

if [[ -n "${BIN:-}" ]]; then
    [[ -x "${BIN}" ]] || fail "The supplied BIN is not executable: ${BIN}."
    demo_binary="$(cd -- "$(dirname -- "${BIN}")" && pwd -P)/$(basename -- "${BIN}")"
    printf 'Using supplied binary: %s\n' "${demo_binary}"
else
    for demo_tool in go make; do
        command -v "${demo_tool}" >/dev/null 2>&1 || fail "Required tool is missing: ${demo_tool}."
    done
    printf 'Building trails-completionist...\n'
    make -C "${demo_repo_root}" local-build APP=trails-completionist
    demo_binary="${demo_repo_root}/out/trails-completionist"
fi

mkdir -p -- "${demo_output_dir}/data" "${demo_tracks_dir}"
demo_output_dir="$(cd -- "${demo_output_dir}" && pwd -P)"
demo_tracks_dir="$(cd -- "${demo_tracks_dir}" && pwd -P)"
demo_data_dir="${demo_output_dir}/data"
demo_osm_file="${demo_data_dir}/Portland.osm.gz"
demo_boundary_file="${demo_data_dir}/portland-boundary.geojson"
demo_scratch_dir="$(mktemp -d "${demo_data_dir}/.demo-download.XXXXXX")"
trap 'rm -rf -- "${demo_scratch_dir}"' EXIT

demo_curl=(curl --fail --location --show-error --retry 3 --connect-timeout 20 --max-time 900)
if [[ "${demo_refresh}" == true || ! -s "${demo_osm_file}" ]]; then
    printf 'Downloading the Portland OSM extract from BBBike...\n'
    "${demo_curl[@]}" \
        --output "${demo_scratch_dir}/Portland.osm.gz" \
        'https://download.bbbike.org/osm/bbbike/Portland/Portland.osm.gz'
    gzip -t "${demo_scratch_dir}/Portland.osm.gz" || fail 'The OSM download is not a complete gzip archive.'
    mv -- "${demo_scratch_dir}/Portland.osm.gz" "${demo_osm_file}"
else
    printf 'Using cached OSM data: %s\n' "${demo_osm_file}"
fi

if [[ "${demo_refresh}" == true || ! -s "${demo_boundary_file}" ]]; then
    printf 'Downloading the official Portland city boundary...\n'
    "${demo_curl[@]}" --get \
        --data-urlencode "where=CITYNAME='Portland'" \
        --data-urlencode 'outFields=CITYNAME' \
        --data-urlencode 'outSR=4326' \
        --data-urlencode 'returnGeometry=true' \
        --data-urlencode 'f=geojson' \
        --output "${demo_scratch_dir}/portland-boundary.geojson" \
        'https://www.portlandmaps.com/arcgis/rest/services/Public/Boundaries/MapServer/13/query'
    python3 - "${demo_scratch_dir}/portland-boundary.geojson" <<'PY'
import json
import sys

try:
    with open(sys.argv[1], encoding="utf-8") as boundary_file:
        boundary = json.load(boundary_file)
except (OSError, ValueError) as error:
    raise SystemExit(f"Cannot read the Portland city boundary: {error}") from None
if not isinstance(boundary, dict):
    raise SystemExit("The Portland boundary download is not a GeoJSON object.")
features = boundary.get("features")
if (
    boundary.get("type") != "FeatureCollection"
    or not isinstance(features, list)
    or not features
    or boundary.get("exceededTransferLimit")
    or boundary.get("properties", {}).get("exceededTransferLimit")
    or any((feature.get("geometry") or {}).get("type") not in ("Polygon", "MultiPolygon") for feature in features)
):
    raise SystemExit("The Portland boundary download is not a complete polygon FeatureCollection.")
PY
    mv -- "${demo_scratch_dir}/portland-boundary.geojson" "${demo_boundary_file}"
else
    printf 'Using cached city boundary: %s\n' "${demo_boundary_file}"
fi

# Keep an empty directory valid. The app requires activity files when trackFiles is set.
find "${demo_tracks_dir}" ! -type d \( -iname '*.gpx' -o -iname '*.tcx' \) -print >"${demo_scratch_dir}/activities"
demo_track_files=''
if [[ -s "${demo_scratch_dir}/activities" ]]; then
    demo_track_files="${demo_tracks_dir}"
    printf 'Processing activity files recursively in: %s\n' "${demo_tracks_dir}"
else
    printf 'No GPX or TCX files found in %s. Running the inventory demo.\n' "${demo_tracks_dir}"
fi

printf '\nDemo output: %s\n' "${demo_output_dir}"
printf 'Checklist:   %s/checklist.md\n' "${demo_output_dir}"
printf 'HTML page:   %s/trails.html\n' "${demo_output_dir}"
printf 'OSM audit:   %s/osm-audit.json\n' "${demo_output_dir}"
if [[ "${demo_serve}" == true ]]; then
    printf 'The server will listen on %s after processing.\n' "${demo_listen_address}"
    case "${demo_listen_address}" in
        0.0.0.0:*|:*)
            printf 'Local page: http://127.0.0.1:%s/\n' "${demo_listen_address##*:}"
            printf 'LAN page:   http://<this-computer-LAN-IP>:%s/\n' "${demo_listen_address##*:}"
            ;;
        \[::\]:*)
            printf 'Local page: http://[::1]:%s/\n' "${demo_listen_address##*:}"
            printf 'LAN page:   use this computer\047s LAN IP address and port %s.\n' "${demo_listen_address##*:}"
            ;;
        *) printf 'Page: http://%s/\n' "${demo_listen_address}" ;;
    esac
    printf 'Press Ctrl+C to stop the demo.\n'
fi
printf 'Processing OSM data and both example inputs. The first run can take a minute.\n'

# Remove temporary downloads before the server replaces this shell.
rm -rf -- "${demo_scratch_dir}"
trap - EXIT
exec "${demo_binary}" full \
    --osmRegionFile "${demo_osm_file}" \
    --boundaryFile "${demo_boundary_file}" \
    --inputFile "${demo_app_dir}/trails_input_file_example.txt" \
    --inventoryFile "${demo_app_dir}/trails_portland_inventory_example.geojson" \
    --trackFiles "${demo_track_files}" \
    --checklistFile "${demo_output_dir}/checklist.md" \
    --htmlFile "${demo_output_dir}/trails.html" \
    --reportFile "${demo_output_dir}/osm-audit.json" \
    --matchDistance 25 \
    --minimumCoverage 0.9 \
    --serveAddress "${demo_listen_address}" \
    --serve="${demo_serve}"
