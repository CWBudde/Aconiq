# PLAN.md — Implementation Plan for Aconiq Environmental Noise System

Status: 28 March 2026

This plan is organized for execution rather than historical phase order.

- The active roadmap comes first and contains all unfinished or mixed-status work.
- Completed phases and the completed portions of mixed phases are compacted into a shipped baseline later in the document.
- Original phase numbers are retained in parentheses for traceability where useful.

## Strategic positioning

Aconiq is an auditable, deterministic noise calculation and automation platform, not a GUI clone of CadnaA, SoundPLAN, or IMMI. Core differentiators:

- Deterministic, reproducible runs with artifact provenance and golden-test regression.
- CLI-first plus local API for automation, CI/CD integration, and batch workflows.
- Open standards modules as plug-ins with explicit compliance boundaries per norm.
- Offline-first project format with full traceability from inputs to standard/profile to outputs.

The path to DACH adoption runs through four gates:

1. Legal clarity and compliance boundaries.
2. Real-world CRS and interoperability support.
3. Believable normative validation per standard.
4. DACH-specific assessment and reporting workflows.

## Clarifications

- Offline-only is acceptable for the near-term MVP. The CLI is primary; the local API and browser GUI are secondary.
- Input data support now includes GeoJSON, GeoPackage, FlatGeobuf, CSV, CityGML, OSM/Overpass, and GeoTIFF terrain.
- All named standards remain long-term targets, but they do not all carry the same delivery priority.
- The frontend stack remains React + TypeScript + Vite + Bun + shadcn/ui, but frontend polish is not on the critical path.

## Guiding principles

- Separate generic acoustics/geometry/compute core from standards modules.
- Treat quality assurance as a product feature, not a cleanup task.
- Publish conformance boundaries, tolerances, known deviations, and evidence per normative module.
- Keep the project format local-first; multiuser/server remains optional future work.

## Working definitions

- Project: folder with manifest, inputs, and artifacts.
- Scenario: input model plus standard selection plus parameters.
- Run: concrete calculation of one scenario against one receiver set with a fixed standard version/profile.
- Standards module: implementation of emission, propagation, indicators, and tables for a specific standard and version/profile.

---

## Active roadmap

### Priority 1 — ISO 9613-2 residual work

Status: the octave-band point-source workflow is implemented and documented. The remaining items are geometry-driven extensions and secondary use cases.

Why now: TA Lärm is already implemented as an assessment layer, so ISO 9613-2 is the main remaining propagation track needed for a complete industrial workflow.

Shipped already:

- [x] Typed inputs, deterministic point-source octave-band attenuation chain, import/run mapping, exported indicators, provenance, tests, and golden scenarios.
- [x] Normative attenuation terms implemented for A_atm, A_gr, simplified ground, A_bar with precomputed geometry, and C_met.
- [x] ISO 9613-2 conformance declaration, tolerances, embedded reference data decisions, synthetic validation cases, and acceptance-runner comparison rules.

Open work:

- [ ] Extract the shared barrier-intersection/ray-geometry logic from RLS-19 into a common package and wire automatic barrier detection for ISO 9613-2 diffraction inputs.
- [ ] Add reflections via image sources for enclosed industrial-yard cases once building geometry is readily available. This becomes more valuable when the SoundPlan import path below delivers richer building data.
- [ ] Add line and area source subdivision for extended industrial sources such as conveyor belts, cooling towers, and facades.
- [ ] Add spatial ground zones so the per-region G values come from polygon geometry instead of a single global ground factor.

### Priority 2 — Remaining Schall 03 closure work

Status: the normative computation core is complete; the remaining gaps are verification and one input-path extension.

Why now: Schall 03 itself is no longer the blocker, but its remaining open items affect reporting confidence and the completeness of the conformance story.

Done already, compacted:

