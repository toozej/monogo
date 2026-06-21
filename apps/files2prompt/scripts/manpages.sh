#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/files2prompt/ man | gzip -c -9 >manpages/files2prompt.1.gz
