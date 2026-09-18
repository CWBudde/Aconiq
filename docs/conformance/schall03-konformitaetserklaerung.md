# Schall 03 Konformitätserklärung — Aconiq

Status: DRAFT — Eisenbahn Strecke + Straßenbahnen + Rangier- und Umschlagbahnhöfe + Reflexionen + Abschirmung scope (Phase 20 + 20a + 20b + 20c + 20d)

## Software

- Name: Aconiq
- Module: `schall03`
- Module version: `anlage2-2014-strecke-v1` (stamped as `model_version` on every normative run)
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
- Extent of the speed clamp (Nr. 5.3.2): scoped to Weichen, Kreuzungen und Haltestellen an Strecken plus 25 m on either side, declared per segment via `features` and applied by splitting the segment at the zone boundaries; a segment declaring no features keeps the clamp over its whole length ✓
- Permanently slow section exception (Nr. 5.3.2): sections with dauerhaft v ≤ 30 km/h use v = 30 km/h instead of clamping to 50 km/h (`PermanentlySlow` flag) ✓
- Curve noise penalty (Nr. 5.3.2): K_L = +4 dB for curve radii r < 200 m ✓

**Assessment (Gl. 37–38)**

- Gl. 37–38 for Straßenbahnen use the same formula structure as Gl. 33–34; supported via the existing Beurteilungspegel pipeline
- K_S = 0 dB. The Schienenbonus was abolished by the 11. BImSchG-Änderungsgesetz (BGBl. 2013 I S. 1943), effective 2019-01-01 for Straßenbahnen — the same amendment that removed it for Eisenbahnen in 2015.

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
- Gl. 35–36 addieren K_S ausschließlich zum Streckenanteil, nicht zum Rangierbahnhofanteil ✓. Der Wert ist K_S = 0 dB: der Schienenbonus wurde durch das 11. BImSchG-Änderungsgesetz (BGBl. 2013 I S. 1943) abgeschafft — für Eisenbahnen zum 01.01.2015, für Straßenbahnen zum 01.01.2019. Die Termstruktur bleibt erhalten, der Zuschlag ist null.

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
- Geschlossene Gebäudegrundrisse als abschirmende Hindernisse ✓ — der Strahl durchstößt Vorder- und Rückwand, die Gummibandmethode behält beide Kanten, e ist die Gebäudetiefe und es gilt die Doppelbeugung nach Gl. 22 mit D_z ≤ 25 dB.
- Seitliche Beugung je Hindernis, nicht je Wandtafel ✓ — Wandtafeln mit gemeinsamer `ObstacleID` bilden ein Hindernis. Seitenkanten sind die freien Enden eines offenen Hindernisses beziehungsweise die äußersten Silhouettenpunkte eines geschlossenen; ein innen liegender Wandpunkt ist keine Seitenkante, weil der Weg um ihn herum durch das Hindernis hindurch führte. Ein `BarrierSegment` ohne `ObstacleID` ist ein Hindernis für sich, so dass eine einzelne Wandtafel weiterhin um beide Enden umbeugt wird.
- Eine reflektierende Fläche schirmt ihre eigene Reflexion nicht ab ✓ — ein gespiegelter Strahl durchstößt seinen Reflektor bauartbedingt; genau dieser Durchstoßpunkt ist der Reflexionspunkt und keine Beugungskante. Ein Gebäude ist Hindernis und Reflektor zugleich, weshalb die `ObstacleID` auch an der `ReflectingWall` hängt und die Tafeln dieses Hindernisses vor der Beugungsrechnung aus der Hindernisliste des jeweiligen Spiegelwegs entfernt werden. Eine `ReflectingWall` ohne `ObstacleID` gehört zu keinem benannten Hindernis; dann wird nichts entfernt.

#### Software-Version

`phase20d-normative-barrier-diffraction-v2`

## Reachability from the CLI

Everything checkmarked above is library behaviour. What a user gets from
`aconiq run --standard schall03` depends on which of two chains the run
resolves to, and the two are not variants of one another.

