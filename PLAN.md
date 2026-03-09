# PLAN.md — Implementation Plan for an Environmental Noise System

Status: 8 March 2026

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

## Phases 1–9 — Completed foundation track

**Goal:** summarize the completed platform and workflow baseline established before the first real normative standards modules.

- [x] Repo and delivery foundations are in place: repository layout, compliance boundaries, target platforms, definition-of-done, risk register, and offline-only MVP constraints are documented.
- [x] Backend and CLI foundations are complete: Go module/package structure, config/logging/error layers, Cobra command skeleton, shared flags, and baseline testability.
- [x] Reproducibility foundations are complete: CI lint/test/format checks, determinism policy, and golden-test conventions/workflows are documented and enforced.
- [x] Project lifecycle foundations are complete: manifest v1, project/run domain model, JSON-first storage, `noise init`, `noise status`, per-run provenance, and migration strategy.
- [x] Input and validation foundations are complete: GeoJSON-only import, minimal feature schemas, validation, and debug model exports.
- [x] Geo and result foundations are complete: CRS handling, geometry primitives, spatial indexing, receiver-set models, raster/table result containers, and export skeleton.
- [x] Compute foundations are complete: generic run pipeline, chunking, worker pool, deterministic reduction, progress events, cancellation/cleanup rules, disk-backed cache, and key determinism/cancel tests.
- [x] End-to-end and modularity foundations are complete: non-normative `dummy/freefield` E2E runs, golden demo coverage, and the standards plugin/profile/provenance framework.
- [x] Supporting technical investigations for geometry libraries, CRS/PROJ strategy, GeoTIFF writing, and contour generation were completed and documented.

---

## Phase 10 — CNOSSOS-EU: Road (required, implement earlier)

Status: complete.

- Shipped baseline: `cnossos-road` planning-track road module with typed line sources, deterministic emission/propagation, `Lday` / `Levening` / `Lnight` / `Lden`, CLI integration, provenance, and raster/table export.
- QA / evidence: module tests, golden regression, CLI E2E, synthetic acceptance fixtures, rounding/tolerance note, per-feature import extraction, and public attributable road reference totals.
- Boundary: shipped as a deterministic preview baseline, not a full public conformance suite.
- Baseline doc: `docs/phase10-cnossos-road-baseline.md`

---

## Phase 11 — CNOSSOS-EU: Rail (required, deferred)

Status: complete.

- Shipped baseline: `cnossos-rail` planning-track rail module with typed line sources, deterministic emission/propagation, rail-specific bridge/curve adjustments, CLI integration, provenance, and raster/table export.
- QA / evidence: module tests, golden regression, CLI E2E, synthetic acceptance fixtures, rounding/tolerance note, per-feature import extraction, and public attributable rail reference totals.
- Boundary: shipped as a deterministic preview baseline, not a full public conformance suite.
- Baseline doc: `docs/phase11-cnossos-rail-baseline.md`

---

## Phase 12 — CNOSSOS-EU: Industry (required, deferred)

Status: complete.

- Shipped baseline: `cnossos-industry` planning-track industry module with typed point/area sources, sound-power-based emission, deterministic propagation, CLI integration, provenance, and raster/table export.
- QA / evidence: module tests, golden regression, CLI E2E, synthetic acceptance fixtures, rounding/tolerance note, per-feature import extraction, and public attributable industry reference totals / exposure shares.
- Boundary: shipped as a deterministic preview baseline, not a full public conformance suite.
- Baseline doc: `docs/phase12-cnossos-industry-baseline.md`

---

## Phase 13 — CNOSSOS-EU: Aircraft (required long-term, deferred)

Status: complete.

- Shipped baseline: `cnossos-aircraft` planning-track aircraft module with airport-vicinity scope, typed 3D trajectories, movement-based emission/propagation, CLI integration, provenance, and raster/table export.
- QA / evidence: module tests, golden regression, CLI E2E, synthetic acceptance fixtures, rounding/tolerance note, per-feature import extraction, clarified minimum input contract, and public attributable aircraft reference totals.
- Boundary: shipped as a deterministic preview baseline, not a full public conformance suite.
- Baseline doc: `docs/phase13-cnossos-aircraft-baseline.md`

---

## Phase 14 — Germany mapping track: BUB (required, deferred)

Status: complete.

