package schall03

import (
	"math"
	"testing"
)

// almostEqualBarrier is a local helper for barrier tests.
func almostEqualBarrier(t *testing.T, got, want, tol float64, label string) {
	t.Helper()

	if math.Abs(got-want) > tol {
		t.Errorf("%s: expected %.4f, got %.4f (tol %.4f)", label, want, got, tol)
	}
}

// ---------------------------------------------------------------------------
// Gl. 21: D_z — screening attenuation
// ---------------------------------------------------------------------------

func TestBarrierDzGl21(t *testing.T) {
	t.Parallel()

	// D_z = 10·lg(3 + C₂/λ · C₃ · z · K_met)
	// Single barrier (C₃=1), no met correction (K_met=1)
	// z=0.5m, f=1000Hz → λ=0.34m, C₂=40
	// D_z = 10·lg(3 + 40/0.34 · 1 · 0.5 · 1) = 10·lg(3 + 58.82) = 10·lg(61.82) = 17.91
	lam := speedOfSound / 1000.0
	got := barrierDz(lam, 1.0, 0.5, 1.0)
	almostEqualBarrier(t, got, 17.91, 0.1, "D_z(z=0.5, f=1000Hz)")

	// z ≤ 0 → returns 0
	got0 := barrierDz(lam, 1.0, 0.0, 1.0)
	almostEqualBarrier(t, got0, 0.0, 0.001, "D_z(z=0) = 0")

	gotNeg := barrierDz(lam, 1.0, -1.0, 1.0)
	almostEqualBarrier(t, gotNeg, 0.0, 0.001, "D_z(z<0) = 0")
}

// ---------------------------------------------------------------------------
// Gl. 23-24: K_met — meteorological correction
// ---------------------------------------------------------------------------

func TestKmetGl23(t *testing.T) {
	t.Parallel()

	// K_met = exp(-1/2000 · sqrt(ds·dr·d / (2·z)))
	// ds=10, dr=5, d=14, z=1
	// arg = sqrt(10·5·14 / 2) = sqrt(350) = 18.71
	// K_met = exp(-18.71/2000) = exp(-0.009355) ≈ 0.9907
	km := kmet(10.0, 5.0, 14.0, 1.0)
	almostEqualBarrier(t, km, math.Exp(-math.Sqrt(10.0*5.0*14.0/2.0)/2000.0), 0.0001, "K_met(ds=10, dr=5)")

	// z ≤ 0 → K_met = 1
	km0 := kmet(10.0, 5.0, 14.0, 0.0)
	almostEqualBarrier(t, km0, 1.0, 0.0001, "K_met(z=0) = 1")

	kmNeg := kmet(10.0, 5.0, 14.0, -1.0)
	almostEqualBarrier(t, kmNeg, 1.0, 0.0001, "K_met(z<0) = 1")
}

// ---------------------------------------------------------------------------
// Gl. 25: path difference (parallel edges)
// ---------------------------------------------------------------------------

func TestPathDifferenceParallel(t *testing.T) {
	t.Parallel()

	// z = sqrt((ds + dr + e)² + dPar²) - d
	// ds=5, dr=5, e=0, dPar=0, d=10:
	// z = sqrt(100) - 10 = 0
	got := pathDifferenceParallel(5.0, 5.0, 0.0, 0.0, 10.0)
	almostEqualBarrier(t, got, 0.0, 0.0001, "path diff (no offset)")

	// ds=3, dr=4, e=0, dPar=0, d=5:
	// z = sqrt(49) - 5 = 7 - 5 = 2
	got2 := pathDifferenceParallel(3.0, 4.0, 0.0, 0.0, 5.0)
	almostEqualBarrier(t, got2, 2.0, 0.0001, "path diff (3-4-5 triangle)")

	// ds=3, dr=4, e=0, dPar=3, d=5:
	// z = sqrt(49 + 9) - 5 = sqrt(58) - 5 = 7.616 - 5 = 2.616
	got3 := pathDifferenceParallel(3.0, 4.0, 0.0, 3.0, 5.0)
	almostEqualBarrier(t, got3, math.Sqrt(58.0)-5.0, 0.0001, "path diff (with lateral)")
}

// ---------------------------------------------------------------------------
// Gl. 26: path difference (non-parallel edges)
// ---------------------------------------------------------------------------

