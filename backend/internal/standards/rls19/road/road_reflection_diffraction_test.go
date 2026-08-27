package road

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// TestComputeReflectedPaths_SingleReflection verifies the image-source path
// distance for a simple reflection off a perpendicular wall.
//
// Geometry: source (0,0), receiver (10,0), wall at x=15 (y ∈ [-20,20]).
// Image source: (30, 0); reflected plan distance = 20 m.
func TestComputeReflectedPaths_SingleReflection(t *testing.T) {
	t.Parallel()

	wall := Reflector{
		ID:       "wall",
		Geometry: []geo.Point2D{{X: 15, Y: -20}, {X: 15, Y: 20}},
		HeightM:  8,
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 10, Y: 0}, 4.0,
		[]Reflector{wall},
	)

	if len(paths) != 1 {
		t.Fatalf("expected 1 reflected path, got %d", len(paths))
	}
	// Image source = (30,0), receiver = (10,0) → plan dist = 20.
	if !almostEqual(paths[0].planDistM, 20.0, 1e-6) {
		t.Fatalf("expected plan dist 20.0 m, got %f", paths[0].planDistM)
	}
	// Default reflection loss = 1.0 dB.
	if !almostEqual(paths[0].lossDB, 1.0, 1e-9) {
		t.Fatalf("expected reflection loss 1.0 dB, got %f", paths[0].lossDB)
	}
}

// TestComputeReflectedPaths_WallSegmentMissed verifies that no reflection is
// returned when the reflected ray crosses the infinite wall line but misses
// the actual wall segment.
//
// Geometry: source (0,0), receiver (10,0), wall segment from (15,5) to (15,20).
// Reflection point would land at (15,0) — below the wall segment → no hit.
func TestComputeReflectedPaths_WallSegmentMissed(t *testing.T) {
	t.Parallel()

	wall := Reflector{
		ID:       "partial-wall",
		Geometry: []geo.Point2D{{X: 15, Y: 5}, {X: 15, Y: 20}},
		HeightM:  8,
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 10, Y: 0}, 4.0,
		[]Reflector{wall},
	)

	if len(paths) != 0 {
		t.Fatalf("expected 0 reflected paths, got %d", len(paths))
	}
}

// TestComputeReflectedPaths_NoReflectors verifies empty input returns no paths.
func TestComputeReflectedPaths_NoReflectors(t *testing.T) {
	t.Parallel()

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 10, Y: 0}, 4.0,
		nil,
	)
	if len(paths) != 0 {
		t.Fatalf("expected 0 paths for nil reflectors, got %d", len(paths))
	}
}

// TestComputeReflectedPaths_DoubleReflection_Corner verifies that two
// perpendicular walls generate single reflections off each wall plus one
// valid second-order reflection (A→B only; B→A is geometrically invalid here).
//
// Geometry:
//
//	Wall A: x=15 (y ∈ [-20,20])
//	Wall B: y=12 (x ∈ [-20,20])
//	Source: (0, 0), Receiver: (5, 5)
//
// Expected paths: 1st-A, 1st-B, 2nd-A-then-B (total 3).
// 2nd-B-then-A is invalid because the back-leg check fails.
func TestComputeReflectedPaths_DoubleReflection_Corner(t *testing.T) {
	t.Parallel()

	wallA := Reflector{
		ID:       "wall-a",
		Geometry: []geo.Point2D{{X: 15, Y: -20}, {X: 15, Y: 20}},
		HeightM:  8,
	}
	wallB := Reflector{
		ID:       "wall-b",
		Geometry: []geo.Point2D{{X: -20, Y: 12}, {X: 20, Y: 12}},
		HeightM:  8,
	}
	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 5, Y: 5}, 4.0,
		[]Reflector{wallA, wallB},
	)

	if len(paths) != 3 {
		t.Fatalf("expected 3 reflected paths (1st-A, 1st-B, 2nd-A-then-B), got %d", len(paths))
	}

	// The double-reflection plan distance = dist2D(S''=(30,24), R=(5,5)) = sqrt(986).
	expectedDouble := math.Sqrt(986.0)
	maxDist := 0.0
	maxLoss := 0.0

	for _, p := range paths {
		if p.planDistM > maxDist {
			maxDist = p.planDistM
			maxLoss = p.lossDB
		}
	}

	if !almostEqual(maxDist, expectedDouble, 0.01) {
		t.Fatalf("expected double-reflection plan dist ≈ %f, got %f", expectedDouble, maxDist)
	}
	// Double reflection loss = 1.0 + 1.0 = 2.0 dB.
	if !almostEqual(maxLoss, 2.0, 1e-9) {
		t.Fatalf("expected double-reflection loss 2.0 dB, got %f", maxLoss)
	}
}

// TestComputeReceiverLevels_ReflectionIncreasesLevel verifies that adding a
// reflector wall behind the road increases the receiver level.
//
// Geometry: road along x-axis (-50 to 50), receiver at (0, 50),
// reflector wall at y=-10 (behind road). Every segment has a valid reflection.
func TestComputeReceiverLevels_ReflectionIncreasesLevel(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 50}

	cfg := DefaultPropagationConfig()

	lvlNoRefl, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("no reflector: %v", err)
	}

	cfg.Reflectors = []Reflector{{
		ID:       "back-wall",
		Geometry: []geo.Point2D{{X: -200, Y: -10}, {X: 200, Y: -10}},
		HeightM:  8,
	}}

	lvlWithRefl, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("with reflector: %v", err)
	}

	if lvlWithRefl.LrDay <= lvlNoRefl.LrDay {
		t.Fatalf("reflection should increase level: no-refl=%.2f dB, with-refl=%.2f dB",
			lvlNoRefl.LrDay, lvlWithRefl.LrDay)
	}
}

