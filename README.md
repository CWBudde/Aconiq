# Aconiq

Aconiq is an environmental noise modeling system: CLI-first, offline-first, and deterministic. It is built for auditable calculation and automation — every run records the standard, version, profile, parameters and input hashes it used, and the same inputs always produce the same numbers.

Monorepo contents:

- Go backend (`backend/`) — CLI, local HTTP API, compute engine, standards modules, reporting and export
- React/TypeScript frontend (`frontend/`) — local UI over the API, plus a WebAssembly compute mode
- Roadmap and open work (`PLAN.md`); specs, policies and conformance declarations (`docs/`)

**`PLAN.md` is the single source of truth for project status.** It is a forward-only roadmap of what is still ahead; it is candid about where the calculations do and do not yet carry evidence, and you should read it before relying on any module here.

## Repository Layout

- `backend/` — Go application: domain logic, engine, standards modules, IO, reporting, QA harness
- `frontend/` — React 19 + TypeScript + Vite + Bun + shadcn/ui + MapLibre browser UI
- `docs/` — format and schema specs, policies (`determinism`, `formatting`), ADRs, testing notes, per-module baselines
- `docs/conformance/` — German-language conformance declarations per normative module
- `docs/preflight/` — early project decisions: compliance boundaries, target platforms, definition of done, risk register, offline-only constraints
- `interoperability/` — reference material per standard (GTA, ISO 9613-2, RLS-19, Schall 03, TA Lärm)
- `examples/`, `scripts/` — reserved, currently empty

## Development

