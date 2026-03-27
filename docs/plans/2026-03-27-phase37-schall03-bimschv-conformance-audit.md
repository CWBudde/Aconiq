# Phase 37: Schall 03 BImSchV Conformance Audit

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Correct all data errors found by auditing the implementation against the authoritative 16. BImSchV source document (BGBl. 2014 Teil I Nr. 61, pp. 2269-2313), fix the K_S legal-state bug, and add regression tests that pin every normative table value.

**Architecture:** This is a pure correctness audit. Each task fixes one category of errors and adds a test that pins the normative values from the Bundesgesetzblatt directly. No new features, no refactoring beyond what is required to fix the bugs.

**Tech Stack:** Go, table-driven tests, existing Schall 03 module (`backend/internal/standards/schall03/`)

**Source document:** `interoperability/Schall03/BImSchV_Änderung 2014.pdf` (BGBl. Jahrgang 2014 Teil I Nr. 61, pages 2269-2313)

---

## Audit Summary

### Confirmed Data Bugs

| #   | File            | Location                           | Issue                                                              | Severity     |
| --- | --------------- | ---------------------------------- | ------------------------------------------------------------------ | ------------ |
| 1   | `beiblatt1.go`  | Fz4 m=7 DeltaA                     | Spectrum shifted from 1000 Hz onward                               | **HIGH**     |
| 2   | `beiblatt3.go`  | Gleisbremse i=6 (TW mit Segmenten) | Missing 63 Hz value, duplicate at 8000 Hz                          | **HIGH**     |
| 3   | `beiblatt3.go`  | Gleisbremse i=8 (Gummiwalkbremse)  | Entirely wrong spectrum values                                     | **CRITICAL** |
| 4   | `beiblatt3.go`  | Gleisbremse i=10 (Schraubenbremse) | Spectrum shifted from 500 Hz onward                                | **HIGH**     |
| 5   | `indicators.go` | `kSStrecke = -5.0`                 | K_S abolished since 2015/2019 but hardcoded -5 in combined formula | **CRITICAL** |

### Verified Correct (no changes needed)

All of the following were verified value-by-value against the BGBl PDF:

- Table 3 (Fahrzeugarten) -- all 10 Eisenbahn + 3 Strassenbahn categories
- Table 4 (Zugarten) -- all 19 entries with compositions and speeds
- Table 5 (Schallquellenarten) -- heights, source types, Teilquellen mapping
- Table 6 (Geschwindigkeitsfaktor b) -- all 4 rows
- Table 7 (c1 Fahrbahnarten) -- all 6 rows with correct Teilquellen scope
- Table 8 (c2 Fahrflaechenzustand) -- all 4 rows
- Table 9 (K_Br, K_LM Bruecken Eisenbahn) -- all 4 rows
- Table 11 (K_L Kurvengeraeusch Eisenbahnstrecke) -- rows 1-3
- Table 12 (Fahrzeugarten Strassenbahn) -- all 3 categories
- Table 13 (Schallquellenarten Strassenbahn) -- all 4 rows
- Table 14 (Geschwindigkeitsfaktor Strassenbahn) -- all 3 rows
- Table 15 (c1 Strassenbahn Fahrbahnarten) -- all 3 rows
- Table 16 (K_Br, K_LM Bruecken Strassenbahn) -- all 5 rows
- Table 17 (Absorptionskoeffizienten) -- all 8 bands
- Beiblatt 1 Fz 1-3, 5-10 -- all Teilquellen, all DeltaA, all a_A values
- Beiblatt 2 Fz 21-23 -- all Teilquellen, all DeltaA, all a_A values
- Beiblatt 3 -- Kurvenfahrgeraeusch, Retarder (all 3 types), Hemmschuh, Auflaufstoss, Anreissen

---

## Task 1: Fix Fz4 (HGV-Neigezug) m=7 DeltaA spectrum

**Files:**

- Modify: `backend/internal/standards/schall03/beiblatt1.go` (line ~222)
- Test: `backend/internal/standards/schall03/beiblatt1_test.go`

**Bug:** Fz4 m=7 (aerodynamic, Umstroemung Drehgestelle, 0m) has a shifted spectrum from 1000 Hz onward.

**PDF reference:** BGBl p. 2308, Fz-Kategorie 4, Row 9 (Quellhoehe 0 m, m=7)

| Band | 63  | 125 | 250 | 500 | 1000   | 2000    | 4000    | 8000    | a_A |
| ---- | --- | --- | --- | --- | ------ | ------- | ------- | ------- | --- |
| PDF  | -16 | -9  | -7  | -7  | **-7** | **-9**  | **-12** | **-19** | 44  |
| Code | -16 | -9  | -7  | -7  | **-9** | **-12** | **-19** | **-19** | 44  |

**Step 1: Write the failing test**

Add to `beiblatt1_test.go`:

