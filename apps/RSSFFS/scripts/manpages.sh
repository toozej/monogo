#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/RSSFFS/ man | gzip -c -9 >manpages/RSSFFS.1.gz
