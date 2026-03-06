# PLAN.md — Implementation Plan for an Environmental Noise System

Status: 6 March 2026

This is a **comprehensive, phased implementation plan** (Go backend + React/TypeScript frontend + GIS/MapLibre). It is intentionally **very granular** (bite-sized checklist tasks) so the system remains runnable and testable throughout.

## Clarifications (explicit decisions)

- **Offline-only is fine for now.** The near-term MVP is CLI-driven and writes artifacts into a project folder. A local API (`serve`) and browser GUI are deferred.
- **Input data:** **GeoJSON only for now.** All other import formats exist as explicit deferred phases.
- **Standards:** **all standards mentioned are required long-term.** Each one has a dedicated phase (even if deferred).
- **Frontend stack (when GUI starts):** **React + TypeScript + Vite + Bun + shadcn/ui**.

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
- [x] Add per-feature CNOSSOS Road attribute extraction from imported GeoJSON instead of run-level defaults only
- [ ] Expand Road baseline toward standards-faithful normative coverage beyond the current preview approximation

---

## Phase 11 — CNOSSOS-EU: Rail (required, deferred)

- [x] Define CNOSSOS Rail source schema
- [x] Implement rail emission path
- [x] Implement required propagation adjustments
- [x] Add golden projects + regression tests

### Remaining limitations / follow-up

- [ ] Add per-feature CNOSSOS Rail attribute extraction from imported GeoJSON instead of run-level defaults only
- [ ] Expand Rail baseline toward standards-faithful normative coverage beyond the current preview approximation
- [ ] Document rounding/tolerance rules used by the implementation
- [ ] Collect license-safe public validation/verification scenarios for CNOSSOS Rail

---

## Phase 12 — CNOSSOS-EU: Industry (required, deferred)

- [x] Define CNOSSOS Industry source schema
  - [x] Add `cnossos-industry` module under `backend/internal/standards/cnossos/industry`
  - [x] Define typed `IndustrySource` payload for `point` and `area` sources
  - [x] Add source attributes: source height, sound power, tonality, impulsivity
  - [x] Add per-period operating factors for `day`, `evening`, `night`
  - [x] Register version/profile metadata, supported source types, indicators, and run parameter schema in the standards framework
- [x] Implement industry emission path
  - [x] Compute period emissions from sound power plus tonality / impulsivity corrections
  - [x] Apply operating-factor weighting per period
  - [x] Add area-source scaling from plan area
  - [x] Add indicator conversion for `Lday`, `Levening`, `Lnight`, `Lden`
- [x] Implement required propagation adjustments
  - [x] Add deterministic point/area source to receiver distance coupling
  - [x] Implement attenuation chain: geometric divergence + air absorption + ground attenuation + screening term
  - [x] Add baseline industry adjustments: facade reflection term and area-source near-field boost
  - [x] Wire `noise run --standard cnossos-industry` through model extraction, receiver-grid generation, compute, export, and provenance
- [x] Add golden projects + regression tests
  - [x] Add module-level unit tests for schema validation, emission, propagation, indicator aggregation, and export
  - [x] Add golden scenario fixture + snapshot for deterministic regression coverage
  - [x] Add CLI end-to-end test covering `cnossos-industry` run output production
  - [x] Add baseline phase documentation in `docs/phase12-cnossos-industry-baseline.md`

### Remaining limitations / follow-up

- [x] Add per-feature CNOSSOS Industry attribute extraction from imported GeoJSON instead of run-level defaults only
- [ ] Expand Industry baseline toward standards-faithful normative coverage beyond the current preview approximation
- [ ] Collect license-safe public validation/verification scenarios for CNOSSOS Industry
- [ ] Document rounding/tolerance rules used by the implementation

---

## Phase 13 — CNOSSOS-EU: Aircraft (required long-term, deferred)

- [x] Define CNOSSOS Aircraft scope + compliance boundary
  - [x] Deliver first baseline as an airport-vicinity profile, not national strategic mapping
  - [x] Scope v1 to movement-based source modeling, period aggregation, receiver indicators, raster/table export
  - [x] Document baseline-preview compliance boundary in `docs/phase13-cnossos-aircraft-baseline.md`
- [x] Define aircraft data model and APIs
  - [x] Add `cnossos-aircraft` module under `backend/internal/standards/cnossos/aircraft`
  - [x] Define typed aircraft source schema with runway/airport reference, operation type, aircraft class, movement periods, and 3D flight-track geometry
  - [x] Add supporting airport/runway identifiers needed by the compute path
  - [x] Add standards-framework descriptor with version/profile metadata, supported source types, indicators, and run parameter schema
  - [x] Add CLI/model extraction path from normalized GeoJSON + run parameters into typed aircraft sources
- [x] Implement aircraft emission path
  - [x] Implement deterministic movement-based emission calculation for aircraft operations
  - [x] Support period splits for `day`, `evening`, `night`
  - [x] Handle baseline operation distinctions for departure/arrival, aircraft class, and engine state
  - [x] Convert period levels to `Lday`, `Levening`, `Lnight`, `Lden`
