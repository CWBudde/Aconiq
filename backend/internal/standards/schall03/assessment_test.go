package schall03

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// ---------------------------------------------------------------------------
// Gl. 33: Beurteilungspegel
// ---------------------------------------------------------------------------

func TestBeurteilungspegelGl33(t *testing.T) {
	t.Parallel()

	// L_r,T = L_pAeq,T + K_S (Gl. 33).
	// K_S = 0 for Eisenbahnen since the 2015 amendment to 16. BImSchV.
	lr := beurteilungspegel(65.3, 0.0)
	if lr != 65.3 {
		t.Errorf("expected 65.3, got %v", lr)
	}
}

func TestSchienenbonus(t *testing.T) {
	t.Parallel()

	lpAeq := 70.0

	lrDefault := beurteilungspegel(lpAeq, 0.0)
	if lrDefault != 70.0 {
		t.Errorf("K_S=0: expected 70.0, got %v", lrDefault)
	}

	// Historical K_S = -5 dB (abolished but still testable)
	lrHistorical := beurteilungspegel(lpAeq, -5.0)
	if math.Abs(lrHistorical-65.0) > 0.001 {
		t.Errorf("K_S=-5: expected 65.0, got %v", lrHistorical)
	}
}

// ---------------------------------------------------------------------------
// Rounding to whole dB
// ---------------------------------------------------------------------------

