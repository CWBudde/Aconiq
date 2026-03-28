# Phase 22 — TA Lärm Assessment Layer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement TA Lärm (Technische Anleitung zum Schutz gegen Lärm) as a regulatory assessment and reporting layer for industrial/commercial noise in Germany.

**Architecture:** TA Lärm is NOT a propagation standard — it is an assessment layer that consumes immission levels from propagation standards (ISO 9613-2, CNOSSOS-Industry) and applies threshold comparison, surcharges, load accounting, and compliance logic. The implementation follows the existing `bimschv16` assessment package pattern: a standalone package under `internal/assessment/talaerm/` with pure functions, no side effects, and comprehensive test coverage.

**Tech Stack:** Go, following existing `bimschv16` patterns. Package: `github.com/aconiq/backend/internal/assessment/talaerm`

**Normative basis:** TA Lärm vom 26.08.1998 (GMBl. S. 503), geändert durch Verwaltungsvorschrift vom 01.06.2017 (BAnz. AT 08.06.2017 B5). The 2017 amendment added category c) "urbane Gebiete". LAI-Hinweise (Stand 24.02.2023) serve as authoritative interpretation guidance.

**Reference documents (in `interoperability/TA-Laerm/`):**

- `TA-Laerm.pdf` — Original 1998 text (VSGA 03/2000)
- `TA-Laerm2.pdf` — Consolidated 2017 text with amendments (authoritative)
- `talaerm3.pdf` — juris version (same content)
- `lai-hinweise-auslegung-ta-laerm-stand-2023-02-24_1682411716.pdf` — LAI interpretation guidance

---

## Scope Boundary

**In scope (assessment layer):**

- Area categories and Immissionsrichtwerte (Nr. 6.1–6.3)
- Beurteilungszeiten (Nr. 6.4) and lauteste Nachtstunde
- Zuschläge: KT, KI, KR (Anhang + Nr. 6.5)
- Beurteilungspegel Lr computation (Anhang G2)
- Vorbelastung / Zusatzbelastung / Gesamtbelastung (Nr. 2.4, Anhang G1)
- Relevanzprüfung — irrelevance criterion (Nr. 3.2.1)
- Spitzenpegelkriterium — peak level limits (Nr. 6.1)
- Gemengelagen — intermediate values (Nr. 6.7)
- Messabschlag — measurement deduction (Nr. 6.9)
- Per-receiver structured assessment output
- German-language assessment text

**Out of scope (propagation / measurement):**

- Anhang A.2 propagation computation (ISO 9613-2 — separate standard module)
- Anhang A.3 measurement procedures
- Sections 3–5 procedural/administrative rules (not computable)
- Nr. 7.1 emergency exceptions (administrative decision)
- Nr. 7.3 low-frequency noise assessment (separate DIN 45680 workflow)
- Körperschallübertragung (Nr. A.1.1.4 — building vibration, separate domain)

---

## Phase 22.1 — Area Categories and Threshold Tables

**Goal:** Define the 7 TA Lärm area categories (a–g per Nr. 6.1, including 2017 "urbane Gebiete") and all three threshold tables (outdoor, indoor, rare events).

**Package:** `backend/internal/assessment/talaerm/`

**Files:**

- Create: `backend/internal/assessment/talaerm/categories.go`
- Create: `backend/internal/assessment/talaerm/categories_test.go`

**What to implement:**

1. `AreaCategory` string type with 7 constants:
   - `AreaIndustrial` = a) Industriegebiete — 70 dB(A) day only (no day/night split)
   - `AreaCommercial` = b) Gewerbegebiete — 65/50 dB(A)
   - `AreaUrban` = c) urbane Gebiete — 63/45 dB(A) _(2017 amendment)_
   - `AreaMixed` = d) Kern-, Dorf-, Mischgebiete — 60/45 dB(A)
   - `AreaResidential` = e) allgemeine Wohn-/Kleinsiedlungsgebiete — 55/40 dB(A)
   - `AreaPureResidential` = f) reine Wohngebiete — 50/35 dB(A)
   - `AreaHealthcare` = g) Kurgebiete, Krankenhäuser, Pflegeanstalten — 45/35 dB(A)

2. `Thresholds` struct: `Day int`, `Night int` (dB(A))

