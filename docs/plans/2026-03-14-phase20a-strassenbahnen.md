# Phase 20a: Straßenbahnen Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extend the schall03 module with Straßenbahn (tram) vehicle support — Fz 21–23 from Beiblatt 2, Tables 14–16, Straßenbahn speed rules, and conformance test scenarios.

**Architecture:** New `beiblatt2.go` holds Fz 21–23 normative data; new `tables_strassenbahn.go` holds Tables 14–16. A shared `LookupFzKategorie(fz)` function searches both arrays. Each Teilquelle carries its own speed-factor spectrum `B *BeiblattSpectrum` (nil = fall back to Eisenbahn Table 6). The emission pipeline detects Straßenbahn by `fz >= 21` and routes to the correct correction tables.

**Tech Stack:** Go 1.22+, `backend/internal/standards/schall03/`, `just test`, `just lint`

---

## Normative Data Reference

All values extracted from `docs/bimsch16_anl2_neu-1.pdf`.

### Beiblatt 2 — Fz 21–23

**Fz 21 — Straßenbahn-Niederflurfahrzeuge (n_Achs,0 = 8)**

| m   | SourceType | h   | H (SO) | Δa [63..8000 Hz]              | a_A |
| --- | ---------- | --- | ------ | ----------------------------- | --- |
| 1   | rolling    | 1   | 0 m    | -34,-25,-20,-10,-2,-7,-12,-20 | 63  |
| 2   | rolling    | 1   | 0 m    | -34,-25,-20,-10,-2,-7,-12,-20 | 63  |
| 4   | aggregate  | 2   | 4 m    | -26,-15,-11,-8,-5,-6,-10,-11  | 39  |

Note: Fz 21 with Klimaanlage → a_A of m=4 raised to 47 (use 39 as baseline without).

**Fz 22 — Straßenbahn-Hochflurfahrzeuge (n_Achs,0 = 8)**

| m   | SourceType | h   | H (SO) | Δa [63..8000 Hz]              | a_A |
| --- | ---------- | --- | ------ | ----------------------------- | --- |
| 1   | rolling    | 1   | 0 m    | -32,-23,-17,-11,-2,-7,-12,-19 | 63  |
| 2   | rolling    | 1   | 0 m    | -32,-23,-17,-11,-2,-7,-12,-19 | 63  |
| 3   | aggregate  | 1   | 0 m    | -26,-15,-11,-8,-5,-6,-10,-11  | 39  |

**Fz 23 — U-Bahn-Fahrzeuge (n_Achs,0 = 8)**

| m   | SourceType | h   | H (SO) | Δa [63..8000 Hz]             | a_A |
| --- | ---------- | --- | ------ | ---------------------------- | --- |
| 1   | rolling    | 1   | 0 m    | -34,-25,-13,-9,-4,-6,-10,-17 | 60  |
| 2   | rolling    | 1   | 0 m    | -34,-25,-13,-9,-4,-6,-10,-17 | 60  |
| 3   | aggregate  | 1   | 0 m    | -26,-15,-11,-8,-5,-6,-10,-11 | 39  |

### Table 14 — Geschwindigkeitsfaktor b für Straßenbahnen

| Row | Schallquellenart                | m    | 63  | 125 | 250 | 500 | 1000 | 2000 | 4000 | 8000 |
| --- | ------------------------------- | ---- | --- | --- | --- | --- | ---- | ---- | ---- | ---- |
| 1   | Fahrgeräusch Niederflur/Hochfl. | 1, 2 | 0   | 0   | -5  | 5   | 20   | 15   | 15   | 20   |
| 2   | Fahrgeräusch U-Bahn             | 1, 2 | 15  | 10  | 20  | 20  | 30   | 25   | 25   | 20   |
| 3   | Aggregatgeräusche               | 3, 4 | -10 | -10 | -10 | -10 | -10  | -10  | -10  | -10  |

**Implementation:** embed b directly in each Teilquelle via `B *BeiblattSpectrum` field.

### Table 15 — Pegelkorrekturen c1 für Fahrbahnarten (Straßenbahn)

Applies to Teilquellen m=1,2. Reference (no correction): Schwellengleis.

| Type const                | Description                                          | c1 [63..8000 Hz]        |
| ------------------------- | ---------------------------------------------------- | ----------------------- |
| `SFahrbahnStraßenbuendig` | Straßenbündiger Bahnkörper und feste Fahrbahn        | 2, 3, 2, 5, 8, 4, 2, 1  |
| `SFahrbahnGruenTief`      | Begrünter Bahnkörper, tief liegende Vegetationsebene | -2,-4,-3,-1,-1,-1,-1,-3 |
| `SFahrbahnGruenHoch`      | Begrünter Bahnkörper, hoch liegende Vegetationsebene | 1,-1,-3,-4,-4,-7,-7,-5  |

Zero value = Schwellengleis (reference, no correction).

### Table 16 — Korrekturen K_Br und K_LM für Brücken (Straßenbahn)

5 types; apply to m=1,2 (Fahrgeräusche) only.

| Index | Description                                                  | K_Br | K_LM |
| ----- | ------------------------------------------------------------ | ---- | ---- |
| 1     | Stählerner Überbau, Gleise direkt aufgelagert                | 12   | -6   |
| 2     | Stählerner Überbau, Schwellengleis im Schotterbett           | 6    | -3   |
| 3     | Stählerner/massiver Überbau, Rillenschiene in Straßenfahrb.  | 4    | —    |
| 4     | Massive Fahrbahnplatte/besond. stähl. Überbau + Schotterbett | 3    | -3   |
| 5     | Massive Fahrbahnplatte, feste Fahrbahn, direkt aufgelagert   | 4    | —    |

### Nr. 5.3.2 — Straßenbahn Speed Rules

- Effective speed = min(v_track_max, v_vehicle_max)
- If effective speed < 50 km/h → clamp to 50 km/h
- Exception: permanently slow sections (≤ 30 km/h, r > 200 m, no stops/switches/crossings) → clamp to 30 km/h (out of scope for Phase 20a; note in code)
- Curve penalty: if r < 200 m and no K_L mitigation → add K_L = +4 dB to rolling noise (m=1,2)

