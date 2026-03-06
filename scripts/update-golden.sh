#!/usr/bin/env bash
set -euo pipefail

export GOCACHE="${GOCACHE:-/tmp/go-build-soundplan}"

(
	cd backend
	UPDATE_GOLDEN=1 go test ./...
)
