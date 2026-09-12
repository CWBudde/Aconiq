# RLS-19 Konformitaetserklaerung — Aconiq

Status: DRAFT — not yet submitted

## Software

| Field      | Value                             |
| ---------- | --------------------------------- |
| Name       | Aconiq                            |
| Module     | rls19-road                        |
| Version    | (to be filled at release)         |
| License    | MIT                               |
| Repository | https://github.com/cwbudde/Aconiq |

## Standard

| Field           | Value                                                                |
| --------------- | -------------------------------------------------------------------- |
| Standard        | RLS-19 (Richtlinien fuer den Laermschutz an Strassen, Ausgabe 2019)  |
| Errata          | Korrekturblatt 2/2020 (Februar 2020) — all three corrections applied |
| Legal basis     | 16. BImSchV                                                          |
| TEST-20 version | 2.1 (July 2025)                                                      |
| FGSV catalogue  | 334/2                                                                |

## Supported scope

### Emission chain

| Step | Description                                                        | Status      |
| ---- | ------------------------------------------------------------------ | ----------- |
| E1   | Berechnung des Grundwertes (base value)                            | Implemented |
| E2   | Korrektur fuer Strassendeckschichten (surface/DStrO correction)    | Implemented |
| E3   | Korrektur fuer Laengsneigung (gradient correction)                 | Implemented |
| E4   | Knotenpunktkorrektur (junction correction)                         | Implemented |
| E5   | Mehrfachreflexionszuschlag (multiple reflection surcharge)         | Implemented |
| E6   | Schallleistungspegel eines Fahrzeugs (single-vehicle sound power)  | Implemented |
| E7   | Laengenbezogener Schallleistungspegel (length-related sound power) | Implemented |

### Parking sources (§3.4)

| Step | Description                                                           | Status      |
| ---- | --------------------------------------------------------------------- | ----------- |
| P1   | Flaechenbezogener Schallleistungspegel (Eq. 10, corrected form)       | Implemented |
| P2   | Zuschlag D_P,PT je Parkplatztyp (Tabelle 6: Pkw/Motorrad/Lkw-Omnibus) | Implemented |
| P3   | Standardbewegungsraten N (Tabelle 7: P+R, Tank-/Rastanlagen)          | Implemented |
| P4   | Propagation from parking centroid (point source, §3.5 and §3.6 chain) | Implemented |

### Propagation

| Feature                                                         | Status      |
| --------------------------------------------------------------- | ----------- |
| Teilstueckverfahren (segment method)                            | Implemented |
| Configurable segment length (reference/check settings)          | Implemented |
| Length weighting 10·lg(l_i / l_0) with l_0 = 1 m (§3.5, Eq. 10) | Implemented |
| Single barrier shielding                                        | Implemented |
| Double barrier shielding                                        | Implemented |
| Terrain: cutting (Tieflage)                                     | Implemented |
| Terrain: elevated (Hochlage)                                    | Implemented |
| Terrain: ascending road                                         | Implemented |
| Terrain: receding road                                          | Implemented |
| Single explicit reflector                                       | Implemented |
| Up to 2 reflectors                                              | Implemented |

Note on the length weighting: the emission chain yields the length-related sound
power level L_m,E in dB(A)/m (step E7). Each Teilstueck of length l_i therefore
contributes L_m,E + 10·lg(l_i / l_0) with the reference length l_0 = 1 m, on the
direct path and on every mirrored path alike. The weight must not be normalised
by the total length of the source line; such a normalisation would make the whole
road radiate the sound power of a single one-metre section and would make the
result depend on how the source geometry happens to be split. Earlier revisions
of this module normalised by the total length; see "Implementation corrections"
below.

### Reflections (§3.6, Tabelle 8)

