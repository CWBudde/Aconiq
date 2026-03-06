# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Soundplan is a SoundPLAN-like environmental noise modeling system (CLI-first, offline-first). The Go backend is the active development target; the React/TypeScript frontend is deferred. Currently at **Phase 6** (result containers completed; Phases 0–6 done).

## Commands

All backend commands run from `backend/` unless noted.

**Build & run:**
```bash
cd backend && go build ./...
cd backend && go run ./cmd/noise -- --help
```

**Tests:**
```bash
scripts/test-go.sh          # runs go test ./... from backend/
```

**Update golden snapshots** (after intentional behavior changes):
```bash
scripts/update-golden.sh    # runs UPDATE_GOLDEN=1 go test ./...
```

**Format (mandatory before commit):**
```bash
gofmt -w $(find backend -type f -name '*.go')
scripts/check-gofmt.sh      # CI enforcer; fails on unformatted files
```

**Vet:**
```bash
cd backend && go vet ./...
```

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
scripts/          CI/build automation
examples/         License-safe sample projects (reserved)
```

### Go Package Structure (`backend/internal/`)

| Package | Responsibility |
|---|---|
| `app/cli/` | Cobra commands (`init`, `import`, `validate`, `run`, `status`, `export`, `bench`) |
| `app/config/` | Config loading (project path, log level, cache dir) |
| `app/logging/` | Structured logging baseline |
| `domain/project/` | Core entities: `Project`, `Scenario`, `Run`, `StandardRef`, `ArtifactRef` |
| `domain/errors/` | Error taxonomy (user input vs internal) |
| `geo/` | CRS, geometry primitives, spatial index, receiver sets |
| `geo/modelgeojson/` | GeoJSON normalization and validation |
| `io/projectfs/` | Project folder store (JSON manifest in `.noise/project.json`) |
| `report/results/` | Raster container API + receiver table API |
| `qa/golden/` | Golden snapshot helper |
| `engine/` | Compute engine (not yet implemented) |
| `standards/` | Standards modules (not yet implemented) |

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

**Formatting:** `gofmt` is mandatory; CI will fail without it.

**Golden tests:** Snapshots live in `testdata/` next to the owning test package, named `<scenario>.golden.<ext>`. Update only intentionally via `scripts/update-golden.sh`; review diffs before committing.

**Floating-point:** Keep calculations at `float64`. Apply rounding only at defined output boundaries. Document rounding rules per standards module. Use stable (pairwise/compensated) summation for sensitive reductions.

## Module Name

`github.com/soundplan/soundplan/backend`