- Shipped baseline: `bub-road` mapping-track road module with explicit mapping context, divergent `bub/road`, `bub/rail`, and `bub/industry` package split, CLI integration, provenance, and raster/table export.
- QA / evidence: module tests, golden regression, CLI E2E, synthetic acceptance fixtures, per-feature import extraction, documented rounding/tolerances, and documented handling of external BUB datasets / rights.
- Boundary: shipped as a deterministic mapping preview baseline; official BUB raw datasets remain external unless redistribution rights are clear.
- Baseline doc: `docs/phase14-bub-road-baseline.md`

---

## Phase 15 — Germany mapping track: BUF (required, deferred)

Status: complete.

- Shipped baseline: `buf-aircraft` mapping-track aircraft module treated as a standalone module, with typed 3D trajectories, CLI integration, provenance, and raster/table export.
- QA / evidence: module tests, golden regression, CLI E2E, synthetic acceptance fixtures, per-feature import extraction, documented rounding/tolerances, clarified scope/I-O contract, and attributable public aircraft reference totals.
- Boundary: shipped as a deterministic mapping preview baseline, not a full official BUF conformance suite.
- Baseline doc: `docs/phase15-buf-aircraft-baseline.md`

---

## Phase 16 — Germany mapping track: BEB (required, deferred)

Status: complete.

- Shipped baseline: `beb-exposure` downstream mapping-track exposure module with building inputs, upstream support for `bub-road` and `buf-aircraft`, threshold-based affected totals, and `5 dB` `Lden` / `Lnight` banded summaries.
- QA / evidence: module tests, golden regression, CLI E2E, synthetic acceptance fixtures, per-feature occupancy extraction, documented dataset/rights handling, documented rounding/aggregation conventions, and attributable public exposed-population reference totals.
- Boundary: shipped as a deterministic mapping preview baseline, not a full official BEB conformance suite with official building/population data.
- Baseline doc: `docs/phase16-beb-exposure-baseline.md`

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

- [x] Offline reporting v1 is complete with Markdown/HTML templating, required report sections, and an HTML MVP.

---

## Phase 20b — PDF report export via Typst (deferred)

**Goal:** deterministic, versioned PDF output generated from offline report context.

- [x] Add Typst template set for report PDF export
- [x] Add `noise export --pdf` mode using Typst compilation
- [x] Ensure report context (`report-context.json`) is sufficient for PDF rendering without re-reading run artifacts
- [x] Define deterministic font and asset strategy for reproducible output hashes
- [ ] Add PDF golden/snapshot checks in CI (metadata and selected page text/image probes)
- [ ] Decide whether a DOCX report/export path is required after Markdown/HTML/PDF

### Research

- [x] Evaluate Typst invocation strategy (embedded binary vs system dependency)
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
  - [x] Broader cache keys and reuse for equivalent workloads
- [ ] Robust disk-backed cache + cleanup strategies
  - [x] Per-run chunk cache persists on disk
  - [x] Shared keyed chunk cache persists on disk across equivalent runs
  - [x] Benchmark suite cache cleanup via `noise bench --keep-last`
  - [x] General cache retention/cleanup policy across non-benchmark runs
  - [x] Stale-cache invalidation strategy for changed inputs/config
- [x] Implement `noise bench`
  - [x] Standard benchmark scenarios
  - [x] Output: runtime, memory, IO, numeric drift
  - [x] Persist benchmark suite summary under `.noise/cache/bench/<bench-id>/summary.json`
  - [x] Warm-cache reuse check against cached chunk artifacts
  - [ ] Numeric drift comparison across multiple worker/topology variants beyond the single reference-vs-cold run

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

## Phase 24 — Frontend foundation (React/TS + Vite + Bun)

**Goal:** establish the frontend workspace and developer workflow.

- [x] The frontend scaffold, source layout, strict TS/lint/format setup, environment handling, CI jobs, and frontend ADRs are in place.

---

## Phase 25 — UI system & design baseline (shadcn/ui)

**Goal:** build a consistent, accessible UI foundation.

- [x] The shadcn/ui design baseline is complete: theme tokens, reusable shell/form/table primitives, accessibility defaults, dark mode, and related frontend research decisions are documented.

---

## Phase 26 — App shell, routing, and data orchestration

**Goal:** make the SPA structure scalable and type-safe.

- [x] SPA routing, code-splitting, TanStack Query orchestration, Zustand/UI state boundaries, URL state, and route-level error/loading architecture are complete.

---

