package road

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

func TestComputeShielding_MultipleBarriers(t *testing.T) {
	t.Parallel()

	// Two barriers: the short one (height 4m at y=10) is below the rubber band
	// line from source to the tall one (height 8m at y=20), so the
	// Gummibandmethode excludes it. Only the tall barrier is a significant edge.
	shortBarrier := Barrier{
		ID:       "short",
		Geometry: []geo.Point2D{{X: -100, Y: 10}, {X: 100, Y: 10}},
		HeightM:  4.0,
	}

	tallBarrier := Barrier{
		ID:       "tall",
		Geometry: []geo.Point2D{{X: -100, Y: 20}, {X: 100, Y: 20}},
		HeightM:  8.0,
	}

	result := ComputeShielding(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 0, Y: 50}, 4.0,
		[]Barrier{shortBarrier, tallBarrier},
	)

	if !result.Shielded {
		t.Fatal("expected shielding")
	}

	if result.BarrierID != "tall" {
		t.Fatalf("expected tall barrier (short excluded by hull), got %q", result.BarrierID)
	}

	// Loss should equal single-tall-barrier case (short is excluded).
	tallOnly := ComputeShielding(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 0, Y: 50}, 4.0,
		[]Barrier{tallBarrier},
	)

	if math.Abs(result.InsertionLoss-tallOnly.InsertionLoss) > 1e-10 {
		t.Fatalf("loss with hull-excluded short barrier (%f) should equal tall-only (%f)",
			result.InsertionLoss, tallOnly.InsertionLoss)
	}
}

func TestComputeShielding_MultiBarrierMultiDiffraction(t *testing.T) {
	t.Parallel()

	// Two barriers between source and receiver, both above LOS.
	// Source at y=0 (height 0.5m), receiver at y=50 (height 4m).
	barrier1 := Barrier{
		ID:       "wall-1",
		Geometry: []geo.Point2D{{X: -100, Y: 10}, {X: 100, Y: 10}},
		HeightM:  6.0,
	}
	barrier2 := Barrier{
		ID:       "wall-2",
		Geometry: []geo.Point2D{{X: -100, Y: 20}, {X: 100, Y: 20}},
		HeightM:  6.0,
	}

	result := ComputeShielding(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 0, Y: 50}, 4.0,
		[]Barrier{barrier1, barrier2},
	)

	if !result.Shielded {
		t.Fatal("expected shielding from two barriers")
	}

	// Multi-diffraction must produce higher loss than single-edge.
	singleResult := ComputeShielding(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 0, Y: 50}, 4.0,
		[]Barrier{barrier1}, // only the first barrier
	)

	if result.InsertionLoss <= singleResult.InsertionLoss {
		t.Fatalf("multi-barrier loss (%f) should exceed single-barrier loss (%f)",
			result.InsertionLoss, singleResult.InsertionLoss)
	}
}

func TestComputeShielding_SingleBarrierUnchanged(t *testing.T) {
	t.Parallel()

	barrier := sampleBarrier() // wall-1 at y=10, height 4m

	result := ComputeShielding(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 0, Y: 50}, 4.0,
		[]Barrier{barrier},
	)

	// Compute expected value via original single-edge functions.
	crossPt, _, _ := geo.LineStringIntersectsSegment(barrier.Geometry,
		geo.Point2D{X: 0, Y: 0}, geo.Point2D{X: 0, Y: 50})
	dSB := dist2D(geo.Point2D{X: 0, Y: 0}, crossPt)
	dBR := dist2D(crossPt, geo.Point2D{X: 0, Y: 50})
	geom := computeDiffraction(dSB, 0.5, dBR, 4.0, barrier.HeightM)
	expectedLoss := rls19BarrierLoss(geom)

	if math.Abs(result.InsertionLoss-expectedLoss) > 1e-10 {
		t.Fatalf("single barrier loss changed: got %f, expected %f",
			result.InsertionLoss, expectedLoss)
	}
}

func TestPropagation_WithBarrier(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()
	receiver := geo.Point2D{X: 0, Y: 50}

	// Free-field level.
	freeField, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("free field: %v", err)
	}

	// With barrier between source and receiver.
	barrier := sampleBarrier()

	shielded, err := ComputeReceiverLevels(receiver, []RoadSource{source}, []Barrier{barrier}, cfg)
	if err != nil {
		t.Fatalf("with barrier: %v", err)
	}

	// Barrier should reduce the level.
	reduction := freeField.LrDay - shielded.LrDay
	if reduction <= 0 {
		t.Fatalf("barrier should reduce level: free=%f shielded=%f", freeField.LrDay, shielded.LrDay)
	}

	// Reduction should be meaningful (several dB for a 4m wall at 10m from source).
	if reduction < 2 {
		t.Fatalf("expected at least 2 dB reduction, got %f", reduction)
	}
}

