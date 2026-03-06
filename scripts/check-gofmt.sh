#!/usr/bin/env bash
set -euo pipefail

mapfile -t files < <(find backend -type f -name '*.go' | sort)
if [ "${#files[@]}" -eq 0 ]; then
	echo "No Go files found."
	exit 0
fi

unformatted="$(gofmt -l "${files[@]}")"
if [ -n "$unformatted" ]; then
	echo "The following files are not gofmt-formatted:" >&2
	echo "$unformatted" >&2
	echo "Run: gofmt -w <files>" >&2
	exit 1
fi

echo "gofmt check passed."