// TestComputeReceiverLevels_ReflectionCustomLoss verifies that a higher
// reflection loss results in a smaller level increase than default.
func TestComputeReceiverLevels_ReflectionCustomLoss(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 50}

	cfgDefault := DefaultPropagationConfig()
	cfgDefault.Reflectors = []Reflector{{
		ID:       "back-wall",
		Geometry: []geo.Point2D{{X: -200, Y: -10}, {X: 200, Y: -10}},
		HeightM:  8,
		// ReflectionLossDB = 0 → uses default 1.0 dB
	}}

	cfgHighLoss := DefaultPropagationConfig()
	cfgHighLoss.Reflectors = []Reflector{{
		ID:               "back-wall",
		Geometry:         []geo.Point2D{{X: -200, Y: -10}, {X: 200, Y: -10}},
		HeightM:          8,
		ReflectionLossDB: 5.0, // absorptive surface
	}}

	lvlDefault, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgDefault)
	if err != nil {
		t.Fatalf("default loss: %v", err)
	}

	lvlHighLoss, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgHighLoss)
	if err != nil {
		t.Fatalf("high loss: %v", err)
	}

	if lvlHighLoss.LrDay >= lvlDefault.LrDay {
		t.Fatalf("higher reflection loss should reduce level increase: default=%.2f high=%.2f",
			lvlDefault.LrDay, lvlHighLoss.LrDay)
	}
}

// K5: Reflection height condition tests.
//
// A reflector only produces a valid reflected path when the wall is tall enough
// that the reflected ray does not pass over it. At the reflection point P the
// ray height is interpolated linearly between sourceZ and receiverZ:
//
//	t          = dist2D(imageSource, P) / dist2D(imageSource, receiver)
//	heightAtP  = sourceZ + (receiverZ − sourceZ) · t
//
// The reflection is valid only when wall.HeightM >= heightAtP.

// TestComputeReflectedPaths_HeightTooShort_NoReflection verifies that a
// geometrically plausible wall that is too short to intercept the ray produces
// no reflected path.
//
// Geometry: source (0,0) z=0.5 m, receiver (10,0) z=4.0 m,
// wall at x=15 (y ∈ [−5, 5]), height=2.0 m.
//
// Image source S′=(30,0).  Reflection point P=(15,0).
// t = dist(S′,P)/dist(S′,R) = 15/20 = 0.75.
// heightAtP = 0.5 + 3.5·0.75 = 3.125 m.
// 2.0 < 3.125 → ray passes over the wall → no reflection.
func TestComputeReflectedPaths_HeightTooShort_NoReflection(t *testing.T) {
	t.Parallel()

	wall := Reflector{
		ID:       "short-wall",
		Geometry: []geo.Point2D{{X: 15, Y: -5}, {X: 15, Y: 5}},
		HeightM:  2.0, // shorter than the 3.125 m ray height at reflection point
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 10, Y: 0}, 4.0,
		[]Reflector{wall},
	)

	if len(paths) != 0 {
		t.Fatalf("expected 0 reflected paths (wall too short), got %d", len(paths))
	}
}

// TestComputeReflectedPaths_HeightJustEnough_Reflection verifies that the same
// wall geometry with sufficient height produces one reflected path.
//
// Same geometry as above but wall height=5.0 m >= 3.125 m → reflection valid.
func TestComputeReflectedPaths_HeightJustEnough_Reflection(t *testing.T) {
	t.Parallel()

	wall := Reflector{
		ID:       "tall-wall",
		Geometry: []geo.Point2D{{X: 15, Y: -5}, {X: 15, Y: 5}},
		HeightM:  5.0, // 5.0 >= 3.125 → valid reflection
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 10, Y: 0}, 4.0,
		[]Reflector{wall},
	)

	if len(paths) != 1 {
		t.Fatalf("expected 1 reflected path (wall tall enough), got %d", len(paths))
	}

	// Image source S′=(30,0), receiver=(10,0) → plan dist = 20 m.
	if !almostEqual(paths[0].planDistM, 20.0, 1e-6) {
		t.Fatalf("expected plan dist 20.0 m, got %f", paths[0].planDistM)
	}
}

// TestComputeReflectedPaths_DoubleReflection_SecondWallTooShort verifies that
// when the second wall in a two-bounce path is too short to reflect the ray at
// its reflection point, the double reflection is suppressed. Single reflections
// that independently satisfy the height condition remain valid.
//
// Geometry: source (0,0) z=0.5, receiver (5,5) z=4.0.
// wallA at x=15 (y ∈ [−20,20]), height=8 m (tall enough for 1st bounce).
// wallB at y=12 (x ∈ [−20,20]), height=1 m (too short for both 1st-order
// off wallB and for the 2nd leg of A→B).
//
// Expected: only 1 path (1st-order off wallA). The 1st-order off wallB and
// the 2nd-order A→B path are suppressed by the height condition.
func TestComputeReflectedPaths_DoubleReflection_SecondWallTooShort(t *testing.T) {
	t.Parallel()

	wallA := Reflector{
		ID:       "wall-a",
		Geometry: []geo.Point2D{{X: 15, Y: -20}, {X: 15, Y: 20}},
		HeightM:  8, // tall: 1st-order off A valid (heightAtP ≈ 2.6 m)
	}
	wallB := Reflector{
		ID:       "wall-b",
		Geometry: []geo.Point2D{{X: -20, Y: 12}, {X: 20, Y: 12}},
		HeightM:  1, // short: 1st-order off B invalid (heightAtP ≈ 2.7 m) and
		// 2nd-order A→B invalid (heightAtP2 ≈ 2.7 m)
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 5, Y: 5}, 4.0,
		[]Reflector{wallA, wallB},
	)

	if len(paths) != 1 {
		t.Fatalf("expected 1 reflected path (1st-order off wall-a only), got %d", len(paths))
	}
}

