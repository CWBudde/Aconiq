package schall03

import (
	"testing"
)

func TestFzKategorienCount(t *testing.T) {
	t.Parallel()

	if got := len(FzKategorien); got != 10 {
		t.Fatalf("expected 10 Fz-Kategorien, got %d", got)
	}
}

func TestFzKategorie1HGVTriebkopf(t *testing.T) {
	t.Parallel()

	fz := FzKategorien[0]
	if fz.Fz != 1 {
		t.Fatalf("expected Fz=1, got %d", fz.Fz)
	}

	if fz.Name != "HGV-Triebkopf" {
		t.Fatalf("expected name HGV-Triebkopf, got %q", fz.Name)
	}

	if fz.NAchs0 != 4 {
		t.Fatalf("expected NAchs0=4, got %d", fz.NAchs0)
	}

	if len(fz.Teilquellen) != 8 {
		t.Fatalf("expected 8 Teilquellen, got %d", len(fz.Teilquellen))
	}

	// Spot-check m=1 (Schienenrauheit)
	tq1 := fz.Teilquellen[0]
	if tq1.M != 1 {
		t.Errorf("expected M=1, got %d", tq1.M)
	}

	if tq1.SourceType != SourceTypeRolling {
		t.Errorf("expected SourceType=%q, got %q", SourceTypeRolling, tq1.SourceType)
	}

	if tq1.AA != 62 {
		t.Errorf("m=1: expected a_A=62, got %g", tq1.AA)
	}

	expectedDeltaA := BeiblattSpectrum{-50, -40, -24, -8, -3, -6, -11, -30}
	if tq1.DeltaA != expectedDeltaA {
		t.Errorf("m=1: expected DeltaA=%v, got %v", expectedDeltaA, tq1.DeltaA)
	}

	// Spot-check m=8 (Aggregate, 4m)
	tq8 := fz.Teilquellen[5] // m=8 is at index 5
	if tq8.M != 8 {
		t.Errorf("expected M=8, got %d", tq8.M)
	}

	if tq8.HeightH != 2 {
		t.Errorf("m=8: expected HeightH=2, got %d", tq8.HeightH)
	}

	if tq8.HeightM != 4 {
		t.Errorf("m=8: expected HeightM=4, got %g", tq8.HeightM)
	}

	if tq8.AA != 62 {
		t.Errorf("m=8: expected a_A=62, got %g", tq8.AA)
	}

	// Spot-check m=11 (Drive)
	tq11 := fz.Teilquellen[7]
	if tq11.M != 11 {
		t.Errorf("expected M=11, got %d", tq11.M)
	}

	if tq11.AA != 50 {
		t.Errorf("m=11: expected a_A=50, got %g", tq11.AA)
	}
}

func TestFzKategorienAllHaveTeilquellen(t *testing.T) {
	t.Parallel()

	for i, fz := range FzKategorien {
		if len(fz.Teilquellen) == 0 {
			t.Errorf("FzKategorien[%d] (Fz=%d, %q) has no Teilquellen", i, fz.Fz, fz.Name)
		}
	}
}

func TestFzKategorienValidSourceTypes(t *testing.T) {
	t.Parallel()

	validTypes := map[string]bool{
		SourceTypeRolling:     true,
		SourceTypeAerodynamic: true,
		SourceTypeAggregate:   true,
		SourceTypeDrive:       true,
	}

	for _, fz := range FzKategorien {
		for _, tq := range fz.Teilquellen {
			if !validTypes[tq.SourceType] {
				t.Errorf("Fz %d, m=%d: invalid SourceType %q", fz.Fz, tq.M, tq.SourceType)
			}
		}
	}
}

func TestFzKategorienConsecutiveNumbers(t *testing.T) {
	t.Parallel()

	for i, fz := range FzKategorien {
		if fz.Fz != i+1 {
			t.Errorf("FzKategorien[%d]: expected Fz=%d, got %d", i, i+1, fz.Fz)
		}
	}
}

func TestFzKategorienHeightConsistency(t *testing.T) {
	t.Parallel()

	heightMap := map[int]float64{1: 0, 2: 4, 3: 5}

	for _, fz := range FzKategorien {
		for _, tq := range fz.Teilquellen {
			expected, ok := heightMap[tq.HeightH]
			if !ok {
				t.Errorf("Fz %d, m=%d: invalid HeightH=%d", fz.Fz, tq.M, tq.HeightH)

				continue
			}

			if tq.HeightM != expected {
				t.Errorf("Fz %d, m=%d: HeightH=%d expects HeightM=%g, got %g",
					fz.Fz, tq.M, tq.HeightH, expected, tq.HeightM)
			}
		}
	}
}

func TestZugartenCount(t *testing.T) {
	t.Parallel()

	if got := len(Zugarten); got != 19 {
		t.Fatalf("expected 19 Zugarten, got %d", got)
	}
}

