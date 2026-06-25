#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/podgrab/ man | gzip -c -9 >manpages/podgrab.1.gz
