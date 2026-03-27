# PLAN.md — Implementation Plan for Aconiq Environmental Noise System

Status: 13 March 2026

This is a **comprehensive, phased implementation plan** (Go backend + React/TypeScript frontend + GIS/MapLibre). It is intentionally **very granular** (bite-sized checklist tasks) so the system remains runnable and testable throughout.

## Strategic positioning

Aconiq is positioned as an **auditable, deterministic noise calculation and automation platform** — not a GUI clone of CadnaA/SoundPLAN/IMMI. Core differentiators:

- **Deterministic, reproducible runs** with artifact provenance and golden-test regression.
- **CLI-first + local API** for automation, CI/CD integration, and batch workflows.
- **Open standards modules** as plug-ins with explicit compliance boundaries per norm.
- **Offline-first** project format with full traceability (inputs → standard/profile → outputs).

The path to DACH adoption runs through: **(a)** legal clarity (license), **(b)** CRS/interoperability, **(c)** belastbare normative validation per standard, **(d)** DACH-specific reporting and assessment logic.

## Clarifications (explicit decisions)

- **Offline-only is fine for now.** The near-term MVP is CLI-driven and writes artifacts into a project folder. A local API (`serve`) and browser GUI exist but are secondary.
- **Input data:** GeoJSON, GeoPackage, FlatGeobuf, CSV, CityGML, OSM/Overpass, and GeoTIFF terrain are all supported.
- **Standards:** all standards mentioned are required long-term. Each one has a dedicated phase.
- **Frontend stack:** React + TypeScript + Vite + Bun + shadcn/ui (deferred priority).

## Guiding principles

- **Separate core vs. standards:** a generic acoustics/geometry/compute core + standards modules (CNOSSOS, BUB/BUF/BEB, RLS-19, Schall 03, ISO 9613-2, ...) as plug-ins.
- **Quality assurance is a feature:** deterministic runs, golden tests, and acceptance suites (e.g., RLS-19 TEST-20) are first-class.
- **Conformance as a product feature:** each normative module publishes its compliance boundary, supported scope, tolerances, known deviations, and validation evidence.
- **Project-oriented data model:** v1 local (folder + files), v2 optional multiuser/server (e.g., PostGIS + object storage).

## Working definitions

- **Project**: folder with manifest + inputs + artifacts.
- **Scenario**: input model + standard selection + parameters.
- **Run**: a concrete calculation of one scenario against one receiver set, with a fixed standard version/profile.
- **Standards module**: implementation of emission/propagation/indicators/tables for a specific standard and version/profile.

---

## Completed foundations (Phases 1-16, 20, 23-33)

All completed phases are compacted here. Detailed baseline docs exist under `docs/`.

### Platform & workflow (Phases 1-9)

- [x] Repo layout, compliance boundaries, target platforms, definition-of-done, risk register, offline-only MVP constraints.
- [x] Go module/package structure, config/logging/error layers, Cobra command skeleton, shared flags, testability.
- [x] CI lint/test/format checks, determinism policy, golden-test conventions.
- [x] Project lifecycle: manifest v1, project/run domain model, JSON-first storage, `noise init`, `noise status`, per-run provenance, migrations.
- [x] GeoJSON import, feature schemas, validation, debug model exports.
- [x] CRS handling (identity-only), geometry primitives, spatial indexing, receiver-set models, raster/table result containers, export skeleton.
- [x] Generic run pipeline: chunking, worker pool, deterministic reduction, progress events, cancellation/cleanup, disk-backed cache.
- [x] Non-normative `dummy/freefield` E2E runs, golden demo coverage, standards plugin/profile/provenance framework.
- [x] Technical investigations: geometry libraries, CRS/PROJ strategy, GeoTIFF writing, contour generation.

### CNOSSOS-EU family (Phases 10-13) — all shipped as deterministic preview baselines

- [x] **Phase 10 — CNOSSOS Road:** planning-track road module. Baseline doc: `docs/phase10-cnossos-road-baseline.md`
- [x] **Phase 11 — CNOSSOS Rail:** planning-track rail module. Baseline doc: `docs/phase11-cnossos-rail-baseline.md`
- [x] **Phase 12 — CNOSSOS Industry:** planning-track industry module. Baseline doc: `docs/phase12-cnossos-industry-baseline.md`
- [x] **Phase 13 — CNOSSOS Aircraft:** planning-track aircraft module. Baseline doc: `docs/phase13-cnossos-aircraft-baseline.md`

### Germany mapping track (Phases 14-16) — all shipped as deterministic preview baselines

- [x] **Phase 14 — BUB Road.** Baseline doc: `docs/phase14-bub-road-baseline.md`
- [x] **Phase 15 — BUF Aircraft.** Baseline doc: `docs/phase15-buf-aircraft-baseline.md`
- [x] **Phase 16 — BEB Exposure.** Baseline doc: `docs/phase16-beb-exposure-baseline.md`

### Reporting v1 (Phase 20)

- [x] Offline Markdown/HTML templating, required report sections, HTML MVP.
- [x] Typst PDF export (`noise export --pdf`), deterministic font/asset strategy, report context sufficiency.
- [ ] PDF golden/snapshot checks in CI (metadata and selected page text/image probes)
- [ ] Decide whether a DOCX report/export path is required

### Local API + GUI foundation (Phases 23-33)

- [x] `noise serve` with HTTP API v1 (health, project status, runs, standards, SSE events, OpenAPI).
- [x] Frontend scaffold: React/TS/Vite/Bun, shadcn/ui design baseline, SPA routing, TanStack Query, Zustand.
- [x] MapLibre map workspace: basemap, model/result layers, legend, interactions, coordinate display.
- [x] Model editing: source/building/barrier/receiver drawing and editing, validation overlay, import assistant, undo/redo.
- [x] Run configuration and execution UX, explicit receiver authoring, per-source acoustics editing.
- [x] Results analysis: raster rendering, receiver tables, scenario A/B comparison, export center.
- [x] Frontend QA: unit/integration/E2E, accessibility, performance telemetry, autosave, crash recovery.

### Import formats (Phases 34-35)

- [x] GeoPackage, FlatGeobuf, CSV traffic/time tables importers.
- [x] Terrain/DTM import (GeoTIFF, bilinear interpolation, CLI `--terrain`, run-time loading).
- [x] OSM/Overpass import (`noise import --from-osm`).
- [x] CityGML import (building solids → footprints + height, format detection, validation).

---

## Phase 17 — Legal & governance (HIGH PRIORITY — new)

**Goal:** establish legal clarity for open-source release and third-party adoption.

**Why:** Without a license file, the code is visible but not legally usable. The compliance boundaries doc already targets a permissive license but marks it "to be finalized." This blocks all external adoption and contribution.

