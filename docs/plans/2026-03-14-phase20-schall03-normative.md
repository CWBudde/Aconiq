# Phase 20 — Schall 03 Standards-Faithful Eisenbahn Strecke Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the preview Schall 03 module with a standards-faithful implementation of Eisenbahn Strecke emission (Fz 1-10, Beiblatt 1), full propagation chain (Gl. 8-16), barrier diffraction (Gl. 18-26), and normative Beurteilungspegel (Gl. 29-34) per Anlage 2 zu §4 der 16. BImSchV.

**Architecture:** The existing `backend/internal/standards/schall03/` module is rewritten in-place. The emission chain expands from single-spectrum-per-source to multi-Teilquelle (11 sub-sources at 3 heights) per Fahrzeugkategorie. The propagation chain replaces simplified attenuation with the full normative A = A_div + A_atm + A_gr + A_bar. The DataPack embeds Beiblatt 1 normative coefficients directly (Anlage 2 is amtliches Werk per §5 UrhG — no redistribution issues). A dedicated Schall 03 conformance runner provides CI-safe verification.

**Tech Stack:** Go, JSON fixtures, golden snapshot testing. No new dependencies.

**Scope boundary:** Eisenbahn Strecke only (Fz-Kategorien 1-10, Beiblatt 1). Straßenbahnen (Fz 21-23, Beiblatt 2) and Rangier-/Umschlagbahnhöfe (Table 10, Beiblatt 3) are deferred to future phases. Image-source reflections (Gl. 27-28) are deferred. K_S Schienenbonus defaults to 0 dB (abolished 2015 for Eisenbahnen).

**Source document:** `docs/bimsch16_anl2_neu-1.pdf` — Anlage 2 (zu §4) Berechnung des Beurteilungspegels für Schienenwege (Schall 03).

---

## Task 1: Encode Beiblatt 1 normative data (Fz-Kategorien 1-10)

**Files:**

- Create: `backend/internal/standards/schall03/beiblatt1.go`
- Create: `backend/internal/standards/schall03/beiblatt1_test.go`
- Create: `backend/internal/standards/schall03/tables.go`
- Create: `backend/internal/standards/schall03/tables_test.go`

**Step 1: Create the Teilquelle and FzKategorie types**

In `beiblatt1.go`, define the core data types for encoding Beiblatt 1:

```go
package schall03

// OctaveBands represents 8 octave-band values at center frequencies
// 63, 125, 250, 500, 1000, 2000, 4000, 8000 Hz.
const NumOctaveBands = 8

type OctaveBands [NumOctaveBands]float64

// Teilquelle represents a single sub-source within a Fahrzeugkategorie.
// Each Fz has up to 11 Teilquellen at different heights.
type Teilquelle struct {
    M          int         // Teilquelle number (1-11)
    SourceType string      // "rolling" | "aerodynamic" | "aggregate" | "drive"
    HeightH    int         // Höhenbereich: 1 (0m SO), 2 (4m SO), 3 (5m SO)
    HeightM    float64     // actual height above SO in meters
    DeltaA     OctaveBands // Δa_f octave-band difference in dB
    AA         float64     // a_A total A-weighted level in dB
}

// FzKategorie represents one of the 10 Eisenbahn vehicle categories
// from Beiblatt 1 of Anlage 2.
type FzKategorie struct {
    Fz          int           // 1-10
    Name        string        // e.g., "HGV-Triebkopf"
    NAchs0      int           // reference axle count
    Teilquellen []Teilquelle  // all sub-sources for this Fz
}

// ZugartEntry represents one row of Table 4 (Verkehrsdaten für Eisenbahnen).
type ZugartEntry struct {
    Name          string       // e.g., "ICE-1-Zug"
    MaxSpeedKPH   float64      // Höchstgeschwindigkeit im Regelverkehr
    Composition   []FzCount    // Fz-Kategorie composition
}

// FzCount is a (Fz-Kategorie, count) pair within a train composition.
type FzCount struct {
    Fz    int // Fz-Kategorie number (1-10)
    Count int // number of Fahrzeugeinheiten
}
```

**Step 2: Encode all 10 Fz-Kategorien from Beiblatt 1**

In `beiblatt1.go`, create `FzKategorien()` returning all 10 categories. Each category contains its Teilquellen with a_A and Δa_f values transcribed verbatim from the PDF. Example for Fz 1 (HGV-Triebkopf, n_Achs,0 = 4):

```go
func FzKategorien() []FzKategorie {
    return []FzKategorie{
        {
            Fz: 1, Name: "HGV-Triebkopf", NAchs0: 4,
            Teilquellen: []Teilquelle{
                {M: 1, SourceType: "rolling", HeightH: 1, HeightM: 0,
                    DeltaA: OctaveBands{-50, -40, -24, -8, -3, -6, -11, -30}, AA: 62},
                {M: 2, SourceType: "rolling", HeightH: 1, HeightM: 0,
                    DeltaA: OctaveBands{-50, -40, -25, -9, -4, -4, -11, -23}, AA: 51},
                {M: 5, SourceType: "aerodynamic", HeightH: 3, HeightM: 5,
                    DeltaA: OctaveBands{-30, -21, -13, -9, -6, -4, -9, -17}, AA: 43},
                {M: 6, SourceType: "aerodynamic", HeightH: 2, HeightM: 4,
                    DeltaA: OctaveBands{-28, -21, -12, -9, -6, -4, -9, -17}, AA: 46},
                {M: 7, SourceType: "aerodynamic", HeightH: 1, HeightM: 0,
                    DeltaA: OctaveBands{-15, -8, -6, -6, -8, -14, -21, -32}, AA: 35},
                {M: 8, SourceType: "aggregate", HeightH: 2, HeightM: 4,
                    DeltaA: OctaveBands{-35, -24, -10, -5, -5, -8, -15, -26}, AA: 62},
                {M: 9, SourceType: "aggregate", HeightH: 1, HeightM: 0,
                    DeltaA: OctaveBands{-30, -22, -5, -4, -7, -11, -17, -26}, AA: 54},
                {M: 11, SourceType: "drive", HeightH: 1, HeightM: 0,
                    DeltaA: OctaveBands{-32, -24, -5, -4, -8, -12, -18, -29}, AA: 50},
            },
        },
        // ... Fz 2-10 follow same pattern, transcribed from Beiblatt 1 PDF pages 37-43.
    }
}
```

