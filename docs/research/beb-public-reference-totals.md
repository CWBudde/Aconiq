# BEB Public Reference Totals

Status date: 2026-03-08

## Purpose

This note closes the narrow Phase 16 need for license-safe, attributable public
reference totals for BEB-style exposure aggregation.

It does not create a scenario-level conformance suite. Instead, it records
public exposed-population totals that are suitable as external benchmark
references for BEB-like affected-person summaries.

## Source Set

### 1. EPA State of the Environment 2024, Chapter 3

URLs:

- https://epawebapp.epa.ie/ebooks/soe2024/66/
- https://epawebapp.epa.ie/ebooks/soe2024/67/
- https://www.epa.ie/publications/monitoring--assessment/assessment/state-of-the-environment/EPA-SOER-2024-Chapter-03-Environmental-Noise.pdf

Why it matters:

- It publishes public exposed-population totals for road, rail, and industry
  under END strategic noise mapping.
- Those totals are suitable as external references for BEB-style population
  aggregation outputs.

### 2. Dublin Airport aircraft-noise public reporting

URLs:

- https://www.fingal.ie/noise-action-plan
- https://www.fingal.ie/sites/default/files/2024-12/noise-action-plan-for-dublin-airport-2024-2028.pdf
- https://www.fingal.ie/sites/default/files/2024-08/noise-mitigation-effectiveness-review-report-for-2023.pdf

Why it matters:

- It publishes public aircraft exposed-population totals for the airport-noise
  context.
- Those totals are suitable as external references when BEB uses
  `buf-aircraft` as its upstream mapping contract.

## Extracted Public Reference Evidence

Examples of attributable public totals that match BEB-style exposure questions:

### Road-style exposed population totals

From the EPA State of the Environment material:

- people exposed above the road END reporting threshold:
  - about `1,033,400`
- people exposed from major roads:
  - `712,700`

### Rail-style exposed population totals

- people exposed above the railway END reporting threshold:
  - about `15,400`

### Industry-style exposed population shares

- industrial-source exposure shares in Irish agglomeration reporting:
  - Dublin: `<1%`
  - Cork: `1%`
  - Limerick: `0%`

### Aircraft-style exposed population totals

From the Dublin Airport 2023 review:

- people exposed above `65 dB Lden`:
  - `323`
- people exposed above `55 dB Lnight`:
  - `4,465`

## What This Evidence Can Support

This public evidence is strong enough to support:

- a claim that the repo has attributable, license-safe external reference totals
  for BEB-style exposed-population outputs
- future comparison work against public affected/exposed population summaries
- phase documentation that distinguishes:
  - synthetic in-repo regression fixtures
  - public external evidence

## What This Evidence Does Not Yet Support

This evidence does not yet provide:

- a building-by-building public validation case
- a deterministic public fixture that can be run directly in CI today
- enough public detail to claim full BEB conformance

## Conclusion

For Phase 16, this is sufficient to treat the public-evidence requirement as
met at the public reference-total level for affected/exposed population outputs.