3. `ThresholdsOutdoor(category) (Thresholds, error)` — Nr. 6.1 table

4. `ThresholdsIndoor() Thresholds` — Nr. 6.2: fixed 35/25 dB(A) for all categories

5. `ThresholdsRareEvents() Thresholds` — Nr. 6.3: fixed 70/55 dB(A) for categories b–g

6. `PeakLimits` struct and `PeakLimitsOutdoor(category) PeakLimits`:
   - All categories: day +30 dB(A), night +20 dB(A) over threshold
   - Nr. 6.2 indoor: +10 dB(A)
   - Nr. 6.3 rare events for b: day +25, night +15; for c–g: day +20, night +10

7. `AreaCategoryLabelDE(category) string` — German name
8. `ParseAreaCategory(raw string) (AreaCategory, error)` — flexible parser
9. `AreaCategoryCode(category) string` — returns "a"–"g"
10. `IsErhoehtEmpfindlichkeit(category) bool` — true for d, e, f (Nr. 6.5 applicability, note: the 2017 version says "d bis f", see below)

**Important notes:**

- Category a) Industriegebiete has a SINGLE value of 70 dB(A) — no day/night distinction
- The 2017 amendment added c) urbane Gebiete between b) and old c)
- Nr. 6.5 Zuschlag applies to categories "d bis f" per the 2017 text (the LAI-Hinweise clarify that the reference to "Buchstaben d bis f" shifted due to the insertion of "urbane Gebiete", so the surcharge now applies to d=Kern/Dorf/Misch, e=allg.Wohn, f=reine Wohn — NOT to the new urbane Gebiete or g=Healthcare)

**Tests:**

- Verify all 7 categories return correct outdoor thresholds
- Verify indoor is always 35/25
- Verify rare events is always 70/55
- Verify peak limits per category
- Verify ParseAreaCategory handles German names, umlauts, variations
- Verify IsErhoehtEmpfindlichkeit returns correct categories

---

## Phase 22.2 — Time Periods and Teilzeiten

**Goal:** Model TA Lärm assessment time periods (Nr. 6.4) and the Teilzeit (partial time) concept needed for Beurteilungspegel computation.

**Files:**

- Create: `backend/internal/assessment/talaerm/periods.go`
- Create: `backend/internal/assessment/talaerm/periods_test.go`

**What to implement:**

1. `AssessmentPeriod` type: `Day`, `Night`

2. Day/Night definitions per Nr. 6.4:
   - Tag: 06:00–22:00 (Tr = 16h)
   - Nacht: 22:00–06:00 (Tr = 1h or 8h per Nr. 6.4)
   - Night assessment uses the "lauteste Nachtstunde" (loudest full night hour)

3. `Teilzeit` struct:

   ```
   DurationH   float64  // Tj in hours
   LAeq        float64  // Mittelungspegel during Tj
   KT          float64  // Tonhaltigkeit/Informationshaltigkeit surcharge (0, 3, or 6)
   KI          float64  // Impulshaltigkeit surcharge (0, 3, or 6)
   KR          float64  // Erhöhte Empfindlichkeit surcharge (0 or 6)
   ```

4. `NightAssessmentMode` type: `FullNight` (8h) or `LoudestHour` (1h)
   - Nr. 6.4: "Maßgebend für die Beurteilung der Nacht ist die volle Nachtstunde (z.B. 1.00 bis 2.00 Uhr) mit dem höchsten Beurteilungspegel"

5. `ErhoehtEmpfindlichkeitPeriods` — Nr. 6.5 time windows:
   - Werktage: 06:00–07:00, 20:00–22:00
   - Sonn-/Feiertage: 06:00–09:00, 13:00–15:00, 20:00–22:00
   - Surcharge: 6 dB
   - Only applicable in categories d–f (per 2017 text)

6. Helper: `IsErhoehtEmpfindlichkeitTime(hour int, isWeekday bool) bool`

**Tests:**

- Day period duration = 16h
- Night period duration = 8h (full) or 1h (loudest hour)
- Erhöhte Empfindlichkeit time checks for weekdays and weekends
- Edge cases: exactly 06:00, exactly 22:00

---

## Phase 22.3 — Beurteilungspegel Computation (Lr)

