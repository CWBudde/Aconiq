# Phase 18 Schall 03 Baseline

Status date: 2026-03-10

This note records the current Schall 03 implementation status after the Phase 18 baseline groundwork pass.

## Current Status

The repository currently ships a deterministic Schall 03 planning-track baseline under
`backend/internal/standards/schall03`.

What is implemented now:

- typed rail-source extraction for `noise run --standard schall03`
- planning-period outputs `LrDay` and `LrNight`
- octave-band handling from `63 Hz` through `8000 Hz`
- deterministic line integration and cross-source energetic summation
- receiver-table and raster export
- provenance metadata for model version, data-pack version, band model, and reporting boundary

What is not claimed yet:

- full standards-faithful Schall 03 conformance
- redistribution of restricted normative coefficients or tables
- authoritative acceptance against an official external verification suite

## Compliance Boundary

The current implementation keeps the legal boundary explicit:

- no restricted Schall 03 text is embedded verbatim
- coefficient-like preview values are routed through a `DataPack` structure
- the bundled pack remains a repo-safe preview pack
- a future normative pack is expected to live outside the public repository unless redistribution rights are clear

Relevant code boundary:

- `backend/internal/standards/schall03/datapack.go`

## Minimum Normative Delivery Scope

Phase 18 completion should mean the following narrow, auditable delivery scope:

- planning-track rail line sources only
- indicators limited to `LrDay` and `LrNight`
- octave-band emission and propagation retained as the internal computation model
- source contract must include at least train class, traction type, track type, track form, roughness class, bridge flag, curve radius, speed, and day/night traffic
- all normative coefficients required for that scope must come from an external legally safe data pack or equivalent boundary
- user-facing reporting must round at the documented report boundary only; internal arithmetic remains `float64`

Anything broader than that should be treated as a follow-on phase, not a hidden requirement for Phase 18 sign-off.

## Rounding And Reporting

The current Schall 03 contract now mirrors the explicit reporting language used by the RLS-19 work:

- internal arithmetic stays in raw `float64`
- no intermediate rounding is applied during emission, propagation, or energetic summation
- exported artifacts still persist raw computed values
- intended public report precision is `0.1 dB`
- provenance records `reporting_precision_db=0.1` and `reporting_rounding=round-half-away-from-zero at report boundary`

This clarifies the intended public boundary without changing the persisted raw-output contract.

## Remaining Work

The remaining Phase 18 blockers are now limited to the actual normative replacement work:

- replace the bundled preview emission spectra with a standards-faithful Schall 03 emission chain for the chosen scope
- replace the preview propagation adjustments with the standards-faithful correction sequence
- acquire additional license-safe verification assets beyond the current synthetic fixture
- decide whether Schall 03 needs a dedicated conformance runner beyond the shared acceptance catalog
