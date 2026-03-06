# Aconiq

Monorepo for an environmental noise modeling system (CLI-first, offline-first) with:

- Go backend (`backend/`)
- React/TypeScript frontend workspace (`frontend/`)
- Planning and governance docs (`docs/`, `PLAN.md`, `goal.md`)

## Repository Layout

- `backend/`: Go application, domain logic, standards modules, QA harness
- `frontend/`: browser UI workspace (deferred implementation)
- `docs/preflight/`: Phase 0 decisions (compliance, platforms, DoD, risks, constraints)
- `scripts/`: legacy CI scripts (superseded by `justfile`)
- `examples/`: license-safe sample projects and fixtures

## Development

Prerequisites: [Go](https://go.dev/), [just](https://github.com/casey/just), [golangci-lint](https://golangci-lint.run/), [treefmt](https://github.com/numtide/treefmt)

```bash
just          # list all available recipes
just ci       # run full check suite (format, test, lint, tidy)
just fmt      # format all files
just lint     # run linter
just test     # run tests
```

## Supported Capabilities

- Project-oriented local workflow via `noise init`, `noise import`, `noise validate`, `noise status`, `noise run`, `noise export`, `noise serve`, and `noise openapi`
- Local project storage under `.noise/` with JSON manifest, run logs, provenance files, run result bundles, and exports
- Deterministic offline execution with chunked workers, cancellation handling, disk-backed cache, and golden-test coverage

## Design Principles

- Core engine, geometry, IO, reporting, and standards are separated into distinct modules
- Standards are implemented as plug-in style method modules instead of being hard-coded into the engine
- Normative outputs stay separate from non-normative demonstrators and research/post-processing pipelines
- Runs are auditable through deterministic execution, persisted artifacts, and provenance metadata

## Supported Input Model

- GeoJSON `FeatureCollection` import and validation
- Feature kinds: `source`, `building`, `barrier`
- Source geometries:
  - `point`: `Point` or `MultiPoint`
  - `line`: `LineString` or `MultiLineString`
  - `area`: `Polygon` or `MultiPolygon`
- Building geometries: `Polygon` or `MultiPolygon` with `height_m > 0`
- Barrier geometries: `LineString` or `MultiLineString` with `height_m > 0`
- Normalized model/debug exports:
  - `.noise/model/model.normalized.geojson`
  - `.noise/model/model.dump.json`
  - `.noise/model/validation-report.json`

## Supported Standards

- `dummy-freefield` non-normative demonstrator standard
- EU / strategic mapping family:
  - `cnossos-road`
  - `cnossos-rail`
  - `cnossos-industry`
  - `cnossos-aircraft`

- Germany mapping family:
  - `bub-road`
  - `bub-rail`
  - `bub-industry`
  - `buf-aircraft`
  - `beb-exposure`

- Germany project / planning family:
  - `rls19-road`
  - `schall03`

## Scope Boundaries

- GeoJSON is the model import format for the main workflow
- Result persistence uses custom raster containers plus CSV/JSON tables rather than GeoTIFF as the default on-disk format
- The local HTTP API is designed for local-first GUI and automation use
- The frontend workspace is present in the repository, while larger browser workflows remain phased work

## Supported Outputs and Exports

- Receiver tables in JSON and CSV
- Raster result containers as JSON metadata + little-endian `float64` binary payload
- Run summaries, provenance manifests, and hashed input tracking
- Export bundles with copied run artifacts, model dump, and `export-summary.json`
- Offline report generation with:
  - `report-context.json`
  - `report.md`
  - `report.html`

## Supported Local API

- `GET /api/v1/health`
- `GET /api/v1/project/status`
- `GET /api/v1/standards`
- `GET /api/v1/runs`
- `GET /api/v1/runs/{id}/log`
- `GET /api/v1/artifacts/{id}/content`
- `GET /api/v1/events` (SSE)
- `GET /api/v1/openapi.json`

## Deferred Roadmap Highlights

- ISO 9613-2 as an additional industry / engineering method module
- Additional import/export paths such as GeoPackage, FlatGeobuf, CSV traffic tables, GeoTIFF, and tiled/PMTiles-based delivery
- Typst-based PDF report export and richer report template/versioning support
- Expanded browser workflows around modeling, scenario comparison, result visualization, and large-map tiling
- Optional desktop packaging via Wails after the local API/web workflow matures
