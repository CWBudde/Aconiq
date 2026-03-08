# Phase 16 BEB Exposure Baseline

Status date: 2026-03-08

## Goal

Phase 16 brings the first Germany mapping-track BEB exposure module online:
`beb-exposure`.

The completion target for this phase is a shipped, deterministic, CLI-runnable
exposure aggregation baseline that:

- consumes mapping-track upstream levels
- computes building-level exposure and affected-population indicators
- exports building tables, aggregate summaries, and aggregate rasters
- records enough provenance and QA evidence to make the baseline reviewable

It is not a claim that the repository contains the full official BEB data pack
or a complete public conformance suite.

## Implemented Scope

### Pipeline placement and upstream contracts

The repository treats BEB as:

- a downstream exposure / affected-persons stage
- part of the mapping track
- able to consume upstream levels from:
  - `bub-road`
  - `buf-aircraft`

The upstream contract is explicit in the BEB parameter schema through
`upstream_mapping_standard`.

### Input model

The current BEB baseline supports typed building inputs with validation for:

- building footprint polygons
- building height
- usage type
- optional floor count
- optional estimated dwellings
- optional estimated persons

Imported GeoJSON can provide these values per feature; run-level parameters act
as defaults and fallbacks.

### Aggregation model

The shipped baseline implements deterministic exposure aggregation for:

- `Lden`
- `Lnight`
- estimated dwellings
- estimated persons
- affected dwellings above configured thresholds
- affected persons above configured thresholds

It also exports EEA-style `5 dB` band summaries in the BEB summary output for:

- `Lden` bands: `55-59`, `60-64`, `65-69`, `70-74`, `75+`
- `Lnight` bands: `50-54`, `55-59`, `60-64`, `65-69`, `70+`

### Export and reporting contract

The current baseline writes:

- building tables:
  - `buildings.json`
  - `buildings.csv`
- aggregate raster:
  - `beb-exposure.json`
  - `beb-exposure.bin`
- aggregate summary:
  - `beb-summary.json`
- run summary:
  - `run-summary.json`

The summary outputs now include:

- threshold-based totals
- occupancy and facade-evaluation modes
- upstream mapping standard
- `Lden` / `Lnight` 5 dB exposure-band summaries

## QA and Evidence

### In-repo deterministic QA

The repository contains deterministic QA coverage for the Phase 16 baseline:

- module tests for:
  - threshold aggregation
  - occupancy overrides and derived occupancy
  - facade evaluation behavior
  - upstream `buf-aircraft` support
  - export behavior
  - provenance metadata
- golden scenario coverage in
  `backend/internal/standards/beb/exposure/testdata/`
- acceptance fixtures in
  `backend/internal/qa/acceptance/testdata/beb-exposure/`

These fixtures are repo-authored and license-safe. They are regression and
acceptance assets for the implementation, not public official BEB validation
packs.

### Public attributable evidence

Public BEB reference evidence is documented in
`docs/research/beb-public-reference-totals.md`.

That note records attributable public exposed-population totals from Irish END
and airport-noise reporting that are suitable as external benchmark references
for BEB-style affected-person aggregation.

## Data and rights note

Population/building input expectations and rights handling are documented in
`docs/research/beb-dataset-requirements-and-rights.md`.

The current repo position is:

- synthetic building fixtures remain the in-repo baseline
- external building/population datasets should stay outside the public repo
  unless redistribution rights are clear

## Rounding and aggregation conventions

The current BEB rounding and aggregation behavior is documented in
`docs/research/milestone-e-rounding-tolerances.md`.

For the shipped baseline:

- internal aggregation stays in raw `float64`
- threshold checks are inclusive (`>=`)
- runtime exports keep raw values
- golden fixtures round to 6 decimals
- summary totals are sums of raw building-level outputs

## Compliance Boundary

This Phase 16 baseline should be read as:

- a deterministic mapping-track BEB preview implementation
- suitable for repository QA, regression control, and baseline exposure exports
- explicit about what is implemented in code today

This Phase 16 baseline should not be read as:

- a claim of full official BEB conformance
- a substitute for authority-issued German exposure-validation packs
- a complete public scenario-level validation suite with official building/population data

## Completion Statement

Phase 16 is considered complete as a shipped baseline because the repository now
has:

- the online `beb-exposure` module
- explicit downstream contracts for `bub-road` and `buf-aircraft`
- threshold totals plus `5 dB` banded exposure summaries
- documented dataset/rights handling for external population/building inputs
- documented rounding/tolerance and aggregation conventions
- synthetic regression and acceptance evidence
- attributable public reference totals for affected-population style outputs

Future deeper normative and dataset-rights work remains valid, but it is no
longer a blocker for the completion of the Phase 16 baseline itself.