- [x] All normative formulas Gl. 1-36, all tables 1-18, and all source types are implemented.
- [x] Strecke, Straßenbahnen, Rangierbahnhöfe, reflections, and barrier diffraction were delivered across former sub-phases 20 through 20d.
- [x] `ComputeNormativeReceiverLevelsWithScene(receiver, segments, walls, barriers)` is the full normative entry point.
- [x] The permanently slow section exception from Nr. 5.3.2 is implemented.
- [x] The BImSchV source-document audit from former Phase 37 is complete and pins corrected normative table values in tests.
- [x] Conformance declaration and CI-safe suite already exist.

Open work:

- [x] **Task B — End-to-end export golden test.** `TestExportGoldenSnapshot` in `schall03_test.go` golden-tests `receivers.json`, raster metadata, CSV row count, and raster binary size through the full `ComputeReceiverOutputs` + `ExportResultBundle` pipeline.

- [x] **Task C — Barrier diffraction acceptance scenarios.** Extended `scenarioFile` with `Walls` and `Barriers` fields; `computeSnapshots` now calls `ComputeNormativeReceiverLevelsWithScene` when scene elements are present. Added `b1_single_barrier` (basic screening, ~11 dB reduction) and `b2_barrier_with_wall` (barrier+reflection interaction) to the CI-safe suite (v3). Also added JSON tags to `BarrierSegment` and `ReflectingWall` structs for serialization.

- [x] **Task A — Section 9 measurement-based vehicle data (Nr. 9.1–9.3).** Section 9 of Schall 03 allows replacing Beiblatt 1–3 spectra with measured pass-by data (per DIN EN ISO 3095). The existing `DataPack` emission model is too coarse for this (single base spectrum + corrections). Add a `MeasuredVehicle` struct that provides octave-band spectra per Teilquelle (m = 1..11) for a custom Fz category, plus the Table 19/20 wheel/rail roughness-split parameters and Table 20 measurement-condition correction factors. This slots into the emission pipeline as an alternative to the Beiblatt lookup: when a segment's `VehicleInput.Fz` references a measured category number (≥ 100), the emission code uses the measured spectra instead. Subsections to implement: Nr. 9.1.1 (vehicles), 9.1.2 (components), 9.1.4 (track types), 9.1.5 (bridges), 9.1.6 (rail/wheel mitigation), 9.2.1–9.2.2 (evaluation of measurement results), and validation of the 2 dB / 4 dB significance thresholds.

### Priority 3 — 16. BImSchV scope completion

Status: a usable first assessment layer exists, but scope definition and broader workflow coverage remain open.

Why now: RLS-19 and Schall 03 already emit the core indicators. The remaining work is about making the regulatory layer explicit, reviewable, and aligned with report generation.

Shipped already:

- [x] RLS-19 and Schall 03 result flow into the assessment layer.
- [x] Threshold tables per area category and time period are implemented.
- [x] Combined road-plus-rail assessment is implemented where required.
- [x] Anspruch auf Lärmschutzmaßnahmen is determined.
- [x] Structured per-receiver assessment results, German-language text blocks, report integration, export-bundle integration, and verification coverage exist.
- [x] Current workflow supports explicit receivers that carry an area-category property.

Open work:

- [ ] Define the exact 16. BImSchV scope Aconiq claims to support.
- [ ] Clarify which sections and annexes are covered and which are intentionally excluded.
- [ ] Decide whether workflow coverage should expand beyond the current explicit-receiver model to support the reporting and onboarding scenarios planned later in this roadmap.

### Priority 4 — DACH reporting and report verification (former Phase 25 plus reporting residuals)

Goal: move from generic report/export capability to authority-facing German report packages that are deterministic, reviewable, and CI-checked.

Why now: most of the technical core exists. The remaining gap is turning those artifacts into practitioner-ready Gutachten outputs.

Already shipped into the baseline:

- [x] Offline Markdown/HTML templating and required report sections.
- [x] Typst PDF export via `aconiq export --pdf`.
- [x] Deterministic font/asset strategy and sufficient report context for current exports.

Open work:

