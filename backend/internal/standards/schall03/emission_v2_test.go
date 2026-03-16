package schall03

import (
	"math"
	"testing"
)

const floatTol = 1e-3

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < floatTol
}

// TestTeilquelleGl1Basic verifies Gl. 1 for Fz1 m=1 at 250 km/h on
// Schwellengleis (no corrections).
// Expected at 1000 Hz (band index 4):
//
//	a_A=62, Δa_f[4]=-3, n_Q=4, n_Q0=4, b[4]=25, v=250, v_0=100
//	L = 62 + (-3) + 10*lg(4/4) + 25*lg(250/100) = 62 - 3 + 0 + 9.949 = 68.949 dB
func TestTeilquelleGl1Basic(t *testing.T) {
	t.Parallel()

	fz1 := FzKategorien[0]   // Fz-Kategorie 1
	tq := fz1.Teilquellen[0] // m=1

	if tq.M != 1 {
		t.Fatalf("expected m=1, got m=%d", tq.M)
	}

	level := computeTeilquelleLevel(
		tq, 4, 4, 250,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondNone,
		0, false, 0, // bridgeType=0, bridgeMitig=false, curveRadiusM=0
		false,
	)

	// 1000 Hz is band index 4.
	expected := 62.0 + (-3.0) + 0 + 25.0*math.Log10(250.0/100.0)
	if !almostEqual(level[4], expected) {
		t.Errorf("1000 Hz: expected %.3f, got %.3f", expected, level[4])
	}

	// Also verify 63 Hz (band index 0):
	// L = 62 + (-50) + 0 + (-5)*lg(2.5) = 12 + (-5)*0.39794 = 12 - 1.99 = 10.01
	exp63 := 62.0 + (-50.0) + (-5.0)*math.Log10(250.0/100.0)
	if !almostEqual(level[0], exp63) {
		t.Errorf("63 Hz: expected %.3f, got %.3f", exp63, level[0])
	}
}

// TestTeilquelleWithFesteFahrbahn verifies c1 corrections apply correctly.
// Fz1 m=1 at 250 km/h on Feste Fahrbahn:
//
//	c1_schiene[4]=3, c1_reflexion[4]=1 (m=1 gets both)
//	L = 68.949 + 3 + 1 = 72.949 dB at 1000 Hz
func TestTeilquelleWithFesteFahrbahn(t *testing.T) {
	t.Parallel()

	fz1 := FzKategorien[0]
	tq := fz1.Teilquellen[0] // m=1

	level := computeTeilquelleLevel(
		tq, 4, 4, 250,
		FahrbahnartFesteFahrbahn, SFahrbahnSchwellengleis, SurfaceCondNone,
		0, false, 0, // bridgeType=0, bridgeMitig=false, curveRadiusM=0
		false,
	)

	baseAt1000 := 62.0 + (-3.0) + 25.0*math.Log10(250.0/100.0) // 68.949
	expected := baseAt1000 + 3.0 + 1.0                         // schiene + reflexion at 1000 Hz

	if !almostEqual(level[4], expected) {
		t.Errorf("1000 Hz with Feste Fahrbahn: expected %.3f, got %.3f", expected, level[4])
	}
}

// TestTeilquelleWithBridge verifies K_Br applies to rolling noise (m=1,2) only.
func TestTeilquelleWithBridge(t *testing.T) {
	t.Parallel()

	fz1 := FzKategorien[0]

	// m=1 (rolling) should get K_Br=6 for bridge type 2.
	tqM1 := fz1.Teilquellen[0] // m=1
	levelM1 := computeTeilquelleLevel(
		tqM1, 4, 4, 250,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondNone,
		2, false, 0, false,
	)

	baseM1 := 62.0 + (-3.0) + 25.0*math.Log10(250.0/100.0)
	expectedM1 := baseM1 + 6.0 // K_Br=6 for bridge type 2

	if !almostEqual(levelM1[4], expectedM1) {
		t.Errorf("m=1 with bridge type 2 at 1000 Hz: expected %.3f, got %.3f", expectedM1, levelM1[4])
	}

	// m=5 (aerodynamic) should NOT get bridge correction.
	tqM5 := fz1.Teilquellen[2] // m=5
	if tqM5.M != 5 {
		t.Fatalf("expected m=5, got m=%d", tqM5.M)
	}

	levelM5 := computeTeilquelleLevel(
		tqM5, 4, 4, 250,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondNone,
		2, false, 0, false,
	)

	// m=5: a_A=43, Δa[4]=-6, b[4]=50 (aerodynamic), no bridge
	baseM5 := 43.0 + (-6.0) + 50.0*math.Log10(250.0/100.0)
	if !almostEqual(levelM5[4], baseM5) {
		t.Errorf("m=5 with bridge should have NO bridge correction at 1000 Hz: expected %.3f, got %.3f", baseM5, levelM5[4])
	}
}