- [x] Choose and add `LICENSE` file — MIT License
- [x] Add `NOTICE` with dependency license attributions (including MPL-2.0 flag for hashicorp/golang-lru)
- [x] Add `CONTRIBUTING.md` with contribution guide and standards-module guidance
- [x] Add `SECURITY.md` with vulnerability reporting process
- [ ] Add dependency license scan to CI (e.g., `go-licenses` or equivalent)
- [x] Define trademark/naming rules for "Aconiq" — N/A for now
- [ ] Review and finalize the compliance boundaries doc (`docs/policies/compliance-boundaries.md`)

---

## Phase 18 — CRS transformation pipeline (HIGH PRIORITY)

**Goal:** enable real-world DACH data workflows that depend on coordinate reference system transformations.

**Why:** Every real DACH dataset arrives in EPSG:25832/25833, GK zones, or local CRS. Without robust CRS transforms, import/overlay/export is fragile for production use.

**Decision:** Pure-Go library `github.com/wroge/wgs84` v1.1.7 (MIT, zero cgo). Evaluated alternatives: `go-spatial/proj` (immature), `twpayne/go-proj` (requires cgo), `im7mortal/UTM` (GPL), hand-rolling (unnecessary). See `crs.go` for implementation.

- [x] Define CRS strategy — pure-Go, no cgo (`wroge/wgs84` v1)
- [x] Implement core CRS transformation pipeline (`EPSGTransform` in `internal/geo/crs.go`)
  - [x] Support EPSG:4326 ↔ EPSG:25832 (UTM zone 32N) — the primary DACH case
  - [x] Support EPSG:4326 ↔ EPSG:25833 (UTM zone 33N)
  - [x] Support EPSG:25831 and EPSG:25834 (UTM zones 31N, 34N)
  - [x] Support Gauss-Krüger zones (EPSG:31466-31469) with Helmert datum shift (DHDN→ETRS89)
  - [x] Support WGS84 UTM zones 32N/33N (EPSG:32632/32633)
  - [x] Support Web Mercator (EPSG:3857)
  - [x] Support ETRS89 geographic (EPSG:4258)
  - [x] Cross-zone transforms (e.g., UTM32→UTM33) work via geographic intermediate
- [x] Add comprehensive CRS tests (`crs_transform_test.go`)
  - [x] Forward/inverse tests for WGS84↔UTM32, UTM32↔WGS84, GK3→UTM32
  - [x] Cross-zone UTM32→UTM33 test
  - [x] Roundtrip accuracy tests for all supported EPSG codes (6 geographic/projected pairs)
  - [x] All 13 supported EPSG codes verified with zone-appropriate test coordinates
  - [x] Edge-case tests: NaN, Inf, unsupported codes, non-EPSG formats
- [x] Wire CRS transforms into import pipeline
  - [x] Auto-detect input CRS from GeoJSON/GeoPackage/CityGML/FlatGeobuf metadata where available
  - [x] Transform imported geometry to project CRS on import (`NormalizeWithCRS` + `--input-crs` flag)
  - [x] OSM import hardcoded to EPSG:4326
  - [x] Strict error handling for unsupported or ambiguous CRS declarations
- [x] Wire CRS transforms into export pipeline
  - [x] `--target-crs` flag re-projects model GeoJSON to target CRS on export
  - [x] Normalized model GeoJSON copied to export bundle with CRS metadata
  - [x] `ProjectCRS` included in export summary JSON
  - [x] `CRS` field added to `RasterMetadata` for raster sidecar files

---

## Phase 19 — RLS-19: standards-faithful completion & conformance

Status: infrastructure complete; normative formula accuracy requires review against the actual document.

**Note (2026-03-27):** The RLS-19 document (FGSV 052, Ausgabe 2019 + Korrekturblatt Feb 2020) and supplementary documents (ARS 19/2020, BT-Drucksache 19/18471) are now available at `interoperability/RLS-19/`. The `tables.go` implementation was written from placeholder values before the document was available. A full reconciliation pass against the normative text is required.

**Legal context:** RLS-19 is legally binding as of 1 March 2021 under § 3 of the amended 16. BImSchV (BGBl. 2020 I S. 2334). Sections 1 and 3 are directly referenced by the Verordnung. The ARS Nr. 19/2020 makes it binding for Bundesfernstraßen (new construction and Lärmsanierung).

### Completed (infrastructure)

- [x] Scope, compliance boundaries, data model, emission chain (E1-E7/EG), propagation (Teilstückverfahren, shielding, topography, reflections), indicators (LrDay/LrNight), provenance, export — all implemented.
- [x] Dedicated TEST-20 acceptance runner with local-suite and CI-safe modes, per-task tolerances, conformance report artifact.
- [x] Clarify how TEST-20 data is obtained, stored, and legally redistributed (`docs/research/rls19-test20-legal-analysis.md`)
- [x] Track public sources and versions (`docs/research/rls19-road-test20-notes.md` — version tracking section)
- [x] Identify authoritative TEST-20 version for CI gating and define update handling (v2.1, derived-ci-safe-v1)
- [x] Publish a formal "RLS-19 Konformitätserklärung" artifact (`docs/conformance/rls19-konformitaetserklaerung.md` — draft template)
- [x] Add a machine-readable conformance report (JSON) with category coverage exportable alongside run artifacts
- [x] Expand CI-safe suite to cover all TEST-20 task categories: E1-E7 (emission), I1-I9 (immission, ref+check), K1-K5 (complex, ref+check) — 34 total tasks

### Section 3.3.4 — Grundwert des Schallleistungspegels (Eq. 6 / Tabelle 3)

The actual RLS-19 Eq. 6 formula is: `L_{W0,FzG}(v) = A_{W,FzG} + 10·lg[1 + (v/B_{W,FzG})^C_{W,FzG}]`
with normative Tabelle 3 coefficients: Pkw (A=88.0, B=20, C=3.06), Lkw1 (A=100.3, B=40, C=4.33), Lkw2 (A=105.4, B=50, C=4.88).
The current `tables.go` uses a rolling+propulsion split formula with placeholder coefficients — not the RLS-19 formula.

- [x] Replace rolling+propulsion formula with Eq. 6 `A + 10·lg[1 + (v/B)^C]`
- [x] Replace placeholder coefficients in `baseEmissionTable` with normative Tabelle 3 values (Pkw, Lkw1, Lkw2)
- [x] Define Krad coefficients: standard treatment is Lkw2 emission with Pkw speed (note in §3.3.2); document in code
- [x] Remove `RollingNoiseCoeffs` and `PropulsionNoiseCoeffs` structs (now superseded by single Eq. 6 formula)
- [x] Add unit tests for Eq. 6 at boundary speeds (30, 60, 80, 100, 130 km/h) against Table-3-derived reference values

### Section 3.3.2 — Längenbezogener Schallleistungspegel (Eq. 4 / Tabelle 2)

Eq. 4: `L_w' = 10·lg[M] + 10·lg[Σ_vg (share_vg · 10^(0.1·L_{W,FzG}(v_vg)) / v_vg)] – 30`
The `–30` is a normalization term; the `v_vg` divisor converts from per-vehicle to length-related power.
Current `emissionForPeriod` computes `lwA + 10·lg(count)` per group and sums — missing the `/v_vg` and `–30` terms.