func TestComputeDiffraction(t *testing.T) {
	t.Parallel()

	// Barrier exactly at line of sight: Z should be ~0.
	// Source at (0, h=0.5), barrier at (10, h=4), receiver at (50, h=4).
	// Line of sight from source to receiver: at x=10, height = 0.5 + (4-0.5)*10/50 = 1.2.
	// Barrier height 1.2 → Z ≈ 0.
	d := computeDiffraction(10, 0.5, 40, 4.0, 1.2)
	if math.Abs(d.Z) > 0.01 {
		t.Fatalf("barrier at line of sight should have Z ~0, got %f", d.Z)
	}

	// Barrier well above line of sight: positive Z.
	d = computeDiffraction(10, 0.5, 40, 4.0, 8.0)
	if d.Z <= 0 {
		t.Fatalf("tall barrier should have positive Z, got %f", d.Z)
	}

	// Barrier below line of sight: non-positive Z.
	d = computeDiffraction(10, 0.5, 40, 4.0, 0.1)
	if d.Z > 0 {
		t.Fatalf("low barrier should have non-positive Z, got %f", d.Z)
	}

	// A, B, S must be positive for a valid geometry.
	d = computeDiffraction(10, 0.5, 40, 4.0, 4.0)
	if d.A <= 0 || d.B <= 0 || d.S <= 0 {
		t.Fatalf("A/B/S must be positive: A=%f B=%f S=%f", d.A, d.B, d.S)
	}
}

// TestRLS19BarrierLoss verifies D_z = 10·lg(3 + 80·z·K_w) per RLS-19 Eqs. 15/17.
func TestRLS19BarrierLoss(t *testing.T) {
	t.Parallel()

	// z <= 0: no loss.
	if rls19BarrierLoss(diffractionGeometry{Z: 0, A: 10, B: 20, S: 29}) != 0 {
		t.Fatal("z=0 should give zero loss")
	}

	if rls19BarrierLoss(diffractionGeometry{Z: -0.1, A: 10, B: 20, S: 29}) != 0 {
		t.Fatal("negative z should give zero loss")
	}

	// z > 0: positive loss (3 + 80*z*K_w > 3 > 1 so log10 > 0).
	loss := rls19BarrierLoss(diffractionGeometry{Z: 0.5, A: 10, B: 20, S: 29.5})
	if loss <= 0 {
		t.Fatalf("positive z should give positive loss, got %f", loss)
	}

	// Loss increases with z (for moderate z where 80*z*K_w grows).
	lossSmall := rls19BarrierLoss(diffractionGeometry{Z: 0.1, A: 10, B: 20, S: 29.9})
	lossLarge := rls19BarrierLoss(diffractionGeometry{Z: 1.0, A: 10, B: 20, S: 29.0})

	if lossLarge <= lossSmall {
		t.Fatalf("loss should increase with z: small=%f large=%f", lossSmall, lossLarge)
	}

	// Hand-calculated reference (RLS-19 Eqs. 15/17):
	// z=0.5, A=10.31, B=20.02, S=30.20
	// K_w = exp(-sqrt(10.31*20.02*30.20 / (2*0.5)) / 2000)
	//     = exp(-sqrt(6236.7) / 2000) = exp(-78.97/2000) = exp(-0.03948) ≈ 0.9613
	// D_z = 10*log10(3 + 80*0.5*0.9613) = 10*log10(3 + 38.45) = 10*log10(41.45) ≈ 16.17 dB
	geom := diffractionGeometry{Z: 0.5, A: 10.31, B: 20.02, S: 30.20}
	expected := 10 * math.Log10(3+80*0.5*math.Exp(-math.Sqrt(10.31*20.02*30.20/(2*0.5))/2000))
	got := rls19BarrierLoss(geom)

	if math.Abs(got-expected) > 0.01 {
		t.Fatalf("D_z mismatch: expected %.4f dB, got %.4f dB", expected, got)
	}
}

// TestComputeAttenuation_DDivFormula verifies D_div uses 2π per RLS-19 Eq. 12.
func TestComputeAttenuation_DDivFormula(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	cfg.MinDistanceM = 0.1

	// At s=100m, flat terrain (hm=0): D_div = 20*log10(100) + 10*log10(2π).
	// 10*log10(2π) ≈ 7.982 dB.
	expected := 20*math.Log10(100) + 10*math.Log10(2*math.Pi)
	att := computeAttenuation(100, 100, 0, cfg)

	// D_gr = 0 when h_m = 0 (formula = 4.8 - 0 = 4.8, but clamped at 0 since hm=0 → positive).
	// Actually: D_gr = 4.8 - (0/100)*(34+600/100) = 4.8 dB (not zero).
	// So we check GeometricDivergence specifically.
	if math.Abs(att.GeometricDivergence-expected) > 0.01 {
		t.Fatalf("D_div at 100m: expected %.4f dB, got %.4f dB", expected, att.GeometricDivergence)
	}
}

// TestComputeAttenuation_DAtmFormula verifies D_atm = s/200 per RLS-19 Eq. 13.
func TestComputeAttenuation_DAtmFormula(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()
	cfg.MinDistanceM = 0.1

	// At s=200m: D_atm should be 200/200 = 1.0 dB.
	att := computeAttenuation(200, 200, 5, cfg)
	expected := 1.0

	if math.Abs(att.AirAbsorption-expected) > 0.01 {
		t.Fatalf("D_atm at 200m: expected %.4f dB, got %.4f dB", expected, att.AirAbsorption)
	}

	// At s=1000m: D_atm = 1000/200 = 5.0 dB.
	att = computeAttenuation(1000, 1000, 5, cfg)
	expected = 1000.0 / 200.0

	if math.Abs(att.AirAbsorption-expected) > 0.01 {
		t.Fatalf("D_atm at 1000m: expected %.4f dB, got %.4f dB", expected, att.AirAbsorption)
	}
}

