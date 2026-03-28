package iso9613

import (
	"math"
	"testing"
)

func TestOctaveBandCount(t *testing.T) {
	t.Parallel()
	if NumBands != 8 {
		t.Fatalf("expected 8 bands, got %d", NumBands)
	}
}

func TestOctaveBandFrequencies(t *testing.T) {
	t.Parallel()
	expected := [NumBands]float64{63, 125, 250, 500, 1000, 2000, 4000, 8000}
	if OctaveBandFrequencies != expected {
		t.Fatalf("unexpected frequencies: %v", OctaveBandFrequencies)
	}
}

func TestAWeightingCorrections(t *testing.T) {
	t.Parallel()
	if AWeighting[4] != 0.0 {
		t.Fatalf("expected 0.0 at 1 kHz, got %v", AWeighting[4])
	}
}

func TestWavelength(t *testing.T) {
	t.Parallel()
	got := Wavelength(1000)
	if math.Abs(got-0.34) > 1e-9 {
		t.Fatalf("expected 0.34, got %v", got)
	}
}

func TestBandLevelsFromSingleValue(t *testing.T) {
	t.Parallel()
	levels := BandLevelsFromAWeighted(100.0)
	for i, v := range levels {
		if v != 100.0 {
			t.Fatalf("band %d: expected 100.0, got %v", i, v)
		}
	}
}