- [x] Verify whether current emission formula matches Eq. 4 exactly (the `/v_vg – 30` structure)
- [x] If not, correct `emissionForPeriod` to match Eq. 4 including the `v_vg` divisor and `–30` normalization
- [x] Verify total-M + p1/p2 representation is equivalent to per-group counts as currently modelled
- [x] Expose a `DTVToHourly` helper implementing Tabelle 2 (DTV → M, p1, p2 by Straßenart for day/night)
  - Bundesautobahnen/Kraftfahrstraßen: day M=0.0555·DTV, p1=3%, p2=11%; night M=0.0140·DTV, p1=10%, p2=25%
  - Bundesstraßen: day M=0.0575·DTV, p1=3%, p2=7%; night M=0.0100·DTV, p1=7%, p2=13%
  - Landes-/Kreis-/Gemeindeverbindungsstraßen: day M=0.0575·DTV, p1=3%, p2=5%; night M=0.0100·DTV, p1=5%, p2=6%
  - Gemeindestraßen: day M=0.0575·DTV, p1=3%, p2=4%; night M=0.0100·DTV, p1=3%, p2=4%

### Section 3.3.1 — Modellierung einer Straße (Bild 6)

Source line positioning rules depend on lane count (per Bild 6):
1 lane → over lane center; 2 lanes → over outer lane; 3–4 lanes → over Trennlinie; 5+ lanes → over second outermost lane.
For bidirectional roads with different surface types per direction: use the larger correction per vehicle class (Pkw, Lkw).

- [x] Document source line positioning rules in code (currently user-specified, no validation)
- [x] Add optional `LaneCount` field to `RoadSource` to enable automatic source line offset calculation
- [x] Validate that multi-surface-type roads use the per-direction maximum correction rule

### Section 3.3.5 — Straßendeckschichtkorrektur (Tabelle 4a / 4b)

Tabelle 4a corrections are speed-dependent (≤60 vs >60 km/h), apply separately to Pkw and Lkw (Lkw1+Lkw2 share one correction), and do NOT apply to Pflasterbeläge.
Tabelle 4b covers Pflasterbeläge with three speed thresholds (30, 40, ≥50 km/h), no vehicle-group distinction.

The current `surfaceCorrectionTable` has single values per vehicle group (not speed-dependent for standard asphalt types) — values are placeholders.

- [x] Restructure `SurfaceCorrectionEntry` to hold `(PkwLow, PkwHigh, LkwLow, LkwHigh float64)` (low = ≤60, high = >60)
- [x] Populate Tabelle 4a normative values (outside Pflaster) for all surface types:
  - SMA 5/8 (v≤60 only): Pkw –2.6, Lkw –1.8
  - SMA 8/11 (v>60 only): Lkw –1.8/Lkw –2.0 (check direction-specific values in table)
  - AB ≤ AC 11: Pkw –2.7/–1.9, Lkw –1.9/–2.1
  - OPA PA 11: Pkw –4.5/–4.5, Lkw –4.4/–4.4
  - OPA PA 8: Pkw –5.5/–5.5, Lkw –5.4/–5.4
  - Beton (Waschbeton): Pkw –1.4/–1.4, Lkw –2.3/–2.3
  - Lärmarmer Gussasphalt: Pkw –2.0/–2.0, Lkw –1.5/–1.5
  - LOA (AC D LOA): Pkw –3.2 (v≤60 only), Lkw –1.0 (v≤60 only)
  - SMA LA 8: Pkw –2.8 (v≤60 only), Lkw –4.6 (v>60 only)
  - DSH-V 5: Pkw –3.9/–2.8, Lkw –0.9/–2.3
  - (cells with × in table = that speed range not applicable for that surface)
- [x] Populate Tabelle 4b normative values for Pflasterbeläge:
  - Pflaster eben (b≤5.0 mm, b+2f≤9.0 mm): +1.0/+2.0/+3.0 (at 30/40/≥50 km/h)
  - Sonstiges Pflaster (b>5.0 mm or f>2.0 mm, or Kopfsteinpflaster): +5.0/+6.0/+7.0
- [x] Update `SurfaceCorrection(st, vg, speedKPH)` signature to accept speed
- [x] Add unit tests for each Tabelle 4a/4b row at relevant speed thresholds

### Section 3.3.6 — Längsneigungskorrektur (Eqs. 7a / 7b / 7c)

Normative formulas are speed-dependent and differ per vehicle group.

- [x] Replace `GradientCorrection(g, vg)` with `GradientCorrection(g, vg, speedKPH)` implementing the three normative formulas:
  - Pkw (7a): `(g+6)/–6 · (90–min(v_Pkw,70))/20` for g < –6; `(g–2)/10 · (v_Pkw+70)/100` for g > +2; else 0
  - Lkw1 (7b): `(g+4)/–8 · (v_Lkw1–20)/10` for g < –4; `(g–2)/10 · v_Lkw1/10` for g > +2; else 0
  - Lkw2 (7c): `(g+4)/–8 · (v_Lkw2–10)/10` for g < –4; `(g–2)/10 · (v_Lkw2+10)/10` for g > +2; else 0
  - Clamp g to [–12, +12] before applying formulas
- [x] For Krad: use Pkw formula 7a with v_Pkw (per §3.3.2 Krad note)
- [x] Add unit tests at key gradient values for each vehicle group at representative speeds (Eqs. 7a/7b/7c verified)

### Section 3.3.7 — Knotenpunktkorrektur (Eq. 8 / Tabelle 5)

Normative formula: `D_{KKT}(x) = K_KT · max(1 – x/120, 0)` with K_KT: signalized=3 dB, roundabout=2 dB, other=0 dB.

- [x] Replace step-table `junctionCorrectionTable` with Eq. 8 continuous formula
- [x] Set K_KT for `JunctionOther` to 0 dB (currently table had non-zero values)
- [x] Verify decay reaches 0 at exactly x = 120 m for signalized/roundabout
- [x] Add unit tests at x = 0, 60, 120 m for signalized; roundabout at 40 m; Other always 0

### Section 3.3.8 — Mehrfachreflexionszuschlag (Eq. 9, corrected)

Normative formula: `D_refl(h_Beb, w) = min(2·h_Beb/w, 1.6)` (Korrekturblatt Feb 2020 only corrects subscript, not value).
Conditions: parallel walls ≤100 m apart; walls treated as parallel if angle ≤5° to road axis.
Implemented: `ReflectionSurchargeDB` removed; replaced by `BuildingHeightM`/`StreetWidthM` fields with internal Eq. 9 computation.

- [x] Add `BuildingHeightM` and `StreetWidthM` fields to `RoadSource` for Mehrfachreflexion inputs
- [x] Compute `D_refl` from Eq. 9 internally (not as user-supplied dB value)
- [x] Apply only when both walls are present (not for one-sided open situations)
- [x] Deprecate/remove `ReflectionSurchargeDB` field after migration
- [x] Add unit tests for D_refl at h_Beb/w ratios covering the clamp at min(2·h/w, 1.6)

