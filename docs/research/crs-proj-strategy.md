# CRS / PROJ Strategy Evaluation (Phase 1)

Status date: 2026-03-06

## Decision Goal

Define a CRS strategy that balances numerical correctness and cross-platform portability for CLI-first MVP.

## Options

1. Pure Go first (no cgo / no system PROJ dependency)
- Pros: simple cross-platform builds, lower operational complexity.
- Cons: CRS transform coverage and geodetic precision may be limited depending on library choice.

2. PROJ via cgo
- Pros: mature projection coverage and established GIS interoperability.
- Cons: packaging complexity across Linux/macOS/Windows, higher CI and release burden.

## Phase 1 Recommendation

- Adopt pure-Go-first for MVP phases where GeoJSON import, validation, and early compute skeleton are primary goals.
- Keep a clear abstraction boundary in `internal/geo` so a PROJ-backed adapter can be introduced later if required by accuracy/compliance tests.

## Trigger to Revisit

Revisit the decision when either condition is met:
- Normative validation cases expose unacceptable projection errors.
- Required CRS transforms cannot be supported with acceptable quality in pure Go.