Transcribe all remaining Fz-Kategorien (2-10) from the PDF. Pay careful attention to:

- Fz 5 (E-Triebzug/S-Bahn) has separate WSB and RSB Rollgeräusche variants
- Fz 7 (E-Lok) has GG-Bremse and WSB variants for Rollgeräusche
- Fz 9 (Reisezugwagen) has GG-Bremse and WSB variants
- Fz 10 (Güterwagen) has GG-Bremse, VK-Bremse, WSB, RSB, and RoLa variants plus Kesselwagen Teilquellen (m=3,4) at height 4m

**Step 3: Encode Table 4 (Zugarten)**

In `beiblatt1.go`, add:

```go
func Zugarten() []ZugartEntry {
    return []ZugartEntry{
        {Name: "ICE-1-Zug", MaxSpeedKPH: 250, Composition: []FzCount{{1, 2}, {2, 12}}},
        {Name: "ICE-2-Halbzug", MaxSpeedKPH: 250, Composition: []FzCount{{1, 1}, {2, 7}}},
        {Name: "ICE-2-Vollzug", MaxSpeedKPH: 250, Composition: []FzCount{{1, 2}, {2, 14}}},
        {Name: "ICE-3-Halbzug", MaxSpeedKPH: 300, Composition: []FzCount{{3, 1}}},
        {Name: "ICE-3-Vollzug", MaxSpeedKPH: 300, Composition: []FzCount{{3, 2}}},
        {Name: "ICE-T", MaxSpeedKPH: 230, Composition: []FzCount{{4, 1}}},
        {Name: "Thalys-PBKA-Halbzug", MaxSpeedKPH: 300, Composition: []FzCount{{1, 2}, {2, 5}}},
        {Name: "Thalys-PBKA-Vollzug", MaxSpeedKPH: 300, Composition: []FzCount{{1, 4}, {2, 10}}},
        {Name: "ETR-470-Cisalpino", MaxSpeedKPH: 200, Composition: []FzCount{{4, 1}}},
        {Name: "IC-Zug-E-Lok", MaxSpeedKPH: 200, Composition: []FzCount{{7, 1}, {9, 12}}},
        {Name: "IC-Zug-V-Lok", MaxSpeedKPH: 160, Composition: []FzCount{{8, 1}, {9, 12}}},
        {Name: "Nahverkehrszug-E-Lok", MaxSpeedKPH: 160, Composition: []FzCount{{7, 1}, {9, 5}}},
        {Name: "Nahverkehrszug-V-Lok", MaxSpeedKPH: 140, Composition: []FzCount{{8, 1}, {9, 5}}},
        {Name: "Nahverkehrszug-ET", MaxSpeedKPH: 140, Composition: []FzCount{{5, 1}}},
        {Name: "Nahverkehrszug-VT", MaxSpeedKPH: 120, Composition: []FzCount{{6, 1}}},
        {Name: "IC3", MaxSpeedKPH: 180, Composition: []FzCount{{6, 1}}},
        {Name: "S-Bahn", MaxSpeedKPH: 120, Composition: []FzCount{{5, 1}}},
        {Name: "Gueterzug-E-Lok", MaxSpeedKPH: 100, Composition: []FzCount{{7, 1}, {10, 24}}},
        {Name: "Gueterzug-V-Lok", MaxSpeedKPH: 100, Composition: []FzCount{{8, 1}, {10, 24}}},
    }
}
```

**Step 4: Encode correction tables**

In `tables.go`, encode Tables 6, 7, 8, 9, 11, 17:

```go
package schall03

// Table 6: Geschwindigkeitsfaktor b for Eisenbahnen.
// Indexed by source type, values per octave band.
var SpeedFactorB = map[string]OctaveBands{
    "rolling":      {-5, -5, 0, 10, 25, 25, 25, 25},    // Teilquellen 1,2,3,4
    "aerodynamic":  {50, 50, 50, 50, 50, 50, 50, 50},    // Teilquellen 5,6,7
    "aggregate":    {-10, -10, -10, -10, -10, -10, -10, -10}, // Teilquellen 8,9
    "drive":        {20, 20, 20, 20, 20, 20, 20, 20},    // Teilquellen 10,11
}

// Table 7: Pegelkorrekturen c1 for Fahrbahnarten.
// Rows: [Zeile][effect], each has OctaveBands and applicable Teilquellen.
type C1Correction struct {
    Label            string
    Effect           string      // "schiene" or "reflexion"
    Bands            OctaveBands
    ApplicableTQ     []int       // Teilquellen m this applies to
}

var FahrbahnCorrectionsC1 = map[string][]C1Correction{
    "feste_fahrbahn": {
        {Label: "Feste Fahrbahn", Effect: "schiene",
            Bands: OctaveBands{0, 0, 0, 7, 3, 0, 0, 0}, ApplicableTQ: []int{1, 2}},
        {Label: "Feste Fahrbahn", Effect: "reflexion",
            Bands: OctaveBands{1, 1, 1, 1, 1, 1, 1, 1}, ApplicableTQ: []int{1, 2, 7, 9, 11}},
    },
    "feste_fahrbahn_absorber": {
        {Label: "Feste Fahrbahn mit Absorber", Effect: "schiene",
            Bands: OctaveBands{0, 0, 0, 7, 3, 0, 0, 0}, ApplicableTQ: []int{1, 2}},
        {Label: "Feste Fahrbahn mit Absorber", Effect: "reflexion",
            Bands: OctaveBands{0, 0, 0, -2, -2, -3, 0, 0}, ApplicableTQ: []int{1, 2, 7, 9, 11}},
    },
    "bahnuebergang": {
        {Label: "Bahnübergang", Effect: "schiene",
            Bands: OctaveBands{0, 0, 0, 8, 4, 0, 0, 0}, ApplicableTQ: []int{1, 2}},
        {Label: "Bahnübergang", Effect: "reflexion",
            Bands: OctaveBands{1, 1, 1, 1, 1, 1, 1, 1}, ApplicableTQ: []int{1, 2, 7, 9, 11}},
    },
}

// Table 8: Pegelkorrekturen c2 for surface condition.
type C2Correction struct {
    Label        string
    Bands        OctaveBands
    ApplicableTQ []int
}

var SurfaceCorrectionsC2 = map[string][]C2Correction{
    "bueg": {
        {Label: "besonders überwachtes Gleis", Bands: OctaveBands{0, 0, 0, -4, -5, -5, -4, 0},
            ApplicableTQ: []int{1, 3}},
    },
    "schienenstegdaempfer": {
        {Label: "Schienenstegdämpfer h=1", Bands: OctaveBands{0, 0, 0, -2, -3, -3, 0, 0},
            ApplicableTQ: []int{1, 3}},
        {Label: "Schienenstegdämpfer h=2", Bands: OctaveBands{0, 0, 0, -1, -3, -2, 0, 0},
            ApplicableTQ: []int{2, 4}},
    },
    "schienenstegabschirmung": {
        {Label: "Schienenstegabschirmung", Bands: OctaveBands{0, 0, 0, -3, -4, -5, 0, 0},
            ApplicableTQ: []int{1}},
    },
}

// Table 9: Korrekturen K_Br and K_LM for bridges.
type BridgeCorrection struct {
    Label string
    KBr   float64
    KLM   float64 // NaN means not applicable
}

var BridgeCorrections = map[int]BridgeCorrection{
    1: {Label: "Stählerner Überbau, Gleise direkt aufgelagert", KBr: 12, KLM: -6},
    2: {Label: "Stählerner Überbau, Schwellengleis im Schotterbett", KBr: 6, KLM: -3},
    3: {Label: "Massive Fahrbahnplatte oder besonderer stählerner Überbau, Schwellengleis", KBr: 3, KLM: -3},
    4: {Label: "Massive Fahrbahnplatte, feste Fahrbahn", KBr: 4, KLM: 0},
}

// Table 11: Pegelkorrekturen K_L for Auffälligkeit von Geräuschen.
// Only curve noise corrections for Strecke scope.
type AuffaelligkeitKL struct {
    KL  float64
    KLA float64 // counter-correction for mitigation
}

var CurveKL = map[string]AuffaelligkeitKL{
    "r_lt_300":       {KL: 8, KLA: -3},
    "r_300_to_500":   {KL: 3, KLA: -3},
    "r_gte_500":      {KL: 0, KLA: 0},
}

// Table 17: Absorptionskoeffizienten der Luft für Oktavbänder.
// Values in dB per 1000 m at 10°C, 70% relative humidity (per ISO 9613-2).
var AirAbsorptionAlpha = OctaveBands{0.1, 0.4, 1.0, 1.9, 3.7, 9.7, 32.8, 117.0}
```