### Beurteilungspegel Straßenbahn (Gl. 37–38)

Same as Eisenbahn Gl. 33–34: `L_r = L_p,Aeq + K_S` with `K_S = -5 dB`.
The existing `indicators.go` pipeline already applies K_S; no changes needed there.

---

## Tasks

### Task 1: Add `B *BeiblattSpectrum` to `Teilquelle`

**Files:**

- Modify: `backend/internal/standards/schall03/beiblatt1.go`

**Step 1: Add the field**

In `beiblatt1.go`, extend the `Teilquelle` struct:

```go
// Teilquelle describes one sub-source of a Fahrzeug-Kategorie.
type Teilquelle struct {
	M          int              // Teilquelle number (1-11)
	SourceType string           // "rolling" | "aerodynamic" | "aggregate" | "drive"
	HeightH    int              // Hoehenbereich: 1 (0m SO), 2 (4m SO), 3 (5m SO)
	HeightM    float64          // actual height above SO in meters
	DeltaA     BeiblattSpectrum // Delta a_f octave-band difference in dB
	AA         float64          // a_A total A-weighted level in dB
	// B overrides the global speed-factor table lookup (Table 6) when non-nil.
	// Straßenbahn Teilquellen set this to the Table 14 value for their Fz class.
	B *BeiblattSpectrum
}
```

**Step 2: Run tests to confirm nothing broke**

```bash
cd /mnt/projekte/Code/Aconiq/backend && go test ./internal/standards/schall03/...
```

Expected: all pass (B is nil everywhere in existing Eisenbahn data; no behavior change).

**Step 3: Commit**

```bash
cd /mnt/projekte/Code/Aconiq
git add backend/internal/standards/schall03/beiblatt1.go
git commit -m "feat(schall03): add optional speed-factor override B to Teilquelle"
```

---

### Task 2: Create `beiblatt2.go` — normative data for Fz 21–23

**Files:**

- Create: `backend/internal/standards/schall03/beiblatt2.go`

**Step 1: Write the failing test first**

In `backend/internal/standards/schall03/beiblatt1_test.go`, add after the existing `TestZugartenCount`:

```go
func TestFzKategorienStrassenbahnCount(t *testing.T) {
	if got := len(FzKategorienStrassenbahn); got != 3 {
		t.Fatalf("expected 3 Straßenbahn FzKategorien, got %d", got)
	}
}

func TestFzKat21NiederflurAxleCount(t *testing.T) {
	fz21 := FzKategorienStrassenbahn[0]
	if fz21.Fz != 21 {
		t.Errorf("expected Fz=21, got %d", fz21.Fz)
	}
	if fz21.NAchs0 != 8 {
		t.Errorf("expected NAchs0=8, got %d", fz21.NAchs0)
	}
	if len(fz21.Teilquellen) != 3 { // m=1,2,4
		t.Errorf("expected 3 Teilquellen for Fz 21, got %d", len(fz21.Teilquellen))
	}
}

func TestFzKat21SpeedFactorEmbedded(t *testing.T) {
	fz21 := FzKategorienStrassenbahn[0]
	for _, tq := range fz21.Teilquellen {
		if tq.B == nil {
			t.Errorf("Fz 21 Teilquelle m=%d has nil B; all Straßenbahn Teilquellen must embed b", tq.M)
		}
	}
}

func TestFzKat22TeilquellenHaveAggregateAtFloor(t *testing.T) {
	fz22 := FzKategorienStrassenbahn[1]
	found := false
	for _, tq := range fz22.Teilquellen {
		if tq.M == 3 && tq.HeightH == 1 && tq.HeightM == 0 {
			found = true
		}
	}
	if !found {
		t.Error("Fz 22 should have m=3 Aggregat at h=1 (0m)")
	}
}

func TestFzKat23UBahnSpeedFactor(t *testing.T) {
	fz23 := FzKategorienStrassenbahn[2]
	for _, tq := range fz23.Teilquellen {
		if tq.M == 1 || tq.M == 2 {
			want := BeiblattSpectrum{15, 10, 20, 20, 30, 25, 25, 20}
			if tq.B == nil || *tq.B != want {
				t.Errorf("Fz 23 m=%d: wrong speed factor B", tq.M)
			}
		}
	}
}

func TestLookupFzKategorieEisenbahn(t *testing.T) {
	fz, ok := LookupFzKategorie(7)
	if !ok {
		t.Fatal("LookupFzKategorie(7) returned not found")
	}
	if fz.Name != "E-Lok" {
		t.Errorf("expected E-Lok, got %q", fz.Name)
	}
}

func TestLookupFzKategorieStrassenbahn(t *testing.T) {
	fz, ok := LookupFzKategorie(21)
	if !ok {
		t.Fatal("LookupFzKategorie(21) returned not found")
	}
	if fz.Fz != 21 {
		t.Errorf("expected Fz=21, got %d", fz.Fz)
	}
}

func TestLookupFzKategorieUnknown(t *testing.T) {
	_, ok := LookupFzKategorie(99)
	if ok {
		t.Error("LookupFzKategorie(99) should return not found")
	}
}
```

**Step 2: Run to verify it fails**

```bash
cd /mnt/projekte/Code/Aconiq/backend && go test ./internal/standards/schall03/... -run "TestFzKategorienStrassenbahn|TestFzKat|TestLookupFz" 2>&1 | head -20
```

Expected: compile error — `FzKategorienStrassenbahn` and `LookupFzKategorie` undefined.

**Step 3: Create `beiblatt2.go`**

