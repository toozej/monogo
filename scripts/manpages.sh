#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/gotts-it/ man | gzip -c -9 >manpages/gotts-it.1.gz
