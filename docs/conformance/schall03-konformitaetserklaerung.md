# Schall 03 Konformitätserklärung — Aconiq

Status: DRAFT — Eisenbahn Strecke + Straßenbahnen + Rangier- und Umschlagbahnhöfe + Reflexionen + Abschirmung scope (Phase 20 + 20a + 20b + 20c + 20d)

## Software

- Name: Aconiq
- Module: `schall03`
- Version: `phase20d-normative-barrier-diffraction-v1`
- License: MIT

## Standard

- Standard: Schall 03 (Anlage 2 zu §4 der 16. BImSchV)
- Full title: Berechnung des Beurteilungspegels für Schienenwege
- Legal basis: 16. BImSchV (Verkehrslärmschutzverordnung)
- Source document: Anlage 2 zu §4 der 16. BImSchV (amtliches Werk per §5 UrhG — normative coefficients embeddable directly)

## Scope

### Supported (Phase 20 — Eisenbahn Strecke)

**Emission chain (Gl. 1–2)**

- Fz-Kategorien 1–10 (Eisenbahn), full Beiblatt 1 normative data
- Multi-Teilquelle per Fz: up to 11 sub-sources per vehicle category at 3 height levels (0 m, 4 m, 5 m above SO)
- Speed factor per source type (Rollgeräusch, aerodynamisch, Aggregat, Antrieb) — Table 6
- Track corrections c1 for Fahrbahnarten (Schwellengleis reference, feste Fahrbahn, feste Fahrbahn mit Absorber, Bahnübergang) — Table 7
- Surface corrections c2 (büG, Schienenstegdämpfer, Schienenstegabschirmung) — Table 8
- Bridge corrections K_Br and K_LM for types 1–4 — Table 9, applied to Teilquellen 1 and 2 only, with the Table 7 rows 1–4 Fahrbahnart corrections suppressed on bridges per Nr. 4.6 ("Korrekturen für Fahrbahnarten nach Tabelle 7 Zeile 1 bis 4 sind nicht anzusetzen")
- Curve noise Auffälligkeitskorrektur K_L / K_LA (r < 300 m, 300–500 m, ≥ 500 m) — Table 11
- 19 standard Zugarten with factory compositions — Table 4

**Propagation chain (Gl. 8–16)**

- Geometrical divergence A_div (Gl. 11)
- Atmospheric absorption A_atm (Gl. 12, octave-band α — Table 17)
- Ground attenuation A_gr = A_gr,B + A_gr,W (Gl. 13): land absorption (Gl. 14) and water body correction (Gl. 16)
- Solid angle correction D_Ω (Gl. 9)
- Directivity D_I (Gl. 8)
- Line source integration: track subdivided into Teilstücke; energetic summation over subsegments

**Barrier diffraction (Gl. 18–26)**

- Single and double barrier: A_bar per Gl. 18–19
- Path difference z (Gl. 25 for parallel edges, Gl. 26 for non-parallel)
- Meteorological correction K_met (Gl. 23–24)
- Multiple diffraction factor C₃ (Gl. 22)
- Reflective barrier correction D_refl (Gl. 20), restricted per Gl. 20 to reflective walls at d_s ≤ 5 m with an absorbing base
- D_z caps: 20 dB (single barrier), 25 dB (double barrier)
- C₂ = 40 (normative value for Strecke)

**Assessment (Gl. 29–34)**

- Beurteilungspegel L_r,Tag and L_r,Nacht (Gl. 33–34)
- K_S = 0 dB (Schienenbonus abolished for Eisenbahnen since 2015 amendment)
- Indicators: L_p,Aeq,Tag, L_p,Aeq,Nacht (unrounded), L_r,Tag, L_r,Nacht

### Supported (Phase 20a — Nr. 5 Schallemissionen von Straßenbahnen)

**Emission chain — Straßenbahnen (Nr. 5.1–5.3)**

- Fz-Kategorien 21–23 (Beiblatt 2): Fz 21 Niederflurfahrzeuge, Fz 22 Hochflurfahrzeuge, Fz 23 U-Bahn-Fahrzeuge — full normative a_A and Δa_f per Teilquelle
- Speed factors embedded per Teilquelle via `B *BeiblattSpectrum` — Table 14 (Straßenbahn-specific b-values for Rollgeräusch, aerodynamisch, Aggregat, Antrieb)
- Track type corrections c1 for Straßenbahn Fahrbahnarten (3 types) — Table 15
- Bridge corrections K_Br for Straßenbahn bridge types (5 types) — Table 16
- Speed clamp (Nr. 5.3.2): effective speed floor of 50 km/h applied when operating speed < 50 km/h, Straßenbahn segments only ✓
- Permanently slow section exception (Nr. 5.3.2): sections with dauerhaft v ≤ 30 km/h use v = 30 km/h instead of clamping to 50 km/h (`PermanentlySlow` flag) ✓
- Curve noise penalty (Nr. 5.3.2): K_L = +4 dB for curve radii r < 200 m ✓