```go
package schall03

// Beiblatt 2 — Datenblatter Strassenbahnen (Strecke)
//
// This file encodes normative acoustic data from Anlage 2 zu Paragraph 4 der
// 16. BImSchV (Schall 03), Beiblatt 2.  The coefficients are amtliches Werk
// per Paragraph 5 UrhG and may be embedded in MIT-licensed code.
//
// Reference speed v_0 = 100 km/h on Schwellengleis im Schotterbett.
// Octave bands: 63, 125, 250, 500, 1000, 2000, 4000, 8000 Hz.

// FzKategorienStrassenbahn contains the three Strassenbahn Fahrzeug-Kategorien
// from Beiblatt 2 (Fz 21-23).
var FzKategorienStrassenbahn = [3]FzKategorie{
	fzKat21NiederflurET(),
	fzKat22HochflurET(),
	fzKat23UBahn(),
}

// Speed-factor spectra from Table 14.
var (
	bStrassenbahnFahrNiederHoch = BeiblattSpectrum{0, 0, -5, 5, 20, 15, 15, 20}
	bStrassenbahnFahrUBahn      = BeiblattSpectrum{15, 10, 20, 20, 30, 25, 25, 20}
	bStrassenbahnAggregat       = BeiblattSpectrum{-10, -10, -10, -10, -10, -10, -10, -10}
)

// --- Fz-Kategorie 21: Strassenbahn-Niederflurfahrzeuge (n_Achs,0 = 8) ---
func fzKat21NiederflurET() FzKategorie {
	return FzKategorie{
		Fz:     21,
		Name:   "Strassenbahn-Niederflurfahrzeuge",
		NAchs0: 8,
		Teilquellen: []Teilquelle{
			{
				M:          1,
				SourceType: SourceTypeRolling,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-34, -25, -20, -10, -2, -7, -12, -20},
				AA:         63,
				B:          &bStrassenbahnFahrNiederHoch,
			},
			{
				M:          2,
				SourceType: SourceTypeRolling,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-34, -25, -20, -10, -2, -7, -12, -20},
				AA:         63,
				B:          &bStrassenbahnFahrNiederHoch,
			},
			{
				// m=4: Aggregat on roof (Hoehe 4 m).  For vehicles with
				// Klimaanlage the a_A is raised to 47 dB; the baseline
				// (without climate) is 39 dB as given in Beiblatt 2.
				M:          4,
				SourceType: SourceTypeAggregate,
				HeightH:    2,
				HeightM:    4,
				DeltaA:     BeiblattSpectrum{-26, -15, -11, -8, -5, -6, -10, -11},
				AA:         39,
				B:          &bStrassenbahnAggregat,
			},
		},
	}
}

// --- Fz-Kategorie 22: Strassenbahn-Hochflurfahrzeuge (n_Achs,0 = 8) ---
func fzKat22HochflurET() FzKategorie {
	return FzKategorie{
		Fz:     22,
		Name:   "Strassenbahn-Hochflurfahrzeuge",
		NAchs0: 8,
		Teilquellen: []Teilquelle{
			{
				M:          1,
				SourceType: SourceTypeRolling,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-32, -23, -17, -11, -2, -7, -12, -19},
				AA:         63,
				B:          &bStrassenbahnFahrNiederHoch,
			},
			{
				M:          2,
				SourceType: SourceTypeRolling,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-32, -23, -17, -11, -2, -7, -12, -19},
				AA:         63,
				B:          &bStrassenbahnFahrNiederHoch,
			},
			{
				// m=3: Aggregat under floor (Hoehe 0 m).
				M:          3,
				SourceType: SourceTypeAggregate,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-26, -15, -11, -8, -5, -6, -10, -11},
				AA:         39,
				B:          &bStrassenbahnAggregat,
			},
		},
	}
}

// --- Fz-Kategorie 23: U-Bahn-Fahrzeuge (n_Achs,0 = 8) ---
func fzKat23UBahn() FzKategorie {
	return FzKategorie{
		Fz:     23,
		Name:   "U-Bahn-Fahrzeuge",
		NAchs0: 8,
		Teilquellen: []Teilquelle{
			{
				M:          1,
				SourceType: SourceTypeRolling,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-34, -25, -13, -9, -4, -6, -10, -17},
				AA:         60,
				B:          &bStrassenbahnFahrUBahn,
			},
			{
				M:          2,
				SourceType: SourceTypeRolling,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-34, -25, -13, -9, -4, -6, -10, -17},
				AA:         60,
				B:          &bStrassenbahnFahrUBahn,
			},
			{
				// m=3: Aggregat under floor (Hoehe 0 m).
				M:          3,
				SourceType: SourceTypeAggregate,
				HeightH:    1,
				HeightM:    0,
				DeltaA:     BeiblattSpectrum{-26, -15, -11, -8, -5, -6, -10, -11},
				AA:         39,
				B:          &bStrassenbahnAggregat,
			},
		},
	}
}

// LookupFzKategorie returns the FzKategorie for the given Fz number,
// searching both FzKategorien (Eisenbahn, 1-10) and
// FzKategorienStrassenbahn (21-23).
func LookupFzKategorie(fz int) (FzKategorie, bool) {
	for _, k := range FzKategorien {
		if k.Fz == fz {
			return k, true
		}
	}
	for _, k := range FzKategorienStrassenbahn {
		if k.Fz == fz {
			return k, true
		}
	}
	return FzKategorie{}, false
}

// IsStrassenbahnFz reports whether fz is a Strassenbahn vehicle category (21-23).
func IsStrassenbahnFz(fz int) bool {
	return fz >= 21 && fz <= 23
}

// ZugartStrassenbahn lists the three basic Strassenbahn Zugarten.
// Unlike Eisenbahn, the normative standard does not prescribe train compositions
// for Strassenbahnen; operators supply their own data.  These entries are
// convenience defaults for common vehicle types.
var ZugartStrassenbahn = [3]ZugartEntry{
	{Name: "Niederflur-ET", MaxSpeedKPH: 80, Composition: []FzCount{{21, 1}}},
	{Name: "Hochflur-ET", MaxSpeedKPH: 80, Composition: []FzCount{{22, 1}}},
	{Name: "Gelenktriebwagen", MaxSpeedKPH: 70, Composition: []FzCount{{21, 1}}},
}
```

**Step 4: Run tests**