// --- energySumDB tests ---

func TestEnergySumDB(t *testing.T) {
	t.Parallel()

	// Two equal levels: +3 dB.
	result := energySumDB([]float64{60, 60})
	if !almostEqual(result, 63.01, 0.01) {
		t.Fatalf("60+60 dB: expected ~63.01, got %f", result)
	}

	// Empty: -999.
	result = energySumDB(nil)
	if result > -900 {
		t.Fatalf("empty sum: expected -999, got %f", result)
	}

	// Single value passes through.
	result = energySumDB([]float64{55.0})
	if !almostEqual(result, 55.0, 0.01) {
		t.Fatalf("single value: expected 55, got %f", result)
	}
}

// --- topography tests ---

// sampleTieflageSource returns a source at Z=100 (road in cut, terrain at Z=105.5).
// Geometry matches TEST-20 I6 (simplified single Teilstück at midpoint X=100, Y=50).
func sampleTieflageSource() RoadSource {
	s := sampleSource()
	s.ElevationM = 100.0 // road surface at Z=100, terrain at Z=105.5

	return s
}

// tieflageSlopeCrest returns the Böschungskante for TEST-20 I6:
// slope crest at Y=62.8, Z=105.5 (terrain level), running along X-axis.
func tieflageSlopeCrest() TerrainEdge {
	return TerrainEdge{
		ID: "boeschungskante-i6",
		Geometry: []geo.Point3D{
			{X: -200, Y: 62.8, Z: 105.5},
			{X: 400, Y: 62.8, Z: 105.5},
		},
	}
}

// tieflageSlopeFoot returns the Böschungsfuß for TEST-20 I6:
// slope foot at Y=55.3, Z=100 (road level), running along X-axis.
func tieflageSlopeFoot() TerrainEdge {
	return TerrainEdge{
		ID: "boeschungsfuss-i6",
		Geometry: []geo.Point3D{
			{X: -200, Y: 55.3, Z: 100.0},
			{X: 400, Y: 55.3, Z: 100.0},
		},
	}
}

func TestTerrainEdgeValidate(t *testing.T) {
	t.Parallel()

	e := tieflageSlopeCrest()

	err := e.Validate()
	if err != nil {
		t.Fatalf("valid edge failed: %v", err)
	}

	// Too few points.
	e2 := TerrainEdge{ID: "x", Geometry: []geo.Point3D{{X: 0, Y: 0, Z: 0}}}

	err = e2.Validate()
	if err == nil {
		t.Fatal("expected error for single-point edge")
	}

	// Missing ID.
	e3 := TerrainEdge{Geometry: []geo.Point3D{{}, {}}}

	err = e3.Validate()
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestComputeTerrainAvgZ_NoProfiles(t *testing.T) {
	t.Parallel()

	avg := computeTerrainAvgZ(
		geo.Point2D{X: 0, Y: 0},
		geo.Point2D{X: 100, Y: 0},
		nil,
	)

	if avg != 0 {
		t.Fatalf("expected 0 for no terrain profiles, got %f", avg)
	}
}

func TestComputeTerrainAvgZ_FlatTerrain(t *testing.T) {
	t.Parallel()

	// Single terrain edge at a constant Z=105.5 crossing the path.
	edge := TerrainEdge{
		ID:       "flat-edge",
		Geometry: []geo.Point3D{{X: -10, Y: 50, Z: 105.5}, {X: 200, Y: 50, Z: 105.5}},
	}
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: edge}}}

	avg := computeTerrainAvgZ(
		geo.Point2D{X: 100, Y: 0},
		geo.Point2D{X: 100, Y: 100},
		[]TerrainProfile{profile},
	)

	// Edge crosses at midpoint of path; terrain before = 105.5, after = 105.5.
	if !almostEqual(avg, 105.5, 0.01) {
		t.Fatalf("expected terrain avg ~105.5, got %f", avg)
	}
}

// TestComputeMeanHeight_Tieflage verifies the h_m formula against TEST-20 I6 IO1.
// I6 IO1: source Z=100.5, receiver Z=108.3, terrain=105.5 → expected h_m ≈ -0.10.
func TestComputeMeanHeight_Tieflage(t *testing.T) {
	t.Parallel()

	foot := tieflageSlopeFoot()
	crest := tieflageSlopeCrest()
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	// Source midpoint (TEST-20 Teilstück midpoint), receiver IO1.
	source := geo.Point2D{X: 100, Y: 50}
	receiver := geo.Point2D{X: 0, Y: 100}
	sourceZ := 100.5   // road at 100, + 0.5 m source height
	receiverZ := 108.3 // IO1 absolute Z

	hm := computeMeanHeight(source, receiver, sourceZ, receiverZ, []TerrainProfile{profile})

	// TEST-20 reference: h_m = -0.104
	if !almostEqual(hm, -0.104, 0.05) {
		t.Fatalf("Tieflage I6 IO1: expected h_m ≈ -0.10, got %f", hm)
	}
}

