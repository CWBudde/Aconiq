package iso9613

import (
	"math"
	"testing"
)

func TestAtmosphericAbsorptionTable2Row1(t *testing.T) {
	t.Parallel()
	// Table 2 row 1: 10°C, 70% RH
	// α values: 0.1, 0.4, 1.0, 1.9, 3.7, 9.7, 32.8, 117.0 dB/km
	tests := []struct {
		band     int
		distance float64
		expected float64
	}{
		{0, 1000, 0.1}, // 63 Hz, 1 km
		{3, 100, 0.19}, // 500 Hz, 100 m
		{4, 200, 0.74}, // 1 kHz, 200 m
		{7, 100, 11.7}, // 8 kHz, 100 m
	}

	for _, tc := range tests {
		alpha := LookupAlpha(10, 70, tc.band)
		got := AtmosphericAbsorption(alpha, tc.distance)
		if math.Abs(got-tc.expected) > 0.01 {
			t.Errorf("band %d, d=%.0fm: expected %.2f, got %.2f", tc.band, tc.distance, tc.expected, got)
		}
	}
}

func TestAtmosphericAbsorptionTable2AllRows(t *testing.T) {
	t.Parallel()
	// Verify the 500 Hz column (band index 3) for each table row
	tests := []struct {
		tempC    float64
		humidity float64
		alpha500 float64
	}{
		{10, 70, 1.9},
		{20, 70, 2.8},
		{30, 70, 3.1},
		{15, 20, 2.7},
		{15, 50, 2.2},
		{15, 80, 2.4},
	}

	for _, tc := range tests {
		alpha := LookupAlpha(tc.tempC, tc.humidity, 3)
		if math.Abs(alpha-tc.alpha500) > 0.05 {
			t.Errorf("T=%.0f RH=%.0f: expected α₅₀₀=%.1f, got %.1f", tc.tempC, tc.humidity, tc.alpha500, alpha)
		}
	}
}

func TestAtmosphericAbsorptionBandLevels(t *testing.T) {
	t.Parallel()
	// Full octave-band A_atm at 10°C, 70% RH, 200 m
	got := AtmosphericAbsorptionBands(10, 70, 200)
	// Expected: α * 200 / 1000 = α * 0.2
	expected := [NumBands]float64{0.02, 0.08, 0.20, 0.38, 0.74, 1.94, 6.56, 23.4}
	for i := range got {
		if math.Abs(got[i]-expected[i]) > 0.01 {
			t.Errorf("band %d: expected %.2f, got %.2f", i, expected[i], got[i])
		}
	}
}

func TestLookupAlphaInvalidBand(t *testing.T) {
	t.Parallel()
	if LookupAlpha(10, 70, -1) != 0 {
		t.Error("expected 0 for invalid band -1")
	}
	if LookupAlpha(10, 70, 8) != 0 {
		t.Error("expected 0 for invalid band 8")
	}
}
