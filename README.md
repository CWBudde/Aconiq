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

| Command                   | Purpose                                                                                   |
| ------------------------- | ----------------------------------------------------------------------------------------- |
| `aconiq init`             | Create a project (`.noise/`)                                                              |
| `aconiq import`           | Import model data (see formats below)                                                     |
| `aconiq validate`         | Validate the normalized model and write a validation report                               |
| `aconiq reattach-facades` | Re-key facade receivers whose building was re-imported, renumbered or moved               |
| `aconiq run`              | Run one scenario against one standard, version and profile                                |
| `aconiq status`           | Report project, scenario and run state                                                    |
| `aconiq compare`          | Compare results against imported SoundPLAN receiver results with a dB tolerance           |
| `aconiq export`           | Export a run bundle, generate offline reports, and emit GIS formats                       |
| `aconiq bench`            | Run synthetic benchmark scenarios and measure runtime, memory, cache IO and numeric drift |
| `aconiq serve`            | Start the local HTTP API                                                                  |
| `aconiq mcp`              | Serve the project to an AI agent over the Model Context Protocol (stdio)                  |
| `aconiq openapi`          | Export the OpenAPI contract for the local API (default `.noise/api/openapi.v1.json`)      |

All commands accept `--project`, `--cache-dir`, `--verbose` and `--json`. A command's result goes
to stdout and its log records to stderr; `--json` makes both JSON, and stdout then carries exactly
one JSON document. `--help` and `--version` are the exceptions: they print text either way, and
`aconiq mcp`'s stdout is the protocol stream.

## Supported Input Formats

`aconiq import` reads:

- **GeoJSON** — the canonical model format for the main workflow
- **GeoPackage** (`.gpkg`, with `--layer`), **FlatGeobuf** (`.fgb`), **CityGML** 1.0/2.0/3.0 (`.gml`/`.citygml`, every `BuildingPart` becomes its own building; a LoD3 `bldg:Window` in a wall becomes a window on its facade, proposed and not sensitive) — CRS auto-detected where the format carries it, otherwise `--input-crs`
- **SoundPLAN** project directories via `--from-soundplan`
- **OpenStreetMap** via `--from-osm "south,west,north,east"` against an Overpass endpoint
- **LGLN LoD2 buildings** (Lower Saxony) via `--from-lgln "south,west,north,east"` — merged into
  the existing model rather than replacing it (see below)