### Section 3.4 — Parkplätze (missing source type)

Eq. 10 (corrected): `L_W'' = 63 + 10·lg[N·n] + D_{P,PT} – 10·lg[P/1m²]`
with Tabelle 6: D_P,PT = Pkw 0 dB, Motorrad 5 dB, Lkw/Omnibus 10 dB.
Tabelle 7 standard values for N: P+R (day 0.3, night 0.06), Tank/Rastanlage (day 1.5, night 0.8).

- [ ] Add `ParkingSource` type to model with fields: `AreaM2`, `NumSpaces`, `ParkingType`, `MovementsPerSpacePerHour` (day/night)
- [ ] Implement Eq. 10 `ComputeParkingEmission(source ParkingSource) (float64, error)`
- [ ] Add `ParkingType` enum: `Pkw`, `Motorrad`, `LkwOmnibus` with Tabelle 6 surcharges
- [ ] Expose Tabelle 7 standard N values as `DefaultMovementsPerHour(pt ParkingType, period TimePeriod) float64`
- [ ] Integrate parking contribution into Eq. 3 (`L_r''`) and Eq. 1 (`L_r`) computation
- [ ] Add `ParkingSources []ParkingSource` to `PropagationConfig` or compute entrypoint
- [ ] Add unit tests for Eq. 10 against hand-calculated reference values (at least one Pkw and one Lkw case)

### Section 3.5 — Schallausbreitung (Eqs. 11–17)

- [ ] Verify `D_div` (Eq. 12): `20·lg[s] + 10·lg[2π]` — the `2π` factor (not `4π`) accounts for ground reflection; confirm implementation
- [ ] Verify `D_atm` (Eq. 13): `s/200` — confirm no additional frequency-dependent term
- [ ] Verify `D_gr` (Eq. 14): `max(4.8 – h_m/s · (34 + 600/s), 0)` — confirm h_m computed as F/s_gr (area / ground distance per Bild 11)
- [ ] Verify `D_z` (Eq. 15): `10·lg[3 + 80·z·K_w]` with `K_w = exp(–1/2000 · sqrt(A·B·s / (2·z)))` (Eq. 17)
- [ ] Verify `z = A + B + C – s` (Eq. 16) including multi-diffraction C term (Gummibandmethode per Probst 2010)
- [ ] Confirm rule `max{D_gr; D_z}` in Eq. 11 (ground attenuation and shielding are not additive; only the larger applies)

### Section 3.6 — Berücksichtigung von Reflexionen (Tabelle 8)

- [ ] Verify reflection condition: h_R ≥ 1.0 m **and** h_R ≥ 0.3·√(a_R) (where a_R = smaller of source–reflector and reflector–receiver distances)
- [ ] Implement `ReflectorType` enum mapping to Tabelle 8 loss values: `FacadeOrReflecting`=0.5 dB, `ReflectionReducing`=3.0 dB, `StronglyReflectionReducing`=5.0 dB
- [ ] Apply D_RV1 to first reflection and D_RV2 to second reflection (per Eqs. 2 and 3 / Tabelle 8)
- [ ] Only the active Teilstück (the segment from which sound rays pass through the reflector) contributes as mirror source (Bild 14)
- [ ] Verify up to 2nd-order reflections handled; 3rd-order reflections are ignored per standard

### General accuracy and test coverage

- [ ] Run full TEST-20 suite after each formula fix to detect regressions
- [ ] Add golden scenarios for the standard test cases from Diagramm I (single vehicle, speed sweep) to lock in Grundwert values
- [ ] Add golden scenarios for Tabelle 4a surface corrections (DStrO sweep per vehicle group and speed)
- [ ] Add golden scenario for Section 3.4 parking lot case (verify Eq. 10 end-to-end)
- [ ] Update `docs/conformance/rls19-konformitaetserklaerung.md` with accurate scope after formula reconciliation
- [ ] Note Korrekturblatt 2/2020 fixes applied: (1) Eq. 3 corrected form, (2) D_refl subscript corrected, (3) Eq. 10 corrected form

---

## Phase 20 — Schall 03: standards-faithful implementation

Status: **complete** — all normative formulas (Gl. 1–36), all tables (1–18), all source types implemented.

### Completed scope (Phases 20–20d)

| Sub-phase | Scope               | Key deliverables                                                                                                         |
| --------- | ------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| **20**    | Eisenbahn Strecke   | Beiblatt 1 (Fz 1–10), Tables 4–9/11/17, emission Gl. 1–2, propagation Gl. 8–16, assessment Gl. 29–34                     |
| **20a**   | Straßenbahnen       | Beiblatt 2 (Fz 21–23), Tables 12–16, Gl. 37–38, K_S=+5 dB                                                                |
| **20b**   | Rangierbahnhöfe     | Beiblatt 3, Table 10, Gl. 3–7 (point/line/area sources), Gl. 30 (yard immission), Gl. 35–36 (combined assessment), C₂=20 |
| **20c**   | Reflections         | Table 18 (wall absorption), Gl. 27 (Fresnel), Gl. 28 (image sources), up to 3rd order                                    |
| **20d**   | Barrier diffraction | Gl. 17–26 in pipeline, Gummibandmethode, lateral diffraction, barriers on direct + reflected paths, unified scene API    |

**Entry point:** `ComputeNormativeReceiverLevelsWithScene(receiver, segments, walls, barriers)` — full normative pipeline with reflections and barriers.

### Remaining items

- [x] Nr. 5.3.2 permanently slow section exception (≤ 30 km/h) — `PermanentlySlow` flag on TrackSegment/StreckeEmissionInput
- [x] BImSchV source document conformance audit — Phase 37
- [ ] Section 9 measurement-based vehicle data — custom vehicle acoustics from measurements (input pathway, not computation gap; out of scope)
- [ ] End-to-end report/export checks
- [ ] Golden snapshot conformance scenarios for Phase 20d barrier diffraction

### Conformance

Conformance declaration: `docs/conformance/schall03-konformitaetserklaerung.md`
CI-safe test suite: `backend/internal/qa/acceptance/schall03/testdata/ci_safe_suite.json`

### References (Phase 20d barrier diffraction)

- Schall 03 Section 6.5 (Gl. 17–26), ISO 9613-2:1996 Section 7.4, CNOSSOS-EU Section 2.5.5
- Maekawa (1968) "Noise reduction by screens", Pierce (1974) "Diffraction of sound around corners"

---

## Phase 21 — ISO 9613-2: engineering-ready implementation

Status: preview baseline shipped (industry point sources, LpAeq, favorable meteorology).

- [x] Module structure, standards-framework descriptor, source emission interface, propagation chain, CLI wiring, receiver tables, rasters, provenance, unit tests, validation projects, golden scenarios.

### Open items — implementation