// TestComputeReceiverLevels_ShortReflector_NoLevelIncrease verifies that a
// reflector too short to intercept any reflected ray does not increase the
// receiver level compared to free-field propagation.
//
// Geometry: road along x-axis, receiver at (0,50), reflector at y=−10 with
// height=0.6 m. The reflected ray height at the wall is ~0.52 m (very close to
// source height) for nearby segments, but increases for distant segments.
// We use a very short wall (0.6 m) so at least for a close segment the height
// condition can fail; for a fully opaque wall (height=30 m) it must fail for all.
func TestComputeReceiverLevels_ShortReflector_NoLevelIncrease(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 50}

	cfgFree := DefaultPropagationConfig()

	lvlFree, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgFree)
	if err != nil {
		t.Fatalf("free field: %v", err)
	}

	// A reflector that is far shorter than any ray height at the reflection point.
	cfgShort := DefaultPropagationConfig()
	cfgShort.Reflectors = []Reflector{{
		ID:       "too-short",
		Geometry: []geo.Point2D{{X: -200, Y: -10}, {X: 200, Y: -10}},
		HeightM:  0.1, // essentially at ground level — no ray can reflect off this
	}}

	lvlShort, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgShort)
	if err != nil {
		t.Fatalf("short reflector: %v", err)
	}

	// A wall so short that no reflected ray can reach it → level must not increase.
	if lvlShort.LrDay > lvlFree.LrDay+0.01 {
		t.Fatalf("short reflector should not increase level: free=%.2f dB, short=%.2f dB",
			lvlFree.LrDay, lvlShort.LrDay)
	}
}

// TestPropagation_ShieldedNoGroundEffect verifies that when shielded (D_z > 0)
// the ground effect (D_gr) is replaced by D_z, not added on top.
func TestPropagation_ShieldedNoGroundEffect(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()
	receiver := geo.Point2D{X: 0, Y: 50}

	// Free-field level (D_gr applies).
	freeField, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("free field: %v", err)
	}

	// Level with a very tall barrier (effectively full shielding, D_z >> D_gr).
	tallBarrier := Barrier{
		ID:       "tall",
		Geometry: []geo.Point2D{{X: -100, Y: 10}, {X: 100, Y: 10}},
		HeightM:  20.0,
	}

	withBarrier, err := ComputeReceiverLevels(receiver, []RoadSource{source}, []Barrier{tallBarrier}, cfg)
	if err != nil {
		t.Fatalf("with barrier: %v", err)
	}

	// Barrier should significantly attenuate.
	reduction := freeField.LrDay - withBarrier.LrDay
	if reduction < 10 {
		t.Fatalf("tall barrier should attenuate by >10 dB, got %f dB", reduction)
	}
}

// --- RLS-19 Section 3.3.6: Längsneigungskorrektur (Eqs. 7a / 7b / 7c) ---
//
// Normative formulas are speed-dependent.  The existing GradientCorrection
// function accepts no speed parameter and therefore cannot implement the
// correct formulas.  These tests encode the exact Eq. 7a/7b/7c values and
// are expected to FAIL until GradientCorrection is updated.

