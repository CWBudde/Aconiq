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

// TestValidationNoteOneSingle500HzBand pins the absolute level of the default
// import path, where a source carries only an A-weighted sound power level and
// OctaveBandLevels is nil.
//
// ISO 9613-2:1996 NOTE 1 evaluates the 500 Hz attenuation terms once. The
// previous implementation replicated L_WA into all eight bands and then
// energy-summed them with the A-weighting of Eq. 5, which weighted the already
// A-weighted level a second time and inflated the result by
// 10*lg(sum_j 10^(0.1*A_j)) = +6.99 dB.
func TestValidationNoteOneSingle500HzBand(t *testing.T) {
	t.Parallel()

	source := PointSource{
		ID:                "src",
		Point:             geo.Point2D{X: 0, Y: 0},
		SourceHeightM:     4,
		SoundPowerLevelDB: 100,
		// OctaveBandLevels deliberately nil: this is the default import path.
	}
	receiver := geo.PointReceiver{ID: "rcv", Point: geo.Point2D{X: 100, Y: 0}, HeightM: 4}

	cfg := refConfig()
	cfg.GroundFactor = 0 // hard ground keeps A_gr free of the a'..d' functions

	// Hand calculation at 500 Hz, d = dp = 100 m (source and receiver both at 4 m):
	//   A_div = 20*lg(100) + 11                        = 51.00 dB
	//   A_atm = 1.9 dB/km * 100 m / 1000               =  0.19 dB   (Table 2, 10 C / 70 %)
	//   A_gr  = A_s + A_r + A_m = -1.5 + -1.5 + 0      = -3.00 dB   (Table 3, G = 0, q = 0)
	//   A_bar = 0
	//   L_AT(DW) = 100 - (51.00 + 0.19 - 3.00)         = 51.81 dB
	expected := 100.0 - (20*math.Log10(100) + 11 + 1.9*100/1000 - 3.0)

	indicators, err := ComputeReceiverIndicators(receiver, []PointSource{source}, cfg)
	if err != nil {
		t.Fatalf("ComputeReceiverIndicators: %v", err)
	}

	if math.Abs(indicators.LpAeqDW-expected) > 0.001 {
		t.Errorf("Note 1 level: expected %.3f dB, got %.3f dB (delta %.3f dB)",
			expected, indicators.LpAeqDW, indicators.LpAeqDW-expected)
	}
}

// TestValidationNoteOneMatchesEquivalent500HzSpectrum cross-checks that the
// A-weighting is applied exactly once on each of the two paths. A source whose
// energy sits entirely in the 500 Hz band at L_W = L_WA - A_500 must produce
// the same receiver level as the NOTE 1 estimate for L_WA.
func TestValidationNoteOneMatchesEquivalent500HzSpectrum(t *testing.T) {
	t.Parallel()

	const lwa = 100.0

	receiver := geo.PointReceiver{ID: "rcv", Point: geo.Point2D{X: 150, Y: 0}, HeightM: 4}
	cfg := refConfig()

	broadband := PointSource{
		ID:                "broadband",
		Point:             geo.Point2D{X: 0, Y: 0},
		SourceHeightM:     4,
		SoundPowerLevelDB: lwa,
	}

	// Silent everywhere except 500 Hz, where L_W = L_WA - A_500.
	const silent = -400.0

	bands := BandLevels{silent, silent, silent, lwa - AWeighting[NoteOneBandIndex], silent, silent, silent, silent}
	spectral := broadband
	spectral.ID = "spectral"
	spectral.OctaveBandLevels = &bands

	broadbandLevel, err := ComputeReceiverLevel(receiver, []PointSource{broadband}, cfg)
	if err != nil {
		t.Fatalf("broadband: %v", err)
	}

	spectralLevel, err := ComputeReceiverLevel(receiver, []PointSource{spectral}, cfg)
	if err != nil {
		t.Fatalf("spectral: %v", err)
	}

	if math.Abs(broadbandLevel-spectralLevel) > 0.001 {
		t.Errorf("expected the NOTE 1 estimate to match an equivalent 500 Hz spectrum: broadband=%.4f spectral=%.4f",
			broadbandLevel, spectralLevel)
	}
}

// TestValidationGroundMiddleRegion63Hz covers ISO 9613-2:1996 Table 3, where
// A_m is -3q at 63 Hz. The implementation previously returned +3q there, an
// error of 6q dB in the 63 Hz band.
func TestValidationGroundMiddleRegion63Hz(t *testing.T) {
	t.Parallel()

	// hs = hr = 2 m, dp = 200 m: limit = 30*(2+2) = 120 m < dp,
	// so q = 1 - 120/200 = 0.4.
	hs, hr, dp := 2.0, 2.0, 200.0

	q := middleRegionQ(hs, hr, dp)
	if math.Abs(q-0.4) > 1e-12 {
		t.Fatalf("q: expected 0.4, got %v", q)
	}

	// Table 3: A_m(63 Hz) = -3q, independent of G_m.
	for _, gm := range []float64{0, 0.5, 1} {
		got := middleRegionAtten(gm, 0, q)
		if math.Abs(got-(-3*q)) > 1e-12 {
			t.Errorf("A_m(63 Hz, G_m=%.1f): expected %.4f, got %.4f", gm, -3*q, got)
		}
	}

	// Full band: A_s = A_r = -1.5 (63 Hz, any G), A_m = -1.2 => A_gr = -4.2 dB.
	bands := GroundEffectBands(0.5, 0.5, 0.5, hs, hr, dp)

	expected := -1.5 + -1.5 + -3*q
	if math.Abs(bands[0]-expected) > 1e-9 {
		t.Errorf("A_gr(63 Hz): expected %.4f dB, got %.4f dB", expected, bands[0])
	}
}