- [ ] Decide whether a DOCX report/export path is required or whether Typst/PDF remains the only supported report target.
- [ ] Define DACH report template requirements for:
  - [ ] TA Lärm Gutachten.
  - [ ] 16. BImSchV Gutachten.
  - [ ] Generic Schallimmissionsprognose.
- [ ] Implement Typst templates for DACH reports:
  - [ ] Cover page, table of contents, and standard sections such as Aufgabenstellung, Grundlagen, Beurteilungsgrundlagen, Berechnungsverfahren, Ergebnisse, and Beurteilung.
  - [ ] Embedded result tables, maps, and contour plots.
  - [ ] Provenance/reproducibility section with standard version, data-pack version, and input hashes.
- [ ] Add German-language assessment text generation:
  - [ ] Threshold comparison tables with pass/fail per receiver and area.
  - [ ] Condition text blocks such as Auflagen and Nebenbestimmungen suggestions where appropriate.
- [ ] Add PDF golden/snapshot checks in CI, including metadata validation and selected page text/image probes.
- [ ] Add end-to-end report/export checks for Schall 03 and for the common report/export paths generally.

Research that feeds this priority:

- [ ] Define a template/versioning policy for backward-compatible report styles.
- [ ] Survey published Gutachten examples to pin down the minimum structure expected in practice.

### Priority 5 — QA hardening and conformance packaging (former Phases 27 and 29)

Goal: turn correctness, reproducibility, and conformance evidence into maintained product assets rather than ad hoc one-off validation work.

Why now: as more modules move from preview to claimable conformance, the QA and packaging layer becomes part of the product itself.

Open work — QA hardening:

- [ ] Expand `internal/qa/` with:
  - [ ] Loaders for standard test tasks.
  - [ ] Result comparison with tolerances and outlier reports.
  - [ ] Snapshot exporter for debugging.
- [ ] Expand fuzz/property tests:
  - [ ] Geometry robustness.
  - [ ] Numeric monotonicity properties where applicable.
- [ ] Add numeric drift tracking across commits.
- [ ] Add a repro-bundle export that captures run, inputs, standard, and profile in one package.

Open work — conformance packages:

- [ ] Define the conformance package structure per standards module:
  - [ ] Supported scope and sub-scope.
  - [ ] Tolerance rules and comparison methodology.
  - [ ] Reference test cases with provenance, source version, and license status.
  - [ ] Known deviations with rationale.
  - [ ] Machine-readable conformance report JSON.
- [ ] Publish a full RLS-19 conformance package, leveraging TEST-20.
- [ ] Publish a Schall 03 conformance package.
- [ ] Publish an ISO 9613-2 conformance package.
- [ ] Publish conformance packages for the CNOSSOS family: Road, Rail, Industry, and Aircraft.
- [ ] Automate conformance-report generation in CI with per-module pass/fail gating where practical.

### Priority 6 — SoundPlan import and cross-validation (former Phase 36)

Status: Steps 1 and 2 are complete; Step 3 now includes receiver-level comparison and a first heuristic raster comparison loop.

Goal: import SoundPlan projects, map them into Aconiq, run the same calculations, and compare outputs to build practitioner trust and reduce migration friction.

Why now: this is one of the highest-value interoperability and trust-building efforts once the core normative tracks are stable enough.

Format findings already established:

- `Project.sp` and `*.res` are INI-style metadata files.
- `*.geo` files are custom tagged binary geometry files covering rail, buildings, barriers, terrain, and calculation areas.
- `*.abs` files hold result, receiver, and emission data tables.
- `*.dgm`, `*.ntd`, `*.ets`, `*.esn`, and `Höhen.txt` remain relevant auxiliary formats.

Completed so far, compacted:

- [x] `Project.sp` parser with Windows-1252 support and extraction of project metadata, enabled standards, calculation parameters, assessment periods, and geometric defaults.
- [x] `.res` parser extracting run metadata, geometry references, assessment definitions, and command strings.
- [x] Reverse-engineering of `.geo` structure sufficiently far to implement parsers for `GeoRail.geo`, `GeoObjs.geo`, `GeoWand.geo`, `GeoTmp.geo`, and `CalcArea.geo`.
- [x] Result `.abs` parsing for single-point result families, including group results, partial results, receiver metadata, train type catalog, and run-subdirectory loading.