- [x] Implement aircraft propagation path
  - [x] Add source-to-receiver coupling for segmented 3D aircraft trajectories
  - [x] Implement baseline attenuation chain: geometric divergence + air absorption + ground attenuation
  - [x] Add aircraft-specific adjustments for climb, approach, and bank-angle directivity
  - [x] Wire `noise run --standard cnossos-aircraft` through extraction, receiver generation, compute, export, and provenance
- [x] Add result export + regression coverage
  - [x] Export receiver tables (`JSON`, `CSV`) and rasters (`Lden`, `Lnight`)
  - [x] Add module-level unit tests for schema validation, emission, propagation, indicators, and export
  - [x] Add golden scenario fixtures + deterministic snapshots
  - [x] Add CLI end-to-end tests for aircraft runs

### Remaining limitations / follow-up

- [x] Add per-feature CNOSSOS Aircraft attribute extraction from imported GeoJSON instead of run-level defaults only
- [ ] Expand Aircraft source modeling beyond the airport-vicinity baseline to richer airport/runway/trajectory inputs
- [ ] Expand Aircraft baseline toward standards-faithful normative coverage beyond the current preview approximation
- [ ] Collect license-safe public validation/verification scenarios for CNOSSOS Aircraft
- [ ] Document rounding/tolerance rules used by the implementation

### Research

- [ ] Identify license-safe public aircraft/airport validation scenarios
- [ ] Clarify minimum airport/runway/trajectory input set needed for a useful first baseline
- [ ] Document rounding/tolerance rules used by the implementation

---

## Phase 14 — Germany mapping track: BUB (required, deferred)

**Goal:** “mapping context” standard, separate from planning/approval standards.

- [x] Define mapping-track scope + context model
  - [x] Add explicit “context switch” in the domain model (Mapping vs Planning)
  - [x] Define how mapping-track runs relate to existing CNOSSOS / planning-track runs in provenance and UX
  - [x] Define compliance boundary for BUB implementation vs external normative data/text
- [x] Define BUB module structure and data model
  - [x] Add first BUB submodule under `backend/internal/standards/bub/road`
  - [ ] Split module structure into BUB road / rail / industry submodules where the logic diverges
  - [x] Define typed source schema, indicators, and mapping-specific metadata for the first BUB road submodule
  - [x] Add standards-framework descriptor with mapping context metadata and supported source types
- [x] Add BUB run parameter and import model support
  - [x] Add BUB road-specific run parameter schema
  - [x] Define normalized GeoJSON + run parameter mapping for the first BUB road submodule
  - [ ] Identify which inputs can be represented in generic project data vs which require BUB-only parameters or artifacts
- [x] Implement baseline BUB compute and export flow
  - [x] Implement deterministic compute flow for the first BUB submodule brought online (`bub-road`)
  - [x] Wire `noise run` integration, provenance, result artifacts, and exports
  - [x] Ensure mapping-context outputs stay separated from planning-context outputs via explicit standard context metadata
- [x] Add verification and acceptance coverage
  - [x] Add module-level unit tests and golden scenario for `bub-road`
  - [x] Add acceptance suite integration hooks in `internal/qa/`
  - [x] Add phase documentation and implementation notes

### Research

- [ ] Clarify availability and formats for BUB-related datasets (e.g., BUB-D) and import rights
- [x] Determine minimum shippable BUB sub-scope for first implementation: road only baseline first
- [ ] Document required indicators, rounding rules, and validation tolerances for BUB outputs

### Remaining limitations / follow-up

- [ ] Add BUB rail baseline submodule
- [ ] Add BUB industry baseline submodule
- [x] Add per-feature BUB attribute extraction from imported GeoJSON instead of run-level defaults only
- [ ] Expand BUB road baseline toward standards-faithful normative coverage beyond the current preview approximation

---

## Phase 15 — Germany mapping track: BUF (required, deferred)

- [x] Define BUF scope + compliance boundary
  - [x] Clarify BUF purpose within the mapping track and its relationship to BUB/BEB outputs
  - [x] Treat BUF as a standalone airport-noise mapping module, not a downstream BUB post-processing stage
  - [x] Define baseline-preview compliance boundary for BUF implementation vs external normative data/text in `docs/phase15-buf-aircraft-baseline.md`
- [x] Define BUF data model and APIs
  - [x] Add first `buf/aircraft` module under `backend/internal/standards/`
  - [x] Define typed input model for line-based flight tracks with airport/runway refs, operation type, aircraft class, and movement periods
  - [x] Add standards-framework descriptor with mapping context metadata, supported source types, indicators, and run parameter schema
- [x] Implement BUF compute and export path
  - [x] Implement deterministic compute flow for the first BUF scope brought online (`buf-aircraft`)
  - [x] Wire CLI/run integration, provenance, and artifacts
  - [x] Add report/table exports needed by the baseline (`JSON`, `CSV`, raster sidecar/bin)
- [x] Add verification coverage
  - [x] Add module-level tests and golden scenario for `buf-aircraft`
  - [x] Add acceptance hook entry for `buf-aircraft`
  - [x] Add implementation notes and compliance notes

### Remaining limitations / follow-up

