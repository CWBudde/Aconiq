package wasmkernel_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/wasmkernel"
)

func elevationRequest(t *testing.T, crs, building string) []byte {
	t.Helper()

	in, err := json.Marshal(wasmkernel.FacadeElevationRequest{CRS: crs, Model: json.RawMessage(facadeTestModel), Building: building})
	if err != nil {
		t.Fatal(err)
	}

	return in
}

// The kernel adds a JSON boundary and nothing else: it answers what
// facadeset.Elevation answers for the same model.
func TestFacadeElevationAnswersWhatFacadesetAnswers(t *testing.T) {
	t.Parallel()

	out, err := wasmkernel.FacadeElevation(elevationRequest(t, "EPSG:25832", "a"))
	if err != nil {
		t.Fatalf("facadeElevation: %v", err)
	}

	var got facadeset.BuildingElevation

	err = json.Unmarshal(out, &got)
	if err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	model, err := modelgeojson.Normalize([]byte(facadeTestModel), "EPSG:25832", "test")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	want, err := facadeset.Elevation(model, "a")
	if err != nil {
		t.Fatalf("Elevation: %v", err)
	}

	if !reflect.DeepEqual(got, want) || len(got.Facades) != 4 {
		t.Errorf("kernel answered %+v, facadeset %+v", got, want)
	}

	// b stands against all of a's east wall, edge 1.
	if walls := got.Facades[1].PartyWalls; !reflect.DeepEqual(walls, []geo.FacadeSpan{{FromUM: 0, ToUM: 8}}) {
		t.Errorf("east wall party walls = %v, want all 8 m of it", walls)
	}
}

func TestFacadeElevationRefuses(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		input []byte
		want  string
	}{
		"no crs":      {elevationRequest(t, "", "a"), "crs is required"},
		"bad json":    {[]byte("{"), "invalid input JSON"},
		"no such one": {elevationRequest(t, "EPSG:25832", "z"), `facade elevation: building "z" is not in the model`},
		"bad model":   {[]byte(`{"crs":"EPSG:25832","model":{"type":"Nope"},"building":"a"}`), "model:"},
	}

	for name, tc := range cases {
		_, err := wasmkernel.FacadeElevation(tc.input)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
}

// The kernel places what facadeset places, and refuses in its words.
func TestPlaceWindowAnswersWhatFacadesetAnswers(t *testing.T) {
	t.Parallel()

	placement := facadeset.WindowPlacement{
		ID: "drawn", BuildingRef: "a",
		Frame: geo.FacadeWindow{UM: 2, SillZM: 1, WidthM: 1.2, HeightM: 1.5},
	}

	in, err := json.Marshal(wasmkernel.PlaceWindowRequest{CRS: "EPSG:25832", Model: json.RawMessage(facadeTestModel), Window: placement})
	if err != nil {
		t.Fatal(err)
	}

	out, err := wasmkernel.PlaceWindow(in)
	if err != nil {
		t.Fatalf("placeWindow: %v", err)
	}

	model, err := modelgeojson.Normalize([]byte(facadeTestModel), "EPSG:25832", "test")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	feature, err := facadeset.PlaceWindow(model, placement)
	if err != nil {
		t.Fatalf("PlaceWindow: %v", err)
	}

	want, err := json.Marshal(wasmkernel.PlaceWindowResponse{Feature: feature})
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != string(want) {
		t.Errorf("kernel answered %s, facadeset %s", out, want)
	}

	placement.Frame.UM = 9.5

	in, err = json.Marshal(wasmkernel.PlaceWindowRequest{CRS: "EPSG:25832", Model: json.RawMessage(facadeTestModel), Window: placement})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := wasmkernel.PlaceWindow(in); err == nil || !strings.Contains(err.Error(), "place window:") || !strings.Contains(err.Error(), "runs past the end") {
		t.Errorf("past the corner: err = %v", err)
	}
}

// The kernel solves what facadeset solves, and refuses in its words.
func TestRectifyFacadeAnswersWhatFacadesetAnswers(t *testing.T) {
	t.Parallel()

	transform := facadeset.PhotoTransform{
		ImagePoints:  [][]float64{{100, 900}, {865.4, 846.2}, {885.1, 114.9}, {123.5, 139.4}},
		FacadePoints: [][]float64{{0, 0}, {10, 0}, {10, 8}, {0, 8}},
	}

	in, err := json.Marshal(transform)
	if err != nil {
		t.Fatal(err)
	}

	out, err := wasmkernel.RectifyFacade(in)
	if err != nil {
		t.Fatalf("rectifyFacade: %v", err)
	}

	var got facadeset.Rectification

	err = json.Unmarshal(out, &got)
	if err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	want, err := facadeset.Rectify(transform)
	if err != nil || got != want {
		t.Errorf("kernel answered %+v, facadeset %+v (%v)", got, want, err)
	}

	mirrored := `{"image_points":[[1900,900],[1134.6,846.2],[1114.9,114.9],[1876.5,139.4]],` +
		`"facade_points":[[0,0],[10,0],[10,8],[0,8]]}`
	if _, err := wasmkernel.RectifyFacade([]byte(mirrored)); err == nil ||
		!strings.Contains(err.Error(), "rectify facade: the photo cannot be rectified:") ||
		!strings.Contains(err.Error(), "mirrored") {
		t.Errorf("mirrored photo: err = %v", err)
	}

	if _, err := wasmkernel.RectifyFacade([]byte("{")); err == nil || !strings.Contains(err.Error(), "invalid input JSON") {
		t.Errorf("bad json: err = %v", err)
	}
}
