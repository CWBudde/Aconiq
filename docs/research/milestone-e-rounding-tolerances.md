# Milestone E Rounding and Tolerance Rules

Status date: 2026-03-08

This note records the current rounding, tolerance, and reporting-boundary behavior for the
online preview standards modules. The intent is to remove ambiguity at output boundaries while
being explicit about what is already implemented versus what remains future reporting work.

## Global Baseline

These rules apply to all modules covered here unless a section says otherwise.

### Internal arithmetic

- Internal computation remains `float64`.
- No intermediate rounding is applied during emission, propagation, indicator derivation, or
  cross-source energetic summation.
- Threshold and aggregation logic operates on the unrounded computed values.

### Runtime export boundary

- Persisted receiver tables (`JSON`, `CSV`) keep the computed `float64` values.
- Persisted raster outputs keep the computed `float64` values.
- No runtime export path in the covered preview modules currently rounds values to a user-facing
  display precision such as `0.1 dB`.

### Regression snapshot boundary

- Golden regression snapshots round numeric values to 6 decimal places using:
  - `math.Round(value*1e6) / 1e6`
- This 6-decimal rounding is a test-fixture stability rule, not a runtime reporting rule.

### Analytical test tolerance

- Exact indicator checks that compare against a precomputed expected floating-point value use an
  absolute tolerance of `1e-9`.
- This is currently used for `Lden` aggregation checks in the CNOSSOS and BUF preview modules.

### Comparison policy

- When a test uses ordering or monotonic behavior, it currently uses strict comparisons without an
  epsilon:
  - for example, “near level > far level” or “higher traffic increases `Lday`”.
- No module-specific runtime comparison epsilon is applied in current exports or indicator thresholds.

## Shared `Lden` Convention

The current preview modules that expose `Lden` use the same day-evening-night aggregation:

- day contribution: `12 * 10^(Lday/10)`
- evening contribution: `4 * 10^((Levening + 5)/10)`
- night contribution: `8 * 10^((Lnight + 10)/10)`
- averaged over `24`
- if the total linear energy is non-positive, `Lden` returns `-999.0`

The shared current modules using this rule are:

- `cnossos-road`
- `cnossos-rail`
- `cnossos-industry`
- `cnossos-aircraft`
- `bub-road`
- `buf-aircraft`

## Module Rules

### CNOSSOS Road

- Module: `backend/internal/standards/cnossos/road`
- Internal precision:
  - raw `float64`, no intermediate rounding
- Export boundary:
  - raw `float64` values are persisted in receiver tables and rasters
- Regression boundary:
  - golden snapshots round receiver coordinates, receiver height, `Lday`, `Levening`, `Lnight`,
    and `Lden` to 6 decimals
- Explicit tolerance:
  - `Lden` analytical check uses absolute tolerance `1e-9`
- Sentinel convention:
  - `Lden` returns `-999.0` if the linear total is non-positive

### CNOSSOS Rail

- Module: `backend/internal/standards/cnossos/rail`
- Internal precision:
  - raw `float64`, no intermediate rounding
- Export boundary:
  - raw `float64` values are persisted in receiver tables and rasters
- Regression boundary:
  - golden snapshots round receiver coordinates, receiver height, `Lday`, `Levening`, `Lnight`,
    and `Lden` to 6 decimals
- Explicit tolerance:
  - `Lden` analytical check uses absolute tolerance `1e-9`
- Sentinel convention:
  - `Lden` returns `-999.0` if the linear total is non-positive

### CNOSSOS Industry

- Module: `backend/internal/standards/cnossos/industry`
- Internal precision:
  - raw `float64`, no intermediate rounding
- Export boundary:
  - raw `float64` values are persisted in receiver tables and rasters
- Regression boundary:
  - golden snapshots round receiver coordinates, receiver height, `Lday`, `Levening`, `Lnight`,
    and `Lden` to 6 decimals
- Explicit tolerance:
  - `Lden` analytical check uses absolute tolerance `1e-9`
- Sentinel convention:
  - `Lden` returns `-999.0` if the linear total is non-positive

### CNOSSOS Aircraft

