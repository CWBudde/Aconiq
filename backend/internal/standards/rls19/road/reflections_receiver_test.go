package road

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// The receiver-side reflection search is only allowed to be faster than the
// source-side one it replaced, never different. These tests hold it to the
// reference in reflections_reference_test.go path by path — every field, to
// the bit, in order — over the synthetic city and over randomised scenes built
// to hit the degenerate cases a wedge margin could get wrong: sources and
// receivers on a wall's line, on a wall's corner, inside a courtyard, and walls
// that are collinear, share corners or are too low for Tabelle 8.

// exhaustiveStride returns 1 when ACONIQ_EXHAUSTIVE_REFLECTION_CHECK=1 asks
// for every pair to be compared, and stride otherwise. The reference search is
// the slow one, so CI — which also runs these under the race detector —
// compares a sample, and the exhaustive run is for a change to the search.
func exhaustiveStride(stride int) int {
	if os.Getenv("ACONIQ_EXHAUSTIVE_REFLECTION_CHECK") == "1" {
		return 1
	}

	return stride
}

// samePaths reports the first difference between two path lists, or "".
func samePaths(got, want []reflectedPath) string {
	if len(got) != len(want) {
		return fmt.Sprintf("%d paths, want %d", len(got), len(want))
	}

	for i := range want {
		g, w := got[i], want[i]
		if math.Float64bits(g.planDistM) != math.Float64bits(w.planDistM) ||
			math.Float64bits(g.slantDistM) != math.Float64bits(w.slantDistM) ||
			math.Float64bits(g.lossDB) != math.Float64bits(w.lossDB) ||
			math.Float64bits(g.imagePoint.X) != math.Float64bits(w.imagePoint.X) ||
			math.Float64bits(g.imagePoint.Y) != math.Float64bits(w.imagePoint.Y) ||
			g.reflectorClasses != w.reflectorClasses || g.bounces != w.bounces {
			return fmt.Sprintf("path %d: got %+v, want %+v", i, g, w)
		}
	}

	return ""
}

// pathComparison runs both searches over one field for many pairs.
type pathComparison struct {
	t       *testing.T
	field   *reflectorField
	scratch *pathScratch // shared by both searches, as a walk shares it

	pairs, first, second int
}

func newPathComparison(t *testing.T, field *reflectorField) *pathComparison {
	t.Helper()

	return &pathComparison{
		t:       t,
		field:   field,
		scratch: &pathScratch{wallCursor: field.grid.NewCursor()},
	}
}

// check compares the two searches for one pair, with the shared indexed
// scratch, and — when unindexed is set — the unindexed forms of both as well.
func (c *pathComparison) check(source geo.Point2D, sourceZ float64, receiver geo.Point2D, receiverZ float64, unindexed bool) {
	c.t.Helper()

	want := c.field.referencePaths(nil, source, sourceZ, receiver, receiverZ, c.scratch)
	got := c.field.appendPaths(nil, source, sourceZ, receiver, receiverZ, c.scratch)

	if diff := samePaths(got, want); diff != "" {
		c.t.Fatalf("source %v, receiver %v: %s", source, receiver, diff)
	}

	if unindexed {
		wantBare := c.field.referencePaths(nil, source, sourceZ, receiver, receiverZ, nil)
		if diff := samePaths(wantBare, want); diff != "" {
			c.t.Fatalf("source %v, receiver %v: the reference disagrees with itself unindexed: %s", source, receiver, diff)
		}

		gotBare := c.field.appendPaths(nil, source, sourceZ, receiver, receiverZ, nil)
		if diff := samePaths(gotBare, want); diff != "" {
			c.t.Fatalf("source %v, receiver %v, unindexed: %s", source, receiver, diff)
		}
	}

	c.pairs++

	for _, path := range want {
		if path.bounces == 1 {
			c.first++
		} else {
			c.second++
		}
	}
}

// requireBothOrders fails a comparison that never exercised one of the orders,
// because agreement on nothing proves nothing.
func (c *pathComparison) requireBothOrders() {
	c.t.Helper()

	if c.first == 0 || c.second == 0 {
		c.t.Fatalf("over %d pairs: %d first-order and %d second-order paths; the scene must produce both",
			c.pairs, c.first, c.second)
	}

	c.t.Logf("%d pairs, %d first-order and %d second-order paths compared", c.pairs, c.first, c.second)
}