| `schall03_engine` | Chain                                                              | `compliance_boundary`                         |
| ----------------- | ------------------------------------------------------------------ | --------------------------------------------- |
| `auto` (default)  | Normative when the model carries `schall03_operations`; else fails | `anlage2-2014-strecke-eisenbahn-strassenbahn` |
| `normative`       | Same, but fails rather than resolving anything else                | `anlage2-2014-strecke-eisenbahn-strassenbahn` |
| `preview`         | `BuiltinDataPack()` — invented spectra, **not** Schall 03          | `baseline-preview-no-normative-tables`        |

The resolved engine, model version and compliance boundary are written to
`provenance.json` and `run-summary.json` for every run; the preview chain also
writes a warning line to `run.log`. Before 28 August 2026 the CLI reached the
preview chain unconditionally while stamping a normative-sounding model version
beside a preview compliance boundary, so this document's checkmarks described
code no `aconiq run` invocation could reach. See PLAN.md Priority 2.

### Reachable from the CLI today

- Eisenbahn and Straßenbahn Strecken: full emission chain, propagation chain,
  barrier diffraction and reflection, driven from GeoJSON `source` features
  carrying `schall03_operations` (see `docs/geojson-schema-v1.md`).
- Shielding from `barrier` features and, unconditionally, from `building`
  features: every outer-ring edge becomes a wall panel of the building's
  height, and all panels of one ring form one obstacle. Inner rings are
  courtyards and are ignored. Reflection stays opt-in — from `barrier` features
  marked `schall03_reflective` and from `building` features marked
  `schall03_reflecting_wall` — because it raises levels for every model that
  never asked for it.
- The Nr. 5.3.2 substitution extent, from `schall03_track_features` on the
  `source` feature — the Weichen, Kreuzungen und Haltestellen the 50 km/h
  substitute speed is scoped to.

### Library-only, not reachable from the CLI

- **Rangier- und Umschlagbahnhöfe (Nr. 4.8, Phase 20b).** `rangierbahnhof.go`
  and the Beiblatt 3 tables have no GeoJSON representation and no run-pipeline
  branch; Gl. 35–36 combined assessment is therefore unreachable from a run.
- **Measured vehicle data (Nr. 9).** `measured_vehicle.go` accepts it; no input
  format carries it.

### Not yet supported (deferred)

| Feature                                  | Reason deferred |
| ---------------------------------------- | --------------- |
| Section 9 measurement-based vehicle data | Out of scope    |

## Evidence