Open work — remaining parser coverage:

- [x] In `GeoObjs.geo`, extract building heights from `:D\xa0` records and attach address anchors from type `0x03e9` / `:D1` name records to buildings.
- [x] In `GeoWand.geo`, extract barrier material and absorption properties from `:D!` records.
- [x] In `GeoTmp.geo`, extract digital ground model data from `.dgm` binaries.
- [x] Add the fallback import path for `Höhen.txt` elevation points.
- [x] Implement `.abs` parsing for RRAI and RRAD emission data.
- [x] Implement grid-map parsing for `RRLK*` and `RRLK*.GM`:
  - [x] Extract currently reliable metadata such as discovered layer names, file size, linked assessment periods, and run statistics.
  - [x] Decode the current fixture's GM cell stream into per-row active-cell spans plus elevation/day/night values.
  - [x] Extract raster metadata such as origin, spacing, and dimensions from explicit SoundPLAN payload metadata rather than heuristics.
  - [x] Extract raster level values.
- [x] Implement `.ntd` parsing for immission point tables.

Open work — model mapping:

- [ ] Define mapping from SoundPlan standard IDs to Aconiq standards modules:
  - [ ] `20490` to `schall03`.
  - [ ] `10490` to `rls19`.
  - [ ] `30000` to `iso9613`.
  - [ ] Unsupported standards must emit clear warnings.
- [ ] Convert SoundPlan rail geometry into Aconiq `TrackSegment` and `TrainOperation` structures.
- [ ] Map SoundPlan track parameters and train types to Aconiq emission model fields and Fz categories.
- [ ] Convert SoundPlan buildings, barriers, terrain, receivers, and calculation areas into the Aconiq internal model.
- [ ] Determine SoundPlan project CRS and route it through the Phase 18 CRS pipeline.

Open work — workflow and validation:

- [ ] Implement `aconiq import --from-soundplan <project-dir>`.
- [ ] Implement a comparison mode such as `aconiq compare` that can:
  - [x] Compare per-receiver levels.
  - [x] Produce raster difference data and per-cell deltas through a first heuristic scanline alignment.
  - [x] Compute summary statistics such as mean, max, P95, and tolerance exceedances.
  - [x] Support configurable tolerance thresholds, for example ±0.5 dB for conformance and ±1.0 dB for informational comparison.
- [ ] Generate a cross-validation report artifact with tables, deviation distribution, map overlay, and provenance.
- [ ] Add unit tests for all parsers.
- [ ] Add an integration test around `interoperability/Schienenprojekt - Schall 03/` to verify geometry extraction and parameter mapping.
- [ ] Add a cross-validation acceptance test for import, run, and compare.
- [ ] Document format findings in `docs/research/soundplan-format.md`.

Refined execution slices:

- [ ] Slice A — import preparation layer:
  - [x] Add a single project-bundle loader that parses the currently supported SoundPlan inputs.
  - [x] Add explicit SoundPlan standard-ID to Aconiq-standard mapping with deterministic warnings for unsupported IDs.
  - [x] Add `Höhen.txt` terrain fallback loading.
  - [x] Add a structured import-report JSON artifact describing discovered files, parser coverage, warnings, and unresolved fields.
- [ ] Slice B — geometry-to-model conversion:
  - [x] Convert rail tracks into normalized line-source GeoJSON features with explicit placeholder/default properties.
  - [x] Derive imported rail speed, dominant train names, train-class and traction heuristics, bridge flags, and day/night trains per hour from `RRAI` and `RRAD` where available.
  - [x] Convert buildings into normalized building polygons with height handling decisions documented.
  - [x] Convert barriers into normalized barrier lines with height handling decisions documented.
  - [x] Convert receivers into normalized receiver points using project/run defaults for receiver height.
  - [ ] Convert calc area into import metadata for future grid/run setup.