**Goal:** Implement the Beurteilungspegel (rating level) computation per Anhang equation G2, including the Teilzeiten-based energetic average with surcharges.

**Files:**

- Create: `backend/internal/assessment/talaerm/beurteilungspegel.go`
- Create: `backend/internal/assessment/talaerm/beurteilungspegel_test.go`

**What to implement:**

1. `ComputeLr(period AssessmentPeriod, teilzeiten []Teilzeit, cmet float64) (float64, error)` — Equation G2:

   ```
   Lr = 10·lg[ (1/Tr) · Σ Tj · 10^(0.1·(LAeq,j - Cmet + KT,j + KI,j + KR,j)) ]
   ```

   Where:
   - Tr = sum of all Tj = 16h (day) or 1h/8h (night) per Nr. 6.4
   - Tj = duration of partial time j in hours
   - LAeq,j = Mittelungspegel during Tj
   - Cmet = meteorological correction per DIN ISO 9613-2, Eq. (6)
   - KT,j = tonality/information content surcharge (0, 3, or 6 dB)
   - KI,j = impulsiveness surcharge (0, 3, or 6 dB)
   - KR,j = sensitive time surcharge (0 or 6 dB)

2. Validation:
   - Sum of Tj must equal Tr (16h day, 1h or 8h night)
   - All levels must be finite
   - Surcharges must be valid values (0, 3, or 6 for KT/KI; 0 or 6 for KR)

3. `ComputeImpulsSurcharge(lAFTeq, lAeq float64) float64` — G6:

   ```
   KI = LAFTeq - LAeq
   ```

   Result is 0, 3, or 6 dB per Nr. A.2.5.3 (rounded to nearest of these values based on Störwirkung)

4. Simple mode: `ComputeLrSimple(lAeq, cmet, kt, ki, kr float64) float64`
   — For single-Teilzeit case (entire assessment period = one emission state)

**Tests:**

- Single Teilzeit covering full day (16h): Lr should equal LAeq - Cmet + KT + KI + KR
- Two equal Teilzeiten with same levels: result equals single-Teilzeit result
- Teilzeit with 0 dB surcharges: Lr = LAeq - Cmet
- Hand-calculated multi-Teilzeit example
- Validation: Tj sum mismatch → error

---

## Phase 22.4 — Vorbelastung, Zusatzbelastung, Gesamtbelastung

**Goal:** Implement the three-tier load model (Nr. 2.4) and the Gesamtbelastung equation (Anhang G1), plus the Relevanzprüfung (irrelevance criterion from Nr. 3.2.1).

**Files:**

- Create: `backend/internal/assessment/talaerm/load.go`
- Create: `backend/internal/assessment/talaerm/load_test.go`

**What to implement:**

1. `LoadInput` struct:

   ```
   Vorbelastung   *float64  // LV — existing load (all other plants, optional)
   Zusatzbelastung float64  // LZ — additional load from assessed plant
   ```

2. `ComputeGesamtbelastung(lv, lz float64) float64` — Equation G1:

   ```
   LG = 10·lg(10^(0.1·LV) + 10^(0.1·LZ))
   ```

3. `Relevanzpruefung` — Nr. 3.2.1 irrelevance criterion:

   ```
   func IsIrrelevant(zusatzbelastung float64, richtwert int) bool
   ```

   Returns true if Zusatzbelastung is at least 6 dB(A) below the applicable Immissionsrichtwert:
   `Zusatzbelastung ≤ Richtwert - 6`

   When irrelevant, the Vorbelastung need not be determined (Nr. 3.2.1 Abs. 2).

4. `LoadAssessment` struct:

   ```
   Vorbelastung          *float64
   Zusatzbelastung       float64
   Gesamtbelastung       *float64  // nil if Vorbelastung not provided
   IrrelevanzkriteriumDB float64   // distance to threshold
   IsIrrelevant          bool
   ```

5. `AssessLoad(input LoadInput, richtwert int) LoadAssessment`

**Tests:**

- G1: LV=50, LZ=50 → LG≈53.01 dB
- G1: LV=60, LZ=50 → LG≈60.41 dB
- Irrelevance: LZ=49 with Richtwert=55 → irrelevant (49 ≤ 55-6)
- Irrelevance: LZ=50 with Richtwert=55 → NOT irrelevant (50 > 49)
- Vorbelastung nil: Gesamtbelastung not computed, only irrelevance check
- Edge case: both very low levels

