package geo

import (
	"testing"
)

// facadeEdgeBlock is a MultiPolygon building: a main block with two courtyard
// holes and a detached annex, at synthetic UTM-like coordinates.
func facadeEdgeBlock() FacadeBuilding {
	const x0, y0 = 500000.0, 5800000.0

	return FacadeBuilding{
		ID: "b1", HeightM: 9, Levels: 3, HeightSource: SlabHeightMeasured,
		Polygons: [][][]Point2D{
			{
				rect(x0, y0, x0+20, y0+12),
				reversed(rect(x0+2, y0+2, x0+6, y0+5)),
				reversed(rect(x0+10, y0+3, x0+16, y0+9)),
			},
			{rect(x0+30, y0, x0+36, y0+4)},
		},
	}
}

// FacadeEdge names the same endpoints Generate stamps on every receiver, on
// exterior, courtyard and annex edges alike.
func TestFacadeEdgeAgreesWithGenerate(t *testing.T) {
	t.Parallel()

	b := facadeEdgeBlock()
	set := facadeSet(PlacementRLS19Facade, 2.5, "b1")
	set.IncludeCourtyard = true

	receivers := mustGenerate(t, set, b)

	seen := map[[3]int]bool{}

	for _, r := range receivers {
		from, to, ok := FacadeEdge(b.Polygons, r.Facade.Part, r.Facade.Ring, r.Facade.Edge)
		if !ok {
			t.Fatalf("FacadeEdge(%d, %d, %d) not ok for receiver %s", r.Facade.Part, r.Facade.Ring, r.Facade.Edge, r.ID)
		}

		if from != r.Facade.From || to != r.Facade.To {
			t.Fatalf("FacadeEdge(%d, %d, %d) = %+v → %+v, receiver %s says %+v → %+v",
				r.Facade.Part, r.Facade.Ring, r.Facade.Edge, from, to, r.ID, r.Facade.From, r.Facade.To)
		}

		seen[[3]int{r.Facade.Part, r.Facade.Ring, r.Facade.Edge}] = true
	}

	// Two parts; the main block has an exterior and two holes of four
	// edges each, the annex four exterior edges.
	if len(seen) != 16 {
		t.Fatalf("receivers cover %d edges, want 16 (the test would not exercise every ring)", len(seen))
	}
}

// The edge a key names does not depend on how the footprint is written.
func TestFacadeEdgeSurvivesRewriting(t *testing.T) {
	t.Parallel()

	written := facadeEdgeBlock()
	main, annex := written.Polygons[0], written.Polygons[1]
	rewritten := [][][]Point2D{
		{reversed(rotated(annex[0], 2))},
		{rotated(reversed(main[0]), 3), rotated(main[2], 1), reversed(main[1])},
	}

	for part := range 2 {
		for ring := range 3 {
			for edge := range 4 {
				wantFrom, wantTo, wantOK := FacadeEdge(written.Polygons, part, ring, edge)
				gotFrom, gotTo, gotOK := FacadeEdge(rewritten, part, ring, edge)

				if gotOK != wantOK || gotFrom != wantFrom || gotTo != wantTo {
					t.Fatalf("edge (%d, %d, %d): rewritten %+v → %+v (%v), written %+v → %+v (%v)",
						part, ring, edge, gotFrom, gotTo, gotOK, wantFrom, wantTo, wantOK)
				}
			}
		}
	}

	// The canonical exterior starts at its smallest vertex and runs
	// counter-clockwise.
	from, to, ok := FacadeEdge(rewritten, 0, 0, 0)
	if !ok || from != (Point2D{X: 500000, Y: 5800000}) || to != (Point2D{X: 500020, Y: 5800000}) {
		t.Fatalf("FacadeEdge(0, 0, 0) = %+v → %+v (%v), want (500000, 5800000) → (500020, 5800000)", from, to, ok)
	}

	// The last edge closes the ring.
	from, to, ok = FacadeEdge(rewritten, 0, 0, 3)
	if !ok || from != (Point2D{X: 500000, Y: 5800012}) || to != (Point2D{X: 500000, Y: 5800000}) {
		t.Fatalf("FacadeEdge(0, 0, 3) = %+v → %+v (%v), want the closing edge", from, to, ok)
	}
}

func TestFacadeEdgeRefuses(t *testing.T) {
	t.Parallel()

	polygons := facadeEdgeBlock().Polygons

	cases := []struct {
		name             string
		polygons         [][][]Point2D
		part, ring, edge int
	}{
		{"negative part", polygons, -1, 0, 0},
		{"part out of range", polygons, 2, 0, 0},
		{"negative ring", polygons, 0, -1, 0},
		{"ring out of range", polygons, 1, 1, 0},
		{"negative edge", polygons, 0, 0, -1},
		{"edge out of range", polygons, 0, 0, 4},
		{"no footprint", nil, 0, 0, 0},
		{"degenerate ring", [][][]Point2D{{{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 0}}}}, 0, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			from, to, ok := FacadeEdge(tc.polygons, tc.part, tc.ring, tc.edge)
			if ok {
				t.Fatalf("FacadeEdge = %+v → %+v, want not ok", from, to)
			}
		})
	}
}

func TestIsPlacementRule(t *testing.T) {
	t.Parallel()

	for _, rule := range []string{
		PlacementRLS19Facade, PlacementSchall03Facade,
		"rls19-balcony", "rls19-outdoor-area", "schall03-outdoor-area", "talaerm-window", "talaerm-buildable-edge",
	} {
		if !IsPlacementRule(rule) {
			t.Fatalf("IsPlacementRule(%q) = false", rule)
		}
	}

	for _, rule := range []string{"", "custom", "RLS19-FACADE", " rls19-facade"} {
		if IsPlacementRule(rule) {
			t.Fatalf("IsPlacementRule(%q) = true", rule)
		}
	}
}
