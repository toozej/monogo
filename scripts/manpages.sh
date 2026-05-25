#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run main.go man | gzip -c -9 >manpages/go-sort-out-gh-actions.1.gz
