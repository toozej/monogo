#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/lego-stego/ man | gzip -c -9 >manpages/lego-stego.1.gz
