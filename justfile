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

# Release identity stamped into the binary by `build`. Kept in one place so the
# local build and the CI build (which calls this same recipe) agree.
#
# `git describe` needs tags; until the repo has any it falls back to the short
# commit, and a plain `go build ./...` falls back further to the module build
# info embedded by the toolchain (see internal/buildinfo).
ldflags_pkg := "github.com/aconiq/backend/internal/buildinfo"

# Build the CLI
build:
    #!/usr/bin/env bash
    set -euo pipefail
    version="$(git describe --tags --dirty --always 2>/dev/null || echo dev)"
    commit="$(git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)"
    date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    cd backend && go build \
      -ldflags "-X {{ ldflags_pkg }}.version=${version} -X {{ ldflags_pkg }}.commit=${commit} -X {{ ldflags_pkg }}.date=${date}" \
      -o ../bin/aconiq ./cmd/aconiq

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
    #!/usr/bin/env bash
    set -euo pipefail
    cd backend
    # GOROOT: see the comment on `license-report`.
    export GOROOT="$(go env GOROOT)"
    go-licenses check ./... \
      --ignore github.com/aconiq/backend \
      --ignore modernc.org/mathutil \
      --disallowed_types=restricted,forbidden,unknown

# Generate a CSV report of all dependency licenses.
#
# The report is swept over the three target platforms because `go-licenses`
# only sees packages that survive build-tag selection for the current GOOS:
# github.com/inconshreveable/mousetrap is Windows-only, and modernc.org/sqlite
# pulls go-isatty and go-strftime in on non-Linux targets. A Linux-only report
# silently omits them, which is how they went missing from NOTICE.
#
# GOROOT is exported because go-licenses classifies a package as standard
# library by comparing its source path against the GOROOT compiled into the
# binary; when the active toolchain lives in the module cache (a `toolchain`
# directive in go.mod), that comparison fails and every std package is reported
# as an error instead.
license-report:
    #!/usr/bin/env bash
    set -euo pipefail
    cd backend
    # Evaluate GOROOT from inside the module: the `toolchain` directive in
    # go.mod can switch the active toolchain, and only then does `go env`
    # report the GOROOT that go-licenses has to match.
    export GOROOT="$(go env GOROOT)"
    for goos in linux windows darwin; do
      GOOS="${goos}" GOARCH=amd64 go-licenses report ./... \
        --ignore github.com/aconiq/backend \
        --ignore modernc.org/mathutil \
        2>/dev/null
    done | sort -u

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
    cd frontend && bun run test:e2e

# Run all frontend checks (typecheck, lint, test, build, bundle-check)
fe-ci: fe-typecheck fe-lint fe-test fe-build fe-bundle-check

# Refuse tracked interoperability/ paths
#
# interoperability/ holds licensed third-party reference material -- standards
# documents and vendor project bundles -- that must never leave the machines
# entitled to it. Today the only thing keeping it out of the repository is one
# line in .gitignore, which does not cover a file added with `git add -f`, one
# that was already tracked when the rule landed, or a branch that predates it.
# Mirrored by .github/workflows/repo-hygiene.yml, which calls this recipe.
check-no-third-party-data:
    #!/usr/bin/env bash
    set -euo pipefail
    tracked="$(git ls-files -- interoperability/)"
    if [ -z "${tracked}" ]; then
      echo "No tracked paths under interoperability/."
      exit 0
    fi
    if [ -n "${GITHUB_ACTIONS:-}" ]; then
      echo "::error title=Third-party data committed::interoperability/ must not be tracked by git"
    fi
    echo "These paths under interoperability/ are tracked in this commit:"
    echo "${tracked}" | sed 's/^/  /'
    echo
    echo "To fix it, from the repository root:"
    echo
    echo "  git rm -r --cached interoperability/"
    echo "  git commit -m 'chore: untrack interoperability/'"
    echo
    echo "That removes the files from the index while leaving them on disk. If the"
    echo "commit that added them has already been pushed, untracking is not enough:"
    echo "the content is still reachable from the earlier commits, so rewrite the"
    echo "branch (git rebase -i, or git filter-repo for anything older) before"
    echo "merging, and treat the material as disclosed if the branch was public."
    exit 1

# --- Release ---

# Releases are driven by .github/workflows/release.yml on a `v*` tag; the two
# recipes below are the local dry runs for the config that workflow uses. The
# process itself — SemVer rules, tagging, the pre-tag checklist — lives in
# docs/policies/releases.md.
#
# Both need goreleaser >= v2.6 locally. An older binary rejects the config as
# having unknown fields rather than as being too new for it; the workflow pins
# the version it uses in GORELEASER_VERSION.

# Validate .goreleaser.yaml without building anything
release-check:
    goreleaser check

# Outputs to bin/dist/ (see the `dist` key in .goreleaser.yaml); --snapshot
# already implies --skip=publish, so nothing leaves the machine.

# Build the release artifacts locally, without tagging or publishing
release-snapshot:
    goreleaser release --snapshot --clean

# --- General ---

# Clean build artifacts
clean:
    rm -rf bin/ backend/coverage.out backend/coverage.html frontend/dist frontend/public/aconiq.wasm frontend/public/wasm_exec.js

fix:
    just lint-fix
    just fmt
