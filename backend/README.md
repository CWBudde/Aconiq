# Backend

Go backend (CLI-first) for project management, validation, compute engine, standards modules, and reporting.

Planned package structure:

- `cmd/noise/`
- `internal/app/`
- `internal/domain/`
- `internal/geo/`
- `internal/engine/`
- `internal/standards/`
- `internal/io/`
- `internal/report/`
- `internal/qa/`

## QA Baseline

- Golden snapshot helper: `internal/qa/golden`
- Golden update workflow: run `just update-golden` from repository root
- Determinism and formatting policies: see `docs/policies/`

## Phase 3 Baseline

- Project store: `internal/io/projectfs` (JSON manifest in `.noise/project.json`)
- Domain entities: `internal/domain/project`
- Implemented commands: `noise init`, `noise import`, `noise validate`, `noise status`

## Phase 4 Baseline

- GeoJSON normalization/validation package: `internal/geo/modelgeojson`
- Import debug exports:
  - `.noise/model/model.normalized.geojson`
  - `.noise/model/model.dump.json`
  - `.noise/model/validation-report.json`

## Phase 5 Baseline

- Geo core package: `internal/geo`
- CRS model + transform pipeline contract
- Geometry utilities + spatial index + receiver set models

## Phase 6 Baseline

- Result container package: `internal/report/results`
- Raster API + custom binary/JSON persistence
- Receiver table API + CSV/JSON writers
- `noise export` skeleton command with run bundle export

## Phase 7 Baseline

- Compute engine skeleton package: `internal/engine`
- Staged pipeline with progress events and deterministic reduction
- Chunked worker pool, cancellation handling, and per-run/per-chunk disk cache

## Phase 8 Baseline

- Dummy standard module: `internal/standards/dummy/freefield` (explicitly non-normative)
- `noise run --standard dummy-freefield` executes end-to-end:
  - loads normalized model
  - builds receiver grid
  - executes engine
  - writes run results under `.noise/runs/<run-id>/results`
- Persisted outputs:
  - receiver table (`receivers.json`, `receivers.csv`)
  - raster (`ldummy.json`, `ldummy.bin`)
  - run summary (`run-summary.json`)
- Golden E2E fixture + snapshot:
  - `internal/app/cli/testdata/phase8/model.geojson`
  - `internal/app/cli/testdata/phase8-dummy-freefield.golden.json`

## Phase 9 Baseline

- Standards framework package: `internal/standards/framework`
  - standard descriptor model (ID, version/profile, supported source types, indicators)
  - run parameter schema model with normalization + validation
  - version/profile resolution
- Registry package: `internal/standards`
  - central registration of available standards
- Dummy module integration:
  - `dummy-freefield` exported as a standards descriptor with profiles (`default`, `highres`)
- `noise run` integration:
  - resolves selected standard/version/profile through registry
  - validates and normalizes `--param` values against profile schema
  - records resolved standard + normalized parameters in run provenance

## Phase 10 Baseline

- CNOSSOS road module: `internal/standards/cnossos/road`
  - typed road source schema (`RoadSource`) with validation for speed/surface/traffic inputs
  - deterministic piecewise emission model for day/evening/night periods
  - propagation chain baseline (distance + air/ground/barrier attenuation terms)
  - indicators:
    - `Lday`, `Levening`, `Lnight`
    - `Lden` aggregation
  - export helper for:
    - receiver table (`receivers.json`, `receivers.csv`)
    - raster bands (`Lden`, `Lnight`) via `cnossos-road.json/bin`
- Standards registry now includes `cnossos-road` descriptor.
- `noise run --standard cnossos-road` is wired to line-source model extraction and result export.

## Phase 11 Baseline

- CNOSSOS rail module: `internal/standards/cnossos/rail`
  - typed rail source schema (`RailSource`) with validation for traction, roughness, speed, braking, traffic, and geometry
  - deterministic rail emission path (rolling + traction + braking terms)
  - rail-specific propagation adjustments (bridge + curve squeal)
  - indicators:
    - `Lday`, `Levening`, `Lnight`
    - `Lden` aggregation
  - export helper for:
    - receiver table (`receivers.json`, `receivers.csv`)
    - raster bands (`Lden`, `Lnight`) via `cnossos-rail.json/bin`
- Standards registry now includes `cnossos-rail` descriptor.
- Golden regression scenario:
  - `internal/standards/cnossos/rail/testdata/rail_scenario.json`
  - `internal/standards/cnossos/rail/testdata/rail_scenario.golden.json`

## Phase 20 Baseline

- Reporting package: `internal/report/reporting`
  - generates `report-context.json`, `report.md`, `report.html`
  - report sections:
    - input overview
    - standard ID/version/profile + parameters
    - maps/images from raster artifacts
    - receiver table statistics
    - QA status summary
- `noise export` now copies run result artifacts into export bundles and generates report files by default.
- Optional flag: `--skip-report` to export bundle artifacts without report generation.

## Phase 22 Initial Slice

- CLI command: `noise bench`
  - runs built-in synthetic scenarios: `micro`, `corridor`, `district`
  - captures:
    - runtime
    - Go memory counters
    - cache/disk footprint (`run_dir_bytes_*`, chunk cache file count, cache reuse)
    - numeric drift against a single-worker reference run
  - persists benchmark suite summaries under `.noise/cache/bench/<bench-id>/summary.json`
  - prunes older benchmark suites with `--keep-last` to keep bench cache growth bounded
- Engine cache behavior:
  - per-run chunk cache remains under the run cache directory
  - shared keyed chunk cache under `.noise/cache/shared-chunks/` enables reuse across equivalent runs
  - cache keys include input receivers, sources, and cache format version so changed inputs invalidate stale artifacts

## Phase 23 Initial Slice

- Local API package: `internal/api/httpv1`
  - `GET /api/v1/health`
  - `GET /api/v1/project/status`
  - `GET /api/v1/events` (SSE stream: heartbeat + project status snapshots)
  - standardized JSON error envelope (`code`, `message`, `details`, `hint`)
- CLI command: `noise serve`
  - starts local HTTP server (default `127.0.0.1:8080`)
  - graceful shutdown on `SIGINT`/`SIGTERM`
