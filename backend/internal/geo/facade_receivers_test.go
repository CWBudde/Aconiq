package geo

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// rect is a closed counter-clockwise rectangle ring.
func rect(x0, y0, x1, y1 float64) []Point2D {
	return []Point2D{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}, {X: x0, Y: y0}}
}

// reversed returns a ring walked the other way, still closed.
func reversed(ring []Point2D) []Point2D {
	out := slices.Clone(ring)
	slices.Reverse(out)

	return out
}

// rotated returns a closed ring that starts k vertices later.
func rotated(ring []Point2D, k int) []Point2D {
	open := ring[:len(ring)-1]
	out := append(slices.Clone(open[k:]), open[:k]...)

	return append(out, out[0])
}

func house(id string, rings ...[]Point2D) FacadeBuilding {
	return FacadeBuilding{
		ID:           id,
		Polygons:     [][][]Point2D{rings},
		HeightM:      9,
		Levels:       3,
		HeightSource: SlabHeightMeasured,
	}
}

func facadeSet(rule string, spacingM float64, ids ...string) FacadeReceiverSet {
	return FacadeReceiverSet{ID: "set", BuildingIDs: ids, PlacementRule: rule, SpacingM: spacingM}
}

func mustGenerate(t *testing.T, set FacadeReceiverSet, buildings ...FacadeBuilding) []FacadeReceiver {
	t.Helper()

	receivers, err := set.Generate(buildings)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	return receivers
}

// onEdge returns the receivers on one edge of the first part's exterior.
func onEdge(receivers []FacadeReceiver, edge int) []FacadeReceiver {
	var out []FacadeReceiver

	for _, r := range receivers {
		if r.Facade.Part == 0 && r.Facade.Ring == 0 && r.Facade.Edge == edge {
			out = append(out, r)
		}
	}

	return out
}

func TestFacadePointRightHandNormal(t *testing.T) {
	t.Parallel()

	// A counter-clockwise exterior edge along +x has the building on its
	// left, so outward is −y.
	got := FacadePoint(Point2D{X: 0, Y: 0}, Point2D{X: 10, Y: 0}, 2, 0.05)
	if got != (Point2D{X: 2, Y: -0.05}) {
		t.Fatalf("FacadePoint = %+v, want (2, -0.05)", got)
	}

	got = FacadePoint(Point2D{X: 10, Y: 0}, Point2D{X: 10, Y: 8}, 3, 0.5)
	if got != (Point2D{X: 10.5, Y: 3}) {
		t.Fatalf("FacadePoint = %+v, want (10.5, 3)", got)
	}
}

// n = ceil(L / spacing) equal parts, a station at the centre of each.
func TestFacadeReceiversStationsAtPartCentres(t *testing.T) {
	t.Parallel()

	receivers := mustGenerate(t, facadeSet(PlacementRLS19Facade, 4, "b1"), house("b1", rect(0, 0, 10, 8)))

	cases := []struct {
		edge int
		want []float64
	}{
		{edge: 0, want: []float64{10.0 / 6, 5, 50.0 / 6}}, // 10 m: three parts of 10/3 m
		{edge: 1, want: []float64{2, 6}},                  // 8 m: two parts of 4 m
	}

	for _, tc := range cases {
		var got []float64

		for _, r := range onEdge(receivers, tc.edge) {
			if r.Storey == 0 {
				got = append(got, r.StationUM)
			}
		}

		if len(got) != len(tc.want) {
			t.Fatalf("edge %d: stations %v, want %v", tc.edge, got, tc.want)
		}

		for i := range got {
			if math.Abs(got[i]-tc.want[i]) > 1e-12 {
				t.Fatalf("edge %d: stations %v, want %v", tc.edge, got, tc.want)
			}
		}
	}

	// Four edges, 3 + 2 + 3 + 2 stations, three storeys each.
	if len(receivers) != 30 {
		t.Fatalf("got %d receivers, want 30", len(receivers))
	}
}

// The offset comes from the ring's orientation: 5 cm out is outside the
// footprint under both point-in-polygon variants, and Schall 03's 0 m point
// lies on the boundary, where they disagree.
func TestFacadeReceiversStandOutsideTheirFacade(t *testing.T) {
	t.Parallel()

	ring := rect(0, 0, 10, 8)
	footprint := [][]Point2D{ring}

	for _, input := range [][]Point2D{ring, reversed(ring)} {
		for _, r := range mustGenerate(t, facadeSet(PlacementRLS19Facade, 3, "b1"), house("b1", input)) {
			if PointInPolygon(r.Point, footprint) {
				t.Fatalf("%s at %+v is inside the footprint", r.ID, r.Point)
			}

			if d := pointSegmentDistance(r.Point, r.Facade.From, r.Facade.To); math.Abs(d-0.05) > 1e-12 {
				t.Fatalf("%s is %.15f m from its facade, want 0.05", r.ID, d)
			}
		}
	}

	for _, r := range mustGenerate(t, facadeSet(PlacementSchall03Facade, 3, "b1"), house("b1", ring)) {
		if !PointInPolygon(r.Point, footprint) || PointInPolygonInterior(r.Point, footprint) {
			t.Fatalf("%s at %+v is not on the footprint boundary", r.ID, r.Point)
		}
	}
}