// TestReceiverSearchMatchesReferenceOnCity walks every Teilstück of the small
// city against its receiver patch and a few receivers placed on purpose: in a
// courtyard, on a facade's line, on a building corner and well outside.
func TestReceiverSearchMatchesReferenceOnCity(t *testing.T) {
	t.Parallel()

	spec := smallCity()
	city := spec.build()

	scene, err := prepareScene(city.sources, city.barriers, city.config(6, ""))
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	receivers := make([]geo.Point2D, 0, len(city.receivers)+4)
	for _, receiver := range city.receivers {
		receivers = append(receivers, receiver.Point)
	}

	corner := city.buildings[3].Footprint[2]
	facade := city.buildings[5].Footprint
	receivers = append(
		receivers,
		spec.at(spec.pitchM/2, spec.pitchM/2), // the first block's courtyard
		corner,
		geo.Point2D{X: 2*facade[0].X - facade[1].X, Y: 2*facade[0].Y - facade[1].Y}, // on a facade's line
		spec.at(-400, 250),
	)

	comparison := newPathComparison(t, &scene.reflectors)
	stride := exhaustiveStride(61)

	for r, receiver := range receivers {
		count := r // a different sample for every receiver

		for _, source := range scene.sources {
			for _, seg := range source.segments {
				if count%stride == 0 {
					comparison.check(seg.MidPoint, seg.MidZ+pointSourceHeightM, receiver, 4, count%(7*stride) == 0)
				}

				count++
			}
		}
	}

	comparison.requireBothOrders()
}

// TestReceiverSearchMatchesReferenceOnFullCity samples the benchmark scene,
// which is too large to walk exhaustively against the reference. It is
// opt-in: building one receiver's entries on the full city takes a second,
// and the race detector multiplies that.
func TestReceiverSearchMatchesReferenceOnFullCity(t *testing.T) {
	t.Parallel()

	if exhaustiveStride(2) != 1 {
		t.Skip("set ACONIQ_EXHAUSTIVE_REFLECTION_CHECK=1 to compare on the full city")
	}

	stride := 97

	city := fullCity().build()

	scene, err := prepareScene(city.sources, city.barriers, city.config(10, ""))
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	comparison := newPathComparison(t, &scene.reflectors)

	for _, receiver := range []geo.PointReceiver{city.receivers[0], city.receivers[len(city.receivers)-1]} {
		count := 0

		for _, source := range scene.sources {
			for _, seg := range source.segments {
				if count%stride == 0 {
					comparison.check(seg.MidPoint, seg.MidZ+pointSourceHeightM, receiver.Point, 4, false)
				}

				count++
			}
		}
	}

	comparison.requireBothOrders()
}

// randomReflectorScene is one seeded scene of free-standing walls and
// buildings, with the sources and receivers to test it from.
type randomReflectorScene struct {
	reflectors []Reflector
	sources    []geo.Point2D
	receivers  []geo.Point2D
}