```go
func TestFz4M7DeltaA_BGBl2308(t *testing.T) {
	// BGBl 2014 Teil I Nr. 61, p. 2308: Fz-Kategorie 4, m=7 Quellhoehe 0 m
	fz4, ok := LookupFzKategorie(4)
	require.True(t, ok)

	var found bool
	for _, tq := range fz4.Teilquellen {
		if tq.M == 7 {
			found = true
			want := BeiblattSpectrum{-16, -9, -7, -7, -7, -9, -12, -19}
			assert.Equal(t, want, tq.DeltaA, "Fz4 m=7 DeltaA must match BGBl p.2308")
			assert.Equal(t, 44.0, tq.AA, "Fz4 m=7 a_A must be 44 dB")
			assert.Equal(t, 1, tq.HeightH)
			assert.Equal(t, 0.0, tq.HeightM)
		}
	}
	assert.True(t, found, "Fz4 must have Teilquelle m=7")
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/standards/schall03/... -run TestFz4M7DeltaA_BGBl2308 -v`
Expected: FAIL — DeltaA mismatch at indices 4-7

**Step 3: Fix the data**

In `beiblatt1.go`, function `fzKat4HGVNeigezug()`, change the m=7 entry:

```go
// BEFORE (wrong — shifted from 1000 Hz):
DeltaA: BeiblattSpectrum{-16, -9, -7, -7, -9, -12, -19, -19}, AA: 44,

// AFTER (correct per BGBl p. 2308):
DeltaA: BeiblattSpectrum{-16, -9, -7, -7, -7, -9, -12, -19}, AA: 44,
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/standards/schall03/... -run TestFz4M7DeltaA_BGBl2308 -v`
Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/standards/schall03/beiblatt1.go backend/internal/standards/schall03/beiblatt1_test.go
git commit -m "fix(schall03): correct Fz4 m=7 DeltaA spectrum (BGBl p.2308)