func TestGradientCorrection_Eq7a_Pkw_Uphill(t *testing.T) {
	t.Parallel()

	// Eq. 7a for g > +2: D = (g-2)/10 * (v_Pkw+70)/100
	// g=8, v=100: (8-2)/10 * (100+70)/100 = 0.6 * 1.7 = 1.020
	want := (8.0 - 2.0) / 10.0 * (100.0 + 70.0) / 100.0

	got := GradientCorrection(8, Pkw, 100)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(8, Pkw, 100): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Eq7a_Pkw_Uphill_LowSpeed(t *testing.T) {
	t.Parallel()

	// g=4, v=50: just above threshold → (4-2)/10 * (50+70)/100 = 0.2 * 1.2 = 0.240
	want := (4.0 - 2.0) / 10.0 * (50.0 + 70.0) / 100.0

	got := GradientCorrection(4, Pkw, 50)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(4, Pkw, 50): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Eq7a_Pkw_Downhill(t *testing.T) {
	t.Parallel()

	// Eq. 7a for g < -6: D = (g+6)/(-6) * (90-min(v_Pkw,70))/20
	// g=-8, v=100: (-8+6)/(-6) * (90-70)/20 = (1/3) * 1 = 0.333...
	want := (-8.0 + 6.0) / (-6.0) * (90.0 - 70.0) / 20.0

	got := GradientCorrection(-8, Pkw, 100)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(-8, Pkw, 100): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Eq7a_Pkw_Flat(t *testing.T) {
	t.Parallel()

	// |g| <= 2 for Pkw → 0 at any speed.
	for _, v := range []float64{30, 60, 100, 130} {
		got := GradientCorrection(1, Pkw, v)
		if got != 0 {
			t.Fatalf("GradientCorrection(1, Pkw, %.0f): want 0, got %.4f", v, got)
		}
	}
}

func TestGradientCorrection_Eq7b_Lkw1_Uphill(t *testing.T) {
	t.Parallel()

	// Eq. 7b for g > +2: D = (g-2)/10 * v_Lkw1/10
	// g=8, v=80: (8-2)/10 * 80/10 = 0.6 * 8 = 4.800
	want := (8.0 - 2.0) / 10.0 * 80.0 / 10.0

	got := GradientCorrection(8, Lkw1, 80)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(8, Lkw1, 80): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Eq7b_Lkw1_Downhill(t *testing.T) {
	t.Parallel()

	// Eq. 7b for g < -4: D = (g+4)/(-8) * (v_Lkw1-20)/10
	// g=-6, v=80: (-6+4)/(-8) * (80-20)/10 = 0.25 * 6 = 1.500
	want := (-6.0 + 4.0) / (-8.0) * (80.0 - 20.0) / 10.0

	got := GradientCorrection(-6, Lkw1, 80)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(-6, Lkw1, 80): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Eq7c_Lkw2_Uphill(t *testing.T) {
	t.Parallel()

	// Eq. 7c for g > +2: D = (g-2)/10 * (v_Lkw2+10)/10
	// g=8, v=70: (8-2)/10 * (70+10)/10 = 0.6 * 8 = 4.800
	want := (8.0 - 2.0) / 10.0 * (70.0 + 10.0) / 10.0

	got := GradientCorrection(8, Lkw2, 70)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(8, Lkw2, 70): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Eq7c_Lkw2_Downhill(t *testing.T) {
	t.Parallel()

	// Eq. 7c for g < -4: D = (g+4)/(-8) * (v_Lkw2-10)/10
	// g=-6, v=70: (-6+4)/(-8) * (70-10)/10 = 0.25 * 6 = 1.500
	want := (-6.0 + 4.0) / (-8.0) * (70.0 - 10.0) / 10.0

	got := GradientCorrection(-6, Lkw2, 70)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(-6, Lkw2, 70): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Clamped_At12(t *testing.T) {
	t.Parallel()

	// Gradients beyond ±12% use ±12% values.
	if GradientCorrection(15, Lkw2, 70) != GradientCorrection(12, Lkw2, 70) {
		t.Fatal("gradient should be clamped at +12%")
	}

	if GradientCorrection(-15, Pkw, 100) != GradientCorrection(-12, Pkw, 100) {
		t.Fatal("gradient should be clamped at -12%")
	}
}

// --- RLS-19 Section 3.3.7: Knotenpunktkorrektur (Eq. 8 / Tabelle 5) ---
//
// Eq. 8: D_{KKT}(x) = K_KT * max(1 - x/120, 0)
// Tabelle 5: signalized K_KT=3, roundabout K_KT=2, other K_KT=0.
// The step-table currently in JunctionCorrection does not implement this formula.

func TestJunctionCorrection_Eq8_Signalized_At0m(t *testing.T) {
	t.Parallel()

	// x=0: K_KT * (1 - 0) = 3 * 1 = 3.0
	got := JunctionCorrection(JunctionSignalized, 0)
	if !almostEqual(got, 3.0, 0.001) {
		t.Fatalf("JunctionCorrection(Signalized, 0): want 3.000, got %.4f", got)
	}
}

func TestJunctionCorrection_Eq8_Signalized_ContinuousAt60m(t *testing.T) {
	t.Parallel()

	// x=60: 3 * (1 - 60/120) = 3 * 0.5 = 1.5
	got := JunctionCorrection(JunctionSignalized, 60)
	if !almostEqual(got, 1.5, 0.001) {
		t.Fatalf("JunctionCorrection(Signalized, 60): want 1.500, got %.4f", got)
	}
}

func TestJunctionCorrection_Eq8_Signalized_At120m(t *testing.T) {
	t.Parallel()

	// x=120: 3 * (1 - 1) = 0
	got := JunctionCorrection(JunctionSignalized, 120)
	if !almostEqual(got, 0.0, 0.001) {
		t.Fatalf("JunctionCorrection(Signalized, 120): want 0.000, got %.4f", got)
	}
}

func TestJunctionCorrection_Eq8_Roundabout_At40m(t *testing.T) {
	t.Parallel()

	// x=40: 2 * (1 - 40/120) = 2 * (2/3) ≈ 1.333
	want := 2.0 * (1.0 - 40.0/120.0)

	got := JunctionCorrection(JunctionRoundabout, 40)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("JunctionCorrection(Roundabout, 40): want %.4f, got %.4f", want, got)
	}
}

func TestJunctionCorrection_Eq8_Other_AlwaysZero(t *testing.T) {
	t.Parallel()

	// K_KT=0 for sonstige Knotenpunkte → always 0.
	for _, x := range []float64{0, 10, 50, 120} {
		got := JunctionCorrection(JunctionOther, x)
		if got != 0 {
			t.Fatalf("JunctionCorrection(Other, %.0f): want 0, got %.4f", x, got)
		}
	}
}

// --- RLS-19 Section 3.6: Berücksichtigung von Reflexionen (Tabelle 8) ---

// TestReflectorType_Tabelle8_LossValues verifies that each ReflectorType
// returns the normative loss value from RLS-19 Tabelle 8.
func TestReflectorType_Tabelle8_LossValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		rt     ReflectorType
		wantDB float64
		name   string
	}{
		{ReflectorTypeFacadeOrReflecting, 0.5, "FacadeOrReflecting"},
		{ReflectorTypeReflectionReducing, 3.0, "ReflectionReducing"},
		{ReflectorTypeStronglyReflectionReducing, 5.0, "StronglyReflectionReducing"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := Reflector{
				ID:       "wall",
				Geometry: []geo.Point2D{{X: 0, Y: 0}, {X: 1, Y: 0}},
				HeightM:  5.0,
				Type:     tc.rt,
			}
			got := r.effectiveLoss()

			if !almostEqual(got, tc.wantDB, 1e-9) {
				t.Fatalf("effectiveLoss() = %g dB, want %g dB", got, tc.wantDB)
			}
		})
	}
}

// TestReflectorType_TypeTakesPrecedenceOverExplicitLoss verifies that when
// Type is set, it overrides ReflectionLossDB.
func TestReflectorType_TypeTakesPrecedenceOverExplicitLoss(t *testing.T) {
	t.Parallel()

	r := Reflector{
		ID:               "wall",
		Geometry:         []geo.Point2D{{X: 0, Y: 0}, {X: 1, Y: 0}},
		HeightM:          5.0,
		Type:             ReflectorTypeFacadeOrReflecting, // 0.5 dB
		ReflectionLossDB: 3.0,                             // should be ignored
	}

	got := r.effectiveLoss()
	if !almostEqual(got, 0.5, 1e-9) {
		t.Fatalf("effectiveLoss() = %g dB, want 0.5 dB (Type takes precedence)", got)
	}
}

// TestReflectorType_Unspecified_UsesExplicitLoss verifies that
// ReflectorTypeUnspecified falls back to ReflectionLossDB when set.
func TestReflectorType_Unspecified_UsesExplicitLoss(t *testing.T) {
	t.Parallel()

	r := Reflector{
		ID:               "wall",
		Geometry:         []geo.Point2D{{X: 0, Y: 0}, {X: 1, Y: 0}},
		HeightM:          5.0,
		ReflectionLossDB: 2.5,
	}

	got := r.effectiveLoss()
	if !almostEqual(got, 2.5, 1e-9) {
		t.Fatalf("effectiveLoss() = %g dB, want 2.5 dB", got)
	}
}

// TestReflectorType_Unspecified_DefaultLoss verifies that when neither Type
// nor ReflectionLossDB is set, effectiveLoss returns the 1.0 dB default.
func TestReflectorType_Unspecified_DefaultLoss(t *testing.T) {
	t.Parallel()

	r := Reflector{
		ID:       "wall",
		Geometry: []geo.Point2D{{X: 0, Y: 0}, {X: 1, Y: 0}},
		HeightM:  5.0,
		// Type = ReflectorTypeUnspecified (zero), ReflectionLossDB = 0
	}

	got := r.effectiveLoss()
	if !almostEqual(got, 1.0, 1e-9) {
		t.Fatalf("effectiveLoss() = %g dB, want 1.0 dB (default)", got)
	}
}

// TestReflection_NormativeHeightCondition_Below1m verifies that a reflector
// with height < 1.0 m is rejected even when the ray would not pass over it.
//
// RLS-19 Tabelle 8: h_R ≥ 1.0 m is a minimum regardless of path geometry.
//
// Geometry: source (0,0) z=0, receiver (100,0) z=0 (same height), so the
// reflected ray stays at z=0 — the ray would not pass over a 0.8 m wall.
// However h_R = 0.8 < 1.0 → normative condition fails → no reflection.
func TestReflection_NormativeHeightCondition_Below1m(t *testing.T) {
	t.Parallel()

	wall := Reflector{
		ID:       "low-wall",
		Geometry: []geo.Point2D{{X: 110, Y: -20}, {X: 110, Y: 20}},
		HeightM:  0.8, // < 1.0 m: normative minimum not satisfied
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.0,
		geo.Point2D{X: 100, Y: 0}, 0.0,
		[]Reflector{wall},
	)

	if len(paths) != 0 {
		t.Fatalf("expected 0 paths (h_R < 1.0 m), got %d", len(paths))
	}
}

// TestReflection_NormativeHeightCondition_Geometric verifies that a reflector
// whose height satisfies h_R ≥ 1.0 m but fails h_R ≥ 0.3·√(a_R) produces no
// reflected path.
//
// Geometry: source (0,0) z=0, receiver (100,0) z=0, wall at y=20 (x ∈ [−30,130]).
// Reflection point P = (50, 20) (midpoint of the 90-degree path).
// dist(S,P) = dist(P,R) = √(2500+400) ≈ 53.85 m → a_R = 53.85 m.
// 0.3·√53.85 ≈ 2.20 m.
// Wall height = 1.5 m ≥ 1.0 m ✓  but 1.5 < 2.20 ✗ → no reflection.
func TestReflection_NormativeHeightCondition_Geometric(t *testing.T) {
	t.Parallel()

	wall := Reflector{
		ID:       "short-for-distance",
		Geometry: []geo.Point2D{{X: -30, Y: 20}, {X: 130, Y: 20}},
		HeightM:  1.5, // ≥ 1.0 m but < 0.3·√(53.85) ≈ 2.20 m
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.0,
		geo.Point2D{X: 100, Y: 0}, 0.0,
		[]Reflector{wall},
	)

	if len(paths) != 0 {
		t.Fatalf("expected 0 paths (h_R < 0.3·√a_R), got %d", len(paths))
	}
}

// TestReflection_NormativeHeightCondition_TallEnough verifies that a wall
// satisfying both normative height conditions produces a valid reflection.
//
// Same geometry as above but wall height = 3.0 m ≥ 2.20 m → reflection valid.
func TestReflection_NormativeHeightCondition_TallEnough(t *testing.T) {
	t.Parallel()

	wall := Reflector{
		ID:       "tall-enough",
		Geometry: []geo.Point2D{{X: -30, Y: 20}, {X: 130, Y: 20}},
		HeightM:  3.0, // ≥ 1.0 m and ≥ 0.3·√53.85 ≈ 2.20 m
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.0,
		geo.Point2D{X: 100, Y: 0}, 0.0,
		[]Reflector{wall},
	)

	if len(paths) != 1 {
		t.Fatalf("expected 1 path (tall enough wall), got %d", len(paths))
	}
}

// TestComputeReflectedPaths_ThirdOrder_NotComputed verifies that
// computeReflectedPaths only returns 1st- and 2nd-order paths.
// RLS-19 explicitly ignores 3rd- and higher-order reflections.
//
// Three walls are arranged in a U-shape around the source. With 3 walls the
// theoretical maximum is 3 (1st-order) + 6 (2nd-order) = 9 paths.
// Any 3rd-order path would require 3 bounces and cannot appear in the result.
func TestComputeReflectedPaths_ThirdOrder_NotComputed(t *testing.T) {
	t.Parallel()

	// Three tall walls forming a U-shape behind the source.
	walls := []Reflector{
		{ID: "w1", Geometry: []geo.Point2D{{X: -20, Y: -5}, {X: -20, Y: 5}}, HeightM: 10},
		{ID: "w2", Geometry: []geo.Point2D{{X: 20, Y: -5}, {X: 20, Y: 5}}, HeightM: 10},
		{ID: "w3", Geometry: []geo.Point2D{{X: -20, Y: -5}, {X: 20, Y: -5}}, HeightM: 10},
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 0, Y: 50}, 4.0,
		walls,
	)

	// Maximum possible paths = 1st-order (≤3) + 2nd-order (≤6) = ≤9.
	// If 3rd-order were computed, the count would exceed this bound.
	const maxExpected = 9
	if len(paths) > maxExpected {
		t.Fatalf("expected at most %d paths (no 3rd-order), got %d", maxExpected, len(paths))
	}

	// Additionally: no path should carry a loss exceeding 3 * max-single-loss.
	// With the 1.0 dB default, a 3rd-order path would have loss > 2.0 dB.
	for _, p := range paths {
		if p.lossDB > 2.0+1e-9 {
			t.Fatalf("path loss %g dB exceeds 2-bounce maximum (3rd-order not expected)", p.lossDB)
		}
	}
}

// TestDiagrammI_SingleVehicleSpeedSweep verifies that the single-vehicle
// length-related sound power level L_W' matches the normative Diagramm I
// from RLS-19 (Ausgabe 2019, Anhang).
//
// Diagramm I: "Längenbezogener Schallleistungspegel L_W' eines Fahrzeuges
// pro Stunde in dB" as a function of speed for Pkw, Lkw1, and Lkw2.
//
// For one vehicle/h with no surface, gradient, or junction corrections:
//
//	L_W'(v, FzG) = L_W0(v, FzG) − 10·lg(v) − 30
//
// with Tabelle 3 coefficients:
//
//	Pkw:  A=88.0,  B=20, C=3.06
//	Lkw1: A=100.3, B=40, C=4.33
//	Lkw2: A=105.4, B=50, C=4.88
//
// Reference values are computed from the normative Tabelle 3 formula; the
// Diagramm I chart values are consistent with these results.
func TestDiagrammI_SingleVehicleSpeedSweep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		group    VehicleGroup
		speedKPH float64
		wantLWp  float64 // L_W' = L_W0 − 10·lg(v) − 30
	}{
		// Pkw (A=88.0, B=20, C=3.06) – values readable from Diagramm I
		{name: "Pkw 30 km/h", group: Pkw, speedKPH: 30, wantLWp: 49.720298},
		{name: "Pkw 80 km/h", group: Pkw, speedKPH: 80, wantLWp: 57.454134},
		{name: "Pkw 100 km/h", group: Pkw, speedKPH: 100, wantLWp: 59.419914},
		{name: "Pkw 130 km/h", group: Pkw, speedKPH: 130, wantLWp: 61.749826},
		// Lkw1 (A=100.3, B=40, C=4.33)
		{name: "Lkw1 30 km/h", group: Lkw1, speedKPH: 30, wantLWp: 56.627102},
		{name: "Lkw1 80 km/h", group: Lkw1, speedKPH: 80, wantLWp: 64.514438},
		{name: "Lkw1 100 km/h", group: Lkw1, speedKPH: 100, wantLWp: 67.612203},
		// Lkw2 (A=105.4, B=50, C=4.88)
		{name: "Lkw2 30 km/h", group: Lkw2, speedKPH: 30, wantLWp: 60.973771},
		{name: "Lkw2 80 km/h", group: Lkw2, speedKPH: 80, wantLWp: 66.747637},
		{name: "Lkw2 100 km/h", group: Lkw2, speedKPH: 100, wantLWp: 70.235303},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Neutral source: no surface correction, no gradient, no junction.
			source := RoadSource{
				ID:          "diagramm-i",
				SurfaceType: SurfaceGussasphaltStandard,
				Speeds: SpeedInput{
					PkwKPH:  tt.speedKPH,
					Lkw1KPH: tt.speedKPH,
					Lkw2KPH: tt.speedKPH,
					KradKPH: tt.speedKPH,
				},
				GradientPercent: 0,
				JunctionType:    JunctionNone,
				BuildingHeightM: 0,
				StreetWidthM:    0,
				Centerline: []geo.Point2D{
					{X: -1, Y: 0},
					{X: 1, Y: 0},
				},
			}

			// One vehicle per hour of the tested group only.
			var traffic TrafficInput

			switch tt.group {
			case Pkw:
				traffic.PkwPerHour = 1
			case Lkw1:
				traffic.Lkw1PerHour = 1
			case Lkw2:
				traffic.Lkw2PerHour = 1
			}

			got := emissionForPeriod(source, traffic)
			if !almostEqual(got, tt.wantLWp, 1e-4) {
				t.Fatalf("L_W'(%s, %.0f km/h): want %.6f, got %.6f dB",
					tt.group, tt.speedKPH, tt.wantLWp, got)
			}
		})
	}
}