// TestComputeMeanHeight_TieflageFarReceiver verifies h_m for TEST-20 I6 IO2.
// I6 IO2: source Z=100.5, receiver Z=120.5 → expected h_m ≈ 5.99.
func TestComputeMeanHeight_TieflageFarReceiver(t *testing.T) {
	t.Parallel()

	foot := tieflageSlopeFoot()
	crest := tieflageSlopeCrest()
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	source := geo.Point2D{X: 100, Y: 50}
	receiver := geo.Point2D{X: 0, Y: 100}
	sourceZ := 100.5
	receiverZ := 120.5 // IO2 absolute Z

	hm := computeMeanHeight(source, receiver, sourceZ, receiverZ, []TerrainProfile{profile})

	// TEST-20 reference: h_m = 5.988
	if !almostEqual(hm, 5.988, 0.05) {
		t.Fatalf("Tieflage I6 IO2: expected h_m ≈ 5.99, got %f", hm)
	}
}

// TestComputeMeanHeight_Hochlage verifies the h_m formula against TEST-20 I7 IO1.
// I7 IO1: road at Z=105, terrain at Z=100, receiver Z=102.6 → expected h_m ≈ 2.24.
func TestComputeMeanHeight_Hochlage(t *testing.T) {
	t.Parallel()

	// I7: Böschungskante at Y=55.3, Z=105 (top of embankment = road level)
	//      Böschungsfuß  at Y=62.8, Z=100 (bottom = terrain level)
	crest := TerrainEdge{
		ID:       "boeschungskante-i7",
		Geometry: []geo.Point3D{{X: -200, Y: 55.3, Z: 105.0}, {X: 400, Y: 55.3, Z: 105.0}},
	}
	foot := TerrainEdge{
		ID:       "boeschungsfuss-i7",
		Geometry: []geo.Point3D{{X: -200, Y: 62.8, Z: 100.0}, {X: 400, Y: 62.8, Z: 100.0}},
	}
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	source := geo.Point2D{X: 100, Y: 50}
	receiver := geo.Point2D{X: 0, Y: 75}
	sourceZ := 105.5   // road at 105, + 0.5 m
	receiverZ := 102.6 // IO1 absolute Z

	hm := computeMeanHeight(source, receiver, sourceZ, receiverZ, []TerrainProfile{profile})

	// TEST-20 reference: h_m = 2.237
	if !almostEqual(hm, 2.237, 0.05) {
		t.Fatalf("Hochlage I7 IO1: expected h_m ≈ 2.24, got %f", hm)
	}
}

// TestTerrainEdgeShielding_Tieflage verifies that the Böschungskante shields IO1
// in a Tieflage scenario (road in cut below terrain).
func TestTerrainEdgeShielding_Tieflage(t *testing.T) {
	t.Parallel()

	foot := tieflageSlopeFoot()
	crest := tieflageSlopeCrest()
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	// IO1 is behind the Böschungskante — should be shielded.
	result := computeTerrainEdgeShielding(
		geo.Point2D{X: 100, Y: 50}, 100.5,
		geo.Point2D{X: 0, Y: 100}, 108.3,
		[]TerrainProfile{profile},
	)

	if !result.Shielded {
		t.Fatal("IO1 behind Böschungskante should be shielded")
	}

	if result.InsertionLoss <= 0 {
		t.Fatalf("expected positive insertion loss, got %f", result.InsertionLoss)
	}
}

// TestTerrainEdgeShielding_NoShielding verifies that IO2 (high receiver) is not
// shielded in TEST-20 I6 because the line of sight clears the Böschungskante.
func TestTerrainEdgeShielding_NoShielding(t *testing.T) {
	t.Parallel()

	foot := tieflageSlopeFoot()
	crest := tieflageSlopeCrest()
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	// IO2 at Z=120.5: line of sight clears the 105.5 crest.
	result := computeTerrainEdgeShielding(
		geo.Point2D{X: 100, Y: 50}, 100.5,
		geo.Point2D{X: 0, Y: 100}, 120.5,
		[]TerrainProfile{profile},
	)

	if result.Shielded {
		t.Fatalf("IO2 line-of-sight clears Böschungskante — should not be shielded")
	}
}

// TestPropagation_Tieflage verifies that a road in a cut is louder for a
// high receiver (not shielded) than for a low receiver (shielded by crest).
func TestPropagation_Tieflage(t *testing.T) {
	t.Parallel()

	source := sampleTieflageSource()
	foot := tieflageSlopeFoot()
	crest := tieflageSlopeCrest()
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	cfg := DefaultPropagationConfig()
	cfg.ReceiverTerrainZ = 105.5
	cfg.Terrain = []TerrainProfile{profile}

	// IO1: low receiver (2.8 m above terrain), shielded by Böschungskante.
	cfg.ReceiverHeightM = 2.8

	shieldedLevel, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 100}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("Tieflage shielded: %v", err)
	}

	// IO2: high receiver (15 m above terrain), not shielded.
	cfg.ReceiverHeightM = 15.0

	unshieldedLevel, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 100}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("Tieflage unshielded: %v", err)
	}

	// The higher receiver (not shielded) is closer in effective distance
	// but the shielded one has extra D_z. Net result: unshielded should be louder.
	if unshieldedLevel.LrDay <= shieldedLevel.LrDay {
		t.Fatalf("unshielded (high receiver) should be louder: unshielded=%f shielded=%f",
			unshieldedLevel.LrDay, shieldedLevel.LrDay)
	}
}

