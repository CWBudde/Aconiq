# Phase 15 BUF Aircraft Baseline

Status date: 2026-03-08

## Goal

Phase 15 brings the first Germany mapping-track BUF module online:
`buf-aircraft`.

The completion target for this phase is a shipped, deterministic, CLI-runnable
mapping baseline that:

- uses explicit mapping context metadata
- accepts typed aircraft trajectory sources
- computes `Lday`, `Levening`, `Lnight`, and `Lden`
- exports receiver tables and rasters
- records enough provenance and QA evidence to make the baseline reviewable

It is not a claim that the repository contains a full official BUF conformance
suite or redistributes all normative German source packages.

## Implemented Scope

### Mapping context and module shape

- Online mapping module for this phase:
  - `backend/internal/standards/buf/aircraft`
- The repository treats BUF as:
  - a standalone aircraft mapping module
  - not a downstream BUB post-processing stage
- Standards-framework descriptors use explicit mapping context metadata rather
  than planning context metadata.

### BUF aircraft source model

The `buf-aircraft` baseline supports typed aircraft trajectory sources with
validation for:

- airport and runway identifiers
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

### Normative scope and current I/O contract

The current baseline is intentionally a compact mapping-oriented aircraft
contract. It supports:

- airport-vicinity style trajectory modeling
- movement-based period handling
- mapping-context indicators and exports

It does not yet claim a fully general German airport-noise data model with all
possible operational inputs, reporting tables, or authority-issued validation
packs.

### Compute, indicators, and export

The current aircraft baseline implements:

- deterministic movement-based emission logic
- operation and aircraft-class distinctions
- segmented 3D source-to-receiver coupling
- geometric divergence
- air absorption
- ground attenuation
- aircraft-specific contextual adjustments for:
  - climb
  - approach
  - bank-angle directivity
- indicators:
  - `Lday`
  - `Levening`
  - `Lnight`
  - `Lden`
- result export:
  - receiver tables: `receivers.json`, `receivers.csv`
  - raster sidecar/data pair: `buf-aircraft.json`, `buf-aircraft.bin`
- CLI wiring:
  - `aconiq run --standard buf-aircraft`
  - extraction from normalized GeoJSON
  - receiver-grid generation
  - result persistence
  - provenance metadata

## QA and Evidence

### In-repo deterministic QA

The repository contains deterministic QA coverage for the Phase 15 baseline:

- module unit tests and golden scenario coverage for `buf-aircraft`
- CLI end-to-end coverage using
  `backend/internal/app/cli/testdata/phase15/aircraft_model.geojson`
- synthetic acceptance fixtures in
  `backend/internal/qa/acceptance/testdata/buf-aircraft/` for:
  - `aircraft_mapping`
  - `aircraft_contextual`

These fixtures are repo-authored and license-safe. They are regression and
acceptance assets for the implementation, not public official BUF validation
packs.

### Public attributable evidence

Public aircraft reference evidence for the mapping-track baseline is documented
in `docs/research/buf-aircraft-public-reference-totals.md`.

That note records attributable public aircraft-noise exposure totals under the
END / Directive 2015/996 framework. The evidence is suitable for external
benchmarking at the public reference-total level.

## Rounding and tolerance contract

The current BUF aircraft rounding and tolerance behavior is documented in
`docs/research/milestone-e-rounding-tolerances.md`.

For the shipped baseline:

- runtime exports keep raw `float64` values
- golden fixtures round to 6 decimals
- analytical `Lden` checks use the shared preview tolerance conventions

## Compliance Boundary

This Phase 15 baseline should be read as:

- a deterministic mapping-track BUF aircraft preview implementation
- suitable for repository QA, regression control, and mapping-context exports
- explicit about what is implemented in code today

This Phase 15 baseline should not be read as:

- a claim of full official BUF conformance
- a substitute for authority-issued German validation packs
- a complete public scenario-level verification suite

## Completion Statement

Phase 15 is considered complete as a shipped baseline because the repository now
has:

- the online `buf-aircraft` mapping module
- explicit mapping-context provenance and exports
- clarified standalone module scope and current I/O contract
- documented rounding/tolerance behavior
- synthetic regression and acceptance evidence
- at least one attributable, license-safe public aircraft reference source

Future deeper normative and data-rights work remains valid, but it is no longer
a blocker for the completion of the Phase 15 baseline itself.