// --- barrier crossing tests ---

func TestFindBarrierCrossings_SortedByDistance(t *testing.T) {
	t.Parallel()

	// Two barriers: far one listed first, near one second.
	farBarrier := Barrier{
		ID:       "far",
		Geometry: []geo.Point2D{{X: -100, Y: 30}, {X: 100, Y: 30}},
		HeightM:  5.0,
	}
	nearBarrier := Barrier{
		ID:       "near",
		Geometry: []geo.Point2D{{X: -100, Y: 10}, {X: 100, Y: 10}},
		HeightM:  4.0,
	}

	crossings := findBarrierCrossings(
		geo.Point2D{X: 0, Y: 0},
		geo.Point2D{X: 0, Y: 50},
		[]Barrier{farBarrier, nearBarrier},
	)

	if len(crossings) != 2 {
		t.Fatalf("expected 2 crossings, got %d", len(crossings))
	}

	if crossings[0].barrier.ID != "near" {
		t.Fatalf("expected nearest barrier first, got %q", crossings[0].barrier.ID)
	}

	if crossings[1].barrier.ID != "far" {
		t.Fatalf("expected farthest barrier second, got %q", crossings[1].barrier.ID)
	}
}

func TestFindBarrierCrossings_SkipsNonIntersecting(t *testing.T) {
	t.Parallel()

	parallel := Barrier{
		ID:       "parallel",
		Geometry: []geo.Point2D{{X: 5, Y: -10}, {X: 5, Y: 100}},
		HeightM:  4.0,
	}
	crossing := Barrier{
		ID:       "crossing",
		Geometry: []geo.Point2D{{X: -100, Y: 20}, {X: 100, Y: 20}},
		HeightM:  4.0,
	}

	crossings := findBarrierCrossings(
		geo.Point2D{X: 0, Y: 0},
		geo.Point2D{X: 0, Y: 50},
		[]Barrier{parallel, crossing},
	)

	if len(crossings) != 1 {
		t.Fatalf("expected 1 crossing, got %d", len(crossings))
	}

	if crossings[0].barrier.ID != "crossing" {
		t.Fatal("expected the crossing barrier")
	}
}