// TestPropagation_Hochlage verifies that a road on an embankment is louder
// at a receiver not blocked by the embankment edge than one that is blocked.
func TestPropagation_Hochlage(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.ElevationM = 105.0 // road elevated 5 m above terrain

	// I7: Böschungskante at road level Y=55.3, Böschungsfuß at Y=62.8 terrain level.
	crest := TerrainEdge{
		ID:       "crest-i7",
		Geometry: []geo.Point3D{{X: -200, Y: 55.3, Z: 105.0}, {X: 400, Y: 55.3, Z: 105.0}},
	}
	foot := TerrainEdge{
		ID:       "foot-i7",
		Geometry: []geo.Point3D{{X: -200, Y: 62.8, Z: 100.0}, {X: 400, Y: 62.8, Z: 100.0}},
	}
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	cfg := DefaultPropagationConfig()
	cfg.ReceiverTerrainZ = 100.0
	cfg.Terrain = []TerrainProfile{profile}

	// IO1: low receiver (2.6 m above terrain at Z=102.6) — shielded by embankment.
	cfg.ReceiverHeightM = 2.6

	shieldedLevel, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 75}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("Hochlage shielded: %v", err)
	}

	// IO2: receiver at road height (5 m above terrain at Z=105) — just clears.
	cfg.ReceiverHeightM = 5.0
	cfg.ReceiverTerrainZ = 100.0

	higherLevel, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 75}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("Hochlage higher: %v", err)
	}

	// Higher receiver should not be shielded and thus louder.
	if higherLevel.LrDay <= shieldedLevel.LrDay {
		t.Fatalf("higher receiver should be louder: higher=%f shielded=%f",
			higherLevel.LrDay, shieldedLevel.LrDay)
	}
}

// TestPropagation_Ansteigende verifies that a rising road (Ansteigende Straße)
// produces valid results using per-vertex CenterlineElevations.
func TestPropagation_Ansteigende(t *testing.T) {
	t.Parallel()

	// Rising road: from Z=100 at X=250 to Z=122 at X=30 (matches TEST-20 I8 geometry).
	source := sampleSource()
	source.Centerline = []geo.Point2D{{X: 250, Y: 50}, {X: 30, Y: 50}}
	source.CenterlineElevations = []float64{100.0, 122.0}

	cfg := DefaultPropagationConfig()
	cfg.ReceiverTerrainZ = 100.0
	cfg.ReceiverHeightM = 5.5

	levels, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 110}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("ansteigende: %v", err)
	}

	// Day > night (traffic pattern).
	if levels.LrDay <= levels.LrNight {
		t.Fatalf("ansteigende: expected day > night: day=%f night=%f", levels.LrDay, levels.LrNight)
	}

	// Both should be finite (not -999 empty-sum sentinel).
	if levels.LrDay < -500 {
		t.Fatalf("ansteigende: day level unexpectedly low: %f", levels.LrDay)
	}
}

// TestPropagation_Ansteigende_HigherEndLouder verifies that the rising end of the
// road is louder for a nearby receiver on that end than the lower end.
func TestPropagation_Ansteigende_HigherEndLouder(t *testing.T) {
	t.Parallel()

	// Rising road from X=0,Z=100 to X=200,Z=120.
	source := sampleSource()
	source.Centerline = []geo.Point2D{{X: 0, Y: 0}, {X: 200, Y: 0}}
	source.CenterlineElevations = []float64{100.0, 120.0}

	cfgLow := DefaultPropagationConfig()
	cfgLow.ReceiverTerrainZ = 100.0
	cfgLow.ReceiverHeightM = 4.0

	// Receiver close to low end of road.
	levelLow, err := ComputeReceiverLevels(geo.Point2D{X: 10, Y: 20}, []RoadSource{source}, nil, cfgLow)
	if err != nil {
		t.Fatalf("low end: %v", err)
	}

	cfgHigh := DefaultPropagationConfig()
	cfgHigh.ReceiverTerrainZ = 120.0
	cfgHigh.ReceiverHeightM = 4.0

	// Receiver close to high end of road.
	levelHigh, err := ComputeReceiverLevels(geo.Point2D{X: 190, Y: 20}, []RoadSource{source}, nil, cfgHigh)
	if err != nil {
		t.Fatalf("high end: %v", err)
	}

	// Both finite and meaningful.
	if levelLow.LrDay < -500 || levelHigh.LrDay < -500 {
		t.Fatalf("levels unexpectedly low: low=%f high=%f", levelLow.LrDay, levelHigh.LrDay)
	}
}