func pointSegmentDistance(p, a, b Point2D) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	t = max(0, min(1, t))

	return Distance(p, Point2D{X: a.X + t*dx, Y: a.Y + t*dy})
}

// The same building written with another start vertex, the other
// orientation, its parts and its holes in another order gives the same
// receivers to the bit.
func TestFacadeReceiversSurviveRewriting(t *testing.T) {
	t.Parallel()

	outer := rect(0, 0, 20, 12)
	holeA := reversed(rect(2, 2, 6, 5))
	holeB := reversed(rect(10, 3, 16, 9))
	annex := rect(30, 0, 36, 4)

	set := facadeSet(PlacementRLS19Facade, 2.5, "b1")
	set.IncludeCourtyard = true

	written := FacadeBuilding{
		ID: "b1", HeightM: 9, Levels: 3, HeightSource: SlabHeightMeasured,
		Polygons: [][][]Point2D{{outer, holeA, holeB}, {annex}},
	}
	rewritten := written
	rewritten.Polygons = [][][]Point2D{
		{reversed(rotated(annex, 2))},
		{rotated(reversed(outer), 3), rotated(holeB, 1), reversed(holeA)},
	}

	want := mustGenerate(t, set, written)
	got := mustGenerate(t, set, rewritten)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rewriting the building changed its receivers:\n got %v\nwant %v", ids(got), ids(want))
	}

	// The canonical order is fixed: the exterior starts at its smallest
	// vertex, so edge 0 runs from (0, 0) to (20, 0).
	e0 := onEdge(want, 0)
	if len(e0) == 0 || e0[0].Facade.From != (Point2D{X: 0, Y: 0}) || e0[0].Facade.To != (Point2D{X: 20, Y: 0}) {
		t.Fatalf("exterior edge 0 = %+v, want (0,0) → (20,0)", e0)
	}
}

func ids(receivers []FacadeReceiver) []string {
	out := make([]string, len(receivers))
	for i, r := range receivers {
		out[i] = r.ID
	}

	return out
}

func TestFacadeReceiversStoreyHeights(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		heightM    float64
		levels     int
		source     string
		groundM    float64
		wantZ      []float64
		wantSource string
	}{
		{"measured height over levels", 14, 4, SlabHeightMeasured, 0, []float64{3.5, 7, 10.5, 14}, SlabHeightMeasured},
		{"levels-derived height", 12, 4, SlabHeightLevels, 0, []float64{3, 6, 9, 12}, SlabHeightLevels},
		{"assumed height with levels", 9, 3, SlabHeightAssumed, 0, []float64{3, 6, 9}, SlabHeightLevels},
		{"no storey count", 10, 0, SlabHeightMeasured, 0, []float64{3, 6, 9}, SlabHeightAssumed},
		{"no storey count, low building", 2, 0, SlabHeightAssumed, 0, []float64{3}, SlabHeightAssumed},
		{"raised ground floor", 9, 3, SlabHeightLevels, 0.6, []float64{3.6, 6.6, 9.6}, SlabHeightLevels},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := FacadeBuilding{
				ID: "b1", Polygons: [][][]Point2D{{rect(0, 0, 4, 4)}},
				HeightM: tc.heightM, Levels: tc.levels, HeightSource: tc.source, GroundFloorM: tc.groundM,
			}

			var got []float64

			for _, r := range onEdge(mustGenerate(t, facadeSet(PlacementRLS19Facade, 10, "b1"), b), 0) {
				if r.SlabHeightSource != tc.wantSource {
					t.Fatalf("slab_height_source = %q, want %q", r.SlabHeightSource, tc.wantSource)
				}

				if r.Storey != len(got) {
					t.Fatalf("storey %d out of order", r.Storey)
				}

				got = append(got, r.HeightM)
			}

			if len(got) != len(tc.wantZ) {
				t.Fatalf("heights %v, want %v", got, tc.wantZ)
			}

			for i := range got {
				if math.Abs(got[i]-tc.wantZ[i]) > 1e-12 {
					t.Fatalf("heights %v, want %v", got, tc.wantZ)
				}
			}
		})
	}
}

