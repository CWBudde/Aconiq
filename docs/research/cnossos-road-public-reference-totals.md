# CNOSSOS Road Public Reference Totals

Status date: 2026-03-08

## Purpose

This note closes the narrow Phase 10 need for at least one license-safe,
attributable public road benchmark source.

It does not create a scenario-level conformance suite. Instead, it records
public reference totals that can be cited as external evidence for the shipped
`cnossos-road` baseline.

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
- public proof that the cited Irish Round 4 road outputs are CNOSSOS-EU-based

### 2. EPA State of the Environment 2024, Chapter 3

URLs:

- https://epawebapp.epa.ie/ebooks/soe2024/66/
- https://epawebapp.epa.ie/ebooks/soe2024/67/
- https://www.epa.ie/publications/monitoring--assessment/assessment/state-of-the-environment/EPA-SOER-2024-Chapter-03-Environmental-Noise.pdf

Why it matters:

- It publishes road-noise population exposure totals and banded counts from
  Round 4 mapping.
- The source is public and attributable.
- The underlying mapping is explicitly tied to the EPA CNOSSOS-EU Round 4
  workflow described above.

## Extracted Public Reference Totals

The following totals are useful as public reference evidence for road outputs.

### `Lden` road exposure

For the Round 4 road-noise mapping described by the EPA:

- total people above the END reporting threshold for road traffic noise:
  - about `1,033,400` for the mapped areas
- of that total, people exposed from major roads:
  - `712,700`
- major-road subtotal broken out on the EPA page:
  - Dublin: `306,500`
  - Cork: `41,500`
  - Limerick: `22,000`
  - outside those three cities: `342,700`

For city/agglomeration tables, the public major-road `Lden > 55 dB` totals are:

- Cork: `41,500`
- Dublin: `306,500`
- Limerick: `22,000`

### `Lnight` road exposure

For the city/agglomeration table published by the EPA, public major-road
`Lnight > 50 dB` totals are:

- Cork: `19,100`
- Dublin: `195,100`
- Limerick: `8,400`

## What This Evidence Can Support

This public evidence is strong enough to support:

- a claim that the repo has at least one attributable, license-safe external
  reference source for CNOSSOS road outputs
- future reference-total comparison work against public road exposure totals
- phase documentation that distinguishes:
  - synthetic in-repo regression fixtures
  - public external evidence

## What This Evidence Does Not Yet Support

This evidence does not yet provide:

- a small scenario-level validation case with raw source geometry and exact
  expected receiver outputs
- a deterministic public fixture that can be run directly in CI today
- enough public detail to claim full module-level CNOSSOS road conformance

## Conclusion

For Phase 10, this is sufficient to treat the public-evidence requirement as
met at the reference-total level:

- public
- license-safe
- attributable
- CNOSSOS-EU-based
- road-specific

Any future stronger requirement should be tracked separately as a later
conformance or public-fixture milestone, not as an unfinished Phase 10 blocker.