```bash
cd /mnt/projekte/Code/Aconiq/backend && go test ./internal/standards/schall03/... -run "TestFzKategorienStrassenbahn|TestFzKat|TestLookupFz" -v
```

Expected: all pass.

**Step 5: Run full test suite + lint**

```bash
cd /mnt/projekte/Code/Aconiq && just test && just lint
```

Expected: all tests pass; lint clean.

**Step 6: Commit**

```bash
cd /mnt/projekte/Code/Aconiq
git add backend/internal/standards/schall03/beiblatt2.go backend/internal/standards/schall03/beiblatt1_test.go
git commit -m "feat(schall03): Beiblatt 2 normative data — Fz 21/22/23 Strassenbahn"
```

---

### Task 3: Create `tables_strassenbahn.go` — Tables 15 & 16

**Files:**

- Create: `backend/internal/standards/schall03/tables_strassenbahn.go`
- Modify: `backend/internal/standards/schall03/tables_test.go`

**Step 1: Write failing tests**

Add to `tables_test.go`:

```go
func TestC1StrassenbahnStraßenbuendig(t *testing.T) {
	entry, ok := C1StrassenbahnForType(SFahrbahnStraßenbuendig)
	if !ok {
		t.Fatal("SFahrbahnStraßenbuendig not found")
	}
	want := BeiblattSpectrum{2, 3, 2, 5, 8, 4, 2, 1}
	if entry.C1 != want {
		t.Errorf("got %v, want %v", entry.C1, want)
	}
}

func TestC1StrassenbahnGruenTief(t *testing.T) {
	entry, ok := C1StrassenbahnForType(SFahrbahnGruenTief)
	if !ok {
		t.Fatal("SFahrbahnGruenTief not found")
	}
	want := BeiblattSpectrum{-2, -4, -3, -1, -1, -1, -1, -3}
	if entry.C1 != want {
		t.Errorf("got %v, want %v", entry.C1, want)
	}
}

func TestC1StrassenbahnSchwellengleis(t *testing.T) {
	// Schwellengleis reference should return no correction.
	_, ok := C1StrassenbahnForType(SFahrbahnSchwellengleis)
	if ok {
		t.Error("SFahrbahnSchwellengleis should not be in table (reference = zero correction)")
	}
}

func TestBridgeStrassenbahnTable16Count(t *testing.T) {
	if len(BridgeCorrectionStrassenbahnTable) != 5 {
		t.Errorf("expected 5 Strassenbahn bridge types, got %d", len(BridgeCorrectionStrassenbahnTable))
	}
}

func TestBridgeStrassenbahnSteelDirect(t *testing.T) {
	entry := BridgeCorrectionStrassenbahnTable[0]
	if entry.KBr != 12 || entry.KLM != -6 {
		t.Errorf("expected KBr=12 KLM=-6, got KBr=%g KLM=%g", entry.KBr, entry.KLM)
	}
}

func TestBridgeStrassenbahnRillenschiene(t *testing.T) {
	entry := BridgeCorrectionStrassenbahnTable[2] // index 3, 0-based = 2
	if entry.KBr != 4 || !math.IsNaN(entry.KLM) {
		t.Errorf("Rillenschiene: expected KBr=4 KLM=NaN, got KBr=%g KLM=%g", entry.KBr, entry.KLM)
	}
}
```

**Step 2: Run to verify fails**

```bash
cd /mnt/projekte/Code/Aconiq/backend && go test ./internal/standards/schall03/... -run "TestC1Strassenbahn|TestBridgeStrassenbahn" 2>&1 | head -15
```

Expected: compile error.

**Step 3: Create `tables_strassenbahn.go`**