Prerequisites: [Go](https://go.dev/), [just](https://github.com/casey/just), [golangci-lint](https://golangci-lint.run/), [treefmt](https://github.com/numtide/treefmt). The frontend additionally needs [Bun](https://bun.sh/); `aconiq export --pdf` shells out to a [Typst](https://typst.app/) binary.

```bash
just          # list all available recipes
just build    # build the CLI into bin/aconiq
just test     # run Go tests
just lint     # run golangci-lint
just fmt      # format all files
just go-ci    # full Go gate: format, vet, lint, test, race, tidy, vulncheck, licenses, build
just ci       # go-ci plus the frontend gate (typecheck, lint, test, build, bundle budget)
just dev      # run the API on :8080 and the frontend dev server on :5173
```

Note that `just check-formatted` runs `treefmt --fail-on-change`, which formats in place before reporting — it rewrites files rather than only checking them.

## Command-Line Interface

| Command           | Purpose                                                                                   |
| ----------------- | ----------------------------------------------------------------------------------------- |
| `aconiq init`     | Create a project (`.noise/`)                                                              |
| `aconiq import`   | Import model data (see formats below)                                                     |
| `aconiq validate` | Validate the normalized model and write a validation report                               |
| `aconiq run`      | Run one scenario against one standard, version and profile                                |
| `aconiq status`   | Report project, scenario and run state                                                    |
| `aconiq compare`  | Compare results against imported SoundPLAN receiver results with a dB tolerance           |
| `aconiq export`   | Export a run bundle, generate offline reports, and emit GIS formats                       |
| `aconiq bench`    | Run synthetic benchmark scenarios and measure runtime, memory, cache IO and numeric drift |
| `aconiq serve`    | Start the local HTTP API                                                                  |
| `aconiq openapi`  | Export the OpenAPI contract for the local API (default `.noise/api/openapi.v1.json`)      |

All commands accept `--project`, `--cache-dir`, `--verbose` and `--json`.

## Supported Input Formats

`aconiq import` reads:

- **GeoJSON** — the canonical model format for the main workflow
- **GeoPackage** (`.gpkg`, with `--layer`), **FlatGeobuf** (`.fgb`), **CityGML** (`.gml`/`.citygml`) — CRS auto-detected where the format carries it, otherwise `--input-crs`
- **SoundPLAN** project directories via `--from-soundplan`
- **OpenStreetMap** via `--from-osm "south,west,north,east"` against an Overpass endpoint
- **CSV** attribute/traffic tables merged into model features via `--traffic`
- **GeoTIFF** digital terrain models via `--terrain`, queried with bilinear interpolation

### Model Schema

A `FeatureCollection` whose features carry a `kind`:

| `kind`     | Geometry                          | Required properties                         |
| ---------- | --------------------------------- | ------------------------------------------- |
| `source`   | matches `source_type` (see below) | `source_type` = `point` \| `line` \| `area` |
| `building` | `Polygon` / `MultiPolygon`        | `height_m > 0`                              |
| `barrier`  | `LineString` / `MultiLineString`  | `height_m > 0`                              |
| `receiver` | `Point`                           | `height_m > 0`                              |

Source geometry must match `source_type`: `point` → `Point`/`MultiPoint`, `line` → `LineString`/`MultiLineString`, `area` → `Polygon`/`MultiPolygon`.

Import writes normalization and debug artifacts to `.noise/model/model.normalized.geojson`, `model.dump.json` and `validation-report.json`.

## Standards Modules

Standards are plug-in modules registered in a central registry and described by a common descriptor framework (ID, version, profile, supported source types, indicators, parameter schema). The registry currently exposes 13 standard IDs.

**They are not equivalent in maturity, and the names do not tell you which are which.** The authoritative breakdown is the "Standards evidence tiers" table in `PLAN.md`; in summary:

| Module                                                                 | Tier             | What that means                                                                                        |
| ---------------------------------------------------------------------- | ---------------- | ------------------------------------------------------------------------------------------------------ |
| `rls19-road`, `schall03`, `iso9613`                                    | normative intent | Real normative structure, coefficients and tables, with open defects tracked in `PLAN.md`              |
| `beb-exposure`                                                         | preview          | Reasonable aggregation logic, but it consumes preview-grade input levels                               |
| `cnossos-road`, `cnossos-rail`, `cnossos-industry`, `cnossos-aircraft` | **scaffold**     | No directive coefficients, invented base levels, no octave bands — not an implementation of CNOSSOS-EU |
| `bub-road`, `bub-rail`, `bub-industry`, `buf-aircraft`                 | **scaffold**     | Re-parameterised clones of or aliases over the CNOSSOS scaffolds                                       |
| `dummy-freefield`                                                      | test fixture     | Intentionally non-normative                                                                            |

Assessment logic against German limit values lives alongside the standards: **16. BImSchV** (wired into export bundles) and **TA Lärm** (implemented as a library; not yet exposed through a CLI command). Published conformance boundaries are in `docs/conformance/`.

Normative outputs may only come from normative modules; DSP/FFT-based tooling is non-normative post-processing only.

## Design Principles

- Never let a module's name assert more than its evidence supports
- Core engine, geometry, IO, reporting and standards are separate modules
- Standards are plug-in method modules rather than engine-internal special cases
- Runs are auditable through deterministic execution, persisted artifacts and provenance metadata
- Determinism is a product feature: identical outputs regardless of worker count, fixed-order reduction, no map-iteration influence on numbers

## Project Storage

A project is a folder containing `.noise/`:

- `project.json` — manifest of scenarios, runs, artifacts and migrations
- `runs/<run-id>/run.log`, `provenance.json`, and `results/`
- `model/` — normalized model and validation artifacts
- `artifacts/`, `logs/`, `exports/`
- `cache/` — engine chunk caches (per-run and shared) and benchmark suites

## Outputs and Exports

- Receiver tables as `receivers.json` and `receivers.csv`
- Raster result containers as JSON metadata plus a little-endian `float64` binary payload
- `run-summary.json` per run; provenance manifests with hashed input tracking
- Export bundles (`aconiq export`) with copied run artifacts, model dump and `export-summary.json`
- Offline reports generated by default: `report-context.json`, `report.md`, `report.html`, `report.typ` — and `report.pdf` with `--pdf` (requires an external `typst` binary)
- GIS export via `--format`: `geotiff`, `cog` (Cloud Optimized GeoTIFF), `gpkg`, `contour-geojson`, `contour-gpkg`, with `--contour-interval` in dB
- `--target-crs` re-projects the exported model GeoJSON

## Local HTTP API

`aconiq serve` binds `127.0.0.1:8080` by default and serves:

```
GET  /api/v1/health
GET  /api/v1/project/status
GET  /api/v1/standards
GET  /api/v1/runs                  list runs
POST /api/v1/runs                  start a run
GET  /api/v1/runs/{id}/log
GET  /api/v1/artifacts/{id}/content
GET  /api/v1/events                server-sent events: heartbeat and project status
POST /api/v1/import/osm
POST /api/v1/import/terrain
GET  /api/v1/openapi.json
```

Errors use a single JSON envelope (`code`, `message`, `details`, `hint`).

## Scope Boundaries

- The local API is designed for local-first GUI and automation use, not multi-user deployment
- Result persistence uses custom raster containers plus CSV/JSON tables as the default on-disk format; GeoTIFF, COG and GeoPackage are export targets rather than the working format
- The project format is local-first; a server/PostGIS format v2 is future work

## Deferred Work

Selected items from `PLAN.md` that are explicitly not implemented today:

- Vector tiles and an end-to-end PMTiles pipeline for model and result delivery
- DACH report packages: German Gutachten templates (TA Lärm, 16. BImSchV, Schallimmissionsprognose), embedded templates, PDF golden checks, and a template versioning policy
- Broader browser workflows: model editing, scenario comparison, contour overlays, run-to-run diffs, large-map performance guardrails
- Desktop packaging via Wails, with the API running in-process and frontend assets embedded
- Project format v2 with PostGIS storage and object-storage artifacts

## License

See `LICENSE` and `NOTICE`. Contribution guidance is in `CONTRIBUTING.md`; security reporting in `SECURITY.md`.
