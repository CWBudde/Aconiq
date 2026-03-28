package iso9613

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// Validation tests verify the full ISO 9613-2 octave-band chain against
// hand-calculated reference values derived from the standard's formulas.
//
// Reference scenario (used by most tests):
//   - Source: point at origin, height 10 m, omnidirectional (D_c = 0),
//     L_W = 100 dB per octave band (all 8 bands equal)
//   - Receiver: height 4 m, at (200, 0)
//   - Ground: porous (G = 1.0) for all regions
//   - Temperature: 10 C, humidity: 70%
//   - No barrier, C_0 = 0

func refSource() PointSource {
	bands := BandLevels{100, 100, 100, 100, 100, 100, 100, 100}

	return PointSource{
		ID:                "src",
		Point:             geo.Point2D{X: 0, Y: 0},
		SourceHeightM:     10,
		SoundPowerLevelDB: 100,
		OctaveBandLevels:  &bands,
	}
}

func refReceiver() geo.PointReceiver {
	return geo.PointReceiver{
		ID:      "rcv",
		Point:   geo.Point2D{X: 200, Y: 0},
		HeightM: 4,
	}
}

func refConfig() PropagationConfig {
	return PropagationConfig{
		GroundFactor:            1.0,
		AirTemperatureC:         10,
		RelativeHumidityPercent: 70,
		MeteorologyAssumption:   MeteorologyDownwind,
		Barrier:                 nil,
		C0:                      0,
		MinDistanceM:            1,
	}
}

func TestValidationSingleSourceFullChain(t *testing.T) {
	t.Parallel()

	source := refSource()
	receiver := refReceiver()
	cfg := refConfig()

	// Hand-calculated geometry:
	// d = sqrt(200^2 + (10-4)^2) = sqrt(40036) ~ 200.09 m
	// A_div = 20*lg(200.09) + 11 ~ 57.025 dB
	expectedD := math.Sqrt(200*200 + 6*6)
	expectedAdiv := 20*math.Log10(expectedD) + 11

	// Verify BandAttenuation returns consistent geometry distance.
	atten, dist := BandAttenuation(receiver, source, cfg)
	if math.Abs(dist-expectedD) > 0.001 {
		t.Errorf("distance: expected %.3f, got %.3f", expectedD, dist)
	}

	// A_div is frequency-independent; check it through the total attenuation.
	// Since A_div is a common offset, the difference between band attenuations
	// should equal (A_atm[i] + A_gr[i]) - (A_atm[j] + A_gr[j]).
	// Instead, directly verify A_div via the exported function.
	gotAdiv := geometricDivergence(dist)
	if math.Abs(gotAdiv-expectedAdiv) > 0.001 {
		t.Errorf("A_div: expected %.3f, got %.3f", expectedAdiv, gotAdiv)
	}

	// Verify the overall receiver level is in a reasonable range.
	// 100 dB source at 200 m: expect somewhere in 40-80 dB A-weighted.
	indicators, err := ComputeReceiverIndicators(receiver, []PointSource{source}, cfg)
	if err != nil {
		t.Fatalf("ComputeReceiverIndicators: %v", err)
	}

	if indicators.LpAeqDW < 40 || indicators.LpAeqDW > 80 {
		t.Errorf("L_AT(DW) out of plausible range [40,80]: got %.2f dB", indicators.LpAeqDW)
	}

	// With C0 = 0, LT == DW.
	if indicators.LpAeqLT != indicators.LpAeqDW {
		t.Errorf("with C0=0, expected LT == DW, got LT=%.2f DW=%.2f", indicators.LpAeqLT, indicators.LpAeqDW)
	}

	// The 8 kHz band must have much higher atmospheric absorption than 63 Hz.
	// Table 2 row 1 alpha: 63 Hz = 0.1, 8 kHz = 117.0 dB/km.
	// At 200.09 m: A_atm(63) ~ 0.020, A_atm(8k) ~ 23.4.
	if atten[7]-atten[0] < 15 {
		t.Errorf("expected 8 kHz attenuation much higher than 63 Hz: A[7]=%.2f, A[0]=%.2f",
			atten[7], atten[0])
	}
}