- [ ] Slice C — CLI integration:
  - [x] Add `aconiq import --from-soundplan <project-dir>`.
  - [x] Persist normalized model, dump, validation report, and SoundPlan import report under `.noise/model/`.
  - [x] Surface unsupported standards and unresolved mappings as non-fatal warnings in CLI output.
  - [x] Add `aconiq compare` receiver validation against `RREC` and heuristic raster validation against decoded `RRLK*.GM` runs.
- [ ] Slice D — validation loop:
  - [x] Add integration coverage for the sample Schall 03 project bundle and normalized model output.
  - [x] Add first run-level comparison for single-point receiver results.
  - [x] Surface SoundPLAN raster/grid runs in import and compare reports via `RRLK*.GM` metadata parsing.
  - [x] Upgrade raster reporting from metadata-only to decoded value/range reporting with active-cell row spans.
  - [ ] Extend comparison from decoded SoundPLAN raster values to actual spatially aligned raster/grid level deltas once origin/alignment is decoded.

Research and legal questions that remain attached to this priority:

- [ ] How stable is the binary `.geo` format across SoundPlan versions?
- [ ] Are there XML, text, or ASCII export options from SoundPlan that are easier to parse than `.geo`?
- [ ] Can SoundPlan export via better-documented intermediate formats such as CadnaA-compatible exchange?
- [ ] Confirm and document the legal interoperability position for parsing the proprietary format.

### Priority 7 — Performance and scaling (former Phase 28)

Goal: keep city-scale workloads practical without weakening determinism.

Shipped already:

- [x] Broader cache keys and reuse for equivalent workloads.
- [x] Per-run and shared keyed chunk cache on disk.
- [x] Cache retention/cleanup policy and stale-cache invalidation.
- [x] `aconiq bench` with standard scenarios, runtime/memory/IO/drift output, summary persistence, warm-cache reuse, and benchmark-suite cleanup support.

Open work:

- [ ] Optimize the tiled compute pipeline:
  - [ ] Spatial index tuning.
  - [ ] Candidate pruning.
- [ ] Compare numeric drift across multiple worker and topology variants inside the benchmark flow.

Optional, advanced, non-normative work under this priority:

- [ ] `algo-fft` and `algo-dsp` for non-normative post-processing pipelines.
- [ ] `algo-pde` for research-only wave and low-frequency propagation experiments.
- [ ] WebAssembly delivery for interactive research/demo modules.

### Priority 8 — Example projects and DACH onboarding (former Phase 30)

Goal: make adoption easier by giving new users complete, license-safe, runnable examples.

Why now: reproducible examples can offset the lack of commercial training/support and make the standards/assessment story concrete.

Open work:

- [ ] Create synthetic, license-safe example projects for:
  - [ ] RLS-19 road corridor in a 16. BImSchV context.
  - [ ] Schall 03 rail section.
  - [ ] ISO 9613-2 industrial point source in a TA Lärm context.
  - [ ] Combined road-plus-rail scenario.
- [ ] Ensure each example includes input data, run config, expected outputs, and a step-by-step README.
- [ ] Add a German-language getting-started guide.
- [ ] Add CI jobs that keep example projects green across releases.

### Priority 9 — Community and release engineering (former Phase 31)

Goal: make the project visibly maintainable and adoptable from outside the core development circle.

Open work:

- [ ] Define versioning and changelog process, including SemVer and `CHANGELOG.md`.
- [ ] Build release binaries, at least for the CLI, via GitHub Releases.
- [ ] Enable GitHub Issues with templates for bug, feature request, and conformance questions.
- [ ] Keep a public roadmap, either in this file or via a GitHub project board.
- [ ] Build the documentation site with:
  - [ ] Getting started in English and German.
  - [ ] Project format specification.
  - [ ] Standards-module overview, status, and conformance boundaries.
  - [ ] QA/acceptance process and tolerances.
