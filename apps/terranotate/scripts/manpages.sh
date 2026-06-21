#!/bin/sh
set -e
rm -rf manpages
mkdir manpages
go run ./cmd/terranotate/ man | gzip -c -9 >manpages/terranotate.1.gz
