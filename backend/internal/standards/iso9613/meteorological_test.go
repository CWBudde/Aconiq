package iso9613

import (
	"math"
	"testing"
)

func TestMeteorologicalCorrectionShortDistance(t *testing.T) {
	t.Parallel()

	// C_met = 0 when dp <= 10*(hs+hr)
	// hs=5, hr=4, dp=80 <= 10*9=90
	got := MeteorologicalCorrection(3.0, 5, 4, 80)
	if got != 0 {
		t.Errorf("expected 0 for short distance, got %v", got)
	}
}

func TestMeteorologicalCorrectionLongDistance(t *testing.T) {
	t.Parallel()

	// C_met = C0 * [1 - 10*(hs+hr)/dp]
	// C0=3, hs=5, hr=4, dp=200: C_met = 3 * [1 - 90/200] = 3 * 0.55 = 1.65
	got := MeteorologicalCorrection(3.0, 5, 4, 200)
	if math.Abs(got-1.65) > 0.01 {
		t.Errorf("expected 1.65, got %v", got)
	}
}

func TestMeteorologicalCorrectionZeroC0(t *testing.T) {
	t.Parallel()

	// C0=0 (pure downwind): C_met always 0
	got := MeteorologicalCorrection(0, 5, 4, 500)
	if got != 0 {
		t.Errorf("expected 0 for C0=0, got %v", got)
	}
}

func TestMeteorologicalCorrectionAtBoundary(t *testing.T) {
	t.Parallel()

	// Exactly at boundary: dp = 10*(hs+hr) = 90
	got := MeteorologicalCorrection(3.0, 5, 4, 90)
	if got != 0 {
		t.Errorf("expected 0 at boundary, got %v", got)
	}
}