---

## Phase 22.5 — Assessment Logic (Regelfallprüfung)

**Goal:** Implement the per-receiver assessment comparing Beurteilungspegel against Immissionsrichtwerte, including peak level check and Gemengelagen.

**Files:**

- Create: `backend/internal/assessment/talaerm/assessment.go`
- Create: `backend/internal/assessment/talaerm/assessment_test.go`

**What to implement:**

1. `ReceiverInput` struct:

   ```
   ReceiverID       string
   AreaCategory     AreaCategory
   Zusatzbelastung  PeriodLevels    // Lr day/night from assessed plant
   Vorbelastung     *PeriodLevels   // Lr day/night from all other plants (optional)
   PeakDay          *float64        // LAFmax day (optional)
   PeakNight        *float64        // LAFmax night (optional)
   Gemengelage      *Gemengelage    // mixed-area override (optional)
   IsMeasurementBased bool          // triggers Nr. 6.9 Messabschlag
   ```

2. `Gemengelage` struct — Nr. 6.7:

   ```
   EffectiveRichtwerteDay   int  // Zwischenwert, ≤ Kern/Dorf/Misch threshold
   EffectiveRichtwerteNight int
   ```

   Rule: intermediate value between adjacent area categories, must not exceed Kern/Dorf/Mischgebiet values (60/45).

3. `PeakAssessment` struct:

   ```
   PeakDay          *float64
   PeakNight        *float64
   PeakLimitDay     int
   PeakLimitNight   int
   DayExceeds       bool
   NightExceeds     bool
   ```

   Limits: outdoor threshold + 30 dB(A) day, + 20 dB(A) night

4. `ReceiverAssessment` struct:

   ```
   ReceiverID            string
   AreaCategory          AreaCategory
   AreaCategoryLabelDE   string
   AreaCategoryCode      string
   Richtwerte            Thresholds        // effective (may be Gemengelage override)
   Zusatzbelastung       LevelAssessment
   Vorbelastung          *LevelAssessment
   Gesamtbelastung       *LevelAssessment
   Irrelevant            IrrelevanzResult
   PeakAssessment        *PeakAssessment
   MeasurementDeduction  bool              // Nr. 6.9 applied
   Exceeds               bool              // final pass/fail
   SummaryDE             string
   ```

5. `AssessReceiver(input ReceiverInput) (ReceiverAssessment, error)`

6. `IrrelevanzResult` struct:

   ```
   DayIrrelevant   bool
   NightIrrelevant bool
   DayMarginDB     float64
   NightMarginDB   float64
   ```

7. Nr. 6.9 Messabschlag: when `IsMeasurementBased`, subtract 3 dB(A) from Beurteilungspegel before threshold comparison.

8. German summary text generation: `buildSummaryDE(result ReceiverAssessment) string`

**Tests:**

- Simple pass case: Lr well below Richtwert
- Simple fail case: Lr above Richtwert
- Irrelevance: Zusatzbelastung 6+ dB below → irrelevant, pass
- Gesamtbelastung exceeds but Zusatzbelastung is irrelevant → pass
- Peak level exceedance → fail even if Lr passes
- Gemengelage intermediate value
- Messabschlag applied: 3 dB deduction
- All 7 categories exercised

---

## Phase 22.6 — Export Envelope and Reporting

**Goal:** Create the structured JSON export format and integration with the existing report infrastructure.

**Files:**

- Create: `backend/internal/assessment/talaerm/export.go`
- Create: `backend/internal/assessment/talaerm/export_test.go`

**What to implement:**

1. `ExportEnvelope` struct (follows bimschv16 pattern):

   ```
   Regulation         string                // "TA Lärm"
   Edition            string                // "26.08.1998, geändert 01.06.2017"
   GeneratedAt        time.Time
   SourceStandardID   string                // e.g. "iso9613-2"
   ReceiverCount      int
   AssessedCount      int
   ExceedingCount     int
   IrrelevantCount    int
   CategoryCounts     map[string]int
   Results            []ReceiverAssessment
   Skipped            []SkippedReceiver
   ```

