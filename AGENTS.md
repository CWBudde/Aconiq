# AGENTS.md

This file provides guidance to AI agents (Claude Code, Codex etc.) when working with code in this repository. It describes **structure, commands and policies** — the things that stay true between sessions.

## Where status lives

`PLAN.md` is the single source of truth for project status and open work. It is forward-only: it records what is still ahead, not what is done. Completed work is recorded in git history, in `docs/conformance/`, and in the per-module baseline notes under `docs/`.

The one exception is a priority's **`### Landed`** checklist. Where the completed items are load-bearing for everything after them — the build and the CI gates, above all — they stay as a short `- [x]` list, because a reader has to know which gates already hold before trusting the open items underneath. Those entries are checklist lines, not retrospectives: name the change, cite the commit, and add only what changes future behaviour, such as a constraint that is still live or a belief the work proved wrong. The detail belongs in the commit message. An entry that reads like a delivery report has outgrown this file.

Do not restate status here, and do not infer it from this file. Read `PLAN.md`.

## Project Overview

Aconiq is an environmental noise modeling system: CLI-first, offline-first, deterministic, with auditable runs. Two build targets exist:

- `backend/` — Go. The CLI (`cmd/aconiq`), a local HTTP API, the compute engine, the standards modules and the reporting/export stack. This is the primary artifact.
- `frontend/` — React 19 + TypeScript + Vite + Bun + shadcn/ui + MapLibre. It is built, linted, type-checked and tested by `just fe-ci` and by `.github/workflows/frontend-ci.yml`. It talks to the local API, and can alternatively run against a WebAssembly kernel built from `backend/cmd/wasm`.

## Commands

