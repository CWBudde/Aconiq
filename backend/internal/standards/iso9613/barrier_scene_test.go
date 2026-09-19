package iso9613

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// The tests in this file reach A_bar the way a run does: from a scene of
// barriers on the propagation config. Not one of them constructs a
// BarrierGeometry, which is the whole point — every other barrier test in
// this package builds one by hand, and that is why the screening formulas
// could be complete and tested while no run ever evaluated them.

// sceneConfig is a propagation config with the screening scene attached and
// nothing else varying, so a level difference is attributable to the barrier.
func sceneConfig(barriers []Barrier) PropagationConfig {
	cfg := DefaultPropagationConfig()
	cfg.Barriers = barriers

	return cfg
}

// wallAcross returns a barrier crossing the x axis at x, tall enough to
// screen the paths these tests build.
func wallAcross(id string, x, heightM float64) Barrier {
	return Barrier{
		ID:       id,
		Geometry: []geo.Point2D{{X: x, Y: -50}, {X: x, Y: 50}},
		HeightM:  heightM,
	}
}

func testSource() PointSource {
	return PointSource{
		ID:                "src",
		Point:             geo.Point2D{X: 0, Y: 0},
		SourceHeightM:     1,
		SoundPowerLevelDB: 100,
	}
}

func testReceiver() geo.PointReceiver {
	return geo.PointReceiver{ID: "rec", Point: geo.Point2D{X: 100, Y: 0}, HeightM: 2}
}

func TestSceneBarrierLowersTheLevel(t *testing.T) {
	t.Parallel()

	receiver := testReceiver()
	sources := []PointSource{testSource()}

	unscreened := ComputeDownwindLevel(receiver, sources, sceneConfig(nil))
	screened := ComputeDownwindLevel(receiver, sources, sceneConfig([]Barrier{wallAcross("wall", 50, 8)}))

	if !(screened < unscreened) {
		t.Fatalf("a barrier across the path must lower the level: screened %.6f dB, unscreened %.6f dB", screened, unscreened)
	}

	// The whole defect this closes was a silent 0 dB. A difference too small
	// to see would be the same failure wearing a different number.
	if unscreened-screened < 1 {
		t.Fatalf("screening gained only %.6f dB, which is not a barrier taking effect", unscreened-screened)
	}
}

func TestSceneBarrierBesideThePathChangesNothing(t *testing.T) {
	t.Parallel()

	receiver := testReceiver()
	sources := []PointSource{testSource()}

	aside := Barrier{
		ID:       "aside",
		Geometry: []geo.Point2D{{X: 50, Y: 20}, {X: 50, Y: 60}},
		HeightM:  8,
	}

	unscreened := ComputeDownwindLevel(receiver, sources, sceneConfig(nil))
	withAside := ComputeDownwindLevel(receiver, sources, sceneConfig([]Barrier{aside}))

	if math.Abs(withAside-unscreened) > 1e-12 {
		t.Fatalf("a barrier the ray never crosses must not change the level: %.12f vs %.12f", withAside, unscreened)
	}
}

func TestSceneBarrierBelowTheLineOfSightChangesNothing(t *testing.T) {
	t.Parallel()

	receiver := testReceiver()
	sources := []PointSource{testSource()}

	// The sight line runs from 1 m to 2 m, so it stands at 1.5 m mid-path.
	low := wallAcross("low", 50, 1.2)

	unscreened := ComputeDownwindLevel(receiver, sources, sceneConfig(nil))
	withLow := ComputeDownwindLevel(receiver, sources, sceneConfig([]Barrier{low}))

	if math.Abs(withLow-unscreened) > 1e-12 {
		t.Fatalf("a barrier under the line of sight must not screen: %.12f vs %.12f", withLow, unscreened)
	}
}

func TestSceneTallerBarrierScreensHarder(t *testing.T) {
	t.Parallel()

	receiver := testReceiver()
	sources := []PointSource{testSource()}

	low := ComputeDownwindLevel(receiver, sources, sceneConfig([]Barrier{wallAcross("wall", 50, 5)}))
	high := ComputeDownwindLevel(receiver, sources, sceneConfig([]Barrier{wallAcross("wall", 50, 12)}))

	if !(high < low) {
		t.Fatalf("a taller barrier must screen at least as hard: 12 m gave %.6f dB, 5 m gave %.6f dB", high, low)
	}
}