```go
package schall03

import "math"

// Normative correction tables for Strassenbahnen from Schall 03 (Anlage 2 zu
// Paragraph 4 der 16. BImSchV), Nummer 5 and Beiblatt 2.
//
// Octave bands: 63, 125, 250, 500, 1000, 2000, 4000, 8000 Hz.

// ---------------------------------------------------------------------------
// Table 15: Pegelkorrekturen c1 fuer Fahrbahnarten (Strassenbahnen)
// ---------------------------------------------------------------------------

// SFahrbahnartType identifies a Strassenbahn track surface type for Table 15.
type SFahrbahnartType int

const (
	// SFahrbahnSchwellengleis is the reference track type (Schwellengleis im
	// Schotterbett); no c1 correction is applied.  Intentionally does not
	// match any entry in C1StrassenbahnTable.
	SFahrbahnSchwellengleis SFahrbahnartType = iota - 1

	SFahrbahnStraßenbuendig // Strassenbuendiger Bahnkoerper und feste Fahrbahn
	SFahrbahnGruenTief      // Begruentter Bahnkoerper, tief liegende Vegetationsebene
	SFahrbahnGruenHoch      // Begruentter Bahnkoerper, hoch liegende Vegetationsebene
)

// SC1Entry holds the c1 correction for one Strassenbahn Fahrbahnart (Table 15).
// Corrections apply to Teilquellen m=1 and m=2 (Fahrgeraeusche) only.
type SC1Entry struct {
	Type SFahrbahnartType
	Name string
	C1   BeiblattSpectrum // correction in dB per octave band
}

// C1StrassenbahnTable contains Table 15 data.
var C1StrassenbahnTable = [3]SC1Entry{
	{
		Type: SFahrbahnStraßenbuendig,
		Name: "Strassenbuendiger Bahnkoerper und feste Fahrbahn",
		C1:   BeiblattSpectrum{2, 3, 2, 5, 8, 4, 2, 1},
	},
	{
		Type: SFahrbahnGruenTief,
		Name: "Begruentter Bahnkoerper, tief liegende Vegetationsebene",
		C1:   BeiblattSpectrum{-2, -4, -3, -1, -1, -1, -1, -3},
	},
	{
		Type: SFahrbahnGruenHoch,
		Name: "Begruentter Bahnkoerper, hoch liegende Vegetationsebene",
		C1:   BeiblattSpectrum{1, -1, -3, -4, -4, -7, -7, -5},
	},
}

// C1StrassenbahnForType returns the SC1Entry for the given SFahrbahnartType.
// Returns (zero, false) for SFahrbahnSchwellengleis or unknown types.
func C1StrassenbahnForType(t SFahrbahnartType) (SC1Entry, bool) {
	for _, e := range C1StrassenbahnTable {
		if e.Type == t {
			return e, true
		}
	}

	return SC1Entry{}, false
}

// sumC1StrassenbahnForTeilquelle returns the c1 correction from Table 15 for
// a given Fahrbahnart, Teilquelle m, and octave band index f.
// Corrections apply to Fahrgeraeusche (m=1, m=2) only; all other m return 0.
func sumC1StrassenbahnForTeilquelle(fahrbahn SFahrbahnartType, m, f int) float64 {
	if m != 1 && m != 2 {
		return 0
	}

	entry, ok := C1StrassenbahnForType(fahrbahn)
	if !ok {
		return 0
	}

	return entry.C1[f] //nolint:gosec // f bounded by NumBeiblattOctaveBands
}

// ---------------------------------------------------------------------------
// Table 16: Korrekturen K_Br und K_LM fuer Bruecken (Strassenbahnen)
// ---------------------------------------------------------------------------

// BridgeCorrectionStrassenbahnTable contains the five rows of Table 16.
// Corrections apply to Fahrgeraeusche (m=1, m=2) only.
var BridgeCorrectionStrassenbahnTable = [5]BridgeCorrectionEntry{
	{
		Type:        1,
		Description: "Bruecke mit staehlernem Ueberbau, Gleise direkt aufgelagert",
		KBr:         12,
		KLM:         -6,
	},
	{
		Type:        2,
		Description: "Bruecke mit staehlernem Ueberbau und Schwellengleis im Schotterbett",
		KBr:         6,
		KLM:         -3,
	},
	{
		Type:        3,
		Description: "Bruecke mit staehlernem oder massivem Ueberbau, Gleise in Strassenfahrbahn eingebettet (Rillenschiene)",
		KBr:         4,
		KLM:         math.NaN(),
	},
	{
		Type:        4,
		Description: "Bruecke mit massiver Fahrbahnplatte oder besonderem staehlernen Ueberbau, Schwellengleis im Schotterbett",
		KBr:         3,
		KLM:         -3,
	},
	{
		Type:        5,
		Description: "Bruecke mit massiver Fahrbahnplatte, Gleise direkt aufgelagert (feste Fahrbahn)",
		KBr:         4,
		KLM:         math.NaN(),
	},
}

// bridgeCorrectionStrassenbahnForTeilquelle returns the K_Br (+K_LM) bridge
// correction from Table 16 for a given bridge type and Teilquelle m.
// Only applies to Fahrgeraeusche (m=1, m=2).
func bridgeCorrectionStrassenbahnForTeilquelle(bridgeType int, bridgeMitig bool, m int) float64 {
	if m != 1 && m != 2 {
		return 0
	}

	if bridgeType < 1 || bridgeType > len(BridgeCorrectionStrassenbahnTable) {
		return 0
	}

	entry := BridgeCorrectionStrassenbahnTable[bridgeType-1]
	correction := entry.KBr

	if bridgeMitig && !math.IsNaN(entry.KLM) {
		correction += entry.KLM
	}

	return correction
}

// curveCorrectionStrassenbahnForTeilquelle returns the Strassenbahn K_L
// correction per Nr. 5.3.2 for a given curve radius and Teilquelle m.
//
// For curves with r < 200 m (and no active K_L mitigation measures), a fixed
// +4 dB is added to Fahrgeraeusche (m=1, m=2).  Curves with r >= 200 m carry
// no correction.
func curveCorrectionStrassenbahnForTeilquelle(curveRadiusM float64, m int) float64 {
	if m != 1 && m != 2 {
		return 0
	}

	if curveRadiusM > 0 && curveRadiusM < 200 {
		return 4
	}

	return 0
}
```

**Step 4: Run tests**

```bash
cd /mnt/projekte/Code/Aconiq/backend && go test ./internal/standards/schall03/... -run "TestC1Strassenbahn|TestBridgeStrassenbahn" -v
```

Expected: all pass.

**Step 5: Run full suite + lint**

```bash
cd /mnt/projekte/Code/Aconiq && just test && just lint
```

**Step 6: Commit**

```bash
cd /mnt/projekte/Code/Aconiq
git add backend/internal/standards/schall03/tables_strassenbahn.go backend/internal/standards/schall03/tables_test.go
git commit -m "feat(schall03): Strassenbahn Tables 15+16 — c1 track types and bridge corrections"
```

---

### Task 4: Wire Straßenbahn into the emission pipeline

**Files:**

- Modify: `backend/internal/standards/schall03/emission_v2.go`
- Modify: `backend/internal/standards/schall03/emission_v2_test.go`

**Step 1: Write failing tests**

Add to `emission_v2_test.go`:

```go
func TestStrassenbahnEmissionFz21Basic(t *testing.T) {
	// Fz 21 Niederflur at 50 km/h, reference track (Schwellengleis).
	// Checks that the pipeline accepts Fz 21 and returns a non-nil result.
	input := StreckeEmissionInput{
		Vehicles:    []VehicleInput{{Fz: 21, NPerHour: 10}},
		SpeedKPH:    50,
		Fahrbahn:    FahrbahnartSchwellengleis,
		SFahrbahn:   SFahrbahnSchwellengleis,
	}
	result, err := ComputeStreckeEmission(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.PerHeight) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestStrassenbahnSpeedClamp50(t *testing.T) {
	// Speeds below 50 km/h must be clamped to 50 for Strassenbahn (Nr. 5.3.2).
	// Same vehicle at 30 and 50 must produce the same result.
	base := StreckeEmissionInput{
		Vehicles:  []VehicleInput{{Fz: 21, NPerHour: 10}},
		SpeedKPH:  50,
		SFahrbahn: SFahrbahnSchwellengleis,
	}
	clamped := StreckeEmissionInput{
		Vehicles:  []VehicleInput{{Fz: 21, NPerHour: 10}},
		SpeedKPH:  30,
		SFahrbahn: SFahrbahnSchwellengleis,
	}
	r1, _ := ComputeStreckeEmission(base)
	r2, _ := ComputeStreckeEmission(clamped)

	for h, sp1 := range r1.PerHeight {
		sp2 := r2.PerHeight[h]
		for f := range NumBeiblattOctaveBands {
			if sp1[f] != sp2[f] {
				t.Errorf("h=%d f=%d: v=50 gave %g, v=30 gave %g (should be equal after clamp)", h, f, sp1[f], sp2[f])
			}
		}
	}
}

func TestStrassenbahnC1Correction(t *testing.T) {
	// Result with Straßenbündiger Bahnkörper must differ from Schwellengleis.
	ref := StreckeEmissionInput{
		Vehicles:  []VehicleInput{{Fz: 22, NPerHour: 10}},
		SpeedKPH:  60,
		SFahrbahn: SFahrbahnSchwellengleis,
	}
	corr := StreckeEmissionInput{
		Vehicles:  []VehicleInput{{Fz: 22, NPerHour: 10}},
		SpeedKPH:  60,
		SFahrbahn: SFahrbahnStraßenbuendig,
	}
	r1, _ := ComputeStreckeEmission(ref)
	r2, _ := ComputeStreckeEmission(corr)
	// At least one band should differ.
	h := 1
	for f := range NumBeiblattOctaveBands {
		if r1.PerHeight[h][f] != r2.PerHeight[h][f] {
			return // found a difference — pass
		}
	}
	t.Error("c1 correction had no effect; reference and corrected results are identical")
}

func TestMixedEisenbahnAndStrassenbahnRejected(t *testing.T) {
	// A segment must not mix Eisenbahn (Fz 1-10) and Strassenbahn (Fz 21-23).
	input := StreckeEmissionInput{
		Vehicles: []VehicleInput{
			{Fz: 7, NPerHour: 5},  // E-Lok (Eisenbahn)
			{Fz: 21, NPerHour: 5}, // Niederflur (Strassenbahn)
		},
		SpeedKPH:  80,
		SFahrbahn: SFahrbahnSchwellengleis,
	}
	_, err := ComputeStreckeEmission(input)
	if err == nil {
		t.Error("expected error when mixing Eisenbahn and Strassenbahn vehicles")
	}
}
```

**Step 2: Run to verify fails**

```bash
cd /mnt/projekte/Code/Aconiq/backend && go test ./internal/standards/schall03/... -run "TestStrassenbahn|TestMixed" 2>&1 | head -20
```

Expected: compile error — `SFahrbahn` field not found on `StreckeEmissionInput`.

**Step 3: Extend `StreckeEmissionInput` and `VehicleInput`**

In `emission_v2.go`:

1. Add `SFahrbahn SFahrbahnartType` to `StreckeEmissionInput`:

```go
// StreckeEmissionInput holds all parameters needed to compute emission for one
// track segment per Gl. 1-2 of Anlage 2 zu §4 der 16. BImSchV.
type StreckeEmissionInput struct {
	Vehicles []VehicleInput
	SpeedKPH float64
	// Eisenbahn track type (Table 7).  Ignored for Strassenbahn vehicles.
	Fahrbahn FahrbahnartType
	// Strassenbahn track type (Table 15).  Ignored for Eisenbahn vehicles.
	SFahrbahn   SFahrbahnartType
	Surface     SurfaceCondType
	BridgeType  int
	BridgeMitig bool
	CurveRadiusM float64
}
```

2. Update `buildFzMap()` to use `LookupFzKategorie`:

```go
func buildFzMap() map[int]*FzKategorie {
	m := make(map[int]*FzKategorie, len(FzKategorien)+len(FzKategorienStrassenbahn))
	for i := range FzKategorien {
		m[FzKategorien[i].Fz] = &FzKategorien[i]
	}
	for i := range FzKategorienStrassenbahn {
		m[FzKategorienStrassenbahn[i].Fz] = &FzKategorienStrassenbahn[i]
	}
	return m
}
```

3. Update `validateEmissionInput` to reject mixed Eisenbahn/Strassenbahn:

```go
// validateEmissionInput checks the emission input for basic validity.
func validateEmissionInput(input StreckeEmissionInput) error {
	if input.SpeedKPH <= 0 {
		return fmt.Errorf("speed must be > 0, got %g km/h", input.SpeedKPH)
	}
	if math.IsNaN(input.SpeedKPH) || math.IsInf(input.SpeedKPH, 0) {
		return fmt.Errorf("speed must be finite, got %g km/h", input.SpeedKPH)
	}
	if len(input.Vehicles) == 0 {
		return errors.New("at least one vehicle is required")
	}

	fzMap := buildFzMap()
	hasEisenbahn := false
	hasStrassenbahn := false

	for i, vi := range input.Vehicles {
		if _, ok := fzMap[vi.Fz]; !ok {
			return fmt.Errorf("vehicle[%d]: unknown Fz-Kategorie %d", i, vi.Fz)
		}
		if vi.NPerHour < 0 || math.IsNaN(vi.NPerHour) || math.IsInf(vi.NPerHour, 0) {
			return fmt.Errorf("vehicle[%d]: NPerHour must be finite and >= 0, got %g", i, vi.NPerHour)
		}
		if IsStrassenbahnFz(vi.Fz) {
			hasStrassenbahn = true
		} else {
			hasEisenbahn = true
		}
	}

	if hasEisenbahn && hasStrassenbahn {
		return errors.New("cannot mix Eisenbahn (Fz 1-10) and Strassenbahn (Fz 21-23) in one segment")
	}

	return nil
}
```

4. In `ComputeStreckeEmission`, add speed clamping and route Strassenbahn:

```go
func ComputeStreckeEmission(input StreckeEmissionInput) (*StreckeEmissionResult, error) {
	err := validateEmissionInput(input)
	if err != nil {
		return nil, err
	}

	fzMap := buildFzMap()

	// Detect mode from first vehicle (validation ensures they are consistent).
	isStrassenbahn := len(input.Vehicles) > 0 && IsStrassenbahnFz(input.Vehicles[0].Fz)

	// Nr. 5.3.2: Strassenbahn minimum effective speed is 50 km/h.
	// Exception for permanently ≤30 km/h slow sections is not yet modelled;
	// see Phase 20a open items in PLAN.md.
	effectiveSpeed := input.SpeedKPH
	if isStrassenbahn && effectiveSpeed < 50 {
		effectiveSpeed = 50
	}

	heightSums := map[int][NumBeiblattOctaveBands]float64{}

	for _, vi := range input.Vehicles {
		fz := fzMap[vi.Fz]
		nFz := vi.NPerHour

		for _, tq := range fz.Teilquellen {
			nQ := vi.AxleCount
			nQ0 := fz.NAchs0
			if nQ <= 0 {
				nQ = nQ0
			}

			level := computeTeilquelleLevel(
				tq, nQ, nQ0, effectiveSpeed,
				input.Fahrbahn, input.SFahrbahn, input.Surface,
				input.BridgeType, input.BridgeMitig, input.CurveRadiusM,
				isStrassenbahn,
			)

			h := tq.HeightH
			sums := heightSums[h]
			for f := range NumBeiblattOctaveBands {
				sums[f] += nFz * math.Pow(10, 0.1*level[f]) //nolint:gosec
			}
			heightSums[h] = sums
		}
	}

	result := &StreckeEmissionResult{
		PerHeight: make(map[int]BeiblattSpectrum, len(heightSums)),
	}
	for h, sums := range heightSums {
		var spectrum BeiblattSpectrum
		for f := range NumBeiblattOctaveBands {
			if sums[f] > 0 { //nolint:gosec
				spectrum[f] = 10 * math.Log10(sums[f])
			} else {
				spectrum[f] = math.Inf(-1)
			}
		}
		result.PerHeight[h] = spectrum
	}

	return result, nil
}
```

5. Update `computeTeilquelleLevel` signature and body:

```go
func computeTeilquelleLevel(
	tq Teilquelle,
	nQ, nQ0 int,
	speedKPH float64,
	fahrbahn FahrbahnartType,
	sFahrbahn SFahrbahnartType,
	surface SurfaceCondType,
	bridgeType int,
	bridgeMitig bool,
	curveRadiusM float64,
	isStrassenbahn bool,
) BeiblattSpectrum {
	// Speed factor b: use per-Teilquelle override if present (Strassenbahn),
	// otherwise fall back to global Table 6 lookup (Eisenbahn).
	var b BeiblattSpectrum
	if tq.B != nil {
		b = *tq.B
	} else {
		b = SpeedFactorBForTeilquelle(tq.M)
	}

	var result BeiblattSpectrum

	for f := range NumBeiblattOctaveBands {
		L := tq.AA + tq.DeltaA[f]

		// Axle correction: only for rolling noise (SourceType == rolling).
		if tq.SourceType == SourceTypeRolling && nQ > 0 && nQ0 > 0 {
			L += 10.0 * math.Log10(float64(nQ)/float64(nQ0))
		}

		// Speed correction: b_f * lg(v/v0).
		L += b[f] * math.Log10(speedKPH/v0)

		if isStrassenbahn {
			// Strassenbahn: c1 from Table 15 (track type), no c2.
			L += sumC1StrassenbahnForTeilquelle(sFahrbahn, tq.M, f)
			// Strassenbahn: bridge correction from Table 16.
			L += bridgeCorrectionStrassenbahnForTeilquelle(bridgeType, bridgeMitig, tq.M)
			// Strassenbahn: curve correction per Nr. 5.3.2.
			L += curveCorrectionStrassenbahnForTeilquelle(curveRadiusM, tq.M)
		} else {
			// Eisenbahn: c1 Table 7, c2 Table 8, bridge Table 9, curve Table 11.
			L += sumC1ForTeilquelle(fahrbahn, tq.M, f)
			L += sumC2ForTeilquelle(surface, tq.M, f)
			L += bridgeCorrectionForTeilquelle(bridgeType, bridgeMitig, tq.M)
			L += curveCorrectionForTeilquelle(curveRadiusM, tq.M)
		}

		result[f] = L
	}

	return result
}
```

Also update the old `isRollingNoise` usage — **remove the `isRollingNoise` function** since we now use `tq.SourceType == SourceTypeRolling` directly.

**Step 4: Run tests**

```bash
cd /mnt/projekte/Code/Aconiq/backend && go test ./internal/standards/schall03/... -v 2>&1 | tail -30
```

Expected: all tests pass.

**Step 5: Run lint**

```bash
cd /mnt/projekte/Code/Aconiq && just lint
```

Fix any issues (unused functions, formatting, etc.).

**Step 6: Commit**

```bash
cd /mnt/projekte/Code/Aconiq
git add backend/internal/standards/schall03/emission_v2.go backend/internal/standards/schall03/emission_v2_test.go
git commit -m "feat(schall03): wire Strassenbahn into emission pipeline with speed clamp, c1/bridge/curve routing"
```

---

### Task 5: Add Zugarten to `NewTrainOperationFromZugart`

**Files:**

- Modify: `backend/internal/standards/schall03/model.go`
- Modify: `backend/internal/standards/schall03/model_v2_test.go`

**Step 1: Check how `NewTrainOperationFromZugart` currently looks up Zugarten**

Read `model.go` around line 297 to understand the lookup mechanism.

**Step 2: Write failing test**

In `model_v2_test.go`:

```go
func TestNewTrainOperationFromZugartNiederflurET(t *testing.T) {
	op, err := NewTrainOperationFromZugart("Niederflur-ET", 10.0, 5.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op == nil {
		t.Fatal("expected non-nil operation")
	}
}
```

**Step 3: Extend the Zugart lookup in `model.go`**