All common tasks are orchestrated via [`just`](https://github.com/casey/just) from the repo root. Run `just` to list every recipe; the table below covers the ones you will normally need.

### Go

| Command                | What it does                                                     |
| ---------------------- | ---------------------------------------------------------------- |
| `just build`           | Build the CLI → `bin/aconiq`                                     |
| `just test`            | Run all Go tests                                                 |
| `just test-race`       | Run tests with the race detector                                 |
| `just test-coverage`   | Run tests with a coverage report (`backend/coverage.html`)       |
| `just coverage-report` | Render the coverage Markdown CI publishes                        |
| `just coverage-check`  | Check the coverage floor (`COVERAGE_FLOOR` to enforce)           |
| `just update-golden`   | Update golden snapshots (`UPDATE_GOLDEN=1 go test ./...`)        |
| `just lint`            | Run golangci-lint v2 over `backend/`                             |
| `just lint-fix`        | Same, with `--fix`                                               |
| `just vet`             | `go vet` (compiler-adjacent checks golangci-lint does not cover) |
| `just check-tidy`      | Verify `go.mod`/`go.sum` are tidy                                |
| `just govulncheck`     | Scan dependencies and the stdlib for known CVEs                  |
| `just license-check`   | Fail on restricted/forbidden/unknown dependency licenses         |
| `just license-report`  | CSV report of all dependency licenses                            |
| `just wasm-build`      | Build the WASM kernel → `frontend/public/aconiq.wasm`            |

### Formatting

| Command                | What it does                                                        |
| ---------------------- | ------------------------------------------------------------------- |
| `just fmt`             | Format everything via treefmt (Go, shell, markdown, YAML, JSON, TS) |
| `just check-formatted` | The CI formatting gate — read-only                                  |
| `just check-tools`     | Verify the installed toolchain against `tools.versions`             |
| `just install-tools`   | Install the toolchain at exactly those versions                     |

> `just check-formatted` does not touch your tree: it mirrors the files a commit could contain into
> a temporary directory, formats the copy, and diffs. `treefmt --fail-on-change` — which formats in
> place and _then_ reports — is deliberately not used as a check.
>
> Both recipes refuse to run when the formatters do not match `tools.versions`, because treefmt runs
> whatever is on `PATH`: a different prettier or gofumpt rewrites files the pinned one considers
> correct, and a missing one is skipped without a word. `just install-tools` installs the pins.

### Frontend

`just fe-install`, `fe-dev`, `fe-build`, `fe-typecheck`, `fe-lint`, `fe-lint-fix`, `fe-test`, `fe-test-coverage`, `fe-coverage-report`, `fe-e2e`, `fe-bundle-check`, and `fe-ci` (typecheck + lint + test + build + bundle-check). `just fe-build-wasm` builds the frontend in WASM-only mode.

### Aggregates

| Command      | What it does                                                                    |
| ------------ | ------------------------------------------------------------------------------- |
| `just go-ci` | The full Go gate; mirrors `.github/workflows/go-ci.yml` recipe for recipe       |
| `just ci`    | `go-ci` + `fe-ci`                                                               |
| `just dev`   | Build, then run `aconiq serve` (:8080) and the Vite dev server (:5173) together |
| `just fix`   | `lint-fix` then `fmt`                                                           |
| `just clean` | Remove build artifacts                                                          |

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
backend/          Go application: CLI, local API, engine, standards, reporting
frontend/         React/TypeScript UI (Vite + Bun), plus the WASM kernel client
docs/             Specs, policies, ADRs, conformance declarations, research notes
interoperability/ Reference material per standard (GTA, ISO 9613-2, RLS-19, Schall 03, TA Lärm)
examples/         Reserved for license-safe sample projects — currently empty
scripts/          Shell behind the justfile: toolchain install/check, the read-only format gate
justfile          Task runner (just) — primary entry point for dev commands
.golangci.yml     golangci-lint v2 config (defaults plus tuned disables and exclusions)
treefmt.toml      Multi-language formatter config (gofumpt, gci, shfmt, prettier)
tools.versions    Pinned tool versions — read by the justfile and by every CI workflow
PLAN.md           Roadmap and the single status source
```

### Binaries (`backend/cmd/`)

| Binary    | Purpose                                                                             |
| --------- | ----------------------------------------------------------------------------------- |
| `aconiq/` | The CLI                                                                             |
| `wasm/`   | `js/wasm` entry point exposing the compute kernel to the browser as `window.aconiq` |

`window.aconiq` exposes `rls19Road`, `transform`, `standards`, `loadTerrain`, `clearTerrain`,
`defaultConfig`, `health` and `projectStatus`. Three of those carry contracts worth knowing before
writing browser-mode code:

- **`transform`** is the frontend's only map projection. It takes a batch of flat, interleaved
  coordinates and a `target_crs` of `"auto"`, and makes the CRS decision `aconiq run` makes — so
  browser mode and the CLI resolve the same UTM zone, and refuse the same site in the same words.
  The frontend must not grow a second transverse-Mercator implementation.
- **`standards`** publishes what the kernel can actually run, in the same JSON shape
  `GET /api/v1/standards` answers with. Both go through `internal/standards/descriptorjson`, so
  the evidence tier, the parameter defaults and the enums are declared once, by the Go module.
- **`loadTerrain(data, crs)`** takes the DTM's own CRS as a second argument, and requires it. The
  GeoTIFF loader in `internal/geo/terrain` reads the tie point (33922) and the pixel scale (33550)
  and no GeoKeyDirectory, so the raster does not say what it is in; and the browser projects the
  model itself, through `transform`, so a compute request reaches the kernel as bare numbers that
  name no CRS either. The caller is the only one who knows. A request computing over a loaded
  terrain must therefore also carry `projection` (`project_crs`, `compute_crs`, `applied`) — the
  same pair `cli.computeProjection` records — and is refused without it rather than answered from
  a grid queried in the wrong CRS. `terrain.InComputeCRS` is the wrapper both targets use.

The logic behind all three lives in `internal/wasmkernel`, which carries **no build tag** so that a
host `go test` can reach it; `cmd/wasm/main.go` is `//go:build js && wasm` and has no tests.

### Go Package Structure (`backend/internal/`)

| Package                     | Responsibility                                                                                                     |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `api/httpv1/`               | Local HTTP API: handler/mux, CORS, and the hand-built OpenAPI document                                             |
| `app/cli/`                  | Cobra commands and their run pipeline (input, extraction per standard, persistence, output)                        |
| `app/config/`               | Config resolution (project path, cache dir, log level, JSON output)                                                |
| `app/logging/`              | Structured logging baseline and per-command run logging                                                            |
| `assessment/bimschv16/`     | 16. BImSchV threshold tables and receiver assessment; emitted into export bundles                                  |
| `assessment/talaerm/`       | TA Lärm Beurteilungspegel, area categories, periods and export envelope (library only; no CLI wiring yet)          |
| `buildinfo/`                | Release identity (version, commit, build date), stamped at link time, used by `--version` and provenance           |
| `domain/errors/`            | Error taxonomy (user input vs. internal), drives CLI exit codes                                                    |
| `domain/project/`           | Core entities: `Project`, `Scenario`, `Run`, `StandardRef`, `ArtifactRef`, provenance                              |
| `engine/`                   | Compute engine: chunked worker pool, progress events, cancellation, run/shared disk cache, deterministic reduction |
| `geo/`                      | CRS model and transforms, geometry primitives, spatial index, receiver sets                                        |
| `geo/modelgeojson/`         | GeoJSON model normalization and validation                                                                         |
| `geo/terrain/`              | Terrain model interface plus GeoTIFF DTM loading and bilinear elevation queries                                    |
| `io/citygmlimport/`         | CityGML building import                                                                                            |
| `io/csvimport/`             | CSV attribute/traffic tables merged into model features                                                            |
| `io/fgbimport/`             | FlatGeobuf (`.fgb`) import                                                                                         |
| `io/gpkgimport/`            | GeoPackage (`.gpkg`) import, including WKB decoding                                                                |
| `io/osmimport/`             | OpenStreetMap import via the Overpass API                                                                          |
| `io/projectfs/`             | Project folder store (JSON manifest in `.noise/project.json`)                                                      |
| `io/soundplanimport/`       | SoundPLAN project bundle import (geometry, terrain, rail ops, grids, absolute results)                             |
| `qa/acceptance/`            | Acceptance fixture catalog and hook registry, with per-standard runners (`rls19_test20/`, `schall03/`)             |
| `qa/golden/`                | Golden snapshot helper                                                                                             |
| `report/export/`            | Export formats: GeoTIFF, COG, GeoPackage, contour GeoJSON/GPKG, and the format matrix                              |
| `report/reporting/`         | Offline report generation: `report-context.json`, `report.md`, `report.html`, `report.typ`, optional PDF           |
| `report/results/`           | Result containers: raster API + binary/JSON persistence, receiver table API + CSV/JSON                             |
| `standards/`                | The registry that assembles the standards modules the CLI can run                                                  |
| `standards/descriptorjson/` | The one JSON encoding of a standards descriptor, shared by the HTTP API and the WASM kernel                        |
| `standards/framework/`      | Standard descriptors, parameter schemas, version/profile resolution, registry type                                 |
| `standards/<module>/`       | Individual standards modules — see the evidence-tier warning below                                                 |
| `wasmkernel/`               | What the WASM kernel can run and what its JS entry points do, build-tag-free so host tests can reach it            |

### CLI Surface

`backend/internal/app/cli/root.go` registers exactly eleven commands:

| Command             | Purpose                                                                                        |
| ------------------- | ---------------------------------------------------------------------------------------------- |
| `aconiq init`       | Create a project (`.noise/`)                                                                   |
| `aconiq import`     | Import GeoJSON, GeoPackage, FlatGeobuf, CityGML, SoundPLAN, OSM/Overpass, CSV, GeoTIFF terrain |
| `aconiq compare`    | Compare a run against imported SoundPLAN receiver results with a dB tolerance                  |
| `aconiq validate`   | Validate the normalized model and emit a validation report                                     |
| `aconiq run`        | Run one scenario against one standard/version/profile                                          |
| `aconiq delete-run` | Remove a run, its artifact refs and `.noise/runs/<id>/`; export bundles are kept               |
| `aconiq status`     | Report project, scenario and run state                                                         |
| `aconiq export`     | Export a run bundle, generate reports, and emit GIS formats (`--format`)                       |
| `aconiq serve`      | Start the local HTTP API (default `127.0.0.1:8080`)                                            |
| `aconiq openapi`    | Export the OpenAPI contract (default `.noise/api/openapi.v1.json`)                             |
| `aconiq bench`      | Run synthetic benchmark scenarios (runtime, memory, cache IO, numeric drift)                   |

### Local HTTP API (`internal/api/httpv1`)

```
GET  /api/v1/health
GET  /api/v1/project/status
GET  /api/v1/standards
GET  /api/v1/runs                      list runs
POST /api/v1/runs                      start a run
DELETE /api/v1/runs/{id}               delete a run
GET  /api/v1/runs/{id}/log
GET  /api/v1/runs/{id}/contours        contour lines from the run's result raster (`?interval=` dB step, `?crs=`)
GET  /api/v1/artifacts/{id}/content
GET  /api/v1/events                    SSE: heartbeat + project status snapshots
POST /api/v1/import/osm
POST /api/v1/import/terrain
GET  /api/v1/model                     the saved model, optionally reprojected (`?crs=`)
POST /api/v1/model                     replace the project model (the `aconiq import` GeoJSON contract)
POST /api/v1/transform                 project a coordinate batch between two CRS (`target_crs: "auto"` picks the zone `aconiq run` would)
GET  /api/v1/openapi.json
```

Responses use a standardized JSON error envelope (`code`, `message`, `details`, `hint`). Keep `handler.go` and `openapi.go` in sync — the spec is hand-built, not generated from the mux.

### Project Format v1

A project is a folder with `.noise/` containing:

- `.noise/project.json` — manifest (scenarios, runs, artifacts, migrations)
- `.noise/runs/<run-id>/run.log` — run log
- `.noise/runs/<run-id>/provenance.json` — standard ID, version, parameters, input hashes
- `.noise/runs/<run-id>/results/` — receiver tables, rasters, run summary
- `.noise/model/` — import debug exports (normalized GeoJSON, dump, validation report)
- `.noise/artifacts/`, `.noise/logs/` — generated artifacts and logs
- `.noise/exports/` — export bundles
- `.noise/cache/` — engine cache: per-run chunk caches, `shared-chunks/`, `bench/`

See `docs/project-format-v1.md` and `docs/project-migrations.md`.

### GeoJSON Input Schema (v1)

`aconiq import` accepts a `FeatureCollection` whose features carry `kind` = `source`, `building`, `barrier`, `receiver`, or `calc-area`.

- `source` requires `source_type` (`point`|`line`|`area`), and the geometry must match it: `Point`/`MultiPoint`, `LineString`/`MultiLineString`, `Polygon`/`MultiPolygon`.
- `building` requires `height_m > 0` and `Polygon`/`MultiPolygon`.
- `barrier` requires `height_m > 0` and `LineString`/`MultiLineString`.
- `receiver` requires `height_m > 0` and `Point`.
- `calc-area` requires `Polygon`, carries no `height_m`, and may appear at most once. It replaces
  the source extent for the auto receiver grid; `grid_padding_m` still applies to it.

See `docs/geojson-schema-v1.md`.

### Result Containers v1

- **Raster:** custom binary (`float64` little-endian) + JSON metadata sidecar, in `internal/report/results`
- **Receiver table:** CSV + JSON, with ordered indicators and validation

See `docs/result-containers-v1.md`.

### Standards Architecture

Standards are plug-in modules under `internal/standards/`, registered centrally in `internal/standards/registry.go` and described through `internal/standards/framework` (descriptor, parameter schema, version/profile). Each module is independently testable. `internal/assessment/` holds German assessment logic (16. BImSchV, TA Lärm) that consumes computed levels rather than producing them.

**A module's name does not tell you how much its output can be trusted.** The registry currently exposes 13 standard IDs as peers — `dummy-freefield`, `cnossos-road`, `cnossos-rail`, `cnossos-industry`, `cnossos-aircraft`, `bub-road`, `bub-rail`, `bub-industry`, `buf-aircraft`, `beb-exposure`, `iso9613`, `rls19-road`, `schall03` — but they sit at very different evidence tiers:

- `rls19-road`, `schall03`, `iso9613` carry real normative structure, coefficients and tables (with open defects tracked in `PLAN.md`); `talaerm` and `bimschv16` carry real threshold tables.
- `beb-exposure` is **preview**: the aggregation logic is reasonable, but it consumes preview-grade levels.
- The `cnossos-*` modules, `bub-*` and `buf-aircraft` are **scaffolds**: no directive coefficients, invented base levels, no octave bands. `bub-rail`/`bub-industry` are aliases over `cnossos/*` and `buf-aircraft` is a near-copy of `cnossos/aircraft`. Do not describe them as implementations of CNOSSOS-EU or of the German mapping directives.
- `dummy-freefield` is an intentional test fixture and explicitly non-normative.

The tier is not a documentation convention — it is a field the code carries and enforces:

- `framework.StandardDescriptor` has an `EvidenceTier` field (`internal/standards/framework`), with the values `normative`, `preview`, `scaffold` and `test-fixture`. `StandardDescriptor.Validate()` **requires** it, so a module that declares no tier cannot be registered. Adding a standards module means choosing a tier and defending it.
- Scaffold-tier standards require an explicit `--experimental` opt-in on `aconiq run`. Without it the run is refused rather than quietly emitting authoritative-looking dB(A) values.
- The tier travels with the result: it is stamped into `provenance.json` and `run-summary.json`, printed by `aconiq run` and `aconiq status`, carried into generated reports and export bundles, and exposed on `GET /api/v1/standards`. A consumer that never reads the docs still sees it.
- The tier is the machine-readable form of the `compliance_boundary` string each module already returns in its `ProvenanceMetadata` — derive the free text from the tier, do not maintain a second, parallel signal.

`PLAN.md`'s "Standards evidence tiers" table is authoritative and current; read it before writing anything that characterises a module. Published boundaries live in `docs/conformance/`: `*-konformitaetserklaerung.md` for the normative modules, and `cnossos-umfangserklaerung.md` (the `cnossos-*`, `bub-*` and `buf-aircraft` scaffolds) plus `beb-umfangserklaerung.md` (`beb-exposure`) — scope statements, explicitly not conformance declarations. The `docs/phase*-baseline.md` files are historical delivery notes; where they disagree with a conformance or scope document, the document in `docs/conformance/` wins.

**Normative outputs must only come from normative modules** — DSP/FFT-based tools are non-normative post-processing only.

## Key Policies

**Never let a module's name assert more than its evidence supports.** This applies to code, docs, CLI help, and API responses alike.

**Determinism:** Same inputs + standard/profile → identical outputs regardless of worker count. Map iteration must never influence numeric results. Partial results merge in fixed order (no "first finished wins"). See `docs/policies/determinism.md`.

**Data handling:** Licensed third-party material — standards texts, customer project bundles — lives under `interoperability/` and is never tracked. `just check-no-third-party-data` refuses a commit that tracks a path there, but it is a tracked-path check, not a content scanner: a licensed table pasted into a Go file passes it. What may be held, by whom, how a licensed fixture reaches a test run, and what to do if it leaks are in `docs/policies/data-handling.md`.

**Formatting:** Enforced via `just fmt` (treefmt: gofumpt + gci + shfmt + shellcheck + prettier), at the versions `tools.versions` pins. `just check-formatted` is the CI gate and is read-only; see the note above.

**Linting:** `just lint` runs golangci-lint v2 with `default: all` **minus a tuned disable list**, plus path- and text-scoped exclusion rules. It is not "all linters enabled". Every disable and exclusion is justified in `.golangci.yml` itself and in `docs/lint-triage.md` — keep the two in sync. `issues.uniq-by-line` is deliberately off so a finding cannot hide behind another on the same line. Leave the tree with no findings; fix issues before committing rather than adding suppressions.

**Golden tests:** Snapshots live in `testdata/` next to the owning test package, named `<scenario>.golden.<ext>`. Update only intentionally via `just update-golden`; review diffs before committing. See `docs/testing/golden-tests.md`.

**Coverage:** Measured on both targets, reported on every pull request as a job summary and a sticky comment, and held to a floor that is **advisory** — the `go-coverage` and `frontend-coverage` jobs are deliberately not required checks, so a coverage regression never blocks a merge. Backend floors live in `go-ci.yml` (`COVERAGE_FLOOR`), frontend floors in `frontend/vitest.config.ts`. Ratchet them up in `docs/testing/coverage.md`, which carries the ledger and the reasoning; never lower one to make a check green.

**Floating-point:** Keep calculations at `float64`. Apply rounding only at defined output boundaries. Document rounding rules per standards module. Use stable (pairwise/compensated) summation for sensitive reductions.

**Language:** Technical content — commit messages, PR titles and bodies, code comments, documentation — is written in English. German appears only where it is the subject matter: the conformance declarations and scope statements in `docs/conformance/`, and the German-language summary strings the assessment modules emit.

## Module Name

`github.com/aconiq/backend`
