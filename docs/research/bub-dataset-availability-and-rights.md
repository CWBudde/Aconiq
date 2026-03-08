# BUB Dataset Availability and Import Rights

Status date: 2026-03-08

## Purpose

This note closes the Phase 14 research question about what is currently known
regarding BUB-related public context, likely dataset availability, and import
rights.

It is a clarification note, not a legal opinion.

## Publicly Available Context

The repository already records one public German strategic-noise-mapping anchor:

- Umweltbundesamt noise maps page:
  - https://www.umweltbundesamt.de/themen/laerm/umgebungslaermrichtlinie/laermkarten

This is enough to support:

- public context that Germany publishes strategic noise-mapping outputs
- a public anchor for BUB / BUF / BEB evidence discovery

This is not enough by itself to support:

- vendoring raw BUB-specific source datasets into the public repository
- assuming that every downloadable German source dataset is redistribution-safe

## Current Repo Position

The current repository position is:

- public strategic-noise-mapping context is known
- synthetic fixtures are the in-repo validation baseline for `bub-road`
- no official BUB raw datasets are vendored in this repository
- no blanket import-rights clearance has been documented for BUB-specific raw
  data packages

## Practical Handling Rule

Until explicit rights are documented, treat BUB-specific raw input packages as:

- external inputs
- not committed to the public repo by default
- suitable for local/private workflows only if obtained and handled separately

The public repo should continue to rely on:

- synthetic fixtures
- source inventories
- public context pages
- implementation notes that avoid redistributing restricted normative material

## Format and Availability Clarification

At the current repository level, the most defensible statement is:

- BUB-related public reporting outputs are available in Germany
- the repo has not yet pinned a redistribution-safe, authoritative raw BUB data
  package format for public vendoring
- any future importer for official BUB-specific datasets should be designed as an
  external-input workflow unless rights are clarified

## Conclusion

For Phase 14, this is enough to close the research item:

- public context is identified
- the repo position on import rights is explicit
- the handling rule is documented

Any future stronger rights conclusion should be tracked as a separate legal/data
packaging task, not as an unfinished Phase 14 blocker.