| Feature                                                             | Status      |
| ------------------------------------------------------------------- | ----------- |
| First-order reflections (Spiegelschallquellen 1. Ordnung)           | Implemented |
| Second-order reflections (Spiegelschallquellen 2. Ordnung)          | Implemented |
| Third-order reflections ignored per standard                        | Implemented |
| Height condition: h_R >= 1.0 m and h_R >= 0.3\*sqrt(a_R)            | Implemented |
| ReflectorType enum: FacadeOrReflecting (0.5 dB), ReflectionReducing | Implemented |
| (3.0 dB), StronglyReflectionReducing (5.0 dB) per Tabelle 8         | Implemented |
| Active-Teilstueck rule (Bild 14): segment-intersection enforcement  | Implemented |
| Applied to Parkplatzteilflaechen as well (Eq. 1, Eq. 3)             | Implemented |
| Mirror source shielded like an original source (§3.5, Eq. 11)       | Implemented |

### Indicators

| Indicator | Period      | Status      |
| --------- | ----------- | ----------- |
| LrDay     | 06:00-22:00 | Implemented |
| LrNight   | 22:00-06:00 | Implemented |

## Korrekturblatt 2/2020 — applied corrections

The Korrekturblatt Februar 2020 (FGSV 052, 2/2020) issued three corrections,
all of which are applied in this implementation:

| No. | Location              | Correction                                                                               | Applied in       |
| --- | --------------------- | ---------------------------------------------------------------------------------------- | ---------------- |
| 1   | p. 12, §3.2, Eq. 3    | Corrected form of the Beurteilungspegel formula (index alignment in sum notation)        | `propagation.go` |
| 2   | p. 16, §3.3.8, Eq. 9  | Index "refl" at D_refl corrected to subscript (typographic fix; formula value unchanged) | `emission.go`    |
| 3   | p. 17, §3.4.1, Eq. 10 | Corrected form: `L_W'' = 63 + 10·lg[N·n] + D_P,PT − 10·lg[P/1m²]` (area term added)      | `parking.go`     |

Note: The corrected Eq. 10 includes `−10·lg[P/1m²]` to express the
_area-related_ level L*W''. When propagating parking as a point source, the
implementation uses the \_total* sound power `L_W = L_W'' + 10·lg[P/1m²] =
63 + 10·lg[N·n] + D_P,PT`, which cancels the area term.

## Implementation corrections

Defects found in this module after its first draft, and how they were resolved.
Entries here are corrections to Aconiq, not to the standard.