**Step 5: Write tests for Beiblatt 1 data integrity**

In `beiblatt1_test.go`:

```go
func TestFzKategorienCount(t *testing.T) {
    t.Parallel()
    fzs := FzKategorien()
    if len(fzs) != 10 {
        t.Fatalf("expected 10 Fz-Kategorien, got %d", len(fzs))
    }
    for i, fz := range fzs {
        if fz.Fz != i+1 {
            t.Errorf("fz[%d]: expected Fz=%d, got %d", i, i+1, fz.Fz)
        }
        if fz.NAchs0 <= 0 {
            t.Errorf("fz %d: NAchs0 must be positive", fz.Fz)
        }
        if len(fz.Teilquellen) == 0 {
            t.Errorf("fz %d: must have at least one Teilquelle", fz.Fz)
        }
    }
}

func TestFzKategorie1HGVTriebkopf(t *testing.T) {
    t.Parallel()
    fzs := FzKategorien()
    fz1 := fzs[0]
    if fz1.Name != "HGV-Triebkopf" {
        t.Fatalf("expected HGV-Triebkopf, got %q", fz1.Name)
    }
    if fz1.NAchs0 != 4 {
        t.Fatalf("expected n_Achs,0=4, got %d", fz1.NAchs0)
    }
    // Spot-check: Teilquelle m=1 (Schienenrauheit), a_A=62
    tq1 := findTeilquelle(fz1.Teilquellen, 1)
    if tq1 == nil {
        t.Fatal("Teilquelle m=1 not found")
    }
    if tq1.AA != 62 {
        t.Errorf("Fz1 m=1: expected a_A=62, got %v", tq1.AA)
    }
    if tq1.DeltaA[0] != -50 { // 63 Hz
        t.Errorf("Fz1 m=1 63Hz: expected Δa=-50, got %v", tq1.DeltaA[0])
    }
}

func TestZugartenCount(t *testing.T) {
    t.Parallel()
    za := Zugarten()
    if len(za) != 19 {
        t.Fatalf("expected 19 Zugarten, got %d", len(za))
    }
}

func TestZugartICE1Composition(t *testing.T) {
    t.Parallel()
    za := Zugarten()
    ice1 := za[0]
    if ice1.Name != "ICE-1-Zug" {
        t.Fatalf("expected ICE-1-Zug, got %q", ice1.Name)
    }
    if ice1.MaxSpeedKPH != 250 {
        t.Fatalf("expected 250 km/h, got %v", ice1.MaxSpeedKPH)
    }
    // 2x Fz1 + 12x Fz2
    if len(ice1.Composition) != 2 {
        t.Fatalf("expected 2 Fz entries, got %d", len(ice1.Composition))
    }
}
```

In `tables_test.go`, add spot-check tests for Tables 6, 7, 8, 9, 11, 17:

```go
func TestAirAbsorptionAlphaValues(t *testing.T) {
    t.Parallel()
    // Table 17: 63Hz=0.1, 1000Hz=3.7, 8000Hz=117
    if AirAbsorptionAlpha[0] != 0.1 {
        t.Errorf("63Hz: expected 0.1, got %v", AirAbsorptionAlpha[0])
    }
    if AirAbsorptionAlpha[4] != 3.7 {
        t.Errorf("1000Hz: expected 3.7, got %v", AirAbsorptionAlpha[4])
    }
    if AirAbsorptionAlpha[7] != 117.0 {
        t.Errorf("8000Hz: expected 117, got %v", AirAbsorptionAlpha[7])
    }
}

func TestSpeedFactorBRolling(t *testing.T) {
    t.Parallel()
    // Table 6 row 2: rolling noise b = [-5,-5,0,10,25,25,25,25]
    b := SpeedFactorB["rolling"]
    if b[0] != -5 || b[3] != 10 || b[4] != 25 {
        t.Errorf("rolling speed factors wrong: %v", b)
    }
}
```