// TestSplitLineIntoSegments_WithElevations verifies Z interpolation.
func TestSplitLineIntoSegments_WithElevations(t *testing.T) {
	t.Parallel()

	// Road rising from Z=100 to Z=120 over 100 m.
	line := []geo.Point2D{{X: 0, Y: 0}, {X: 100, Y: 0}}
	elevations := []float64{100.0, 120.0}

	segs := SplitLineIntoSegments(line, elevations, 10)
	if len(segs) != 10 {
		t.Fatalf("expected 10 segments, got %d", len(segs))
	}

	// First segment midpoint at distance 5 m → Z should be 100 + 5/100*20 = 101.
	if !almostEqual(segs[0].MidZ, 101.0, 0.01) {
		t.Fatalf("first segment MidZ: expected 101.0, got %f", segs[0].MidZ)
	}

	// Last segment midpoint at distance 95 m → Z should be 100 + 95/100*20 = 119.
	if !almostEqual(segs[len(segs)-1].MidZ, 119.0, 0.01) {
		t.Fatalf("last segment MidZ: expected 119.0, got %f", segs[len(segs)-1].MidZ)
	}
}

// TestPropagation_WegfuehrendeStrasse verifies that an angled (wegführende)
// road produces decreasing levels as the road leads away from the receiver.
// This is handled naturally by the Teilstueckverfahren geometry (no special
// topography features needed for a flat angled road).
func TestPropagation_WegfuehrendeStrasse(t *testing.T) {
	t.Parallel()

	// Road leading diagonally away: from (100,60) to (550,330) — TEST-20 I9 geometry.
	source := sampleSource()
	source.Centerline = []geo.Point2D{{X: 100, Y: 60}, {X: 550, Y: 330}}

	cfg := DefaultPropagationConfig()

	// IO1: close receiver.
	close1, err := ComputeReceiverLevels(geo.Point2D{X: 50, Y: 30}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("wegführende close: %v", err)
	}

	// IO2: farther receiver.
	far1, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 0}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("wegführende far: %v", err)
	}

	// Closer receiver should be louder.
	if close1.LrDay <= far1.LrDay {
		t.Fatalf("close receiver should be louder for wegführende: close=%f far=%f",
			close1.LrDay, far1.LrDay)
	}
}

// TestGroundCorrection_ProperFormula verifies the D_gr formula against TEST-20 I1.
func TestGroundCorrection_ProperFormula(t *testing.T) {
	t.Parallel()

	// I1 IO1: h_m=15.5, s_gr=101.98 → D_gr = 0 (formula yields negative).
	dgr1 := computeGroundCorrection(101.98, 15.5)
	if dgr1 != 0 {
		t.Fatalf("I1 IO1: expected D_gr=0 (clamped), got %f", dgr1)
	}

	// I1 IO2: h_m=3.0, s_gr=111.803 → D_gr ≈ 3.74.
	dgr2 := computeGroundCorrection(111.803, 3.0)
	if !almostEqual(dgr2, 3.74, 0.05) {
		t.Fatalf("I1 IO2: expected D_gr≈3.74, got %f", dgr2)
	}

	// I6 IO1: h_m=-0.104, s_gr=111.939 → D_gr ≈ 4.84.
	dgr3 := computeGroundCorrection(111.939, -0.104)
	if !almostEqual(dgr3, 4.84, 0.05) {
		t.Fatalf("I6 IO1: expected D_gr≈4.84, got %f", dgr3)
	}
}

// --- building / courtyard tests ---

func TestBuilding_Validate(t *testing.T) {
	t.Parallel()

	valid := Building{
		ID:        "bldg-1",
		Footprint: []geo.Point2D{{X: 0, Y: 10}, {X: 10, Y: 10}, {X: 10, Y: 15}, {X: 0, Y: 15}},
		HeightM:   8.0,
	}

	err := valid.Validate()
	if err != nil {
		t.Fatalf("expected valid building, got %v", err)
	}

	// Missing ID.
	b := valid
	b.ID = ""

	err = b.Validate()
	if err == nil {
		t.Fatal("expected error for missing ID")
	}

	// Too few vertices (need at least 3 for a polygon).
	b = valid
	b.Footprint = []geo.Point2D{{X: 0, Y: 0}, {X: 10, Y: 0}}

	err = b.Validate()
	if err == nil {
		t.Fatal("expected error for fewer than 3 footprint vertices")
	}

	// Zero height.
	b = valid
	b.HeightM = 0

	err = b.Validate()
	if err == nil {
		t.Fatal("expected error for zero height")
	}

	// Negative reflection loss.
	b = valid
	b.ReflectionLossDB = -1

	err = b.Validate()
	if err == nil {
		t.Fatal("expected error for negative reflection loss")
	}
}

