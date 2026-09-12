package httpv1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// GET and POST are both served now, so the method refusal has to be shown with
// a method that is neither.
func TestModelEndpointRejectsBadMethod(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Model Method"), nil)

	req := newAPIRequest(http.MethodPut, "/api/v1/model", nil)
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
	assertOperationResponses(t, item, "get", []string{"200", "400", "404", "405"})

	components, ok := spec["components"].(map[string]any)
	if !ok {
		t.Fatal("expected components object")
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("expected schemas object")
	}

	for _, name := range []string{"ModelSaveRequest", "ModelSaveResponse", "ModelResponse", "ProjectModelStatus", "ValidationIssue"} {
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

func getModel(t *testing.T, handler http.Handler, query string) *httptest.ResponseRecorder {
	t.Helper()

	req := newAPIRequest(http.MethodGet, "/api/v1/model"+query, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

// A project that loaded but holds no model is a different situation from a
// project that does not exist, and the frontend has to be able to act on the
// difference — so it gets its own code, and must not inherit the not-found
// hint that tells the user to run `aconiq init`.
func TestModelGetReportsModelNotFoundBeforeAnySave(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Model Get Empty"), nil)

	rec := getModel(t, handler, "")

	assertErrorCode(t, rec, http.StatusNotFound, errorCodeModelNotFound)

	var response errorResponse

	decodeResponse(t, rec.Body.Bytes(), &response)

	if strings.Contains(response.Error.Hint, "Initialize the project") {
		t.Fatalf("a saved-model miss must not read as a missing project: %q", response.Error.Hint)
	}
}

func TestModelGetReturnsTheStoredModelAndTheSaveHash(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Model Get")
	handler := NewHandler(store, nil)

	saved := postModel(t, handler, `{"model": `+validModelFeatureCollection+`}`)
	if saved.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", saved.Code, saved.Body.String())
	}

	var saveResponse modelSaveResponse

	decodeResponse(t, saved.Body.Bytes(), &saveResponse)

	if saveResponse.Hash == "" {
		t.Fatal("the save response carried no hash")
	}

	rec := getModel(t, handler, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response modelGetResponse

	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Hash != saveResponse.Hash {
		t.Errorf("the read receipt differs from the write receipt: %q vs %q", response.Hash, saveResponse.Hash)
	}

	if response.FeatureCount != 4 {
		t.Errorf("expected feature_count 4, got %d", response.FeatureCount)
	}

	if response.CRS != "EPSG:25832" || response.ProjectCRS != "EPSG:25832" {
		t.Errorf("unexpected crs pair: %q / %q", response.CRS, response.ProjectCRS)
	}

	// Without a requested CRS the payload is the file, not a re-rendering of it.
	// The envelope is re-indented on the way out, so the comparison is made on
	// the compacted forms; member order still survives it, and member order is
	// exactly what a round trip through a struct would not preserve.
	onDisk, err := os.ReadFile(store.ModelArtifactPaths().Normalized)
	if err != nil {
		t.Fatalf("read normalized model: %v", err)
	}

	if !bytes.Equal(compactJSON(t, response.Model), compactJSON(t, onDisk)) {
		t.Errorf("expected the stored document verbatim, got %s", response.Model)
	}
}

func compactJSON(t *testing.T, payload []byte) []byte {
	t.Helper()

	var out bytes.Buffer

	err := json.Compact(&out, payload)
	if err != nil {
		t.Fatalf("compact json: %v", err)
	}

	return out.Bytes()
}

// The point of the CRS parameter: a project in a projected CRS can be handed to
// a web map, which only speaks WGS84.
func TestModelGetReprojectsOnRequest(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Model Get CRS"), nil)

	if rec := postModel(t, handler, `{"model": `+validModelFeatureCollection+`}`); rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	rec := getModel(t, handler, "?crs=EPSG:4326")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response modelGetResponse

	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.CRS != "EPSG:4326" {
		t.Errorf("expected the response to name the requested CRS, got %q", response.CRS)
	}

	if response.ProjectCRS != "EPSG:25832" {
		t.Errorf("expected the project CRS alongside it, got %q", response.ProjectCRS)
	}

	var collection modelgeojson.FeatureCollection

	decodeResponse(t, response.Model, &collection)

	if len(collection.Features) != 4 {
		t.Fatalf("expected 4 features, got %d", len(collection.Features))
	}

	// The source sits at 500000 / 5700000 in EPSG:25832, which is roughly
	// 9.0° E, 51.5° N. The tolerance is wide on purpose: this asserts that a
	// transform ran and landed in the right place, not the transform's accuracy.
	lon, lat := firstPointCoordinates(t, collection)
	if lon < 8.5 || lon > 9.5 {
		t.Errorf("longitude %v is not a WGS84 longitude for this point", lon)
	}

	if lat < 51.0 || lat > 52.0 {
		t.Errorf("latitude %v is not a WGS84 latitude for this point", lat)
	}
}

func TestModelGetRejectsAnUnparseableCRS(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Model Get Bad CRS"), nil)

	if rec := postModel(t, handler, `{"model": `+validModelFeatureCollection+`}`); rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	assertErrorCode(t, getModel(t, handler, "?crs=EPSG:garbage"), http.StatusBadRequest, errorCodeBadRequest)
}

// The status hash is what lets a restored draft prove it already equals the
// project without fetching the model, so it has to be the same string the save
// handed out — and absent when there is nothing to compare against.
func TestProjectStatusCarriesTheModelHash(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Model Status"), nil)

	before := projectStatusOf(t, handler)
	if before.Model != nil {
		t.Fatalf("expected no model member before any save, got %#v", before.Model)
	}

	saved := postModel(t, handler, `{"model": `+validModelFeatureCollection+`}`)
	if saved.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", saved.Code, saved.Body.String())
	}

	var saveResponse modelSaveResponse

	decodeResponse(t, saved.Body.Bytes(), &saveResponse)

	after := projectStatusOf(t, handler)
	if after.Model == nil {
		t.Fatal("expected a model member after a save")
	}

	if after.Model.Hash != saveResponse.Hash {
		t.Errorf("status hash %q differs from the save receipt %q", after.Model.Hash, saveResponse.Hash)
	}

	if after.Model.UpdatedAt.IsZero() {
		t.Error("expected the model artifact's timestamp")
	}
}

func projectStatusOf(t *testing.T, handler http.Handler) projectStatusResponse {
	t.Helper()

	req := newAPIRequest(http.MethodGet, "/api/v1/project/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var status projectStatusResponse

	decodeResponse(t, rec.Body.Bytes(), &status)

	return status
}

// firstPointCoordinates pulls the lon/lat out of the first Point feature.
func firstPointCoordinates(t *testing.T, collection modelgeojson.FeatureCollection) (float64, float64) {
	t.Helper()

	for _, feature := range collection.Features {
		if feature.Geometry.Type != "Point" {
			continue
		}

		pair, ok := feature.Geometry.Coordinates.([]any)
		if !ok || len(pair) != 2 {
			t.Fatalf("unexpected point coordinates: %#v", feature.Geometry.Coordinates)
		}

		x, okX := pair[0].(float64)
		y, okY := pair[1].(float64)

		if !okX || !okY {
			t.Fatalf("unexpected point coordinates: %#v", pair)
		}

		return x, y
	}

	t.Fatal("the collection carries no Point feature")

	return 0, 0
}

// The SSE stream suppresses an event whose dedupe key is unchanged. Saving a
// model changes the payload and nothing the key used to be made of, so without
// the model hash in the key the stream would serve the first hash forever while
// every other test still passed.
func TestProjectStatusStreamKeyFollowsTheModelHash(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Stream Model Key")
	handler := NewHandler(store, nil)
	streamer := Handler{store: store, now: time.Now}

	_, beforeKey := streamer.buildProjectStatusStreamEvent()

	saved := postModel(t, handler, `{"model": `+validModelFeatureCollection+`}`)
	if saved.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", saved.Code, saved.Body.String())
	}

	var saveResponse modelSaveResponse

	decodeResponse(t, saved.Body.Bytes(), &saveResponse)

	event, afterKey := streamer.buildProjectStatusStreamEvent()

	if afterKey == beforeKey {
		t.Fatalf("the dedupe key did not move when the model was saved: %q", afterKey)
	}

	if !strings.Contains(afterKey, saveResponse.Hash) {
		t.Errorf("expected the model hash in the dedupe key %q", afterKey)
	}

	status, ok := event["project"].(projectStatusResponse)
	if !ok {
		t.Fatalf("unexpected stream project member: %#v", event["project"])
	}

	if status.Model == nil || status.Model.Hash != saveResponse.Hash {
		t.Errorf("the stream event does not carry the saved model's hash: %#v", status.Model)
	}
}