**Step 6: Run tests**

```bash
cd backend && go test ./internal/standards/schall03/ -run "TestFzKategorien|TestZugart|TestAirAbsorption|TestSpeedFactor" -v -count=1
```

Expected: all PASS.

**Step 7: Commit**

```bash
git add backend/internal/standards/schall03/beiblatt1.go backend/internal/standards/schall03/beiblatt1_test.go backend/internal/standards/schall03/tables.go backend/internal/standards/schall03/tables_test.go
git commit -m "feat(schall03): encode Beiblatt 1 Fz-Kategorien 1-10, Zugarten Table 4, and normative correction tables"
```

---

## Task 2: Rewrite emission chain (Gl. 1-2)

**Files:**

- Modify: `backend/internal/standards/schall03/emission.go`
- Create: `backend/internal/standards/schall03/emission_test.go`

**Step 1: Write failing tests for normative emission**

Create `emission_test.go` with tests that verify Gl. 1 computation:

```go
func TestEmissionGl1SingleTeilquelle(t *testing.T) {
    t.Parallel()
    // Hand-calculated: Fz1, Teilquelle m=1 (Schienenrauheit)
    // a_A=62, Δa_f[4]=-3 (1000 Hz band)
    // n_Q = n_Achs = 4 (= n_Achs,0 = 4), so 10·lg(4/4) = 0
    // v=250 km/h, v₀=100 km/h, b[4]=25 (rolling, 1000 Hz)
    // speed term: 25 · lg(250/100) = 25 * 0.39794 = 9.949
    // No corrections (schwellengleis, no bridge, no curve)
    // L_W'A,f=1000,h=1,m=1,Fz=1 = 62 + (-3) + 0 + 9.949 + 0 + 0 = 68.949
    result := computeTeilquelleEmission(/* ... */)
    // Check 1000 Hz band
    assertApprox(t, result.Bands[4], 68.949, 0.01)
}

func TestEmissionGl2MultipleVehicles(t *testing.T) {
    t.Parallel()
    // Gl. 2: energetic sum of n_Fz vehicles of same type
    // 10 · lg(n_Fz · 10^(0.1 · L_W'A,f,h,m,Fz))
    // For n_Fz=2 identical Fz1 at same speed:
    // L = L_single + 10·lg(2) = L_single + 3.01 dB
}

func TestEmissionICE1ZugFullSpectrum(t *testing.T) {
    t.Parallel()
    // Full ICE-1-Zug (2x Fz1 + 12x Fz2) at 250 km/h
    // Verify all 8 octave bands produce plausible levels
    // and that the multi-Fz energetic summation works.
}
```

**Step 2: Rewrite emission.go**

Replace the preview emission function with Gl. 1-2 implementation:

```go
// EmissionResult contains the octave-band sound power levels per height level
// for a given traffic period.
type EmissionResult struct {
    // PerHeight maps height level h (1,2,3) to OctaveBands of L_W'A,f,h.
    PerHeight map[int]OctaveBands
}

// ComputeStreckeEmission computes the emission for an Eisenbahn Strecke segment
// per Gl. 1 and Gl. 2, summing over all Fahrzeugeinheiten and their Teilquellen.
func ComputeStreckeEmission(op TrainOperation, infra TrackInfrastructure) (day, night EmissionResult, err error) {
    // For each FzEntry in the composition:
    //   For each Teilquelle of that FzKategorie:
    //     Compute L_W'A,f,h,m,Fz per Gl. 1
    //     Apply corrections: c1, c2, K_Br, K_L per applicability rules
    //   Sum over m per height h and octave band f
    // Then for n_Fz vehicles: Gl. 2 energetic summation
    // Finally sum over all Fz types in composition
}
```

The key per-Teilquelle computation (Gl. 1):

```go
func computeTeilquelleEmission(tq Teilquelle, fz FzKategorie, nQ int, nQ0 int, speedKPH float64, infra TrackInfrastructure) OctaveBands {
    var result OctaveBands
    b := SpeedFactorB[tq.SourceType]
    v0 := 100.0

    for f := 0; f < NumOctaveBands; f++ {
        L := float64(tq.AA) + tq.DeltaA[f]
        L += 10.0 * math.Log10(float64(nQ)/float64(nQ0))
        L += b[f] * math.Log10(speedKPH/v0)
        L += sumC1Corrections(infra.Fahrbahn, tq.M, f)
        L += sumC2Corrections(infra.SurfaceCondition, tq.M, f)
        L += bridgeCorrection(infra.BridgeType, tq.M)
        L += auffaelligkeitCorrection(infra.CurveRadiusM, tq.M)
        result[f] = L
    }
    return result
}
```

**Step 3: Run tests**

```bash
cd backend && go test ./internal/standards/schall03/ -run "TestEmission" -v -count=1
```

Expected: all PASS.

**Step 4: Commit**

```bash
git add backend/internal/standards/schall03/emission.go backend/internal/standards/schall03/emission_test.go
git commit -m "feat(schall03): rewrite emission chain with normative Gl. 1-2, multi-Teilquelle per Fz-Kategorie"
```

---

## Task 3: Expand source model

**Files:**

- Modify: `backend/internal/standards/schall03/model.go`
- Create: `backend/internal/standards/schall03/model_v2_test.go`

**Step 1: Write failing tests for new source model**

```go
func TestTrainOperationFromZugart(t *testing.T) {
    t.Parallel()
    op, err := NewTrainOperationFromZugart("ICE-1-Zug", 12.0, 4.0)
    if err != nil {
        t.Fatal(err)
    }
    if op.SpeedKPH != 250 {
        t.Errorf("expected speed 250, got %v", op.SpeedKPH)
    }
    if len(op.FzComposition) != 2 {
        t.Errorf("expected 2 Fz entries, got %d", len(op.FzComposition))
    }
}

func TestTrainOperationCustomComposition(t *testing.T) {
    t.Parallel()
    op := TrainOperation{
        TrainType:     "custom",
        FzComposition: []FzCount{{Fz: 7, Count: 1}, {Fz: 10, Count: 30}},
        SpeedKPH:      80,
        TrainsPerHourDay:   5,
        TrainsPerHourNight: 2,
    }
    if err := op.Validate(); err != nil {
        t.Fatal(err)
    }
}

func TestSpeedDetermination(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name        string
        streckeMax  float64
        fahrzeugMax float64
        isStation   bool
        want        float64
    }{
        {"normal", 200, 250, false, 200},
        {"vehicle slower", 200, 160, false, 160},
        {"minimum 50", 30, 160, false, 50},
        {"station min 70", 120, 160, true, 70},
    }
    for _, tt := range tests {
        got := resolveEffectiveSpeed(tt.streckeMax, tt.fahrzeugMax, tt.isStation)
        if got != tt.want {
            t.Errorf("%s: expected %v, got %v", tt.name, tt.want, got)
        }
    }
}
```