- [x] Add per-feature BUF aircraft attribute extraction from imported GeoJSON instead of run-level defaults only
- [ ] Expand BUF baseline toward standards-faithful airport/runway/trajectory coverage beyond the current preview approximation
- [ ] Collect license-safe public validation/verification scenarios for BUF aircraft
- [ ] Document rounding/tolerance rules used by the implementation

### Research

- [x] Clarify the exact module shape enough for first implementation: BUF is treated as a standalone aircraft mapping module
- [ ] Clarify the exact normative scope and input/output requirements for BUF beyond the current baseline
- [ ] Identify license-safe validation scenarios or reference totals for BUF
- [x] Decide whether BUF should be modeled as a standard module or as a post-processing stage

---

## Phase 16 — Germany mapping track: BEB (required, deferred)

- [x] Define BEB scope + pipeline placement
  - [x] Confirm `beb/` as an exposure / affected-persons stage downstream of mapping results
  - [x] Define first upstream contract as `bub-road` levels consumed within the BEB baseline
  - [x] Define baseline-preview compliance boundary for BEB implementation vs external normative data/text in `docs/phase16-beb-exposure-baseline.md`
- [x] Define BEB data model and required inputs
  - [x] Define typed BEB inputs for building footprints, building height, building usage, and occupancy-derived aggregation units
  - [x] Define required upstream contract for the first baseline (`bub-road` only)
  - [x] Define output schemas for building-level exposure records plus totals summaries
- [x] Implement BEB pipeline stage
  - [x] Implement deterministic exposure aggregation logic
  - [x] Add report/table outputs for BEB
  - [x] Wire BEB into run provenance and artifact persistence
- [x] Add verification coverage
  - [x] Add known-totals tests
  - [x] Add golden scenarios and edge-case tests for aggregation logic
  - [x] Add implementation notes and assumptions

### Remaining limitations / follow-up

- [x] Add per-feature BEB occupancy / dwelling / usage extraction from imported GeoJSON instead of run-level defaults only
- [ ] Expand BEB upstream contracts beyond `bub-road` to `buf-aircraft` and other mapping modules
- [ ] Add exposure-band aggregation/output in `5 dB` bands for `Lden` and `Lnight`
- [ ] Collect license-safe public validation/verification totals for BEB
- [ ] Document rounding/tolerance rules and aggregation conventions used by the implementation
- [ ] Expand the BEB baseline toward standards-faithful normative exposure aggregation beyond the current preview approximation

### Research

- [ ] Clarify required population/building datasets and import rights
- [ ] Align BEB export/report schemas with EEA-style `Lden` / `Lnight` banded exposure summaries
- [ ] Identify license-safe reference totals or validation scenarios for BEB
- [ ] Document rounding/tolerance rules and aggregation conventions for BEB outputs

---

## Cross-phase execution sequence — Phases 10 to 16

This sequence orders the remaining work by implementation dependency and product value, not by
original phase number.

### Milestone A — Real model inputs first

**Goal:** stop depending mainly on run-level defaults and make the shipped baselines usable on
mixed imported models.

- [x] Add per-feature CNOSSOS Road attribute extraction from imported GeoJSON
- [ ] Add per-feature CNOSSOS Rail attribute extraction from imported GeoJSON
- [x] Add per-feature CNOSSOS Industry attribute extraction from imported GeoJSON
- [x] Add per-feature CNOSSOS Aircraft attribute extraction from imported GeoJSON
- [x] Add per-feature BUB Road attribute extraction from imported GeoJSON
- [x] Add per-feature BUF Aircraft attribute extraction from imported GeoJSON
- [x] Add per-feature BEB occupancy / dwelling / usage extraction from imported GeoJSON

### Milestone B — Raise normative fidelity of shipped baselines

**Goal:** close the largest correctness gap in already-online standards modules.

- [x] Expand CNOSSOS Road baseline toward standards-faithful normative coverage beyond the current
      preview approximation
- [x] Expand CNOSSOS Rail baseline toward standards-faithful normative coverage beyond the current
      preview approximation
- [x] Expand CNOSSOS Industry baseline toward standards-faithful normative coverage beyond the
      current preview approximation
- [x] Expand CNOSSOS Aircraft baseline toward standards-faithful airport/runway/trajectory
      coverage beyond the current preview approximation
- [x] Expand BUB Road baseline toward standards-faithful normative coverage beyond the current
      preview approximation
- [x] Expand BUF Aircraft baseline toward standards-faithful normative coverage beyond the current
      preview approximation
- [x] Expand BEB Exposure baseline toward standards-faithful normative aggregation beyond the
      current preview approximation

### Milestone C — Complete missing Germany mapping modules and contracts

**Goal:** finish the largest remaining structural gaps in the Germany mapping track.

- [x] Add BUB Rail baseline submodule
- [x] Add BUB Industry baseline submodule
- [x] Expand BEB upstream contracts beyond `bub-road` to at least `buf-aircraft`

### Milestone D — Verification and acceptance evidence

**Goal:** make Phases 10 to 16 defensible with license-safe evidence.

