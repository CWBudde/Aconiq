# BEB Dataset Requirements and Import Rights

Status date: 2026-03-08

## Purpose

This note closes the Phase 16 research item about required building/population
datasets and how the public repository should treat their rights status.

It is a clarification note, not a legal opinion.

## Required Dataset Categories

The BEB baseline conceptually needs:

- building footprints or equivalent building aggregation geometry
- building height or floor-count information
- building usage classification
- occupancy-related quantities:
  - dwellings
  - persons

The shipped baseline supports synthetic or imported approximations for these via:

- explicit per-feature properties
- height-derived occupancy fallbacks
- run-level defaults

## Current Repo Position

The current repository position is:

- synthetic building fixtures are the in-repo QA baseline
- no official external building/population datasets are vendored in the repo
- no blanket redistribution-rights conclusion is documented for population or
  building stock datasets that might be used in public BEB workflows

## Practical Handling Rule

Until explicit rights are documented, treat external BEB input datasets as:

- external inputs
- not committed to the public repository by default
- suitable for local/private workflows only if obtained and handled separately

The public repo should continue to rely on:

- synthetic fixtures
- import contracts
- occupancy fallbacks
- attributable public reference totals for external benchmarking

## Export and Reporting Alignment

The BEB baseline now exports:

- threshold-based affected totals
- `Lden` / `Lnight` 5 dB exposure-band summaries

This is sufficient to align the shipped BEB summary contract with EEA-style
banded reporting at a baseline level without depending on private datasets.

## Conclusion

For Phase 16, this is enough to close the dataset/rights research item:

- required dataset categories are explicit
- the repo handling rule is explicit
- the shipped export contract now includes banded exposure summaries

Any future stronger rights conclusion should be tracked as a separate
legal/data-packaging task, not as an unfinished Phase 16 blocker.
