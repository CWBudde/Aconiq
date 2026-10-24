package facadeset

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// A window drawn in the elevation view is the feature the same window from a
// window list becomes: the same key, frame, review fields and centre.
func TestPlaceWindowAsAWindowListPlacesIt(t *testing.T) {
	t.Parallel()

	model := windowModel(t)

	got, err := PlaceWindow(model, WindowPlacement{
		ID: "drawn", BuildingRef: "house",
		Frame:     geo.FacadeWindow{UM: 2, SillZM: 3.9, WidthM: 1.2, HeightM: 1.5},
		Sensitive: true, ReviewStatus: geo.ReviewStatusConfirmed,
	})
	if err != nil {
		t.Fatalf("PlaceWindow: %v", err)
	}

	listed := windowModel(t, windowSpec("drawn", 2, true, geo.ReviewStatusConfirmed)).Features[1]
	want := modelgeojson.Model{Features: []modelgeojson.Feature{listed}}.ToFeatureCollection().Features[0]

	if got.ID != "drawn" || got.Geometry.Type != "Point" {
		t.Errorf("feature = %+v, want the Point feature drawn", got)
	}

	for _, key := range []string{"kind", "facade", "window", "sensitive", "review_status"} {
		if !equalJSON(t, got.Properties[key], want.Properties[key]) {
			t.Errorf("%s = %#v, want %#v", key, got.Properties[key], want.Properties[key])
		}
	}

	if !equalJSON(t, got.Geometry.Coordinates, want.Geometry.Coordinates) {
		t.Errorf("centre = %v, want %v", got.Geometry.Coordinates, want.Geometry.Coordinates)
	}

	// Proposed when the view sends no review status, as a list's window is.
	proposed, err := PlaceWindow(model, WindowPlacement{
		ID: "new", BuildingRef: "house", Frame: geo.FacadeWindow{UM: 1, SillZM: 1, WidthM: 1, HeightM: 1},
	})
	if err != nil || proposed.Properties["review_status"] != geo.ReviewStatusProposed || proposed.Properties["sensitive"] != false {
		t.Errorf("new window = %+v, %v, want proposed and not sensitive", proposed, err)
	}
}

func TestPlaceWindowRefuses(t *testing.T) {
	t.Parallel()

	model := windowModel(t)
	frame := geo.FacadeWindow{UM: 2, SillZM: 1, WidthM: 1, HeightM: 1}

	cases := map[string]struct {
		placement WindowPlacement
		want      string
	}{
		"past the corner": {WindowPlacement{ID: "w", BuildingRef: "house", Frame: geo.FacadeWindow{UM: 9.5, SillZM: 1, WidthM: 1, HeightM: 1}}, "runs past the end"},
		"no such edge":    {WindowPlacement{ID: "w", BuildingRef: "house", Edge: 9, Frame: frame}, "has no part 0, ring 0, edge 9"},
		"no building":     {WindowPlacement{ID: "w", BuildingRef: "shed", Frame: frame}, "not in the model"},
		"empty frame":     {WindowPlacement{ID: "w", BuildingRef: "house"}, "width_m"},
		"slash in id":     {WindowPlacement{ID: "a/b", BuildingRef: "house", Frame: frame}, "must not contain '/'"},
	}

	for name, tc := range cases {
		_, err := PlaceWindow(model, tc.placement)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
}

func equalJSON(t *testing.T, a, b any) bool {
	t.Helper()

	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)

	if errA != nil || errB != nil {
		t.Fatalf("marshal: %v, %v", errA, errB)
	}

	return string(ja) == string(jb)
}