- [ ] Collect license-safe public validation / verification cases for CNOSSOS Road
- [ ] Collect license-safe public validation / verification cases for CNOSSOS Rail
- [ ] Collect license-safe public validation / verification cases for CNOSSOS Industry
- [ ] Collect license-safe public validation / verification cases for CNOSSOS Aircraft
- [x] Identify license-safe validation scenarios or reference totals for BUB Road
- [x] Identify license-safe validation scenarios or reference totals for BUF Aircraft
- [x] Identify license-safe validation scenarios or reference totals for BEB Exposure
- [x] Convert collected scenarios into deterministic acceptance fixtures under `internal/qa/`

### Milestone E — Rounding, tolerances, and reporting contracts

**Goal:** remove ambiguity at output boundaries.

- [x] Document rounding / tolerance rules for CNOSSOS Road
- [x] Document rounding / tolerance rules for CNOSSOS Rail
- [x] Document rounding / tolerance rules for CNOSSOS Industry
- [x] Document rounding / tolerance rules for CNOSSOS Aircraft
- [x] Document rounding / tolerance rules for BUB Road
- [x] Document rounding / tolerance rules for BUF Aircraft
- [x] Document rounding / tolerance rules for BEB Exposure, including aggregation conventions

### Milestone F — Data rights and import packaging

**Goal:** close the remaining legal and operational blockers around real datasets.

- [ ] Clarify availability and import rights for BUB-related datasets and formats
- [ ] Clarify the exact normative scope and input/output requirements for BUF beyond the current
      baseline
- [ ] Clarify required population/building datasets and import rights for BEB
- [ ] Decide which domain-specific properties belong in normalized project data vs sidecar
      artifacts or standards data packs

### Recommended implementation order

1. Milestone A for Phases 10, 14, 15, and 16 first.
2. Milestone B for Phases 10, 14, 15, and 16 in parallel with targeted acceptance fixtures.
3. Milestone C to add BUB rail and BUB industry, then expand BEB upstream contracts.
4. Milestones D and E across all online modules as soon as each module stabilizes numerically.
5. Milestone A and B cleanup for Phases 11, 12, and 13.
6. Milestone F continuously, but block any dataset-dependent scope expansions on it.

---

## Phase 17 — Germany planning track: RLS‑19 (required, deferred)

- [x] Define scope + compliance boundaries for `rls19/`
  - [x] Planning track (16. BImSchV): compute `Lr` for **day (06–22)** and **night (22–06)**
  - [x] Separate **implementation** from **restricted normative tables/text** (no verbatim standard embedding)
  - [x] Add a short “legal notes” section for the module (sources used, what is stored in-repo vs external)

- [x] Replace the current `rls19-road` _preview_ with a standards-faithful implementation
  - [x] Keep deterministic behavior (segment splitting, stable reductions, canonical ordering)
  - [x] Keep float64 internally; document any required rounding rules at output boundaries

- [x] Data model + inputs (RLS‑19 road)
  - [x] Vehicle groups: Pkw, Lkw1, Lkw2, Krad (and how buses are mapped/handled)
  - [x] Time periods: day/night traffic inputs (support both direct hourly inputs and optional DTV→hourly conversion helpers)
  - [x] Road: surface/cover type, speeds per vehicle group, gradient/sign, junction type + distance, lane/direction layout
  - [ ] Geometry: source line(s) per direction, receivers (height), buildings/barriers for reflection/shielding scenarios

- [x] Emission (align with TEST‑20 “E\*” coverage)
  - [x] E1: Base (speed-dependent) “Grundwert” per vehicle group
  - [x] E2: Straßendeckschicht correction (including “not provided combination” warnings)
  - [x] E3: Längsneigung correction (vehicle-group dependent; sign matters)
  - [x] E4: Knotenpunkt correction (signalized / roundabout / other; distance-dependent)
  - [x] E5: Mehrfachreflexionszuschlag / street-canyon surcharge inputs (building height + canyon width)
  - [x] E6: Per-vehicle sound power levels with additive corrections
  - [x] E7/EG: Längenbezogener Schallleistungspegel per lane/direction and per period (day/night)
  - [x] Make all normative coefficients/tables data-driven (loadable from an external “standards data pack”)

- [x] Propagation (align with TEST‑20 “I*” + “K*” coverage)
  - [x] Use the Teilstückverfahren-style approach with deterministic segment/sub-segment splitting
  - [x] Free-field case: geometric divergence + air absorption + ground/meteorology
  - [x] Shielding: wall/berm diffraction handling consistent with the test suite scenarios
  - [x] Topography: road in cut (Tieflage), embankment (Hochlage), rising/descending roads, “wegführende” roads
  - [x] Reflections: implement up to two reflections and reflection-loss handling
  - [x] Building/courtyard scenarios: house fronts parallel to road, perpendicular buildings, “Hinterhof”
  - [x] Reflection conditions test task (K5): ensure the specific reflection edge-cases pass

- [x] Indicators / outputs
  - [x] Export `LrDay` and `LrNight` to rasters + receiver tables (consistent naming + metadata)
  - [x] Add provenance fields for: RLS‑19 version/profile, data-pack version, and key parameters
  - [x] Document rounding + reporting precision (keep distinct from internal computation)

