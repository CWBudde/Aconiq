# PLAN.md — Implementation Plan for a “SoundPLAN-like” Environmental Noise System

Status: 6 March 2026

This is a **comprehensive, phased implementation plan** (Go backend + React/TypeScript frontend + GIS/MapLibre). It is intentionally **very granular** (bite-sized checklist tasks) so the system remains runnable and testable throughout.

## Clarifications (explicit decisions)

- **Offline-only is fine for now.** The near-term MVP is CLI-driven and writes artifacts into a project folder. A local API (`serve`) and browser GUI are deferred.
- **Input data:** **GeoJSON only for now.** All other import formats exist as explicit deferred phases.
- **Standards:** **all standards mentioned are required long-term.** Each one has a dedicated phase (even if deferred).

## Guiding principles (from goal.md)

- **Separate core vs. standards:** a generic acoustics/geometry/compute core + standards modules (CNOSSOS, BUB/BUF/BEB, RLS‑19, Schall 03, ISO 9613‑2, …) as plug-ins.
- **Quality assurance is a feature:** deterministic runs, golden tests, and acceptance suites (e.g., RLS‑19 TEST‑20) are first-class.
- **Project-oriented data model:** v1 local (folder + files/SQLite), v2 optional multiuser/server (e.g., PostGIS + object storage).

## Working definitions

- **Project**: folder with manifest + inputs + artifacts.
- **Scenario**: input model + standard selection + parameters.
- **Run**: a concrete calculation of one scenario against one receiver set, with a fixed standard version/profile.
- **Standards module**: implementation of emission/propagation/indicators/tables for a specific standard and version/profile.

---

## Phase 0 — Preflight (repo, constraints, risks)

**Goal:** lock down non-negotiables and remove avoidable unknowns.

- [x] Initialize repository layout (Go + frontend in one repo, clear folder structure)
- [x] Clarify licensing/compliance boundaries (code, data sources, test tasks)
- [x] Define target platforms (Linux/Mac/Windows; CPU arch; future WASM optional)
- [x] Define a minimal “Definition of Done” for each phase
- [x] Start and maintain a risk register (e.g., GDAL/cgo portability, paywalled standards, Wails v3 alpha maturity)
- [x] Decide and document “offline-only for now” constraints (no HTTP server required for MVP)

---

## Phase 1 — Foundations (Go architecture + CLI skeleton)

**Goal:** compile, run, and test; no domain logic “guessed” yet.

### Backend (Go)

- [x] Create Go module
- [x] Create packages: `cmd/`, `internal/app/`, `internal/domain/`, `internal/geo/`, `internal/engine/`, `internal/standards/`, `internal/io/`, `internal/report/`, `internal/qa/`
- [x] Define configuration layer (project path, logging level, cache dir)
- [x] Structured logging baseline (run IDs, timings)
- [x] Error taxonomy (user input errors vs internal errors)

### CLI (Cobra)

- [x] Create `noise` root command
- [x] Add placeholder subcommands: `init`, `import`, `validate`, `run`, `status`, `export`, `bench`
- [x] Common flags/config plumbing (`--project`, `--verbose`, `--json`)

### Tests

- [x] Ensure `go test ./...` works locally

### Research (technical choices)

- [x] Evaluate Go geometry/spatial libs (robustness, performance, license)
- [x] Evaluate CRS/PROJ strategy (pure Go vs cgo/PROJ; accuracy vs portability)

---

## Phase 2 — CI + determinism + golden test harness

**Goal:** every change is reproducible and regression-testable.

- [x] CI pipeline: lint + tests for Go
- [x] Formatting policy (gofmt; optional gofumpt) + enforced in CI
- [x] Determinism policy document
  - [x] Floating-point rules (reduction order, rounding, stable summation when needed)
  - [x] Deterministic parallel reduction strategy
- [x] Golden-test harness
  - [x] `testdata/` conventions
  - [x] snapshot update workflow

---

## Phase 3 — Project format v1 (local) + provenance

**Goal:** a stable project folder that can be versioned and migrated.

- [x] Design project manifest v1 (version, CRS, scenarios, standards, artifacts)
- [x] Domain model: `Project`, `Scenario`, `Run`, `StandardRef`, `ArtifactRef`
- [x] Choose storage strategy v1
  - [ ] Option A: SQLite for metadata + files for geometry/results
  - [x] Option B: JSON-only initially + introduce SQLite later
- [x] Implement `noise init`
- [x] Implement `noise status` (run list, last status, logs)

### Provenance / audit trail

