package iso9613

import (
	"math"
	"testing"
)

// table2Tolerance is how far the computed ISO 9613-1 coefficient may sit from
// the ISO 9613-2 Table 2 tabulation of it, per octave band, in dB/km.
//
// The residual is the table's own precision, not an error in the formula.
// Table 2 prints one decimal, so a tabulated value carries ±0.05 dB/km of
// rounding by construction; up to 1 kHz the whole disagreement fits inside
// that. Above 1 kHz the tabulated values are large (up to 202 dB/km) and the
// disagreement grows in absolute terms while staying at or below ~1.4 %
// relative, which is the precision of the tabulation itself — the printed rows
// were rounded from a computation with its own input rounding. The worst case
// is 1.38 dB/km at 10 °C / 70 % RH, 8 kHz: 0.7 dB over 500 m, against the
// ±1 to ±3 dB accuracy ISO 9613-2, clause 9 claims for the method.
//
// Do not loosen an entry to make a band pass. Every entry below is the measured
// worst case over the six rows plus a small margin; a band that exceeds it means
// the formula has been mistranscribed.
var table2Tolerance = [NumBands]float64{0.05, 0.05, 0.05, 0.05, 0.06, 0.15, 0.70, 1.50}

// TestAlphaMatchesTable2 is the regression test for the analytical coefficient:
// ISO 9613-2 Table 2 is the oracle, all 48 tabulated values of it.
//
// Table 2 is no longer the compute path — ISO 9613-2, 7.2 tabulates six
// reference conditions and refers to ISO 9613-1 for the coefficient, so the
// formula is the normative source. Reproducing the table from the formula is
// what establishes that the implemented formula is the one the table came from.
func TestAlphaMatchesTable2(t *testing.T) {
	t.Parallel()

	for _, row := range table2 {
		for band := range NumBands {
			got := LookupAlpha(row.TempC, row.Humidity, band)

			diff := math.Abs(got - row.Alpha[band])
			if diff > table2Tolerance[band] {
				t.Errorf("T=%.0f°C RH=%.0f%% %.0f Hz: α = %.4f dB/km, Table 2 has %.1f (Δ %.4f > tolerance %.2f)",
					row.TempC, row.Humidity, OctaveBandFrequencies[band], got, row.Alpha[band], diff, table2Tolerance[band])
			}
		}
	}
}

// TestAlphaAnchors pins three conditions the formula must reproduce exactly, so
// that a transcription error which happens to stay inside the Table 2
// tolerances above still fails. The values are the ISO 9613-1 model evaluated
// at the reference pressure; the tabulated values are given for contrast.
func TestAlphaAnchors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tempC    float64
		humidity float64
		band     int
		expected float64 // dB/km
	}{
		{"20C/70% 4 kHz (Table 2: 22.9)", 20, 70, 6, 23.0858},
		{"15C/20% 8 kHz (Table 2: 202.0)", 15, 20, 7, 203.0283},
		{"30C/70% 63 Hz (Table 2: 0.1)", 30, 70, 0, 0.0652},
		{"10C/70% 63 Hz, the default condition (Table 2: 0.1)", 10, 70, 0, 0.1213},
	}

	for _, tc := range tests {
		got := LookupAlpha(tc.tempC, tc.humidity, tc.band)
		if math.Abs(got-tc.expected) > 0.0001 {
			t.Errorf("%s: expected %.4f dB/km, got %.4f", tc.name, tc.expected, got)
		}
	}
}

// TestAlphaVariesContinuously covers what nearest-row selection could not: a
// condition between two tabulated rows has to move the coefficient, and a
// condition far from every row has to be evaluated rather than snapped to the
// closest row. 5 °C / 30 % RH at 4 kHz is the case that motivated the change —
// nearest-row selection returned the 10 °C / 70 % row's 32.8 dB/km there, which
// is 25 dB low over 500 m.
func TestAlphaVariesContinuously(t *testing.T) {
	t.Parallel()

	const band4kHz = 6

	cold := LookupAlpha(5, 30, band4kHz)
	if math.Abs(cold-83.0314) > 0.001 {
		t.Errorf("5°C/30%% 4 kHz: expected 83.0314 dB/km, got %.4f", cold)
	}

	// Strictly monotone in humidity at 4 kHz and 20 °C: over this span the
	// oxygen relaxation frequency has already moved past 4 kHz, so more water
	// vapour means less absorption. Nearest-row selection returned one of three
	// values across the whole span.
	previous := math.Inf(1)

	for humidity := 20.0; humidity <= 40.0; humidity += 5 {
		alpha := LookupAlpha(20, humidity, band4kHz)
		if alpha >= previous {
			t.Errorf("α at 20°C, RH=%.0f%% is %.4f, not below the %.4f of the previous step", humidity, alpha, previous)
		}

		previous = alpha
	}
}

