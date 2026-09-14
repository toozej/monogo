#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
task_tmp=$(mktemp -d)
test_container=""
podman_pid=""
cleanup() {
    if [[ -n "$podman_pid" ]]; then
        kill "$podman_pid" 2>/dev/null || true
        wait "$podman_pid" 2>/dev/null || true
    fi
    if [[ -n "$test_container" ]]; then
        docker rm -f "$test_container" >/dev/null
    fi
    rm -rf "$task_tmp"
}
trap cleanup EXIT
export GOCACHE="${GOCACHE:-$task_tmp/cache}"
export GOFLAGS="${GOFLAGS:--mod=mod}"
if [[ "${GOCICLE_TEST_BROWSER:-0}" == 1 ]]; then
    npm install --prefix "$task_tmp/browser" --ignore-scripts --no-audit --no-fund playwright@1.58.2
    export PLAYWRIGHT_BROWSERS_PATH="$task_tmp/browsers"
    export GOCICLE_PLAYWRIGHT_MODULE="$task_tmp/browser/node_modules/playwright"
    node "$GOCICLE_PLAYWRIGHT_MODULE/cli.js" install chromium
fi
test_container=$(docker run -d --rm -e POSTGRES_PASSWORD=gocicle-test -p 127.0.0.1::5432 postgres:17)
test_port=$(docker port "$test_container" 5432/tcp | cut -d: -f2)
export GOCICLE_TEST_DATABASE_URL="postgres://postgres:gocicle-test@127.0.0.1:$test_port/postgres?sslmode=disable"
for _ in {1..40}; do
    if docker exec "$test_container" pg_isready -U postgres >/dev/null 2>&1; then
        break
    fi
    sleep 1
done
export GOCICLE_TEST_RUNTIME_SOCKET=/var/run/docker.sock
export GOCICLE_TEST_RUNTIME=docker
make test APP=gocicle
if command -v podman >/dev/null; then
    export GOCICLE_TEST_RUNTIME_SOCKET="$task_tmp/podman.sock"
    export GOCICLE_TEST_RUNTIME=podman
    podman system service --time=0 "unix://$GOCICLE_TEST_RUNTIME_SOCKET" >"$task_tmp/podman.log" 2>&1 &
    podman_pid=$!
    for _ in {1..40}; do
        [[ -S "$GOCICLE_TEST_RUNTIME_SOCKET" ]] && break
        sleep 1
    done
    if ! go test -race -count=1 ./apps/gocicle/internal/runtime -run TestRuntimeIntegration; then
        cat "$task_tmp/podman.log" >&2
        exit 1
    fi
else
    echo 'Podman is unavailable. The live Podman test did not run.' >&2
fi