**Step 2: Add new types and constructor functions to model.go**

Add `TrainOperation`, `TrackInfrastructure` (expanded), `NewTrainOperationFromZugart()`, `resolveEffectiveSpeed()`, and validation. Keep the old `RailSource` type with a `ToTrainOperation()` migration method for backward compatibility.

**Step 3: Run all tests (including existing)**

```bash
cd backend && go test ./internal/standards/schall03/ -v -count=1
```

Expected: all PASS (old tests still work via backward-compat mapping).

**Step 4: Commit**

```bash
git add backend/internal/standards/schall03/model.go backend/internal/standards/schall03/model_v2_test.go
git commit -m "feat(schall03): expand source model with TrainOperation, Zugarten lookup, and speed determination"
```

---

## Task 4: Rewrite propagation chain (Gl. 8-16)

**Files:**

- Modify: `backend/internal/standards/schall03/propagation.go`
- Create: `backend/internal/standards/schall03/propagation_test.go`

**Step 1: Write failing tests for each equation**

```go
func TestAdivGl11(t *testing.T) {
    t.Parallel()
    // A_div = 10·lg(4π·d²/d₀²) with d₀=1m
    // d=100m: A_div = 10·lg(4π·10000) = 10·lg(125663.7) = 50.99 dB
    got := adiv(100.0)
    assertApprox(t, got, 50.99, 0.01)
}

func TestAatmGl12(t *testing.T) {
    t.Parallel()
    // A_atm = α·d/1000
    // 1000 Hz band (α=3.7), d=500m: A_atm = 3.7·500/1000 = 1.85 dB
    got := aatm(AirAbsorptionAlpha[4], 500.0)
    assertApprox(t, got, 1.85, 0.001)
}

func TestAgrBGl14(t *testing.T) {
    t.Parallel()
    // A_gr,B = [4.8 - (2·h_m/d)·(17 + 300·d_p/d)] ≥ 0
    // h_m=5, d=200, d_p=200: A_gr,B = 4.8 - (10/200)·(17+300) = 4.8 - 15.85 → 0 (clamped)
    got := agrB(5.0, 200.0, 200.0)
    assertApprox(t, got, 0.0, 0.001)

    // h_m=1, d=50, d_p=50: A_gr,B = 4.8 - (2/50)·(17+300) = 4.8 - 12.68 → 0 (clamped)
    got2 := agrB(1.0, 50.0, 50.0)
    assertApprox(t, got2, 0.0, 0.001)

    // h_m=10, d=100, d_p=100: A_gr,B = 4.8 - (20/100)·(17+300) = 4.8 - 63.4 → 0 (clamped)
    got3 := agrB(10.0, 100.0, 100.0)
    assertApprox(t, got3, 0.0, 0.001)

    // Large h_m relative to d — should approach 4.8
    // h_m=0.1, d=1000, d_p=1000: A_gr,B = 4.8 - (0.2/1000)·(17+300) = 4.8 - 0.0634 = 4.737
    got4 := agrB(0.1, 1000.0, 1000.0)
    assertApprox(t, got4, 4.737, 0.01)
}

func TestDirectivityGl8(t *testing.T) {
    t.Parallel()
    // D_I = 10·lg(0.22 + 1.27·sin²(δ))
    // δ=90° (perpendicular): D_I = 10·lg(0.22+1.27) = 10·lg(1.49) = 1.73 dB
    got := directivityDI(math.Pi / 2)
    assertApprox(t, got, 1.73, 0.01)

    // δ=0° (along track): D_I = 10·lg(0.22) = -6.58 dB
    got2 := directivityDI(0)
    assertApprox(t, got2, -6.58, 0.01)
}

func TestSolidAngleGl9(t *testing.T) {
    t.Parallel()
    // D_Ω = 10·lg(1 + (d_p² + (h_g-h_r)²) / (d_p² + (h_g+h_r)²))
    // h_g=0 (SO), h_r=4, d_p=25:
    // D_Ω = 10·lg(1 + (625+16)/(625+16)) = 10·lg(2) = 3.01 dB
    got := solidAngleDOmega(25.0, 0.0, 4.0)
    assertApprox(t, got, 3.01, 0.01)
}
```

**Step 2: Implement each propagation function**

In `propagation.go`, replace the preview code with normative functions:

```go
// adiv computes geometric divergence per Gl. 11.
// A_div = 10·lg(4π·d²/d₀²) with d₀ = 1 m.
func adiv(d float64) float64 {
    return 10.0 * math.Log10(4.0*math.Pi*d*d)
}

// aatm computes air absorption per Gl. 12.
// A_atm = α·d/1000 for a single octave band.
func aatm(alpha float64, d float64) float64 {
    return alpha * d / 1000.0
}

// agrB computes ground absorption over land per Gl. 14.
// A_gr,B = [4.8 - (2·h_m/d)·(17 + 300·d_p/d)] ≥ 0 dB
func agrB(hm float64, d float64, dp float64) float64 {
    val := 4.8 - (2.0*hm/d)*(17.0+300.0*dp/d)
    return math.Max(val, 0.0)
}

// agrW computes ground effect correction for water bodies per Gl. 16.
// A_gr,W = -3·d_w/d_p
func agrW(dw float64, dp float64) float64 {
    if dp == 0 {
        return 0
    }
    return -3.0 * dw / dp
}

// directivityDI computes the directivity index per Gl. 8.
// D_I = 10·lg(0.22 + 1.27·sin²(δ)) where δ is angle to track axis.
func directivityDI(delta float64) float64 {
    sinD := math.Sin(delta)
    return 10.0 * math.Log10(0.22 + 1.27*sinD*sinD)
}

// solidAngleDOmega computes the solid angle correction per Gl. 9.
// D_Ω = 10·lg(1 + (d_p² + (h_g-h_r)²) / (d_p² + (h_g+h_r)²))
func solidAngleDOmega(dp float64, hg float64, hr float64) float64 {
    num := dp*dp + (hg-hr)*(hg-hr)
    den := dp*dp + (hg+hr)*(hg+hr)
    if den == 0 {
        return 0
    }
    return 10.0 * math.Log10(1.0 + num/den)
}
```