- [x] QA / acceptance integration
  - [x] Add a dedicated `internal/qa/acceptance/rls19_test20` runner with stable, per-task pass/fail output
  - [x] Support two modes:
    - [x] “Local suite” mode: run against locally downloaded TEST‑20 PDFs / extracted data (not committed)
    - [x] “CI-safe” mode: run only against license-safe derived fixtures (or skip with explicit reason)
  - [x] Per-task tolerance rules (match TEST‑20 expectations: emission strictness, immission reference vs check settings)
  - [x] Generate a conformance report artifact (suite version(s), task list, pass/fail, tolerances used)

### Research

- [ ] Clarify how TEST‑20 data is obtained, stored, and legally redistributed (if at all)
- [ ] Track public sources and versions:
  - [ ] BASt download page for TEST‑20 + conformance form (note: published versions may differ from DIN-hosted copies)
    - https://www.bast.de/DE/Publikationen/Regelwerke/Verkehrstechnik/Unterseiten/test20.html
    - https://www.bast.de/DE/Publikationen/Regelwerke/Verkehrstechnik/Downloads/test20-aufgaben.pdf?__blob=publicationFile&v=1
    - https://www.bast.de/DE/Publikationen/Regelwerke/Verkehrstechnik/Downloads/test20-konformitaet.pdf?__blob=publicationFile&v=1
  - [ ] Legal adoption context (16. BImSchV update referencing RLS‑19 day/night periods)
    - https://dserver.bundestag.de/btd/19/184/1918471.pdf
  - [ ] Example guidance for RLS‑19 input derivation from traffic data (DTV → day/night + vehicle groups)
    - https://www.berlin.de/sen/uvk/_assets/verkehr/verkehrsdaten/umrechungsfaktoren-von-verkehrsmengen/rechenbeispiel.pdf
    - https://rp-darmstadt.hessen.de/sites/rp-darmstadt.hessen.de/files/2023-06/22_02laermkennwerte_rls2019_0.pdf
  - [ ] Practitioner overview of RLS‑19 structure (emission vs propagation, two reflections)
    - https://www.ingenieur.de/fachmedien/laermbekaempfung/verkehrslaerm/richtlinien-fuer-den-laermschutz-an-strassen-rls19/
  - [ ] Identify authoritative version for CI gating (and how updates are handled)

---

## Phase 18 — Germany planning track: Schall 03 (required, deferred)

- [x] Define Schall 03 scope + compliance boundary
  - [x] Define supported planning use-case and legal context for the first implementation
  - [x] Separate implementation logic from restricted normative text/tables
  - [x] Document compliance boundary, source material used, and what must stay external
- [x] Build the current Schall 03 preview baseline
  - [x] Add `schall03/` module under `backend/internal/standards/`
  - [x] Define typed rail/planning source schema, receiver inputs, and infrastructure metadata
  - [x] Define the octave-band data model (`63 Hz` to `8000 Hz`) and deterministic energetic summation helpers
  - [x] Add a standards-framework descriptor with version/profile metadata, supported source types, indicators, and run parameter schema
  - [x] Implement a preview day/night source-emission model for the chosen first scope
  - [x] Implement a preview propagation chain with deterministic line integration and baseline slab/bridge/curve corrections
  - [x] Keep deterministic behavior for segmentation, band aggregation, and reductions
- [x] Wire the Schall 03 preview into runs and exports
  - [x] Add CLI/model extraction from normalized GeoJSON + run parameters into typed Schall 03 inputs
  - [x] Wire `noise run --standard schall03` through extraction, compute, provenance, and artifact persistence
  - [x] Export receiver tables and rasters with consistent metadata
  - [x] Record preview-model provenance metadata including band model and compliance boundary
- [x] Add repo-safe verification for the preview baseline
  - [x] Add module-level unit tests for schema validation, emission, propagation, band aggregation, and export
  - [x] Add a module-level golden scenario and deterministic snapshot
  - [x] Add a license-safe synthetic acceptance fixture to the shared acceptance catalog
  - [x] Add implementation notes and legal/compliance notes
- [ ] Replace the preview baseline with a standards-faithful Schall 03 implementation
  - [ ] Define the exact minimum normative delivery scope for Phase 18 completion
  - [ ] Move restricted coefficients/tables behind an external data-pack or other legally safe boundary
  - [ ] Replace the preview source-emission spectra with the standards-faithful Schall 03 emission chain for the chosen scope
  - [ ] Replace the preview propagation adjustments with the standards-faithful propagation/correction sequence for the chosen scope
  - [ ] Clarify and document authoritative output rounding and reporting rules separately from internal `float64` computation
  - [ ] Expand the typed source model where needed for normative inputs (for example train classes, track forms, and additional infrastructure detail)
- [ ] Strengthen Schall 03 conformance and acceptance coverage
  - [ ] Acquire or derive additional license-safe verification cases beyond the current synthetic preview fixture
  - [ ] Add a dedicated Schall 03 acceptance/conformance runner if the future fixture set needs tolerances, modes, or reporting beyond the shared catalog
  - [ ] Add comparison tolerances and fixture-version tracking rules for Schall 03 validation assets
  - [ ] Add end-to-end report/export checks once Schall 03 moves beyond preview status

