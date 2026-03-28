# AGENTS.md

This file provides guidance to AI agents (Claude Code, Codex etc.) when working with code in this repository.

## Project Overview

Aconiq is an environmental noise modeling system (CLI-first, offline-first). The Go backend is the active development target; the React/TypeScript frontend is deferred. Currently at **Phase 6** (result containers completed; Phases 0–6 done).

## Commands

All common tasks are orchestrated via [`just`](https://github.com/casey/just) from the repo root. Run `just` to list available recipes.

| Command                | What it does                                       |
| ---------------------- | -------------------------------------------------- |
| `just fmt`             | Format all files (Go, shell, markdown, YAML, JSON) |
| `just check-formatted` | Check formatting without writing (CI)              |
| `just lint`            | Run golangci-lint (`backend/`)                     |
| `just lint-fix`        | Run golangci-lint with `--fix`                     |
| `just test`            | Run all Go tests                                   |
| `just test-race`       | Run tests with race detector                       |
| `just test-coverage`   | Run tests with coverage report                     |
| `just update-golden`   | Update golden snapshots (`UPDATE_GOLDEN=1`)        |
| `just build`           | Build CLI → `bin/noise`                            |
| `just check-tidy`      | Verify `go.mod` is tidy                            |
| `just license-check`   | Check dependency licenses for policy violations    |
| `just license-report`  | Generate CSV report of all dependency licenses     |
| `just ci`              | Run all checks (format, test, lint, tidy, license) |
| `just clean`           | Remove build artifacts                             |

**Run a single test:**

```bash
cd backend && go test ./internal/geo/... -run TestFunctionName
```

**Fuzz tests:**

```bash
cd backend && go test ./internal/geo/... -fuzz FuzzFunctionName
```

## Architecture

### Monorepo Layout

```
backend/          Go application (active)
frontend/         React/TypeScript (deferred)
docs/             Specs, policies, research notes
scripts/          CI/build automation (legacy, being replaced by justfile)
examples/         License-safe sample projects (reserved)
justfile          Task runner (just) — primary entry point for dev commands
.golangci.yml     golangci-lint v2 config (all linters enabled, tuned exclusions)
treefmt.toml      Multi-language formatter config (gofumpt, gci, shfmt, prettier)
```

### Go Package Structure (`backend/internal/`)

| Package             | Responsibility                                                                    |
| ------------------- | --------------------------------------------------------------------------------- |
| `app/cli/`          | Cobra commands (`init`, `import`, `validate`, `run`, `status`, `export`, `bench`) |
| `app/config/`       | Config loading (project path, log level, cache dir)                               |
| `app/logging/`      | Structured logging baseline                                                       |
| `domain/project/`   | Core entities: `Project`, `Scenario`, `Run`, `StandardRef`, `ArtifactRef`         |
| `domain/errors/`    | Error taxonomy (user input vs internal)                                           |
| `geo/`              | CRS, geometry primitives, spatial index, receiver sets                            |
| `geo/modelgeojson/` | GeoJSON normalization and validation                                              |
| `io/projectfs/`     | Project folder store (JSON manifest in `.noise/project.json`)                     |
| `report/results/`   | Raster container API + receiver table API                                         |
| `qa/golden/`        | Golden snapshot helper                                                            |
| `engine/`           | Compute engine (not yet implemented)                                              |
| `standards/`        | Standards modules (not yet implemented)                                           |

### Project Format v1

A project is a folder with `.noise/` containing:

- `.noise/project.json` — manifest (scenarios, runs, artifacts, migrations)
- `.noise/runs/<run-id>/run.log` — run log
- `.noise/runs/<run-id>/provenance.json` — standard ID, version, parameters, input hashes
- `.noise/model/` — import debug exports (normalized GeoJSON, dump, validation report)
- `.noise/artifacts/` — generated artifacts
- `.noise/exports/` — export bundles

### GeoJSON Input Schema (v1)

`noise import` accepts a `FeatureCollection` with features having `kind` = `source`, `building`, or `barrier`. Sources require `source_type` (`point`|`line`|`area`); buildings and barriers require `height_m > 0`.

### Result Containers v1

- **Raster:** custom binary (`float64` LE) + JSON metadata sidecar in `internal/report/results`
- **Receiver table:** CSV + JSON, with ordered indicators and validation

### Standards Architecture (planned)

Standards are plug-in modules under `internal/standards/` (e.g., `cnossos/`, `rls19/`, `schall03/`, `iso9613/`, `bub/`). Each module is independently testable. **Normative outputs must only come from normative modules** — DSP/FFT-based tools are non-normative post-processing only.

## Key Policies

**Determinism:** Same inputs + standard/profile → identical outputs regardless of worker count. Map iteration must never influence numeric results. Partial results merge in fixed order (no "first finished wins"). See `docs/policies/determinism.md`.

**Formatting:** Enforced via `just fmt` (treefmt: gofumpt + gci + shfmt + prettier). `just check-formatted` is the CI gate.

**Linting:** `just lint` runs golangci-lint v2 with all linters enabled and project-tuned exclusions. Fix issues before committing.

**Golden tests:** Snapshots live in `testdata/` next to the owning test package, named `<scenario>.golden.<ext>`. Update only intentionally via `just update-golden`; review diffs before committing.

**Floating-point:** Keep calculations at `float64`. Apply rounding only at defined output boundaries. Document rounding rules per standards module. Use stable (pairwise/compensated) summation for sensitive reductions.

## Module Name

`github.com/aconiq/backend`