// TestTeilquelleWithBridgeMitigation verifies K_LM is applied when mitigation
// is active and K_LM is not NaN.
func TestTeilquelleWithBridgeMitigation(t *testing.T) {
	t.Parallel()

	fz1 := FzKategorien[0]
	tqM1 := fz1.Teilquellen[0] // m=1

	// Bridge type 2 (K_Br=6, K_LM=-3) with mitigation.
	level := computeTeilquelleLevel(
		tqM1, 4, 4, 250,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondNone,
		2, true, 0, false,
	)

	baseM1 := 62.0 + (-3.0) + 25.0*math.Log10(250.0/100.0)
	expected := baseM1 + 6.0 + (-3.0) // K_Br + K_LM

	if !almostEqual(level[4], expected) {
		t.Errorf("m=1 with bridge type 2 + mitigation at 1000 Hz: expected %.3f, got %.3f", expected, level[4])
	}
}

// TestTeilquelleWithCurve verifies K_L applies correctly.
// Curve r=200m: K_L=8 for m=1,2 (rolling noise).
func TestTeilquelleWithCurve(t *testing.T) {
	t.Parallel()

	fz1 := FzKategorien[0]
	tqM1 := fz1.Teilquellen[0] // m=1

	level := computeTeilquelleLevel(
		tqM1, 4, 4, 250,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondNone,
		0, false, 200, false,
	)

	baseM1 := 62.0 + (-3.0) + 25.0*math.Log10(250.0/100.0)
	expected := baseM1 + 8.0 // K_L=8 for r<300

	if !almostEqual(level[4], expected) {
		t.Errorf("m=1 with curve r=200m at 1000 Hz: expected %.3f, got %.3f", expected, level[4])
	}

	// m=8 (aggregate) should NOT get curve correction.
	tqM8 := fz1.Teilquellen[5] // m=8
	if tqM8.M != 8 {
		t.Fatalf("expected m=8, got m=%d", tqM8.M)
	}

	levelM8 := computeTeilquelleLevel(
		tqM8, 4, 4, 250,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondNone,
		0, false, 200, false,
	)

	baseM8 := 62.0 + (-5.0) + (-10.0)*math.Log10(250.0/100.0)
	if !almostEqual(levelM8[4], baseM8) {
		t.Errorf("m=8 with curve should have NO curve correction at 1000 Hz: expected %.3f, got %.3f", baseM8, levelM8[4])
	}
}

// TestEmissionGl2EnergSum verifies the energetic summation in Gl. 2.
// Two Fahrzeugeinheiten of the same type should yield +3.01 dB vs one.
func TestEmissionGl2EnergSum(t *testing.T) {
	t.Parallel()

	input1 := StreckeEmissionInput{
		Vehicles: []VehicleInput{{Fz: 1, NPerHour: 1}},
		SpeedKPH: 250,
	}

	input2 := StreckeEmissionInput{
		Vehicles: []VehicleInput{{Fz: 1, NPerHour: 2}},
		SpeedKPH: 250,
	}

	res1, err := ComputeStreckeEmission(input1)
	if err != nil {
		t.Fatalf("single: %v", err)
	}

	res2, err := ComputeStreckeEmission(input2)
	if err != nil {
		t.Fatalf("double: %v", err)
	}

	// Check all heights present in both results.
	for h, spec1 := range res1.PerHeight {
		spec2, ok := res2.PerHeight[h]
		if !ok {
			t.Errorf("height %d missing in double result", h)
			continue
		}

		for f := range NumBeiblattOctaveBands {
			diff := spec2[f] - spec1[f]
			// 10*lg(2) = 3.0103
			if !almostEqual(diff, 10*math.Log10(2)) {
				t.Errorf("height %d band %d: expected +3.01 dB, got %.3f dB", h, f, diff)
			}
		}
	}
}