Then wire into the full propagation function that replaces `ComputeReceiverPeriodLevelsWithDataPack`:

```go
// ComputeReceiverImmission computes the immission at a receiver per Gl. 29.
// Sums over all octave bands, height levels, Teilstücke, and propagation paths.
func ComputeReceiverImmission(receiver geo.Point2D, receiverHeightM float64,
    sources []StreckeSource, cfg PropagationConfigV2) (float64, float64, error) {
    // For each source:
    //   Subdivide track into Teilstücke per Nr. 3.4
    //   For each Teilstück kS:
    //     Compute segment midpoint, d, d_p, δ angle
    //     For each height level h with emission:
    //       L_WA,f,h,kS = L_W'A,f,h + 10·lg(l_kS/l₀) per Gl. 6
    //       For each octave band f:
    //         D_I per Gl. 8
    //         D_Ω per Gl. 9
    //         A = A_div + A_atm + A_gr + A_bar per Gl. 10
    //         contribution = L_WA + D_I + D_Ω - A
    //   Energetic sum per Gl. 29
}
```

**Step 3: Run tests**

```bash
cd backend && go test ./internal/standards/schall03/ -run "TestAdiv|TestAatm|TestAgr|TestDirectivity|TestSolidAngle" -v -count=1
```

Expected: all PASS.

**Step 4: Commit**

```bash
git add backend/internal/standards/schall03/propagation.go backend/internal/standards/schall03/propagation_test.go
git commit -m "feat(schall03): rewrite propagation chain with normative Gl. 8-16 (A_div, A_atm, A_gr, D_I, D_Ω)"
```

---

## Task 5: Implement barrier diffraction (Gl. 18-26)

**Files:**

- Create: `backend/internal/standards/schall03/barrier.go`
- Create: `backend/internal/standards/schall03/barrier_test.go`

**Step 1: Write failing tests for barrier equations**

```go
func TestBarrierDzGl21(t *testing.T) {
    t.Parallel()
    // D_z = 10·lg(3 + C₂/λ · C₃ · z · K_met)
    // Single barrier (C₃=1), no met correction (K_met=1)
    // z=0.5m, f=1000Hz → λ=0.34m, C₂=40 (Strecke)
    // D_z = 10·lg(3 + 40/0.34 · 1 · 0.5 · 1) = 10·lg(3 + 58.82) = 10·lg(61.82) = 17.91
    got := barrierDz(40, 0.34, 1.0, 0.5, 1.0)
    assertApprox(t, got, 17.91, 0.1)
}

func TestBarrierPathDifferenceZ(t *testing.T) {
    t.Parallel()
    // Gl. 25 for parallel edges:
    // z = sqrt((d_s + d_r + e)² + d_∥²) - d
    // Test with known geometry
}

func TestKmetGl23(t *testing.T) {
    t.Parallel()
    // K_met = exp(-1/2000 · sqrt(d_s·d_r·d / (2·z))) for z > 0
    // K_met = 1 for z ≤ 0
}

func TestDreflGl20(t *testing.T) {
    t.Parallel()
    // D_refl = (3 - h_abs/1m) ≥ 0 dB
    // h_abs=1m: D_refl = 2
    // h_abs=4m: D_refl = 0 (clamped)
    assertApprox(t, drefl(1.0), 2.0, 0.001)
    assertApprox(t, drefl(4.0), 0.0, 0.001)
}

func TestAbarSingleBarrier(t *testing.T) {
    t.Parallel()
    // Gl. 18: A_bar = D_z ≥ 0 for lateral diffraction
    // Gl. 19: A_bar = D_z - D_refl - A_gr ≥ 0 for top diffraction
}

func TestDzCappedAt20(t *testing.T) {
    t.Parallel()
    // Single barrier: D_z capped at 20 dB
    // Double barrier: D_z capped at 25 dB
}
```

**Step 2: Implement barrier functions**

In `barrier.go`:

```go
// OctaveBandFrequencies are the center frequencies for the 8 octave bands.
var OctaveBandFrequencies = [NumOctaveBands]float64{63, 125, 250, 500, 1000, 2000, 4000, 8000}

const speedOfSound = 340.0 // m/s

// wavelength returns the wavelength for an octave-band center frequency.
func wavelength(fm float64) float64 {
    return speedOfSound / fm
}

// barrierDz computes the screening attenuation per Gl. 21.
func barrierDz(c2 float64, lambda float64, c3 float64, z float64, kmet float64) float64 {
    if z <= 0 {
        return 0
    }
    dz := 10.0 * math.Log10(3.0+c2/lambda*c3*z*kmet)
    return dz
}

// kmet computes the meteorological correction factor per Gl. 23-24.
func kmet(ds float64, dr float64, d float64, z float64) float64 {
    if z <= 0 {
        return 1.0
    }
    return math.Exp(-1.0 / 2000.0 * math.Sqrt(ds*dr*d/(2.0*z)))
}

// pathDifferenceParallel computes z per Gl. 25 for parallel barrier edges.
func pathDifferenceParallel(ds float64, dr float64, e float64, dPar float64, d float64) float64 {
    total := ds + dr + e
    return math.Sqrt(total*total+dPar*dPar) - d
}

// pathDifferenceNonParallel computes z per Gl. 26 for non-parallel edges.
func pathDifferenceNonParallel(ds float64, dr float64, e float64, d float64) float64 {
    total := ds + dr + e
    return math.Sqrt(total*total) - d
}

// c3Multiple computes the additional screening factor for multiple diffraction per Gl. 22.
func c3Multiple(lambda float64, e float64) float64 {
    ratio := 5.0 * lambda / e
    return (1.0 + ratio*ratio) / (1.0/3.0 + ratio*ratio)
}

// drefl computes the correction for reflective barriers with absorbing base per Gl. 20.
func drefl(habs float64) float64 {
    return math.Max(3.0-habs, 0.0)
}

// ComputeAbar computes the barrier attenuation A_bar for a single propagation path
// per Gl. 18-19, frequency-dependent.
func ComputeAbar(geom BarrierGeometry, agrValue float64) OctaveBands {
    // Implementation handles both lateral (Gl. 18) and top (Gl. 19) diffraction
    // Selects edges via rubber-band method
    // Applies C₂=40 for Strecke, C₃, K_met, D_refl
    // Caps at 20 dB (single) or 25 dB (double)
}
```