### Research

- [ ] Clarify Schall 03 rounding rules and octave-band handling details
- [ ] Acquire license-safe verification or acceptance cases for Schall 03
- [ ] Define minimum shippable Schall 03 scope for first delivery
- [ ] Identify which normative coefficients/tables can be represented via an external data pack

---

## Phase 19 — Industry (international): ISO 9613‑2 (required, deferred)

- [ ] Define ISO 9613‑2 scope + compliance boundary
  - [ ] Define first supported engineering use-case for international industry calculations
  - [ ] Clarify whether the first delivery targets point sources only or point/line/area support
  - [ ] Ensure module separation so normative outputs stay normative and standard-specific
  - [ ] Document compliance boundary, source material used, and what must stay external
- [ ] Define ISO 9613‑2 data model and module structure
  - [ ] Add `iso9613/` module under `backend/internal/standards/`
  - [ ] Define typed source schemas, receiver inputs, ground/terrain inputs, and meteorological assumptions needed by the engineering method
  - [ ] Add standards-framework descriptor with version/profile metadata, supported source types, indicators, and run parameter schema
  - [ ] Define import/run mapping from normalized GeoJSON + run parameters into typed ISO 9613‑2 inputs
- [ ] Implement ISO 9613‑2 compute chain
  - [ ] Implement source emission interface for engineering-method sources
  - [ ] Implement the ISO 9613‑2 propagation chain needed for the first supported scope
  - [ ] Keep deterministic behavior for segmenting, attenuation ordering, and energetic summation
  - [ ] Document output rounding/reporting rules separately from internal float64 computation
- [ ] Wire ISO 9613‑2 into runs and exports
  - [ ] Wire `noise run --standard iso9613` through extraction, compute, provenance, and artifact persistence
  - [ ] Export receiver tables and rasters with consistent metadata
  - [ ] Ensure results remain clearly separated from CNOSSOS / national-track outputs
- [ ] Add verification coverage
  - [ ] Add module-level unit tests for schema validation, emission, propagation, and export
  - [ ] Add validation projects (license-safe)
  - [ ] Add golden scenarios and deterministic snapshots
  - [ ] Add implementation notes and legal/compliance notes

### Research

- [ ] Identify public example cases or create synthetic validation cases for ISO 9613‑2
- [ ] Clarify minimum source/terrain/ground input set needed for a useful first implementation
- [ ] Identify which coefficients/tables should live in an external standards data pack
- [ ] Document tolerances and comparison rules for validation cases

---

## Phase 20 — Reporting v1 (offline)

**Goal:** reproducible reports from offline artifacts.

- [x] Report templating v1 (Markdown/HTML)
- [x] Required report sections
  - [x] Input overview
  - [x] Standard ID + version/profile + parameters
  - [x] Maps/images if available
  - [x] Tables (receiver stats)
  - [x] QA status (which suites passed)
- [x] HTML-only MVP

---

## Phase 20b — PDF report export via Typst (deferred)

**Goal:** deterministic, versioned PDF output generated from offline report context.

- [ ] Add Typst template set for report PDF export
- [ ] Add `noise export --pdf` mode using Typst compilation
- [ ] Ensure report context (`report-context.json`) is sufficient for PDF rendering without re-reading run artifacts
- [ ] Define deterministic font and asset strategy for reproducible output hashes
- [ ] Add PDF golden/snapshot checks in CI (metadata and selected page text/image probes)
- [ ] Decide whether a DOCX report/export path is required after Markdown/HTML/PDF

### Research

- [ ] Evaluate Typst invocation strategy (embedded binary vs system dependency)
- [ ] Define template/versioning policy for backward-compatible report styles

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
- [ ] Evaluate `algo-pde` for research-only wave/low-frequency propagation experiments and sensitivity analysis
- [ ] Evaluate WebAssembly delivery for interactive research/demo modules without mixing them into normative runs

---

## Phase 23 — Deferred: local API + GUI activation (when “offline-only” changes)

**Goal:** introduce the local API contract and runtime needed to support a browser app.

- [x] Introduce `noise serve` (local-only)
  - [x] Initial `noise serve` command with graceful shutdown
  - [x] Define initial HTTP API v1 endpoints + DTOs (`/api/v1/health`, `/api/v1/project/status`, `/api/v1/runs`, `/api/v1/standards`)
  - [x] Progress streaming (SSE/WebSocket)
    - [x] Initial SSE endpoint `/api/v1/events` (heartbeat + project status stream)
    - [ ] WebSocket support (optional path, deferred)
  - [x] Standardized error format
  - [x] API versioning policy (`/api/v1` prefix on all routes)
  - [x] Local CORS/CSRF model for desktop-local usage (localhost/127.0.0.1 any port; `--cors-origins` flag)
- [x] API contract artifacts
  - [x] OpenAPI spec generation in CI (artifacts uploaded as `openapi-spec`)
  - [ ] TypeScript client generation pipeline for frontend (hand-crafted types kept in sync; auto-gen deferred)
  - [x] Error envelope schema (`code`, `message`, `details`, `hint`) — in Go + TS
- [ ] E2E smoke flow API-side (headless): import → validate → run → export

