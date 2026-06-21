#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/go-find-liquor/ man | gzip -c -9 >manpages/go-find-liquor.1.gz