| No. | Area                                      | Defect                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Resolution                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| --- | ----------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| C1  | §3.5 Teilstueckverfahren                  | The per-Teilstueck length weight was computed as `10·lg(l_i / l_total)` instead of `10·lg(l_i / l_0)` with `l_0 = 1 m`. Because the weights then summed to unity, a road of any length radiated the sound power of a 1 m section.                                                                                                                                                                                                                                                                                                                                                            | Corrected in `propagation.go` (direct and mirrored paths share the same weight). Error magnitude was `−10·lg(l_total / 1 m)`: −20 dB for a 100 m source line, −23 dB for 200 m, −30 dB for 1 km.                                                                                                                                                                                                                                                                                                                                                                                                                         |
| C2  | §3.3.6 Eq. 7c                             | The Lkw2 downhill branch used `(g+4)/(−8) · (v_Lkw2 − 10)/10`. The `− 10` belongs to the Eq. 7c _uphill_ branch (`v + 10`) and to Eq. 7b (`v − 20`), not here; Eq. 7c downhill is `(g+4)/(−8) · v_Lkw2/10`.                                                                                                                                                                                                                                                                                                                                                                                  | Corrected in `tables.go`. Lkw2 sound power was understated by `(g+4)/(−8) · 1.0 dB` — 0.125 dB at `g = −5 %`, 1.0 dB at the `g = −12 %` clamp.                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| C3  | §3.3.3 Anmerkung (Kräder), §3.3.6         | Kräder were routed into the Eq. 7a (Pkw) gradient branch. The Anmerkung prescribes Eq. 7c with `v_Pkw`, which has a different dead band (−4 % .. +2 % instead of −6 % .. +2 %) and much larger coefficients.                                                                                                                                                                                                                                                                                                                                                                                 | Corrected in `tables.go`; the speed argument was already `v_Pkw`. At `v_Pkw = 100 km/h` the per-Krad correction rises by 2.79 dB at `g = +5 %`, 1.25 dB at `g = −5 %` and 6.83 dB at `g = −10 %`.                                                                                                                                                                                                                                                                                                                                                                                                                        |
| C4  | §3.3.3 Anmerkung (Kräder), Tabellen 4a/4b | Kräder were given the Pkw row of Tabelle 4a, a Tabelle 4b Pflaster surcharge, or the legacy damaged-surface fallback. The Anmerkung prescribes `D_SD = 0` for Kräder, unconditionally.                                                                                                                                                                                                                                                                                                                                                                                                       | `SurfaceCorrection` now returns 0 for Krad before any table lookup. Sign of the previous error depended on the surface: a credited reduction of up to 5.5 dB per Krad on OPA 8, or a charged surcharge of up to 7.0 dB per Krad on rough Pflaster.                                                                                                                                                                                                                                                                                                                                                                       |
| C5  | Tabelle 4a                                | The `v ≤ 60 km/h` cells of "Betone nach ZTV Beton-StB 07 mit Waschbetonoberfläche" and "Lärmarmer Gussasphalt, Verfahren B" are crossed out in the table, but were transcribed as if the `v > 60 km/h` value applied in both bands.                                                                                                                                                                                                                                                                                                                                                          | Both rows now carry `notApplicableSurfaceCorrection()` below 60 km/h, matching every other partially-crossed row. A reduction of 1.4 dB (Beton, Pkw) to 2.3 dB (Beton, Lkw) was being credited where the table grants none.                                                                                                                                                                                                                                                                                                                                                                                              |
| C6  | §3.6 Tabelle 8                            | An untyped reflector with no explicit loss fell back to 1.0 dB, and imported buildings were seeded with the same value. 1.0 dB is in no row of Tabelle 8 — the table has only 0.5, 3.0 and 5.0 dB.                                                                                                                                                                                                                                                                                                                                                                                           | The fallback is now the facade row, 0.5 dB (`reflections.go`, `building.go`, `run_extract_rls19.go`). It is both the commonest untyped case and the conservative one — the smallest loss in the table, hence the highest resulting level. `ReflectionLossDB` remains a deliberate override.                                                                                                                                                                                                                                                                                                                              |
| C7  | Tabelle 4a                                | `SurfaceCorrection` returned 0 both for a cell the table crosses out — OPA at or below 60 km/h, and after C5 also Beton und lärmarmer Gussasphalt — and for a correction that is genuinely zero. A modeller who picked an inapplicable surface/speed pairing got a plausible number rather than a refusal.                                                                                                                                                                                                                                                                                   | `RoadSource.Validate` now refuses the pairing before any level is computed, deciding on the same table lookup the emission path reads and at the same effective speed, and naming the group, the speed field and the band the row is tabulated in. Kräder, a group with no traffic in either period, and an unstated surface stay exempt; nine of the seventeen surfaces can now produce the error. No receiver level moves — this is a refusal, not a recomputation.                                                                                                                                                    |
| C8  | §3.4.1 Tabellen 6 und 7                   | `vehicle_type` was a Tabelle 6 row ordinal tagged `omitempty`, so an omitted Parkplatztyp decoded as Pkw and applied `D_P,PT = 0` dB instead of the +5 dB Motorrad or +10 dB Lkw/Omnibus row; an omitted `movements_per_space_*` passed validation as `N = 0` and produced the −999 dB silence sentinel, reporting an occupied Parkplatz as inaudible. Two further `default:` branches returned 0 for an unrecognised Parkplatztyp. No omission was distinguishable from a deliberate input.                                                                                                 | Both Parkplatztypen are carried by name (`parking_type`, and the Tabelle 7 facility type), the unset value is explicit and refused by `ParkingSource.Validate`, and the movement rates are optional pointers so an explicit `0` stays legal while an omission is an error. Tabelle 6 and Tabelle 7 are declared data the digest reads directly rather than switch statements it re-evaluates. Previous silent error: up to 10 dB understated on the surcharge, and a whole source suppressed on the movement rate. No receiver level moves — no fixture exercised the path.                                              |
| C9  | §3.5, §3.5.1 Gl. 11, §3.6                 | A Spiegelschallquelle took `D_div + D_atm + D_gr` only. Nr. 3.5 states that the source of a propagation calculation may itself be a Spiegelschallquelle, and Gl. 11 gives one Dämpfung for a source, `D_A = D_div + D_atm + max{D_gr; D_z}` — so a barrier standing across a mirrored path did nothing at all, while the same barrier attenuated the direct path beside it.                                                                                                                                                                                                                  | `appendReflectedContribs` now measures `D_z` from the image source. A reflected ray crosses its own reflector by construction, so the path carries the reflectors it bounced off and `barriersExcluding` drops them first; without that, a building — barrier and reflector at once under one ID — would invent a diffraction edge exactly where the standard sees a reflection. `D_RV` stays outside the maximum: it is a term of Gl. 2 and Gl. 3, not a candidate in Gl. 11's `max`. Previous error: the whole `D_z` of the mirrored path, up to 3.26 dB on the receiver level in the fixtures below.                  |
| C10 | §3.2 Gl. 1 und Gl. 3, §3.6                | Parkplätze took no Spiegelschallquellen. The deviation was declared on the grounds that Nr. 3.4 does not prescribe reflections for a point-source lot and that the Nr. 3.6 machinery is built around Quelllinien. Both are wrong: Gl. 3 gives the Beurteilungspegel of all Parkplatzflächen as a sum over `j` of `L_W'',j + 10·lg[P_j] − D_A,j − D_RV1,j − D_RV2,j` and names `D_RV1,j` the Reflexionsverlust "für die Parkplatzteilfläche j nach dem Abschnitt 3.6", and Gl. 1 sums Fahrstreifenteilstücke and Parkplatzteilflächen "jeweils einschließlich etwaiger Spiegelschallquellen". | A Parkplatz now reaches the same helper a Teilstück does. `computeReflectedPaths` always took a point and a height, and the Bild 14 active-Teilstück rule is a property of its segment-intersection gate rather than a separate step — for a one-point lot it asks only whether the lot sees the wall. The one line-specific part, the per-metre emission plus `10·lg(l_i/l_0)`, is now composed by the caller. Previous error: a lot in front of a reflecting facade was understated by its mirrored contribution. No existing snapshot moves — no fixture carried both a lot and a reflector; the new P4 fixture does. |

