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
lint:
    cd backend && golangci-lint run --timeout=2m ./...

# Run linters with auto-fix
lint-fix:
    cd backend && golangci-lint run --fix --timeout=2m ./...

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
    cd backend && go build -o ../bin/noise ./cmd/noise

# Build the WebAssembly computation kernel (outputs to frontend/public/)
wasm-build:
    cd backend && GOOS=js GOARCH=wasm go build -o ../frontend/public/aconiq.wasm ./cmd/wasm
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" frontend/public/wasm_exec.js

# Run all checks (formatting, linting, tests, tidiness)
ci: check-formatted test lint check-tidy fe-ci

# Start dev environment: backend API server + frontend Vite dev server in parallel.
# Requires a project at the repo root (run `bin/noise init .` first if needed).
# Backend serves on :8080; frontend on :5173 (CORS is pre-configured for localhost).
dev: build
    #!/usr/bin/env bash
    set -euo pipefail
    trap 'kill $(jobs -p) 2>/dev/null; wait' EXIT INT TERM
    bin/noise serve --project . &
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
