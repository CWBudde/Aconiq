package schall03

import (
	"math"
	"testing"
)

// almostEqualProp is a local helper for propagation tests.
func almostEqualProp(t *testing.T, got, want, tol float64, label string) {
	t.Helper()

	if math.Abs(got-want) > tol {
		t.Errorf("%s: expected %.4f, got %.4f (tol %.4f)", label, want, got, tol)
	}
}

// ---------------------------------------------------------------------------
// Gl. 11: A_div — geometric divergence
// ---------------------------------------------------------------------------

func TestAdivGl11(t *testing.T) {
	t.Parallel()

	// A_div = 10·lg(4π·d²/d₀²) with d₀=1m
	// d=100m: A_div = 10·lg(4π·10000) = 10·lg(125663.7) = 50.99 dB
	got := adiv(100.0)
	almostEqualProp(t, got, 50.99, 0.01, "adiv(100m)")

	// d=25m: A_div = 10·lg(4π·625) = 10·lg(7853.98) = 38.95 dB
	got25 := adiv(25.0)
	almostEqualProp(t, got25, 38.95, 0.01, "adiv(25m)")

	// d=1m: A_div = 10·lg(4π) = 10.99 dB
	got1 := adiv(1.0)
	almostEqualProp(t, got1, 10.99, 0.01, "adiv(1m)")
}

// ---------------------------------------------------------------------------
// Gl. 12: A_atm — air absorption
// ---------------------------------------------------------------------------

func TestAatmGl12(t *testing.T) {
	t.Parallel()

	// A_atm = α·d/1000
	// 1000 Hz band (α=3.7), d=500m: A_atm = 3.7·500/1000 = 1.85 dB
	got := aatm(AirAbsorptionAlpha[4], 500.0)
	almostEqualProp(t, got, 1.85, 0.001, "aatm(1000Hz, 500m)")

	// 63 Hz band (α=0.1), d=1000m: A_atm = 0.1·1000/1000 = 0.1 dB
	got63 := aatm(AirAbsorptionAlpha[0], 1000.0)
	almostEqualProp(t, got63, 0.1, 0.001, "aatm(63Hz, 1000m)")

	// 8000 Hz band (α=117.0), d=200m: A_atm = 117·200/1000 = 23.4 dB
	got8k := aatm(AirAbsorptionAlpha[7], 200.0)
	almostEqualProp(t, got8k, 23.4, 0.001, "aatm(8kHz, 200m)")
}

// ---------------------------------------------------------------------------
// Gl. 14: A_gr,B — ground absorption over land
// ---------------------------------------------------------------------------

func TestAgrBGl14(t *testing.T) {
	t.Parallel()

	// A_gr,B = [4.8 - (2·h_m/d)·(17 + 300·d_p/d)] ≥ 0
	// h_m=0.1, d=1000, d_p=1000:
	// A_gr,B = 4.8 - (0.2/1000)·(17+300) = 4.8 - 0.0634 = 4.737
	got := agrB(0.1, 1000.0, 1000.0)
	almostEqualProp(t, got, 4.737, 0.01, "agrB(h=0.1, d=1000, dp=1000)")

	// Large h_m — result clamped to 0.
	// h_m=5, d=200, d_p=200:
	// 4.8 - (10/200)·(17+300) = 4.8 - 15.85 = -11.05 → clamped to 0
	got2 := agrB(5.0, 200.0, 200.0)
	almostEqualProp(t, got2, 0.0, 0.001, "agrB clamped to zero")

	// Near-grazing with low d_p:
	// h_m=1, d=50, d_p=50:
	// 4.8 - (2/50)·(17+300) = 4.8 - 12.68 = -7.88 → clamped to 0
	got3 := agrB(1.0, 50.0, 50.0)
	almostEqualProp(t, got3, 0.0, 0.001, "agrB low d_p clamped")
}

// ---------------------------------------------------------------------------
// Gl. 16: A_gr,W — water body ground correction
// ---------------------------------------------------------------------------

func TestAgrWGl16(t *testing.T) {
	t.Parallel()

	// A_gr,W = -3·d_w/d_p
	// d_w=100m, d_p=200m: A_gr,W = -3·0.5 = -1.5 dB
	got := agrW(100.0, 200.0)
	almostEqualProp(t, got, -1.5, 0.001, "agrW(dw=100, dp=200)")

	// d_w = d_p (all water): A_gr,W = -3 dB
	got2 := agrW(300.0, 300.0)
	almostEqualProp(t, got2, -3.0, 0.001, "agrW all water")

	// d_p=0 (degenerate): returns 0
	got3 := agrW(0.0, 0.0)
	almostEqualProp(t, got3, 0.0, 0.001, "agrW degenerate dp=0")
}

// ---------------------------------------------------------------------------
// Gl. 8: D_I — directivity correction
// ---------------------------------------------------------------------------

func TestDirectivityDIGl8(t *testing.T) {
	t.Parallel()

	// D_I = 10·lg(0.22 + 1.27·sin²(δ))
	// δ=90° (perpendicular): D_I = 10·lg(0.22+1.27) = 10·lg(1.49) = 1.73 dB
	got90 := directivityDI(math.Pi / 2)
	almostEqualProp(t, got90, 1.73, 0.01, "D_I at 90°")

	// δ=0° (along track): D_I = 10·lg(0.22) = -6.58 dB
	got0 := directivityDI(0)
	almostEqualProp(t, got0, -6.58, 0.01, "D_I at 0°")

	// δ=30°: sin²(30°)=0.25 → D_I = 10·lg(0.22 + 1.27·0.25) = 10·lg(0.5375) = -2.70 dB
	got30 := directivityDI(math.Pi / 6)
	almostEqualProp(t, got30, -2.70, 0.01, "D_I at 30°")
}

// ---------------------------------------------------------------------------
// Gl. 9: D_Ω — solid angle correction
// ---------------------------------------------------------------------------

func TestSolidAngleDOmegaGl9(t *testing.T) {
	t.Parallel()

	// D_Ω = 10·lg(1 + (d_p² + (h_g-h_r)²) / (d_p² + (h_g+h_r)²))
	// h_g=0 (SO), h_r=4, d_p=25:
	// (625 + 16) / (625 + 16) = 1 → D_Ω = 10·lg(2) = 3.01 dB
	got := solidAngleDOmega(25.0, 0.0, 4.0)
	almostEqualProp(t, got, 3.01, 0.01, "D_Ω h_g=0, h_r=4, dp=25")

	// h_g = h_r = 0, d_p > 0:
	// (d_p² + 0) / (d_p² + 0) = 1 → D_Ω = 10·lg(2) = 3.01 dB
	got2 := solidAngleDOmega(50.0, 0.0, 0.0)
	almostEqualProp(t, got2, 3.01, 0.01, "D_Ω both at zero height")

	// Large h_r → ratio approaches 1 → D_Ω approaches 3.01 dB
	// h_g=0, h_r=1000, d_p=0.01:
	// num = 0.0001 + 1e6, den = 0.0001 + 1e6 → ratio → 1 → 3.01 dB
	got3 := solidAngleDOmega(0.01, 0.0, 1000.0)
	almostEqualProp(t, got3, 3.01, 0.01, "D_Ω large h_r")
}