**Step 3: Run tests**

```bash
cd backend && go test ./internal/standards/schall03/ -run "TestBarrier|TestKmet|TestDrefl|TestAbar|TestDzCapped" -v -count=1
```

Expected: all PASS.

**Step 4: Commit**

```bash
git add backend/internal/standards/schall03/barrier.go backend/internal/standards/schall03/barrier_test.go
git commit -m "feat(schall03): implement barrier diffraction Gl. 18-26 (D_z, C2/C3, K_met, D_refl)"
```

---

## Task 6: Implement assessment and indicators (Gl. 29-34)

**Files:**

- Modify: `backend/internal/standards/schall03/indicators.go`
- Modify: `backend/internal/standards/schall03/compute.go`
- Create: `backend/internal/standards/schall03/assessment_test.go`

**Step 1: Write failing tests**

```go
func TestBeurteilungspegelGl33(t *testing.T) {
    t.Parallel()
    // L_r,Tag = L_p,Aeq,Tag + K_S
    // K_S = 0 dB (abolished for Eisenbahnen since 2015)
    lpAeqTag := 65.3
    lr := beurteilungspegel(lpAeqTag, 0.0)
    if lr != 65.3 {
        t.Errorf("expected 65.3, got %v", lr)
    }
}

func TestBeurteilungspegelRounding(t *testing.T) {
    t.Parallel()
    // Rounding to whole dB for comparison with Immissionsgrenzwerte
    assertApprox(t, roundToWholeDB(65.3), 65.0, 0.001)
    assertApprox(t, roundToWholeDB(65.5), 66.0, 0.001)
    assertApprox(t, roundToWholeDB(65.8), 66.0, 0.001)
}

func TestSchienenbonus(t *testing.T) {
    t.Parallel()
    // Default: K_S = 0 (abolished)
    // Historical: K_S = -5
    lpAeq := 70.0
    lrDefault := beurteilungspegel(lpAeq, 0.0)
    lrHistorical := beurteilungspegel(lpAeq, -5.0)
    assertApprox(t, lrDefault, 70.0, 0.001)
    assertApprox(t, lrHistorical, 65.0, 0.001)
}
```

**Step 2: Update indicators.go and compute.go**

Update `indicators.go` with:

- New `BuiltinModelVersion = "phase20-normative-eisenbahn-strecke-v1"`
- Additional indicators: `LpAeqTag`, `LpAeqNacht` (unrounded)
- `beurteilungspegel()` and `roundToWholeDB()` functions

Update `compute.go` to wire the new emission → propagation → assessment pipeline, replacing the preview compute chain. Maintain the `ComputeReceiverOutputs` function signature for CLI compatibility.

Update the `Descriptor()` in `model.go` with:

- New version: `"phase20-eisenbahn-strecke-v1"`
- Updated profile with new parameters (Zugarten support, infrastructure options)
- Updated indicators list

**Step 3: Run all tests**

```bash
cd backend && go test ./internal/standards/schall03/ -v -count=1
```

Expected: all PASS (including updated golden tests).

**Step 4: Update golden snapshots**

```bash
cd backend && UPDATE_GOLDEN=1 go test ./internal/standards/schall03/ -run TestGoldenScenario -count=1
```

Review the golden diff — values will change due to normative coefficients replacing preview ones.

**Step 5: Commit**

```bash
git add backend/internal/standards/schall03/
git commit -m "feat(schall03): implement Beurteilungspegel Gl. 29-34, K_S handling, update descriptor to phase20"
```

---

## Task 7: Conformance runner and CI-safe test suite

**Files:**

- Create: `backend/internal/qa/acceptance/schall03/runner.go`
- Create: `backend/internal/qa/acceptance/schall03/runner_test.go`
- Create: `backend/internal/qa/acceptance/schall03/testdata/ci_safe_suite.json`
- Create: `backend/internal/qa/acceptance/schall03/testdata/ci_safe/` (scenario + golden pairs)

**Step 1: Create runner following the RLS-19 pattern**

Model the runner on `backend/internal/qa/acceptance/rls19_test20/runner.go` (Lines 25-235). The structure is:

```go
type Options struct {
    Mode      string // "ci-safe"
    OutputDir string
}

type Report struct {
    SuiteName        string                    `json:"suite_name"`
    StandardID       string                    `json:"standard_id"`
    Mode             string                    `json:"mode"`
    Status           string                    `json:"status"`
    SuiteVersion     string                    `json:"suite_version"`
    EvidenceClass    string                    `json:"evidence_class"`
    Provenance       string                    `json:"provenance"`
    GeneratedAt      time.Time                 `json:"generated_at"`
    TaskCount        int                       `json:"task_count"`
    PassedCount      int                       `json:"passed_count"`
    FailedCount      int                       `json:"failed_count"`
    Tasks            []TaskResult              `json:"tasks"`
    CategoryCoverage map[string]CategoryStatus `json:"category_coverage,omitempty"`
    ReportPath       string                    `json:"report_path,omitempty"`
}
```

**Step 2: Create CI-safe test scenarios**

Minimum scenarios for Phase 20:

| Scenario                | Category    | What it exercises                                       |
| ----------------------- | ----------- | ------------------------------------------------------- |
| `e1_ice1_emission`      | emission    | ICE-1 at 250 km/h, verify multi-Fz multi-Teilquelle sum |
| `e2_gueterzug_emission` | emission    | Güterzug at 100 km/h, different brake types             |
| `e3_feste_fahrbahn`     | emission    | ICE-3 on feste Fahrbahn, c1 corrections                 |
| `e4_bueg_correction`    | emission    | büG surface correction c2                               |
| `e5_bridge_kbr`         | emission    | Bridge type 2, K_Br=6 dB                                |
| `e6_curve_kl`           | emission    | Curve r=250m, K_L=3 dB                                  |
| `p1_free_field`         | propagation | Free-field: A_div + A_atm only, single receiver at 25m  |
| `p2_ground_effect`      | propagation | Ground effect: A_gr,B with varied h_m                   |
| `p3_directivity`        | propagation | Verify D_I at different angles along track              |
| `b1_single_barrier`     | barrier     | Single barrier, Dz with C₂=40                           |
| `b2_double_barrier`     | barrier     | Double barrier with C₃                                  |
| `a1_beurteilungspegel`  | assessment  | Full chain: emission → propagation → L_r,Tag/L_r,Nacht  |

