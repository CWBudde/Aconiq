# CNOSSOS Industry Public Reference Totals

Status date: 2026-03-08

## Purpose

This note closes the narrow Phase 12 need for at least one license-safe,
attributable public industry benchmark source.

It does not create a scenario-level conformance suite. Instead, it records
public exposure references that can be cited as external evidence for the
shipped `cnossos-industry` baseline.

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

- method attribution for the public industry references below
- public proof that the cited Irish Round 4 outputs belong to the same CNOSSOS-EU
  strategic mapping context

### 2. Dublin Agglomeration Noise Action Plan 2024-2028

URL:

- https://www.dublincity.ie/sites/default/files/2024-04/draft-dublin-agglomeration-noise-action-plan.pdf

Why it matters:

- It publishes public source-split exposure shares for the Dublin agglomeration.
- For industrial sources, it reports:
  - `<1%` of the population exposed at `Lden >= 55 dB`
  - `<1%` of the population exposed at `Lnight >= 50 dB`

### 3. Cork Agglomeration Noise Action Plan 2024-2028

URL:

- https://www.corkcity.ie/media/0tmbbz53/cork-agglomeration-noise-action-plan-2024-2028.pdf

Why it matters:

- It publishes public source-split exposure shares for the Cork agglomeration.
- For industrial sources, it reports:
  - `1%` of the population exposed at `Lden >= 55 dB`
  - `1%` of the population exposed at `Lnight >= 50 dB`

### 4. Limerick Agglomeration Noise Action Plan 2024-2028

URL:

- https://www.limerick.ie/sites/default/files/media/documents/2024-08/limerick-agglomeration-noise-action-plan-2024-2028.pdf

Why it matters:

- It publishes public source-split exposure shares for the Limerick agglomeration.
- For industrial sources, it reports:
  - `0%` of the population exposed at `Lden >= 55 dB`
  - `0%` of the population exposed at `Lnight >= 50 dB`

## Extracted Public Reference Evidence

The following public references are useful as external evidence for industry
outputs in Irish Round 4 agglomeration mapping:

### `Lden` industrial exposure

- Dublin agglomeration:
  - industrial-source exposure share at `Lden >= 55 dB`: `<1%`
- Cork agglomeration:
  - industrial-source exposure share at `Lden >= 55 dB`: `1%`
- Limerick agglomeration:
  - industrial-source exposure share at `Lden >= 55 dB`: `0%`

### `Lnight` industrial exposure

- Dublin agglomeration:
  - industrial-source exposure share at `Lnight >= 50 dB`: `<1%`
- Cork agglomeration:
  - industrial-source exposure share at `Lnight >= 50 dB`: `1%`
- Limerick agglomeration:
  - industrial-source exposure share at `Lnight >= 50 dB`: `0%`

## What This Evidence Can Support

This public evidence is strong enough to support:

- a claim that the repo has at least one attributable, license-safe external
  reference source for CNOSSOS industry outputs
- future comparison work against public industrial-noise exposure shares or
  derived reference totals
- phase documentation that distinguishes:
  - synthetic in-repo regression fixtures
  - public external evidence

## What This Evidence Does Not Yet Support

This evidence does not yet provide:

- a small scenario-level validation case with raw source geometry and exact
  expected receiver outputs
- a deterministic public fixture that can be run directly in CI today
- enough public detail to claim full module-level CNOSSOS industry conformance

## Conclusion

For Phase 12, this is sufficient to treat the public-evidence requirement as
met at the reference-total / exposure-share level:

- public
- license-safe
- attributable
- CNOSSOS-EU-based
- industry-specific

Any future stronger requirement should be tracked separately as a later
conformance or public-fixture milestone, not as an unfinished Phase 12 blocker.