func TestAtmosphericAbsorptionTable2Row1(t *testing.T) {
	t.Parallel()
	// Table 2 row 1: 10 °C, 70 % RH. The expected values are A_atm = α·d/1000
	// with the computed α; the tolerance is the Table 2 tolerance for the band
	// scaled by the distance, so this asserts Eq. 8 against the tabulated row
	// rather than against the implementation.
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

		tolerance := AtmosphericAbsorption(table2Tolerance[tc.band], tc.distance) + 0.001
		if math.Abs(got-tc.expected) > tolerance {
			t.Errorf("band %d, d=%.0fm: expected %.2f ± %.3f, got %.2f", tc.band, tc.distance, tc.expected, tolerance, got)
		}
	}
}

func TestAtmosphericAbsorptionTable2AllRows(t *testing.T) {
	t.Parallel()
	// Verify the 500 Hz column (band index 3) for each table row, within the
	// band's Table 2 tolerance.
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
		if math.Abs(alpha-tc.alpha500) > table2Tolerance[3] {
			t.Errorf("T=%.0f RH=%.0f: expected α₅₀₀=%.1f ± %.2f, got %.4f", tc.tempC, tc.humidity, tc.alpha500, table2Tolerance[3], alpha)
		}
	}
}

func TestAtmosphericAbsorptionBandLevels(t *testing.T) {
	t.Parallel()
	// Full octave-band A_atm at 10 °C, 70 % RH, 200 m: α·0.2, with α from the
	// ISO 9613-1 model.
	got := AtmosphericAbsorptionBands(10, 70, 200)
	expected := [NumBands]float64{0.0243, 0.0813, 0.2076, 0.3848, 0.7315, 1.9403, 6.6117, 23.6763}

	for i := range got {
		if math.Abs(got[i]-expected[i]) > 0.001 {
			t.Errorf("band %d: expected %.4f, got %.4f", i, expected[i], got[i])
		}
	}
}

// TestAirTemperatureIsBounded covers the range the parameter schema and the
// propagation config now share. Unbounded, -273 °C was an accepted input and
// the ISO 9613-1 coefficient then divided by an absolute temperature at or
// below zero, so the run emitted infinite attenuation instead of refusing.
func TestAirTemperatureIsBounded(t *testing.T) {
	t.Parallel()

	for _, tempC := range []float64{-273.15, minAirTemperatureC - 0.1, maxAirTemperatureC + 0.1} {
		cfg := DefaultPropagationConfig()
		cfg.AirTemperatureC = tempC

		if cfg.Validate() == nil {
			t.Errorf("air_temperature_c = %g °C was accepted", tempC)
		}

		met := Meteorology{Assumption: MeteorologyDownwind, TemperatureC: tempC, RelativeHumidityPercent: 70}
		if met.Validate() == nil {
			t.Errorf("meteorology temperature_c = %g °C was accepted", tempC)
		}
	}

	for _, tempC := range []float64{minAirTemperatureC, 10, maxAirTemperatureC} {
		cfg := DefaultPropagationConfig()
		cfg.AirTemperatureC = tempC

		if err := cfg.Validate(); err != nil {
			t.Errorf("air_temperature_c = %g °C was refused: %v", tempC, err)
		}
	}

	// The published schema carries the same bounds, so a caller that validates
	// against the descriptor refuses what the config would refuse.
	for _, definition := range parameterDefinitions() {
		if definition.Name != "air_temperature_c" {
			continue
		}

		if definition.Min == nil || definition.Max == nil {
			t.Fatal("air_temperature_c publishes no Min/Max")
		}

		if *definition.Min != minAirTemperatureC || *definition.Max != maxAirTemperatureC {
			t.Errorf("air_temperature_c schema bounds are [%g,%g], want [%g,%g]",
				*definition.Min, *definition.Max, minAirTemperatureC, maxAirTemperatureC)
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
