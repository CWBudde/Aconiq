package httpv1

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// elevationHouse is a synthetic 10 m × 8 m house of three storeys in
// EPSG:25832.
const elevationHouse = `{"type":"FeatureCollection","features":[
{"type":"Feature","id":"house","properties":{"kind":"building","height_m":9,"building:levels":"3","height_source":"measured"},
 "geometry":{"type":"Polygon","coordinates":[[[547000,5795000],[547010,5795000],[547010,5795008],[547000,5795008],[547000,5795000]]]}}]}`

func postFacadeElevation(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := newAPIRequest(http.MethodPost, "/api/v1/facades/elevation", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	NewHandler(mustStore(t, "Facade Elevation"), nil).ServeHTTP(rec, req)

	return rec
}

func TestFacadeElevationEndpoint(t *testing.T) {
	t.Parallel()

	rec := postFacadeElevation(t, `{"crs":"EPSG:25832","building":"house","model":`+elevationHouse+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp facadeElevationResponse

	decodeResponse(t, rec.Body.Bytes(), &resp)

	if resp.BuildingRef != "house" || len(resp.Facades) != 4 || len(resp.SlabHeightsM) != 3 || resp.SlabHeightSource != "measured" {
		t.Fatalf("elevation = %+v, want the house's four facades and three storeys", resp)
	}

	if f := resp.Facades[0]; f.From != [2]float64{547000, 5795000} || math.Abs(f.LengthM-10) > 1e-9 || f.Windows == nil {
		t.Errorf("facade 0 = %+v, want the 10 m south edge with an empty window list", f)
	}

	if resp.Detached == nil {
		t.Error("detached is null, want an empty list")
	}
}

func TestFacadeElevationEndpointRefuses(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body string
		want string
	}{
		"no crs":        {`{"building":"house","model":` + elevationHouse + `}`, "crs, model and building are required"},
		"no building":   {`{"crs":"EPSG:25832","model":` + elevationHouse + `}`, "crs, model and building are required"},
		"no model":      {`{"crs":"EPSG:25832","building":"house"}`, "crs, model and building are required"},
		"bad model":     {`{"crs":"EPSG:25832","building":"house","model":{"type":"Nope"}}`, "not a valid GeoJSON"},
		"unknown":       {`{"crs":"EPSG:25832","building":"shed","model":` + elevationHouse + `}`, `shed\" is not in the model`},
		"not json body": {`{`, "valid JSON"},
	}

	for name, tc := range cases {
		rec := postFacadeElevation(t, tc.body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%s: %d %s, want 400 mentioning %q", name, rec.Code, rec.Body.String(), tc.want)
		}
	}
}

// The API's DTOs spell an elevation exactly as facadeset — and so the
// WebAssembly kernel — spells it, windows and findings included.
func TestFacadeElevationDTOsMatchTheFacadesetContract(t *testing.T) {
	t.Parallel()

	finding := modelgeojson.ValidationIssue{Level: "warning", Code: modelgeojson.CodeWindowAboveBuilding, FeatureID: "w1", Message: "m"}
	window := facadeset.ElevationWindow{
		ID: "w1", Frame: geo.FacadeWindow{UM: 1, SillZM: 2, WidthM: 3, HeightM: 4},
		Sensitive: true, ReviewStatus: "confirmed", Findings: []modelgeojson.ValidationIssue{finding},
	}
	elevation := facadeset.BuildingElevation{
		BuildingRef: "b", HeightM: 9, Levels: 3, SlabHeightsM: []float64{3, 6, 9}, SlabHeightSource: "measured",
		Facades: []facadeset.FacadeElevation{{
			Part: 0, Ring: 1, Edge: 2, From: [2]float64{1, 2}, To: [2]float64{3, 4}, LengthM: 2.8, FacingDeg: 135, Courtyard: true,
			Windows: []facadeset.ElevationWindow{window},
		}},
		Detached: []facadeset.ElevationWindow{window},
	}

	want, err := json.Marshal(elevation)
	if err != nil {
		t.Fatalf("marshal facadeset elevation: %v", err)
	}

	got, err := json.Marshal(facadeElevationResponseOf(elevation))
	if err != nil {
		t.Fatalf("marshal API elevation: %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("shapes differ:\n api:       %s\n facadeset: %s", got, want)
	}
}

func TestOpenAPIDocumentsFacadeElevation(t *testing.T) {
	t.Parallel()

	spec := BuildOpenAPISpec("")

	paths, _ := spec["paths"].(map[string]any)

	item, ok := paths["/api/v1/facades/elevation"].(map[string]any)
	if !ok {
		t.Fatal("expected /api/v1/facades/elevation path item")
	}

	assertOperationResponses(t, item, "post", []string{"200", "400", "405", "413", "415"})

	components, _ := spec["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)

	for _, name := range []string{"FacadeElevationRequest", "FacadeElevationResponse", "FacadeElevation", "ElevationWindow"} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("expected schema %s", name)
		}
	}
}
