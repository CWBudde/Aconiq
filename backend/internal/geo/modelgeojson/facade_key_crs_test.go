package modelgeojson

import (
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// roundTrippedFacadeModel places facade receivers on an axis-aligned
// rectangle at (x0, y0) in EPSG:25832, whose two western vertices tie on x,
// and returns that model together with the same model after an
// EPSG:25832 → EPSG:4326 → EPSG:25832 round trip, as a save through a
// geographic CRS leaves it. Synthetic coordinates; no real address.
func roundTrippedFacadeModel(t *testing.T, x0, y0 float64) (placed, roundTripped Model) {
	t.Helper()

	footprint := [][][]geo.Point2D{{facadeRect(x0, y0, x0+20, y0+12)}}
	receivers := facadeReceiverFeatures(t, facadeTestBuilding(footprint), false)
	placed = Model{ProjectCRS: "EPSG:25832", Features: append([]Feature{buildingFeature("b1", footprint)}, receivers...)}

	geographic, err := Reproject(placed, "EPSG:4326")
	if err != nil {
		t.Fatalf("Reproject into EPSG:4326: %v", err)
	}

	roundTripped, err = Reproject(geographic, "EPSG:25832")
	if err != nil {
		t.Fatalf("Reproject back into EPSG:25832: %v", err)
	}

	return placed, roundTripped
}

// renumberingPositions are offsets of roundTrippedFacadeModel's rectangle
// from (547000, 5795000), a fixed grid rather than one chosen position:
// whether a round trip breaks a tie, and which way, is a matter of the last
// bit of a projection, which need not agree between architectures.
func renumberingPositions() [][2]float64 {
	positions := make([][2]float64, 0, 64)

	for i := range 8 {
		for j := range 8 {
			positions = append(positions, [2]float64{547000 + float64(i)*37.13, 5795000 + float64(j)*41.7})
		}
	}

	return positions
}

// A facade key's part, ring and edge are canonical in the compute CRS, and
// canonicalisation compares exactly, so two vertices tied on x there are
// ordered by y. A CRS round trip moves a coordinate by about 1e-9 m, which is
// enough to break such a tie the other way: the canonical ring then starts at
// the other vertex and every edge index of the building shifts by one.
//
// That is never silent. The stored from / to no longer match the edge their
// index names, so validation reports every receiver of the building as moved,
// both on the round-tripped model and on the model held in EPSG:4326, and a
// normative run refuses them until they are re-attached.
func TestFacadeKeyIndicesRenumberWhenACRSRoundTripBreaksATie(t *testing.T) {
	t.Parallel()

	renumbered := 0

	for _, position := range renumberingPositions() {
		placed, roundTripped := roundTrippedFacadeModel(t, position[0], position[1])
		assertCleanReport(t, Validate(placed))

		report := Validate(roundTripped)
		if report.ErrorCount() != 0 || len(issuesWithCode(report.Warnings, CodeReceiverFacadeUnresolved)) != 0 {
			t.Fatalf("at %v: errors %#v, warnings %#v; a round trip must not make a key unreadable or unresolved",
				position, report.Errors, report.Warnings)
		}

		moved := movedReceiverIDs(report)
		if len(moved) == 0 {
			assertCleanReport(t, report)

			continue
		}

		renumbered++

		if len(moved) != len(placed.Features)-1 {
			t.Fatalf("at %v: %d of %d receivers moved; a renumbered ring moves every edge of the building",
				position, len(moved), len(placed.Features)-1)
		}

		geographic, err := Reproject(placed, "EPSG:4326")
		if err != nil {
			t.Fatalf("Reproject into EPSG:4326: %v", err)
		}

		if got := movedReceiverIDs(Validate(geographic)); !equalStrings(got, moved) {
			t.Fatalf("at %v: moved %v when held in EPSG:4326, %v after the round trip; both measure in EPSG:25832",
				position, got, moved)
		}
	}

	if renumbered == 0 {
		t.Fatal("no position renumbered its edges; if a round trip can no longer break a tie, " +
			"facade-receivers.md (The facade key) says otherwise and must change with it")
	}
}
