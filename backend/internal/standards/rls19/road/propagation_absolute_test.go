package road

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// singleSegmentSource returns a straight Pkw-only road of the given length,
// centred on the origin and lying on the X axis at elevation 0.
//
// The traffic composition is deliberately reduced to a single vehicle group so
// that the emission level has a short closed form (see the derivation in
// TestPropagation_AbsoluteLevelMatchesClosedForm).
func singleSegmentSource(lengthM float64) RoadSource {
	return RoadSource{
		ID:          "road-abs",
		SurfaceType: SurfaceSMA,
		Centerline: []geo.Point2D{
			{X: -lengthM / 2, Y: 0},
			{X: lengthM / 2, Y: 0},
		},
		Speeds:       SpeedInput{PkwKPH: 100, Lkw1KPH: 100, Lkw2KPH: 100, KradKPH: 100},
		TrafficDay:   TrafficInput{PkwPerHour: 1000},
		TrafficNight: TrafficInput{PkwPerHour: 1000},
	}
}

// TestPropagation_AbsoluteLevelMatchesClosedForm pins an absolute immission
// level that is derived by hand from the RLS-19 formulas, independently of the
// implementation. It is the regression guard for the "line-source contributions
// normalised by the total road length" defect: under that bug the length
// weighting collapsed to 10·lg(l_i / l_total), the road radiated the power of a
// single 1 m section, and this receiver read 35.23 dB(A) instead of 41.25.
//
// Scenario
//
//	road:      4 m long, centred on the origin, on the X axis, elevation 0
//	traffic:   1000 Pkw/h, v = 100 km/h, surface SMA, no gradient, no junction,
//	           no multiple-reflection surcharge
//	receiver:  (0, 100), 4 m above ground, terrain Z = 0
//	config:    segment_length_m = 4  ->  exactly one sub-segment, midpoint (0,0)
//	           min_distance_m = 3, no barriers, no reflectors, no terrain
//
// Derivation
//
//	E1  base emission (Eq. 6), v = 100 km/h is inside [30, 130], no clamping:
//	    L_W0,Pkw = 88.0 + 10·lg(1 + (100/20)^3.06)              = 109.419914 dB(A)
//	E2  surface correction, Table 4, SMA, Pkw, v > 60 km/h:
//	    D_StrO   = −1.8 dB
//	E3  gradient correction, 0 %:                                =   0 dB
//	E4  junction correction, none:                               =   0 dB
//	E5  multiple-reflection surcharge, no street canyon given:   =   0 dB
//	E6  L_WA   = 109.419914 − 1.8                                = 107.619914 dB(A)
//	E7  length-related emission level (Eq. 4), single group:
//	    L_m,E  = 10·lg(M · 10^(L_WA/10) / v) − 30
//	           = L_WA + 10·lg(1000/100) − 30
//	           = 107.619914 + 10 − 30                            =  87.619914 dB(A)/m
//
//	Geometry of the single sub-segment:
//	    source Z   = 0 (road surface) + 0.5 (RLS-19 source height)  = 0.5 m
//	    receiver Z = 0 (terrain) + 4 (height above ground)          = 4.0 m
//	    s_gr (plan)  = 100 m
//	    s    (slant) = sqrt(100² + 3.5²)                        = 100.061231 m
//
//	Propagation terms:
//	    D_div = 20·lg(s) + 10·lg(2π)                            =  47.987116 dB
//	    D_atm = 5.0 dB/km · s/1000                              =   0.500306 dB
//	    h_m   = (0.5 + 4.0)/2 − 0                               =   2.25 m
//	    D_gr  = 4.8 − (2·h_m/s_gr)·(17 + 300/s_gr)
//	          = 4.8 − 0.045 · 20                                =   3.900000 dB
//
//	Length weighting (Eq. 10), reference length l_0 = 1 m:
//	    10·lg(l/l_0) = 10·lg(4)                                 =   6.020600 dB
//
//	L_r = L_m,E + 10·lg(l/l_0) − (D_div + D_atm + D_gr)
//	    = 87.619914 + 6.020600 − 52.387422                      =  41.253092 dB(A)
func TestPropagation_AbsoluteLevelMatchesClosedForm(t *testing.T) {
	t.Parallel()

	const expectedLrDB = 41.253092

	cfg := PropagationConfig{
		SegmentLengthM:  4.0,
		MinDistanceM:    3.0,
		ReceiverHeightM: 4.0,
	}

	levels, err := ComputeReceiverLevels(
		geo.Point2D{X: 0, Y: 100},
		[]RoadSource{singleSegmentSource(4.0)},
		nil,
		cfg,
	)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}

	if !almostEqual(levels.LrDay, expectedLrDB, 1e-4) {
		t.Errorf("LrDay = %.6f dB(A), want %.6f dB(A) from the RLS-19 closed form",
			levels.LrDay, expectedLrDB)
	}

	// Day and night traffic are identical in this scenario, so both periods must
	// land on the same absolute level.
	if !almostEqual(levels.LrNight, expectedLrDB, 1e-4) {
		t.Errorf("LrNight = %.6f dB(A), want %.6f dB(A) from the RLS-19 closed form",
			levels.LrNight, expectedLrDB)
	}
}