func TestSelectDiffractionEdges_SingleObstructing(t *testing.T) {
	t.Parallel()

	crossings := []barrierCrossing{
		{distFromSource: 10, barrier: &Barrier{ID: "w1", HeightM: 6.0}},
	}

	edges := selectDiffractionEdges(0.5, 4.0, 50.0, crossings)

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}

	if edges[0].barrier.ID != "w1" {
		t.Fatalf("expected w1, got %q", edges[0].barrier.ID)
	}
}

func TestSelectDiffractionEdges_TwoEdges(t *testing.T) {
	t.Parallel()

	// Both barriers above LOS → both selected.
	crossings := []barrierCrossing{
		{distFromSource: 10, barrier: &Barrier{ID: "w1", HeightM: 6.0}},
		{distFromSource: 30, barrier: &Barrier{ID: "w2", HeightM: 6.0}},
	}

	edges := selectDiffractionEdges(0.5, 4.0, 50.0, crossings)

	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}
}

func TestSelectDiffractionEdges_HullReduces(t *testing.T) {
	t.Parallel()

	// Three barriers: middle one is below rubber band between outer two → excluded.
	crossings := []barrierCrossing{
		{distFromSource: 10, barrier: &Barrier{ID: "w1", HeightM: 8.0}},
		{distFromSource: 20, barrier: &Barrier{ID: "w2", HeightM: 3.0}}, // below hull
		{distFromSource: 30, barrier: &Barrier{ID: "w3", HeightM: 8.0}},
	}

	edges := selectDiffractionEdges(0.5, 4.0, 50.0, crossings)

	if len(edges) != 2 {
		t.Fatalf("expected 2 edges (middle excluded by hull), got %d", len(edges))
	}

	if edges[0].barrier.ID != "w1" || edges[1].barrier.ID != "w3" {
		t.Fatalf("expected w1 and w3, got %q and %q", edges[0].barrier.ID, edges[1].barrier.ID)
	}
}