- [x] Each run writes a provenance manifest (standard ID, version/profile, parameters, input file hashes)
- [x] Define project migrations strategy (v1 → later)

---

## Phase 4 — Input/Output v1: GeoJSON-only + validation skeleton

**Goal:** load model data and validate it without running any calculation.

### Import (GeoJSON only)

- [x] Define GeoJSON feature schemas (minimal common set)
  - [x] Sources: point/line/area
  - [x] Buildings/barriers: geometry + minimal attributes (e.g., height)
- [x] Implement `noise import` (GeoJSON)

### Validation

- [x] Implement `noise validate`
  - [x] Required fields
  - [x] Geometry sanity (no NaNs, rings, basic self-intersection checks where possible)
  - [x] CRS plausibility checks

### Export

- [x] Add debug exports (normalized GeoJSON/JSON “model dump”)

---

## Phase 5 — Geo core: CRS, spatial index, receiver sets

**Goal:** solid geometry primitives and receiver management.

- [x] CRS model (project CRS, import CRS, transform pipeline)
- [x] Geometry utilities (point-line distance, point-in-polygon, bboxes)
- [x] Spatial index (R-tree or equivalent) for candidate queries
- [x] Receiver set types
  - [x] Point receivers (list)
  - [x] Grid receivers (bbox + resolution + height)
  - [x] Facade receivers (data model + stub; full implementation deferred)

### Tests

- [x] Geo unit tests (edge cases)
- [x] Fuzz/property tests for geometry primitives

---

## Phase 6 — Result containers v1 (rasters + tables)

**Goal:** persist results so they are inspectable and exportable.

- [x] Define raster container API (indexing, bands, NoData, units)
- [x] Choose v1 persistence format
  - [x] Option A: custom binary + JSON metadata
  - [ ] Option B: GeoTIFF early (only if dependency strategy is acceptable)
- [x] Choose receiver table format (CSV/JSON first; Parquet deferred)
- [x] Implement `noise export` skeleton

### Research

- [x] Evaluate GeoTIFF writing in Go (pure Go vs GDAL via cgo)
- [x] Evaluate contours/isoline generation library (Marching Squares)

---

## Phase 7 — Compute engine skeleton (jobs, parallelism, cancellation)

**Goal:** a generic compute pipeline without committing to a specific standard yet.

- [x] Define run pipeline (Load → Prepare → Chunk → Compute → Reduce → Persist)
- [x] Receiver chunking strategy (tiles/chunks)
- [x] Worker pool
- [x] Deterministic reduction
- [x] Progress model for offline logs (structured events)
- [x] Cancellation (context cancellation) and cleanup rules
- [x] Disk-backed cache v1 (per run/chunk)

### Tests

- [x] Determinism test: 1 worker vs N workers produce identical output hashes
- [x] Cancel test: abort leaves a consistent state

---

## Phase 8 — End-to-end (offline) with a non-normative dummy standard

**Goal:** complete E2E pipeline with minimal math (explicitly non-normative).

- [x] Implement standards module `dummy/freefield`
  - [x] Simple geometric distance attenuation (clearly marked as non-normative)
- [x] Implement `noise run --standard dummy-freefield`
- [x] Persist results (raster + receiver table)
- [x] Add a golden project (1–2 sources, small grid) with expected values

---

## Phase 9 — Standards framework (plugin API + versioning profiles)

**Goal:** make standards truly modular before implementing multiple complex ones.

- [x] Define standards plugin interface
  - [x] Standard ID, version/profile, supported source types, supported indicators
  - [x] Parameter schema definition for runs
- [x] Implement standard version profiles (e.g., CNOSSOS profiles)
- [x] Enforce run provenance (standard + profile + parameters are always recorded)

---

## Phase 10 — CNOSSOS-EU: Road (required, implement earlier)

**Goal:** first real normative module.

- [x] Define CNOSSOS Road source schema (speed, surface, traffic, …)
- [x] Implement emission model (table/piecewise logic)
- [x] Implement propagation chain needed for Road use-case
- [x] Implement indicators
  - [x] Lday, Levening, Lnight
  - [x] Lden aggregation
  - [x] Lnight output
- [x] Export: Lden/Lnight rasters + receiver point tables

### QA / Research

- [ ] Collect public validation/verification cases for CNOSSOS Road (license-safe)
- [ ] Document rounding/tolerance rules used by the implementation

---

## Phase 11 — CNOSSOS-EU: Rail (required, deferred)

