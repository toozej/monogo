#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/ghreleases2rss/ man | gzip -c -9 >manpages/ghreleases2rss.1.gz
