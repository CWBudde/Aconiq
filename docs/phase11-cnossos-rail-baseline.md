# Phase 11 CNOSSOS Rail Baseline

Status date: 2026-03-06

This phase introduces a deterministic CNOSSOS rail baseline module with typed source schema, rail emission logic, rail-specific propagation adjustments, and golden regression coverage.

## Implemented

- Package: `backend/internal/standards/cnossos/rail`
  - source schema:
    - `RailSource` (track centerline, traction, roughness, speed, braking, curve radius, bridge flag)
    - per-period traffic (`day`, `evening`, `night`) in trains/hour
  - emission:
    - rolling component
    - traction component
    - braking component
    - energetic summation
  - propagation:
    - distance-to-track coupling
    - attenuation chain: geometric + air + ground
    - rail adjustments: bridge correction and curve squeal correction
  - indicators:
    - `Lday`, `Levening`, `Lnight`
    - `Lden` day-evening-night aggregation
  - export:
    - receiver tables (`JSON`, `CSV`)
    - 2-band raster (`Lden`, `Lnight`)

## Standards Framework Integration

- `cnossos-rail` descriptor added to `backend/internal/standards/registry.go`.
- Descriptor includes:
  - version/profile metadata
  - supported source type (`line`)
  - supported indicators (`Lday`, `Levening`, `Lnight`, `Lden`)
  - run parameter schema

## Golden Regression

- Scenario fixture: `backend/internal/standards/cnossos/rail/testdata/rail_scenario.json`
- Golden snapshot: `backend/internal/standards/cnossos/rail/testdata/rail_scenario.golden.json`
- Regression test: `backend/internal/standards/cnossos/rail/rail_test.go`
