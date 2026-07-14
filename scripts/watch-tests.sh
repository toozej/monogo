#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "Watching for changes and running tests..."
while true; do
	"$ROOT/scripts/test-changed.sh" HEAD
	sleep 2
done
