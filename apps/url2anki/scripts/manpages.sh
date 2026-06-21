#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/url2anki/ man | gzip -c -9 >manpages/url2anki.1.gz