2. `SkippedReceiver` struct: `ReceiverID string`, `Reason string`

3. `BuildExportEnvelope(...)` function — assemble envelope from model + results

4. German report text block generation for gutachterliche Stellungnahme:
   - Header with regulation reference
   - Per-receiver assessment summary
   - Overall conclusion

**Tests:**

- Envelope with mixed pass/fail receivers
- Skipped receivers counted correctly
- Category counts accurate
- JSON round-trip

---

## Phase 22.7 — Verification and Conformance

**Goal:** Comprehensive test coverage, golden test scenarios, and conformance documentation.

**Files:**

- Create: `backend/internal/assessment/talaerm/golden_test.go`
- Create: `backend/internal/assessment/talaerm/testdata/` (golden files)
- Create: `docs/conformance/ta-laerm-konformitaetserklaerung.md`

**What to implement:**

1. Golden test scenarios:
   - **Scenario 1 — Simple industrial site**: single source, commercial area (b), day/night, no surcharges → pass
   - **Scenario 2 — Mixed area with surcharges**: residential (e), KT=3, KI=6, KR=6 → exceedance
   - **Scenario 3 — Irrelevance**: Zusatzbelastung 10 dB below Richtwert → irrelevant
   - **Scenario 4 — Gemengelage**: intermediate value between commercial and mixed
   - **Scenario 5 — Peak level**: Lr passes but peak exceeds → fail
   - **Scenario 6 — Multiple Teilzeiten**: 3 operating phases with different emissions
   - **Scenario 7 — Measurement-based**: Messabschlag applied, borderline case
   - **Scenario 8 — All categories**: one receiver per category, verify all thresholds

2. Conformance declaration document listing:
   - Implemented sections with normative references
   - Known limitations / out-of-scope items
   - Version/edition tracking
   - Mapping to underlying propagation standards

**Tests:**

- All golden scenarios with `UPDATE_GOLDEN` support
- Each assessment pathway exercised at least once

---

## Dependencies and Ordering

```
22.1 (categories/thresholds) ─── no deps
22.2 (time periods)          ─── no deps
22.3 (Lr computation)        ─── depends on 22.2 (Teilzeit type)
22.4 (load model)            ─── depends on 22.1 (Thresholds)
22.5 (assessment logic)      ─── depends on 22.1, 22.3, 22.4
22.6 (export/reporting)      ─── depends on 22.5
22.7 (verification)          ─── depends on 22.5, 22.6
```

Phases 22.1 and 22.2 can be implemented in parallel. Phase 22.3 requires 22.2. Phase 22.5 is the integration point. Phase 22.7 is the capstone.

---

## Key Normative Values (Quick Reference)

### Nr. 6.1 — Immissionsrichtwerte außerhalb von Gebäuden

| Code | Category                                 | Tag dB(A) | Nacht dB(A) |
| ---- | ---------------------------------------- | --------- | ----------- |
| a    | Industriegebiete                         | 70        | 70          |
| b    | Gewerbegebiete                           | 65        | 50          |
| c    | urbane Gebiete _(2017)_                  | 63        | 45          |
| d    | Kern-/Dorf-/Mischgebiete                 | 60        | 45          |
| e    | allg. Wohn-/Kleinsiedlungsgebiete        | 55        | 40          |
| f    | reine Wohngebiete                        | 50        | 35          |
| g    | Kurgebiete/Krankenhäuser/Pflegeanstalten | 45        | 35          |

### Nr. 6.1 — Kurzzeitige Geräuschspitzen (Peak)

- Tag: Richtwert + 30 dB(A)
- Nacht: Richtwert + 20 dB(A)

### Nr. 6.2 — Innerhalb von Gebäuden

- Tag: 35 dB(A), Nacht: 25 dB(A)
- Peak: +10 dB(A)

### Nr. 6.3 — Seltene Ereignisse (categories b–g)

- Tag: 70 dB(A), Nacht: 55 dB(A)
- Peak for b: Tag +25, Nacht +15
- Peak for c–g: Tag +20, Nacht +10

### Nr. 6.5 — Zuschlag erhöhte Empfindlichkeit (categories d–f)

- Werktage: 06–07, 20–22
- Sonn-/Feiertage: 06–09, 13–15, 20–22
- KR = 6 dB