- [ ] Ensure module separation so normative outputs stay normative and standard-specific
- [ ] Define typed source schemas for ground/terrain inputs and meteorological assumptions
- [ ] Define import/run mapping from normalized GeoJSON + run parameters into typed ISO 9613-2 inputs
- [ ] Keep deterministic behavior for segmenting, attenuation ordering, and energetic summation
- [ ] Ensure results remain clearly separated from CNOSSOS / national-track outputs
- [ ] Expand from point-source-only to line/area source support where needed

### Open items — conformance

- [ ] Add implementation notes and legal/compliance notes
- [ ] Identify public example cases or create synthetic validation cases
- [ ] Document tolerances and comparison rules for validation cases
- [ ] Identify which coefficients/tables should live in an external standards data pack
- [ ] Publish ISO 9613-2 conformance boundary document (scope, tolerances, known deviations)

---

## Phase 22 — TA Lärm assessment layer (NEW)

**Goal:** implement TA Lärm as a **regulatory assessment and reporting layer**, not as a propagation standard.

**Why:** TA Lärm is the central administrative regulation (Verwaltungsvorschrift) for industrial noise protection in Germany. It defines assessment logic, area categories, time periods, surcharges, and thresholds — on top of propagation results from ISO 9613-2, CNOSSOS-Industry, or other standards. Without TA Lärm assessment logic, Aconiq cannot produce legally compliant industrial noise reports for German authorities.

- [ ] Define TA Lärm scope and implementation boundary
  - [ ] Clarify which TA Lärm sections are implemented (Nrn. 3-7 assessment, Anhang threshold tables)
  - [ ] Separate assessment/evaluation logic from underlying propagation standards
  - [ ] Document what is normative assessment vs. informational guidance
- [ ] Implement TA Lärm assessment logic
  - [ ] Area categories (Gebietskategorien) and associated threshold tables (Immissionsrichtwerte)
  - [ ] Time periods: Tag (06-22), Nacht (22-06), lauteste Nachtstunde
  - [ ] Zuschläge: Ton-, Impuls-, Informationshaltigkeit
  - [ ] Vorbelastung / Zusatzbelastung / Gesamtbelastung accounting
  - [ ] Relevanzprüfung (irrelevance criterion)
  - [ ] Spitzenpegelkriterium (peak level criterion)
- [ ] Implement TA Lärm reporting
  - [ ] Structured assessment result (per receiver: Beurteilungspegel vs. Richtwert, pass/fail)
  - [ ] German-language assessment text blocks for gutachterliche Stellungnahme
  - [ ] Export as part of report context (JSON + Typst/HTML rendering)
- [ ] Add verification coverage
  - [ ] Synthetic test cases for each assessment pathway
  - [ ] Golden scenarios for typical industrial assessment cases
  - [ ] Document TA Lärm version/edition and mapping to norm editions used

---

## Phase 23 — 16. BImSchV assessment layer (NEW)

**Goal:** implement the 16. BImSchV as a regulatory assessment layer for traffic noise (road and rail).

**Why:** The 16. BImSchV (Verkehrslärmschutzverordnung) is the legal framework for traffic noise assessment in Germany. Schall 03 (rail) and RLS-19 (road) are its computational annexes. Without the assessment layer, Aconiq produces propagation results but cannot generate legally compliant traffic noise assessments.

- [ ] Define 16. BImSchV scope
  - [ ] Clarify which sections and annexes are covered
  - [ ] Define how RLS-19 and Schall 03 results feed into assessment
- [ ] Implement assessment logic
  - [ ] Threshold tables (Immissionsgrenzwerte) per area category and time period
  - [ ] Combined assessment for road + rail (Gesamtlärmpegel where required)
  - [ ] Anspruch auf Lärmschutzmaßnahmen determination
- [ ] Implement reporting
  - [ ] Structured assessment result per receiver
  - [ ] German-language result text blocks
  - [ ] Integration into report context and export bundles
- [ ] Add verification coverage

---

## Phase 24 — Interoperability: export formats (MEDIUM PRIORITY)

**Goal:** export results in formats expected by authorities and planning workflows.

**Why:** Aconiq uses internal raster/table containers (float64 LE + JSON sidecar). Authorities and planners expect GeoTIFF rasters, GeoPackage vectors, and contour line exports. Without standard exports, results cannot be integrated into third-party GIS workflows.

- [x] GeoTIFF / COG raster export
  - [x] Export result rasters as GeoTIFF with embedded CRS metadata
  - [x] Support Cloud Optimized GeoTIFF (COG) profile for web/GIS compatibility
  - [x] Preserve indicator metadata in TIFF tags or sidecar
- [x] GeoPackage vector export
  - [x] Export receiver tables as GeoPackage with attributed points
  - [x] Export model features as GeoPackage for archival/exchange
- [x] Contour line generation and export
  - [x] Generate ISO-band contour lines from raster results (5 dB steps per EU END convention)
  - [x] Export contours as GeoJSON and GeoPackage
  - [ ] ~~Add contour overlays and labels in frontend map view~~ (deferred — frontend inactive)
- [x] Define export format matrix (which formats are required vs. optional per use case)

---

## Phase 25 — DACH reporting templates (MEDIUM PRIORITY — new)

**Goal:** produce German-language noise assessment reports in formats expected by authorities and courts.

**Why:** DACH adoption requires reports that follow established gutachterliche conventions. Commercial tools score heavily on report workflow — Aconiq's differentiator is reproducible, auditable report generation from deterministic artifacts.

- [ ] Define DACH report template requirements
  - [ ] TA Lärm Gutachten template (industry assessment)
  - [ ] 16. BImSchV Gutachten template (traffic noise assessment)
  - [ ] Generic Schallimmissionsprognose template
- [ ] Implement Typst templates for DACH reports
  - [ ] Cover page, table of contents, standard sections (Aufgabenstellung, Grundlagen, Beurteilungsgrundlagen, Berechnungsverfahren, Ergebnisse, Beurteilung)
  - [ ] Embedded result tables, maps, and contour plots
  - [ ] Provenance/reproducibility section (standard version, data pack version, input hashes)
- [ ] Add German-language text generation for assessment results
  - [ ] Threshold comparison tables with pass/fail per receiver/area
  - [ ] Condition text blocks (Auflagen, Nebenbestimmungen suggestions)
- [ ] PDF golden/snapshot checks in CI

### Research

- [ ] Define template/versioning policy for backward-compatible report styles
- [ ] Survey typical Gutachten structure from published examples

---

## Phase 26 — CityGML import completion

Status: **complete** — building import hardened with attribute preservation, structured import report, and documented decisions.

### Decisions

- [x] CityGML versions: both 2.0 and 3.0 accepted (namespace-based auto-detection in go-citygml)
- [x] CRS / axis-order: handled by Phase 18 pipeline (auto-detect from srsName, transform via wroge/wgs84)
- [x] Terrain, barriers, bridges, tunnels: deferred — terrain comes from DTM files; others rarely appear in noise-relevant CityGML
- [x] BuildingPart: deferred (needs upstream go-citygml library work)

### Completed items

