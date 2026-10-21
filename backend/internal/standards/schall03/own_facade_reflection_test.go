package schall03_test

import (
	"slices"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// Own-facade reflections at a facade receiver (PLAN.md Priority 18 B1,
// Priority 1 §1.9).
//
// Anlage 2 Nr. 2.2.10 of the 16. BImSchV puts the Immissionsort at a building
// "auf der Fassade", 0.2 m above the window head, and excludes ALL reflections
// at the associated facade — not only the last one, as RLS-19 does. Nothing in
// this module links a receiver to a facade, so every path that touches the
// receiver's own facade is counted, at every order up to the third.
//
// These tests measure what that does and PIN it. The bands are NOT a
// conformance tolerance and must never be read as one: they hold today's
// defective state, about ±0.25 dB around the measured value, so it cannot
// change unnoticed. When §1.9 is fixed, each test flips to asserting the
// exclusion (and, for the on-facade point, that its own facade does not
// shield it).
//
// Measured on 2026-10-22, LpAeq day, one ICE-1-Zug operation:
//
//	0.05 m in front, facade as walls + footprint      +3.238 dB over free field
//	0.05 m in front, facade as one wall               +2.519 dB
//	0 m (the Nr. 2.2.10 position), footprint only     −19.076 dB: the own facade shields the direct path
//	0 m, facade as walls + footprint                  −3.110 dB
//	canyon: 5 of 6 paths touch the own facade, 3 end on it; removing it −2.607 dB
//	coplanar neighbour wall, 0.10 m from the party wall  +0.030 dB on top of the own facade
//	coplanar neighbour building                       −0.474 dB (it shields the own image)

const (
	ownFacadeY      = 20.0
	facadeReceiverZ = 3.0
	facadeTopM      = 10.0
	pinnedBandDB    = 0.25
)

func facadeTrack(t *testing.T, halfLengthM float64) schall03.TrackSegment {
	t.Helper()

	op, err := schall03.NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatal(err)
	}

	return schall03.TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -halfLengthM, Y: 0}, {X: halfLengthM, Y: 0}},
		Fahrbahn:        schall03.FahrbahnartSchwellengleis,
		Surface:         schall03.SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []schall03.TrainOperation{*op},
	}
}

// facadeHouse is a 10 m deep, 10 m high block whose street facade lies on
// y = ownFacadeY between x0 and x1, as reflecting walls and as a footprint,
// sharing one ObstacleID the way the CLI builds them. Wall 0 is the street
// facade, wall 2 the back wall.
func facadeHouse(id string, x0, x1 float64) ([]schall03.ReflectingWall, []schall03.BarrierSegment) {
	ring := []geo.Point2D{
		{X: x0, Y: ownFacadeY},
		{X: x1, Y: ownFacadeY},
		{X: x1, Y: ownFacadeY + 10},
		{X: x0, Y: ownFacadeY + 10},
		{X: x0, Y: ownFacadeY},
	}

	walls := make([]schall03.ReflectingWall, 0, len(ring)-1)
	footprint := make([]schall03.BarrierSegment, 0, len(ring)-1)

	for i := range len(ring) - 1 {
		walls = append(walls, schall03.ReflectingWall{
			A: ring[i], B: ring[i+1], HeightM: facadeTopM,
			Surface: schall03.WallSurfaceBuilding, ObstacleID: id,
		})
		footprint = append(footprint, schall03.BarrierSegment{
			A: ring[i], B: ring[i+1], TopHeightM: facadeTopM, ObstacleID: id,
		})
	}

	return walls, footprint
}

// facadeWall is one reflecting wall with no footprint behind it.
func facadeWall(x0, x1, y, heightM float64) schall03.ReflectingWall {
	return schall03.ReflectingWall{
		A: geo.Point2D{X: x0, Y: y}, B: geo.Point2D{X: x1, Y: y},
		HeightM: heightM, Surface: schall03.WallSurfaceBuilding,
	}
}

func facadeLevel(
	t *testing.T, receiver geo.Point2D, track schall03.TrackSegment,
	walls []schall03.ReflectingWall, footprint []schall03.BarrierSegment,
) float64 {
	t.Helper()

	result, err := schall03.ComputeNormativeReceiverLevelsWithScene(
		schall03.ReceiverInput{ID: "r1", Point: receiver, HeightM: facadeReceiverZ},
		[]schall03.TrackSegment{track}, walls, footprint,
	)
	if err != nil {
		t.Fatal(err)
	}

	return result.LpAeqDay
}

func assertPinned(t *testing.T, what string, got, measured float64) {
	t.Helper()

	t.Logf("%s: %+.3f dB (pinned at %+.3f)", what, got, measured)

	if got < measured-pinnedBandDB || got > measured+pinnedBandDB {
		t.Errorf("%s = %+.3f dB, pinned at %+.3f ± %.2f dB: the own-facade behaviour changed; "+
			"if §1.9 was fixed, flip this test", what, got, measured, pinnedBandDB)
	}
}

func pathWalls(paths []schall03.ReflectionPath) [][]int {
	out := make([][]int, len(paths))
	for i, p := range paths {
		out[i] = p.Walls
	}

	return out
}

