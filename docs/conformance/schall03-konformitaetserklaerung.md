# Schall 03 Konformitätserklärung — Aconiq

Status: DRAFT — Eisenbahn Strecke scope (Phase 20)

## Software

- Name: Aconiq
- Module: `schall03`
- Version: `phase20-normative-eisenbahn-strecke-v1`
- License: MIT

## Standard

- Standard: Schall 03 (Anlage 2 zu §4 der 16. BImSchV)
- Full title: Berechnung des Beurteilungspegels für Schienenwege
- Legal basis: 16. BImSchV (Verkehrslärmschutzverordnung)
- Source document: Anlage 2 zu §4 der 16. BImSchV (amtliches Werk per §5 UrhG — normative coefficients embeddable directly)

## Scope

### Supported (Phase 20)

**Emission chain (Gl. 1–2)**

- Fz-Kategorien 1–10 (Eisenbahn), full Beiblatt 1 normative data
- Multi-Teilquelle per Fz: up to 11 sub-sources per vehicle category at 3 height levels (0 m, 4 m, 5 m above SO)
- Speed factor per source type (Rollgeräusch, aerodynamisch, Aggregat, Antrieb) — Table 6
- Track corrections c1 for Fahrbahnarten (Schwellengleis reference, feste Fahrbahn, feste Fahrbahn mit Absorber, Bahnübergang) — Table 7
- Surface corrections c2 (büG, Schienenstegdämpfer, Schienenstegabschirmung) — Table 8
- Bridge corrections K_Br and K_LM for types 1–4 — Table 9
- Curve noise Auffälligkeitskorrektur K_L / K_LA (r < 300 m, 300–500 m, ≥ 500 m) — Table 11
- 19 standard Zugarten with factory compositions — Table 4

**Propagation chain (Gl. 8–16)**

- Geometrical divergence A_div (Gl. 11)
- Atmospheric absorption A_atm (Gl. 12, octave-band α — Table 17)
- Ground attenuation A_gr,B over land (Gl. 14)
- Solid angle correction D_Ω (Gl. 9)
- Directivity D_I (Gl. 8)
- Line source integration: track subdivided into Teilstücke; energetic summation over subsegments

**Barrier diffraction (Gl. 18–26)**

- Single and double barrier: A_bar per Gl. 18–19
- Path difference z (Gl. 25 for parallel edges, Gl. 26 for non-parallel)
- Meteorological correction K_met (Gl. 23–24)
- Multiple diffraction factor C₃ (Gl. 22)
- Reflective barrier correction D_refl (Gl. 20)
- D_z caps: 20 dB (single barrier), 25 dB (double barrier)
- C₂ = 40 (normative value for Strecke)

**Assessment (Gl. 29–34)**

- Beurteilungspegel L_r,Tag and L_r,Nacht (Gl. 33–34)
- K_S = 0 dB (Schienenbonus abolished for Eisenbahnen since 2015 amendment)
- Indicators: L_p,Aeq,Tag, L_p,Aeq,Nacht (unrounded), L_r,Tag, L_r,Nacht

### Not yet supported (deferred)

| Feature | Reason deferred |
|---------|----------------|
| Straßenbahnen (Fz 21–23, Beiblatt 2) | Phase 20a |
| Rangier- und Umschlagbahnhöfe (Table 10, Beiblatt 3) | Phase 20b |
| Image-source reflections (Gl. 27–28, Table 18) | Phase 20c |
| Ground correction for water bodies A_gr,W (Gl. 16) | Phase 20 deferred |
| Section 9 measurement-based vehicle data | Out of scope |

## Evidence

- CI-safe test suite: 4 repo-authored synthetic scenarios covering emission (straight track, bridge), propagation (free field, two-receiver distance check), and full assessment
- Suite location: `backend/internal/qa/acceptance/schall03/testdata/ci_safe_suite.json`
- No official conformance test suite exists for Schall 03; comparison with hand-calculated reference values used for unit tests

## Tolerances

- Comparison tolerance for golden snapshot tests: 0.0001 dB (numerical identity)
- Expected precision for real calculations: within 0.1 dB of hand-calculated reference values

## Known limitations and deviations

1. **Line source integration step**: Subsegment length is variable (auto-computed from track geometry); this may introduce minor numerical differences vs. implementations using a fixed step. Results converge to the same value as step length decreases.
2. **Ground absorption**: Only A_gr,B (land) is implemented; A_gr,W (water body correction, Gl. 16) is not yet applied.
3. **Reflection paths**: Image-source reflections per Gl. 27–28 are not applied. Only direct propagation paths are computed.