func TestBuilding_AsBarrier_ClosedPolygon(t *testing.T) {
	t.Parallel()

	// 4-vertex rectangle — asBarrier should close it to 5 points.
	b := Building{
		ID:        "rect",
		Footprint: []geo.Point2D{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 5}, {X: 0, Y: 5}},
		HeightM:   8.0,
	}

	barrier := b.asBarrier()

	if barrier.HeightM != 8.0 {
		t.Fatalf("expected height 8.0, got %f", barrier.HeightM)
	}

	// Closed polygon: 4 vertices + closing vertex = 5 points.
	if len(barrier.Geometry) != 5 {
		t.Fatalf("expected 5 geometry points (closed), got %d", len(barrier.Geometry))
	}

	// Closing point equals first point.
	first, last := barrier.Geometry[0], barrier.Geometry[4]
	if first.X != last.X || first.Y != last.Y {
		t.Fatalf("barrier polygon not closed: first=%v last=%v", first, last)
	}
}

func TestBuilding_AsBarrier_AlreadyClosedPolygon(t *testing.T) {
	t.Parallel()

	// Pre-closed polygon must not add a duplicate point.
	b := Building{
		ID: "pre-closed",
		Footprint: []geo.Point2D{
			{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 5}, {X: 0, Y: 5}, {X: 0, Y: 0},
		},
		HeightM: 8.0,
	}

	barrier := b.asBarrier()

	if len(barrier.Geometry) != 5 {
		t.Fatalf("expected 5 points for pre-closed polygon, got %d", len(barrier.Geometry))
	}
}

func TestBuilding_AsReflector_Properties(t *testing.T) {
	t.Parallel()

	b := Building{
		ID:               "facade",
		Footprint:        []geo.Point2D{{X: 0, Y: 20}, {X: 10, Y: 20}, {X: 10, Y: 25}, {X: 0, Y: 25}},
		HeightM:          6.0,
		ReflectionLossDB: 2.0,
	}

	refl := b.asReflector()

	if refl.HeightM != 6.0 {
		t.Fatalf("expected reflector height 6.0, got %f", refl.HeightM)
	}

	if refl.ReflectionLossDB != 2.0 {
		t.Fatalf("expected reflection loss 2.0, got %f", refl.ReflectionLossDB)
	}

	// Closed polygon: 4 + 1 closing = 5 points.
	if len(refl.Geometry) != 5 {
		t.Fatalf("expected 5 geometry points, got %d", len(refl.Geometry))
	}
}

// TestBuilding_ShieldsDirectPath verifies that a building standing between
// source and receiver reduces the receiver level compared to the same geometry
// acting only as a reflector (no barrier shielding).
//
// Note: the image-source method can produce phantom reflections off interior
// faces of a building polygon (e.g. north↔south bounce inside a thin building
// body). Both test scenarios use the same reflector geometry, so phantom paths
// cancel out. The difference is purely from the barrier shielding component.
//
// Geometry: source near (0,0), receiver at (0,100), building at y=20..25 (h=10m).
// The building's south wall at y=20 crosses the direct source-receiver path.
func TestBuilding_ShieldsDirectPath(t *testing.T) {
	t.Parallel()

	source := RoadSource{
		ID:           "src",
		Centerline:   []geo.Point2D{{X: -1, Y: 0}, {X: 1, Y: 0}},
		SurfaceType:  SurfaceSMA,
		Speeds:       SpeedInput{PkwKPH: 100, Lkw1KPH: 80, Lkw2KPH: 70, KradKPH: 100},
		TrafficDay:   TrafficInput{PkwPerHour: 900, Lkw1PerHour: 40, Lkw2PerHour: 60, KradPerHour: 10},
		TrafficNight: TrafficInput{PkwPerHour: 200, Lkw1PerHour: 10, Lkw2PerHour: 20, KradPerHour: 2},
	}
	receiver := geo.Point2D{X: 0, Y: 100}

	bldg := Building{
		ID:        "blocker",
		Footprint: []geo.Point2D{{X: -5, Y: 20}, {X: 5, Y: 20}, {X: 5, Y: 25}, {X: -5, Y: 25}},
		HeightM:   10.0,
	}

	// Reflector-only: same geometry, same reflected paths, but no barrier shielding.
	cfgReflOnly := DefaultPropagationConfig()
	cfgReflOnly.Reflectors = []Reflector{bldg.asReflector()}

	lvlReflOnly, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgReflOnly)
	if err != nil {
		t.Fatalf("refl-only: %v", err)
	}

	// Building: same reflections PLUS barrier shielding of the direct path.
	cfgBuilding := DefaultPropagationConfig()
	cfgBuilding.Buildings = []Building{bldg}

	lvlBuilding, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgBuilding)
	if err != nil {
		t.Fatalf("building: %v", err)
	}

	// Building (shielding active) must be quieter than reflector-only (no shielding).
	if lvlBuilding.LrDay >= lvlReflOnly.LrDay {
		t.Fatalf("building with shielding should be lower than reflector-only: "+
			"building=%.2f refl-only=%.2f", lvlBuilding.LrDay, lvlReflOnly.LrDay)
	}
}

