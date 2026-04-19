# Phase 14 BUB Road Baseline

Status date: 2026-03-08

## Goal

Phase 14 brings the first Germany mapping-track BUB module online:
`bub-road`.

The completion target for this phase is a shipped, deterministic, CLI-runnable
mapping baseline that:

- uses explicit mapping context metadata
- accepts typed BUB road line sources
- computes `Lday`, `Levening`, `Lnight`, and `Lden`
- exports receiver tables and rasters
- records enough provenance and QA evidence to make the baseline reviewable

It is not a claim that the repository contains the full official German BUB data
package or a complete public conformance suite.

## Implemented Scope

### Mapping context and module structure

- Online mapping module for this phase:
  - `backend/internal/standards/bub/road`
- Split BUB submodule packages also exist for divergent logic areas:
  - `backend/internal/standards/bub/rail`
  - `backend/internal/standards/bub/industry`
- Standards-framework descriptors use explicit mapping context metadata rather
  than planning context metadata.

### BUB road source model

The `bub-road` baseline supports typed road line sources with validation for:

- geometry
- road function class
- surface type
- speed
- gradient
- junction context
- temperature
- studded tyre share
- per-period traffic splits

Imported GeoJSON can provide these values per feature; run-level parameters act
as defaults.

### Generic project data vs BUB-only inputs

Generic project/run concepts used by BUB:

- project, scenario, and run lifecycle
- normalized GeoJSON source geometry
- receiver grid generation
- result tables and rasters
- provenance, artifacts, and export bundles

BUB-specific phase inputs and metadata:

- mapping context selection
- `road_function_class`
- BUB road parameter defaults and mapping-oriented interpretation
- BUB compliance-boundary metadata in run provenance

Future official BUB datasets or raw source packages should remain external until
import rights are explicitly confirmed.

### Compute, indicators, and export

The current road baseline implements:

- deterministic road emission and propagation logic for the mapping context
- contextual source handling for mapping-oriented road function classes
- indicators:
  - `Lday`
  - `Levening`
  - `Lnight`
  - `Lden`
- result export:
  - receiver tables: `receivers.json`, `receivers.csv`
  - raster sidecar/data pair: `bub-road.json`, `bub-road.bin`
- CLI wiring:
  - `aconiq run --standard bub-road`
  - extraction from normalized GeoJSON
  - receiver-grid generation
  - result persistence
  - provenance metadata

## QA and Evidence

### In-repo deterministic QA

The repository contains deterministic QA coverage for the Phase 14 baseline:

- module unit tests and golden scenario coverage for `bub-road`
- CLI end-to-end coverage using
  `backend/internal/app/cli/testdata/phase14/bub_road_model.geojson`
- synthetic acceptance fixtures in `backend/internal/qa/acceptance/testdata/bub-road/`
  for:
  - `road_mapping`
  - `road_contextual`

These fixtures are repo-authored and license-safe. They are regression and
acceptance assets for the implementation, not public official BUB validation
packs.

### Public context and rights note

Public context and import-rights status are documented in
`docs/research/bub-dataset-availability-and-rights.md`.

That note clarifies the current repo position:

- public German strategic-noise-mapping context is available
- official BUB datasets are not vendored in this repository
- import rights for raw BUB-specific source packages are not yet treated as
  cleared for in-repo redistribution

## Rounding and tolerance contract

The current BUB road rounding and tolerance behavior is documented in
`docs/research/milestone-e-rounding-tolerances.md`.

For the shipped baseline:

- runtime exports keep raw `float64` values
- golden fixtures round to 6 decimals
- no dedicated analytical `Lden` epsilon is currently documented beyond the
  shared preview rules

## Compliance Boundary

This Phase 14 baseline should be read as:

- a deterministic mapping-track BUB road preview implementation
- suitable for repository QA, regression control, and mapping-context exports
- explicit about what is implemented in code today

This Phase 14 baseline should not be read as:

- a claim that the repository redistributes official BUB source datasets
- a substitute for authority-issued German conformance packs
- a full public scenario-level validation suite for all BUB subdomains

## Completion Statement

Phase 14 is considered complete as a shipped baseline because the repository now
has:

- the online `bub-road` mapping module
- explicit mapping-context provenance and exports
- documented generic-vs-BUB-specific input boundaries
- clarified public dataset / import-rights status
- documented rounding/tolerance behavior
- synthetic regression and acceptance evidence
- split BUB submodule packages for road, rail, and industry where logic diverges

Future deeper normative and data-rights work remains valid, but it is no longer
a blocker for the completion of the Phase 14 baseline itself.
