package modelgeojson

import (
	"maps"
	"reflect"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// keepStoredTolerance is the tolerance the model save passes: 1 µm, far above
// the nanometres a CRS round trip moves a vertex and far below anything a
// person moves on a map.
const keepStoredTolerance = 1e-6

// The point of KeepStoredCoordinates: a model that went through EPSG:4326 and
// back keeps every facade key, at every position probed, where the round trip
// alone renumbers some of them.
func TestKeepStoredCoordinatesKeepsFacadeKeysThroughARoundTrip(t *testing.T) {
	t.Parallel()

	renumbered := 0

	for _, position := range renumberingPositions() {
		placed, roundTripped := roundTrippedFacadeModel(t, position[0], position[1])
		if len(movedReceiverIDs(Validate(roundTripped))) > 0 {
			renumbered++
		}

		kept := KeepStoredCoordinates(roundTripped, placed, keepStoredTolerance)
		assertCleanReport(t, Validate(kept))

		if !reflect.DeepEqual(kept.Features, placed.Features) {
			t.Fatalf("at %v: an unchanged model must come back with the stored coordinates, bit for bit", position)
		}
	}

	if renumbered == 0 {
		t.Fatal("no probed position renumbered, so this test no longer exercises a renumbering round trip; move the positions")
	}
}

func TestKeepStoredCoordinatesTakesWhatChanged(t *testing.T) {
	t.Parallel()

	stored, receiver := oneFacadeReceiver(t)
	nudged := func(coordinates any, dx float64) any {
		pair, _ := coordinates.([]any)

		x, _ := pair[0].(float64)

		return []any{x + dx, pair[1]}
	}

	cases := []struct {
		name string
		edit func(f Feature) Feature
		// keepsGeometry and keepsFacade say what the result takes from the
		// stored feature; everything else is the incoming one's.
		keepsGeometry, keepsFacade bool
	}{
		{
			name:          "a receiver moved 1 m keeps its facade endpoints, which describe the building",
			edit:          func(f Feature) Feature { f.Coordinates = nudged(f.Coordinates, 1); return f },
			keepsGeometry: false, keepsFacade: true,
		},
		{
			name:          "a nanometre is a round trip, not an edit",
			edit:          func(f Feature) Feature { f.Coordinates = nudged(f.Coordinates, 1e-9); return f },
			keepsGeometry: true, keepsFacade: true,
		},
		{
			name: "a property change lands, and the coordinates are still kept",
			edit: func(f Feature) Feature {
				f.Properties = maps.Clone(f.Properties)
				f.Properties[PropertyReviewStatus] = geo.ReviewStatusRejected

				return f
			},
			keepsGeometry: true, keepsFacade: true,
		},
		{
			name: "a facade re-keyed to another edge takes its own endpoints",
			edit: func(f Feature) Feature {
				facade := maps.Clone(f.Properties[PropertyFacade].(map[string]any))
				facade["edge"] = facade["edge"].(float64) + 1
				f.Properties = maps.Clone(f.Properties)
				f.Properties[PropertyFacade] = facade

				return f
			},
			keepsGeometry: true, keepsFacade: false,
		},
		{
			name: "a different geometry shape is a new geometry",
			edit: func(f Feature) Feature {
				pair, _ := f.Coordinates.([]any)
				f.Coordinates = []any{pair[0], pair[1], 3.0}

				return f
			},
			keepsGeometry: false, keepsFacade: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			incoming := tc.edit(receiver)
			model := KeepStoredCoordinates(facadeModel(stored.Features[0], incoming), stored, keepStoredTolerance)
			got := model.Features[1]

			wantGeometry := incoming.Coordinates
			if tc.keepsGeometry {
				wantGeometry = receiver.Coordinates
			}

			if !reflect.DeepEqual(got.Coordinates, wantGeometry) {
				t.Errorf("coordinates = %#v, want %#v", got.Coordinates, wantGeometry)
			}

			wantFacade := incoming.Properties[PropertyFacade]
			if tc.keepsFacade {
				wantFacade = receiver.Properties[PropertyFacade]
			}

			if !reflect.DeepEqual(got.Properties[PropertyFacade], wantFacade) {
				t.Errorf("facade = %#v, want %#v", got.Properties[PropertyFacade], wantFacade)
			}

			if got.Properties[PropertyReviewStatus] != incoming.Properties[PropertyReviewStatus] {
				t.Errorf("review_status = %v, want the incoming %v", got.Properties[PropertyReviewStatus], incoming.Properties[PropertyReviewStatus])
			}
		})
	}
}

func TestKeepStoredCoordinatesLeavesUnmatchedFeaturesAlone(t *testing.T) {
	t.Parallel()

	stored, receiver := oneFacadeReceiver(t)

	renamed := receiver
	renamed.ID = "new-receiver"
	renamed.Coordinates = []any{1e-9, 0.0}

	building := stored.Features[0]
	ring := building.Coordinates.([]any)[0].([]any)
	dropped := building
	dropped.Coordinates = []any{append(append([]any{}, ring[:3]...), ring[0])}

	incoming := facadeModel(dropped, renamed)
	kept := KeepStoredCoordinates(incoming, stored, keepStoredTolerance)

	if !reflect.DeepEqual(kept.Features, incoming.Features) {
		t.Fatalf("a feature with no stored twin, or a ring with another vertex count, must pass through: got %#v", kept.Features)
	}
}

func TestKeepStoredCoordinatesNeitherInputIsWrittenThrough(t *testing.T) {
	t.Parallel()

	stored, receiver := oneFacadeReceiver(t)
	incoming := facadeModel(stored.Features[0], receiver)
	incoming.Features[1].Coordinates = []any{receiver.Coordinates.([]any)[0].(float64) + 1e-9, receiver.Coordinates.([]any)[1]}
	before := incoming.Features[1].Coordinates

	_ = KeepStoredCoordinates(incoming, stored, keepStoredTolerance)

	if !reflect.DeepEqual(incoming.Features[1].Coordinates, before) {
		t.Fatal("KeepStoredCoordinates wrote through the incoming model")
	}
}

// A geographic project measures in degrees, where a tolerance in metres means
// nothing: the save keeps what arrived.
func TestKeepStoredCoordinatesDoesNothingOnAGeographicProject(t *testing.T) {
	t.Parallel()

	stored := Model{ProjectCRS: "EPSG:4326", Features: []Feature{{
		ID: "r1", Kind: FeatureKindReceiver, GeometryType: GeometryTypePoint, Coordinates: []any{9.0, 51.0},
	}}}
	incoming := Model{ProjectCRS: "EPSG:4326", Features: []Feature{{
		ID: "r1", Kind: FeatureKindReceiver, GeometryType: GeometryTypePoint, Coordinates: []any{9.0 + 1e-9, 51.0},
	}}}

	if kept := KeepStoredCoordinates(incoming, stored, keepStoredTolerance); !reflect.DeepEqual(kept.Features, incoming.Features) {
		t.Fatalf("got %#v, want the incoming features", kept.Features)
	}
}