Each scenario has a `.scenario.json` input and `.golden.json` expected output.

**Step 3: Write runner test**

```go
func TestRunCISafeSuiteProducesPassingReport(t *testing.T) {
    t.Parallel()
    report, err := Run(Options{
        Mode:      "ci-safe",
        OutputDir: t.TempDir(),
    })
    if err != nil {
        t.Fatalf("run: %v", err)
    }
    if report.Status != "passed" {
        t.Errorf("expected passed, got %s", report.Status)
        for _, task := range report.Tasks {
            if task.Status == "failed" {
                t.Logf("FAIL: %s — %s", task.Name, task.Message)
            }
        }
    }
}
```

**Step 4: Generate golden snapshots**

```bash
cd backend && UPDATE_GOLDEN=1 go test ./internal/qa/acceptance/schall03/ -run TestUpdateGoldenSnapshots -count=1
```

**Step 5: Run full suite**

```bash
cd backend && go test ./internal/qa/acceptance/schall03/ -v -count=1
```

Expected: all PASS.

**Step 6: Commit**

```bash
git add backend/internal/qa/acceptance/schall03/
git commit -m "feat(schall03): add dedicated conformance runner with CI-safe test suite for Eisenbahn Strecke"
```

---

## Task 8: Documentation and PLAN.md updates

**Files:**

- Create: `docs/conformance/schall03-konformitaetserklaerung.md`
- Modify: `PLAN.md`

**Step 1: Create conformance boundary document**

```markdown
# Schall 03 Konformitätserklärung — Aconiq

Status: DRAFT — Eisenbahn Strecke only

## Software

- Name: Aconiq
- Module: schall03
- Version: phase20-eisenbahn-strecke-v1
- License: MIT

## Standard

- Standard: Schall 03 (Anlage 2 zu §4 der 16. BImSchV)
- Legal basis: 16. BImSchV
- Source document: Anlage 2 (amtliches Werk per §5 UrhG)

## Scope

### Supported

- Eisenbahn Strecke: Fz-Kategorien 1-10, Beiblatt 1
- Full emission chain: Gl. 1-2, multi-Teilquelle, 11 sub-sources, 3 height levels
- Zugarten decomposition: Table 4 (19 standard train types)
- Corrections: Fahrbahn c1 (Table 7), surface c2 (Table 8), bridge K_Br/K_LM (Table 9), Auffälligkeit K_L (Table 11)
- Propagation: A*div (Gl. 11), A_atm (Gl. 12, Table 17), A_gr (Gl. 13-16), D_I (Gl. 8), D*Ω (Gl. 9)
- Barrier diffraction: A_bar (Gl. 18-26), single and double barriers, K_met
- Assessment: Beurteilungspegel Gl. 33-34, K_S=0 (abolished)
- Indicators: L_r,Tag, L_r,Nacht, L_p,Aeq,Tag, L_p,Aeq,Nacht

### Not yet supported

- Straßenbahnen (Fz 21-23, Beiblatt 2)
- Rangier- und Umschlagbahnhöfe (Table 10, Beiblatt 3)
- Image-source reflections (Gl. 27-28)
- Section 9 innovations (measurement-based vehicle data)
- Water body ground correction (A_gr,W, Gl. 16) — deferred

## Evidence

- CI-safe suite: repo-authored synthetic scenarios
- No official conformance test suite exists for Schall 03
```

**Step 2: Update PLAN.md**

Mark Phase 20 implementation items as completed. Add new future phases:

```markdown
## Phase 20a — Schall 03: Straßenbahnen (deferred)

- [ ] Encode Beiblatt 2 (Fz-Kategorien 21-23)
- [ ] Encode Tables 12-16 (Straßenbahn-specific corrections)
- [ ] Implement Straßenbahn speed rules (Nr. 5.3)
- [ ] Add Straßenbahn Fahrbahn corrections (Table 15)
- [ ] Add Straßenbahn bridge corrections (Table 16)
- [ ] Add CI-safe test scenarios for Straßenbahn emission and propagation
- [ ] Update conformance boundary document

## Phase 20b — Schall 03: Rangier- und Umschlagbahnhöfe (deferred)

- [ ] Encode Beiblatt 3 (Rangierbahnhof sound source data)
- [ ] Encode Table 10 (Schallquellen in Rangierbahnhöfen)
- [ ] Implement point and line source handling for yard sources
- [ ] Implement area source (Flächenschallquelle) aggregation per Gl. 5
- [ ] Add Rangierbahnhof assessment (Gl. 35-36)
- [ ] Add CI-safe test scenarios
- [ ] Update conformance boundary document

## Phase 20c — Schall 03: Reflections (deferred)

- [ ] Implement image-source reflections per Gl. 27-28
- [ ] Encode Table 18 (Absorptionsverlust an Wänden)
- [ ] Implement Fresnel zone minimum size check (Gl. 27)
- [ ] Up to 3rd-order reflections
- [ ] Add CI-safe test scenarios for reflection paths
```

**Step 3: Commit**

```bash
git add docs/conformance/schall03-konformitaetserklaerung.md PLAN.md
git commit -m "docs: add Schall 03 conformance boundary document and update PLAN.md with Phase 20/20a/20b/20c"
```

---

## Dependency graph

```
Task 1 (Beiblatt 1 data)
  └─→ Task 2 (emission chain)
       └─→ Task 3 (source model) ─── can partially parallel with Task 2
            └─→ Task 4 (propagation)
                 └─→ Task 5 (barrier diffraction)
                      └─→ Task 6 (assessment + indicators)
                           └─→ Task 7 (conformance runner)
                                └─→ Task 8 (documentation)
```

Tasks 1-3 (data + emission + source model) are the foundation.
Tasks 4-5 (propagation + barriers) build on the source model.
Task 6 (assessment) wires everything together.
Task 7 (conformance) validates the full chain.
Task 8 (docs) is the final step.
