# Phase 10 CNOSSOS Road Baseline

Status date: 2026-03-06

This phase introduces a deterministic CNOSSOS road baseline module with typed source schema, period emission logic, propagation, indicators, and export helpers.

## Implemented

- Package: `backend/internal/standards/cnossos/road`
  - source schema:
    - `RoadSource` (centerline, speed, surface, gradient)
    - per-period traffic (`day`, `evening`, `night`) split by light/heavy classes
  - emission:
    - deterministic piecewise speed correction
    - surface correction
    - gradient correction
    - energetic summation of light/heavy contributions
  - propagation:
    - distance-to-linestring receiver coupling
    - attenuation chain: geometric + air + ground + barrier
  - indicators:
    - `Lday`, `Levening`, `Lnight`
    - `Lden` via day-evening-night aggregation
  - export:
    - receiver tables (`JSON`, `CSV`)
    - 2-band raster (`Lden`, `Lnight`)

## Standards Framework Integration

- `cnossos-road` descriptor added to `backend/internal/standards/registry.go`.
- Descriptor includes:
  - version/profile metadata
  - supported source type (`line`)
  - supported indicators (`Lday`, `Levening`, `Lnight`, `Lden`)
  - run parameter schema

## Notes

- This baseline is deterministic and test-covered.
- CLI `noise run` integration for `cnossos-road` is still pending model-mapping wiring.
- Public validation datasets and formal tolerance/rounding benchmarking remain tracked as open QA tasks in `PLAN.md`.
