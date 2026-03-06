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
- Golden update workflow: run `scripts/update-golden.sh` from repository root
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