- [x] Define CNOSSOS Rail source schema
- [x] Implement rail emission path
- [x] Implement required propagation adjustments
- [x] Add golden projects + regression tests

---

## Phase 12 — CNOSSOS-EU: Industry (required, deferred)

- [ ] Define CNOSSOS Industry source schema
- [ ] Implement industry emission path
- [ ] Implement required propagation adjustments
- [ ] Add golden projects + regression tests

---

## Phase 13 — CNOSSOS-EU: Aircraft (required long-term, deferred)

- [ ] Define aircraft data model and APIs
- [ ] Implement as a full module (not just a stub)
- [ ] Add golden projects + regression tests

---

## Phase 14 — Germany mapping track: BUB (required, deferred)

**Goal:** “mapping context” standard, separate from planning/approval standards.

- [ ] Add explicit “context switch” in the domain model (Mapping vs Planning)
- [ ] Implement `bub/` module structure (road/rail/industry within BUB logic)
- [ ] Add BUB-specific run parameter schemas
- [ ] Add acceptance suite integration hooks in `internal/qa/`

### Research

- [ ] Clarify availability and formats for BUB-related datasets (e.g., BUB-D) and import rights

---

## Phase 15 — Germany mapping track: BUF (required, deferred)

- [ ] Implement `buf/` module (once scope is clarified)
- [ ] Add golden projects + acceptance tests where possible

---

## Phase 16 — Germany mapping track: BEB (required, deferred)

- [ ] Implement `beb/` (exposure counts / affected persons) as a separate pipeline stage
- [ ] Add report/table outputs for BEB
- [ ] Add known-totals tests

---

## Phase 17 — Germany planning track: RLS‑19 (required, deferred)

- [ ] Implement `rls19/` module
  - [ ] Emission
  - [ ] Propagation
  - [ ] Result indicators/outputs for the workflow
- [ ] Integrate official acceptance suite where legally possible
  - [ ] RLS‑19 TEST‑20 in CI
  - [ ] Per-test tolerance rules
- [ ] Generate a conformance report artifact (test suite status + versions)

### Research

- [ ] Clarify how TEST‑20 data is obtained, stored, and legally redistributed (if at all)

---

## Phase 18 — Germany planning track: Schall 03 (required, deferred)

- [ ] Implement `schall03/` module
  - [ ] Octave band data model and energetic summation
  - [ ] Rounding and band handling rules (documented)
- [ ] Add golden projects + acceptance tests where possible

---

## Phase 19 — Industry (international): ISO 9613‑2 (required, deferred)

- [ ] Implement `iso9613/` module (engineering method)
- [ ] Add validation projects (license-safe)
- [ ] Ensure module separation so normative outputs stay normative

---

## Phase 20 — Reporting v1 (offline)

**Goal:** reproducible reports from offline artifacts.

- [ ] Report templating v1 (Markdown/HTML)
- [ ] Required report sections
  - [ ] Input overview
  - [ ] Standard ID + version/profile + parameters
  - [ ] Maps/images if available
  - [ ] Tables (receiver stats)
  - [ ] QA status (which suites passed)
- [ ] PDF export (optional) or HTML-only MVP

### Research

- [ ] Evaluate HTML→PDF pipeline (headless Chromium) vs Go PDF libraries

---

## Phase 21 — QA hardening (test catalogs, fuzzing, drift tracking)

**Goal:** make correctness and reproducibility measurable.

- [ ] Expand `internal/qa/`
  - [ ] Loaders for standard test tasks
  - [ ] Result comparison with tolerances + outlier reports
  - [ ] Snapshot exporter for debugging
- [ ] Expand fuzz/property tests
  - [ ] Geometry robustness
  - [ ] Numeric monotonicity properties (where applicable)
- [ ] Numeric drift tracking (benchmarks + comparisons across commits)
- [ ] “Repro bundle” export: run + inputs + standard/profile in one package

---

## Phase 22 — Performance & scaling (city-scale)

**Goal:** large receiver grids and many sources perform well.

- [ ] Optimize tiled compute pipeline
  - [ ] Spatial index tuning
  - [ ] Candidate pruning
  - [ ] Cache keys and reuse
- [ ] Robust disk-backed cache + cleanup strategies
- [ ] Implement `noise bench`
  - [ ] Standard benchmark scenarios
  - [ ] Output: runtime, memory, IO, numeric drift

### Optional (advanced, non-normative)

- [ ] Use algo-fft/algo-dsp for post-processing pipelines (kept separate from normative outputs)

---

