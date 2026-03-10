# Phase 19 ISO 9613-2 Baseline

Status date: 2026-03-10

This note records the initial Phase 19 groundwork for the deferred ISO 9613-2 implementation.

## Current Status

The repository now exposes an `iso9613` standards entry in the shared standards registry under:

- `backend/internal/standards/iso9613`

What is implemented now:

- a standards-framework descriptor for `noise run --standard iso9613`
- a narrow first-scope contract centered on industrial point sources only
- typed payloads for point sources, receivers, ground zones, and meteorological assumptions
- a runnable preview compute chain for point sources with deterministic energetic summation
- receiver-table and raster export for `LpAeq`
- provenance metadata that records the current preview boundary and intended reporting precision
- API visibility via `/api/v1/standards`
- one repo-authored acceptance fixture with deterministic golden coverage for the point-source preview scope

What is not claimed yet:

- normative conformance against ISO reference examples
- redistribution of restricted normative material beyond repo-safe metadata and boundary notes
- point/line/area parity, terrain-aware diffraction details, or a standards-faithful attenuation chain
- tolerance rules and versioned comparison policy for external validation assets

## First Delivery Scope

The initial engineering target is intentionally narrow:

- international industry calculations
- point sources only
- one exported scalar indicator: `LpAeq`
- homogeneous-ground scaffold input via one normalized `ground_factor`
- favorable propagation assumption only: downwind or moderate temperature inversion

Line sources, area sources, terrain diffraction details, and a standards-faithful attenuation chain remain follow-on work.

## Compliance Boundary

The current scaffold keeps the legal and technical boundary explicit:

- no normative ISO 9613-2 text is embedded
- no claim is made that current metadata equals a standards-faithful implementation
- the public repository only contains typed contracts, descriptor metadata, and provenance boundary markers
- any future restricted coefficients, example cases, or data packs must remain externally reviewable and legally safe before being bundled

Current provenance boundary values:

- `model_version=phase19-preview-v2`
- `compliance_boundary=phase19-iso9613-point-source-preview`
- `implementation_status=preview-point-source-run-wired`

## Rounding And Reporting

The intended reporting boundary is already documented for future implementation consistency:

- internal arithmetic should remain `float64`
- no intermediate rounding should be applied inside the future attenuation chain
- public reporting precision is intended to be `0.1 dB`

## Next Steps

The remaining Phase 19 work now moves from preview to stronger conformance:

- define normalized GeoJSON to ISO 9613 typed-input extraction for point sources
- replace the preview attenuation terms with a tighter ISO 9613-2 engineering-method chain
- add license-safe validation fixtures and golden acceptance coverage
- decide how ground zones, explicit barriers, and broader source types enter the first non-preview delivery