- [x] Preserve CityGML attributes (`citygml_class`, `citygml_function`, `citygml_usage`, `citygml_lod`) as informational properties
- [x] Structured import report: total/imported/skipped counts with per-building skip reasons
- [x] Import report written to `.noise/model/citygml-import-report.json` and printed to CLI
- [x] Skip reasons: no height, invalid height, no footprint, degenerate polygon
- [x] Imported features remain compatible with standards-specific extraction (via modelgeojson normalization)

---

## Phase 27 — QA hardening (test catalogs, fuzzing, drift tracking)

**Goal:** make correctness and reproducibility measurable.

- [ ] Expand `internal/qa/`
  - [ ] Loaders for standard test tasks
  - [ ] Result comparison with tolerances + outlier reports
  - [ ] Snapshot exporter for debugging
- [ ] Expand fuzz/property tests
  - [ ] Geometry robustness
  - [ ] Numeric monotonicity properties (where applicable)
- [ ] Numeric drift tracking (benchmarks + comparisons across commits)
- [ ] "Repro bundle" export: run + inputs + standard/profile in one package

---

## Phase 28 — Performance & scaling (city-scale)

**Goal:** large receiver grids and many sources perform well.

- [ ] Optimize tiled compute pipeline
  - [ ] Spatial index tuning
  - [ ] Candidate pruning
  - [x] Broader cache keys and reuse for equivalent workloads
- [ ] Robust disk-backed cache + cleanup strategies (partially complete, see below)
  - [x] Per-run and shared keyed chunk cache on disk
  - [x] Benchmark suite cache cleanup via `noise bench --keep-last`
  - [x] General cache retention/cleanup policy and stale-cache invalidation
- [x] `noise bench` with standard scenarios, runtime/memory/IO/drift output, summary persistence, warm-cache reuse
  - [ ] Numeric drift comparison across multiple worker/topology variants

### Optional (advanced, non-normative)

- [ ] algo-fft/algo-dsp for non-normative post-processing pipelines
- [ ] `algo-pde` for research-only wave/low-frequency propagation experiments
- [ ] WebAssembly delivery for interactive research/demo modules

---

## Phase 29 — Conformance packages per standard (NEW)

**Goal:** publish per-module conformance packages that establish trust with practitioners and authorities.

**Why:** DACH acceptance depends less on feature lists and more on belastbare Nachweise. SoundPLAN communicates a "Konformitätserklärung" for RLS-19. Aconiq can match or exceed this with open, machine-readable conformance artifacts.