func TestZugartICE1Composition(t *testing.T) {
	t.Parallel()

	ice1 := Zugarten[0]
	if ice1.Name != "ICE-1-Zug" {
		t.Fatalf("expected ICE-1-Zug, got %q", ice1.Name)
	}

	if ice1.MaxSpeedKPH != 250 {
		t.Errorf("expected MaxSpeedKPH=250, got %g", ice1.MaxSpeedKPH)
	}

	if len(ice1.Composition) != 2 {
		t.Fatalf("expected 2 composition entries, got %d", len(ice1.Composition))
	}

	if ice1.Composition[0].Fz != 1 || ice1.Composition[0].Count != 2 {
		t.Errorf("expected 2xFz1, got %d x Fz%d", ice1.Composition[0].Count, ice1.Composition[0].Fz)
	}

	if ice1.Composition[1].Fz != 2 || ice1.Composition[1].Count != 12 {
		t.Errorf("expected 12xFz2, got %d x Fz%d", ice1.Composition[1].Count, ice1.Composition[1].Fz)
	}
}

func TestZugartenValidFzReferences(t *testing.T) {
	t.Parallel()

	for _, za := range Zugarten {
		for _, fc := range za.Composition {
			if fc.Fz < 1 || fc.Fz > 10 {
				t.Errorf("Zugart %q references invalid Fz=%d", za.Name, fc.Fz)
			}

			if fc.Count < 1 {
				t.Errorf("Zugart %q has Fz%d count=%d (must be >= 1)", za.Name, fc.Fz, fc.Count)
			}
		}
	}
}

func TestFz8HasNoM9(t *testing.T) {
	t.Parallel()

	fz8 := FzKategorien[7]
	if fz8.Fz != 8 {
		t.Fatalf("expected Fz=8, got %d", fz8.Fz)
	}

	for _, tq := range fz8.Teilquellen {
		if tq.M == 9 {
			t.Error("Fz 8 (V-Lok) should not have Teilquelle m=9")
		}
	}
}

func TestFz3DefaultZweiSystem(t *testing.T) {
	t.Parallel()

	fz3 := FzKategorien[2]
	if fz3.Fz != 3 {
		t.Fatalf("expected Fz=3, got %d", fz3.Fz)
	}

	for _, tq := range fz3.Teilquellen {
		if tq.M == 6 {
			if tq.AA != 46 {
				t.Errorf("Fz 3, m=6: expected Zwei-System a_A=46, got %g", tq.AA)
			}

			return
		}
	}

	t.Error("Fz 3 missing Teilquelle m=6")
}

func TestFz10DefaultVKBremse(t *testing.T) {
	t.Parallel()

	fz10 := FzKategorien[9]
	if fz10.Fz != 10 {
		t.Fatalf("expected Fz=10, got %d", fz10.Fz)
	}

	for _, tq := range fz10.Teilquellen {
		if tq.M == 2 {
			// VK-Bremse: a_A=58 (not GG-Bremse 71)
			if tq.AA != 58 {
				t.Errorf("Fz 10, m=2: expected VK-Bremse a_A=58, got %g", tq.AA)
			}

			return
		}
	}

	t.Error("Fz 10 missing Teilquelle m=2")
}

func TestFzKategorienStrassenbahnCount(t *testing.T) {
	if got := len(FzKategorienStrassenbahn); got != 3 {
		t.Fatalf("expected 3 Strassenbahn FzKategorien, got %d", got)
	}
}

func TestFzKat21Niederflur(t *testing.T) {
	fz21 := FzKategorienStrassenbahn[0]

	if fz21.Fz != 21 {
		t.Errorf("expected Fz=21, got %d", fz21.Fz)
	}

	if fz21.NAchs0 != 8 {
		t.Errorf("expected NAchs0=8, got %d", fz21.NAchs0)
	}

	if len(fz21.Teilquellen) != 3 {
		t.Errorf("expected 3 Teilquellen for Fz 21 (m=1,2,4), got %d", len(fz21.Teilquellen))
	}
}

func TestFzKat21SpeedFactorEmbedded(t *testing.T) {
	fz21 := FzKategorienStrassenbahn[0]

	for _, tq := range fz21.Teilquellen {
		if tq.B == nil {
			t.Errorf("Fz 21 Teilquelle m=%d has nil B; all Strassenbahn Teilquellen must embed B", tq.M)
		}
	}
}

func TestFzKat22HasAggregateAtFloor(t *testing.T) {
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
				t.Errorf("Fz 23 m=%d: wrong speed factor B, got %v", tq.M, tq.B)
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

func TestZugartStrassenbahnCount(t *testing.T) {
	if got := len(ZugartStrassenbahn); got != 3 {
		t.Fatalf("expected 3 ZugartStrassenbahn entries, got %d", got)
	}
}

func TestZugartNiederflurETComposition(t *testing.T) {
	found := false

	for _, za := range ZugartStrassenbahn {
		if za.Name == "Niederflur-ET" {
			found = true

			if len(za.Composition) != 1 || za.Composition[0].Fz != 21 {
				t.Errorf("Niederflur-ET should have 1 Fz=21 entry, got %v", za.Composition)
			}
		}
	}

	if !found {
		t.Error("ZugartStrassenbahn missing Niederflur-ET")
	}
}