func TestSceneTwoBarriersGiveDoubleDiffraction(t *testing.T) {
	t.Parallel()

	source := testSource()
	receiver := testReceiver()

	single := DeriveBarrierGeometry(
		source.Point, source.SourceHeightM, receiver.Point, receiver.HeightM,
		[]Barrier{wallAcross("a", 30, 8)},
	)
	if single == nil {
		t.Fatal("one crossing barrier must produce a geometry")
	}

	if single.IsDouble() {
		t.Fatalf("one edge must be single diffraction, got e = %.6f", single.E)
	}

	double := DeriveBarrierGeometry(
		source.Point, source.SourceHeightM, receiver.Point, receiver.HeightM,
		[]Barrier{wallAcross("a", 30, 8), wallAcross("b", 70, 8)},
	)
	if double == nil {
		t.Fatal("two crossing barriers must produce a geometry")
	}

	if !double.IsDouble() {
		t.Fatal("two surviving edges must be double diffraction")
	}

	// e runs along the diffracted path between the two edges, which here are
	// both 8 m high and 40 m apart.
	if math.Abs(double.E-40) > 1e-9 {
		t.Fatalf("e = %.9f m, want the 40 m between the two edges", double.E)
	}
}

func TestSceneDerivedGeometryHasNoLateralComponent(t *testing.T) {
	t.Parallel()

	source := testSource()
	receiver := testReceiver()

	geometry := DeriveBarrierGeometry(
		source.Point, source.SourceHeightM, receiver.Point, receiver.HeightM,
		[]Barrier{wallAcross("wall", 50, 8)},
	)
	if geometry == nil {
		t.Fatal("expected a geometry")
	}

	// a is the source-to-receiver component parallel to the diffraction edge.
	// It is zero here because this wall stands square to the ray, and only
	// then — see TestObliqueScreenCarriesAParallelComponent for the general
	// case. Reading it as "always zero for top diffraction" is what made the
	// path difference wrong for every oblique screen.
	if geometry.A != 0 {
		t.Fatalf("a = %.9f, want 0 for a wall square to the ray", geometry.A)
	}

	if geometry.LineOfSightClear {
		t.Fatal("a derived geometry only exists where an edge obstructs, so the sight line is never clear")
	}
}

func TestSceneRubberBandDropsAHiddenBarrier(t *testing.T) {
	t.Parallel()

	source := testSource()
	receiver := testReceiver()

	// A low wall between two tall ones is under the band and contributes no
	// edge, so the geometry must be the same as without it.
	tall := []Barrier{wallAcross("a", 20, 12), wallAcross("c", 80, 12)}
	withHidden := append([]Barrier{}, tall...)
	withHidden = append(withHidden, wallAcross("b", 50, 6))

	without := DeriveBarrierGeometry(source.Point, source.SourceHeightM, receiver.Point, receiver.HeightM, tall)
	with := DeriveBarrierGeometry(source.Point, source.SourceHeightM, receiver.Point, receiver.HeightM, withHidden)

	if without == nil || with == nil {
		t.Fatal("both scenes must produce a geometry")
	}

	if *without != *with {
		t.Fatalf("a barrier the band spans over must not change the geometry:\n without %+v\n with    %+v", *without, *with)
	}
}

func TestSceneNoBarriersDerivesNothing(t *testing.T) {
	t.Parallel()

	source := testSource()
	receiver := testReceiver()

	if got := DeriveBarrierGeometry(source.Point, source.SourceHeightM, receiver.Point, receiver.HeightM, nil); got != nil {
		t.Fatalf("an empty scene must derive no geometry, got %+v", *got)
	}
}

func TestExplicitBarrierGeometryWinsOverTheScene(t *testing.T) {
	t.Parallel()

	receiver := testReceiver()
	sources := []PointSource{testSource()}

	// A caller that computed its own geometry keeps it: the scene must not
	// quietly overrule a decision already taken.
	cfg := sceneConfig([]Barrier{wallAcross("wall", 50, 12)})
	sceneOnly := ComputeDownwindLevel(receiver, sources, cfg)

	explicit := DeriveBarrierGeometry(
		sources[0].Point, sources[0].SourceHeightM, receiver.Point, receiver.HeightM,
		[]Barrier{wallAcross("shallower", 50, 4)},
	)
	if explicit == nil {
		t.Fatal("expected a geometry to hand in")
	}

	cfg.Barrier = explicit

	withExplicit := ComputeDownwindLevel(receiver, sources, cfg)
	if !(withExplicit > sceneOnly) {
		t.Fatalf("the handed-in shallower geometry must win over the 12 m scene: %.6f vs %.6f", withExplicit, sceneOnly)
	}
}

func TestPropagationConfigRejectsAnInvalidSceneBarrier(t *testing.T) {
	t.Parallel()

	cfg := sceneConfig([]Barrier{{ID: "", Geometry: []geo.Point2D{{X: 0, Y: 0}, {X: 1, Y: 1}}, HeightM: 3}})

	if err := cfg.Validate(); err == nil {
		t.Fatal("a barrier without an id must be refused")
	}

	cfg = sceneConfig([]Barrier{{ID: "flat", Geometry: []geo.Point2D{{X: 0, Y: 0}, {X: 1, Y: 1}}, HeightM: 0}})

	if err := cfg.Validate(); err == nil {
		t.Fatal("a barrier with no height must be refused")
	}
}