// TestValidationCmetIsAppliedPerSource covers Eq. 6 together with clause 8:
// C_met depends on the source height and the projected distance of one point
// source, so it must be applied to each path before the energy summation of
// Eq. 5. Deriving a single C_met from the farthest source over-corrected every
// nearer source by up to C_0 dB.
func TestValidationCmetIsAppliedPerSource(t *testing.T) {
	t.Parallel()

	receiver := geo.PointReceiver{ID: "rcv", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4}

	near := PointSource{ID: "near", Point: geo.Point2D{X: 100, Y: 0}, SourceHeightM: 4, SoundPowerLevelDB: 100}
	far := PointSource{ID: "far", Point: geo.Point2D{X: 1000, Y: 0}, SourceHeightM: 4, SoundPowerLevelDB: 120}

	cfg := refConfig()
	cfg.C0 = 5

	// limit = 10*(hs + hr) = 80 m.
	// near: C_met = 5*(1 - 80/100)  = 1.00 dB
	// far:  C_met = 5*(1 - 80/1000) = 4.60 dB
	cmetNear := MeteorologicalCorrection(cfg.C0, near.SourceHeightM, receiver.HeightM, 100)
	cmetFar := MeteorologicalCorrection(cfg.C0, far.SourceHeightM, receiver.HeightM, 1000)

	if math.Abs(cmetNear-1.0) > 1e-9 || math.Abs(cmetFar-4.6) > 1e-9 {
		t.Fatalf("C_met setup: near=%.4f far=%.4f", cmetNear, cmetFar)
	}

	downwind := refConfig() // C0 = 0 => per-source downwind levels

	nearDW, err := ComputeReceiverLevel(receiver, []PointSource{near}, downwind)
	if err != nil {
		t.Fatalf("near: %v", err)
	}

	farDW, err := ComputeReceiverLevel(receiver, []PointSource{far}, downwind)
	if err != nil {
		t.Fatalf("far: %v", err)
	}

	indicators, err := ComputeReceiverIndicators(receiver, []PointSource{near, far}, cfg)
	if err != nil {
		t.Fatalf("ComputeReceiverIndicators: %v", err)
	}

	expectedLT := 10 * math.Log10(math.Pow(10, 0.1*(nearDW-cmetNear))+math.Pow(10, 0.1*(farDW-cmetFar)))
	if math.Abs(indicators.LpAeqLT-expectedLT) > 1e-9 {
		t.Errorf("L_AT(LT): expected %.6f dB, got %.6f dB", expectedLT, indicators.LpAeqLT)
	}

	// The pre-fix behaviour subtracted the farthest source's C_met from the
	// whole sum, over-correcting the near source.
	overCorrected := indicators.LpAeqDW - cmetFar
	if indicators.LpAeqLT <= overCorrected {
		t.Errorf("expected the near source not to be over-corrected: LT=%.4f, single-C_met result=%.4f",
			indicators.LpAeqLT, overCorrected)
	}

	// The total correction must lie between the two per-path corrections.
	total := indicators.LpAeqDW - indicators.LpAeqLT
	if total < cmetNear-1e-9 || total > cmetFar+1e-9 {
		t.Errorf("total correction %.4f dB outside [%.4f, %.4f]", total, cmetNear, cmetFar)
	}
}

// TestValidationCmetZeroIsExactlyNeutral guards the invariant that moving
// C_met into the per-source loop leaves results bit-identical while C_0 = 0,
// which is the CLI default (c0_met = 0).
func TestValidationCmetZeroIsExactlyNeutral(t *testing.T) {
	t.Parallel()

	receiver := geo.PointReceiver{ID: "rcv", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4}
	sources := []PointSource{
		{ID: "a", Point: geo.Point2D{X: 100, Y: 0}, SourceHeightM: 4, SoundPowerLevelDB: 100},
		{ID: "b", Point: geo.Point2D{X: 1000, Y: 25}, SourceHeightM: 12, SoundPowerLevelDB: 120},
		{ID: "c", Point: geo.Point2D{X: -350, Y: -40}, SourceHeightM: 1, SoundPowerLevelDB: 95},
	}

	cfg := refConfig()
	if cfg.C0 != 0 {
		t.Fatalf("expected C0 = 0 in the reference config, got %v", cfg.C0)
	}

	indicators, err := ComputeReceiverIndicators(receiver, sources, cfg)
	if err != nil {
		t.Fatalf("ComputeReceiverIndicators: %v", err)
	}

	if indicators.LpAeqLT != indicators.LpAeqDW {
		t.Errorf("with C0 = 0 expected LT to be bit-identical to DW, got LT=%v DW=%v",
			indicators.LpAeqLT, indicators.LpAeqDW)
	}
}