// TestPropagation_AbsoluteLevelScalesWithRoadLength verifies that the emitted
// power is proportional to the road length. Both roads are collapsed into a
// single sub-segment with the same midpoint, so every propagation term is
// identical and the level difference must be exactly 10·lg(l2/l1).
//
// Under the total-length normalisation defect both roads produced the identical
// level, because the weighting summed to unity regardless of road length.
func TestPropagation_AbsoluteLevelScalesWithRoadLength(t *testing.T) {
	t.Parallel()

	receiver := geo.Point2D{X: 0, Y: 100}

	shortCfg := PropagationConfig{SegmentLengthM: 4.0, MinDistanceM: 3.0, ReceiverHeightM: 4.0}
	longCfg := PropagationConfig{SegmentLengthM: 40.0, MinDistanceM: 3.0, ReceiverHeightM: 4.0}

	shortLevels, err := ComputeReceiverLevels(receiver, []RoadSource{singleSegmentSource(4.0)}, nil, shortCfg)
	if err != nil {
		t.Fatalf("compute 4 m road: %v", err)
	}

	longLevels, err := ComputeReceiverLevels(receiver, []RoadSource{singleSegmentSource(40.0)}, nil, longCfg)
	if err != nil {
		t.Fatalf("compute 40 m road: %v", err)
	}

	const wantDelta = 10.0 // 10·lg(40/4)

	delta := longLevels.LrDay - shortLevels.LrDay
	if !almostEqual(delta, wantDelta, 1e-6) {
		t.Errorf("level difference for a 10x longer road = %.6f dB, want %.6f dB", delta, wantDelta)
	}
}

// TestPropagation_LevelInvariantUnderSourceSplitting checks that chopping one
// road source into two abutting halves does not change the result. The emitted
// power must depend on the road geometry only, never on how the user chose to
// split the input features.
//
// Under the total-length normalisation defect each half was renormalised to its
// own total length, so the split geometry gained ~3 dB.
func TestPropagation_LevelInvariantUnderSourceSplitting(t *testing.T) {
	t.Parallel()

	receiver := geo.Point2D{X: 0, Y: 50}
	cfg := DefaultPropagationConfig()

	whole := singleSegmentSource(200.0)

	left := whole
	left.ID = "road-abs-left"
	left.Centerline = []geo.Point2D{{X: -100, Y: 0}, {X: 0, Y: 0}}

	right := whole
	right.ID = "road-abs-right"
	right.Centerline = []geo.Point2D{{X: 0, Y: 0}, {X: 100, Y: 0}}

	single, err := ComputeReceiverLevels(receiver, []RoadSource{whole}, nil, cfg)
	if err != nil {
		t.Fatalf("compute single source: %v", err)
	}

	split, err := ComputeReceiverLevels(receiver, []RoadSource{left, right}, nil, cfg)
	if err != nil {
		t.Fatalf("compute split sources: %v", err)
	}

	if !almostEqual(single.LrDay, split.LrDay, 1e-6) {
		t.Errorf("splitting one 200 m source into two 100 m sources changed the level: single=%.6f split=%.6f",
			single.LrDay, split.LrDay)
	}
}