// 5 cm in front of a single facade, the offset RLS-19 uses: what a hand-placed
// or SoundPLAN-imported facade point gets under Schall 03.
func TestOwnFacade_Schall03_FiveCentimetres(t *testing.T) {
	t.Parallel()

	track := facadeTrack(t, 100)
	receiver := geo.Point2D{X: 0, Y: ownFacadeY - 0.05}
	walls, footprint := facadeHouse("house", -5, 5)

	free := facadeLevel(t, receiver, track, nil, nil)

	assertPinned(t, "own facade as one wall, over free field",
		facadeLevel(t, receiver, track, []schall03.ReflectingWall{walls[0]}, nil)-free, 2.519)

	// As walls plus footprint, the back wall's image also reaches the
	// receiver through the building, because a path is never shielded by
	// the obstacle it bounced off.
	assertPinned(t, "own facade as walls and footprint, over free field",
		facadeLevel(t, receiver, track, walls, footprint)-free, 3.238)

	assertPinned(t, "footprint alone, over free field",
		facadeLevel(t, receiver, track, nil, footprint)-free, 0)

	paths := schall03.EnumerateReflectionPaths(geo.Point2D{X: 0, Y: 0}, receiver, walls, schall03.MaxReflectionOrder)

	var front, back bool

	for _, p := range paths {
		if p.Order == 1 && p.Walls[0] == 0 {
			front = true
		}

		if p.Order == 1 && p.Walls[0] == 2 {
			back = true
		}
	}

	if !front || !back {
		t.Errorf("want first-order paths off the street facade (wall 0) and the back wall (wall 2), got %v",
			pathWalls(paths))
	}
}

// On the facade, 0 m: the position Nr. 2.2.10 itself prescribes. The footprint
// edge the point stands on is a closed segment, so the direct path "crosses"
// it at its end point and the own facade shields the receiver.
func TestOwnFacade_Schall03_OnTheFacade(t *testing.T) {
	t.Parallel()

	track := facadeTrack(t, 100)
	receiver := geo.Point2D{X: 0, Y: ownFacadeY}
	walls, footprint := facadeHouse("house", -5, 5)

	free := facadeLevel(t, receiver, track, nil, nil)

	assertPinned(t, "footprint alone, receiver on its edge, over free field",
		facadeLevel(t, receiver, track, nil, footprint)-free, -19.076)
	assertPinned(t, "walls and footprint, receiver on the facade, over free field",
		facadeLevel(t, receiver, track, walls, footprint)-free, -3.110)
	assertPinned(t, "own facade as one wall, receiver on it, over free field",
		facadeLevel(t, receiver, track, []schall03.ReflectingWall{walls[0]}, nil)-free, 1.047)
}

// A canyon: the own facade on y = 20, an opposite wall on y = −20, the track
// between, both 20 m high so the 63 Hz Fresnel condition admits every order.
// Nr. 2.2.10 drops every path that touches the own facade; a last-bounce rule
// like RLS-19's would keep two of them.
func TestOwnFacade_Schall03_Canyon(t *testing.T) {
	t.Parallel()

	track := facadeTrack(t, 100)
	receiver := geo.Point2D{X: 0, Y: ownFacadeY - 0.05}
	own := facadeWall(-20, 20, ownFacadeY, 20) // wall 0
	opposite := facadeWall(-100, 100, -20, 20) // wall 1
	walls := []schall03.ReflectingWall{own, opposite}

	paths := schall03.EnumerateReflectionPaths(geo.Point2D{X: 0, Y: 0}, receiver, walls, schall03.MaxReflectionOrder)

	var touches, endsOn int

	for _, p := range paths {
		if slices.Contains(p.Walls, 0) {
			touches++
		}

		if p.Walls[len(p.Walls)-1] == 0 {
			endsOn++
		}
	}

	t.Logf("paths from the source point at x = 0: %v; touching the own facade: %d; ending on it: %d",
		pathWalls(paths), touches, endsOn)

	if len(paths) != 6 || touches != 5 || endsOn != 3 {
		t.Errorf("want 6 paths, 5 touching the own facade, 3 ending on it; got %d, %d, %d",
			len(paths), touches, endsOn)
	}

	assertPinned(t, "own facade in a canyon, over the opposite wall alone",
		facadeLevel(t, receiver, track, walls, nil)-facadeLevel(t, receiver, track, []schall03.ReflectingWall{opposite}, nil),
		2.607)
}

// A coplanar neighbour across a party wall at x = 5.
func TestOwnFacade_Schall03_CoplanarNeighbour(t *testing.T) {
	t.Parallel()

	track := facadeTrack(t, 300)
	receiver := geo.Point2D{X: 4.9, Y: ownFacadeY - 0.05}

	own := facadeWall(-5, 5, ownFacadeY, facadeTopM)
	neighbour := facadeWall(5, 15, ownFacadeY, facadeTopM)

	ownOnly := facadeLevel(t, receiver, track, []schall03.ReflectingWall{own}, nil)
	plane := facadeLevel(t, receiver, track, []schall03.ReflectingWall{own, neighbour}, nil)
	assertPinned(t, "neighbour wall on top of the own facade, 0.10 m from the party wall", plane-ownOnly, 0.030)

	houseWalls, houseFootprint := facadeHouse("house", -5, 5)
	nbWalls, nbFootprint := facadeHouse("neighbour", 5, 15)

	ownBuilding := facadeLevel(t, receiver, track, houseWalls, houseFootprint)
	block := facadeLevel(
		t, receiver, track,
		append(append([]schall03.ReflectingWall{}, houseWalls...), nbWalls...),
		append(append([]schall03.BarrierSegment{}, houseFootprint...), nbFootprint...),
	)
	assertPinned(t, "neighbour building on top of the own building, 0.10 m from the party wall", block-ownBuilding, -0.474)
}
