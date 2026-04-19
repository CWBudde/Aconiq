# Phase 11 CNOSSOS Rail Baseline

Status date: 2026-03-08

## Goal

Phase 11 brings the first planning-track CNOSSOS rail module online:
`cnossos-rail`.

The completion target for this phase is a shipped, deterministic, CLI-runnable
baseline that:

- accepts typed rail line sources
- computes `Lday`, `Levening`, `Lnight`, and `Lden`
- exports receiver tables and rasters
- records enough provenance and QA evidence to make the baseline reviewable

It is not a claim that the repository contains a full public conformance suite
for every legal detail of CNOSSOS-EU railway noise.

## Implemented Scope

### Standards module

- Module path: `backend/internal/standards/cnossos/rail`
- Registry ID: `cnossos-rail`
- Built-in model version: `phase11-preview-v2`
- Standards-framework descriptor present with:
  - planning context metadata
  - version/profile metadata
  - supported source types
  - supported indicators
  - validated run parameter schema

### Source model

The baseline supports typed rail line sources through `RailSource` with
validation for:

- geometry (`LineString`/`MultiLineString` after normalization)
- traction type
- track type
- track roughness class
- average train speed
- braking share
- curve radius
- bridge flag
- per-period traffic counts

Imported GeoJSON can provide these values per feature; run-level parameters act
as defaults.

### Emission baseline

The current rail module implements a deterministic preview emission path with:

- period-specific day/evening/night traffic handling
- rolling noise contribution
- traction noise contribution
- braking contribution
- infrastructure contribution

### Propagation baseline

The current rail module implements a deterministic line-source propagation path
with:

- fixed subsegment discretization
- geometric divergence
- air absorption
- ground attenuation term
- rail-specific contextual adjustments for:
  - bridge correction
  - curve squeal
- deterministic multi-source energetic summation

### Indicators and export

- Indicators:
  - `Lday`
  - `Levening`
  - `Lnight`
  - `Lden`
- Result export:
  - receiver tables: `receivers.json`, `receivers.csv`
  - raster sidecar/data pair: `cnossos-rail.json`, `cnossos-rail.bin`
- CLI wiring:
  - `aconiq run --standard cnossos-rail`
  - extraction from normalized GeoJSON
  - receiver-grid generation
  - result persistence
  - provenance metadata

## QA and Evidence

### In-repo deterministic QA

The repository contains deterministic QA coverage for the Phase 11 baseline:

- module unit tests for schema validation, emission behavior, propagation
  behavior, indicator aggregation, export, and provenance metadata
- CLI end-to-end coverage using
  `backend/internal/app/cli/testdata/phase11/rail_model.geojson`
- synthetic acceptance fixtures in `backend/internal/qa/acceptance/testdata/cnossos-rail/`
  for:
  - `rail_preview`
  - `rail_contextual`

These fixtures are repo-authored and license-safe, but they are regression
fixtures for our implementation rather than public normative reference cases.

### Public attributable evidence

Public rail evidence is documented in
`docs/research/cnossos-rail-public-reference-totals.md`.

That note records official Irish EPA Round 4 railway-noise reference totals
derived from publicly described CNOSSOS-EU strategic noise mapping outputs. The
evidence is suitable for attributable external benchmarking at the
reference-total level.

## Compliance Boundary

This Phase 11 baseline should be read as:

- a deterministic planning-track CNOSSOS rail preview implementation
- suitable for repository QA, regression control, and baseline public evidence
- explicit about what is implemented in code today

This Phase 11 baseline should not be read as:

- a claim of full legal conformance to every CNOSSOS rail annex detail
- a substitute for authority-issued or vendor-issued conformance packs
- a complete public scenario-level verification suite

Important current limits:

- source support is limited to rail line sources
- the propagation path is a compact baseline, not a full public conformance
  implementation of every CNOSSOS rail detail
- exported values remain raw `float64`; there is no separate user-facing
  reporting precision contract yet

## Completion Statement

Phase 11 is considered complete as a shipped baseline because the repository now
has:

- the online `cnossos-rail` module
- deterministic CLI integration and export
- documented rounding/tolerance behavior
- per-feature import extraction
- synthetic regression and acceptance evidence
- at least one attributable, license-safe public rail reference-total source

Future deeper conformance work remains valid engineering work, but it is no
longer a blocker for the completion of the Phase 11 baseline itself.
