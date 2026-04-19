# Phase 12 CNOSSOS Industry Baseline

Status date: 2026-03-08

## Goal

Phase 12 brings the first planning-track CNOSSOS industry module online:
`cnossos-industry`.

The completion target for this phase is a shipped, deterministic, CLI-runnable
baseline that:

- accepts typed industry point and area sources
- computes `Lday`, `Levening`, `Lnight`, and `Lden`
- exports receiver tables and rasters
- records enough provenance and QA evidence to make the baseline reviewable

It is not a claim that the repository contains a full public conformance suite
for every legal detail of CNOSSOS-EU industrial noise.

## Implemented Scope

### Standards module

- Module path: `backend/internal/standards/cnossos/industry`
- Registry ID: `cnossos-industry`
- Built-in model version: `phase12-preview-v2`
- Standards-framework descriptor present with:
  - planning context metadata
  - version/profile metadata
  - supported source types
  - supported indicators
  - validated run parameter schema

### Source model

The baseline supports typed industry sources through `IndustrySource` with
validation for:

- source geometry for `point` and `area` sources
- source height
- sound power level
- tonality correction
- impulsivity correction
- per-period operating factors for:
  - `day`
  - `evening`
  - `night`

Imported GeoJSON can provide these values per feature; run-level parameters act
as defaults.

### Emission baseline

The current industry module implements a deterministic preview emission path
with:

- period-specific operating-factor weighting
- sound-power-based emission calculation
- tonality correction
- impulsivity correction
- area-source scaling from plan area

### Propagation baseline

The current industry module implements a deterministic point/area-source
propagation path with:

- deterministic point/area source to receiver coupling
- geometric divergence
- air absorption
- ground attenuation term
- screening term
- baseline contextual adjustments for:
  - facade reflection
  - area-source near-field boost
- deterministic multi-source energetic summation

### Indicators and export

- Indicators:
  - `Lday`
  - `Levening`
  - `Lnight`
  - `Lden`
- Result export:
  - receiver tables: `receivers.json`, `receivers.csv`
  - raster sidecar/data pair: `cnossos-industry.json`, `cnossos-industry.bin`
- CLI wiring:
  - `aconiq run --standard cnossos-industry`
  - extraction from normalized GeoJSON
  - receiver-grid generation
  - result persistence
  - provenance metadata

## QA and Evidence

### In-repo deterministic QA

The repository contains deterministic QA coverage for the Phase 12 baseline:

- module unit tests for schema validation, emission behavior, propagation
  behavior, indicator aggregation, export, and provenance metadata
- CLI end-to-end coverage using
  `backend/internal/app/cli/testdata/phase12/industry_model.geojson`
- synthetic acceptance fixtures in
  `backend/internal/qa/acceptance/testdata/cnossos-industry/` for:
  - `industry_preview`
  - `industry_contextual`

These fixtures are repo-authored and license-safe, but they are regression
fixtures for our implementation rather than public normative reference cases.

### Public attributable evidence

Public industry evidence is documented in
`docs/research/cnossos-industry-public-reference-totals.md`.

That note records official Irish agglomeration action-plan exposure references
for industrial noise derived from publicly described Round 4 CNOSSOS-EU
strategic noise mapping outputs. The evidence is suitable for attributable
external benchmarking at the public reference-total and exposure-share level.

## Compliance Boundary

This Phase 12 baseline should be read as:

- a deterministic planning-track CNOSSOS industry preview implementation
- suitable for repository QA, regression control, and baseline public evidence
- explicit about what is implemented in code today

This Phase 12 baseline should not be read as:

- a claim of full legal conformance to every CNOSSOS industry annex detail
- a substitute for authority-issued or vendor-issued conformance packs
- a complete public scenario-level verification suite

Important current limits:

- source support is limited to point and area industry sources
- the propagation path is a compact baseline, not a full public conformance
  implementation of every CNOSSOS industry detail
- exported values remain raw `float64`; there is no separate user-facing
  reporting precision contract yet

## Completion Statement

Phase 12 is considered complete as a shipped baseline because the repository now
has:

- the online `cnossos-industry` module
- deterministic CLI integration and export
- documented rounding/tolerance behavior
- per-feature import extraction
- synthetic regression and acceptance evidence
- at least one attributable, license-safe public industry reference source

Future deeper conformance work remains valid engineering work, but it is no
longer a blocker for the completion of the Phase 12 baseline itself.