// TestEmissionICE1Full tests full ICE-1 (2×Fz1 + 12×Fz2) at 250 km/h.
func TestEmissionICE1Full(t *testing.T) {
	t.Parallel()

	input := StreckeEmissionInput{
		Vehicles: []VehicleInput{
			{Fz: 1, NPerHour: 2},
			{Fz: 2, NPerHour: 12},
		},
		SpeedKPH: 250,
	}

	result, err := ComputeStreckeEmission(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fz1 has heights 1, 2, 3; Fz2 has heights 1, 2.
	if _, ok := result.PerHeight[1]; !ok {
		t.Error("missing height 1")
	}

	if _, ok := result.PerHeight[2]; !ok {
		t.Error("missing height 2")
	}

	if _, ok := result.PerHeight[3]; !ok {
		t.Error("missing height 3 (from Fz1 m=5 aerodynamic)")
	}

	// Verify plausible levels (roughly 80-100 dB range for a full train at 250 km/h).
	for h, spec := range result.PerHeight {
		for f := range NumBeiblattOctaveBands {
			if math.IsInf(spec[f], -1) {
				continue
			}

			if spec[f] < 20 || spec[f] > 130 {
				t.Errorf("height %d band %d: level %.1f dB outside plausible range [20, 130]", h, f, spec[f])
			}
		}
	}

	// The 1000 Hz band at height 1 should be the loudest since rolling noise
	// dominates and most Teilquellen are at height 1.
	h1 := result.PerHeight[1]
	// Just verify it's in a reasonable range.
	if h1[4] < 70 || h1[4] > 110 {
		t.Errorf("height 1, 1000 Hz: expected 70-110 dB, got %.1f", h1[4])
	}
}

// TestEmissionSpeedZeroReturnsError verifies error on zero speed.
func TestEmissionSpeedZeroReturnsError(t *testing.T) {
	t.Parallel()

	input := StreckeEmissionInput{
		Vehicles: []VehicleInput{{Fz: 1, NPerHour: 1}},
		SpeedKPH: 0,
	}

	_, err := ComputeStreckeEmission(input)
	if err == nil {
		t.Error("expected error for zero speed, got nil")
	}
}

// TestEmissionUnknownFzReturnsError verifies error on invalid Fz number.
func TestEmissionUnknownFzReturnsError(t *testing.T) {
	t.Parallel()

	input := StreckeEmissionInput{
		Vehicles: []VehicleInput{{Fz: 99, NPerHour: 1}},
		SpeedKPH: 100,
	}

	_, err := ComputeStreckeEmission(input)
	if err == nil {
		t.Error("expected error for unknown Fz, got nil")
	}
}

// TestEmissionNegativeSpeedReturnsError verifies error on negative speed.
func TestEmissionNegativeSpeedReturnsError(t *testing.T) {
	t.Parallel()

	input := StreckeEmissionInput{
		Vehicles: []VehicleInput{{Fz: 1, NPerHour: 1}},
		SpeedKPH: -10,
	}

	_, err := ComputeStreckeEmission(input)
	if err == nil {
		t.Error("expected error for negative speed, got nil")
	}
}

// TestEmissionNoVehiclesReturnsError verifies error when no vehicles given.
func TestEmissionNoVehiclesReturnsError(t *testing.T) {
	t.Parallel()

	input := StreckeEmissionInput{
		Vehicles: nil,
		SpeedKPH: 100,
	}

	_, err := ComputeStreckeEmission(input)
	if err == nil {
		t.Error("expected error for no vehicles, got nil")
	}
}

// TestAxleCorrectionNotAppliedToNonRolling verifies that the axle count
// correction only applies to rolling noise (m=1,2,3,4), not to other source
// types.
func TestAxleCorrectionNotAppliedToNonRolling(t *testing.T) {
	t.Parallel()

	fz1 := FzKategorien[0]
	// m=8 is aggregate.
	tqM8 := fz1.Teilquellen[5]
	if tqM8.M != 8 {
		t.Fatalf("expected m=8, got m=%d", tqM8.M)
	}

	// With nQ=8, nQ0=4, the axle correction would be 10*lg(2) = 3.01 dB
	// if it were applied.
	levelWith8 := computeTeilquelleLevel(
		tqM8, 8, 4, 100,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondNone,
		0, false, 0, // bridgeType=0, bridgeMitig=false, curveRadiusM=0
		false,
	)

	levelWith4 := computeTeilquelleLevel(
		tqM8, 4, 4, 100,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondNone,
		0, false, 0, // bridgeType=0, bridgeMitig=false, curveRadiusM=0
		false,
	)

	// For non-rolling noise, axle count should not matter.
	for f := range NumBeiblattOctaveBands {
		if !almostEqual(levelWith8[f], levelWith4[f]) {
			t.Errorf("band %d: m=8 should ignore axle count, got nQ=8: %.3f, nQ=4: %.3f",
				f, levelWith8[f], levelWith4[f])
		}
	}
}

// TestSurfaceCondBuG verifies büG c2 correction is applied to correct Teilquellen.
func TestSurfaceCondBuG(t *testing.T) {
	t.Parallel()

	fz1 := FzKategorien[0]
	tqM1 := fz1.Teilquellen[0] // m=1

	levelNone := computeTeilquelleLevel(
		tqM1, 4, 4, 100,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondNone,
		0, false, 0, // bridgeType=0, bridgeMitig=false, curveRadiusM=0
		false,
	)

	levelBuG := computeTeilquelleLevel(
		tqM1, 4, 4, 100,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondBuG,
		0, false, 0, // bridgeType=0, bridgeMitig=false, curveRadiusM=0
		false,
	)

	// büG at 1000 Hz (band 4): c2=-5 for m=1.
	diff := levelBuG[4] - levelNone[4]
	if !almostEqual(diff, -5.0) {
		t.Errorf("büG m=1 at 1000 Hz: expected -5 dB difference, got %.3f", diff)
	}

	// m=2 is NOT in büG Teilquellen list (büG applies to m=1,3 only).
	tqM2 := fz1.Teilquellen[1] // m=2
	levelM2None := computeTeilquelleLevel(
		tqM2, 4, 4, 100,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondNone,
		0, false, 0, // bridgeType=0, bridgeMitig=false, curveRadiusM=0
		false,
	)

	levelM2BuG := computeTeilquelleLevel(
		tqM2, 4, 4, 100,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondBuG,
		0, false, 0, // bridgeType=0, bridgeMitig=false, curveRadiusM=0
		false,
	)

	for f := range NumBeiblattOctaveBands {
		if !almostEqual(levelM2BuG[f], levelM2None[f]) {
			t.Errorf("büG should not affect m=2, band %d: got diff %.3f", f, levelM2BuG[f]-levelM2None[f])
		}
	}
}

func TestStrassenbahnEmissionFz21Basic(t *testing.T) {
	// Fz 21 Niederflur at 60 km/h, reference track (Schwellengleis).
	// Checks that the pipeline accepts Fz 21 and returns a non-nil result.
	input := StreckeEmissionInput{
		Vehicles:  []VehicleInput{{Fz: 21, NPerHour: 10}},
		SpeedKPH:  60,
		SFahrbahn: SFahrbahnSchwellengleis,
	}

	result, err := ComputeStreckeEmission(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil || len(result.PerHeight) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestStrassenbahnSpeedClamp50(t *testing.T) {
	// Speeds below 50 km/h must be clamped to 50 for Strassenbahn (Nr. 5.3.2).
	// Same vehicle at 30 and 50 must produce the same result.
	base := StreckeEmissionInput{
		Vehicles:  []VehicleInput{{Fz: 21, NPerHour: 10}},
		SpeedKPH:  50,
		SFahrbahn: SFahrbahnSchwellengleis,
	}
	clamped := StreckeEmissionInput{
		Vehicles:  []VehicleInput{{Fz: 21, NPerHour: 10}},
		SpeedKPH:  30,
		SFahrbahn: SFahrbahnSchwellengleis,
	}
	r1, _ := ComputeStreckeEmission(base)
	r2, _ := ComputeStreckeEmission(clamped)

	for h, sp1 := range r1.PerHeight {
		sp2 := r2.PerHeight[h]
		for f := range NumBeiblattOctaveBands {
			if sp1[f] != sp2[f] {
				t.Errorf("h=%d f=%d: v=50 gave %g, v=30 gave %g (should be equal after clamp)", h, f, sp1[f], sp2[f])
			}
		}
	}
}

func TestStrassenbahnPermanentlySlowException(t *testing.T) {
	// Nr. 5.3.2 exception: sections permanently at ≤ 30 km/h use v=30 km/h
	// instead of clamping to 50.  Result must differ from the clamped case.
	clamped := StreckeEmissionInput{
		Vehicles:  []VehicleInput{{Fz: 21, NPerHour: 10}},
		SpeedKPH:  20,
		SFahrbahn: SFahrbahnSchwellengleis,
	}
	permanentlySlow := StreckeEmissionInput{
		Vehicles:        []VehicleInput{{Fz: 21, NPerHour: 10}},
		SpeedKPH:        20,
		SFahrbahn:       SFahrbahnSchwellengleis,
		PermanentlySlow: true,
	}

	rClamped, err := ComputeStreckeEmission(clamped)
	if err != nil {
		t.Fatal(err)
	}

	rSlow, err := ComputeStreckeEmission(permanentlySlow)
	if err != nil {
		t.Fatal(err)
	}

	// Clamped uses v=50 km/h; permanently slow uses v=30 km/h.
	// The levels must differ (different effective speeds).
	foundDiff := false

	for h, spClamped := range rClamped.PerHeight {
		spSlow := rSlow.PerHeight[h]

		for f := range NumBeiblattOctaveBands {
			if spClamped[f] != spSlow[f] {
				foundDiff = true
			}
		}
	}

	if !foundDiff {
		t.Error("permanently slow exception should produce different levels from clamped case")
	}
}

func TestStrassenbahnC1Correction(t *testing.T) {
	// Result with Strassenbuendiger Bahnkoerper must differ from Schwellengleis.
	ref := StreckeEmissionInput{
		Vehicles:  []VehicleInput{{Fz: 22, NPerHour: 10}},
		SpeedKPH:  60,
		SFahrbahn: SFahrbahnSchwellengleis,
	}
	corr := StreckeEmissionInput{
		Vehicles:  []VehicleInput{{Fz: 22, NPerHour: 10}},
		SpeedKPH:  60,
		SFahrbahn: SFahrbahnStrassenbuendig,
	}
	r1, _ := ComputeStreckeEmission(ref)
	r2, _ := ComputeStreckeEmission(corr)
	// At least one band should differ.
	h := 1
	for f := range NumBeiblattOctaveBands {
		if r1.PerHeight[h][f] != r2.PerHeight[h][f] {
			return // found a difference — pass
		}
	}

	t.Error("c1 correction had no effect; reference and corrected results are identical")
}

func TestMixedEisenbahnAndStrassenbahnRejected(t *testing.T) {
	// A segment must not mix Eisenbahn (Fz 1-10) and Strassenbahn (Fz 21-23).
	input := StreckeEmissionInput{
		Vehicles: []VehicleInput{
			{Fz: 7, NPerHour: 5},  // E-Lok (Eisenbahn)
			{Fz: 21, NPerHour: 5}, // Niederflur (Strassenbahn)
		},
		SpeedKPH:  80,
		SFahrbahn: SFahrbahnSchwellengleis,
	}

	_, err := ComputeStreckeEmission(input)
	if err == nil {
		t.Error("expected error when mixing Eisenbahn and Strassenbahn vehicles")
	}
}

// TestCustomAxleCount verifies axle correction is applied for rolling noise
// when a custom axle count differs from the reference.
func TestCustomAxleCount(t *testing.T) {
	t.Parallel()

	fz1 := FzKategorien[0]
	tqM1 := fz1.Teilquellen[0] // m=1, nQ0=4

	// nQ=8 vs nQ0=4: correction = 10*lg(8/4) = 10*lg(2) ≈ 3.01 dB.
	levelDefault := computeTeilquelleLevel(
		tqM1, 4, 4, 100,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondNone,
		0, false, 0, // bridgeType=0, bridgeMitig=false, curveRadiusM=0
		false,
	)

	levelDouble := computeTeilquelleLevel(
		tqM1, 8, 4, 100,
		FahrbahnartSchwellengleis, SFahrbahnSchwellengleis, SurfaceCondNone,
		0, false, 0, // bridgeType=0, bridgeMitig=false, curveRadiusM=0
		false,
	)

	for f := range NumBeiblattOctaveBands {
		diff := levelDouble[f] - levelDefault[f]
		if !almostEqual(diff, 10*math.Log10(2)) {
			t.Errorf("band %d: expected axle correction +3.01 dB, got %.3f", f, diff)
		}
	}
}