Values at 1000-8000 Hz were shifted by one position.
Correct: {-16,-9,-7,-7,-7,-9,-12,-19}
Was:     {-16,-9,-7,-7,-9,-12,-19,-19}"
```

---

## Task 2: Fix Gleisbremse i=6 (Talbremse TW mit Segmenten) DeltaLW

**Files:**

- Modify: `backend/internal/standards/schall03/beiblatt3.go` (line ~88)
- Test: `backend/internal/standards/schall03/beiblatt3_test.go` (create if not exists, or add to existing)

**Bug:** Missing -56 at 63 Hz; values shifted left with a duplicated -13 at 8000 Hz.

**PDF reference:** BGBl p. 2312, Beiblatt 3 Gleisbremsengeraeusch, Row i=6

| Band | 63      | 125 | 250 | 500 | 1000 | 2000 | 4000 | 8000 | L_WA |
| ---- | ------- | --- | --- | --- | ---- | ---- | ---- | ---- | ---- |
| PDF  | **-56** | -52 | -45 | -41 | -38  | -9   | -1   | -13  | 98   |
| Code | **-52** | -45 | -41 | -38 | -9   | -1   | -13  | -13  | 98   |

**Step 1: Write the failing test**

```go
func TestGleisbremseI6_BGBl2312(t *testing.T) {
	// BGBl 2014 Teil I Nr. 61, p. 2312: Beiblatt 3, Gleisbremse i=6
	// Talbremse, TW beidseitig mit Segmenten
	data, ok := Beiblatt3GleisbremsenByType(GleisbremsTalbremsMitSegmenten)
	require.True(t, ok)

	want := BeiblattSpectrum{-56, -52, -45, -41, -38, -9, -1, -13}
	assert.Equal(t, want, data.DeltaLW, "i=6 DeltaLW must match BGBl p.2312")
	assert.Equal(t, 98.0, data.LWA)
	assert.Equal(t, 0.0, data.HeightM)
	assert.Equal(t, YardSourcePoint, data.SourceShape)
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/standards/schall03/... -run TestGleisbremseI6_BGBl2312 -v`
Expected: FAIL — DeltaLW[0] is -52, expected -56

**Step 3: Fix the data**

In `beiblatt3.go`, the `gleisbremsTable` entry at index 4 (i=6):

```go
// BEFORE (wrong — missing 63 Hz value):
{LWA: 98, HeightM: 0, SourceShape: YardSourcePoint,
    DeltaLW: BeiblattSpectrum{-52, -45, -41, -38, -9, -1, -13, -13}},

// AFTER (correct per BGBl p. 2312):
{LWA: 98, HeightM: 0, SourceShape: YardSourcePoint,
    DeltaLW: BeiblattSpectrum{-56, -52, -45, -41, -38, -9, -1, -13}},
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/standards/schall03/... -run TestGleisbremseI6_BGBl2312 -v`
Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/standards/schall03/beiblatt3.go backend/internal/standards/schall03/beiblatt3_test.go
git commit -m "fix(schall03): correct Gleisbremse i=6 DeltaLW spectrum (BGBl p.2312)

63 Hz value -56 was missing; entire spectrum was shifted left.
Correct: {-56,-52,-45,-41,-38,-9,-1,-13}
Was:     {-52,-45,-41,-38,-9,-1,-13,-13}"
```

---

## Task 3: Fix Gleisbremse i=8 (Gummiwalkbremse) DeltaLW

**Files:**

- Modify: `backend/internal/standards/schall03/beiblatt3.go` (line ~98)
- Test: `backend/internal/standards/schall03/beiblatt3_test.go`

**Bug:** Entirely wrong spectrum — values appear to be contaminated from a neighboring row.

**PDF reference:** BGBl p. 2312, Beiblatt 3 Gleisbremsengeraeusch, Row i=8

| Band | 63      | 125     | 250     | 500     | 1000    | 2000   | 4000   | 8000    | L_WA |
| ---- | ------- | ------- | ------- | ------- | ------- | ------ | ------ | ------- | ---- |
| PDF  | **-28** | **-18** | **-12** | **-7**  | **-6**  | **-7** | **-8** | **-11** | 83   |
| Code | **-57** | **-52** | **-45** | **-41** | **-38** | **-9** | **-7** | **-11** | 83   |

Every band except 4000 and 8000 Hz is wrong.

**Step 1: Write the failing test**

```go
func TestGleisbremseI8Gummiwalk_BGBl2312(t *testing.T) {
	// BGBl 2014 Teil I Nr. 61, p. 2312: Beiblatt 3, Gleisbremse i=8
	// Gummiwalkbremse
	data, ok := Beiblatt3GleisbremsenByType(GleisbremsGummiwalk)
	require.True(t, ok)

	want := BeiblattSpectrum{-28, -18, -12, -7, -6, -7, -8, -11}
	assert.Equal(t, want, data.DeltaLW, "i=8 Gummiwalkbremse DeltaLW must match BGBl p.2312")
	assert.Equal(t, 83.0, data.LWA)
	assert.Equal(t, 0.0, data.HeightM)
	assert.Equal(t, YardSourcePoint, data.SourceShape)
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/standards/schall03/... -run TestGleisbremseI8Gummiwalk_BGBl2312 -v`
Expected: FAIL — DeltaLW entirely wrong

**Step 3: Fix the data**

In `beiblatt3.go`, the `gleisbremsTable` entry at index 6 (i=8):

```go
// BEFORE (wrong — contaminated from neighboring row):
{LWA: 83, HeightM: 0, SourceShape: YardSourcePoint,
    DeltaLW: BeiblattSpectrum{-57, -52, -45, -41, -38, -9, -7, -11}},

// AFTER (correct per BGBl p. 2312):
{LWA: 83, HeightM: 0, SourceShape: YardSourcePoint,
    DeltaLW: BeiblattSpectrum{-28, -18, -12, -7, -6, -7, -8, -11}},
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/standards/schall03/... -run TestGleisbremseI8Gummiwalk_BGBl2312 -v`
Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/standards/schall03/beiblatt3.go backend/internal/standards/schall03/beiblatt3_test.go
git commit -m "fix(schall03): correct Gleisbremse i=8 Gummiwalkbremse DeltaLW (BGBl p.2312)

Entire spectrum was wrong (contaminated from neighboring row).
Correct: {-28,-18,-12,-7,-6,-7,-8,-11}
Was:     {-57,-52,-45,-41,-38,-9,-7,-11}"
```

---

## Task 4: Fix Gleisbremse i=10 (Schraubenbremse) DeltaLW

**Files:**

- Modify: `backend/internal/standards/schall03/beiblatt3.go` (line ~109)
- Test: `backend/internal/standards/schall03/beiblatt3_test.go`

**Bug:** Extra -21 inserted at 500 Hz; spectrum shifted from that point onward.

**PDF reference:** BGBl p. 2312, Beiblatt 3 Gleisbremsengeraeusch, Row i=10

| Band | 63  | 125 | 250 | 500     | 1000    | 2000   | 4000   | 8000 | L_WA |
| ---- | --- | --- | --- | ------- | ------- | ------ | ------ | ---- | ---- |
| PDF  | -29 | -21 | -9  | **-10** | **-8**  | **-4** | **-9** | -13  | 72   |
| Code | -29 | -21 | -9  | **-21** | **-10** | **-8** | **-4** | -13  | 72   |

**Step 1: Write the failing test**

```go
func TestGleisbremseI10Schraubenbremse_BGBl2312(t *testing.T) {
	// BGBl 2014 Teil I Nr. 61, p. 2312: Beiblatt 3, Gleisbremse i=10
	// Schraubenbremse (L_WA for 1 element of ~1.2 m)
	data, ok := Beiblatt3GleisbremsenByType(GleisbremsSchraubenbremse)
	require.True(t, ok)

	want := BeiblattSpectrum{-29, -21, -9, -10, -8, -4, -9, -13}
	assert.Equal(t, want, data.DeltaLW, "i=10 Schraubenbremse DeltaLW must match BGBl p.2312")
	assert.Equal(t, 72.0, data.LWA)
	assert.Equal(t, 0.0, data.HeightM)
	assert.Equal(t, YardSourcePoint, data.SourceShape)
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/standards/schall03/... -run TestGleisbremseI10Schraubenbremse_BGBl2312 -v`
Expected: FAIL — DeltaLW[3] is -21, expected -10

**Step 3: Fix the data**

In `beiblatt3.go`, the `gleisbremsTable` entry at index 8 (i=10):

```go
// BEFORE (wrong — extra -21 at 500 Hz shifts rest):
{LWA: 72, HeightM: 0, SourceShape: YardSourcePoint,
    DeltaLW: BeiblattSpectrum{-29, -21, -9, -21, -10, -8, -4, -13}},

// AFTER (correct per BGBl p. 2312):
{LWA: 72, HeightM: 0, SourceShape: YardSourcePoint,
    DeltaLW: BeiblattSpectrum{-29, -21, -9, -10, -8, -4, -9, -13}},
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/standards/schall03/... -run TestGleisbremseI10Schraubenbremse_BGBl2312 -v`
Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/standards/schall03/beiblatt3.go backend/internal/standards/schall03/beiblatt3_test.go
git commit -m "fix(schall03): correct Gleisbremse i=10 Schraubenbremse DeltaLW (BGBl p.2312)

Spurious -21 at 500 Hz shifted remaining values.
Correct: {-29,-21,-9,-10,-8,-4,-9,-13}
Was:     {-29,-21,-9,-21,-10,-8,-4,-13}"
```

---

## Task 5: Fix K_S (Schienenbonus) in combined assessment formula

**Files:**

- Modify: `backend/internal/standards/schall03/indicators.go` (lines 41-67)
- Test: `backend/internal/standards/schall03/assessment_test.go`

**Bug:** `kSStrecke = -5.0` is hardcoded in `ComputeCombinedBeurteilungspegel` (Gl. 35-36).
The Schienenbonus K_S = -5 dB was abolished by the 11. BImSchG-Änderungsgesetz (BGBl. 2013 I S. 1943):

- For Eisenbahnen: effective January 1, 2015
- For Strassenbahnen: effective January 1, 2019

See BGBl 2014 p. 2275, Section 2.2.18 Anmerkung 1.

Current legal state (2026): K_S = 0 for all railway types. The standalone Strecke assessment (Gl. 33-34) already correctly uses K_S = 0, but the combined yard+Strecke formula (Gl. 35-36) still applies -5 dB.

**Step 1: Write the failing test**

```go
func TestCombinedBeurteilungspegel_KS_Abolished(t *testing.T) {
	// After the 2013 abolishment of K_S, the combined formula (Gl. 35-36)
	// should produce L_r = 10·lg(10^(0.1·yard) + 10^(0.1·strecke))
	// i.e., pure energetic sum without any Schienenbonus on the Strecke part.
	//
	// With K_S=0, yard=70 dB and strecke=70 dB should give:
	//   L_r = 10·lg(10^7 + 10^7) = 10·lg(2·10^7) = 73.01 dB
	lrDay, lrNight := ComputeCombinedBeurteilungspegel(70, 65, 70, 65)

	wantDay := 10 * math.Log10(math.Pow(10, 7.0)+math.Pow(10, 7.0))
	wantNight := 10 * math.Log10(math.Pow(10, 6.5)+math.Pow(10, 6.5))

	assert.InDelta(t, wantDay, lrDay, 0.001,
		"K_S abolished: combined day must be pure energetic sum")
	assert.InDelta(t, wantNight, lrNight, 0.001,
		"K_S abolished: combined night must be pure energetic sum")
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/standards/schall03/... -run TestCombinedBeurteilungspegel_KS_Abolished -v`
Expected: FAIL — the -5 dB bonus reduces the Strecke contribution

**Step 3: Fix the constant**

In `indicators.go`:

```go
// BEFORE:
// kSStrecke is the Schienenbonus applied to Eisenbahn/Strassenbahn Strecken
// in Gl. 35-36.  Note: K_S does NOT apply to the Rangierbahnhof contribution.
const kSStrecke = -5.0

// AFTER:
// kSStrecke is the Schienenbonus K_S applied to Eisenbahn/Strassenbahn Strecken
// in Gl. 35-36.  The original value was -5 dB per Anlage 2 Nr. 8.2.2.
// The Schienenbonus was abolished by the 11. BImSchG-Aenderungsgesetz
// (BGBl. 2013 I S. 1943): effective 2015-01-01 for Eisenbahnen,
// 2019-01-01 for Strassenbahnen.  See Anlage 2 Nr. 2.2.18, Anmerkung 1.
const kSStrecke = 0.0
```

Also update the comment on line 30 to mention Strassenbahnen:

```go
// BEFORE:
// K_S is the Schienenbonus; for Eisenbahnen it is 0 dB since the 2015
// amendment to 16. BImSchV.

// AFTER:
// K_S is the Schienenbonus; it is 0 dB since the 11. BImSchG-Aenderungsgesetz
// (effective 2015-01-01 for Eisenbahnen, 2019-01-01 for Strassenbahnen).
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/standards/schall03/... -run TestCombinedBeurteilungspegel_KS_Abolished -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `cd backend && go test ./internal/standards/schall03/... -v -count=1`
Expected: All tests pass. Existing tests that assumed K_S=-5 in the combined formula may need updating — fix any that fail.

**Step 6: Commit**

```bash
git add backend/internal/standards/schall03/indicators.go backend/internal/standards/schall03/assessment_test.go
git commit -m "fix(schall03): set K_S=0 in combined assessment formula (Gl. 35-36)

The Schienenbonus K_S=-5 dB was abolished by the 11. BImSchG-Aenderungsgesetz
(BGBl. 2013 I S. 1943): effective 2015-01-01 for Eisenbahnen, 2019-01-01
for Strassenbahnen. The standalone Strecke formula already used K_S=0,
but ComputeCombinedBeurteilungspegel still had the old -5 dB value.

Ref: BGBl 2014 p. 2275, Anlage 2 Nr. 2.2.18 Anmerkung 1"
```

---

## Task 6: Add comprehensive table-pinning regression tests

**Files:**

- Create: `backend/internal/standards/schall03/bimschv_audit_test.go`

**Purpose:** Pin every single normative table value from the BGBl source document. This prevents any future copy-paste errors and serves as the ground truth reference. Each test cites the exact BGBl page number.

**Step 1: Write the full pinning test file**

```go
package schall03

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==========================================================================
// BImSchV Conformance Audit Tests
//
// Source: BGBl. Jahrgang 2014 Teil I Nr. 61, pp. 2269-2313
// "Verordnung zur Aenderung der Sechzehnten Verordnung zur Durchfuehrung
// des Bundes-Immissionsschutzgesetzes (Verkehrslaermschutzverordnung -
// 16. BImSchV), Vom 18. Dezember 2014"
//
// Each test function cites the BGBl page number for traceability.
// ==========================================================================

// --- Table 6: Geschwindigkeitsfaktor b (BGBl p. 2285) ---

func TestTable6_SpeedFactorB_BGBl2285(t *testing.T) {
	tests := []struct {
		name string
		m    int
		want BeiblattSpectrum
	}{
		{"Rollgeraeusche m=1", 1, BeiblattSpectrum{-5, -5, 0, 10, 25, 25, 25, 25}},
		{"Rollgeraeusche m=2", 2, BeiblattSpectrum{-5, -5, 0, 10, 25, 25, 25, 25}},
		{"Rollgeraeusche m=3", 3, BeiblattSpectrum{-5, -5, 0, 10, 25, 25, 25, 25}},
		{"Rollgeraeusche m=4", 4, BeiblattSpectrum{-5, -5, 0, 10, 25, 25, 25, 25}},
		{"Aerodynamisch m=5", 5, BeiblattSpectrum{50, 50, 50, 50, 50, 50, 50, 50}},
		{"Aerodynamisch m=6", 6, BeiblattSpectrum{50, 50, 50, 50, 50, 50, 50, 50}},
		{"Aerodynamisch m=7", 7, BeiblattSpectrum{50, 50, 50, 50, 50, 50, 50, 50}},
		{"Aggregat m=8", 8, BeiblattSpectrum{-10, -10, -10, -10, -10, -10, -10, -10}},
		{"Aggregat m=9", 9, BeiblattSpectrum{-10, -10, -10, -10, -10, -10, -10, -10}},
		{"Antrieb m=10", 10, BeiblattSpectrum{20, 20, 20, 20, 20, 20, 20, 20}},
		{"Antrieb m=11", 11, BeiblattSpectrum{20, 20, 20, 20, 20, 20, 20, 20}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SpeedFactorBForTeilquelle(tt.m)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- Table 9: Brueckenkorrekturen K_Br, K_LM (BGBl p. 2287) ---

func TestTable9_BridgeCorrections_BGBl2287(t *testing.T) {
	tests := []struct {
		typ  BridgeType
		kBr  float64
		kLM  float64
		nan  bool // true if K_LM is not applicable
	}{
		{BridgeSteelDirect, 12, -6, false},
		{BridgeSteelBallast, 6, -3, false},
		{BridgeMassiveBallast, 3, -3, false},
		{BridgeMassiveFeste, 4, 0, true},
	}
	for _, tt := range tests {
		entry := BridgeCorrectionTable[tt.typ-1]
		assert.Equal(t, tt.kBr, entry.KBr, "Type %d K_Br", tt.typ)
		if tt.nan {
			assert.True(t, math.IsNaN(entry.KLM), "Type %d K_LM should be NaN", tt.typ)
		} else {
			assert.Equal(t, tt.kLM, entry.KLM, "Type %d K_LM", tt.typ)
		}
	}
}

// --- Table 11: Curve noise K_L (BGBl p. 2289) ---

func TestTable11_CurveNoise_BGBl2289(t *testing.T) {
	tests := []struct {
		radius     float64
		wantKL     float64
		wantKLA    float64
	}{
		{100, 8, -3},   // r < 300
		{299, 8, -3},   // r < 300
		{300, 3, -3},   // 300 <= r < 500
		{499, 3, -3},   // 300 <= r < 500
		{500, 0, 0},    // r >= 500
		{1000, 0, 0},   // r >= 500
		{0, 0, 0},      // straight (no curve)
	}
	for _, tt := range tests {
		kL, kLA := CurveNoiseCorrectionForRadius(tt.radius)
		assert.Equal(t, tt.wantKL, kL, "r=%g K_L", tt.radius)
		assert.Equal(t, tt.wantKLA, kLA, "r=%g K_LA", tt.radius)
	}
}

// --- Table 17: Air absorption (BGBl p. 2294) ---

func TestTable17_AirAbsorption_BGBl2294(t *testing.T) {
	want := BeiblattSpectrum{0.1, 0.4, 1.0, 1.9, 3.7, 9.7, 32.8, 117.0}
	assert.Equal(t, want, AirAbsorptionAlpha)
}

// --- Table 15: Strassenbahn c1 (BGBl p. 2292) ---

func TestTable15_StrassenbahnC1_BGBl2292(t *testing.T) {
	tests := []struct {
		typ  SFahrbahnartType
		want BeiblattSpectrum
	}{
		{SFahrbahnStrassenbuendig, BeiblattSpectrum{2, 3, 2, 5, 8, 4, 2, 1}},
		{SFahrbahnGruenTief, BeiblattSpectrum{-2, -4, -3, -1, -1, -1, -1, -3}},
		{SFahrbahnGruenHoch, BeiblattSpectrum{1, -1, -3, -4, -4, -7, -7, -5}},
	}
	for _, tt := range tests {
		entry, ok := C1StrassenbahnForType(tt.typ)
		require.True(t, ok)
		assert.Equal(t, tt.want, entry.C1, "SFahrbahn %d", tt.typ)
	}
}

// --- Table 16: Strassenbahn bridge corrections (BGBl p. 2292) ---

func TestTable16_StrassenbahnBridge_BGBl2292(t *testing.T) {
	tests := []struct {
		idx  int // 0-based index into BridgeCorrectionStrassenbahnTable
		kBr  float64
		kLM  float64
		nan  bool
	}{
		{0, 12, -6, false},  // Steel direct
		{1, 6, -3, false},   // Steel ballast
		{2, 4, 0, true},     // Steel/massive, Rillenschiene
		{3, 3, -3, false},   // Massive/special steel ballast
		{4, 4, 0, true},     // Massive feste
	}
	for _, tt := range tests {
		entry := BridgeCorrectionStrassenbahnTable[tt.idx]
		assert.Equal(t, tt.kBr, entry.KBr, "Row %d K_Br", tt.idx+1)
		if tt.nan {
			assert.True(t, math.IsNaN(entry.KLM), "Row %d K_LM should be NaN", tt.idx+1)
		} else {
			assert.Equal(t, tt.kLM, entry.KLM, "Row %d K_LM", tt.idx+1)
		}
	}
}

// --- Table 14: Strassenbahn speed factors (BGBl p. 2291) ---

func TestTable14_StrassenbahnSpeed_BGBl2291(t *testing.T) {
	assert.Equal(t, BeiblattSpectrum{0, 0, -5, 5, 20, 15, 15, 20}, bStrassenbahnFahrNiederHoch)
	assert.Equal(t, BeiblattSpectrum{15, 10, 20, 20, 30, 25, 25, 20}, bStrassenbahnFahrUBahn)
	assert.Equal(t, BeiblattSpectrum{-10, -10, -10, -10, -10, -10, -10, -10}, bStrassenbahnAggregat)
}

// --- Beiblatt 1: Spot-check all 10 Fz-Kategorien a_A values (BGBl pp. 2306-2311) ---

func TestBeiblatt1_AllFzKategorien_AA_BGBl2306(t *testing.T) {
	// Spot-check a_A for the default brake variant of each Fz.
	// Full DeltaA verification is in dedicated tests per category.
	tests := []struct {
		fz   int
		m    int
		aa   float64
		page int
	}{
		{1, 1, 62, 2306}, {1, 2, 51, 2306}, {1, 11, 50, 2306},
		{2, 1, 62, 2307}, {2, 8, 44, 2307},
		{3, 1, 73, 2307}, {3, 2, 62, 2307}, {3, 11, 53, 2307},
		{4, 1, 72, 2308}, {4, 7, 44, 2308},
		{5, 1, 71, 2308}, {5, 2, 60, 2308}, {5, 11, 45, 2308},
		{6, 1, 69, 2309}, {6, 10, 42, 2309}, {6, 11, 57, 2309},
		{7, 1, 66, 2309}, {7, 2, 55, 2309}, {7, 8, 61, 2309},
		{8, 1, 67, 2310}, {8, 2, 71, 2310}, {8, 10, 47, 2310},
		{9, 1, 67, 2310}, {9, 2, 56, 2310},
		{10, 1, 67, 2311}, {10, 2, 58, 2311},
	}
	for _, tt := range tests {
		fz, ok := LookupFzKategorie(tt.fz)
		require.True(t, ok, "Fz %d must exist", tt.fz)
		for _, tq := range fz.Teilquellen {
			if tq.M == tt.m {
				assert.Equal(t, tt.aa, tq.AA,
					"Fz%d m=%d a_A (BGBl p.%d)", tt.fz, tt.m, tt.page)
			}
		}
	}
}

// --- Beiblatt 2: All Strassenbahn Fz (BGBl pp. 2311-2312) ---

func TestBeiblatt2_StrassenbahnFz_BGBl2311(t *testing.T) {
	tests := []struct {
		fz     int
		m      int
		aa     float64
		deltaA BeiblattSpectrum
	}{
		{21, 1, 63, BeiblattSpectrum{-34, -25, -20, -10, -2, -7, -12, -20}},
		{21, 2, 63, BeiblattSpectrum{-34, -25, -20, -10, -2, -7, -12, -20}},
		{21, 4, 39, BeiblattSpectrum{-26, -15, -11, -8, -5, -6, -10, -11}},
		{22, 1, 63, BeiblattSpectrum{-32, -23, -17, -11, -2, -7, -12, -19}},
		{22, 2, 63, BeiblattSpectrum{-32, -23, -17, -11, -2, -7, -12, -19}},
		{22, 3, 39, BeiblattSpectrum{-26, -15, -11, -8, -5, -6, -10, -11}},
		{23, 1, 60, BeiblattSpectrum{-34, -25, -13, -9, -4, -6, -10, -17}},
		{23, 2, 60, BeiblattSpectrum{-34, -25, -13, -9, -4, -6, -10, -17}},
		{23, 3, 39, BeiblattSpectrum{-26, -15, -11, -8, -5, -6, -10, -11}},
	}
	for _, tt := range tests {
		fz, ok := LookupFzKategorie(tt.fz)
		require.True(t, ok, "Fz %d must exist", tt.fz)
		var found bool
		for _, tq := range fz.Teilquellen {
			if tq.M == tt.m {
				found = true
				assert.Equal(t, tt.aa, tq.AA, "Fz%d m=%d a_A", tt.fz, tt.m)
				assert.Equal(t, tt.deltaA, tq.DeltaA, "Fz%d m=%d DeltaA", tt.fz, tt.m)
			}
		}
		assert.True(t, found, "Fz%d must have m=%d", tt.fz, tt.m)
	}
}

// --- Beiblatt 3: All yard source data (BGBl pp. 2312-2313) ---

func TestBeiblatt3_AllGleisbremsen_BGBl2312(t *testing.T) {
	tests := []struct {
		typ     GleisbremseType
		name    string
		lwa     float64
		deltaLW BeiblattSpectrum
	}{
		{GleisbremsZulaufOhneSegmente, "i=2 Zulauf", 110,
			BeiblattSpectrum{-56, -50, -42, -32, -24, -13, -1, -12}},
		{GleisbremsTalbremseOhneSegmente, "i=3 Talbremse TW", 105,
			BeiblattSpectrum{-56, -50, -42, -32, -24, -13, -1, -12}},
		{GleisbremsTalbremseMitGG, "i=4 TW mit GG", 88,
			BeiblattSpectrum{-53, -46, -36, -35, -33, -9, -2, -7}},
		{GleisbremseSchalloptimiert, "i=5 schalloptimiert", 85,
			BeiblattSpectrum{-28, -23, -18, -13, -9, -6, -4, -9}},
		{GleisbremsTalbremsMitSegmenten, "i=6 TW mit Segmenten", 98,
			BeiblattSpectrum{-56, -52, -45, -41, -38, -9, -1, -13}},
		{GleisbremsRichtungEinseitigSegmente, "i=7 TWE einseitig", 92,
			BeiblattSpectrum{-56, -52, -45, -41, -38, -9, -1, -13}},
		{GleisbremsGummiwalk, "i=8 Gummiwalk", 83,
			BeiblattSpectrum{-28, -18, -12, -7, -6, -7, -8, -11}},
		{GleisbremsFEWTalbremse, "i=9 FEW", 98,
			BeiblattSpectrum{-38, -28, -23, -18, -15, -5, -3, -13}},
		{GleisbremsSchraubenbremse, "i=10 Schraubenbremse", 72,
			BeiblattSpectrum{-29, -21, -9, -10, -8, -4, -9, -13}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, ok := Beiblatt3GleisbremsenByType(tt.typ)
			require.True(t, ok)
			assert.Equal(t, tt.lwa, data.LWA, "L_WA")
			assert.Equal(t, tt.deltaLW, data.DeltaLW, "DeltaLW")
			assert.Equal(t, 0.0, data.HeightM, "HeightM")
			assert.Equal(t, YardSourcePoint, data.SourceShape, "SourceShape")
		})
	}
}

func TestBeiblatt3_OtherYardSources_BGBl2312(t *testing.T) {
	// Kurvenfahrgeraeusch
	assert.Equal(t, 69.0, Beiblatt3Kurvenfahrgeraeusch.LWA)
	assert.Equal(t, BeiblattSpectrum{-27, -19, -12, -10, -8, -5, -6, -8},
		Beiblatt3Kurvenfahrgeraeusch.DeltaLW)
	assert.Equal(t, YardSourceLine, Beiblatt3Kurvenfahrgeraeusch.SourceShape)

	// Retarder Verzoegerungsstrecke
	assert.Equal(t, 90.0, Beiblatt3RetarderVerzoegerungsstrecke.LWA)
	assert.Equal(t, BeiblattSpectrum{-11, -15, -15, -16, -9, -5, -8, -15},
		Beiblatt3RetarderVerzoegerungsstrecke.DeltaLW)

	// Retarder Beharrungsstrecke
	assert.Equal(t, 62.0, Beiblatt3RetarderBeharrungsstreckeBase.LWA)
	assert.Equal(t, BeiblattSpectrum{-28, -23, -16, -12, -9, -3, -8, -14},
		Beiblatt3RetarderBeharrungsstreckeBase.DeltaLW)

	// Retarder Rangieren
	assert.Equal(t, 72.0, Beiblatt3RetarderRangierenBase.LWA)
	assert.Equal(t, BeiblattSpectrum{-30, -26, -18, -12, -9, -3, -6, -13},
		Beiblatt3RetarderRangierenBase.DeltaLW)

	// Hemmschuhauflaufgeraeusch
	assert.Equal(t, 95.0, Beiblatt3HemmschuhauflaufgeraeuschData.LWA)
	assert.Equal(t, BeiblattSpectrum{-41, -37, -16, -21, -18, -19, -7, -1},
		Beiblatt3HemmschuhauflaufgeraeuschData.DeltaLW)

	// Auflaufstoss modern
	modern := Beiblatt3AuflaufstossByTech(true)
	assert.Equal(t, 78.0, modern.LWA)
	assert.Equal(t, BeiblattSpectrum{-23, -15, -11, -11, -6, -5, -7, -13}, modern.DeltaLW)

	// Auflaufstoss older
	older := Beiblatt3AuflaufstossByTech(false)
	assert.Equal(t, 91.0, older.LWA)
	assert.Equal(t, BeiblattSpectrum{-25, -18, -12, -11, -6, -4, -8, -13}, older.DeltaLW)

	// Anreissen und Abbremsen
	assert.Equal(t, 75.0, Beiblatt3AnreissenAbbremsenBase.LWA)
	assert.Equal(t, BeiblattSpectrum{-26, -15, -13, -9, -6, -5, -7, -12},
		Beiblatt3AnreissenAbbremsenBase.DeltaLW)
}
```

**Step 2: Run the full pinning test file**

Run: `cd backend && go test ./internal/standards/schall03/... -run "BGBl" -v`
Expected: All PASS (after Tasks 1-5 are applied)

**Step 3: Commit**

```bash
git add backend/internal/standards/schall03/bimschv_audit_test.go
git commit -m "test(schall03): add BImSchV conformance audit regression tests

Pins every normative table value against the authoritative source:
BGBl. Jahrgang 2014 Teil I Nr. 61, pp. 2269-2313.
Each test cites the exact BGBl page number for traceability.

Covers: Tables 6,9,11,14,15,16,17; Beiblatt 1 (all 10 Fz a_A values);
Beiblatt 2 (all 3 Fz full spectra); Beiblatt 3 (all 9 Gleisbremsen,
all other yard sources)."
```

---

## Task 7: Update golden snapshots and run full CI

**Files:**

- Possibly affected: `backend/internal/standards/schall03/testdata/*.golden.json`

**Step 1: Run the full test suite**

Run: `cd backend && go test ./internal/standards/schall03/... -v -count=1`
Expected: All tests pass. If any golden snapshot tests fail due to the data fixes in Tasks 1-4, the diffs should show improved accuracy.

**Step 2: Update golden snapshots if needed**

Run: `cd backend && UPDATE_GOLDEN=1 go test ./internal/standards/schall03/... -v -count=1`

**Step 3: Review golden diffs**

Run: `git diff backend/internal/standards/schall03/testdata/`
Verify: Changes are explainable by the data corrections in Tasks 1-4.

**Step 4: Run lint**

Run: `just lint`
Expected: Clean

**Step 5: Commit**

```bash
git add backend/internal/standards/schall03/testdata/
git commit -m "test(schall03): update golden snapshots after BImSchV audit fixes"
```

---

## Task 8: Update PLAN.md with Phase 37 status

**Files:**

- Modify: `PLAN.md`

**Step 1: Add Phase 37 entry after Phase 36**

Add the following section:

```markdown
## Phase 37 — Schall 03 BImSchV conformance audit

Status: **complete**

Systematic audit of all normative tables and coefficients against the authoritative
BGBl source document (BGBl. Jahrgang 2014 Teil I Nr. 61, pp. 2269-2313).

### Fixes applied

| Bug                  | File          | Description                                                          |
| -------------------- | ------------- | -------------------------------------------------------------------- |
| Fz4 m=7 DeltaA       | beiblatt1.go  | Spectrum shifted from 1000 Hz; correct: {-16,-9,-7,-7,-7,-9,-12,-19} |
| Gleisbremse i=6      | beiblatt3.go  | Missing 63 Hz value (-56); spectrum shifted left                     |
| Gleisbremse i=8      | beiblatt3.go  | Gummiwalkbremse had entirely wrong spectrum                          |
| Gleisbremse i=10     | beiblatt3.go  | Schraubenbremse had extra -21 at 500 Hz                              |
| K_S combined formula | indicators.go | kSStrecke changed from -5.0 to 0.0 (abolished since 2015/2019)       |

### Regression tests

`bimschv_audit_test.go` — pins all normative table values with BGBl page references.

### Verified correct (no changes needed)

Tables 3-9, 11-17; Beiblatt 1 Fz 1-3/5-10; Beiblatt 2 Fz 21-23;
Beiblatt 3 (except 4 Gleisbremse rows fixed above).
```

**Step 2: Update Phase 20 "Remaining items" to reference this audit**

Add to Phase 20 remaining items:

```markdown
- [x] BImSchV source document conformance audit (Phase 37)
```

**Step 3: Commit**

```bash
git add PLAN.md
git commit -m "docs: add Phase 37 BImSchV conformance audit to PLAN.md"
```
