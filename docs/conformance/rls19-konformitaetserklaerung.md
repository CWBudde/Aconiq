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

| Field           | Value                                                               |
| --------------- | ------------------------------------------------------------------- |
| Standard        | RLS-19 (Richtlinien fuer den Laermschutz an Strassen, Ausgabe 2019) |
| Legal basis     | 16. BImSchV                                                         |
| TEST-20 version | 2.1 (July 2025)                                                     |
| FGSV catalogue  | 334/2                                                               |

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

### Indicators

| Indicator | Period      | Status      |
| --------- | ----------- | ----------- |
| LrDay     | 06:00-22:00 | Implemented |
| LrNight   | 22:00-06:00 | Implemented |

## Not yet supported

- (document any known gaps here)

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