## Phase 23 — Deferred: local API + GUI (when “offline-only” changes)

**Goal:** enable a browser GUI to drive runs and visualize results.

- [ ] Introduce `noise serve` (local-only)
  - [ ] Define HTTP API v1 endpoints + DTOs
  - [ ] Progress streaming (SSE/WebSocket)
  - [ ] Standardized error format
- [ ] Frontend foundation
  - [ ] Project explorer (scenarios, runs, artifacts)
  - [ ] MapLibre layers for model and results
  - [ ] Editors and validation overlays
  - [ ] Run dialog + run monitor
- [ ] E2E tests (Playwright): import → validate → run → visualize → export

### Research

- [ ] OpenAPI vs gRPC/Connect
- [ ] TypeScript DTO generation

---

## Phase 24 — Deferred: input formats beyond GeoJSON

**Goal:** add professional importers without blocking early delivery.

- [ ] GeoPackage importer (deferred)
- [ ] FlatGeobuf importer (deferred)
- [ ] CSV traffic/time tables importer (deferred)
- [ ] Terrain/DTM import (deferred)
- [ ] Building footprints/import pipelines beyond GeoJSON (deferred)

For each importer:

- [ ] Define schema + units
- [ ] Implement import + validation
- [ ] Add roundtrip tests

---

## Phase 25 — Deferred: tiling/PMTiles

**Goal:** fast map rendering and efficient distribution when GUI exists.

- [ ] Evaluate vector tiles for model/results
- [ ] Evaluate PMTiles end-to-end pipeline
- [ ] Define storage/size budgets

---

## Phase 26 — Optional: desktop packaging (Wails)

**Goal:** ship a single-binary desktop option.

- [ ] Make the API runnable in-proc (no port needed)
- [ ] Embed frontend assets into Go binary
- [ ] Define build targets (`web` vs `wails`)
- [ ] Smoke tests for desktop build

### Risk / Research

- [ ] Re-check Wails v3 maturity (alpha risk) and define fallback options

---

## Phase 27 — Optional: project format v2 (multiuser/server)

**Goal:** PostGIS + object storage, versioned scenarios/runs.

- [ ] Map data model to PostGIS (geometries, indexes)
- [ ] Store artifacts in object storage (rasters/tiles/reports)
- [ ] Minimal auth/users (only if required)
- [ ] Migration tool: v1 project → v2

---

## Phase 28 — Release engineering + documentation + conformance artifacts

**Goal:** usable releases with traceable QA.

- [ ] Versioning/changelog process
- [ ] Build release binaries (CLI; desktop optional)
- [ ] Documentation
  - [ ] Getting started
  - [ ] Project format spec
  - [ ] Standards modules overview + status
  - [ ] QA/acceptance process and tolerances
- [ ] Provide example projects (synthetic, license-safe)

---

# Missing information & research backlog (actionable)

This list is explicitly focused on “what is missing” and turns it into concrete tasks.

## Standards & test data

- [ ] CNOSSOS Road/Rail/Industry/Aircraft: collect license-safe validation cases and define tolerances
- [ ] BUB/BUF/BEB: obtain the current documents/annexes and define the exact input requirements per module
- [ ] RLS‑19 TEST‑20: clarify sourcing, storage format, legal redistribution, and CI automation
- [ ] Schall 03: clarify rounding rules, octave band handling, and acquire license-safe verification cases
- [ ] ISO 9613‑2: identify public example cases (or create synthetic ones) to validate implementation

## GIS / formats

- [ ] CRS/PROJ decision (accuracy vs portability)
- [ ] GeoTIFF vs custom raster: portability and dependency strategy
- [ ] (Deferred) GPKG/FlatGeobuf/CSV import: choose libraries and schemas

## Determinism & tolerances

- [ ] Standardize numeric tolerances (per standard/test suite)
- [ ] Define stable summation strategy and document where it applies

## UX/workflow (deferred while offline-only)

- [ ] When GUI starts: define minimal workflow (import → validate → run → visualize → export)
- [ ] Define “must-have” exports (GeoTIFF/CSV/PNG/report) and which are deferred

---

# Suggested near-term MVP path (offline-only, GeoJSON-only)

- [ ] Phases 1–3 (foundations + CI + project format)
- [ ] Phases 4–7 (GeoJSON import/validate + geo core + result containers + engine)
- [x] Phase 8 (dummy E2E)
- [x] Phase 9 (standards framework)
- [x] Phase 10 (CNOSSOS Road)

Then: Reporting (Phase 20) and/or start the deferred standards phases.