- [ ] Add release-tag golden-test gates in CI.
- [ ] Establish German-language community presence, for example via a blog post, conference talk, or Fachzeitschrift article.

## Deferred and optional tracks

### Deferred frontend hardening (former Phase 32)

Goal: keep the already-built frontend foundation viable without pulling focus from the normative and reporting roadmap.

Deferred from earlier phases:

- [ ] WebSocket support for progress streaming. SSE already works, so this is optional.
- [ ] TypeScript client-generation pipeline for frontend API types.
- [ ] Headless E2E smoke flow on the API side: import to validate to run to export.
- [ ] Box select and multi-select on the map.
- [ ] Contour overlays and labels on the result map.
- [ ] Contribution breakdown per receiver or selected result.
- [ ] Run-to-run diff layer.
- [ ] Scenario change-set summary for model and parameter differences.
- [ ] Performance guardrails for large feature counts, such as clustering or tile fallback.
- [ ] Building-footprint/import pipelines beyond GeoJSON.

Per-source acoustics hardening:

- [ ] UI-level interaction coverage for editing, clearing, and restoring per-source acoustic overrides.
- [ ] Surface per-source acoustic overrides and inferred review flags in popups or run-setup summaries.
- [ ] Decide whether additional OSM-derived defaults are deterministic enough to enable.
- [ ] Define follow-on source-editing scopes for other standards/modules.

### Tiling and PMTiles (former Phase 33)

- [ ] Evaluate vector tiles for model and result delivery.
- [ ] Evaluate an end-to-end PMTiles pipeline.
- [ ] Define storage and size budgets.

### Desktop packaging (former Phase 34)

- [ ] Make the API runnable in-process with no external port requirement.
- [ ] Embed frontend assets into the Go binary.
- [ ] Define build targets for `web` versus `wails`.
- [ ] Add smoke tests for desktop builds.
- [ ] Re-check Wails v3 maturity and define fallback options.

### Project format v2 (former Phase 35)

- [ ] Map the data model to PostGIS with geometry storage and indexes.
- [ ] Store artifacts in object storage for rasters, tiles, and reports.
- [ ] Add minimal auth/users only if genuinely required.
- [ ] Add migration tooling from v1 projects to v2.

---

## Shipped baseline (compacted)

This section keeps the completed work visible without forcing the active roadmap to scroll past finished material.

### Core platform and workflow foundations (former Phases 1-9)

- [x] Repository layout, compliance preflight docs, target platforms, definition of done, risk register, and offline-only MVP constraints.
- [x] Go module/package structure, config/logging/error layers, Cobra CLI skeleton, shared flags, and testability.
- [x] CI for lint/test/format, determinism policy, and golden-test conventions.
- [x] Project lifecycle with manifest v1, project/run domain model, JSON-first storage, `aconiq init`, `aconiq status`, provenance, and migrations.
- [x] GeoJSON import, feature schemas, validation, and debug model exports.
- [x] Geometry primitives, spatial indexing, receiver-set models, raster/table result containers, and export skeleton.
- [x] Generic deterministic run pipeline with chunking, worker pool, progress events, cancellation/cleanup, and disk-backed cache.
- [x] Non-normative `dummy/freefield` E2E runs, golden demo coverage, and standards plugin/profile/provenance framework.
- [x] Early technical investigations around geometry libraries, CRS strategy, GeoTIFF writing, and contour generation.

### Preview standards baselines and Germany mapping track (former Phases 10-16)

- [x] CNOSSOS Road baseline: `docs/phase10-cnossos-road-baseline.md`.
- [x] CNOSSOS Rail baseline: `docs/phase11-cnossos-rail-baseline.md`.
- [x] CNOSSOS Industry baseline: `docs/phase12-cnossos-industry-baseline.md`.
- [x] CNOSSOS Aircraft baseline: `docs/phase13-cnossos-aircraft-baseline.md`.
- [x] BUB Road baseline: `docs/phase14-bub-road-baseline.md`.
- [x] BUF Aircraft baseline: `docs/phase15-buf-aircraft-baseline.md`.
- [x] BEB Exposure baseline: `docs/phase16-beb-exposure-baseline.md`.

