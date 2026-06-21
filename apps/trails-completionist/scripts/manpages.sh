#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/trails-completionist/ man | gzip -c -9 >manpages/trails-completionist.1.gz
