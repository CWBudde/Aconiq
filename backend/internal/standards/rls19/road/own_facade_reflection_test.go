package road

import (
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// Own-facade reflections at a facade receiver (PLAN.md Priority 18 B1,
// Priority 1 §1.9).
//
// RLS-19 puts the Immissionsort at a building 0.05 m in front of the outer
// facade, at the height of the storey's slab, and does not count a path whose
// last reflection lies on the facade the Immissionsort belongs to (Section 1,
// entry Schallimmission / Immissionsort, Anmerkung). Nothing in this module
// links a receiver to a facade, so the source's image in the receiver's own
// facade is counted like any other reflection.
//
// These tests measure what that adds and PIN it. The bands are NOT a
// conformance tolerance and must never be read as one: they hold today's
// defective state, about ±0.25 dB around the measured value, so it cannot
// change unnoticed. When §1.9 is fixed, each test flips to asserting that the
// own-facade paths are gone.
//
// Measured on 2026-10-22, LrDay, sampleSource traffic:
//
//	single facade, as a Building (what the CLI builds)  +3.293 dB over free field
//	single facade, as a reflector line                   +2.749 dB
//	canyon, own facade removed                           −2.758 dB
//	coplanar neighbour line, 0.10 m from the party wall  +0.147 dB on top of the own facade
//	coplanar neighbour Building                          −0.682 dB (it shields the own image)

const (
	ownFacadeY        = 20.0 // the receiver's own facade lies on y = 20
	facadeOffsetM     = 0.05 // RLS-19: 5 cm in front of the outer facade
	facadeReceiverZ   = 3.0  // ceiling slab of the ground floor at s = 3.0 m
	facadeBuildingTop = 10.0
	pinnedBandDB      = 0.25
)

// facadeBuilding is a 10 m deep, 10 m high block whose street facade lies on
// y = ownFacadeY between x0 and x1.
func facadeBuilding(id string, x0, x1 float64) Building {
	return Building{
		ID: id,
		Footprint: []geo.Point2D{
			{X: x0, Y: ownFacadeY},
			{X: x1, Y: ownFacadeY},
			{X: x1, Y: ownFacadeY + 10},
			{X: x0, Y: ownFacadeY + 10},
		},
		HeightM: facadeBuildingTop,
	}
}

// facadeLine is a single reflecting wall on y = y between x0 and x1, with no
// footprint behind it: a reflector that shields nothing.
func facadeLine(id string, x0, x1, y float64) Reflector {
	return Reflector{
		ID:       id,
		Geometry: []geo.Point2D{{X: x0, Y: y}, {X: x1, Y: y}},
		HeightM:  facadeBuildingTop,
	}
}

type facadeScene struct {
	reflectors []Reflector
	buildings  []Building
	barriers   []Barrier
}

func facadeReceiverLevel(t *testing.T, receiver geo.Point2D, sources []RoadSource, scene facadeScene) float64 {
	t.Helper()

	cfg := DefaultPropagationConfig()
	cfg.ReceiverHeightM = facadeReceiverZ
	cfg.Reflectors = scene.reflectors
	cfg.Buildings = scene.buildings

	levels, err := ComputeReceiverLevels(receiver, sources, scene.barriers, cfg)
	if err != nil {
		t.Fatalf("ComputeReceiverLevels: %v", err)
	}

	return levels.LrDay
}

func assertPinned(t *testing.T, what string, got, measured float64) {
	t.Helper()

	t.Logf("%s: %+.3f dB (pinned at %+.3f)", what, got, measured)

	if got < measured-pinnedBandDB || got > measured+pinnedBandDB {
		t.Errorf("%s = %+.3f dB, pinned at %+.3f ± %.2f dB: the own-facade behaviour changed; "+
			"if §1.9 was fixed, flip this test", what, got, measured, pinnedBandDB)
	}
}

// A single facade, the receiver 5 cm in front of it, a road 20 m away.
func TestOwnFacade_RLS19_SingleFacade(t *testing.T) {
	t.Parallel()

	receiver := geo.Point2D{X: 0, Y: ownFacadeY - facadeOffsetM}
	sources := []RoadSource{sampleSource()}
	house := facadeBuilding("house", -5, 5)
	front := facadeLine("front", -5, 5, ownFacadeY)

	free := facadeReceiverLevel(t, receiver, sources, facadeScene{})

	// The facade alone: its image is almost as strong as the direct sound,
	// 0.5 dB down and 0.1 m further away, so the level nearly doubles.
	line := facadeReceiverLevel(t, receiver, sources, facadeScene{reflectors: []Reflector{front}})
	assertPinned(t, "own facade as a reflector line, over free field", line-free, 2.749)

	// The facade as a Building, which is what the CLI builds from a model.
	// A path never shields itself on its own reflector, and that exclusion
	// is per building (conformance declaration, Nr. 3.6), so the back wall's
	// image reaches the receiver through the building and adds the rest.
	building := facadeReceiverLevel(t, receiver, sources, facadeScene{buildings: []Building{house}})
	assertPinned(t, "own facade as a Building, over free field", building-free, 3.293)

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, pointSourceHeightM,
		receiver, facadeReceiverZ,
		[]Reflector{house.asReflector()},
	)

	var ownImage, backWallImage bool

	for _, p := range paths {
		switch {
		case p.bounces == 1 && p.imagePoint.Y == 2*ownFacadeY:
			ownImage = true
		case p.bounces == 1 && p.imagePoint.Y == 2*(ownFacadeY+10):
			backWallImage = true
		}
	}

	if !ownImage {
		t.Errorf("no first-order path off the own facade among %d: the pinned defect is gone", len(paths))
	}

	if !backWallImage {
		t.Errorf("no first-order path off the back wall among %d: the per-building shielding exclusion changed", len(paths))
	}

	// The same facade modelled as a separate reflector in front of its own
	// building as a barrier: the building now shields the image, because the
	// two carry different IDs. The two models of one facade disagree by the
	// whole effect, so a fix must not rely on either.
	shielded := facadeReceiverLevel(t, receiver, sources, facadeScene{
		reflectors: []Reflector{front},
		barriers:   []Barrier{house.asBarrier()},
	})
	assertPinned(t, "reflector line in front of its own building as a barrier, over free field", shielded-free, 0.011)
}