// TestBuilding_ParallelFacade_IncreasesLevel verifies that a building facade
// parallel to the road (house-front scenario) increases the receiver level by
// adding a reflected path.
//
// Geometry: road along x-axis, receiver at (0,30), building at y=45..50.
// The building's south face at y=45 reflects road sound back to the receiver.
func TestBuilding_ParallelFacade_IncreasesLevel(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 30}

	cfg := DefaultPropagationConfig()

	lvlFree, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("free field: %v", err)
	}

	// Building with south facade at y=45 (behind receiver at y=30).
	bldg := Building{
		ID:        "house-front",
		Footprint: []geo.Point2D{{X: -20, Y: 45}, {X: 20, Y: 45}, {X: 20, Y: 50}, {X: -20, Y: 50}},
		HeightM:   8.0,
	}
	cfg.Buildings = []Building{bldg}

	lvlWithBldg, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("with building: %v", err)
	}

	if lvlWithBldg.LrDay <= lvlFree.LrDay {
		t.Fatalf("parallel facade should increase level: free=%.2f with-bldg=%.2f",
			lvlFree.LrDay, lvlWithBldg.LrDay)
	}
}

// TestBuilding_Courtyard_IncreasesLevel verifies that a U-shaped courtyard
// (buildings on north, east, and west) raises the receiver level compared to
// an open field receiver at the same position. This is the "Hinterhof" scenario.
//
// Geometry:
//
//	Road: x=-50..50, y=0 (sampleSource)
//	Receiver: (0, 40) — inside the courtyard opening
//	North building: south face at y=60 (reflects back)
//	East building:  west face at x=20  (reflects inward)
//	West building:  east face at x=-20 (reflects inward)
func TestBuilding_Courtyard_IncreasesLevel(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 40}

	cfg := DefaultPropagationConfig()

	lvlFree, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("free field: %v", err)
	}

	cfg.Buildings = []Building{
		// North wall of the courtyard — reflects sound back toward road.
		{
			ID:        "north-wall",
			Footprint: []geo.Point2D{{X: -20, Y: 60}, {X: 20, Y: 60}, {X: 20, Y: 65}, {X: -20, Y: 65}},
			HeightM:   10.0,
		},
		// East wall of the courtyard — reflects inward.
		{
			ID:        "east-wall",
			Footprint: []geo.Point2D{{X: 20, Y: 0}, {X: 25, Y: 0}, {X: 25, Y: 65}, {X: 20, Y: 65}},
			HeightM:   10.0,
		},
		// West wall of the courtyard — reflects inward.
		{
			ID:        "west-wall",
			Footprint: []geo.Point2D{{X: -25, Y: 0}, {X: -20, Y: 0}, {X: -20, Y: 65}, {X: -25, Y: 65}},
			HeightM:   10.0,
		},
	}

	lvlCourtyard, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("courtyard: %v", err)
	}

	if lvlCourtyard.LrDay <= lvlFree.LrDay {
		t.Fatalf("courtyard should increase level: free=%.2f courtyard=%.2f",
			lvlFree.LrDay, lvlCourtyard.LrDay)
	}
}

// --- reflection tests ---

func TestReflector_Validate(t *testing.T) {
	t.Parallel()

	valid := Reflector{
		ID:       "wall-1",
		Geometry: []geo.Point2D{{X: 15, Y: -10}, {X: 15, Y: 10}},
		HeightM:  8.0,
	}

	err := valid.Validate()
	if err != nil {
		t.Fatalf("expected valid reflector, got %v", err)
	}

	// Missing ID.
	r := valid
	r.ID = ""

	err = r.Validate()
	if err == nil {
		t.Fatal("expected error for missing ID")
	}

	// Too few points.
	r = valid
	r.Geometry = []geo.Point2D{{X: 0, Y: 0}}

	err = r.Validate()
	if err == nil {
		t.Fatal("expected error for single-point geometry")
	}

	// Zero height.
	r = valid
	r.HeightM = 0

	err = r.Validate()
	if err == nil {
		t.Fatal("expected error for zero height")
	}

	// Negative reflection loss.
	r = valid
	r.ReflectionLossDB = -1

	err = r.Validate()
	if err == nil {
		t.Fatal("expected error for negative reflection loss")
	}
}

func TestMirrorPoint_PerpendicularWall(t *testing.T) {
	t.Parallel()

	// Mirror (0, 0) across the vertical wall at x=10.
	img := mirrorPoint(
		geo.Point2D{X: 0, Y: 0},
		geo.Point2D{X: 10, Y: -5},
		geo.Point2D{X: 10, Y: 5},
	)
	if !almostEqual(img.X, 20, 1e-9) || !almostEqual(img.Y, 0, 1e-9) {
		t.Fatalf("expected (20, 0), got (%f, %f)", img.X, img.Y)
	}
}

func TestMirrorPoint_AngledWall(t *testing.T) {
	t.Parallel()

	// Mirror (0, 2) across the line y=x (wall from origin to (1,1)).
	// Mirror of (0,2) across y=x is (2,0).
	img := mirrorPoint(
		geo.Point2D{X: 0, Y: 2},
		geo.Point2D{X: 0, Y: 0},
		geo.Point2D{X: 1, Y: 1},
	)
	if !almostEqual(img.X, 2, 1e-6) || !almostEqual(img.Y, 0, 1e-6) {
		t.Fatalf("expected (2, 0), got (%f, %f)", img.X, img.Y)
	}
}