**Assessment (Gl. 37–38)**

- Gl. 37–38 for Straßenbahnen use the same formula structure as Gl. 33–34; supported via the existing Beurteilungspegel pipeline
- K_S = +5 dB (Schienenbonus retained for Straßenbahnen per current 16. BImSchV)

### Unterstützt (Phase 20b — Nr. 4.8 Rangier- und Umschlagbahnhöfe)

#### Beiblatt 3 — Schallquellen

- Kurvenfahrgeräusch (Linienschallquelle j=1, r ≤ 300 m) ✓
- Gleisbremsengeräusch — 9 Varianten (i=2 bis i=10, L_WA 72–110 dB) ✓
- Retardergeräusch Verzögerungsstrecke (Punktschallquelle i=11, L_WA=90 dB) ✓
- Retardergeräusch Beharrungsstrecke (Linienschallquelle j=2, L_WA=62+10·lg(n_ret)) ✓
- Retardergeräusch Rangieren auf Beharrungsstrecke (Linienschallquelle j=3, L_WA=72+10·lg(n_ret)) ✓
- Hemmschuhauflaufgeräusch (Punktschallquelle i=12, L_WA=95 dB) ✓
- Auflaufstoßgeräusch — modern (i=13, L_WA=78 dB) und klassisch (i=14, L_WA=91 dB) ✓
- Anreißen/Abbremsen loser Wagen (Linienschallquelle j=4, L_WA=75 dB) ✓

#### Emissionsberechnung

- Gl. 3: Einzelschallquelle (Punktschallquelle) ✓
- Gl. 4: Linienschallquelle ✓
- Gl. 5: Flächenschallquelle (Aggregation) ✓
- Gl. 6: Teilstück einer Linienschallquelle → Punktpegel ✓
- Gl. 7: Teilfläche einer Flächenschallquelle → Punktpegel ✓

#### Ausbreitungsberechnung

- Keine Richtwirkung D_I für Quellen in Rangier- und Umschlagbahnhöfen ✓
- Schirmmaß mit C₂=20 (statt C₂=40 für Strecken) ✓
- Gl. 30: Immissionspegelberechnung (Summation aller Quellbeiträge) ✓

#### Beurteilung

- Gl. 35–36: Kombinierter Beurteilungspegel (Rangierbahnhof + Strecke) ✓
- Schienenbonus K_S = −5 dB nur für den Streckenanteil (nicht für Rangierbahnhofanteil) ✓

#### Software-Version

`phase20b-normative-rangierbahnhof-v1`

### Unterstützt (Phase 20c — Nr. 6.6 Pegelerhöhung durch Reflexionen)

- Table 18: Absorptionsverlust an Wänden (4 Wandoberflächentypen: hart, Gebäude, absorbierend, hoch absorbierend) ✓
- Gl. 27: Fresnel-Zonenprüfung der Mindestabmessung des Reflektors (bei 63 Hz) ✓
- Gl. 28: Schallleistungspegel der Spiegelschallquelle (L*WA + D*ρ + D_Ir) ✓
- Reflexionen bis einschließlich 3. Ordnung ✓
- Spiegelpunktgeometrie mit Prüfung gleiche Seite von Quelle und Empfänger ✓
- Ausbreitungsberechnung entlang des Reflexionsweges (Gl. 8–16) ✓
- Energetische Summation von direktem und reflektiertem Beitrag (Gl. 29) ✓

#### Software-Version

`phase20c-normative-reflections-v1`

### Unterstützt (Phase 20d — Nr. 6.5 Abschirmung durch Hindernisse)

- Abschirmung durch Hindernisse (Gl. 17–26) im Ausbreitungsweg ✓
- Gummibandmethode (Upper Convex Hull) zur Auswahl maßgeblicher Beugungskanten ✓
- Einfachbeugung (D_z ≤ 20 dB) und Doppelbeugung (D_z ≤ 25 dB, C₃-Faktor Gl. 22) ✓
- Seitliche Beugung um Schirmenden (Gl. 18) ✓
- Minimum aus Beugung über Oberkante und seitlicher Beugung je Oktavband ✓
- D_refl-Korrektur nach Gl. 20 ✓ — ausschließlich für reflektierende Schallschutzwände im Abstand d_s ≤ 5 m mit absorbierendem Sockel der Höhe h_abs; absorbierende Wände und Wände mit d_s > 5 m behalten ihre volle Abschirmwirkung (Gl. 20 Anmerkung 5). Die Eigenschaft „reflektierend“ wird über das Feld `reflective` je `BarrierSegment` gesetzt; ohne Angabe gilt die Wand als absorbierend.
- e nach Bild 6 als Laufweglänge zwischen erster und letzter Schirmkante (e = e₁ + e₂ + e₃ …), nicht als Sehne zwischen den äußeren Kanten ✓
- Meteorologische Korrektur K_met (Gl. 23–24) ✓
- Barriereattenuation auf direkten Ausbreitungswegen ✓
- Barriereattenuation auf reflektierten Ausbreitungswegen (Spiegelquelle als Quelle) ✓
- Einheitliche Szenen-API: `ComputeNormativeReceiverLevelsWithScene(receiver, segments, walls, barriers)` ✓