// A party wall carries no receivers: their probe lands inside the
// neighbour. Where the neighbour is shorter, the free part keeps its
// stations.
func TestFacadeReceiversSkipPartyWalls(t *testing.T) {
	t.Parallel()

	a := house("a", rect(0, 0, 10, 8))
	full := house("b", rect(10, 0, 20, 8))
	short := house("b", rect(10, 0, 20, 4))

	// Edge 1 of a is its east wall, x = 10 from (10, 0) to (10, 8).
	set := facadeSet(PlacementRLS19Facade, 2, "a")

	if got := onEdge(mustGenerate(t, set, a, full), 1); len(got) != 0 {
		t.Fatalf("party wall carries %d receivers, want none: %v", len(got), ids(got))
	}

	var stations []float64

	for _, r := range onEdge(mustGenerate(t, set, a, short), 1) {
		if r.Storey == 0 {
			stations = append(stations, r.StationUM)
		}
	}

	if !slices.Equal(stations, []float64{5, 7}) {
		t.Fatalf("half-shared wall keeps stations %v, want [5 7]", stations)
	}

	// The neighbour is only masked, never given receivers of its own.
	for _, r := range mustGenerate(t, set, a, full) {
		if r.Facade.BuildingRef != "a" {
			t.Fatalf("receiver %s belongs to an unselected building", r.ID)
		}
	}

	// Schall 03's 0 m point stands on the party wall itself, which the
	// strict-interior mask would keep; the probe does not.
	if got := onEdge(mustGenerate(t, facadeSet(PlacementSchall03Facade, 2, "a"), a, full), 1); len(got) != 0 {
		t.Fatalf("Schall 03 party wall carries %d receivers, want none", len(got))
	}
}

func TestFacadeReceiversCourtyard(t *testing.T) {
	t.Parallel()

	hole := reversed(rect(3, 3, 7, 5))
	b := house("b1", rect(0, 0, 10, 8), hole)

	set := facadeSet(PlacementRLS19Facade, 2, "b1")

	for _, r := range mustGenerate(t, set, b) {
		if r.Facade.Ring != 0 {
			t.Fatalf("courtyard receiver %s without include_courtyard", r.ID)
		}
	}

	set.IncludeCourtyard = true

	receivers := mustGenerate(t, set, b)
	courtyard := 0

	for _, r := range receivers {
		if r.Facade.Ring != 1 {
			continue
		}

		courtyard++

		// Outward from the building is into the courtyard.
		if !PointInPolygonInterior(r.Point, [][]Point2D{rect(3, 3, 7, 5)}) {
			t.Fatalf("courtyard receiver %s at %+v is not in the courtyard", r.ID, r.Point)
		}
	}

	if courtyard == 0 {
		t.Fatal("no courtyard receivers with include_courtyard")
	}

	assertUniqueIDs(t, receivers)
}

func assertUniqueIDs(t *testing.T, receivers []FacadeReceiver) {
	t.Helper()

	seen := make(map[string]bool, len(receivers))
	for _, r := range receivers {
		if seen[r.ID] {
			t.Fatalf("duplicate receiver id %q", r.ID)
		}

		seen[r.ID] = true
	}
}

// IDs carry the whole key in a fixed order, and so does the output order.
func TestFacadeReceiversIDsAndOrder(t *testing.T) {
	t.Parallel()

	set := facadeSet(PlacementRLS19Facade, 4, "b2", "b1")
	set.IncludeCourtyard = true

	buildings := []FacadeBuilding{
		house("b1", rect(0, 0, 10, 8), reversed(rect(3, 3, 7, 5))),
		house("b2", rect(30, 0, 40, 8)),
	}

	receivers := mustGenerate(t, set, buildings...)
	assertUniqueIDs(t, receivers)

	first := receivers[0]
	if first.ID != "set/b1/p0r0e0/s0/f0" {
		t.Fatalf("first id = %q, want set/b1/p0r0e0/s0/f0", first.ID)
	}

	if first.PlacementRule != PlacementRLS19Facade || first.ReviewStatus != ReviewStatusProposed {
		t.Fatalf("first receiver = %+v, want rule %q and status %q", first, PlacementRLS19Facade, ReviewStatusProposed)
	}

	if !slices.IsSortedFunc(receivers, compareFacadeReceivers) {
		t.Fatal("receivers are not in key order")
	}

	// Courtyard edge 0 and exterior edge 0 at the same station and storey
	// are different receivers.
	if !slices.ContainsFunc(receivers, func(r FacadeReceiver) bool { return r.ID == "set/b1/p0r1e0/s0/f0" }) {
		t.Fatal("no courtyard edge 0 receiver")
	}

	// Swapping the input order changes nothing.
	slices.Reverse(buildings)

	set.BuildingIDs = []string{"b1", "b2"}

	if again := mustGenerate(t, set, buildings...); !reflect.DeepEqual(again, receivers) {
		t.Fatal("input order changed the receivers")
	}
}