- Module: `backend/internal/standards/cnossos/aircraft`
- Internal precision:
  - raw `float64`, no intermediate rounding
- Export boundary:
  - raw `float64` values are persisted in receiver tables and rasters
- Regression boundary:
  - golden snapshots round receiver coordinates, receiver height, `Lday`, `Levening`, `Lnight`,
    and `Lden` to 6 decimals
- Explicit tolerance:
  - `Lden` analytical check uses absolute tolerance `1e-9`
- Sentinel convention:
  - `Lden` returns `-999.0` if the linear total is non-positive

### BUB Road

- Module: `backend/internal/standards/bub/road`
- Internal precision:
  - raw `float64`, no intermediate rounding
- Export boundary:
  - raw `float64` values are persisted in receiver tables and rasters
- Regression boundary:
  - golden snapshots round receiver coordinates, receiver height, `Lday`, `Levening`, `Lnight`,
    and `Lden` to 6 decimals
- Explicit tolerance:
  - no dedicated analytical `Lden` tolerance test is currently present; monotonic/behavioral tests
    use strict comparisons without epsilon
- Sentinel convention:
  - `Lden` returns `-999.0` if the linear total is non-positive

### BUF Aircraft

- Module: `backend/internal/standards/buf/aircraft`
- Internal precision:
  - raw `float64`, no intermediate rounding
- Export boundary:
  - raw `float64` values are persisted in receiver tables and rasters
- Regression boundary:
  - golden snapshots round receiver coordinates, receiver height, `Lday`, `Levening`, `Lnight`,
    and `Lden` to 6 decimals
- Explicit tolerance:
  - `Lden` analytical check uses absolute tolerance `1e-9`
- Sentinel convention:
  - `Lden` returns `-999.0` if the linear total is non-positive

### BEB Exposure

- Module: `backend/internal/standards/beb/exposure`
- Internal precision:
  - raw `float64`, no intermediate rounding
- Upstream indicator usage:
  - BEB consumes upstream `Lden` and `Lnight` values without pre-rounding
- Threshold convention:
  - affected dwellings / persons are counted with an inclusive threshold test:
    - `levelDB >= thresholdDB`
  - no epsilon is applied to this comparison
- Occupancy aggregation convention:
  - if `estimated_dwellings` is missing, floors are derived as `ceil(height_m / floor_height_m)`
  - the floor count is clamped to a minimum of `1`
  - estimated persons default to `estimated_dwellings * persons_per_dwelling`
- Summary convention:
  - summary totals are sums of the raw building-level values
  - no final rounding is applied before export
- Export boundary:
  - building tables and the aggregate raster persist raw `float64` values
- Regression boundary:
  - golden snapshots round building coordinates, `Lden`, `Lnight`, estimated counts, affected
    counts, and summary values to 6 decimals
- Geometry fallback tolerance:
  - polygon centroid fallback treats a polygon as degenerate when absolute double area is below `1e-12`

## Current Reporting Limitation

Unlike `rls19-road`, these preview modules do not yet record a dedicated user-facing reporting
precision such as `0.1 dB` in provenance or run summaries. Their current contract is therefore:

- raw `float64` at export/runtime
- 6-decimal rounding only for golden regression fixtures
- `1e-9` only where an explicit analytical floating-point assertion exists in tests

If a later reporting phase introduces rounded public display values, that will be a new reporting
boundary layered on top of the current persisted raw outputs.

### Schall 03

- Module: `backend/internal/standards/schall03`
- Internal precision:
  - raw `float64`, no intermediate rounding
- Export boundary:
  - raw `float64` values are persisted in receiver tables and rasters
- Reporting intent:
  - provenance records `reporting_precision_db = 0.1`
  - provenance records `reporting_rounding = round-half-away-from-zero at report boundary`
  - this is an intended public reporting boundary, not a current export-rounding step
- Regression boundary:
  - golden snapshots round receiver coordinates, receiver height, `LrDay`, and `LrNight` to 6 decimals
- Explicit tolerance:
  - monotonic/behavioral checks currently use strict comparisons without epsilon