All CI-safe expected snapshots under
`backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/` and the
`rls19-road` acceptance fixture were regenerated after C1. Expected values
recorded before that correction are void; the observed shift per fixture equals
`10·lg(l_total / 1 m)` of its source line, as the sources are otherwise
unchanged.

C2–C5 moved the snapshots again. C2 reached only the two receding-road fixtures
(`i9_*`, the only ones below `g = −4 %`), by +0.021 dB. C3 reached the five
fixtures that carry any gradient (`e3_*`, `i8_*`, `i9_*`), by +0.12 to +0.30 dB.
C4 reached 33 of the 34 fixtures — all but `e6_vehicle_sound_power`, the one
with `krad_per_hour = 0` — by +0.196 dB on SMA and +0.646 dB on OPA. C5 moved no
receiver level, because no fixture uses either affected surface type. C6 moved
no receiver level either: every CI-safe fixture sets `reflection_loss_db`
explicitly, which is why the suite never exposed the invented default. C7 moved
no fixture at all: the only surfaces under `rls19_test20/testdata` are SMA,
tabulated in both speed bands, and one OPA source running at 70–100 km/h, all
above the 60 km/h boundary — so the suite never exercised a crossed-out cell
either. C8 moved none either, because nothing could reach the Parkplatz path.

C9 moved exactly the three fixtures that carry both a reflector and something
able to shield its mirrored path, all downward:

