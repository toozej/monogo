#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/kmhd2playlist/ man | gzip -c -9 >manpages/kmhd2playlist.1.gz