func TestPathDifferenceNonParallel(t *testing.T) {
	t.Parallel()

	// z = (ds + dr + e) - d
	// ds=3, dr=4, e=0, d=5: z = 7 - 5 = 2
	got := pathDifferenceNonParallel(3.0, 4.0, 0.0, 5.0)
	almostEqualBarrier(t, got, 2.0, 0.0001, "non-parallel path diff")

	// With thick barrier e=1: z = 8 - 5 = 3
	got2 := pathDifferenceNonParallel(3.0, 4.0, 1.0, 5.0)
	almostEqualBarrier(t, got2, 3.0, 0.0001, "non-parallel with e=1")
}

// ---------------------------------------------------------------------------
// Gl. 22: C₃ — multiple diffraction factor
// ---------------------------------------------------------------------------

func TestC3Multiple(t *testing.T) {
	t.Parallel()

	// C₃ = (1 + (5λ/e)²) / (1/3 + (5λ/e)²)
	// λ=0.34 (f=1000Hz), e=1m:
	// ratio = 5·0.34/1 = 1.7
	// C₃ = (1 + 2.89) / (0.333 + 2.89) = 3.89/3.223 = 1.207
	lam1000 := speedOfSound / 1000.0
	got := c3Multiple(lam1000, 1.0)
	ratio := 5.0 * lam1000 / 1.0
	expected := (1.0 + ratio*ratio) / (1.0/3.0 + ratio*ratio)
	almostEqualBarrier(t, got, expected, 0.0001, "C₃(λ=0.34, e=1)")

	// e=0 → returns 1.0 (single barrier fallback)
	got0 := c3Multiple(lam1000, 0.0)
	almostEqualBarrier(t, got0, 1.0, 0.0001, "C₃(e=0) = 1")
}

// ---------------------------------------------------------------------------
// Gl. 20: D_refl — absorbing base correction
// ---------------------------------------------------------------------------

func TestDreflGl20(t *testing.T) {
	t.Parallel()

	// D_refl = max(3 - h_abs, 0)
	// h_abs=1m: D_refl = 2
	almostEqualBarrier(t, drefl(1.0), 2.0, 0.001, "drefl(1m)")

	// h_abs=3m: D_refl = 0
	almostEqualBarrier(t, drefl(3.0), 0.0, 0.001, "drefl(3m)")

	// h_abs=4m: clamped to 0
	almostEqualBarrier(t, drefl(4.0), 0.0, 0.001, "drefl(4m)")

	// h_abs=0: D_refl = 3
	almostEqualBarrier(t, drefl(0.0), 3.0, 0.001, "drefl(0m)")
}

// ---------------------------------------------------------------------------
// Gl. 18: A_bar (lateral diffraction) — single barrier
// ---------------------------------------------------------------------------

func TestAbarLateralSingleBarrier(t *testing.T) {
	t.Parallel()

	// A_bar = D_z ≥ 0 for lateral diffraction (Gl. 18)
	geom := BarrierGeometry{
		Ds:             5.0,
		Dr:             5.0,
		D:              9.0,
		Z:              1.0, // (5+5) - 9 = 1 m path difference
		E:              0,
		Habs:           0,
		IsDouble:       false,
		TopDiffraction: false,
	}

	var agrZero BeiblattSpectrum
	abar := ComputeAbar(geom, agrZero)

	// All bands should be > 0 (path difference z=1m > 0)
	for f := range NumBeiblattOctaveBands {
		if abar[f] <= 0 {
			t.Errorf("band %d: expected A_bar > 0, got %v", f, abar[f])
		}
	}

	// High-frequency band (8000 Hz) should show stronger attenuation than 63 Hz
	if abar[7] <= abar[0] {
		t.Errorf("expected higher A_bar at 8000 Hz than 63 Hz, got %v vs %v", abar[7], abar[0])
	}
}

// ---------------------------------------------------------------------------
// D_z capped at 20 dB (single) and 25 dB (double)
// ---------------------------------------------------------------------------