func TestBeurteilungspegelRounding(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input float64
		want  float64
	}{
		{65.3, 65.0},
		{65.5, 66.0},
		{65.8, 66.0},
		{65.0, 65.0},
		{64.49, 64.0},
		{-65.6, -66.0},
		{-65.3, -65.0},
	}

	for _, tc := range cases {
		got := roundToWholeDB(tc.input)
		if math.Abs(got-tc.want) > 0.001 {
			t.Errorf("roundToWholeDB(%v): expected %v, got %v", tc.input, tc.want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// ComputeNormativeReceiverLevels: plausibility test
// ---------------------------------------------------------------------------

func TestNormativePipelineProducesFiniteLevels(t *testing.T) {
	t.Parallel()

	// Simple straight track, single train type.
	op, err := NewTrainOperationFromZugart("IC-Zug-E-Lok", 8.0, 2.0)
	if err != nil {
		t.Fatal(err)
	}

	seg := TrackSegment{
		ID: "seg1",
		TrackCenterline: []geo.Point2D{
			{X: 0, Y: 0},
			{X: 100, Y: 0},
		},
		ElevationM:    0,
		Fahrbahn:      FahrbahnartSchwellengleis,
		Surface:       SurfaceCondNone,
		BridgeType:    0,
		CurveRadiusM:  0,
		IsStation:     false,
		StreckeMaxKPH: 200,
		Operations:    []TrainOperation{*op},
	}

	receiver := ReceiverInput{
		ID:      "r1",
		Point:   geo.Point2D{X: 50, Y: 25},
		HeightM: 4.0,
	}

	levels, err := ComputeNormativeReceiverLevels(receiver, []TrackSegment{seg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if math.IsNaN(levels.LpAeqDay) || math.IsInf(levels.LpAeqDay, 0) {
		t.Errorf("LpAeqDay is not finite: %v", levels.LpAeqDay)
	}

	if math.IsNaN(levels.LpAeqNight) || math.IsInf(levels.LpAeqNight, 0) {
		t.Errorf("LpAeqNight is not finite: %v", levels.LpAeqNight)
	}

	// Day should be louder than night (more trains per hour).
	if levels.LpAeqDay < levels.LpAeqNight {
		t.Errorf("day level %v should be >= night level %v", levels.LpAeqDay, levels.LpAeqNight)
	}

	// Levels should be physically plausible: roughly 40-90 dB(A) at 25m.
	if levels.LpAeqDay < 30 || levels.LpAeqDay > 100 {
		t.Errorf("LpAeqDay=%v is outside plausible range [30, 100] dB(A)", levels.LpAeqDay)
	}
}

func TestNormativePipelineRequiresSegments(t *testing.T) {
	t.Parallel()

	receiver := ReceiverInput{
		ID:      "r1",
		Point:   geo.Point2D{X: 0, Y: 25},
		HeightM: 4.0,
	}

	_, err := ComputeNormativeReceiverLevels(receiver, nil)
	if err == nil {
		t.Error("expected error for empty segments")
	}
}

// ---------------------------------------------------------------------------
// Gl. 13: A_gr = A_gr,B + A_gr,W — water body ground correction wired in
// ---------------------------------------------------------------------------

func TestNormativePipelineWaterBodyRaisesLevels(t *testing.T) {
	t.Parallel()

	// A_gr,W = -3·d_w/d_p is negative → subtracting a negative from the
	// attenuation chain means less total attenuation → higher levels.
	// A receiver over fully over water (fraction=1.0) must produce a higher
	// level than the same geometry over pure land (fraction=0.0).
	op, err := NewTrainOperationFromZugart("IC-Zug-E-Lok", 4.0, 1.0)
	if err != nil {
		t.Fatal(err)
	}

	makeSeg := func(waterFrac float64) TrackSegment {
		return TrackSegment{
			ID: "seg",
			TrackCenterline: []geo.Point2D{
				{X: 0, Y: 0},
				{X: 200, Y: 0},
			},
			ElevationM:         0,
			Fahrbahn:           FahrbahnartSchwellengleis,
			Surface:            SurfaceCondNone,
			BridgeType:         0,
			CurveRadiusM:       0,
			StreckeMaxKPH:      200,
			WaterBodyFractionW: waterFrac,
			Operations:         []TrainOperation{*op},
		}
	}

	receiver := ReceiverInput{ID: "r", Point: geo.Point2D{X: 100, Y: 50}, HeightM: 4.0}

	lvlLand, err := ComputeNormativeReceiverLevels(receiver, []TrackSegment{makeSeg(0.0)})
	if err != nil {
		t.Fatal(err)
	}

	lvlWater, err := ComputeNormativeReceiverLevels(receiver, []TrackSegment{makeSeg(1.0)})
	if err != nil {
		t.Fatal(err)
	}

	// Full water path: A_gr,W = -3 dB, A_gr,B = 0 (land fraction=0).
	// A_gr = -3 dB means less attenuation → higher level.
	if lvlWater.LpAeqDay <= lvlLand.LpAeqDay {
		t.Errorf("full-water path (%v dB) should be louder than pure-land path (%v dB)",
			lvlWater.LpAeqDay, lvlLand.LpAeqDay)
	}
}

func TestNormativePipelineWaterBodyValidation(t *testing.T) {
	t.Parallel()

	op, err := NewTrainOperationFromZugart("IC-Zug-E-Lok", 4.0, 1.0)
	if err != nil {
		t.Fatal(err)
	}

	seg := TrackSegment{
		ID:                 "seg",
		TrackCenterline:    []geo.Point2D{{X: 0, Y: 0}, {X: 100, Y: 0}},
		ElevationM:         0,
		Fahrbahn:           FahrbahnartSchwellengleis,
		Surface:            SurfaceCondNone,
		StreckeMaxKPH:      100,
		WaterBodyFractionW: 1.5, // out of range
		Operations:         []TrainOperation{*op},
	}

	receiver := ReceiverInput{ID: "r", Point: geo.Point2D{X: 50, Y: 25}, HeightM: 4.0}

	_, err = ComputeNormativeReceiverLevels(receiver, []TrackSegment{seg})
	if err == nil {
		t.Error("expected validation error for WaterBodyFractionW > 1")
	}
}

func TestNormativePipelineWaterBodyCloserReceiverLouder(t *testing.T) {
	t.Parallel()

	op, err := NewTrainOperationFromZugart("Gueterzug-E-Lok", 4.0, 2.0)
	if err != nil {
		t.Fatal(err)
	}

	seg := TrackSegment{
		ID: "seg1",
		TrackCenterline: []geo.Point2D{
			{X: 0, Y: 0},
			{X: 200, Y: 0},
		},
		ElevationM:    0,
		Fahrbahn:      FahrbahnartSchwellengleis,
		Surface:       SurfaceCondNone,
		BridgeType:    0,
		CurveRadiusM:  0,
		IsStation:     false,
		StreckeMaxKPH: 100,
		Operations:    []TrainOperation{*op},
	}

	near := ReceiverInput{ID: "near", Point: geo.Point2D{X: 100, Y: 10}, HeightM: 4.0}
	far := ReceiverInput{ID: "far", Point: geo.Point2D{X: 100, Y: 100}, HeightM: 4.0}

	lvlNear, err := ComputeNormativeReceiverLevels(near, []TrackSegment{seg})
	if err != nil {
		t.Fatal(err)
	}

	lvlFar, err := ComputeNormativeReceiverLevels(far, []TrackSegment{seg})
	if err != nil {
		t.Fatal(err)
	}

	if lvlNear.LpAeqDay <= lvlFar.LpAeqDay {
		t.Errorf("near receiver (%v dB) should be louder than far (%v dB)", lvlNear.LpAeqDay, lvlFar.LpAeqDay)
	}
}

// ---------------------------------------------------------------------------
// Gl. 35-36: ComputeCombinedBeurteilungspegel — Rangierbahnhof + Strecken
// ---------------------------------------------------------------------------

func TestCombinedAssessmentGl35YardDominant(t *testing.T) {
	t.Parallel()

	// When yard contribution dominates, result ≈ yard level.
	// Yard day=80 dB, Strecke day=-100 dB (negligible)
	lrTag, lrNacht := ComputeCombinedBeurteilungspegel(80, 10, -100, -100)
	if math.Abs(lrTag-80) > 0.1 {
		t.Errorf("yard-dominant day: expected ~80, got %g", lrTag)
	}

	if math.Abs(lrNacht-10) > 0.1 {
		t.Errorf("yard-dominant night: expected ~10, got %g", lrNacht)
	}
}

func TestCombinedAssessmentGl35EqualContributions(t *testing.T) {
	t.Parallel()

	// Yard=70, Strecke=70 (K_S=0 → 70). Equal contributions sum to 73.01 dB.
	// L_r = 10·lg(10^7 + 10^7) = 10·lg(2·10^7) = 73.01
	lrTag, _ := ComputeCombinedBeurteilungspegel(70, 0, 70, 0)
	if math.Abs(lrTag-73.01) > 0.1 {
		t.Errorf("equal contributions: expected ~73.01, got %g", lrTag)
	}
}

func TestCombinedAssessmentGl35KSAppliedOnlyToStrecken(t *testing.T) {
	t.Parallel()

	// K_S=0 (abolished), so Strecke term passes through unchanged.
	// Yard negligible (-200), Strecke=55 → result ≈ 55 dB.
	lrTag, _ := ComputeCombinedBeurteilungspegel(-200, -200, 55, -200)
	if math.Abs(lrTag-55) > 0.2 {
		t.Errorf("K_S on Strecke only: expected ~55, got %g", lrTag)
	}
}

func TestCombinedBeurteilungspegel_KS_Abolished(t *testing.T) {
	t.Parallel()

	// With K_S=0 (Schienenbonus abolished), combining yard=70 dB and
	// strecke=70 dB should be a pure energetic sum with no -5 dB bonus
	// on the strecke part.
	//
	// Expected: L_r = 10·lg(10^7 + 10^7) = 10·lg(2·10^7) ≈ 73.01 dB
	wantTag := 10 * math.Log10(math.Pow(10, 0.1*70)+math.Pow(10, 0.1*70))

	lrTag, lrNacht := ComputeCombinedBeurteilungspegel(70, 70, 70, 70)

	if math.Abs(lrTag-wantTag) > 0.01 {
		t.Errorf("day: expected pure energetic sum ≈ %g, got %g", wantTag, lrTag)
	}

	if math.Abs(lrNacht-wantTag) > 0.01 {
		t.Errorf("night: expected pure energetic sum ≈ %g, got %g", wantTag, lrNacht)
	}

	// Ensure the old K_S=-5 value is NOT applied: if it were, the strecke
	// contribution would be 65 dB and the result would be ~71.19 dB.
	wrongResult := 10 * math.Log10(math.Pow(10, 0.1*70)+math.Pow(10, 0.1*65))
	if math.Abs(lrTag-wrongResult) < 0.5 {
		t.Errorf("day result %g matches old K_S=-5 value %g — Schienenbonus not abolished", lrTag, wrongResult)
	}
}
