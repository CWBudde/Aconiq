# RLS-19 Konformitaetserklaerung — Aconiq

Status: DRAFT — not yet submitted

## Software

| Field      | Value                             |
| ---------- | --------------------------------- |
| Name       | Aconiq                            |
| Module     | rls19-road                        |
| Version    | (to be filled at release)         |
| License    | MIT                               |
| Repository | https://github.com/aconiq/backend |

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

| Step | Description                                                      | Status      |
| ---- | ---------------------------------------------------------------- | ----------- |
| P1   | Flaechenbezogener Schallleistungspegel (Eq. 10, corrected form)  | Implemented |
| P2   | Fahrzeugtypzuschlag D_P,PT (Tabelle 6: Pkw/Motorrad/Lkw-Omnibus) | Implemented |
| P3   | Standardbewegungsraten N (Tabelle 7: P+R, Tank-/Rastanlagen)     | Implemented |
| P4   | Propagation from parking centroid (point source, §3.5 chain)     | Implemented |

### Propagation

| Feature                                                | Status      |
| ------------------------------------------------------ | ----------- |
| Teilstueckverfahren (segment method)                   | Implemented |
| Configurable segment length (reference/check settings) | Implemented |
| Single barrier shielding                               | Implemented |
| Double barrier shielding                               | Implemented |
| Terrain: cutting (Tieflage)                            | Implemented |
| Terrain: elevated (Hochlage)                           | Implemented |
| Terrain: ascending road                                | Implemented |
| Terrain: receding road                                 | Implemented |
| Single explicit reflector                              | Implemented |
| Up to 2 reflectors                                     | Implemented |

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

## Not yet supported

- Multi-diffraction (Gummibandmethode) for barriers with C term (§3.5.5, Eq. 16):
  single-edge diffraction is implemented; multi-diffraction with C>0 is not.
- Section 9 measurement-based vehicle data (custom acoustics from measurements).

## TEST-20 task coverage

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