### Research

- [x] OpenAPI vs gRPC/Connect → REST/OpenAPI chosen (local-first ergonomics, browser-native fetch)
- [ ] DTO generation strategy and backward compatibility policy (deferred; types hand-crafted for now)

---

## Phase 23a — Frontend foundation (React/TS + Vite + Bun)

**Goal:** establish the frontend workspace and developer workflow.

- [x] Create frontend app scaffold with Bun + Vite + React + TypeScript
- [x] Define source layout
  - [x] `frontend/src/` for the main app (flat, no workspaces)
  - [x] `frontend/src/ui/` for shared UI wrappers/theme primitives
  - [x] `frontend/src/map/` for map adapters and layer helpers
  - [x] `frontend/src/api/` for backend API client and types
- [x] Configure strict TypeScript + ESLint + formatter integration
- [x] Configure environment handling (`.env`, API base URL, build-time flags)
- [x] Add frontend CI jobs (`bun install`, typecheck, lint, test, build)
- [x] Create architecture decision records for frontend conventions

---

## Phase 23b — UI system & design baseline (shadcn/ui)

**Goal:** build a consistent, accessible UI foundation.

- [x] Initialize shadcn/ui in the Vite app (Tailwind CSS v4 + shadcn CLI)
- [x] Define design tokens and theme contract
  - [x] Color scales for maps + UI chrome (OKLCH, muted teal accent, warm slate neutrals)
  - [x] Typography scale (IBM Plex Sans / IBM Plex Mono)
  - [x] Spacing/radius/elevation tokens (CSS variables via `@theme inline`)
- [x] Build reusable app primitives
  - [x] App shell layout
  - [x] Sidebar/navigation
  - [x] Dialog/sheet patterns
  - [x] Data table wrapper
  - [x] Form field wrappers
- [x] Accessibility baseline
  - [x] Keyboard navigation requirements for all shell components (via Radix UI primitives)
  - [x] Focus management and visible focus states (outline-ring)
  - [x] Screen-reader labels for icon-only actions (aria-label on theme toggle)
- [x] Dark mode support (system preference default, localStorage persistence)

### Research

- [x] Form stack decision: deferred to Phase 23e (React Hook Form + Zod likely; FormField wrapper ready)
- [x] Notification/toast strategy: deferred to Phase 23f (`sonner` likely; no toast needed yet)

---

## Phase 23c — App shell, routing, and data orchestration

**Goal:** make the SPA structure scalable and type-safe.

- [x] Routing architecture decision and implementation
  - [x] React Router v7 (data mode) — simple, well-established for ~7 routes
  - [x] Route-level code splitting via `lazy()` (each page is a separate chunk)
  - [x] Route guard support via Zustand `runInProgress` flag (enforced in Phase 23f)
- [x] Server-state strategy
  - [x] TanStack Query for API data fetching/cache/invalidation
  - [x] Query key factory in `src/api/query-keys.ts` (hierarchical, invalidation-friendly)
  - [x] Cache defaults: 30s stale, 5min GC, 1 retry, no refetch-on-focus
- [x] Client-state strategy
  - [x] Zustand for UI-only state (active nav, run-in-progress guard)
  - [x] URL state via React Router search params (shareable state for scenarios/runs)
- [x] Error boundaries + Suspense/loading architecture per route
  - [x] `ErrorBoundary` component with retry action
  - [x] `PageSkeleton` loading fallback
  - [x] Suspense wraps lazy-loaded route content in `RootLayout`

---

## Phase 23d — Map workspace core (MapLibre)

**Goal:** ship a robust interactive map workspace for model and result layers.

- [x] Implement MapLibre map module with controlled camera state
  - [x] Ref-based React wrapper (`MapView`), map instance via context (`useMap`)
  - [x] Navigation control + metric scale bar
- [x] Layer system v1
  - [x] Basemap/style loader (OpenFreeMap: light/bright/dark + offline fallback)
  - [x] Model layers (sources point/line/area, buildings fill+outline, barriers dashed, receivers)
  - [x] Result layers (raster placeholder, contour lines + labels)
  - [x] Layer ordering and visibility controls (`LayerControl` panel with per-group toggles)
- [x] Legend and color ramp subsystem
  - [x] `NOISE_LEVEL_RAMP` (green→red, 5 dB steps, ISO/EU END convention)
  - [x] `Legend` component with color swatches
  - [x] `rampToExpression()` for MapLibre paint property interpolation
- [x] Map interaction model
  - [x] Identify/select features (click → popup with properties table)
  - [x] Hover cursor change on interactive layers
  - [ ] Box select and multi-select support (deferred to Phase 23e)
- [x] CRS and coordinate display (`CoordinateDisplay` shows lat/lng on mouse move)
- [ ] Performance guardrails for large feature counts (deferred — clustering/tile fallback when needed)
- [x] Map state store (Zustand: basemap, layer visibility, selection, hover)

### Research

- [x] React binding strategy: native `maplibre-gl` with thin ref-based wrapper (no wrapper library)
- [x] Offline basemap: inline fallback style for air-gapped use; PMTiles deferred to Phase 25

---

