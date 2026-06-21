#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/go-listen/ man | gzip -c -9 >manpages/go-listen.1.gz
