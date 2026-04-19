# ISO 9613-2 Module Baseline

Status date: 2026-03-28

## Current Status

The module at `backend/internal/standards/iso9613` implements the ISO 9613-2:1996 octave-band engineering method for industrial point sources.

What is implemented:

- Octave-band processing across 8 standard midband frequencies (63 Hz to 8 kHz)
- Geometric divergence A_div (Eq. 7)
- Atmospheric absorption A_atm (Eq. 8) with Table 2 coefficients and nearest-row lookup
- Ground effect A_gr (Eq. 9, Table 3) — general three-region model with functions a'(h), b'(h), c'(h), d'(h)
- Simplified ground effect (Eq. 10) as alternative calculation path
- Barrier screening formulas A_bar (Eq. 12-18) — D_z, C_3, z, K_met with pre-computed geometry input
- Meteorological correction C_met (Eq. 21-22) with configurable C_0
- A-weighted energetic summation (Eq. 5) with IEC 61672-1 A-weighting corrections
- Long-term level L_AT(LT) via Eq. 6
- Backward-compatible single-value fallback using 500 Hz attenuation terms
- Two exported indicators: LpAeq_DW (downwind) and LpAeq_LT (long-term)
- Typed payloads for point sources, receivers, ground zones, and meteorological assumptions
- Provenance metadata, receiver tables, raster export
- CLI integration via `aconiq run --standard iso9613`
- Acceptance fixtures with golden coverage

What is not implemented:

- Geometric barrier detection (ray-barrier intersection; formulas are ready, geometry detection is shared with RLS-19)
- Lateral diffraction around vertical edges (Section 7.4.3, Eq. 13)
- Reflections via image sources (Section 7.5, Eq. 19-20, Table 4)
- Line and area source subdivision (Section 4)
- Spatial ground zones (single global G used for all three regions)
- A_misc: foliage, industrial site, housing (Annex A, informative)
- Full ISO 9613-1 atmospheric absorption model (nearest-row Table 2 lookup used instead)

## Compliance Boundary

The module does not embed normative ISO 9613-2 text. Restricted coefficients and example cases are not bundled.

Current provenance boundary values:

- `model_version=iso9613-octaveband-v1`
- `compliance_boundary=iso9613-engineering-octaveband`
- `implementation_status=octaveband-point-source`

Known simplifications documented in the conformance boundary:

- Atmospheric absorption uses nearest-row Table 2 lookup rather than the full ISO 9613-1 model
- Ground effect uses a single global G for all three regions (source, receiver, middle)
- Barrier attenuation requires pre-computed diffraction geometry (no built-in ray-barrier intersection)

## Rounding and Reporting

- Internal arithmetic remains `float64`
- No intermediate rounding inside the attenuation chain
- Public reporting precision: `0.1 dB`

## Reference Documents

- ISO 9613-2:1996 (complete copy in `interoperability/ISO9613-2/`)
- ISO 9613-2:2024 (preview only, truncated at Section 7.1)
- Design document: `docs/plans/2026-03-28-iso9613-octave-band.md`

## Next Steps

See PLAN.md Priority 3 for deferred implementation and conformance tasks.
