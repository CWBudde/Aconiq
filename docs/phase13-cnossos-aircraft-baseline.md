# Phase 13 CNOSSOS Aircraft Baseline

Status date: 2026-03-08

## Goal

Phase 13 brings the first planning-track CNOSSOS aircraft module online:
`cnossos-aircraft`.

The completion target for this phase is a shipped, deterministic, CLI-runnable
baseline that:

- accepts typed aircraft trajectory sources
- computes `Lday`, `Levening`, `Lnight`, and `Lden`
- exports receiver tables and rasters
- records enough provenance and QA evidence to make the baseline reviewable

It is not a claim that the repository contains a full public conformance suite
for every legal detail of CNOSSOS-EU aircraft noise.

## Implemented Scope

### Standards module

- Module path: `backend/internal/standards/cnossos/aircraft`
- Registry ID: `cnossos-aircraft`
- Built-in model version: `phase13-preview-v2`
- Default profile: `airport-vicinity`
- Standards-framework descriptor present with:
  - planning context metadata
  - version/profile metadata
  - supported source types
  - supported indicators
  - validated run parameter schema

### Source model

The baseline supports typed aircraft trajectory sources through `AircraftSource`
with validation for:

- runway and airport identifiers
- operation type
- aircraft class
- procedure type
- thrust mode
- segmented 3D flight-track geometry
- lateral offset
- reference power level
- engine-state factor
- bank angle
- per-period movement rates

Imported GeoJSON can provide these values per feature; run-level parameters act
as defaults.

### Minimum useful baseline input set

The current baseline expects enough information to describe a useful
airport-vicinity source contract:

- `airport_id`
- `runway_id`
- operation type (`departure` or `arrival`)
- aircraft class
- procedure type
- thrust mode
- a segmented 3D trajectory with at least two points
- per-period movement counts

This is the minimum contract that the current module can use to produce
meaningful differentiated outputs while staying within a compact preview scope.

### Emission baseline

The current aircraft module implements a deterministic preview emission path
with:

- movement-based period handling
- operation-specific departure/arrival distinctions
- aircraft-class distinctions
- engine-state scaling

### Propagation baseline

The current aircraft module implements a deterministic trajectory-based
propagation path with:

- segmented 3D source-to-receiver coupling
- geometric divergence
- air absorption
- ground attenuation term
- aircraft-specific contextual adjustments for:
  - climb
  - approach
  - bank-angle directivity
- deterministic multi-source energetic summation

### Indicators and export

- Indicators:
  - `Lday`
  - `Levening`
  - `Lnight`
  - `Lden`
- Result export:
  - receiver tables: `receivers.json`, `receivers.csv`
  - raster sidecar/data pair: `cnossos-aircraft.json`, `cnossos-aircraft.bin`
- CLI wiring:
  - `aconiq run --standard cnossos-aircraft`
  - extraction from normalized GeoJSON
  - receiver-grid generation
  - result persistence
  - provenance metadata

## QA and Evidence

### In-repo deterministic QA

The repository contains deterministic QA coverage for the Phase 13 baseline:

- module unit tests for schema validation, emission behavior, propagation
  behavior, indicator aggregation, export, and provenance metadata
- CLI end-to-end coverage using
  `backend/internal/app/cli/testdata/phase13/aircraft_model.geojson`
- synthetic acceptance fixtures in
  `backend/internal/qa/acceptance/testdata/cnossos-aircraft/` for:
  - `aircraft_preview`
  - `aircraft_contextual`

These fixtures are repo-authored and license-safe, but they are regression
fixtures for our implementation rather than public normative reference cases.

### Public attributable evidence

Public aircraft evidence is documented in
`docs/research/cnossos-aircraft-public-reference-totals.md`.

That note records official Dublin Airport public exposure references under the
Irish END/Directive 2015/996 aircraft-noise framework. The evidence is suitable
for attributable external benchmarking at the public reference-total level.

## Compliance Boundary

This Phase 13 baseline should be read as:

- a deterministic planning-track CNOSSOS aircraft preview implementation
- suitable for repository QA, regression control, and baseline public evidence
- explicit about what is implemented in code today

This Phase 13 baseline should not be read as:

- a claim of full legal conformance to every CNOSSOS aircraft annex detail
- a substitute for authority-issued or vendor-issued conformance packs
- a complete public scenario-level verification suite

Important current limits:

- scope is intentionally limited to an `airport-vicinity` profile
- the source contract is richer than the first preview slice, but it is still
  not a full airport-operations data model
- exported values remain raw `float64`; there is no separate user-facing
  reporting precision contract yet

## Completion Statement

Phase 13 is considered complete as a shipped baseline because the repository now
has:

- the online `cnossos-aircraft` module
- deterministic CLI integration and export
- documented rounding/tolerance behavior
- per-feature import extraction
- a clarified minimum airport/runway/trajectory input contract
- synthetic regression and acceptance evidence
- at least one attributable, license-safe public aircraft reference source

Future deeper conformance work remains valid engineering work, but it is no
longer a blocker for the completion of the Phase 13 baseline itself.