### CRS, import, and export foundations (former Phases 18, 24, 26, and the completed import-format work)

- [x] Pure-Go CRS transformation pipeline based on `github.com/wroge/wgs84`, covering the main DACH EPSG and Gauss-Krüger cases, with import/export integration and comprehensive tests.
- [x] GeoTIFF/COG raster export, GeoPackage vector export, contour generation/export, and export-format matrix.
- [x] GeoPackage, FlatGeobuf, CSV traffic/time-table importers.
- [x] Terrain/DTM import from GeoTIFF with bilinear interpolation and runtime loading.
- [x] OSM/Overpass import via `aconiq import --from-osm`.
- [x] CityGML import hardening with attribute preservation, structured import reporting, and documented decisions.

### Assessment and conformance foundations already shipped (former Phases 20 shipped scope, 22, and 37)

- [x] Legal and compliance close-out is complete, including CI license scanning and finalized compliance boundaries.
- [x] RLS-19 conformance closure is complete, including the Eq. 16 multi-diffraction shielding work and full reconciliation against the authoritative source documents.
- [x] Schall 03 normative core, reflections, barrier diffraction, and BImSchV conformance audit.
- [x] TA Lärm assessment layer complete, including thresholds, Teilzeiten, Lr computation, load assessment, receiver assessment logic, export envelope, report text blocks, golden scenarios, and conformance declaration.
- [x] Existing conformance declarations for RLS-19, ISO 9613-2, Schall 03, and TA Lärm are already in the repository.

### API and frontend foundation (former shipped portions of Phases 23 and 32)

- [x] `aconiq serve` with HTTP API v1, health/status/runs/standards endpoints, SSE events, and OpenAPI.
- [x] React/TypeScript/Vite/Bun frontend scaffold with shadcn/ui baseline, SPA routing, TanStack Query, and Zustand.
- [x] MapLibre workspace with basemap, model/result layers, legend, interactions, and coordinate display.
- [x] Model editing for sources, buildings, barriers, and receivers, plus validation overlay, import assistant, and undo/redo.
- [x] Run configuration and execution UX, explicit receiver authoring, and per-source acoustics editing foundation.
- [x] Results analysis with raster rendering, receiver tables, scenario comparison, export center, and frontend QA coverage.

### Partially complete but materially shipped regulatory workflow (former Phase 23)

- [x] Initial 16. BImSchV assessment workflow for explicit receivers with area-category properties.
- [x] Per-receiver threshold comparison, combined road/rail handling, German assessment text, export-bundle JSON, and report integration.

---

## Research backlog

### Standards and validation data

- [ ] CNOSSOS Road/Rail/Industry/Aircraft: collect license-safe validation cases and define tolerances.
- [ ] BUB/BUF/BEB: obtain current documents/annexes and define exact input requirements per module.
- [ ] Schall 03: acquire license-safe verification cases and clarify redistribution rights for normative tables.
- [ ] TA Lärm: survey published Gutachten for structural conventions and assessment patterns.
- [ ] 16. BImSchV: clarify combined assessment rules for road plus rail.

### GIS, CRS, and format research

- [ ] GeoTIFF export: settle the long-term dependency strategy for writing, given that the existing reader is pure Go.
- [ ] Contour generation: define the preferred algorithm and quality requirements.

### Determinism and tolerances

- [ ] Standardize numeric tolerances per standard and test suite.
- [ ] Define the stable summation strategy and document where it must apply.

### UX and workflow questions

- [ ] Define DTO-generation strategy and backward-compatibility policy.
- [ ] Define which exports are must-have versus deferred, for example GeoTIFF, CSV, PNG, and report artifacts.
- [ ] Define map-layer performance thresholds and tile-fallback triggers.
- [ ] Define the accessibility baseline for map-heavy interactions.