// newRandomReflectorScene draws a scene in a 300 m box, at the origin or at
// UTM magnitudes. Besides random geometry it plants the configurations a
// margin could get wrong: collinear walls, walls sharing a corner, walls below
// Tabelle 8's 1.0 m, a closed courtyard, and sources and receivers exactly on
// wall lines and corners.
func newRandomReflectorScene(seed uint64) randomReflectorScene {
	rng := rand.New(rand.NewPCG(seed, 0x0dd))

	origin := geo.Point2D{}
	if seed%2 == 1 {
		origin = geo.Point2D{X: 548_000 + uniform(rng, 0, 1000), Y: 5_802_000 + uniform(rng, 0, 1000)}
	}

	at := func(x, y float64) geo.Point2D { return geo.Point2D{X: origin.X + x, Y: origin.Y + y} }
	box := func() geo.Point2D { return at(uniform(rng, 0, 300), uniform(rng, 0, 300)) }

	var scene randomReflectorScene

	addWall := func(id string, height float64, points ...geo.Point2D) {
		scene.reflectors = append(scene.reflectors, Reflector{ID: id, Geometry: points, HeightM: height})
	}

	// Free-standing walls and polylines, some of them too low to count.
	for i := range 12 + rng.IntN(12) {
		start := box()
		points := make([]geo.Point2D, 0, 4)
		points = append(points, start)

		for range 1 + rng.IntN(3) {
			angle := uniform(rng, 0, 2*math.Pi)
			length := uniform(rng, 1, 60)
			last := points[len(points)-1]
			points = append(points, geo.Point2D{X: last.X + length*math.Cos(angle), Y: last.Y + length*math.Sin(angle)})
		}

		height := uniform(rng, 1, 25)
		if rng.IntN(6) == 0 {
			height = uniform(rng, 0.3, 1)
		}

		addWall(fmt.Sprintf("wall-%d", i), height, points...)
	}

	// Collinear walls along one line, with gaps, and one sharing a corner.
	lineStart := box()
	angle := uniform(rng, 0, 2*math.Pi)
	dir := geo.Point2D{X: math.Cos(angle), Y: math.Sin(angle)}
	along := func(t float64) geo.Point2D { return geo.Point2D{X: lineStart.X + t*dir.X, Y: lineStart.Y + t*dir.Y} }

	addWall("collinear-0", 12, along(0), along(20))
	addWall("collinear-1", 9, along(25), along(55))
	addWall("collinear-2", 15, along(55), along(70))
	addWall("corner", 10, along(70), geo.Point2D{X: along(70).X - 25*dir.Y, Y: along(70).Y + 25*dir.X})

	// A closed courtyard: four row buildings around an open yard.
	yard := box()
	courtyard := [][]geo.Point2D{
		{{X: 0, Y: 0}, {X: 60, Y: 0}, {X: 60, Y: 12}, {X: 0, Y: 12}},
		{{X: 0, Y: 48}, {X: 60, Y: 48}, {X: 60, Y: 60}, {X: 0, Y: 60}},
		{{X: 0, Y: 12}, {X: 12, Y: 12}, {X: 12, Y: 48}, {X: 0, Y: 48}},
		{{X: 48, Y: 12}, {X: 60, Y: 12}, {X: 60, Y: 48}, {X: 48, Y: 48}},
	}

	for i, footprint := range courtyard {
		building := Building{ID: fmt.Sprintf("yard-%d", i), HeightM: uniform(rng, 6, 20)}
		for _, p := range footprint {
			building.Footprint = append(building.Footprint, geo.Point2D{X: yard.X + p.X, Y: yard.Y + p.Y})
		}

		scene.reflectors = append(scene.reflectors, building.asReflector())
	}

	// Sources and receivers: random, on a wall's line and its extension, on
	// a corner, and in the yard.
	onLine := func() geo.Point2D {
		wall := scene.reflectors[rng.IntN(len(scene.reflectors))]
		k := rng.IntN(len(wall.Geometry) - 1)
		a, b := wall.Geometry[k], wall.Geometry[k+1]
		t := []float64{-0.75, -0.1, 0, 0.5, 1, 1.3, 2.5}[rng.IntN(7)]

		return geo.Point2D{X: a.X + t*(b.X-a.X), Y: a.Y + t*(b.Y-a.Y)}
	}

	for range 10 {
		scene.sources = append(scene.sources, box())
	}

	for range 6 {
		scene.sources = append(scene.sources, onLine())
	}

	scene.sources = append(scene.sources, along(-15), along(90), along(70))

	for range 3 {
		scene.receivers = append(scene.receivers, box())
	}

	scene.receivers = append(
		scene.receivers,
		geo.Point2D{X: yard.X + 30, Y: yard.Y + 30},
		onLine(),
		along(22.5), // between two collinear walls, on their line
		along(25),   // on a wall's end
	)

	return scene
}

// TestReceiverSearchMatchesReferenceOnRandomScenes compares the searches over
// many seeded scenes.
func TestReceiverSearchMatchesReferenceOnRandomScenes(t *testing.T) {
	t.Parallel()

	var total pathComparison

	total.t = t

	seeds := uint64(12)
	if exhaustiveStride(2) == 1 {
		seeds = 200
	}

	for seed := range seeds {
		scene := newRandomReflectorScene(seed)
		field := newReflectorField(scene.reflectors)
		comparison := newPathComparison(t, &field)

		for _, receiver := range scene.receivers {
			for s, source := range scene.sources {
				for _, heights := range [][2]float64{{0.5, 4}, {3, 1.5}, {0.5, 22}} {
					comparison.check(source, heights[0], receiver, heights[1], s%7 == 0)
				}
			}
		}

		total.pairs += comparison.pairs
		total.first += comparison.first
		total.second += comparison.second
	}

	total.requireBothOrders()
}

// TestReceiverSearchSurvivesReceiverChanges reuses one scratch across
// receivers in an order that revisits them, so a stale receiver-side table
// would be read for the wrong receiver.
func TestReceiverSearchSurvivesReceiverChanges(t *testing.T) {
	t.Parallel()

	scene := newRandomReflectorScene(3)
	field := newReflectorField(scene.reflectors)
	comparison := newPathComparison(t, &field)

	for round := range 3 {
		for r := range scene.receivers {
			receiver := scene.receivers[(r*(round+2))%len(scene.receivers)]
			for _, source := range scene.sources[:6] {
				comparison.check(source, 0.5, receiver, 4, false)
			}
		}
	}

	comparison.requireBothOrders()
}