| Fixture                | LrDay und LrNight  |
| ---------------------- | ------------------ |
| I4 barrier + reflector | −3.260 dB          |
| K2 building fronts     | −2.141 / −2.131 dB |
| K4 courtyard           | −0.208 / −0.231 dB |

K3 (perpendicular buildings) does not move: each mirrored path there is crossed
only by the building it reflected off, which is the one the exclusion drops. I3
and K5 carry no barrier and no second building, and every other fixture carries
no reflector. C10 moved nothing at all — no fixture combined a Parkplatz with a
reflector, which is what the new P4 fixture adds.

The per-Krad corrections in C3 and C4 are large, but the fixtures carry only
10 Krad/h of 1010 Kfz/h, so the receiver-level shift is an order of magnitude
smaller than the emission shift. A model with a realistic motorcycle share on a
steep grade will move considerably more.

## Reachability from the CLI

Everything checkmarked above is library behaviour. This section is what
`aconiq run --standard rls19-road` actually reaches.

Until the `rls19_parking_*` vocabulary landed, the Parkplatz rows P1–P4
described code no `aconiq run` invocation could reach: `ParkingSources` was
never assigned outside tests, there was no GeoJSON representation and no
extractor, and both the road extractor and `ComputeReceiverLevels` refused a
model with no line source. The rows were accurate about the library and
misleading about the product.

### Reachable from the CLI today

| Feature                         | Input                                                             |
| ------------------------------- | ----------------------------------------------------------------- |
| Road line sources (E1–E7, §3.5) | `source` features with `source_type: line`                        |
| Shielding                       | `barrier` features, and `building` features as barriers           |
| Reflections (§3.6)              | `building` features and explicit reflectors                       |
| Parkplätze (P1–P4, §3.4)        | `source` features with `source_type: area` plus `rls19_parking_*` |

### Known deviations on the Parkplatz path

- **No Teilflächen-Unterteilung.** Nr. 3.4 asks for a Parkplatz to be divided
  into Teilflächen (Bild 10), and Eq. 10 gives the area-related level of one
  Teilfläche. A feature is modelled as a single point source at its centroid,
  which is the un-subdivided case. A modeller who needs the subdivision states
  each Teilfläche as its own feature with its own `n`; the software does not
  divide one for them.
- **The area term is not evaluated.** `L_W'' = … − 10·lg[P/1m²]` and the
  conversion back to a total sound power cancel exactly, so the Stellplatzfläche
  is read from the polygon and validated but does not enter the level. See the
  Korrekturblatt note above.

### Known deviations on the reflection path (Nr. 3.6)

These apply to road Teilstücke and Parkplatzteilflächen alike, since C9 and C10
put both on one chain.

- **A reflecting surface does not shield its own mirrored path.** The exclusion
  that keeps a reflector from being read as a diffraction edge is per reflector,
  not per facade, so another wall of the same building is dropped with it. The
  direction is conservative: the level is over-predicted, never under.
- **No terrain-edge shielding and no terrain-following ground term on a mirrored
  leg.** The mean height of a reflected path is the flat-ground
  `(z_Quelle + z_Immissionsort)/2`, and terrain edges are not applied to it,
  because the image path does not lie over the sampled ground profile. Both
  conventions are the same one, applied consistently.

### Not reachable from the CLI

- Terrain, explicit reflectors and per-direction source specifications are read
  by `aconiq run` but not by browser (WASM) mode. Browser mode now builds road
  sources, barriers, buildings and Parkplätze from the same model the CLI reads,
  and `frontend/src/wasm/parking-vocabulary.test.ts` plus
  `surface-types.test.ts` pin the shared vocabularies against the Go source. They
  pin names, not numbers: a behavioural comparison needs the real kernel in the
  frontend CI job, which is tracked in `PLAN.md`.

## Not yet supported

- Section 9 measurement-based vehicle data (custom acoustics from measurements).

## Recently completed