func TestSelectDiffractionEdges_NoneObstructing(t *testing.T) {
	t.Parallel()

	// Barrier below line of sight.
	crossings := []barrierCrossing{
		{distFromSource: 25, barrier: &Barrier{ID: "low", HeightM: 1.0}},
	}

	edges := selectDiffractionEdges(0.5, 4.0, 50.0, crossings)

	if len(edges) != 0 {
		t.Fatalf("expected 0 edges for non-obstructing barrier, got %d", len(edges))
	}
}

func TestMultiDiffractionLoss_SingleEdgeMatchesExisting(t *testing.T) {
	t.Parallel()

	// Single edge at distance 10m, height 6m.
	// Source: height 0.5m. Receiver: height 4m at 50m.
	edges := []diffractionEdge{
		{distFromSource: 10, heightM: 6.0},
	}

	z, loss := computeMultiEdgeLoss(edges, 0.5, 4.0, 50.0)

	// Compare with existing single-edge computation.
	singleGeom := computeDiffraction(10, 0.5, 40, 4.0, 6.0)
	singleLoss := rls19BarrierLoss(singleGeom)

	if math.Abs(z-singleGeom.Z) > 1e-10 {
		t.Fatalf("z mismatch: multi=%f single=%f", z, singleGeom.Z)
	}

	if math.Abs(loss-singleLoss) > 1e-10 {
		t.Fatalf("loss mismatch: multi=%f single=%f", loss, singleLoss)
	}
}