## Phase 27 — Map workspace core (MapLibre)

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
  - [ ] Box select and multi-select support (deferred to Phase 28)
- [x] CRS and coordinate display (`CoordinateDisplay` shows lat/lng on mouse move)
- [ ] Performance guardrails for large feature counts (deferred — clustering/tile fallback when needed)
- [x] Map state store (Zustand: basemap, layer visibility, selection, hover)

### Research

- [x] React binding strategy: native `maplibre-gl` with thin ref-based wrapper (no wrapper library)
- [x] Offline basemap: inline fallback style for air-gapped use; PMTiles deferred to Phase 36

---

## Phase 28 — Model editing workflows

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
- [x] Calculation area workflow
  - [x] Define a first-class calculation-area map object
  - [x] Use calculation area to constrain receiver-grid/run setup defaults where applicable
- [x] Undo/redo command stack for map edits (CommandStack + Ctrl+Z/Ctrl+Shift+Z)

---

## Phase 29 — Run configuration and execution UX

**Goal:** make run setup and monitoring reliable for long-running jobs.

- [x] Baseline run setup and monitoring are complete: schema-driven parameter editing, auto-grid receiver setup, live execution monitoring, cancel/retry controls, run history, and determinism-aware UX messaging.

---

## Phase 30 — Explicit receiver authoring and manual receiver sets

**Goal:** let users place and manage receiver positions directly instead of relying only on auto-grid generation.

- [x] Add a first-class receiver model to the frontend/domain layer
  - [x] Receiver point entity with stable ID, XY position, and receiver height
  - [x] Distinguish receiver objects from sources/buildings/barriers in model state and persistence/autosave
  - [x] Validation rules for explicit receiver sets (finite coordinates, positive height, duplicate-ID handling)
- [x] Add receiver authoring to the map workspace
  - [x] Draw/add receiver points directly on the map
  - [x] Edit receiver position and height from the feature editor
  - [x] Support selection, delete, undo/redo, and import-preview counts for receivers
  - [x] Show receiver layer and visibility controls in both MapLibre and fallback map modes
- [x] Enable manual receiver-set selection in run setup
  - [x] Replace the current disabled “Custom receiver set” placeholder with a working mode
  - [x] Let the run dialog choose between auto-grid and explicit receiver-set execution
  - [x] Clarify behavior when no explicit receivers exist
- [x] Wire manual receivers into browser/WASM execution
  - [x] Pass explicit receivers to RLS-19 runs instead of `buildReceiverGrid()` when custom mode is selected
  - [x] Export explicit-receiver results to receiver tables and run provenance
  - [x] Keep deterministic ordering of manually authored receivers
- [x] Wire manual receivers into backend/CLI/API execution
  - [x] Align receiver-set storage and run contracts with the Go project/run model
  - [x] Ensure HTTP/API and CLI runs can execute against explicit receiver sets
  - [x] Add run/report/export coverage for explicit receiver sets

---

## Phase 31 — Explicit source acoustics and per-source noise editing

**Goal:** let users define where the noise comes from by editing acoustic/emission inputs on each source feature, not only as run-wide defaults.

- [ ] Extend source feature data model for per-source acoustics
  - [ ] Support per-source traffic/emission attributes required for the first browser-delivered standards
  - [ ] Support per-source defaults vs run-level defaults with clear precedence rules
  - [ ] Preserve standard-specific source attributes through import/export/undo/autosave
- [ ] Add source-acoustics editing UI
  - [ ] Extend the source feature editor beyond `source_type`
  - [ ] Expose per-source traffic counts, speeds, surface type, gradient, and other RLS-19-relevant inputs
  - [ ] Add validation/help text so invalid or incomplete source inputs are visible before run start
- [ ] Update OSM/import mapping to populate source acoustics where possible
  - [ ] Map OSM attributes into source defaults when reliable
  - [ ] Mark inferred/missing values clearly so users can review them
- [ ] Wire per-source acoustics into browser/WASM execution
  - [ ] Make `buildRoadSources()` prefer feature-level source parameters over run-wide defaults
  - [ ] Keep run-level parameter fields as project-wide defaults / fallback values
  - [ ] Add regression coverage showing two sources with different acoustic inputs produce different results
- [ ] Wire per-source acoustics into backend/CLI/API execution
  - [ ] Align typed extraction from normalized GeoJSON/project model with frontend-authored source parameters
  - [ ] Record per-source acoustic provenance and export/report visibility
  - [ ] Keep deterministic extraction and ordering guarantees
