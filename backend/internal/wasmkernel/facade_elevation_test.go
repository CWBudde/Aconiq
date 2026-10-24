package wasmkernel_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/facadeset"
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