func TestDzCappedSingle(t *testing.T) {
	t.Parallel()

	// Large z → D_z approaches cap
	// Use very large z to ensure cap kicks in for high frequencies.
	geom := BarrierGeometry{
		Ds:             1000.0,
		Dr:             1000.0,
		D:              1.0,
		Z:              1999.0,
		IsDouble:       false,
		TopDiffraction: false,
	}

	var agrZero BeiblattSpectrum
	abar := ComputeAbar(geom, agrZero)

	for f := range NumBeiblattOctaveBands {
		if abar[f] > DzCapSingle+0.001 {
			t.Errorf("band %d: A_bar %.4f exceeds single-barrier cap %.4f", f, abar[f], DzCapSingle)
		}
	}
}

func TestDzCappedDouble(t *testing.T) {
	t.Parallel()

	// Large z with double barrier → cap is 25 dB
	geom := BarrierGeometry{
		Ds:             1000.0,
		Dr:             1000.0,
		D:              1.0,
		Z:              1999.0,
		E:              1.0,
		IsDouble:       true,
		TopDiffraction: false,
	}

	var agrZero BeiblattSpectrum
	abar := ComputeAbar(geom, agrZero)

	for f := range NumBeiblattOctaveBands {
		if abar[f] > DzCapDouble+0.001 {
			t.Errorf("band %d: A_bar %.4f exceeds double-barrier cap %.4f", f, abar[f], DzCapDouble)
		}
	}
}

// ---------------------------------------------------------------------------
// C₂ parameterization: Rangierbahnhof vs Strecke
// ---------------------------------------------------------------------------

func TestBarrierDzC2Yard(t *testing.T) {
	t.Parallel()
	// C₂=20 for Rangierbahnhof must give lower D_z than C₂=40 for same geometry.
	// z=0.5m, λ=0.34m (1000 Hz), C₃=1, K_met=1
	// C₂=40: D_z = 10·lg(3 + 40/0.34·1·0.5·1) = 10·lg(3+58.82) = 10·lg(61.82) ≈ 17.91
	// C₂=20: D_z = 10·lg(3 + 20/0.34·1·0.5·1) = 10·lg(3+29.41) = 10·lg(32.41) ≈ 15.11
	dzStrecke := barrierDzWithC2(c2Strecke, 0.34, 1.0, 0.5, 1.0)

	dzYard := barrierDzWithC2(c2Rangierbahnhof, 0.34, 1.0, 0.5, 1.0)

	if dzYard >= dzStrecke {
		t.Errorf("C₂=20 should give lower D_z than C₂=40: got %g vs %g", dzYard, dzStrecke)
	}

	if math.Abs(dzYard-15.11) > 0.1 {
		t.Errorf("C₂=20 D_z: expected ~15.11, got %g", dzYard)
	}
}

// ---------------------------------------------------------------------------
// Gl. 19: A_bar (top diffraction)
// ---------------------------------------------------------------------------

func TestAbarTopDiffraction(t *testing.T) {
	t.Parallel()

	// Gl. 19: A_bar = D_z - D_refl - A_gr ≥ 0
	// With D_refl correction and A_gr, result can be less than D_z.
	geom := BarrierGeometry{
		Ds:             5.0,
		Dr:             5.0,
		D:              9.0,
		Z:              1.0,
		Habs:           1.0, // D_refl = 2 dB
		IsDouble:       false,
		TopDiffraction: true,
	}

	// Provide A_gr band values that will reduce A_bar.
	var agr BeiblattSpectrum
	for f := range NumBeiblattOctaveBands {
		agr[f] = 1.0 // 1 dB ground attenuation per band
	}

	abarTop := ComputeAbar(geom, agr)

	// A_bar should be clamped to ≥ 0.
	for f := range NumBeiblattOctaveBands {
		if abarTop[f] < 0 {
			t.Errorf("band %d: A_bar %.4f < 0 (must be clamped)", f, abarTop[f])
		}
	}

	// Compare to lateral path: top diffraction with reductions should give
	// equal or smaller A_bar.
	geomLateral := geom
	geomLateral.TopDiffraction = false
	abarLateral := ComputeAbar(geomLateral, agr)

	for f := range NumBeiblattOctaveBands {
		if abarTop[f] > abarLateral[f]+0.001 {
			t.Errorf("band %d: top A_bar %.4f > lateral A_bar %.4f (unexpected)", f, abarTop[f], abarLateral[f])
		}
	}
}
