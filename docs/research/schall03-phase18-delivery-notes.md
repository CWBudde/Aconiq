# Schall 03 Phase 18 Delivery Notes

Status date: 2026-03-10

This note captures the narrow set of decisions needed before Schall 03 can move from preview baseline to
standards-faithful Phase 18 completion.

## First Deliverable Scope

The first normative Schall 03 delivery should stay intentionally small:

- planning-track rail only
- line-source geometry only
- indicators only `LrDay` and `LrNight`
- no attempt to cover every rail/infrastructure variant in the first normative drop

That scope is small enough to validate and document, while still being meaningful for planning workflows.

## Normative Data Boundary

The Schall 03 module now uses a `DataPack` shape so the eventual normative coefficient set can be supplied
without hard-coding restricted tables into the public repository.

The pack boundary should carry at least:

- base octave-band emission spectra
- train-class adjustments
- traction adjustments
- track-form and roughness adjustments
- air-absorption band factors
- default propagation constants used for the chosen scope
- a pack version string and compliance-boundary label

## Fixture Tracking Rules

Until a dedicated Schall 03 conformance runner exists, acceptance assets should follow these rules:

- every Schall 03 fixture must remain license-safe and repository-distributable
- fixture provenance must say whether it is synthetic, derived, or externally attributable
- fixture updates must be reviewed as versioned evidence changes, not routine golden churn
- if tolerances diverge between future fixture classes, Schall 03 should get a dedicated runner instead of overloading the shared acceptance catalog

## Open External Work

These tasks remain outside the current repository-safe baseline:

- obtain legally safe Schall 03 verification cases
- determine which normative values can ship in a public external pack and which must remain user-supplied
- verify the final authoritative report-rounding language against the lawfully obtained Schall 03 source material
