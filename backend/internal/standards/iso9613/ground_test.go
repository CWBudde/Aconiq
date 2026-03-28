package iso9613

import (
	"math"
	"testing"
)

func TestGroundEffectFunctionsFinite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fn   func(float64, float64) float64
	}{
		{"aPrime", aPrime},
		{"bPrime", bPrime},
		{"cPrime", cPrime},
		{"dPrime", dPrime},
	}

	for _, tc := range tests {
		got := tc.fn(5, 200)

		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Errorf("%s(5, 200) returned %v", tc.name, got)
		}

		if got < 0 {
			t.Errorf("%s(5, 200) = %v, expected >= 0", tc.name, got)
		}
	}
}

func TestGroundEffectHardGround(t *testing.T) {
	t.Parallel()
	// Hard ground: G=0
	// For high frequency bands (2k, 4k, 8k):
	// A_s = A_r = -1.5*(1-0) = -1.5 each, so A_s + A_r = -3.0
	hs, hr := 5.0, 4.0
	dp := 200.0
	result := GroundEffectBands(0, 0, 0, hs, hr, dp)

	// 63 Hz: A_s = -1.5, A_r = -1.5, A_m = 3*q (q=0 since dp=200 < 30*9=270)
	// Total = -3.0
	if math.Abs(result[0]-(-3.0)) > 0.01 {
		t.Errorf("63 Hz hard ground: expected -3.0, got %v", result[0])
	}

	// 8 kHz: A_s = -1.5*(1-0) = -1.5, A_r = -1.5, A_m = -3*q*(1-0) = 0
	// Total = -3.0
	if math.Abs(result[7]-(-3.0)) > 0.01 {
		t.Errorf("8 kHz hard ground: expected -3.0, got %v", result[7])
	}
}

func TestGroundEffectPorousGround(t *testing.T) {
	t.Parallel()
	// Porous ground: G=1
	hs, hr := 5.0, 4.0
	dp := 200.0
	result := GroundEffectBands(1, 1, 1, hs, hr, dp)

	// High frequency bands (2k, 4k, 8k): A_s = A_r = -1.5*(1-1) = 0
	for _, band := range []int{5, 6, 7} {
		if math.Abs(result[band]) > 0.01 {
			t.Errorf("band %d porous ground: expected ~0, got %v", band, result[band])
		}
	}
}

func TestGroundEffectSimplified(t *testing.T) {
	t.Parallel()
	// Eq. 10: A_gr = 4.8 - (2*hm/d)*[17 + 300/d] >= 0
	// hm=3, d=200: 4.8 - (6/200)*(17+1.5) = 4.8 - 0.555 = 4.245

	got := GroundEffectSimplified(3, 200)
	expected := 4.8 - (2*3.0/200.0)*(17+300.0/200.0)

	if math.Abs(got-expected) > 0.001 {
		t.Errorf("simplified: expected %.3f, got %.3f", expected, got)
	}
}

func TestGroundEffectSimplifiedClampedToZero(t *testing.T) {
	t.Parallel()

	got := GroundEffectSimplified(100, 50)
	if got != 0 {
		t.Errorf("expected 0 (clamped), got %v", got)
	}
}

func TestMiddleRegionQ(t *testing.T) {
	t.Parallel()

	// q = 0 when dp <= 30*(hs+hr)
	q := middleRegionQ(5, 4, 200)
	if q != 0 {
		t.Errorf("expected q=0 for dp=200 <= 30*9=270, got %v", q)
	}

	// q > 0 when dp > 30*(hs+hr)
	q = middleRegionQ(2, 2, 200)

	expected := 1 - 30.0*4.0/200.0 // 1 - 0.6 = 0.4
	if math.Abs(q-expected) > 0.001 {
		t.Errorf("expected q=%.3f for dp=200 > 30*4=120, got %v", expected, q)
	}
}

func TestGroundEffectSimplifiedZeroDistance(t *testing.T) {
	t.Parallel()

	got := GroundEffectSimplified(3, 0)
	if got != 0 {
		t.Errorf("expected 0 for zero distance, got %v", got)
	}
}