- [ ] Define conformance package structure (per standards module)
  - [ ] Supported scope / Sub-Scope (what is and isn't covered)
  - [ ] Tolerance rules and comparison methodology
  - [ ] Reference test cases (with provenance: source, version, license status)
  - [ ] Known deviations with rationale
  - [ ] Machine-readable conformance report (JSON)
- [ ] RLS-19 conformance package (leveraging TEST-20)
- [ ] Schall 03 conformance package
- [ ] ISO 9613-2 conformance package
- [ ] CNOSSOS family conformance packages (Road, Rail, Industry, Aircraft)
- [ ] Automate conformance report generation in CI (pass/fail gate per module)

---

## Phase 30 — Example projects & DACH onboarding (NEW)

**Goal:** provide ready-to-run DACH example projects for each major use case.

**Why:** The `examples/` directory is currently empty. Adoption requires users to see working examples before investing. Commercial tools offer training/support; Aconiq compensates with reproducible, self-documenting demo projects.

- [ ] Create synthetic, license-safe example projects
  - [ ] RLS-19 road corridor (16. BImSchV context)
  - [ ] Schall 03 rail section
  - [ ] ISO 9613-2 industrial point source (TA Lärm context)
  - [ ] Combined road + rail scenario
- [ ] Each example includes: input data, run config, expected outputs, step-by-step README
- [ ] German-language "Getting Started" guide
- [ ] Add example project CI jobs (ensure examples stay green across releases)

---

## Phase 31 — Community & release engineering (NEW)

**Goal:** establish visible project health and contribution paths for external adoption.

**Why:** Zero stars/forks/issues signal a closed project. Practitioners and institutions need signals of active maintenance, clear versioning, and a path to contribute or report bugs.

- [ ] Versioning and changelog process (SemVer + CHANGELOG.md)
- [ ] Build release binaries (CLI; desktop optional) with GitHub Releases
- [ ] Enable GitHub Issues with issue templates (bug, feature request, conformance question)
- [ ] Public roadmap (this PLAN.md or a GitHub project board)
- [ ] Documentation site
  - [ ] Getting started (EN + DE)
  - [ ] Project format spec
  - [ ] Standards modules overview + status + conformance boundaries
  - [ ] QA/acceptance process and tolerances
- [ ] Release-tag golden test gates in CI
- [ ] German-language community presence (consider blog post, conference talk, or Fachzeitschrift article)

---

## Phase 32 — Frontend hardening & deferred UI features

**Goal:** complete remaining frontend features and polish.

### Deferred from earlier phases

- [ ] WebSocket support for progress streaming (optional, SSE works)
- [ ] TypeScript client generation pipeline for frontend API types
- [ ] E2E smoke flow API-side (headless): import → validate → run → export
- [ ] Box select and multi-select support on map
- [ ] Contour overlays and labels on result map
- [ ] Contribution breakdown per receiver / selected result
- [ ] Run-to-run diff layer
- [ ] Scenario change-set summary for model/parameter differences
- [ ] Performance guardrails for large feature counts (clustering/tile fallback)
- [ ] Building footprints/import pipelines beyond GeoJSON

### Per-source acoustics hardening

- [ ] UI-level interaction coverage for editing, clearing, and restoring per-source acoustic overrides
- [ ] Surface per-source acoustic overrides / inferred review flags in feature popups or run setup summaries
- [ ] Decide whether additional OSM-derived defaults are deterministic enough to enable
- [ ] Define follow-on source-editing scopes for other standards/modules

---

## Phase 33 — Tiling/PMTiles (deferred)

- [ ] Evaluate vector tiles for model/results
- [ ] Evaluate PMTiles end-to-end pipeline
- [ ] Define storage/size budgets

---

## Phase 34 — Desktop packaging (Wails, optional)

- [ ] Make the API runnable in-proc (no port needed)
- [ ] Embed frontend assets into Go binary
- [ ] Define build targets (`web` vs `wails`)
- [ ] Smoke tests for desktop build
- [ ] Re-check Wails v3 maturity and define fallback options

---

## Phase 35 — Project format v2 (multiuser/server, optional)

- [ ] Map data model to PostGIS (geometries, indexes)
- [ ] Store artifacts in object storage (rasters/tiles/reports)
- [ ] Minimal auth/users (only if required)
- [ ] Migration tool: v1 project → v2

---

## Phase 36 — Interoperability: SoundPlan project import & cross-validation

Status: Steps 1–2 complete, Step 3 in progress (core .abs result parsing done, RRAI/RRAD and grid maps pending).

**Goal:** import SoundPlan projects (`.sp` + associated geometry/data files) into Aconiq's internal model, run the same calculations, and compare results against SoundPlan's computed outputs. This enables cross-tool validation and provides a migration path for practitioners switching from SoundPlan.

**Why:** SoundPlan is the dominant DACH noise calculation tool. Many practitioners have existing project archives in SoundPlan format. Being able to (a) load their geometry and emission data, (b) re-calculate in Aconiq, and (c) compare results against SoundPlan's outputs builds trust in Aconiq's conformance and lowers the adoption barrier. It also provides a rich source of real-world validation scenarios beyond synthetic test cases.

### SoundPlan format overview

A SoundPlan project is a directory containing:

- `Project.sp` — INI-style text file (Windows-1252 encoding). Contains project metadata, enabled standards, calculation parameters, assessment periods, land use categories, and geometric defaults.
- `*.geo` — Custom binary geometry files with tagged records (markers `:HZ`, `:G `, `:D1`, `:DL`, `:O&`, etc.). Contain coordinates as float64 LE pairs, object names, emission parameters, and embedded BMP preview thumbnails. Key files: `GeoRail.geo` (rail tracks), `GeoObjs.geo` (buildings/objects), `GeoWand.geo` (noise barriers), `GeoTmp.geo` (terrain contours), `CalcArea.geo` (calculation area).
- `*.res` — INI-style result metadata per calculation run. Contains SoundPlan version, run type, timestamps, referenced geometry files, calculation parameters, assessment definitions (time periods, limit values).
- `*.abs` — Binary data tables (fixed-size records). Used for addresses, emission attributes, and result data (immission levels, partial levels, frequency spectra).
- `*.dgm` — Binary digital ground model.
- `*.ntd` — Immission point tables.
- `*.ets` — Report/print templates.
- `*.esn` — Noise type definitions.
- `Höhen.txt` — ASCII elevation data (semicolon-separated, German decimal commas: `x; y; z; code`).

### Step 1 — Parse Project.sp and result metadata

- [x] Implement `Project.sp` parser (INI with Windows-1252 encoding)
  - [x] Extract project metadata: title, version, description
  - [x] Extract enabled standards and calculation type selectors (`[ENABLEDSTANDARDS]`, `[RAIL]`, `[ROAD]`, `[INDU]`)
  - [x] Parse calculation parameter strings (e.g., `@2:20490 AIR0 BME1000 BMM1000 MP1013 MF70 MT10 ML0:1 ...`)
  - [x] Extract assessment periods (`[TIME SLICES DEN]`: Tag/Nacht hours, limit values)
  - [x] Extract geometric defaults (`[GEODB]`, `[SIMPLESETTINGS]`: receiver height, floor height, reflection order, rail bonus)
- [x] Implement `.res` file parser (INI format)
  - [x] Extract run metadata: run type, timestamps, SoundPlan version, thread count
  - [x] Extract referenced geometry files and their modification timestamps
  - [x] Extract assessment definitions (ZB1/ZB2: time periods, hourly masks, limit arrays)
  - [x] Extract calculation command strings for standard/parameter reconstruction

### Step 2 — Reverse-engineer and parse binary geometry files

- [x] Investigate `.geo` binary format in detail
  - [x] Document the tagged record structure (`:HZ` header, `:G ` geometry points, `:D1` descriptors, `:DL` data links, `:O&` object groups, etc.)
  - [x] Determine coordinate encoding (float64 LE confirmed for rail), bounding box structure, record length fields
  - [x] Identify how object type (building, barrier, rail, terrain) is encoded vs. inferred from filename
  - [x] Handle embedded BMP thumbnails (skip or extract)
- [x] Implement `GeoRail.geo` parser
  - [x] Extract rail track polylines with coordinates and elevations
  - [x] Extract track names and identifiers (e.g., "Hauptstrecke Gleis 1")
  - [x] Extract per-track emission parameters (speed, corrections, bridge surcharges)
- [x] Implement `GeoObjs.geo` parser
  - [x] Extract building footprints/polygons (315 closed polygons, type 0x03ec)
  - [ ] Extract building addresses and attributes (type 0x03e9 with :D1 name records — deferred)
  - [x] Extract receiver/immission point positions (77 points, type 0x0028)
- [x] Implement `GeoWand.geo` parser
  - [x] Extract barrier/wall polylines with heights and top geometry (type 0x03eb, per-point height in z2 field)
  - [ ] Extract barrier material/absorption properties (:D! records with dB values — deferred)
- [x] Implement `GeoTmp.geo` parser (terrain)
  - [x] Extract 26603 elevation points (type 0x040b) and 7 contour/terrain lines (types 0x040a, 0x046e)
  - [ ] Extract digital ground model from `.dgm` binary — deferred
  - [ ] Fallback: import `Höhen.txt` ASCII elevation points (semicolon-separated, German decimals) — deferred
- [x] Implement `CalcArea.geo` parser
  - [x] Extract calculation area rectangle/polygon (type 0x03ff, closed 5-point rectangle)

### Step 3 — Parse binary result/data tables

Uses `github.com/meko-tech/go-absolute-database` (local `../go-absolute-database`) for .abs file I/O.

- [x] Investigate `.abs` binary format
  - [x] Document record structure, header format, field layout — handled by go-absolute-database library (page-based I/O, zlib-compressed schema, auto-detected record layout)
  - [x] Identify how result columns map to indicators (Lr,Tag, Lr,Nacht, partial levels, frequency spectra) — schema introspection via `absdb dump`
- [x] Implement result `.abs` parser for single-point results (`RSPS*/RREC*.abs`, `RGRP*.abs`, `RMPA*.abs`)
  - [x] Extract per-receiver immission levels (day/night) — `ParseGroupResults` reads ZB1/ZB2 from RGRP files (verified: 30 results, levels 48–68 dB)
  - [x] Extract per-receiver partial levels and source contributions — `ParsePartialResults` reads RMPA files (516 records with source names and assessment periods)
  - [x] Extract receiver metadata — `ParseReceiverResults` reads RREC files (27 receivers with names, ObjIDs, floor numbers)
  - [x] Extract train type catalog — `ParseTrainTypes` reads TS03.abs (18 train types with SBA, Vmax, DFz)
  - [x] `LoadRunResults` loads all result files from a run subdirectory
- [ ] Implement result `.abs` parser for RRAI/RRAD emission data (record layout detection issue for v7.61 files — deferred pending go-absolute-database improvement)
- [ ] Implement result `.abs` parser for grid map results (`RRLK*/RRLK*.GM`)
  - [ ] Extract raster metadata (grid origin, spacing, dimensions)
  - [ ] Extract raster level values
- [ ] Implement `.ntd` parser (immission point table)

### Step 4 — Map to Aconiq internal model

- [ ] Define mapping from SoundPlan standard IDs to Aconiq standards modules
  - [ ] Map `20490` (Schall 03 rail) → `schall03` module
  - [ ] Map `10490` (RLS-19 road) → `rls19` module
  - [ ] Map `30000` (ISO 9613-2 industry) → `iso9613` module
  - [ ] Identify unsupported standards and emit clear warnings
- [ ] Convert SoundPlan rail geometry → Aconiq `TrackSegment` + `TrainOperation`
  - [ ] Map SoundPlan track parameters to Aconiq emission model fields
  - [ ] Map SoundPlan Zugarten/train types to Aconiq Fz-Kategorien
- [ ] Convert SoundPlan buildings → Aconiq building features
- [ ] Convert SoundPlan barriers → Aconiq barrier features (including reflecting walls where applicable)
- [ ] Convert SoundPlan terrain → Aconiq terrain model
- [ ] Convert SoundPlan receivers → Aconiq receiver set
- [ ] Convert SoundPlan calculation area → Aconiq grid configuration
- [ ] Handle CRS: determine SoundPlan project coordinate system (likely local or Gauss-Krüger) and transform via Phase 18 pipeline

### Step 5 — Cross-validation workflow

- [ ] Implement `noise import --from-soundplan <project-dir>` CLI command
  - [ ] Load and parse SoundPlan project
  - [ ] Create Aconiq project with mapped model, scenarios, and parameters
  - [ ] Emit import report: what was imported, what was skipped, warnings
- [ ] Implement `noise compare` or comparison mode
  - [ ] Load SoundPlan result data alongside Aconiq run results
  - [ ] Per-receiver level comparison (absolute difference, relative difference)
  - [ ] Raster difference map (Aconiq result minus SoundPlan result)
  - [ ] Summary statistics: mean/max/P95 deviation, number of receivers exceeding tolerance
  - [ ] Configurable tolerance threshold (e.g., ±0.5 dB for conformance, ±1.0 dB for info)
- [ ] Generate cross-validation report artifact
  - [ ] Tabular comparison per receiver point
  - [ ] Deviation histogram / distribution
  - [ ] Map overlay showing spatial deviation pattern
  - [ ] Provenance: SoundPlan version, Aconiq version, standard, parameters

### Step 6 — Test coverage and conformance

- [ ] Add unit tests for each parser (Project.sp, .geo, .abs, .res)
- [ ] Add integration test with the included sample project (`interoperability/Schienenprojekt - Schall 03/`)
  - [ ] Verify geometry extraction matches expected track/building/barrier counts and positions
  - [ ] Verify parameter mapping produces correct Aconiq scenario configuration
- [ ] Add cross-validation acceptance test
  - [ ] Import sample project → run Aconiq calculation → compare against SoundPlan results
  - [ ] Define acceptable tolerance for the sample project (document any known deviations)
- [ ] Document SoundPlan format findings in `docs/research/soundplan-format.md`

### Research / open questions

- [ ] SoundPlan format versioning: how stable is the binary `.geo` format across SoundPlan versions? (sample is v4.1 / VERSION=41080)
- [ ] Are there SoundPlan XML or text export options that could be easier to parse than the binary `.geo` format?
- [ ] Can SoundPlan export to CadnaA or other intermediate formats that are better documented?
- [ ] Investigate whether SoundPlan's "ASCII export" or "data exchange" features produce parseable intermediate files
- [ ] Legal: confirm that parsing a proprietary file format for interoperability is permitted (likely yes under EU interoperability directives, but document the position)

---

## Phase 37 — Schall 03 BImSchV conformance audit

Status: **complete**

Systematic audit of all normative tables and coefficients against the authoritative
BGBl source document (BGBl. Jahrgang 2014 Teil I Nr. 61, pp. 2269-2313).

### Fixes applied

| Bug | File | Description |
|-----|------|-------------|
| Fz4 m=7 DeltaA | beiblatt1.go | Spectrum shifted from 1000 Hz; correct: {-16,-9,-7,-7,-7,-9,-12,-19} |
| Gleisbremse i=6 | beiblatt3.go | Missing 63 Hz value (-56); spectrum shifted left |
| Gleisbremse i=8 | beiblatt3.go | Gummiwalkbremse had entirely wrong spectrum |
| Gleisbremse i=10 | beiblatt3.go | Schraubenbremse had extra -21 at 500 Hz |
| K_S combined formula | indicators.go | kSStrecke changed from -5.0 to 0.0 (abolished since 2015/2019) |

### Regression tests

`bimschv_audit_test.go` — pins all normative table values with BGBl page references.

### Verified correct (no changes needed)

Tables 3-9, 11-17; Beiblatt 1 Fz 1-3/5-10; Beiblatt 2 Fz 21-23;
Beiblatt 3 (except 4 Gleisbremse rows fixed above).

---

# Research backlog

## Standards & test data

- [ ] CNOSSOS Road/Rail/Industry/Aircraft: collect license-safe validation cases and define tolerances
- [ ] BUB/BUF/BEB: obtain current documents/annexes and define exact input requirements per module
- [ ] RLS-19 TEST-20: clarify sourcing, storage format, legal redistribution, and CI automation
- [ ] Schall 03: acquire license-safe verification cases; clarify redistribution rights for normative tables
- [ ] ISO 9613-2: identify public example cases or create synthetic ones
- [ ] TA Lärm: survey published Gutachten for structural conventions and assessment patterns
- [ ] 16. BImSchV: clarify combined assessment rules for road + rail

## GIS / CRS / formats

- [x] CRS/PROJ decision: pure-Go `wroge/wgs84` v1 chosen (no cgo, MIT, Helmert datum shifts)
- [ ] GeoTIFF export: dependency strategy for writing (existing reader is pure-Go)
- [ ] Contour generation: algorithm selection and quality requirements

## Determinism & tolerances

- [ ] Standardize numeric tolerances (per standard/test suite)
- [ ] Define stable summation strategy and document where it applies

## UX/workflow (lower priority while CLI-first)

- [ ] DTO generation strategy and backward compatibility policy
- [ ] Define "must-have" exports (GeoTIFF/CSV/PNG/report) and which are deferred
- [ ] Define map layer performance thresholds (feature count, tile fallback triggers)
- [ ] Define accessibility baseline for map-heavy interactions

---

# Priority order for DACH adoption

The recommended execution order, balancing impact and dependencies:

1. **Phase 17 — Legal & governance** (small effort, hard blocker)
2. **Phase 18 — CRS transformation pipeline** (large effort, hard blocker for real data)
3. **Phase 19-21 — Standards completion** (RLS-19 conformance, Schall 03 faithful, ISO 9613-2 engineering-ready — large effort, core value)
4. **Phase 22-23 — Assessment layers** (TA Lärm, 16. BImSchV — medium effort, required for legal reports)
5. **Phase 29 — Conformance packages** (medium effort, trust-building)
6. **Phase 24 — Export formats** (medium effort, interoperability)
7. **Phase 25 — DACH reporting templates** (medium effort, direct practitioner value)
8. **Phase 30 — Example projects & onboarding** (small-medium effort, adoption driver)
9. **Phase 31 — Community & release engineering** (ongoing, adoption infrastructure)
10. Everything else follows based on demand and resources.