func TestFacadeReceiverSetValidate(t *testing.T) {
	t.Parallel()

	valid := facadeSet(PlacementRLS19Facade, 3, "b1")
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid set: %v", err)
	}

	cases := map[string]FacadeReceiverSet{
		"no id":         {BuildingIDs: []string{"b1"}, PlacementRule: PlacementRLS19Facade, SpacingM: 3},
		"no building":   {ID: "set", PlacementRule: PlacementRLS19Facade, SpacingM: 3},
		"blank id":      facadeSet(PlacementRLS19Facade, 3, " "),
		"twice":         facadeSet(PlacementRLS19Facade, 3, "b1", "b1"),
		"unknown rule":  facadeSet("rls19-window", 3, "b1"),
		"no rule":       facadeSet("", 3, "b1"),
		"zero spacing":  facadeSet(PlacementRLS19Facade, 0, "b1"),
		"nan spacing":   facadeSet(PlacementRLS19Facade, math.NaN(), "b1"),
		"inf spacing":   facadeSet(PlacementRLS19Facade, math.Inf(1), "b1"),
		"negative step": facadeSet(PlacementRLS19Facade, -1, "b1"),
	}

	for name, tc := range cases {
		if err := tc.Validate(); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}

	// The rules that need a window or an area are known, and refused by
	// name rather than as unknown.
	for _, rule := range []string{"rls19-balcony", "rls19-outdoor-area", "schall03-outdoor-area", "talaerm-window", "talaerm-buildable-edge"} {
		err := facadeSet(rule, 3, "b1").Validate()
		if !errors.Is(err, ErrPlacementRuleNotGeometric) {
			t.Errorf("rule %q: err = %v, want ErrPlacementRuleNotGeometric", rule, err)
		}
	}
}

func TestFacadeReceiverSetGenerateRefuses(t *testing.T) {
	t.Parallel()

	set := facadeSet(PlacementRLS19Facade, 3, "b1")
	ok := house("b1", rect(0, 0, 10, 8))

	with := func(edit func(*FacadeBuilding)) FacadeBuilding {
		b := ok
		edit(&b)

		return b
	}

	cases := map[string][]FacadeBuilding{
		"unknown building":    {house("b2", rect(0, 0, 10, 8))},
		"duplicate building":  {ok, ok},
		"no height":           {with(func(b *FacadeBuilding) { b.HeightM = 0 })},
		"nan height":          {with(func(b *FacadeBuilding) { b.HeightM = math.NaN() })},
		"negative levels":     {with(func(b *FacadeBuilding) { b.Levels = -1 })},
		"unknown source":      {with(func(b *FacadeBuilding) { b.HeightSource = "tagged" })},
		"nan ground floor":    {with(func(b *FacadeBuilding) { b.GroundFloorM = math.NaN() })},
		"no polygon":          {with(func(b *FacadeBuilding) { b.Polygons = nil })},
		"flat ring":           {with(func(b *FacadeBuilding) { b.Polygons = [][][]Point2D{{{{X: 0}, {X: 5}, {X: 10}, {X: 0}}}} })},
		"non-finite vertex":   {with(func(b *FacadeBuilding) { b.Polygons = [][][]Point2D{{rect(0, 0, math.Inf(1), 8)}} })},
		"neighbour flat ring": {ok, {ID: "n", Polygons: [][][]Point2D{{{{X: 0}, {X: 5}, {X: 0}}}}}},
	}

	for name, buildings := range cases {
		_, err := set.Generate(buildings)
		if err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}

	// A finite spacing or height can still ask for more receivers than an int
	// holds or memory takes; the count is refused before anything is allocated.
	for name, tc := range map[string]struct {
		spacingM float64
		building FacadeBuilding
	}{
		"overflowing spacing":   {1e-300, ok},
		"too fine a spacing":    {1e-3, ok},
		"overflowing height":    {3, with(func(b *FacadeBuilding) { b.HeightM, b.Levels = 1e300, 0 })},
		"too many levels":       {3, with(func(b *FacadeBuilding) { b.Levels = MaxFacadeReceivers + 1 })},
		"budget across storeys": {0.5, with(func(b *FacadeBuilding) { b.Levels = MaxFacadeReceivers / 50 })},
	} {
		_, err := facadeSet(PlacementRLS19Facade, tc.spacingM, "b1").Generate([]FacadeBuilding{tc.building})
		if err == nil || !strings.Contains(err.Error(), "receivers") {
			t.Errorf("%s: err = %v, want the receiver limit", name, err)
		}
	}

	if _, err := facadeSet("talaerm-window", 3, "b1").Generate([]FacadeBuilding{ok}); err == nil ||
		!strings.Contains(err.Error(), "talaerm-window") {
		t.Errorf("non-geometric rule: err = %v, want it named", err)
	}
}