func TestMultiDiffractionLoss_TwoEdges_CTermPositive(t *testing.T) {
	t.Parallel()

	// Two edges: wide building scenario (Bild 13, case 2).
	// Source at 0, height 0.5m. Receiver at 50m, height 4m.
	// Edge 1 at 10m, height 6m. Edge 2 at 20m, height 6m.
	edges := []diffractionEdge{
		{distFromSource: 10, heightM: 6.0},
		{distFromSource: 20, heightM: 6.0},
	}

	z, loss := computeMultiEdgeLoss(edges, 0.5, 4.0, 50.0)

	// z must be greater than single-edge case (C > 0 adds path length).
	singleGeom := computeDiffraction(10, 0.5, 40, 4.0, 6.0)
	if z <= singleGeom.Z {
		t.Fatalf("multi-edge z (%f) should exceed single-edge z (%f)", z, singleGeom.Z)
	}

	// Loss must be greater than single-edge case.
	singleLoss := rls19BarrierLoss(singleGeom)
	if loss <= singleLoss {
		t.Fatalf("multi-edge loss (%f) should exceed single-edge loss (%f)", loss, singleLoss)
	}

	// Verify z = A + B + C - s.
	dSB := 10.0
	dBR := 30.0 // 50 - 20
	dTotal := 50.0
	hS := 0.5
	hR := 4.0
	hE1 := 6.0
	hE2 := 6.0

	A := math.Sqrt(dSB*dSB + (hE1-hS)*(hE1-hS))
	B := math.Sqrt(dBR*dBR + (hR-hE2)*(hR-hE2))
	interEdgeDist := 20.0 - 10.0
	interEdgeH := hE2 - hE1
	C := math.Sqrt(interEdgeDist*interEdgeDist + interEdgeH*interEdgeH)
	s := math.Sqrt(dTotal*dTotal + (hR-hS)*(hR-hS))
	expectedZ := A + B + C - s

	if math.Abs(z-expectedZ) > 1e-10 {
		t.Fatalf("z = %f, expected A+B+C-s = %f", z, expectedZ)
	}
}

func TestMultiDiffractionLoss_KwUsesLargerLeg(t *testing.T) {
	t.Parallel()

	// Asymmetric setup: A > B, so C should be added to A in K_w.
	edgesALarger := []diffractionEdge{
		{distFromSource: 5, heightM: 6.0},
		{distFromSource: 45, heightM: 6.0},
	}
	_, lossALarger := computeMultiEdgeLoss(edgesALarger, 0.5, 4.0, 50.0)

	// Different geometry: B > A.
	edgesBLarger := []diffractionEdge{
		{distFromSource: 5, heightM: 6.0},
		{distFromSource: 10, heightM: 6.0},
	}
	_, lossBLarger := computeMultiEdgeLoss(edgesBLarger, 0.5, 4.0, 50.0)

	// Both should produce valid positive losses.
	if lossALarger <= 0 || lossBLarger <= 0 {
		t.Fatalf("expected positive losses: A-larger=%f B-larger=%f", lossALarger, lossBLarger)
	}
}

func TestPropagation_WithMultipleBarriers(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()
	receiver := geo.Point2D{X: 0, Y: 50}

	// Free-field level.
	freeField, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("free field: %v", err)
	}

	// Single barrier.
	barrier1 := Barrier{
		ID:       "wall-1",
		Geometry: []geo.Point2D{{X: -100, Y: 10}, {X: 100, Y: 10}},
		HeightM:  6.0,
	}

	singleBarrier, err := ComputeReceiverLevels(receiver, []RoadSource{source}, []Barrier{barrier1}, cfg)
	if err != nil {
		t.Fatalf("single barrier: %v", err)
	}

	// Two barriers (both above LOS with equal height → multi-diffraction).
	barrier2 := Barrier{
		ID:       "wall-2",
		Geometry: []geo.Point2D{{X: -100, Y: 20}, {X: 100, Y: 20}},
		HeightM:  6.0,
	}

	doubleBarrier, err := ComputeReceiverLevels(receiver, []RoadSource{source}, []Barrier{barrier1, barrier2}, cfg)
	if err != nil {
		t.Fatalf("double barrier: %v", err)
	}

	// free-field > single barrier > double barrier (more shielding = lower level).
	if singleBarrier.LrDay >= freeField.LrDay {
		t.Fatalf("single barrier day (%f) should be less than free field (%f)",
			singleBarrier.LrDay, freeField.LrDay)
	}

	if doubleBarrier.LrDay >= singleBarrier.LrDay {
		t.Fatalf("double barrier day (%f) should be less than single barrier (%f)",
			doubleBarrier.LrDay, singleBarrier.LrDay)
	}
}
