# BUF Aircraft Public Reference Totals

Status date: 2026-03-08

## Purpose

This note closes the narrow Phase 15 need for at least one license-safe,
attributable public aircraft benchmark source for the mapping-track aircraft
baseline.

It does not create a scenario-level conformance suite. Instead, it records
public aircraft exposure references that can be cited as external evidence for
the shipped `buf-aircraft` baseline.

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

- method/context attribution for the public aircraft references below
- public proof that the cited airport outputs belong to the official END
  strategic noise-mapping workflow

### 2. Dublin Airport Noise Action Plan 2024-2028

URLs:

- https://www.fingal.ie/noise-action-plan
- https://www.fingal.ie/sites/default/files/2024-12/noise-action-plan-for-dublin-airport-2024-2028.pdf

Why it matters:

- It is an official current public airport-noise action plan.
- It states the relevant aircraft-noise exposure checks for the airport context.

### 3. Review of the effectiveness of noise mitigation measures at Dublin Airport during 2023

URL:

- https://www.fingal.ie/sites/default/files/2024-08/noise-mitigation-effectiveness-review-report-for-2023.pdf

Why it matters:

- It publishes explicit public aircraft exposure counts under the same END /
  Directive 2015/996 noise-mapping framework.

## Extracted Public Reference Evidence

The following public references are useful as external evidence for aircraft
mapping outputs:

### Dublin Airport aircraft exposure, 2023 supplementary year

From the 2023 Dublin Airport review report:

- people exposed above `65 dB Lden` in 2023:
  - `323`
- people exposed above `55 dB Lnight` in 2023:
  - `4,465`

## What This Evidence Can Support

This public evidence is strong enough to support:

- a claim that the repo has at least one attributable, license-safe external
  reference source for BUF aircraft outputs
- future comparison work against public aircraft-noise exposure totals
- phase documentation that distinguishes:
  - synthetic in-repo regression fixtures
  - public external evidence

## What This Evidence Does Not Yet Support

This evidence does not yet provide:

- a small scenario-level validation case with raw source geometry and exact
  expected receiver outputs
- a deterministic public fixture that can be run directly in CI today
- enough public detail to claim full module-level BUF conformance

## Conclusion

For Phase 15, this is sufficient to treat the public-evidence requirement as
met at the reference-total level:

- public
- license-safe
- attributable
- END / Directive 2015/996 aircraft-noise framework
- mapping-aircraft specific

Any future stronger requirement should be tracked separately as a later
conformance or public-fixture milestone, not as an unfinished Phase 15 blocker.
