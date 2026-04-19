# Phase 10 CNOSSOS Road Baseline

Status date: 2026-03-08

## Goal

Phase 10 brings the first real planning-track standards module online:
`cnossos-road`.

The completion target for this phase is a shipped, deterministic, CLI-runnable
baseline that:

- accepts typed road line sources
- computes `Lday`, `Levening`, `Lnight`, and `Lden`
- exports receiver tables and rasters
- records enough provenance and QA evidence to make the baseline reviewable

It is not a claim that the repository contains a full public conformance suite
for every legal detail of CNOSSOS-EU road traffic.

## Implemented Scope

### Standards module

- Module path: `backend/internal/standards/cnossos/road`
- Registry ID: `cnossos-road`
- Built-in model version: `phase10-preview-v2`
- Standards-framework descriptor present with:
  - planning context metadata
  - version/profile metadata
  - supported source types
  - supported indicators
  - validated run parameter schema

### Source model

The baseline supports typed line-road sources through `RoadSource` with
validation for:

- geometry (`LineString`/`MultiLineString` after normalization)
- road category
- surface type
- speed
- gradient
- junction context
- ambient temperature
- studded tyre share
- per-period traffic splits for:
  - light vehicles
  - medium vehicles
  - heavy vehicles
  - powered two-wheelers

Imported GeoJSON can provide these values per feature; run-level parameters act
as defaults.

### Emission baseline

The current road module implements a deterministic preview emission path with:

- period-specific day/evening/night traffic handling
- piecewise/table-style road traffic logic
- vehicle-class contributions
- baseline contextual corrections for:
  - junction proximity
  - temperature
  - studded tyres

### Propagation baseline

The current road module implements a deterministic line-source propagation path
with:

- fixed subsegment discretization
- geometric divergence
- air absorption
- ground attenuation term
- barrier attenuation term
- deterministic multi-source energetic summation

### Indicators and export

- Indicators:
  - `Lday`
  - `Levening`
  - `Lnight`
  - `Lden`
- Result export:
  - receiver tables: `receivers.json`, `receivers.csv`
  - raster sidecar/data pair: `cnossos-road.json`, `cnossos-road.bin`
- CLI wiring:
  - `aconiq run --standard cnossos-road`
  - extraction from normalized GeoJSON
  - receiver-grid generation
  - result persistence
  - provenance metadata

## QA and Evidence

### In-repo deterministic QA

The repository contains deterministic QA coverage for the Phase 10 baseline:

- module unit tests for schema validation, emission behavior, propagation
  behavior, indicator aggregation, export, descriptor validity, and provenance
  metadata
- CLI end-to-end coverage using
  `backend/internal/app/cli/testdata/phase10/road_model.geojson`
- synthetic acceptance fixtures in `backend/internal/qa/acceptance/testdata/cnossos-road/`
  for:
  - `road_preview`
  - `road_contextual`

These fixtures are repo-authored and license-safe, but they are regression
fixtures for our implementation rather than public normative reference cases.

### Public attributable evidence

Public road evidence is documented in
`docs/research/cnossos-road-public-reference-totals.md`.

That note records official Irish EPA Round 4 road-noise reference totals derived
from publicly described CNOSSOS-EU strategic noise mapping outputs. The evidence
is suitable for attributable external benchmarking at the reference-total level.

## Compliance Boundary

This Phase 10 baseline should be read as:

- a deterministic planning-track CNOSSOS road preview implementation
- suitable for repository QA, regression control, and baseline public evidence
- explicit about what is implemented in code today

This Phase 10 baseline should not be read as:

- a claim of full legal conformance to every CNOSSOS road annex detail
- a substitute for authority-issued or vendor-issued conformance packs
- a complete public scenario-level verification suite

Important current limits:

- source support is limited to road line sources
- the propagation path is a compact baseline, not a full public conformance
  implementation of every CNOSSOS road detail
- exported values remain raw `float64`; there is no separate user-facing
  reporting precision contract yet

## Completion Statement

Phase 10 is considered complete as a shipped baseline because the repository now
has:

- the online `cnossos-road` module
- deterministic CLI integration and export
- documented rounding/tolerance behavior
- per-feature import extraction
- synthetic regression and acceptance evidence
- at least one attributable, license-safe public road reference-total source

Future deeper conformance work remains valid engineering work, but it is no
longer a blocker for the completion of the Phase 10 baseline itself.
