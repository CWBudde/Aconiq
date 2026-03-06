# Phase 9 Standards Framework

Status date: 2026-03-06

Phase 9 introduces a modular standards framework so runs are resolved against explicit standard descriptors before execution.

## Implemented

- `backend/internal/standards/framework`
  - Standard descriptor model:
    - standard ID
    - versions and profiles
    - supported source types
    - supported indicators
  - Run parameter schema model:
    - parameter name/type/default/range
    - schema validation
    - parameter normalization (including default filling)
  - Version/profile resolution API
- `backend/internal/standards`
  - Local registry assembly of available standards
- `backend/internal/standards/dummy/freefield`
  - Exported descriptor with version/profile support:
    - version: `v0`
    - profiles: `default`, `highres`

## CLI Integration

`noise run` now:

- resolves `--standard`, `--standard-version`, `--standard-profile` via registry
- normalizes and validates `--param` map using the selected profile schema
- persists resolved standard/version/profile + normalized params into provenance

This enforces that provenance always contains explicit standard identity and parameter values actually used by the run.
