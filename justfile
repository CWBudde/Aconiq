set shell := ["bash", "-uc"]

# Default recipe - show available commands
default:
    @just --list

# Format all code using treefmt
fmt:
    treefmt --allow-missing-formatter

# Check if code is formatted correctly
check-formatted:
    treefmt --allow-missing-formatter --fail-on-change

# Run linters (from backend/)
# No --timeout flag: a CLI flag silently overrides `run.timeout` in
# .golangci.yml, and the two disagreed (2m here vs 5m there). The config is the
# single source of truth, so a slow cold run in CI fails on findings rather than
# on a timeout that only this file knew about.
lint:
    cd backend && golangci-lint run ./...

# Run linters with auto-fix
lint-fix:
    cd backend && golangci-lint run --fix ./...

# Run go vet (compiler-adjacent checks not covered by golangci-lint)
vet:
    cd backend && go vet ./...

# Ensure go.mod is tidy
check-tidy:
    cd backend && go mod tidy
    git diff --exit-code backend/go.mod backend/go.sum

# Run all tests
test:
    cd backend && go test ./...

# Run tests with race detector
test-race:
    cd backend && go test -race ./...

# Run tests with coverage
test-coverage:
    cd backend && go test -v -coverprofile=coverage.out ./...
    cd backend && go tool cover -html=coverage.out -o coverage.html

# Update golden test snapshots
update-golden:
    cd backend && UPDATE_GOLDEN=1 go test ./...

# Build the CLI
build:
    cd backend && go build -o ../bin/aconiq ./cmd/aconiq

# Build the WebAssembly computation kernel (outputs to frontend/public/)
wasm-build:
    cd backend && GOOS=js GOARCH=wasm go build -o ../frontend/public/aconiq.wasm ./cmd/wasm
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" frontend/public/wasm_exec.js

# Build the frontend for the WASM browser demo (no HTTP backend)
fe-build-wasm: wasm-build
    cd frontend && VITE_WASM_MODE=true bun run build

# Scan dependencies and the Go standard library for known vulnerabilities (CVEs)
govulncheck:
    cd backend && govulncheck ./...

# Check dependency licenses for policy violations (restricted, forbidden, unknown)
license-check:
    cd backend && go-licenses check ./... \
      --ignore github.com/aconiq/backend \
      --ignore modernc.org/mathutil \
      --disallowed_types=restricted,forbidden,unknown

# Generate a CSV report of all dependency licenses
license-report:
    cd backend && go-licenses report ./... \
      --ignore github.com/aconiq/backend \
      --ignore modernc.org/mathutil \
      2>/dev/null

# This recipe is the single source of truth for .github/workflows/go-ci.yml:
# every gate in that workflow invokes one of the recipes listed below. The
# workflow splits them over three parallel jobs (go-ci / go-race / govulncheck)
# to keep wall-clock time down; running them in sequence here is the local
# equivalent.
#
# The workflow additionally does two things that have no local counterpart and
# are therefore deliberately absent from this recipe:
#   - `just license-report` + artifact upload (a report, not a gate)
#   - `bin/aconiq openapi` + artifact upload (publishes the spec for consumers)

# Run the full Go CI gate (mirrors .github/workflows/go-ci.yml)
go-ci: check-formatted vet lint test test-race check-tidy govulncheck license-check build

# Note: `fe-bundle-check` (pulled in via fe-ci) is not run by
# .github/workflows/frontend-ci.yml, so this recipe is a superset of the
# frontend gate.

# Run all checks (Go + frontend)
ci: go-ci fe-ci

# Start dev environment: backend API server + frontend Vite dev server in parallel.
# Requires a project at the repo root (run `bin/aconiq init .` first if needed).
# Backend serves on :8080; frontend on :5173 (CORS is pre-configured for localhost).
dev: build
    #!/usr/bin/env bash
    set -euo pipefail
    trap 'kill $(jobs -p) 2>/dev/null; wait' EXIT INT TERM
    bin/aconiq serve --project . &
    cd frontend && bun run dev

# Start only the frontend dev server (no backend)
fe-dev-only:
    cd frontend && bun run dev

# --- Frontend recipes ---

# Install frontend dependencies
fe-install:
    cd frontend && bun install

# Start frontend dev server
fe-dev:
    cd frontend && bun run dev

# Build frontend for production
fe-build:
    cd frontend && bun run build

# Run frontend type checking
fe-typecheck:
    cd frontend && bun run typecheck

# Run frontend linter
fe-lint:
    cd frontend && bun run lint

# Fix frontend lint issues
fe-lint-fix:
    cd frontend && bun run lint:fix

# Run frontend tests
fe-test:
    cd frontend && bun run test

# Check JS bundle size budgets (requires a prior fe-build)
fe-bundle-check:
    node frontend/scripts/check-bundle-size.mjs

# Run E2E tests with Playwright (starts Vite dev server automatically)
fe-e2e:
    npx playwright test

# Run all frontend checks (typecheck, lint, test, build, bundle-check)
fe-ci: fe-typecheck fe-lint fe-test fe-build fe-bundle-check

# --- General ---

# Clean build artifacts
clean:
    rm -rf bin/ backend/coverage.out backend/coverage.html frontend/dist frontend/public/aconiq.wasm frontend/public/wasm_exec.js

fix:
    just lint-fix
    just fmt