- [ ] Extend standards incrementally
  - [ ] Start with `rls19-road` line sources
  - [ ] Define follow-on source-editing scopes for other standards/modules separately once the source schemas are stable

---

## Phase 32 — Results analysis, comparison, and exports UX

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

## Phase 33 — Frontend QA, accessibility, and performance hardening

**Goal:** make frontend behavior stable, testable, and scalable.

- [x] Frontend QA hardening is complete: unit/integration/E2E coverage, automated accessibility checks, performance telemetry and bundle budgets, plus autosave and crash-recovery reliability features.

---

## Phase 34 — Deferred: input formats beyond GeoJSON

**Goal:** add professional importers without blocking early delivery.

- [x] GeoPackage importer
- [ ] FlatGeobuf importer (deferred)
- [x] CSV traffic/time tables importer
- [ ] Terrain/DTM import (deferred)
- [ ] Building footprints/import pipelines beyond GeoJSON (deferred)

For GeoPackage importer:

- [x] Define schema + units
- [x] Implement import + validation
- [x] Add roundtrip tests

For CSV traffic/time tables importer:

- [x] Define schema + units
- [x] Implement import + validation
- [x] Add roundtrip tests

For each remaining importer (FlatGeobuf, Terrain/DTM, Building footprints):

- [ ] Define schema + units
- [ ] Implement import + validation
- [ ] Add roundtrip tests

---

## Phase 35 — OSM/Overpass import (`noise import --from-osm`, deferred, requires internet)

**Goal:** bootstrap scene geometry from OpenStreetMap without manual data preparation.

Use [`go-overpass`](https://github.com/MeKo-Christian/go-overpass) to query the Overpass API by bounding box and convert OSM elements into the project's GeoJSON model input format.

**Note:** this breaks the offline-only constraint and must remain opt-in (explicit `--from-osm` flag, never triggered automatically).

Planned data mappings:

| OSM element          | Noise model feature             | Key tags used                              |
| -------------------- | ------------------------------- | ------------------------------------------ |
| `highway=*`          | `source` (line, `cnossos-road`) | `highway`, `maxspeed`, `lanes`, `surface`  |
| `railway=rail/tram`  | `source` (line, `cnossos-rail`) | `railway`, `maxspeed`                      |
| `building=*`         | `building`                      | `building:levels`, `height`, `roof:height` |
| `barrier=wall/fence` | `barrier`                       | `barrier`, `height`                        |

Tasks:

- [x] Add `go-overpass` dependency (`github.com/MeKo-Christian/go-overpass`)
- [x] Implement `internal/io/osmimport` package: Overpass query by bbox → OSM elements → GeoJSON FeatureCollection
- [x] Map OSM tags to Aconiq feature properties (source type, height, standard attributes); document unmapped/ambiguous tags
- [x] Add `--from-osm` flag to `noise import` with `--bbox <south,west,north,east>` and optional `--overpass-endpoint`
- [x] Write unit tests with mocked Overpass responses; add roundtrip golden fixture
- [x] Document limitations: OSM data quality varies; heights often missing or inaccurate; no guarantee of completeness

---

## Phase 36 — Deferred: tiling/PMTiles

**Goal:** fast map rendering and efficient distribution when GUI exists.

- [ ] Evaluate vector tiles for model/results
- [ ] Evaluate PMTiles end-to-end pipeline
- [ ] Define storage/size budgets

---

## Phase 37 — Optional: desktop packaging (Wails)

**Goal:** ship a single-binary desktop option.

- [ ] Make the API runnable in-proc (no port needed)
- [ ] Embed frontend assets into Go binary
- [ ] Define build targets (`web` vs `wails`)
- [ ] Smoke tests for desktop build

### Risk / Research

- [ ] Re-check Wails v3 maturity (alpha risk) and define fallback options

---

## Phase 38 — Optional: project format v2 (multiuser/server)

**Goal:** PostGIS + object storage, versioned scenarios/runs.

- [ ] Map data model to PostGIS (geometries, indexes)
- [ ] Store artifacts in object storage (rasters/tiles/reports)
- [ ] Minimal auth/users (only if required)
- [ ] Migration tool: v1 project → v2

---

## Phase 39 — Release engineering + documentation + conformance artifacts

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

Then: Reporting (Phase 20), deferred standards phases, and when GUI is activated start frontend track with Phase 23 → 24 → 25 → 26.