- Multi-diffraction (Gummibandmethode) for barriers with C term (§3.5.5, Eq. 16):
  fully implemented. Significant diffraction edges are selected via upper convex
  hull (Gummibandmethode per Probst 2010). Path difference z = A + B + C - s
  (Eq. 16) and modified K_w (Eq. 17, C added to larger of A or B) are computed
  for multi-edge cases. Single-edge diffraction (C=0) is a special case.

## TEST-20 task coverage

The delta columns below are placeholders. They stay empty until the module is run
against the official TEST-20 task set; no values are carried over from the
CI-safe suite, which only pins Aconiq against itself and proves nothing about
agreement with the reference results.

### Emission tasks

| Task | Description          | Status                                 | Max delta |
| ---- | -------------------- | -------------------------------------- | --------- |
| E1   | Base value           | (to be filled from conformance report) |           |
| E2   | Surface correction   |                                        |           |
| E3   | Gradient correction  |                                        |           |
| E4   | Junction correction  |                                        |           |
| E5   | Reflection surcharge |                                        |           |
| E6   | Vehicle sound power  |                                        |           |
| E7   | Length-related power |                                        |           |

### Immission tasks

| Task | Description                 | Reference      | Check | Max delta (ref) | Max delta (check) |
| ---- | --------------------------- | -------------- | ----- | --------------- | ----------------- |
| I1   | Free propagation            | (to be filled) |       |                 |                   |
| I2   | Single parallel barrier     |                |       |                 |                   |
| I3   | Parallel reflecting surface |                |       |                 |                   |
| I4   | Barrier + reflector         |                |       |                 |                   |
| I5   | Two parallel barriers       |                |       |                 |                   |
| I6   | Road in cutting             |                |       |                 |                   |
| I7   | Road elevated               |                |       |                 |                   |
| I8   | Ascending road              |                |       |                 |                   |
| I9   | Receding road               |                |       |                 |                   |

### Complex tasks

| Task | Description              | Reference      | Check | Max delta (ref) | Max delta (check) |
| ---- | ------------------------ | -------------- | ----- | --------------- | ----------------- |
| K1   | Intersection             | (to be filled) |       |                 |                   |
| K2   | Parallel building fronts |                |       |                 |                   |
| K3   | Perpendicular buildings  |                |       |                 |                   |
| K4   | Courtyard                |                |       |                 |                   |

### Parkplatz tasks (§3.4)

| Task | Description                                         | Pins                                       |
| ---- | --------------------------------------------------- | ------------------------------------------ |
| P1   | P+R lot, 200 Stellplätze, Tabelle 7 rates, Pkw      | Eq. 10 and the Tabelle 7 standard values   |
| P2   | The same lot at Tank-/Rastanlage rates, Lkw/Omnibus | Tabelle 6 and the movement-rate dependence |
| P3   | The P2 lot behind an 8 m barrier                    | D_z on the Parkplatz path (Eq. 11)         |
| P4   | The P2 lot in front of a wall 25 m behind it        | D_RV on the Parkplatz path (Eq. 3)         |

These are repo-authored derived fixtures like the rest of the CI-safe suite, so
they pin Aconiq against itself rather than against a reference result. Two
relations between them do not: `P2 − P1 = 10·lg(1,5/0,3) + 10 = 16,9897 dB`
follows from Eq. 10 alone, and `P3 < P2` must hold because a barrier stands
between the lot and the receiver. Both are asserted directly, so a regression in
either cannot be hidden by regenerating the snapshots.

## Tolerances

Per-task tolerance values are defined in the CI-safe suite manifest
(`ci_safe_suite.json`) and recorded in the conformance report artifact.

For official TEST-20 conformance, tolerances will be aligned with BASt
requirements as stated in the TEST-20 tasks document.

## Known deviations

(To be documented if any task consistently exceeds tolerance.)

## Evidence

| Evidence                | Path                                             |
| ----------------------- | ------------------------------------------------ |
| CI-safe suite report    | (generated at test time)                         |
| Local-suite report      | (optional, generated when TEST-20 is available)  |
| Conformance report JSON | `.noise/runs/<run-id>/rls19-test20-ci-safe.json` |
