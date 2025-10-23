#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/kmhd2spotify/ man | gzip -c -9 >manpages/kmhd2spotify.1.gz
