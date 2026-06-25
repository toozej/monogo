#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/rss2socials/ man | gzip -c -9 >manpages/rss2socials.1.gz
