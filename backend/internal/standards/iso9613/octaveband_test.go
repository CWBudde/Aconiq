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

func TestNoteOneBandIndexIs500Hz(t *testing.T) {
	t.Parallel()

	// ISO 9613-2:1996 NOTE 1 uses the 500 Hz attenuation terms when only an
	// A-weighted sound power level is known.
	if OctaveBandFrequencies[NoteOneBandIndex] != 500 {
		t.Fatalf("expected NoteOneBandIndex to select 500 Hz, got %v Hz", OctaveBandFrequencies[NoteOneBandIndex])
	}
}

// TestAWeightingEnergySumMagnitude pins the size of the double-A-weighting
// error that replicating L_WA into all eight bands used to introduce.
func TestAWeightingEnergySumMagnitude(t *testing.T) {
	t.Parallel()

	sum := 0.0
	for _, a := range AWeighting {
		sum += math.Pow(10, 0.1*a)
	}

	got := 10 * math.Log10(sum)
	if math.Abs(got-6.99) > 0.01 {
		t.Fatalf("expected the A-weighting energy sum to be +6.99 dB, got %.4f dB", got)
	}
}