// shortestDiffractedPath brute-forces min |SP| + |PR| over points P along a
// straight horizontal edge. It is the quantity Gl. 16 computes in closed form,
// so it is an independent oracle for the derivation rather than a restatement
// of it.
func shortestDiffractedPath(
	source geo.Point2D, sourceHeightM float64,
	receiver geo.Point2D, receiverHeightM float64,
	edgeA, edgeB geo.Point2D, topHeightM float64,
) float64 {
	dirX, dirY := edgeB.X-edgeA.X, edgeB.Y-edgeA.Y

	length := math.Hypot(dirX, dirY)
	dirX, dirY = dirX/length, dirY/length

	best := math.Inf(1)

	for step := -400000; step <= 400000; step++ {
		t := float64(step) * 0.002
		p := geo.Point2D{X: edgeA.X + t*dirX, Y: edgeA.Y + t*dirY}

		total := math.Hypot(geo.Distance(source, p), topHeightM-sourceHeightM) +
			math.Hypot(geo.Distance(p, receiver), receiverHeightM-topHeightM)
		if total < best {
			best = total
		}
	}

	return best
}

// TestObliqueScreenPathDifferenceMatchesTheShortestPath is the case every
// other barrier test in this package misses: a screen that does not stand
// square to the source-receiver ray.
//
// Gl. 16 decomposes the path into the plane perpendicular to the diffraction
// edge and the component a parallel to it. Measuring d_ss and d_sr along the
// ray and setting a = 0 instead over-states z for any oblique screen, which
// over-states D_z and under-states the level — the wrong direction for an
// immission assessment.
func TestObliqueScreenPathDifferenceMatchesTheShortestPath(t *testing.T) {
	t.Parallel()

	const (
		sourceHeightM   = 5
		receiverHeightM = 4
		topHeightM      = 9
	)

	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 200, Y: 0}

	for _, degrees := range []float64{90, 60, 45, 30, 20, 10} {
		radians := degrees * math.Pi / 180
		dirX, dirY := math.Cos(radians), math.Sin(radians)

		// A long wall through (10, 0), so it crosses the ray near the source
		// and every angle screens the same pair.
		edgeA := geo.Point2D{X: 10 - 400*dirX, Y: -400 * dirY}
		edgeB := geo.Point2D{X: 10 + 400*dirX, Y: 400 * dirY}

		barrier := Barrier{ID: "oblique", Geometry: []geo.Point2D{edgeA, edgeB}, HeightM: topHeightM}

		geometry := DeriveBarrierGeometry(source, sourceHeightM, receiver, receiverHeightM, []Barrier{barrier})
		if geometry == nil {
			t.Fatalf("%.0f deg: the wall crosses the ray and stands above it, so it must screen", degrees)
		}

		want := shortestDiffractedPath(source, sourceHeightM, receiver, receiverHeightM, edgeA, edgeB, topHeightM)
		got := diffractedPathLength(*geometry)

		if math.Abs(got-want) > 1e-3 {
			t.Errorf("%.0f deg: diffracted path %.6f m, want the shortest path %.6f m (difference %.6f)",
				degrees, got, want, got-want)
		}
	}
}

// TestObliqueScreenCarriesAParallelComponent pins the direction of the fix: a
// is zero only for a screen square to the ray, and grows as it turns away.
func TestObliqueScreenCarriesAParallelComponent(t *testing.T) {
	t.Parallel()

	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 100, Y: 0}

	previous := -1.0

	for _, degrees := range []float64{90, 60, 45, 30, 20} {
		radians := degrees * math.Pi / 180
		dirX, dirY := math.Cos(radians), math.Sin(radians)

		edgeA := geo.Point2D{X: 50 - 300*dirX, Y: -300 * dirY}
		edgeB := geo.Point2D{X: 50 + 300*dirX, Y: 300 * dirY}

		geometry := DeriveBarrierGeometry(source, 1, receiver, 2,
			[]Barrier{{ID: "oblique", Geometry: []geo.Point2D{edgeA, edgeB}, HeightM: 8}})
		if geometry == nil {
			t.Fatalf("%.0f deg: expected a geometry", degrees)
		}

		if degrees == 90 && geometry.A > 1e-9 {
			t.Fatalf("a screen square to the ray must carry no parallel component, got a = %.9f", geometry.A)
		}

		if previous >= 0 && geometry.A <= previous {
			t.Fatalf("%.0f deg: a = %.6f did not grow past the less oblique screen's %.6f", degrees, geometry.A, previous)
		}

		previous = geometry.A
	}
}
