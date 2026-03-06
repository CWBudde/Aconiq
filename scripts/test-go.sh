#!/usr/bin/env bash
set -euo pipefail

export GOCACHE="${GOCACHE:-/tmp/go-build-soundplan}"

(
	cd backend
	go test ./...
)