func TestValidationAtmosphericAbsorptionAgainstTable2(t *testing.T) {
	t.Parallel()

	// Full octave-band verification at 10 C / 70% RH for d = 200.09 m.
	// alpha (dB/km) from Table 2 row 1: {0.1, 0.4, 1.0, 1.9, 3.7, 9.7, 32.8, 117.0}
	// A_atm = alpha * d / 1000

	d := math.Sqrt(200*200 + 6*6) // 200.09 m

	alphaTable2 := [NumBands]float64{0.1, 0.4, 1.0, 1.9, 3.7, 9.7, 32.8, 117.0}
	got := AtmosphericAbsorptionBands(10, 70, d)

	for i := range NumBands {
		expected := alphaTable2[i] * d / 1000.0
		if math.Abs(got[i]-expected) > 0.005 {
			t.Errorf("band %d (%g Hz): expected A_atm=%.4f, got %.4f (alpha=%.1f dB/km)",
				i, OctaveBandFrequencies[i], expected, got[i], alphaTable2[i])
		}
	}

	// Cross-check: verify that higher frequency bands always have more absorption.
	for i := 1; i < NumBands; i++ {
		if got[i] < got[i-1] {
			t.Errorf("A_atm not monotonically increasing: band %d (%.4f) < band %d (%.4f)",
				i, got[i], i-1, got[i-1])
		}
	}
}

func TestValidationGroundEffectHardVsPorous(t *testing.T) {
	t.Parallel()

	source := refSource()
	receiver := refReceiver()

	// Porous ground: G = 1.0
	porousCfg := refConfig()
	porousCfg.GroundFactor = 1.0

	// Hard ground: G = 0.0
	hardCfg := refConfig()
	hardCfg.GroundFactor = 0.0

	porousLevel, err := ComputeReceiverLevel(receiver, []PointSource{source}, porousCfg)
	if err != nil {
		t.Fatalf("porous: %v", err)
	}

	hardLevel, err := ComputeReceiverLevel(receiver, []PointSource{source}, hardCfg)
	if err != nil {
		t.Fatalf("hard: %v", err)
	}

	// Hard ground (G=0) produces A_s = A_r = -1.5 for all bands (Table 3),
	// giving a total ground attenuation of -3 dB for all bands.
	// Porous ground (G=1) at mid frequencies gets different (potentially higher)
	// A_gr values from the a'/b'/c'/d' functions.
	// The overall A-weighted levels should differ.
	if porousLevel == hardLevel {
		t.Fatal("expected different levels for hard vs porous ground")
	}

	// Verify the raw ground-effect bands directly.
	dp := 200.0
	hs := 10.0
	hr := 4.0

	hardGr := GroundEffectBands(0, 0, 0, hs, hr, dp)
	porousGr := GroundEffectBands(1, 1, 1, hs, hr, dp)

	// For 63 Hz (band 0): both G=0 and G=1 give A_s = A_r = -1.5.
	// Hard: A_m = -3*q*(1-0) but q = 0 here (dp=200, 30*(10+4)=420 > 200).
	// So A_gr(63) = -3.0 for both.
	if math.Abs(hardGr[0]-porousGr[0]) > 0.001 {
		t.Errorf("63 Hz: expected same A_gr for hard and porous, got hard=%.3f porous=%.3f",
			hardGr[0], porousGr[0])
	}

	// For 2 kHz (band 5): hard gives A_s = A_r = -1.5*(1-0) = -1.5 each = -3.0.
	// Porous gives A_s = A_r = -1.5*(1-1) = 0 each = 0.0.
	// So hard ground gives more attenuation (more negative) at high frequencies.
	expectedHard2k := -3.0
	if math.Abs(hardGr[5]-expectedHard2k) > 0.001 {
		t.Errorf("2 kHz hard: expected A_gr=%.1f, got %.3f", expectedHard2k, hardGr[5])
	}

	expectedPorous2k := 0.0
	if math.Abs(porousGr[5]-expectedPorous2k) > 0.001 {
		t.Errorf("2 kHz porous: expected A_gr=%.1f, got %.3f", expectedPorous2k, porousGr[5])
	}

	// At low-mid frequencies (125 Hz, band 1) with G=1, the a'() terms push A_gr
	// above -3.0, so porous ground gives less attenuation than hard ground.
	if porousGr[1] <= hardGr[1] {
		t.Errorf("125 Hz: expected porous A_gr > hard A_gr, got porous=%.3f hard=%.3f",
			porousGr[1], hardGr[1])
	}
}