// TestPropagation_ReflectedPathUsesSameLengthWeighting pins the reflected path
// to the same 10·lg(l/l_0) weighting as the direct path. With a reflector in
// place the receiver level is an energy sum of the direct and the mirrored
// contribution; both carry the same length weight, so a 10x longer road must
// still shift the total by exactly 10 dB. A reflected path that kept the old
// normalisation would break that exact relation.
func TestPropagation_ReflectedPathUsesSameLengthWeighting(t *testing.T) {
	t.Parallel()

	receiver := geo.Point2D{X: 0, Y: 100}

	reflector := Reflector{
		ID:       "back-wall",
		Geometry: []geo.Point2D{{X: -400, Y: -20}, {X: 400, Y: -20}},
		HeightM:  12,
	}

	shortCfg := PropagationConfig{
		SegmentLengthM: 4.0, MinDistanceM: 3.0, ReceiverHeightM: 4.0,
		Reflectors: []Reflector{reflector},
	}
	longCfg := PropagationConfig{
		SegmentLengthM: 40.0, MinDistanceM: 3.0, ReceiverHeightM: 4.0,
		Reflectors: []Reflector{reflector},
	}

	shortLevels, err := ComputeReceiverLevels(receiver, []RoadSource{singleSegmentSource(4.0)}, nil, shortCfg)
	if err != nil {
		t.Fatalf("compute 4 m road with reflector: %v", err)
	}

	longLevels, err := ComputeReceiverLevels(receiver, []RoadSource{singleSegmentSource(40.0)}, nil, longCfg)
	if err != nil {
		t.Fatalf("compute 40 m road with reflector: %v", err)
	}

	// Guard: the reflector must actually contribute, otherwise this test would
	// silently degrade into a duplicate of the direct-path scaling test.
	noRefl := PropagationConfig{SegmentLengthM: 4.0, MinDistanceM: 3.0, ReceiverHeightM: 4.0}

	baseline, err := ComputeReceiverLevels(receiver, []RoadSource{singleSegmentSource(4.0)}, nil, noRefl)
	if err != nil {
		t.Fatalf("compute 4 m road without reflector: %v", err)
	}

	if shortLevels.LrDay <= baseline.LrDay+1e-9 {
		t.Fatalf("reflector did not contribute: with=%.6f without=%.6f", shortLevels.LrDay, baseline.LrDay)
	}

	const wantDelta = 10.0 // 10·lg(40/4)

	delta := longLevels.LrDay - shortLevels.LrDay
	if !almostEqual(delta, wantDelta, 1e-6) {
		t.Errorf("reflected-path level difference for a 10x longer road = %.6f dB, want %.6f dB", delta, wantDelta)
	}
}

// TestPropagation_AbsoluteLevelSanityForTypicalRoad is a coarse plausibility
// bound rather than a closed form: a busy 1 km road at 25 m must land in the
// 60-75 dB(A) range that RLS-19 practice expects. The defect put this scenario
// near 40 dB(A), which no relational test could catch.
func TestPropagation_AbsoluteLevelSanityForTypicalRoad(t *testing.T) {
	t.Parallel()

	source := singleSegmentSource(1000.0)
	cfg := DefaultPropagationConfig()

	levels, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 25}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}

	if levels.LrDay < 60 || levels.LrDay > 75 {
		t.Errorf("LrDay = %.2f dB(A) for 1000 Pkw/h at 25 m, expected a plausible 60-75 dB(A)", levels.LrDay)
	}
}

// TestPropagation_LengthWeightUsesUnitReferenceLength documents the reference
// length directly, so a future change of referenceLengthM cannot silently
// reintroduce a road-length-dependent normalisation.
func TestPropagation_LengthWeightUsesUnitReferenceLength(t *testing.T) {
	t.Parallel()

	if referenceLengthM != 1.0 {
		t.Fatalf("referenceLengthM = %v, want 1.0 m (RLS-19 l_0)", referenceLengthM)
	}

	// 10·lg(l/l_0) for a 1 m segment must vanish.
	if got := 10 * math.Log10(1.0/referenceLengthM); got != 0 {
		t.Fatalf("length weight for a 1 m segment = %v, want 0", got)
	}
}