Find `NewTrainOperationFromZugart` and extend it to also search `ZugartStrassenbahn`:

```go
func NewTrainOperationFromZugart(name string, nDay, nNight float64) (*TrainOperation, error) {
	// Search Eisenbahn Zugarten (Table 4).
	for _, za := range Zugarten {
		if za.Name == name {
			return newTrainOperationFromEntry(za, nDay, nNight), nil
		}
	}
	// Search Strassenbahn Zugarten (Beiblatt 2 convenience entries).
	for _, za := range ZugartStrassenbahn {
		if za.Name == name {
			return newTrainOperationFromEntry(za, nDay, nNight), nil
		}
	}
	return nil, fmt.Errorf("unknown Zugart %q", name)
}
```

(Or adapt to match the existing structure in `model.go`.)

**Step 4: Run tests**

```bash
cd /mnt/projekte/Code/Aconiq/backend && go test ./internal/standards/schall03/... -run "TestNewTrainOperation"
```

**Step 5: Commit**

```bash
cd /mnt/projekte/Code/Aconiq
git add backend/internal/standards/schall03/model.go backend/internal/standards/schall03/model_v2_test.go
git commit -m "feat(schall03): add Strassenbahn Zugarten to NewTrainOperationFromZugart lookup"
```

---

### Task 6: Add CI-safe conformance scenarios

**Files:**

- Create: `backend/internal/qa/acceptance/schall03/testdata/ci_safe/s1_niederflur_emission.scenario.json`
- Create: `backend/internal/qa/acceptance/schall03/testdata/ci_safe/s2_strassenbahn_fullchain.scenario.json`
- Modify: `backend/internal/qa/acceptance/schall03/testdata/ci_safe_suite.json`

**Step 1: Understand existing scenario format**

Read `backend/internal/qa/acceptance/schall03/testdata/ci_safe/e1_ice1_straight.scenario.json` and `a1_full_chain.scenario.json` to understand the JSON schema.

**Step 2: Create `s1_niederflur_emission.scenario.json`**

Scenario: one Fz 21 Niederflur-ET, 10 trains/hour, 60 km/h, Schwellengleis reference track, no bridges, straight.

```json
{
  "id": "s1_niederflur_emission",
  "description": "Fz 21 Niederflur-ET: emission level at 60 km/h, Schwellengleis, straight track",
  "type": "emission",
  "input": {
    "vehicles": [{ "fz": 21, "n_per_hour": 10 }],
    "speed_kph": 60,
    "fahrbahn": "schwellengleis",
    "s_fahrbahn": "schwellengleis",
    "surface": "none",
    "bridge_type": 0,
    "curve_radius_m": 0
  }
}
```

**Step 3: Create `s2_strassenbahn_fullchain.scenario.json`**

Scenario: Fz 21, 10 trains/hour day, 5 night, 60 km/h, free-field propagation to receiver at 50 m.

Use the same structure as `a1_full_chain.scenario.json` but with Fz 21 vehicles and `s_fahrbahn`.

**Step 4: Register in `ci_safe_suite.json`**

Add the two new scenario IDs to the suite file.

**Step 5: Generate golden snapshots**

```bash
cd /mnt/projekte/Code/Aconiq && just update-golden
```

**Step 6: Verify scenarios pass**

```bash
cd /mnt/projekte/Code/Aconiq && just test
```

**Step 7: Commit**

```bash
cd /mnt/projekte/Code/Aconiq
git add backend/internal/qa/acceptance/schall03/testdata/
git commit -m "feat(schall03): CI-safe conformance scenarios for Strassenbahn Fz 21 emission and full-chain"
```

---

### Task 7: Update conformance documentation

**Files:**

- Modify: `docs/conformance/schall03-konformitaetserklaerung.md`

**Step 1: Read the current conformance doc**

```bash
cat docs/conformance/schall03-konformitaetserklaerung.md
```

**Step 2: Mark Straßenbahnen as supported**

Add a section for Nr. 5 (Schallemissionen von Straßenbahnen):

- Beiblatt 2 data: Fz 21, 22, 23 implemented ✓
- Table 14 speed factors: embedded per Teilquelle ✓
- Table 15 track type c1 corrections: implemented ✓
- Table 16 bridge corrections (5 types): implemented ✓
- Nr. 5.3.2 speed clamp (≥ 50 km/h): implemented ✓
- Nr. 5.3.2 curve penalty (r < 200 m → K_L +4 dB): implemented ✓
- Nr. 5.3.2 permanently slow section exception (≤ 30 km/h): **not yet implemented** (Phase 20a open item)
- Beurteilungspegel Gl. 37–38: supported via existing Gl. 33–34 pipeline (identical formula)

**Step 3: Update PLAN.md to mark Phase 20a checkboxes**

In `PLAN.md`, mark all Phase 20a items as `[x]`.

**Step 4: Commit**

```bash
cd /mnt/projekte/Code/Aconiq
git add docs/conformance/schall03-konformitaetserklaerung.md PLAN.md
git commit -m "docs(schall03): mark Phase 20a Strassenbahnen as supported in conformance declaration"
```

---

## Notes

- **Speed clamp discrepancy:** PLAN.md stated "minimum 20 km/h" but the normative PDF (Nr. 5.3.2) specifies **50 km/h** as the effective minimum. The implementation follows the PDF.
- **No c2 for Straßenbahn:** Table 8 (büG, Schienenstegdämpfer) is Eisenbahn-only. The Straßenbahn section (Nr. 5) contains no equivalent surface condition correction.
- **No predefined Zugarten table for Straßenbahn:** Nr. 5.1 explicitly defers composition data to the transit operator. The three entries in `ZugartStrassenbahn` are convenience defaults, not normative.
- **U-Bahn scope:** Fz 23 is included in Beiblatt 2 and implemented. Its physical scope (underground sections) is outside this standard but the emission calculation is identical.
- **Permanently slow exception** (≤ 30 km/h sections per Nr. 5.3.2): deferred, noted as open item in conformance doc.