func TestValidationBarrierIncreasesAttenuation(t *testing.T) {
	t.Parallel()

	source := refSource()
	receiver := refReceiver()
	cfg := refConfig()

	// Create a barrier that produces a positive path difference (z > 0).
	// The barrier sits midway between source and receiver.
	// Dss + Dsr + E > D means the diffracted path is longer than direct path.
	barrier := &BarrierGeometry{
		Dss: 101, // source to barrier top
		Dsr: 101, // barrier top to receiver
		E:   0,   // single diffraction edge
		A:   0,   // no lateral component
		D:   200, // direct distance
	}

	z := pathDifference(*barrier)
	if z <= 0 {
		t.Fatalf("expected positive path difference, got z=%.4f", z)
	}

	// D_z should increase with frequency (shorter wavelength = more diffraction loss).
	var prevDz float64

	for i, freq := range OctaveBandFrequencies {
		dz := BarrierDz(*barrier, z, freq, 20)
		if dz <= 0 {
			t.Errorf("band %d (%g Hz): expected positive D_z, got %.3f", i, freq, dz)
		}

		if i > 0 && dz < prevDz {
			t.Errorf("D_z not increasing with frequency: band %d (%.3f) < band %d (%.3f)",
				i, dz, i-1, prevDz)
		}

		prevDz = dz
	}

	// The overall level with barrier must be lower than without.
	baseLevel, err := ComputeReceiverLevel(receiver, []PointSource{source}, cfg)
	if err != nil {
		t.Fatalf("base level: %v", err)
	}

	cfgBarrier := cfg
	cfgBarrier.Barrier = barrier

	barrierLevel, err := ComputeReceiverLevel(receiver, []PointSource{source}, cfgBarrier)
	if err != nil {
		t.Fatalf("barrier level: %v", err)
	}

	if barrierLevel >= baseLevel {
		t.Errorf("expected barrier to reduce level: base=%.2f barrier=%.2f", baseLevel, barrierLevel)
	}

	// Verify the reduction is meaningful (at least a few dB for this geometry).
	if baseLevel-barrierLevel < 2 {
		t.Errorf("expected meaningful barrier insertion loss (>2 dB), got %.2f dB",
			baseLevel-barrierLevel)
	}
}

func TestValidationCmetReducesLongTermLevel(t *testing.T) {
	t.Parallel()

	source := refSource()
	receiver := refReceiver()

	// With C0 = 3 and sufficient distance, the meteorological correction
	// reduces L_AT(LT) below L_AT(DW).
	cfg := refConfig()
	cfg.C0 = 3

	indicators, err := ComputeReceiverIndicators(receiver, []PointSource{source}, cfg)
	if err != nil {
		t.Fatalf("ComputeReceiverIndicators: %v", err)
	}

	// Verify C_met formula: C_met = C0 * (1 - 10*(hs+hr)/dp) when dp > 10*(hs+hr).
	// hs=10, hr=4, dp=200: limit = 10*(10+4) = 140, dp=200 > 140.
	// C_met = 3 * (1 - 140/200) = 3 * 0.3 = 0.9.
	expectedCmet := 3.0 * (1 - 140.0/200.0)
	gotCmet := MeteorologicalCorrection(3, 10, 4, 200)

	if math.Abs(gotCmet-expectedCmet) > 0.001 {
		t.Errorf("C_met: expected %.3f, got %.3f", expectedCmet, gotCmet)
	}

	// L_AT(LT) = L_AT(DW) - C_met, so LT < DW.
	if indicators.LpAeqLT >= indicators.LpAeqDW {
		t.Errorf("expected LT < DW with C0=3: LT=%.3f DW=%.3f", indicators.LpAeqLT, indicators.LpAeqDW)
	}

	expectedDiff := expectedCmet
	gotDiff := indicators.LpAeqDW - indicators.LpAeqLT

	if math.Abs(gotDiff-expectedDiff) > 0.01 {
		t.Errorf("DW - LT: expected %.3f (C_met), got %.3f", expectedDiff, gotDiff)
	}

	// When dp <= 10*(hs+hr), C_met should be 0.
	closeReceiver := geo.PointReceiver{
		ID:      "rcv_close",
		Point:   geo.Point2D{X: 100, Y: 0},
		HeightM: 4,
	}

	closeIndicators, err := ComputeReceiverIndicators(closeReceiver, []PointSource{source}, cfg)
	if err != nil {
		t.Fatalf("close receiver: %v", err)
	}

	// dp=100, limit=140, dp < limit => C_met = 0 => LT == DW.
	if closeIndicators.LpAeqLT != closeIndicators.LpAeqDW {
		t.Errorf("expected LT == DW for close receiver (dp < limit): LT=%.3f DW=%.3f",
			closeIndicators.LpAeqLT, closeIndicators.LpAeqDW)
	}
}
