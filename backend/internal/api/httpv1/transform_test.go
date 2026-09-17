package httpv1

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo/crstransform"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/wasmkernel"
)

func postTransform(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := newAPIRequest(http.MethodPost, "/api/v1/transform", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func decodeTransformResponse(t *testing.T, rec *httptest.ResponseRecorder) transformResponse {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp transformResponse

	decodeResponse(t, rec.Body.Bytes(), &resp)

	return resp
}

// The zone an `auto` request resolves to has to be the zone `aconiq run` would
// pick for the same site, or a client can draw a model where the CLI refuses to
// compute it.
func TestTransformEndpointProjectsGeographicIntoTheZoneTheCLIWouldPick(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Transform Auto"), nil)

	rec := postTransform(t, handler,
		`{"source_crs":"EPSG:4326","target_crs":"auto","coordinates":[9.0,51.0,9.01,51.01]}`)

	resp := decodeTransformResponse(t, rec)

	if !resp.Applied {
		t.Error("expected the batch to have been projected")
	}

	if resp.TargetCRS != "EPSG:25832" {
		t.Errorf("target_crs = %q, want EPSG:25832", resp.TargetCRS)
	}

	// A metric CRS or the whole compute path is silently wrong: at 9°E the two
	// points are ~1.2 km apart, not 0.014 units.
	dx := resp.Coordinates[2] - resp.Coordinates[0]
	dy := resp.Coordinates[3] - resp.Coordinates[1]

	if distance := math.Hypot(dx, dy); distance < 1_000 || distance > 2_000 {
		t.Errorf("projected separation = %.1f m, want roughly 1.2 km", distance)
	}
}

// `applied: false` must mean the input values verbatim, not a round trip that
// happens to land close — a client compares them as stored coordinates.
func TestTransformEndpointLeavesAProjectedBatchUntouchedOnAuto(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Transform Projected"), nil)

	input := []float64{500000, 5700000, 500010.5, 5700010.5}

	rec := postTransform(t, handler,
		`{"source_crs":"EPSG:25832","target_crs":"auto","coordinates":[500000,5700000,500010.5,5700010.5]}`)

	resp := decodeTransformResponse(t, rec)

	if resp.Applied {
		t.Error("a projected CRS must not be moved by an auto request")
	}

	if resp.TargetCRS != "EPSG:25832" {
		t.Errorf("target_crs = %q, want the source back", resp.TargetCRS)
	}

	for i, want := range input {
		if resp.Coordinates[i] != want {
			t.Errorf("coordinate %d = %v, want %v verbatim", i, resp.Coordinates[i], want)
		}
	}
}

// The inverse direction is what a map needs in order to draw a metric model, and
// an explicit target transforms unconditionally so that it is available at all.
func TestTransformEndpointHonoursAnExplicitTarget(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Transform Inverse"), nil)

	rec := postTransform(t, handler,
		`{"source_crs":"EPSG:25832","target_crs":"EPSG:4326","coordinates":[500000,5700000]}`)

	resp := decodeTransformResponse(t, rec)

	if !resp.Applied {
		t.Fatal("an explicit target must always transform")
	}

	if lon, lat := resp.Coordinates[0], resp.Coordinates[1]; lon < 8 || lon > 10 || lat < 50 || lat > 52 {
		t.Errorf("inverse result (%v, %v) is not a plausible lon/lat for zone 32", lon, lat)
	}
}

// The endpoint is a pure function of its body: it reads no project and writes
// nothing, which is what keeps it usable before `aconiq init` has run and what
// keeps it out of the way of the invariant that the map is a projection *of* the
// model and never a source *for* it.
func TestTransformEndpointNeedsNoProject(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)

	rec := postTransform(t, handler,
		`{"source_crs":"EPSG:4326","target_crs":"auto","coordinates":[9.0,51.0]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 without an initialised project, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTransformEndpointRejectsMalformedRequests(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Transform Malformed"), nil)

	testCases := []struct {
		name     string
		body     string
		wantCode string
	}{
		{
			name:     "not JSON",
			body:     `{"source_crs":`,
			wantCode: errorCodeBadRequest,
		},
		{
			name:     "odd coordinate count",
			body:     `{"source_crs":"EPSG:4326","coordinates":[9.0,51.0,9.1]}`,
			wantCode: errorCodeBadRequest,
		},
		{
			name:     "missing source CRS",
			body:     `{"coordinates":[9.0,51.0]}`,
			wantCode: errorCodeBadRequest,
		},
		{
			name:     "unparsable source CRS",
			body:     `{"source_crs":"EPSG:nonsense","coordinates":[9.0,51.0]}`,
			wantCode: errorCodeBadRequest,
		},
		{
			name:     "unparsable target CRS",
			body:     `{"source_crs":"EPSG:4326","target_crs":"EPSG:nonsense","coordinates":[9.0,51.0]}`,
			wantCode: errorCodeBadRequest,
		},
		{
			name:     "site outside the supported zones",
			body:     `{"source_crs":"EPSG:4326","target_crs":"auto","coordinates":[-120.0,37.0]}`,
			wantCode: errorCodeCRSNotProjectable,
		},
		{
			name:     "southern hemisphere",
			body:     `{"source_crs":"EPSG:4326","target_crs":"auto","coordinates":[9.0,-51.0]}`,
			wantCode: errorCodeCRSNotProjectable,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			rec := postTransform(t, handler, testCase.body)

			assertErrorCode(t, rec, http.StatusBadRequest, testCase.wantCode)
		})
	}
}

// The refusal reaches the client with the projector's own wording, unprefixed.
//
// This is the load-bearing test of the endpoint. Every other handler here
// prefixes its cause — handleModelSave writes "model is not a valid GeoJSON
// FeatureCollection: " in front of its own — so a plausible, well-meant
// "could not project the batch: " would pass review. It must not: the sentence
// is quoted verbatim from geo.ComputeCRSForGeographic so that browser mode, API
// mode and `aconiq run` refuse the same site in the same words, and comparing
// against wasmkernel.Transform here is what pins all three together.
func TestTransformRefusesAnUnprojectableSiteInTheProjectorsWords(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Transform Wording"), nil)

	for _, body := range []string{
		`{"source_crs":"EPSG:4326","target_crs":"auto","coordinates":[9.0,-51.0]}`,
		`{"source_crs":"EPSG:4326","target_crs":"auto","coordinates":[-120.0,37.0]}`,
	} {
		_, kernelErr := wasmkernel.Transform([]byte(body))
		if kernelErr == nil {
			t.Fatalf("expected the kernel to refuse %s", body)
		}

		rec := postTransform(t, handler, body)

		var response errorResponse

		decodeResponse(t, rec.Body.Bytes(), &response)

		if response.Error.Message != kernelErr.Error() {
			t.Errorf("API message and kernel message differ:\n api:    %q\n kernel: %q",
				response.Error.Message, kernelErr.Error())
		}

		if reason, _ := response.Error.Details["reason"].(string); reason != string(crstransform.ReasonGeographicRefused) {
			t.Errorf("details.reason = %q, want %q", reason, crstransform.ReasonGeographicRefused)
		}
	}
}

func TestTransformEndpointRejectsBadMethod(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Transform Method"), nil)

	req := newAPIRequest(http.MethodGet, "/api/v1/transform", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertErrorCode(t, rec, http.StatusMethodNotAllowed, errorCodeMethodNotAllowed)
}

// httpv1 declares its own DTOs so that a change to the JavaScript boundary
// cannot silently move the HTTP contract. While the two are meant to agree, this
// is what stops them drifting.
func TestTransformDTOsMatchTheProjectorContract(t *testing.T) {
	t.Parallel()

	apiRequest, err := json.Marshal(transformRequest{
		SourceCRS:   "EPSG:4326",
		TargetCRS:   crstransform.AutoTarget,
		Coordinates: []float64{9, 51},
	})
	if err != nil {
		t.Fatalf("marshal API request: %v", err)
	}

	projectorRequest, err := json.Marshal(crstransform.Request{
		SourceCRS:   "EPSG:4326",
		TargetCRS:   crstransform.AutoTarget,
		Coordinates: []float64{9, 51},
	})
	if err != nil {
		t.Fatalf("marshal projector request: %v", err)
	}

	if string(apiRequest) != string(projectorRequest) {
		t.Errorf("request shapes differ:\n api:       %s\n projector: %s", apiRequest, projectorRequest)
	}

	apiResponse, err := json.Marshal(transformResponse{
		SourceCRS:   "EPSG:4326",
		TargetCRS:   "EPSG:25832",
		Applied:     true,
		Coordinates: []float64{500000, 5700000},
	})
	if err != nil {
		t.Fatalf("marshal API response: %v", err)
	}

	projectorResponse, err := json.Marshal(crstransform.Response{
		SourceCRS:   "EPSG:4326",
		TargetCRS:   "EPSG:25832",
		Applied:     true,
		Coordinates: []float64{500000, 5700000},
	})
	if err != nil {
		t.Fatalf("marshal projector response: %v", err)
	}

	if string(apiResponse) != string(projectorResponse) {
		t.Errorf("response shapes differ:\n api:       %s\n projector: %s", apiResponse, projectorResponse)
	}
}

func TestOpenAPIDocumentsTransform(t *testing.T) {
	t.Parallel()

	spec := BuildOpenAPISpec("")

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("expected paths object")
	}

	item, ok := paths["/api/v1/transform"].(map[string]any)
	if !ok {
		t.Fatal("expected /api/v1/transform path item")
	}

	assertOperationResponses(t, item, "post", []string{"200", "400", "405", "413", "415"})

	components, ok := spec["components"].(map[string]any)
	if !ok {
		t.Fatal("expected components object")
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("expected schemas object")
	}

	for _, name := range []string{"TransformRequest", "TransformResponse"} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("expected schema %s", name)
		}
	}

	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("encode path item: %v", err)
	}

	if !strings.Contains(string(encoded), errorCodeCRSNotProjectable) {
		t.Errorf("expected the %s error code to be documented on the operation", errorCodeCRSNotProjectable)
	}
}
