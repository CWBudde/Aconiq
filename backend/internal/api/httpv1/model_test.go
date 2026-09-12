package httpv1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// validModelFeatureCollection is one feature of every kind the v1 input schema
// knows, in the EPSG:25832 coordinates mustStore's project uses.
const validModelFeatureCollection = `{
  "type": "FeatureCollection",
  "features": [
    {"type": "Feature", "id": "src-1", "properties": {"kind": "source", "source_type": "point"},
     "geometry": {"type": "Point", "coordinates": [500000, 5700000]}},
    {"type": "Feature", "id": "bld-1", "properties": {"kind": "building", "height_m": 12},
     "geometry": {"type": "Polygon", "coordinates": [[[500010, 5700010], [500020, 5700010], [500020, 5700020], [500010, 5700020], [500010, 5700010]]]}},
    {"type": "Feature", "id": "bar-1", "properties": {"kind": "barrier", "height_m": 3},
     "geometry": {"type": "LineString", "coordinates": [[500005, 5700000], [500005, 5700030]]}},
    {"type": "Feature", "id": "rcv-1", "properties": {"kind": "receiver", "height_m": 4},
     "geometry": {"type": "Point", "coordinates": [500050, 5700050]}}
  ]
}`

func postModel(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := newAPIRequest(http.MethodPost, "/api/v1/model", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func readNormalizedModel(t *testing.T, store projectfs.Store) modelgeojson.FeatureCollection {
	t.Helper()

	payload, err := os.ReadFile(store.ModelArtifactPaths().Normalized)
	if err != nil {
		t.Fatalf("read normalized model: %v", err)
	}

	var collection modelgeojson.FeatureCollection

	decodeResponse(t, payload, &collection)

	return collection
}

func TestModelSaveEndpointWritesModelAndManifest(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Model Save")
	handler := NewHandler(store, nil)

	rec := postModel(t, handler, `{"model": `+validModelFeatureCollection+`}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var response modelSaveResponse

	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.NormalizedPath != ".noise/model/model.normalized.geojson" {
		t.Errorf("unexpected normalized_path: %q", response.NormalizedPath)
	}

	if response.DumpPath != ".noise/model/model.dump.json" {
		t.Errorf("unexpected dump_path: %q", response.DumpPath)
	}

	if response.ValidationReportPath != ".noise/model/validation-report.json" {
		t.Errorf("unexpected validation_report_path: %q", response.ValidationReportPath)
	}

	if response.FeatureCount != 4 {
		t.Errorf("expected feature_count 4, got %d", response.FeatureCount)
	}

	if response.Warnings == nil {
		t.Error("expected warnings to be an empty list, not null")
	}

	// The three files are on disk, under the paths the response named.
	for _, rel := range []string{response.NormalizedPath, response.DumpPath, response.ValidationReportPath} {
		if _, err := os.Stat(filepath.Join(store.Root(), filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected %s on disk: %v", rel, err)
		}
	}

	normalized := readNormalizedModel(t, store)
	if len(normalized.Features) != 4 {
		t.Errorf("expected 4 normalized features, got %d", len(normalized.Features))
	}

	var dump modelgeojson.ModelDump

	dumpPayload, err := os.ReadFile(store.ModelArtifactPaths().Dump)
	if err != nil {
		t.Fatalf("read model dump: %v", err)
	}

	decodeResponse(t, dumpPayload, &dump)

	if dump.SourcePath != modelSaveSourceLabel {
		t.Errorf("expected provenance source %q, got %q", modelSaveSourceLabel, dump.SourcePath)
	}

	// The manifest carries the same three refs `aconiq import` registers.
	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	assertModelArtifactRefs(t, proj.Artifacts, map[string]string{
		project.ArtifactIDModelNormalized: response.NormalizedPath,
		project.ArtifactIDModelDump:       response.DumpPath,
		project.ArtifactIDModelValidation: response.ValidationReportPath,
	})
}

func TestModelSaveEndpointReplacesThePreviousModel(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Model Replace")
	handler := NewHandler(store, nil)

	rec := postModel(t, handler, `{"model": `+validModelFeatureCollection+`}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first save: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	single := `{"type": "FeatureCollection", "features": [
		{"type": "Feature", "id": "rcv-only", "properties": {"kind": "receiver", "height_m": 4},
		 "geometry": {"type": "Point", "coordinates": [500050, 5700050]}}]}`

	rec = postModel(t, handler, `{"model": `+single+`}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("second save: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	normalized := readNormalizedModel(t, store)
	if len(normalized.Features) != 1 {
		t.Fatalf("expected the second model to replace the first, got %d features", len(normalized.Features))
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Artifacts) != 3 {
		t.Fatalf("expected the model refs to be upserted, not duplicated: got %d refs", len(proj.Artifacts))
	}
}

func TestModelSaveEndpointReprojectsFromTheRequestCRS(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Model CRS")
	handler := NewHandler(store, nil)

	// A receiver in WGS84 lng/lat, as the map draws it; the project is EPSG:25832.
	body := `{"crs": "EPSG:4326", "model": {"type": "FeatureCollection", "features": [
		{"type": "Feature", "id": "rcv-1", "properties": {"kind": "receiver", "height_m": 4},
		 "geometry": {"type": "Point", "coordinates": [9.0, 51.0]}}]}}`

	rec := postModel(t, handler, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	normalized := readNormalizedModel(t, store)
	if len(normalized.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(normalized.Features))
	}

	coords, ok := normalized.Features[0].Geometry.Coordinates.([]any)
	if !ok || len(coords) != 2 {
		t.Fatalf("unexpected coordinates: %#v", normalized.Features[0].Geometry.Coordinates)
	}

	easting, _ := coords[0].(float64)
	northing, _ := coords[1].(float64)

	// 9°E 51°N is the central meridian of zone 32, so easting is the false
	// easting and northing is roughly 51° of meridian arc.
	if easting < 499_000 || easting > 501_000 || northing < 5_640_000 || northing > 5_660_000 {
		t.Fatalf("expected EPSG:25832 coordinates, got [%v, %v]", easting, northing)
	}
}

func TestModelSaveEndpointRejectsInvalidModelWithoutWriting(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Model Invalid")
	handler := NewHandler(store, nil)

	// A building without height_m fails validation; the other feature is fine.
	body := `{"model": {"type": "FeatureCollection", "features": [
		{"type": "Feature", "id": "bld-no-height", "properties": {"kind": "building"},
		 "geometry": {"type": "Polygon", "coordinates": [[[0, 0], [10, 0], [10, 10], [0, 10], [0, 0]]]}},
		{"type": "Feature", "id": "rcv-1", "properties": {"kind": "receiver", "height_m": 4},
		 "geometry": {"type": "Point", "coordinates": [500050, 5700050]}}]}}`

	rec := postModel(t, handler, body)

	assertErrorCode(t, rec, http.StatusBadRequest, errorCodeModelInvalid)

	var response errorResponse

	decodeResponse(t, rec.Body.Bytes(), &response)

	rawErrors, ok := response.Error.Details["errors"].([]any)
	if !ok || len(rawErrors) == 0 {
		t.Fatalf("expected details.errors to list the validation errors, got %#v", response.Error.Details)
	}

	first, ok := rawErrors[0].(map[string]any)
	if !ok {
		t.Fatalf("expected an issue object, got %#v", rawErrors[0])
	}

	if first["feature_id"] != "bld-no-height" {
		t.Errorf("expected the issue to name the failing feature, got %#v", first)
	}

	if first["code"] == "" || first["message"] == "" {
		t.Errorf("expected code and message on the issue, got %#v", first)
	}

	if response.Error.Hint == "" {
		t.Error("expected a hint telling the caller to fix the listed features")
	}

	// Nothing reached the project: no model files, no artifact refs.
	if _, err := os.Stat(store.ModelArtifactPaths().Normalized); !os.IsNotExist(err) {
		t.Errorf("expected no normalized model on disk, stat returned %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Artifacts) != 0 {
		t.Errorf("expected no artifact refs after a rejected model, got %d", len(proj.Artifacts))
	}
}

func TestModelSaveEndpointRejectsEmptyFeatureCollection(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Model Empty"), nil)

	rec := postModel(t, handler, `{"model": {"type": "FeatureCollection", "features": []}}`)

	assertErrorCode(t, rec, http.StatusBadRequest, errorCodeModelInvalid)

	var response errorResponse

	decodeResponse(t, rec.Body.Bytes(), &response)

	rawErrors, ok := response.Error.Details["errors"].([]any)
	if !ok || len(rawErrors) != 1 {
		t.Fatalf("expected exactly one model-wide error, got %#v", response.Error.Details)
	}

	issue, ok := rawErrors[0].(map[string]any)
	if !ok {
		t.Fatalf("expected an issue object, got %#v", rawErrors[0])
	}

	if _, present := issue["feature_id"]; present {
		t.Errorf("a model-wide finding must carry no feature_id, got %#v", issue)
	}

	if issue["code"] != "model.empty" {
		t.Errorf("expected code model.empty, got %#v", issue["code"])
	}
}

func TestModelSaveEndpointRejectsUnknownCRS(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Model CRS Garbage"), nil)

	rec := postModel(t, handler, `{"crs": "garbage", "model": `+validModelFeatureCollection+`}`)

	assertErrorCode(t, rec, http.StatusBadRequest, errorCodeBadRequest)
}

func TestModelSaveEndpointReportsPersistFailureWithoutLeakingPaths(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Model Persist Failure")
	handler := NewHandler(store, nil)

	// A regular file where the model directory has to go makes MkdirAll fail
	// before anything is written.
	modelDir := filepath.Dir(store.ModelArtifactPaths().Normalized)

	err := os.WriteFile(modelDir, []byte("not a directory"), 0o600)
	if err != nil {
		t.Fatalf("plant blocking file: %v", err)
	}

	rec := postModel(t, handler, `{"model": `+validModelFeatureCollection+`}`)

	assertErrorCode(t, rec, http.StatusInternalServerError, errorCodeInternalError)

	if body := rec.Body.String(); strings.Contains(body, store.Root()) {
		t.Errorf("response must not leak the project's absolute path: %s", body)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Artifacts) != 0 {
		t.Errorf("expected the manifest untouched after a persist failure, got %d refs", len(proj.Artifacts))
	}
}

func TestModelSaveEndpointRejectsMalformedRequests(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Model Bad Request")
	handler := NewHandler(store, nil)

	for name, body := range map[string]string{
		"not json":                 `not-json`,
		"missing model":            `{"crs": "EPSG:4326"}`,
		"null model":               `{"model": null}`,
		"not a feature collection": `{"model": {"type": "Point", "coordinates": [1, 2]}}`,
		"feature without props":    `{"model": {"type": "FeatureCollection", "features": [{"type": "Feature", "geometry": {"type": "Point", "coordinates": [1, 2]}}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec := postModel(t, handler, body)

			assertErrorCode(t, rec, http.StatusBadRequest, errorCodeBadRequest)
		})
	}
}

func TestModelSaveEndpointReturnsNotFoundWhenProjectNotInitialized(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)

	rec := postModel(t, handler, `{"model": `+validModelFeatureCollection+`}`)

	assertErrorCode(t, rec, http.StatusNotFound, errorCodeNotFound)
}

func TestModelSaveEndpointRejectsBadMethod(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Model Method"), nil)

	req := newAPIRequest(http.MethodGet, "/api/v1/model", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertErrorCode(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestOpenAPIDocumentsModelSave(t *testing.T) {
	t.Parallel()

	spec := BuildOpenAPISpec("")

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("expected paths object")
	}

	item, ok := paths["/api/v1/model"].(map[string]any)
	if !ok {
		t.Fatal("expected /api/v1/model path item")
	}

	assertOperationResponses(t, item, "post", []string{"201", "400", "404", "405", "413", "415"})

	components, ok := spec["components"].(map[string]any)
	if !ok {
		t.Fatal("expected components object")
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("expected schemas object")
	}

	for _, name := range []string{"ModelSaveRequest", "ModelSaveResponse", "ValidationIssue"} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("expected schema %s", name)
		}
	}

	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("encode path item: %v", err)
	}

	if !strings.Contains(string(encoded), errorCodeModelInvalid) {
		t.Errorf("expected the %s error code to be documented on the operation", errorCodeModelInvalid)
	}
}

func assertModelArtifactRefs(t *testing.T, artifacts []project.ArtifactRef, wantPaths map[string]string) {
	t.Helper()

	wantKinds := map[string]string{
		project.ArtifactIDModelNormalized: project.ArtifactKindModelNormalizedGeoJSON,
		project.ArtifactIDModelDump:       project.ArtifactKindModelDumpJSON,
		project.ArtifactIDModelValidation: project.ArtifactKindModelValidationReport,
	}

	for id, wantPath := range wantPaths {
		found := false

		for _, ref := range artifacts {
			if ref.ID != id {
				continue
			}

			found = true

			if ref.Kind != wantKinds[id] {
				t.Errorf("%s: expected kind %q, got %q", id, wantKinds[id], ref.Kind)
			}

			if ref.Path != wantPath {
				t.Errorf("%s: expected path %q, got %q", id, wantPath, ref.Path)
			}
		}

		if !found {
			t.Errorf("artifact %s missing from manifest", id)
		}
	}
}