#### Software-Version

`phase20d-normative-barrier-diffraction-v1`

### Not yet supported (deferred)

| Feature                                  | Reason deferred |
| ---------------------------------------- | --------------- |
| Section 9 measurement-based vehicle data | Out of scope    |

## Evidence

- CI-safe test suite: repo-authored synthetic scenarios covering emission (straight track, bridge, bridge combined with Feste Fahrbahn, a 40 km/h Eisenbahn line, Straßenbahn, Straßenbahn Langsamfahrstelle), propagation (free field, two-receiver distance check, water body, single barrier, barrier plus reflecting wall, a dominant lateral diffraction path, a three-edge barrier scene), and full assessment including Straßenbahn full-chain
- Suite location: `backend/internal/qa/acceptance/schall03/testdata/ci_safe_suite.json`
- No official conformance test suite exists for Schall 03; comparison with hand-calculated reference values used for unit tests

## Tolerances

- Comparison tolerance for golden snapshot tests: 0.0001 dB (numerical identity)
- Expected precision for real calculations: within 0.1 dB of hand-calculated reference values

## Known limitations and deviations

1. **Line source integration step**: Subsegment length is variable (auto-computed from track geometry); this may introduce minor numerical differences vs. implementations using a fixed step. Results converge to the same value as step length decreases.
2. **Ground absorption**: Both A_gr,B (land, Gl. 14) and A_gr,W (water body, Gl. 16) are implemented. The water body fraction is specified per `TrackSegment` via `water_body_fraction_w` (0–1), which is a simplification: in a full terrain model, water fractions would be computed per propagation path.
3. **Reflection paths**: Image-source reflections per Gl. 27–28 are supported up to 3rd order. Combined reflection + barrier diffraction paths are supported (barrier attenuation applied along reflected path using image source position).
4. **Mean path height h_m (Gl. 14/15)**: Gl. 15 defines h_m = S/d, with S the area between the propagation path and the terrain profile (Bild 4). Aconiq carries no terrain profile for Schall 03 propagation and evaluates the flat-ground special case instead, where the area under the straight source–receiver path is a trapezoid and S/d reduces exactly to (h_g + h_r)/2. Over sloping or undulating ground the value therefore deviates from the normative h_m; ground attenuation A_gr,B is affected accordingly. Removing this simplification requires terrain-profile sampling along each propagation path.
5. **Lateral diffraction around a Seitenkante**: The diffraction point on a vertical side edge is placed at the height that minimises the detour length (linear interpolation between source and receiver height), clamped to the barrier top. The standard specifies the geometry only through Gl. 26 and Bild 5; this is the shortest-path reading of it.
6. **Extent of the Nr. 5.3.2 substitute speed**: Nr. 5.3.2 restricts the 50 km/h substitution to Weichen, Kreuzungen and Haltestellen an Strecken (each plus 25 m on either side), whereas Aconiq applies it to the whole Straßenbahn segment whose Streckenhöchstgeschwindigkeit is below 50 km/h, unless the segment is marked `permanently_slow` (the "dauerhaft v ≤ 30 km/h" exception, which is applied). Modelling a partial substitution requires splitting the segment at the boundaries of those areas; the reading is deliberate, because Anmerkung 1 to Nr. 5.3.2 justifies the raised speed by noise sources — Weichen, Isolier- und Schweißstöße, Beschleunigungs- und Bremsstrecken — that are not confined to the listed lengths. This deviation is open.
   The related deviation on the Eisenbahn side is closed: the 50 km/h floor is no longer applied to Eisenbahn segments. Nr. 4.3 prescribes no substitute speed below the 70 km/h that applies im Bereich von Personenbahnhöfen und Haltepunkten, which is implemented via the `is_station` flag; slow Eisenbahn lines are now computed at their real speed. The previous behaviour read 2.2 dB high at 1000 Hz and 5.5 dB high at 2000 Hz for a 30 km/h approach.
7. **Fahrbahnart default**: `FahrbahnartType` and `SFahrbahnartType` are numbered so that Schwellengleis — the reference track type of Nr. 4.4 and Nr. 5.4, carrying no c1 correction — is the zero value. A `TrackSegment` whose JSON omits `fahrbahn` or `s_fahrbahn` therefore receives no Tabelle 7 or Tabelle 15 correction. Before 28 August 2026 the zero value was Feste Fahrbahn (Eisenbahn) and straßenbündiger Bahnkörper (Straßenbahn), so an omitted field silently added +7/+3 dB Schiene and +1 dB Reflexion, respectively up to +8 dB at 1000 Hz. Scenario files written against the old numbering must be renumbered (−1 → 0, 0 → 1, …).