- **CSV** attribute/traffic tables merged into model features via `--traffic`
- **Window lists** (CSV or JSON, each window a rectangle in its facade's frame) via `--windows` —
  merged into the existing model; `--receiver-mode facade` places its points at the sensitive ones
- **GeoTIFF** digital terrain models via `--terrain`, queried with bilinear interpolation

The official **LGLN LoD2 buildings** for a box in Lower Saxony load through `aconiq import
--from-lgln` or the web UI's Import page (against `aconiq serve`). Both find the 1 km CityGML
tiles through the LGLN STAC API, cache them in `.noise/cache/lgln/`, and swap the LGLN buildings
in for the OSM buildings whose footprint centroid lies in the box. Everything else in the model —
roads, barriers, receivers, the calculation area, drawn buildings — stays, and loading the same
box again changes nothing. The data is CC BY 4.0 — keep the "Quelle: LGLN (year)" note the
import prints.

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
- Model hand-off to SoundPLAN via `--format soundplan-shp`: ESRI Shapefiles per object type, the DTM as elevation points, and `soundplan-mapping.json` naming every column and every property that could not be carried (see `docs/research/soundplan-export.md`)
- `--target-crs` re-projects the exported model GeoJSON

## Local HTTP API

`aconiq serve` binds `127.0.0.1:8080` by default and serves:

```
GET  /api/v1/health
GET  /api/v1/project/status
GET  /api/v1/standards
GET  /api/v1/runs                  list runs
POST /api/v1/runs                  start a run: 202 with its id, computed in the background
GET  /api/v1/runs/{id}             one run, with its progress while it computes
POST /api/v1/runs/{id}/cancel      stop a run this server started
GET  /api/v1/runs/{id}/log
GET  /api/v1/runs/{id}/provenance  the run's provenance.json
GET  /api/v1/artifacts/{id}/content
GET  /api/v1/events                server-sent events: heartbeat and project status
POST /api/v1/import/osm
POST /api/v1/import/terrain
POST /api/v1/model                 replace the project model with normalized GeoJSON
POST /api/v1/transform             project a batch of coordinates between two CRS
GET  /api/v1/openapi.json
```

Errors use a single JSON envelope (`code`, `message`, `details`, `hint`).

## MCP Server

`aconiq mcp` serves one project to an AI agent over the
[Model Context Protocol](https://modelcontextprotocol.io). It speaks MCP over stdin and stdout and
opens no socket. The project is the one `--project` names, fixed for the life of the process, and
no tool takes a path. Each tool is a thin adapter over the local HTTP API, served in process: a
call it forwards is judged by the API and answered with the API's own JSON, error envelope
included. Before forwarding, the MCP layer refuses arguments the tool's input schema does not
accept — a missing required field, a wrong type, an unknown property — and a `save_model`
without `confirm`. `start_run` and `estimate_run` require the scenario, standard, version, profile
and receiver mode, non-empty, where the CLI would default them. stdout carries the protocol
and nothing else; logs go to stderr.

### Setup

With Claude Code:

```bash
claude mcp add aconiq -- aconiq mcp --project /path/to/project
# or, to let the agent save the model and start runs:
claude mcp add aconiq -- aconiq mcp --project /path/to/project --allow-writes
```

Any other MCP client (Claude Desktop, IDE extensions) takes the same command in its server
configuration:

```json
{
  "mcpServers": {
    "aconiq": {
      "command": "aconiq",
      "args": ["mcp", "--project", "/path/to/project"]
    }
  }
}
```

### Tools

Always offered (read-only):

| Tool              | What it answers                                                                                          |
| ----------------- | -------------------------------------------------------------------------------------------------------- |
| `project_status`  | Name, CRS, scenario and run counts, open runs with progress, and the model and terrain hashes            |
| `list_standards`  | The runnable standards with their evidence tier, versions, profiles and parameters (kind, unit, default) |
| `get_model`       | The saved model as GeoJSON, optionally in another CRS (`crs`), with its hash                             |
| `transform`       | Project a coordinate batch between two CRS; `target_crs: "auto"` picks the zone `aconiq run` would use   |
| `list_runs`       | Every run, newest first, with status, standard and artifacts                                             |
| `get_run`         | One run's summary and, while it computes, its progress — poll it, there is no stream                     |
| `get_run_log`     | One run's log                                                                                            |
| `get_run_results` | A finished run's receiver table with units; one over 256 KiB is answered with a link to its artifact     |
| `get_contours`    | Contour lines from a finished grid run, at an `interval` in dB, in the CRS named in the answer           |
| `estimate_run`    | What a run with these arguments would cost in time, from a timed probe; it creates no run                |

Only with `--allow-writes`:

| Tool         | What it does                                                                                                                                             |
| ------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `start_run`  | Start a run of the saved model; scenario, standard, version, profile and receiver mode (`auto-grid`, `custom` or `facade`) are required, never defaulted |
| `cancel_run` | Stop a run this server started                                                                                                                           |
| `save_model` | Replace the whole model; `confirm` must be the hash of the model being replaced, or `none` on a project without one                                      |

Resources: `aconiq://project`, `aconiq://standards`, `aconiq://runs/{id}`,
`aconiq://runs/{id}/provenance` (the run's `provenance.json` as recorded) and
`aconiq://artifacts/{id}`.

### Safety

- Without `--allow-writes` the tools that change the project are not listed at all, so an agent
  cannot plan around them.
- `save_model`'s `confirm` binds the save to the model the agent read: a model that changed since
  is refused with `hash_mismatch`, not overwritten. A model that fails the GeoJSON schema is refused
  with its validation issues; nothing is corrected.
- A scaffold-tier standard is refused unless `start_run` passes `experimental: true`, as on the CLI.
- A normative run over facade receivers (`receiver_mode: "facade"`, or model receivers carrying a
  facade link) that are still proposed, or whose slab height is assumed, is refused unless
  `start_run` passes `allow_unconfirmed: true`; their counts then go into the run's provenance and
  report.
- Every state-changing call, refused ones included, is recorded in `.noise/logs/mcp-audit.jsonl`
  with a digest of its arguments, never the payload. The log is opened before the call, and a
  write whose log cannot be opened is not attempted. If the entry then fails to write — a full
  disk, say — the call has already happened: its result carries a warning and the failure is
  logged at ERROR, but the log has no line for it.
- On exit the server interrupts the runs it started and waits a bounded time for them to stop. A
  run still open after that is closed as interrupted by the next `aconiq mcp` or `aconiq serve`.

The boundaries, the security policy and the exposed fields are specified in
[ADR-002](docs/adr/002-mcp-server-boundaries.md).

### Example prompts

- "Which standards in this project are normative, and which parameters does `rls19-road` take, with
  their units and defaults?"
- "Show me the model's receivers in EPSG:4326."
- "Raise receiver `rcv-3` to 6 m and save the model." (needs `--allow-writes`)
- "Estimate how long an `rls19-road` run over the model's receivers would take, then start it and
  list the five loudest receivers by night." (needs `--allow-writes`)
- "Give me the 5 dB contours of the latest grid run in EPSG:4326."
- "Why did the last run fail? Read its log."

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