// A street canyon: the own facade on y = 20, an opposite facade on y = −20,
// the road between. RLS-19 drops only paths whose LAST bounce is on the own
// facade; own → opposite stays.
func TestOwnFacade_RLS19_Canyon(t *testing.T) {
	t.Parallel()

	receiver := geo.Point2D{X: 0, Y: ownFacadeY - facadeOffsetM}
	sources := []RoadSource{sampleSource()}
	own := facadeLine("own", -5, 5, ownFacadeY)      // class 0
	opposite := facadeLine("opposite", -50, 50, -20) // class 1

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, pointSourceHeightM,
		receiver, facadeReceiverZ,
		[]Reflector{own, opposite},
	)

	var lastOnOwn, ownThenOpposite int

	for _, p := range paths {
		last := p.reflectorClasses[p.bounces-1]

		switch {
		case last == 0:
			lastOnOwn++
		case p.bounces == 2 && p.reflectorClasses[0] == 0:
			ownThenOpposite++
		}
	}

	t.Logf("paths from the substitute source at x = 0: %d; last bounce on own facade: %d; own then opposite: %d",
		len(paths), lastOnOwn, ownThenOpposite)

	if lastOnOwn != 2 || ownThenOpposite != 1 {
		t.Errorf("want 2 paths ending on the own facade (order 1, opposite → own) and 1 own → opposite, "+
			"got %d and %d of %d", lastOnOwn, ownThenOpposite, len(paths))
	}

	withOwn := facadeReceiverLevel(t, receiver, sources, facadeScene{reflectors: []Reflector{own, opposite}})
	oppositeOnly := facadeReceiverLevel(t, receiver, sources, facadeScene{reflectors: []Reflector{opposite}})

	// The upper bound of what the RLS-19 exclusion removes here: it keeps the
	// own → opposite path that removing the facade also takes away.
	assertPinned(t, "own facade in a canyon, over the opposite facade alone", withOwn-oppositeOnly, 2.758)
}

// A coplanar neighbour across a party wall at x = 5: in a block perimeter
// "the facade" is a plane, not one footprint edge.
func TestOwnFacade_RLS19_CoplanarNeighbour(t *testing.T) {
	t.Parallel()

	long := sampleSource()
	long.Centerline = []geo.Point2D{{X: -300, Y: 0}, {X: 300, Y: 0}}
	sources := []RoadSource{long}

	own := facadeLine("own", -5, 5, ownFacadeY)
	neighbour := facadeLine("neighbour", 5, 15, ownFacadeY)

	// 0.10 m from the party wall, the specular points of road segments more
	// than about 40 m away along the road fall on the neighbour's facade.
	nearWall := geo.Point2D{X: 4.9, Y: ownFacadeY - facadeOffsetM}
	ownOnly := facadeReceiverLevel(t, nearWall, sources, facadeScene{reflectors: []Reflector{own}})
	plane := facadeReceiverLevel(t, nearWall, sources, facadeScene{reflectors: []Reflector{own, neighbour}})
	assertPinned(t, "neighbour facade line on top of the own facade, 0.10 m from the party wall", plane-ownOnly, 0.147)

	// 1 m from the party wall no specular point reaches the neighbour.
	oneMetre := geo.Point2D{X: 4.0, Y: ownFacadeY - facadeOffsetM}
	ownOnly1 := facadeReceiverLevel(t, oneMetre, sources, facadeScene{reflectors: []Reflector{own}})
	plane1 := facadeReceiverLevel(t, oneMetre, sources, facadeScene{reflectors: []Reflector{own, neighbour}})
	assertPinned(t, "neighbour facade line on top of the own facade, 1 m from the party wall", plane1-ownOnly1, 0)

	// As Buildings, the neighbour's footprint lies across the own image's
	// unfolded leg behind the facade plane and shields part of it, so adding
	// the neighbour LOWERS the level. Shielding is tested along the whole
	// image → receiver segment, including the part that is only a mirror.
	ownBuilding := facadeReceiverLevel(t, nearWall, sources, facadeScene{
		buildings: []Building{facadeBuilding("house", -5, 5)},
	})
	block := facadeReceiverLevel(t, nearWall, sources, facadeScene{
		buildings: []Building{facadeBuilding("house", -5, 5), facadeBuilding("neighbour", 5, 15)},
	})
	assertPinned(t, "neighbour Building on top of the own Building, 0.10 m from the party wall", block-ownBuilding, -0.682)
}
