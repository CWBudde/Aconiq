package httpv1

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
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
		Sensitive: true, ReviewStatus: "confirmed", OnPartyWall: true, Findings: []modelgeojson.ValidationIssue{finding},
	}
	elevation := facadeset.BuildingElevation{
		BuildingRef: "b", HeightM: 9, Levels: 3, SlabHeightsM: []float64{3, 6, 9}, SlabHeightSource: "measured",
		Facades: []facadeset.FacadeElevation{{
			Part: 0, Ring: 1, Edge: 2, From: [2]float64{1, 2}, To: [2]float64{3, 4}, LengthM: 2.8, FacingDeg: 135, Courtyard: true,
			PartyWalls: []geo.FacadeSpan{{FromUM: 0.5, ToUM: 2}},
			Windows:    []facadeset.ElevationWindow{window},
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

	window, ok := paths["/api/v1/facades/window"].(map[string]any)
	if !ok {
		t.Fatal("expected /api/v1/facades/window path item")
	}

	assertOperationResponses(t, window, "post", []string{"200", "400", "405", "413", "415"})

	rectify, ok := paths["/api/v1/facades/rectify"].(map[string]any)
	if !ok {
		t.Fatal("expected /api/v1/facades/rectify path item")
	}

	assertOperationResponses(t, rectify, "post", []string{"200", "400", "405", "413", "415"})

	components, _ := spec["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)

	for _, name := range []string{
		"FacadeElevationRequest", "FacadeElevationResponse", "FacadeElevation", "ElevationWindow",
		"FacadeWindowFrame", "PlaceWindowRequest", "PlaceWindowResponse", "PhotoTransform", "Rectification",
	} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("expected schema %s", name)
		}
	}
}

func postFacadeWindow(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := newAPIRequest(http.MethodPost, "/api/v1/facades/window", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	NewHandler(mustStore(t, "Facade Window"), nil).ServeHTTP(rec, req)

	return rec
}

func TestFacadeWindowEndpoint(t *testing.T) {
	t.Parallel()

	rec := postFacadeWindow(t, `{"crs":"EPSG:25832","model":`+elevationHouse+`,
		"window":{"id":"w1","building_ref":"house","part":0,"ring":0,"edge":0,
		"window":{"u_m":2,"sill_z_m":1,"width_m":1.2,"height_m":1.5},"sensitive":false}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp placeWindowResponse

	decodeResponse(t, rec.Body.Bytes(), &resp)

	f := resp.Feature
	if f.ID != "w1" || f.Properties["kind"] != "window" || f.Properties["review_status"] != "proposed" || f.Geometry.Type != "Point" {
		t.Errorf("feature = %+v, want a proposed window feature", f)
	}

	if coords, _ := f.Geometry.Coordinates.([]any); len(coords) != 2 || coords[0] != 547002.6 || coords[1] != 5795000.0 {
		t.Errorf("centre = %v, want (547002.6, 5795000)", f.Geometry.Coordinates)
	}

	rec = postFacadeWindow(t, `{"crs":"EPSG:25832","model":`+elevationHouse+`,
		"window":{"id":"w1","building_ref":"house","part":0,"ring":0,"edge":0,
		"window":{"u_m":9.5,"sill_z_m":1,"width_m":1.2,"height_m":1.5},"sensitive":false}}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "runs past the end") {
		t.Errorf("past the corner: %d %s, want 400", rec.Code, rec.Body.String())
	}

	rec = postFacadeWindow(t, `{"model":`+elevationHouse+`}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "crs and model are required") {
		t.Errorf("no crs: %d %s, want 400", rec.Code, rec.Body.String())
	}
}

// The API spells a placement and its answer as facadeset and the kernel do.
func TestPlaceWindowDTOsMatchTheFacadesetContract(t *testing.T) {
	t.Parallel()

	placement := facadeset.WindowPlacement{
		ID: "w", BuildingRef: "b", Part: 1, Ring: 2, Edge: 3,
		Frame:     geo.FacadeWindow{UM: 1, SillZM: 2, WidthM: 3, HeightM: 4},
		Sensitive: true, ReviewStatus: "confirmed", Evidence: []any{map[string]any{"kind": "plan"}},
	}

	raw, err := json.Marshal(placement)
	if err != nil {
		t.Fatal(err)
	}

	var input windowPlacementInput

	err = json.Unmarshal(raw, &input)
	if err != nil {
		t.Fatal(err)
	}

	back, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	if string(back) != string(raw) {
		t.Errorf("placement shapes differ:\n api:       %s\n facadeset: %s", back, raw)
	}

	if !reflect.DeepEqual(input.placement(), placement) {
		t.Errorf("placement() = %+v, want %+v", input.placement(), placement)
	}

	feature := modelgeojson.GeoJSONFeature{
		Type: "Feature", ID: "w", Properties: map[string]any{"kind": "window"},
		Geometry: modelgeojson.Geometry{Type: "Point", Coordinates: []any{1.0, 2.0}},
	}

	want, err := json.Marshal(struct {
		Feature modelgeojson.GeoJSONFeature `json:"feature"`
	}{feature})
	if err != nil {
		t.Fatal(err)
	}

	got, err := json.Marshal(placeWindowResponseOf(feature))
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(want) {
		t.Errorf("response shapes differ:\n api: %s\n go:  %s", got, want)
	}
}

const rectifyCorners = `"image_points":[[100,900],[865.4,846.2],[885.1,114.9],[123.5,139.4]],` +
	`"facade_points":[[0,0],[10,0],[10,8],[0,8]]`

func TestFacadeRectifyEndpoint(t *testing.T) {
	t.Parallel()

	post := func(body string) *httptest.ResponseRecorder {
		req := newAPIRequest(http.MethodPost, "/api/v1/facades/rectify", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		NewHandler(mustStore(t, "Facade Rectify"), nil).ServeHTTP(rec, req)

		return rec
	}

	rec := post("{" + rectifyCorners + "}")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp rectifyFacadeResponse

	decodeResponse(t, rec.Body.Bytes(), &resp)

	want, err := facadeset.Rectify(facadeset.PhotoTransform{
		ImagePoints:  [][]float64{{100, 900}, {865.4, 846.2}, {885.1, 114.9}, {123.5, 139.4}},
		FacadePoints: [][]float64{{0, 0}, {10, 0}, {10, 8}, {0, 8}},
	})
	if err != nil || resp.Matrix != want.Matrix || resp.Inverse != want.Inverse {
		t.Errorf("answered %+v, facadeset %+v (%v)", resp, want, err)
	}

	rec = post(`{"image_points":[[100,900],[865.4,846.2],[885.1,114.9]],"facade_points":[[0,0],[10,0],[10,8],[0,8]]}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "image_points must hold 4 points") {
		t.Errorf("three corners: %d %s, want 400", rec.Code, rec.Body.String())
	}
}

// The API spells a transform and its answer as facadeset and the kernel do.
func TestRectifyFacadeDTOsMatchTheFacadesetContract(t *testing.T) {
	t.Parallel()

	transform := facadeset.PhotoTransform{
		ImagePoints:  [][]float64{{1, 2}, {3, 4}, {5, 6}, {7, 8}},
		FacadePoints: [][]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
	}

	raw, err := json.Marshal(transform)
	if err != nil {
		t.Fatal(err)
	}

	var req rectifyFacadeRequest

	err = json.Unmarshal(raw, &req)
	if err != nil {
		t.Fatal(err)
	}

	back, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	if string(back) != string(raw) {
		t.Errorf("transform shapes differ:\n api:       %s\n facadeset: %s", back, raw)
	}

	rect := facadeset.Rectification{Matrix: [9]float64{1, 2, 3, 4, 5, 6, 7, 8, 9}, Inverse: [9]float64{9, 8, 7, 6, 5, 4, 3, 2, 1}}

	want, err := json.Marshal(rect)
	if err != nil {
		t.Fatal(err)
	}

	got, err := json.Marshal(rectifyFacadeResponse{Matrix: rect.Matrix, Inverse: rect.Inverse})
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(want) {
		t.Errorf("response shapes differ:\n api: %s\n go:  %s", got, want)
	}
}