- CI-safe test suite: repo-authored synthetic scenarios covering emission (straight track, bridge, bridge combined with Feste Fahrbahn, a 40 km/h Eisenbahn line, Straßenbahn, Straßenbahn Langsamfahrstelle, Straßenbahn Haltestellenbereich), propagation (free field, two-receiver distance check, water body, single barrier, barrier plus reflecting wall, a dominant lateral diffraction path, a three-edge barrier scene, a reflective barrier reaching Gl. 20's D_refl, a closed building footprint whose front and back wall form the double edge of Gl. 22), and full assessment including Straßenbahn full-chain
- Suite location: `backend/internal/qa/acceptance/schall03/testdata/ci_safe_suite.json`
- No official conformance test suite exists for Schall 03; comparison with hand-calculated reference values used for unit tests

## Tolerances

- Comparison tolerance for golden snapshot tests: 0.0001 dB (numerical identity)
- Expected precision for real calculations: within 0.1 dB of hand-calculated reference values

## Known limitations and deviations

1. **Line source integration step**: Subsegment length is variable (auto-computed from track geometry); this may introduce minor numerical differences vs. implementations using a fixed step. Results converge to the same value as step length decreases.
2. **Ground absorption**: Both A_gr,B (land, Gl. 14) and A_gr,W (water body, Gl. 16) are implemented. The water body fraction is specified per `TrackSegment` via `water_body_fraction_w` (0–1), which is a simplification: in a full terrain model, water fractions would be computed per propagation path.
3. **Spiegelwege und Abschirmung auf Spiegelwegen (Gl. 27–28 in Kombination mit Nr. 6.5)**: Spiegelquellen-Reflexionen nach Gl. 27–28 werden bis zur 3. Ordnung berücksichtigt, auch zusammen mit Abschirmung nach Nr. 6.5. Die Beugungsprüfung eines Spiegelwegs läuft entlang des **vollständig aufgefalteten** Strahls, also von der Spiegelquelle der **letzten** Reflexion — der an allen Wänden des Wegs der Reihe nach gespiegelten Quelle — zum Immissionsort. Genau dieser Strahl liefert auch die angesetzte Ausbreitungsentfernung, sodass Beugung und Ausbreitung in jeder Ordnung denselben Strahl beschreiben. Die Spiegelquelle der **ersten** Reflexion würde nur für die 1. Ordnung genügen; ein früherer Stand setzte sie für jede Ordnung an und prüfte Wege 2. und 3. Ordnung damit an einem Strahl, den der Schall nicht durchläuft. Hindernisse auf dem tatsächlichen Spiegelweg blieben dabei unberücksichtigt, der berechnete Pegel fiel entsprechend zu hoch aus.
   Die Hindernisse werden für diese Prüfung **nicht mitgespiegelt**: der aufgefaltete Strahl wird gegen die Tafeln an ihren realen Koordinaten geprüft. Für das letzte Teilstück (letzter Reflexionspunkt → Immissionsort) ist das exakt, für die vorangehenden Teilstücke eine Näherung; sie gilt unverändert für alle Ordnungen einschließlich der ersten.
4. **Mittlere Höhe des Ausbreitungswegs h_m (Gl. 14/15)**: Gl. 15 definiert h_m = S/d, mit S der Fläche zwischen dem Ausbreitungsweg und dem Geländeprofil (Bild 4). Bis zu dieser Änderung führte Aconiq für die Schall-03-Ausbreitung kein Geländeprofil mit und wertete stattdessen den **ebenen Sonderfall** aus, bei dem die Fläche unter dem geradlinigen Weg Quelle–Immissionsort ein Trapez ist und S/d exakt auf (h_g + h_r)/2 zusammenfällt — unabhängig davon, was zwischen den beiden Enden tatsächlich liegt. Da h_m in Gl. 14 mit negativem Vorzeichen steht, bedeutet ein zu großes h_m **zu wenig** Bodendämpfung und damit einen zu hohen Pegel, bis die Begrenzung auf ≥ 0 dB greift und die Bodendämpfung vollständig entfällt.

   Die Abweichung ist geschlossen, **soweit ein Geländemodell vorliegt**. S/d zerfällt in zwei unabhängig bildbare Mittelwerte: die mittlere Höhe des geradlinigen Wegs über der Bezugsebene, also (h_g + h_r)/2, abzüglich der mittleren Geländehöhe über derselben Ebene. Letztere wird je Paar (Teilstück × Immissionsort) einmal aufgelöst als

   > (Geländehöhe unter dem Teilstück + Geländehöhe unter dem Immissionsort) / 2 + mittlere Erhebung des Geländes über dieser Sehne

   (`terrain.MeanRiseAboveChord`, nomineller Abtastschritt 25 m). Das Gelände wird dabei gegen die eigenen beiden Endpunkte gemessen und nicht absolut, sodass ein DGM seine **Form** und nicht seinen Höhenbezug beisteuert. Die Höhe unter dem Immissionsort ist dessen `TerrainZ` (Abweichung 10), die unter dem Teilstück ein DGM-Wert an dessen Grundrisspunkt — ausdrücklich **nicht** `elevation_m`, denn das ist die Schienenoberkante und nicht der Grund darunter. h_m wird bei 0 m begrenzt: Gl. 15 kennt keinen Weg unterhalb des mittleren Geländes, und eine Extrapolation ließe die Klammer in Gl. 14 unbegrenzt wachsen; Gelände, das über den Weg steigt, ist ein Fall der Abschirmung nach Nr. 6.5 und nicht der Bodendämpfung.

   Wirkung, end-to-end über die CLI gemessen: an einem freistehenden Immissionsort 60 m neben der Strecke, unter dem der Weg eine 3 m tiefe Mulde quert, lag der bisherige ebene Ansatz **1,16 dB zu niedrig** (L_r,Tag 57,65 dB gegen 58,81 dB). Ein Geländerücken zwischen Quelle und Immissionsort wirkt in die Gegenrichtung.

   **Verbleibende Näherungen**, die diese Änderung nicht aufhebt:
   - Ohne Geländemodell — und ebenso für jeden Ausbreitungsweg, dessen beide Stützpunkte nicht **gemeinsam** im Raster liegen — fällt h_m auf den ebenen Sonderfall zurück. Der Bodenterm gilt ganz oder gar nicht: ein Weg, den das DGM nur an einem Ende erreicht, erhält keinen anteiligen Beitrag, denn `TerrainZ` ist nur dort eine Messung, wo das DGM den Immissionsort überdeckt (andernfalls tritt das Mittel der überdeckten Immissionsorte an seine Stelle, Abweichung 10), und die halbe Differenz zwischen diesem ersatzweisen Mittel und einem echten Rasterwert am Quellenende wäre eine Korrektur auf eine an keinem Ende gemessene Zahl. Das rechnet **bit-identisch** wie der bisherige Stand; eine Fehlstelle wird nicht als Höhe 0 gelesen. Ein normativer Lauf ohne DGM vermerkt beides in `run.log`.
   - Die Bezugsebene für D_Ω (Gl. 9), für d (Gl. 11/12) und für die Abschirmprüfung nach Nr. 6.5 bleibt **eben** (siehe Abweichung 10). Gl. 9 spiegelt die Quelle in einer Ebene, und Nr. 6.5 braucht zwei Endpunkthöhen in einem Bezugssystem; beide sind Endpunktgrößen und haben kein Profil, an dem sie sich ausrichten könnten. Insbesondere schirmt Gelände nicht ab: ein Damm oder eine Kuppe zwischen Quelle und Immissionsort erhöht die Bodendämpfung, erzeugt aber kein D_z. Das wirkt **erhöhend** auf den berechneten Pegel und damit zugunsten des Schutzguts.
   - Auf **Spiegelwegen** wird h_m über dem realen Korridor Teilstück → Immissionsort gemittelt, nicht über dem aufgefalteten Strahl. Der Ursprung eines aufgefalteten Strahls ist eine Konstruktion und kein Ort; das Gelände unter ihm ist Gelände, über das der Schall nie läuft. RLS-19 grenzt an derselben Stelle gleich ab (`hmRefl`).
   - `ComputeYardPointSourceImmission` (Rangier- und Umschlagbahnhöfe, Nr. 7) wertet weiterhin den ebenen Sonderfall aus. Diese Schnittstelle ist reine Bibliothek: kein CLI-Befehl und keine API-Route baut eine `YardPointImmissionInput`, es gibt also keinen Lauf, der ihr ein DGM reichen könnte.

5. **Lateral diffraction around a Seitenkante**: The diffraction point on a vertical side edge is placed at the height that minimises the detour length (linear interpolation between source and receiver height), clamped to the barrier top. The standard specifies the geometry only through Gl. 26 and Bild 5; this is the shortest-path reading of it.
6. **Extent of the Nr. 5.3.2 substitute speed**: Nr. 5.3.2 restricts the 50 km/h substitution to Weichen, Kreuzungen und Haltestellen an Strecken (each plus 25 m on either side). Aconiq models that extent: a `TrackSegment` may declare those features via `features` (`weiche`, `kreuzung`, `haltestelle`, each a point projected onto the track centerline), and the segment is split at the resulting zone boundaries — the 25 m either side of a feature carry the substitute speed, the stretches between them are computed at the real track speed. Overlapping zones are merged, and a feature more than 25 m from the centerline is rejected rather than projected onto a track it is not on. This deviation is closed.
   A segment that declares **no** features keeps the substitution over its whole length. That fallback is deliberate and is the one remaining deviation on this point: reading "no features declared" as "no substitution anywhere" would lower levels for every existing model without the modeller having said anything, and Anmerkung 1 to Nr. 5.3.2 justifies the raised speed by noise sources — Weichen, Isolier- und Schweißstöße, Beschleunigungs- und Bremsstrecken — that are not strictly confined to the listed lengths. The precision is therefore opt-in, and a model that declares its Weichen und Haltestellen gets the narrower, normative extent.
   `permanently_slow` and declared features are mutually exclusive and rejected together, because the "dauerhaft v ≤ 30 km/h" exception presupposes a section carrying no Weichen, Kreuzungen oder Haltestellen.
   The related deviation on the Eisenbahn side is closed: the 50 km/h floor is no longer applied to Eisenbahn segments. Nr. 4.3 prescribes no substitute speed below the 70 km/h that applies im Bereich von Personenbahnhöfen und Haltepunkten, which is implemented via the `is_station` flag; slow Eisenbahn lines are now computed at their real speed. The previous behaviour read 2.2 dB high at 1000 Hz and 5.5 dB high at 2000 Hz for a 30 km/h approach.
7. **Fahrbahnart default**: `FahrbahnartType` and `SFahrbahnartType` are numbered so that Schwellengleis — the reference track type of Nr. 4.4 and Nr. 5.4, carrying no c1 correction — is the zero value. A `TrackSegment` whose JSON omits `fahrbahn` or `s_fahrbahn` therefore receives no Tabelle 7 or Tabelle 15 correction. Before 28 August 2026 the zero value was Feste Fahrbahn (Eisenbahn) and straßenbündiger Bahnkörper (Straßenbahn), so an omitted field silently added +7/+3 dB Schiene and +1 dB Reflexion, respectively up to +8 dB at 1000 Hz. Scenario files no longer carry those ordinals at all: `fahrbahn`, `s_fahrbahn`, `surface` and the wall `surface` are read and written as names from the stable vocabulary (for example `schwellengleis`, `feste-fahrbahn`), and a bare JSON number is rejected with an error naming the accepted values. A file written against the old numbering therefore fails loudly instead of being misread, which is why no schema-version field or migration entry is needed for it.
8. **Seitliche Beugung um einen geschlossenen Grundriss (Silhouetten-Näherung)**: Ein geschlossenes Hindernis hat keine freien Enden. Aconiq nimmt als Seitenkanten je Seite den Grundrisspunkt mit dem größten vorzeichenbehafteten Lotabstand von der Verbindungslinie Quelle–Immissionsort und wertet Gl. 18/26 für diesen einen Punkt aus. Das gespannte Seil (Bild 5) kann jedoch je Seite zwei oder mehr Eckpunkte berühren, etwa Vorder- und Hinterkante derselben Gebäudeseite. Der modellierte Umweg ist deshalb **kürzer** als der tatsächliche, z und damit A_bar fallen **kleiner** aus, und der berechnete Pegel liegt **höher** als der normgerechte — die Abweichung wirkt also zugunsten des Schutzguts. Die Mehrfachbeugung in der Grundrissebene (Laufweglänge e in der Ebene, C₃, D_z ≤ 25 dB nach Gl. 22) ist nicht umgesetzt; sie kann Ergebnisse gegenüber dem Auslieferungsstand nur leiser machen.
   Gl. 20's D_refl wird auf Gebäudewände **nicht** angewendet: die Gleichung ist auf reflektierende Schallschutzwände mit absorbierendem Sockel beschränkt, und ein Wohnhaus ist keine Schallschutzwand. Gebäudetafeln tragen daher `Reflective: false`.
   Gitterimmissionsorte innerhalb eines Gebäudegrundrisses werden nicht maskiert (`run_receivers.go` kennt keine Gebäudemaskierung). Im Auto-Raster liegen solche Punkte im Gebäude und werden seit dieser Änderung von der einen Ringkante abgeschirmt, die ihr Strahl durchstößt, statt frei zu stehen. RLS-19 verhält sich ebenso; der Wert an einem Punkt innerhalb eines Gebäudes ist in beiden Fällen nicht als Immissionsort zu lesen.
9. **Ausschluss je Hindernis, nicht je Wandtafel (Spiegelwege)**: Aus der Hindernisliste eines Spiegelwegs werden alle Tafeln des reflektierenden Hindernisses entfernt, nicht nur die eine Tafel, an der reflektiert wurde. Bei einem geschlossenen Grundriss schirmt damit auch die Rückwand desselben Gebäudes den an seiner Vorderwand reflektierten Strahl nicht ab, obwohl dieser sie durchstößt. Die Abweichung wirkt **erhöhend** auf den berechneten Pegel und damit zugunsten des Schutzguts. Fremde Hindernisse im Spiegelweg schirmen unverändert ab. RLS-19 ist an dieser Stelle gleich abgegrenzt (`barriersExcluding`).
10. **Bezugsebene der Ausbreitungsgeometrie (Gl. 9, Gl. 11/12, Gl. 14/15)**: `elevation_m` einer Strecke ist eine **absolute** Höhe — der SoundPLAN-Import schreibt die Z-Koordinate der Schienenoberkante hinein —, die Höhe eines Immissionsorts (`height_m`) dagegen eine Höhe **über dem Grund, auf dem er steht**. Bis zu dieser Änderung wurden beide Zahlen unmittelbar gegeneinander gerechnet: h_g ging als absolute Höhe in Gl. 9 und Gl. 14/15 ein, und d nach Gl. 11/12 war die Differenz zweier Zahlen aus verschiedenen Bezugssystemen. An einem Standort 400 m über NN ergab das h_m ≈ 204 m statt 3,75 m; die Klammer in Gl. 14 wurde negativ, die Begrenzung auf ≥ 0 dB griff und die **Bodendämpfung entfiel vollständig** (+4,1 dB). Zugleich wuchs d von 200,0 m auf 447,7 m und mit ihm A_div und A_atm (−7,0 dB). Beide Fehler wirken gegenläufig, im freien Feld verschob sich der Summenpegel deshalb nur um −2,7 dB. Die ungleich größere Wirkung hatte die **Abschirmung**: eine 4 m hohe Schallschutzwand steht einer Quelle in scheinbar 404 m Höhe nicht mehr im Weg, hinter der Wand lag der berechnete Pegel **7,0 dB zu hoch**.

    Die Ausbreitungsgeometrie wird jetzt gegen eine Grundebene gerechnet, deren absolute Höhe der Immissionsort mitführt (`ReceiverInput.TerrainZ`); die CLI belegt sie je Immissionsort aus dem importierten DGM. `h_g`, `h_r` und damit `D_Ω` und die Abschirmprüfung sind Höhen über dieser Ebene, `d` ist der Abstand zweier Punkte in ein und demselben Bezugssystem; `h_m` geht von dieser Ebene aus und wird um das Gelände unter dem Weg korrigiert (Abweichung 4). Eine Szene, deren Grundebene bei Z = 0 liegt — und damit jede Szene ohne DGM, in der `elevation_m` selbst als Höhe über Grund gemeint ist —, rechnet **bit-identisch** wie zuvor. Findet das DGM für einen Immissionsort keine Stütze, wird das Mittel der beprobten Immissionsorte angesetzt und die Zahl der Stützen in `run.log` vermerkt; eine Fehlstelle wird nicht als Höhe 0 gelesen. Diese Bezugsebene ist eben und bleibt es. Für `h_m` ist das seit der Umsetzung des Geländeprofils keine Näherung mehr — Gl. 15 misst dort gegen das abgetastete Gelände unter dem Weg (Abweichung 4) —, für `D_Ω` nach Gl. 9 und für die Abschirmprüfung nach Nr. 6.5 dagegen sehr wohl: beide setzen Endpunkthöhen über einer einzigen Ebene an. Genau das ist die verbleibende Näherung, die Abweichung 4 benennt.

    Die Korrektur greift, **soweit ein DGM vorliegt**. Genau dort, wo `elevation_m` als absolute Höhe entsteht, liegt derzeit keines vor: der SoundPLAN-Import erzeugt kein Geländemodell, sondern vermerkt lediglich die Herkunftsdatei der Höhendaten (Höhenlinien, Höhenpunkte, DGM-Dateien) im Importbericht. Ein aus SoundPLAN importiertes Projekt rechnet deshalb weiterhin gegen eine Grundebene bei Z = 0 und liest die Z-Koordinate der Schienenoberkante als Höhe über Grund — mit den oben bezifferten Folgen. Bis der Import ein DGM erzeugt, weist die Berechnung darauf hin: enthält ein normativer Lauf mindestens eine Strecke mit `elevation_m` ungleich 0 und kein Geländemodell, vermerkt `run.log` eine Warnung, die beide Lesarten benennt und auf `aconiq import --terrain` verweist. Abgelehnt wird der Lauf nicht: eine von Hand modellierte Brücke oder Dammlage, bei der `elevation_m` tatsächlich eine Höhe über Grund ist, bleibt zulässig. Umgekehrt wird ein Lauf abgelehnt, dessen DGM **keinen einzigen** Immissionsort überdeckt — Ursache ist in aller Regel ein CRS- oder Ausdehnungsfehler, und Z = 0 anzusetzen würde stillschweigend zur alten Bezugsebene zurückkehren.