## Phase 23e — Model editing workflows

**Goal:** enable practical model authoring and correction from the GUI.

- [x] Source editor workflow
  - [x] Point/line/area drawing and editing (terra-draw integration)
  - [x] Attribute forms per source type (source type picker)
- [x] Building/barrier editing workflow
  - [x] Geometry edits (terra-draw select mode with drag/midpoints/delete)
  - [x] Height and required attribute editors (height input field)
- [x] Validation overlay integration
  - [x] Display per-feature validation issues on map and in side panel (ValidationPanel)
  - [x] Deep-link from issue list to map feature ("Go to" button selects feature)
- [x] Import assistant UI
  - [x] Upload/select local files (drag-and-drop + file picker)
  - [x] Preview normalized model changes (feature counts, skipped, validation)
  - [x] Confirm-and-apply flow with diff summary
- [ ] Calculation area workflow
  - [ ] Define a first-class calculation-area map object
  - [ ] Use calculation area to constrain receiver-grid/run setup defaults where applicable
- [x] Undo/redo command stack for map edits (CommandStack + Ctrl+Z/Ctrl+Shift+Z)

---

## Phase 23f — Run configuration and execution UX

**Goal:** make run setup and monitoring reliable for long-running jobs.

- [x] Run setup dialog
  - [x] Standard/version/profile picker
  - [x] Parameter editor generated from schema
  - [x] Receiver set selector
- [x] Run execution monitor
  - [x] Progress timeline/steps
  - [x] Live logs
  - [x] Cancel/retry actions
- [x] Run history UX
  - [x] Filter by scenario/status/standard
  - [x] Artifact links per run
- [x] Determinism-aware UX hints (same inputs/profile expectation messaging)

---

## Phase 23g — Results analysis, comparison, and exports UX

**Goal:** visualize outputs and compare scenarios/runs effectively.

- [x] Result map views
  - [x] Raster rendering controls (ramp, min/max, opacity)
  - [ ] Contour overlays and labels
  - [x] Receiver value probe tool
- [x] Tabular analysis views
  - [x] Receiver table grid with sorting/filtering/export
  - [x] Indicator summary cards (min/max/mean/percentiles)
  - [ ] Contribution breakdown per receiver / selected result
- [x] Comparison workflows
  - [ ] Run-to-run diff layer
  - [x] Scenario A/B side-by-side mode
  - [ ] Scenario change-set summary for model/parameter differences
- [x] Export center UI
  - [x] Bundle export triggers
  - [x] Report preview for HTML report files
  - [x] Typst-PDF phase hook placeholder (Phase 20b)

---

## Phase 23h — Frontend QA, accessibility, and performance hardening

**Goal:** make frontend behavior stable, testable, and scalable.

- [x] Testing pyramid for frontend
  - [x] Unit/component tests (RTL + Vitest + jsdom: ErrorBoundary, ImportPage, DraftBanner, UIStore, autosave utils — 86 tests total)
  - [x] Route and state integration tests (ImportPage with createMemoryRouter; UIStore isolated)
  - [x] E2E (Playwright): config + smoke test covering load, sidebar nav, import flow, keyboard focus, axe WCAG2A/AA (`playwright.config.ts`, `e2e/smoke.spec.ts`)
- [x] Accessibility test automation
  - [x] Keyboard-only navigation tests (Playwright E2E: Tab focus assertion)
  - [x] Core screen-reader semantics checks (axe-core in vitest for ErrorBoundary; axe-playwright for full pages)
- [x] Performance observability
  - [x] Frame-time and interaction telemetry in dev mode (`PerformanceObserver` longtasks in `main.tsx`)
  - [x] Large-model synthetic benchmark scenes (`src/model/benchmark.test.ts`: 5k-feature normalize/validate/load/filter within budgets)
  - [x] Bundle size budgets + CI guard (`frontend/scripts/check-bundle-size.mjs`, `just fe-bundle-check`, `just fe-ci` extended)
- [x] Reliability features
  - [x] Autosave + unsaved-change protection (`src/model/use-autosave.ts`: debounced localStorage save, `beforeunload` guard)
  - [x] Crash-safe local draft recovery (`src/ui/draft-banner.tsx`: restore-on-startup banner, `discardDraft` on confirm)

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
- [ ] Finalize frontend router decision (React Router Data mode vs TanStack Router)
- [ ] Finalize frontend form strategy (RHF+Zod vs TanStack Form)
- [ ] Define frontend state boundaries (query cache vs global UI store vs URL state)
- [ ] Define map layer performance thresholds (feature count, tile fallback triggers)
- [ ] Define accessibility baseline and automated checks for map-heavy interactions

---

# Suggested near-term MVP path (offline-only, GeoJSON-only)

- [ ] Phases 1–3 (foundations + CI + project format)
- [ ] Phases 4–7 (GeoJSON import/validate + geo core + result containers + engine)
- [x] Phase 8 (dummy E2E)
- [x] Phase 9 (standards framework)
- [x] Phase 10 (CNOSSOS Road)

Then: Reporting (Phase 20), deferred standards phases, and when GUI is activated start frontend track with Phase 23 → 23a → 23b → 23c.
