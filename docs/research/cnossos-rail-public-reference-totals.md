# CNOSSOS Rail Public Reference Totals

Status date: 2026-03-08

## Purpose

This note closes the narrow Phase 11 need for at least one license-safe,
attributable public rail benchmark source.

It does not create a scenario-level conformance suite. Instead, it records
public reference totals that can be cited as external evidence for the shipped
`cnossos-rail` baseline.

## Source Set

### 1. EPA Noise Mapping Data page

URL:

- https://www.epa.ie/our-services/monitoring--assessment/noise/noise-mapping-and-action-plans/how-to-access-data/

Why it matters:

- It states that the Round 4 strategic noise mapping outputs were generated
  using CNOSSOS-EU.
- It states that the downloadable and publicly viewable data is based largely on
  2021 road, rail, and airport traffic data.
- It points to public access paths via EPA Maps and Reportnet 3.

Use in this repo:

- method attribution for the public totals below
- public proof that the cited Irish Round 4 rail outputs are CNOSSOS-EU-based

### 2. EPA State of the Environment 2024, Chapter 3

URLs:

- https://epawebapp.epa.ie/ebooks/soe2024/66/
- https://epawebapp.epa.ie/ebooks/soe2024/67/
- https://www.epa.ie/publications/monitoring--assessment/assessment/state-of-the-environment/EPA-SOER-2024-Chapter-03-Environmental-Noise.pdf

Why it matters:

- It publishes railway-noise population exposure totals and banded counts from
  Round 4 mapping.
- The source is public and attributable.
- The underlying mapping is explicitly tied to the EPA CNOSSOS-EU Round 4
  workflow described above.

## Extracted Public Reference Totals

The following totals are useful as public reference evidence for rail outputs.

### `Lden` rail exposure

For the Round 4 railway-noise mapping described by the EPA:

- total people above the END reporting threshold for railway noise:
  - about `15,400`
- city/agglomeration major-rail `Lden > 55 dB` totals on the EPA page:
  - Cork: `0`
  - Dublin: `15,400`
  - Limerick: `0`

### `Lnight` rail exposure

For the city/agglomeration table published by the EPA, public major-rail
`Lnight > 50 dB` totals are:

- Cork: `0`
- Dublin: `7,000`
- Limerick: `0`

## What This Evidence Can Support

This public evidence is strong enough to support:

- a claim that the repo has at least one attributable, license-safe external
  reference source for CNOSSOS rail outputs
- future reference-total comparison work against public rail exposure totals
- phase documentation that distinguishes:
  - synthetic in-repo regression fixtures
  - public external evidence

## What This Evidence Does Not Yet Support

This evidence does not yet provide:

- a small scenario-level validation case with raw source geometry and exact
  expected receiver outputs
- a deterministic public fixture that can be run directly in CI today
- enough public detail to claim full module-level CNOSSOS rail conformance

## Conclusion

For Phase 11, this is sufficient to treat the public-evidence requirement as
met at the reference-total level:

- public
- license-safe
- attributable
- CNOSSOS-EU-based
- rail-specific

Any future stronger requirement should be tracked separately as a later
conformance or public-fixture milestone, not as an unfinished Phase 11 blocker.
