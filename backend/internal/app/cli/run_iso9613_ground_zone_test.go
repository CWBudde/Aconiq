package cli

import (
	"math"
	"testing"
)

// ISO 9613-2's three-region ground model (Abschnitt 7.3.1, Tabelle 3) was
// implemented in full and then fed the one global `ground_factor` three times,
// because nothing in the model could say where the ground changes. A
// `GroundZone` type sat in the module with no caller at all.
//
// This is the end-to-end half of closing that. Nothing here constructs a
// PropagationConfig: the zone reaches the calculation only if the feature kind,
// the extractor and the per-path region resolution all work.
func TestISO9613RunResolvesGFromGroundZones(t *testing.T) {
	t.Parallel()

	// A function rather than a slice: the third run needs the same grid with
	// one parameter added, and appending to a shared slice is how two runs end
	// up sharing a backing array.
	params := func(extra ...string) []string {
		return append([]string{
			"--param", "grid_resolution_m=20",
			"--param", "grid_padding_m=40",
		}, extra...)
	}

	// The default ground_factor is 0.5, so a porous zone over the whole grid
	// has to move every receiver.
	global := iso9613ReceiverLevels(t, registryRunFixtures["iso9613"], params())
	zoned := iso9613ReceiverLevels(t, []string{"phase19", "iso9613_ground_zone_model.geojson"}, params())

	if len(global) == 0 {
		t.Fatal("the zone-free run produced no receivers")
	}

	if len(global) != len(zoned) {
		t.Fatalf("the two runs produced %d and %d receivers; a ground zone must not change the grid", len(global), len(zoned))
	}

	moved := 0

	for id, globalLevel := range global {
		zonedLevel, ok := zoned[id]
		if !ok {
			t.Fatalf("receiver %q is missing from the zoned run", id)
		}

		if math.Abs(zonedLevel-globalLevel) > 1e-9 {
			moved++
		}
	}

	if moved != len(global) {
		t.Fatalf("a porous zone over the whole grid moved %d of %d receivers; G is still read off one global number", moved, len(global))
	}

	// The zone covers every path end to end, so it has to give exactly what
	// declaring the same factor globally gives. This is what distinguishes
	// "the zone was read" from "something else perturbed the run".
	asGlobalFactor := iso9613ReceiverLevels(t, registryRunFixtures["iso9613"], params("--param", "ground_factor=1"))

	for id, want := range asGlobalFactor {
		got, ok := zoned[id]
		if !ok {
			t.Fatalf("receiver %q is missing from the zoned run", id)
		}

		if math.Abs(got-want) > 1e-9 {
			t.Fatalf("receiver %q: a zone of G=1 over everything gave %.9f dB, a global G=1 gave %.9f dB", id, got, want)
		}
	}
}
