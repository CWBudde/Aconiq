package httpv1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/standards"
)

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	fixedTime := time.Date(2026, 3, 6, 9, 0, 0, 0, time.UTC)
	handler := NewHandler(store, func() time.Time { return fixedTime })

	req := newAPIRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response healthResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Status != "ok" {
		t.Fatalf("unexpected status: %q", response.Status)
	}

	if response.Version != apiVersion {
		t.Fatalf("unexpected version: %q", response.Version)
	}

	if !response.Time.Equal(fixedTime) {
		t.Fatalf("unexpected time: %s", response.Time)
	}
}

func TestProjectStatusEndpoint(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Phase23 API", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	handler := NewHandler(store, nil)
	req := newAPIRequest(http.MethodGet, "/api/v1/project/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response projectStatusResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.ProjectID == "" {
		t.Fatal("project_id must be set")
	}

	if response.Name != "Phase23 API" {
		t.Fatalf("unexpected name: %q", response.Name)
	}

	if response.ProjectPath != projectDir {
		t.Fatalf("unexpected project path: %q", response.ProjectPath)
	}
}

// The REST status and the SSE `project_status` event answer the same question,
// so they must answer it with the same bytes. They were built by two separate
// literals once, and drifted: the stream omitted the last run's context. One
// builder now serves both, and this pins that.
func TestProjectStatusStreamEventMatchesRESTStatus(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Status Parity")
	mustCreateCompletedRun(t, store)

	handler := NewHandler(store, nil)
	req := newAPIRequest(http.MethodGet, "/api/v1/project/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	event, _ := Handler{store: store, now: time.Now}.buildProjectStatusStreamEvent()

	streamPayload, ok := event["project"]
	if !ok {
		t.Fatal("stream event carries no project member")
	}

	restBytes := remarshalJSON(t, rec.Body.Bytes())
	streamBytes := remarshalValue(t, streamPayload)

	if !bytes.Equal(restBytes, streamBytes) {
		t.Fatalf("status payloads differ:\n REST: %s\nsteam: %s", restBytes, streamBytes)
	}

	// The parity is worth nothing if both are empty of the members that drifted.
	if !bytes.Contains(restBytes, []byte(`"context":"planning"`)) {
		t.Fatalf("expected the last run's context in the status payload: %s", restBytes)
	}
}

// remarshalJSON re-encodes a JSON document so two payloads can be compared as
// bytes without their indentation mattering. encoding/json sorts map keys, so
// the result is canonical.
func remarshalJSON(t *testing.T, payload []byte) []byte {
	t.Helper()

	var decoded any

	decodeResponse(t, payload, &decoded)

	return remarshalValue(t, decoded)
}

func remarshalValue(t *testing.T, value any) []byte {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}

	var decoded any

	decodeResponse(t, encoded, &decoded)

	canonical, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-encode payload: %v", err)
	}

	return canonical
}

func TestProjectStatusReturnsNotFoundWhenProjectNotInitialized(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	req := newAPIRequest(http.MethodGet, "/api/v1/project/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "not_found" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestMethodNotAllowedReturnsStandardizedError(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	req := newAPIRequest(http.MethodPost, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "method_not_allowed" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestUnknownRouteReturnsStandardizedNotFound(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	req := newAPIRequest(http.MethodGet, "/api/v1/nope", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "not_found" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestOpenAPIEndpoint(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)

	req := newAPIRequest(http.MethodGet, "/api/v1/openapi.json", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload map[string]any
	decodeResponse(t, rec.Body.Bytes(), &payload)

	if payload["openapi"] != OpenAPIVersion {
		t.Fatalf("unexpected openapi version: %#v", payload["openapi"])
	}

	paths, ok := payload["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected openapi paths object")
	}

	if _, exists := paths["/api/v1/events"]; !exists {
		t.Fatalf("expected /api/v1/events path in openapi document")
	}

	for _, required := range []string{
		"/api/v1/artifacts/{id}/content",
		"/api/v1/import/osm",
		"/api/v1/import/terrain",
		"/api/v1/model",
		"/api/v1/runs",
		"/api/v1/runs/{id}",
		"/api/v1/runs/{id}/contours",
		"/api/v1/runs/{id}/log",
		"/api/v1/standards",
		"/api/v1/transform",
	} {
		if _, exists := paths[required]; !exists {
			t.Fatalf("expected %s path in openapi document", required)
		}
	}

	assertOpenAPIDeclaresEvidenceTier(t, payload)
	assertOpenAPIDeclaresParameterUnit(t, payload)
}

// assertOpenAPIDeclaresEvidenceTier checks that the hand-built spec documents the
// evidence tier a consumer receives from GET /api/v1/standards, including the
// enum of accepted tiers.
func assertOpenAPIDeclaresEvidenceTier(t *testing.T, payload map[string]any) {
	t.Helper()

	components, ok := payload["components"].(map[string]any)
	if !ok {
		t.Fatalf("expected openapi components object")
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatalf("expected openapi schemas object")
	}

	descriptor, ok := schemas["StandardDescriptor"].(map[string]any)
	if !ok {
		t.Fatalf("expected StandardDescriptor schema")
	}

	required := anySliceToStrings(descriptor["required"])
	if !slices.Contains(required, "evidence_tier") {
		t.Fatalf("expected evidence_tier to be required, got %#v", descriptor["required"])
	}

	properties, ok := descriptor["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected StandardDescriptor properties object")
	}

	tier, ok := properties["evidence_tier"].(map[string]any)
	if !ok {
		t.Fatalf("expected evidence_tier property, got %#v", properties["evidence_tier"])
	}

	if tier["type"] != "string" {
		t.Fatalf("unexpected evidence_tier type: %#v", tier["type"])
	}

	if description, isText := tier["description"].(string); !isText || description == "" {
		t.Fatalf("expected evidence_tier description, got %#v", tier["description"])
	}

	enum := anySliceToStrings(tier["enum"])
	for _, expected := range []string{"normative", "preview", "scaffold", "test-fixture"} {
		if !slices.Contains(enum, expected) {
			t.Fatalf("expected %q in the evidence_tier enum, got %#v", expected, enum)
		}
	}
}

// Every schema below sets additionalProperties:false, so a member the server
// emits but the schema omits makes a strict consumer reject the whole response.
// All three carry the standard's assessment context.
func TestOpenAPIDocumentsStandardContext(t *testing.T) {
	t.Parallel()

	spec := BuildOpenAPISpec("")

	components, ok := spec["components"].(map[string]any)
	if !ok {
		t.Fatal("expected components object")
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("expected schemas object")
	}

	for _, name := range []string{"LastRunStatus", "RunSummary", "StandardDescriptor"} {
		schema, isObject := schemas[name].(map[string]any)
		if !isObject {
			t.Fatalf("expected %s schema", name)
		}

		properties, isObject := schema["properties"].(map[string]any)
		if !isObject {
			t.Fatalf("expected %s properties object", name)
		}

		context, isObject := properties["context"].(map[string]any)
		if !isObject {
			t.Fatalf("%s: expected a context property, got %#v", name, properties["context"])
		}

		if context["type"] != "string" {
			t.Errorf("%s: unexpected context type %#v", name, context["type"])
		}

		enum, isStrings := context["enum"].([]string)
		if !isStrings || !slices.Contains(enum, "planning") || !slices.Contains(enum, "mapping") {
			t.Errorf("%s: unexpected context enum %#v", name, context["enum"])
		}
	}

	// standardResponse.Context has no omitempty, so GET /api/v1/standards always
	// emits the key and a consumer may rely on it.
	descriptor, ok := schemas["StandardDescriptor"].(map[string]any)
	if !ok {
		t.Fatal("expected StandardDescriptor schema")
	}

	required, ok := descriptor["required"].([]string)
	if !ok || !slices.Contains(required, "context") {
		t.Fatalf("expected context to be required on StandardDescriptor, got %#v", descriptor["required"])
	}
}

func TestRunsListEndpointReturnsEmptyListWhenNoRuns(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Runs Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	handler := NewHandler(store, nil)
	req := newAPIRequest(http.MethodGet, "/api/v1/runs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response []runSummaryResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if len(response) != 0 {
		t.Fatalf("expected empty list, got %d runs", len(response))
	}
}

func TestRunLogEndpointReturnsNotFoundForUnknownRun(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Runs Log Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	handler := NewHandler(store, nil)
	req := newAPIRequest(http.MethodGet, "/api/v1/runs/run-nope/log", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Code != "not_found" {
		t.Fatalf("unexpected error code: %q", response.Error.Code)
	}
}

func TestStandardsEndpointReturnsRegisteredStandards(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	registry, err := standards.NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	handler := NewHandlerWithRegistry(store, nil, registry)
	req := newAPIRequest(http.MethodGet, "/api/v1/standards", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response []standardResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if len(response) == 0 {
		t.Fatal("expected at least one standard")
	}

	validTiers := []string{"normative", "preview", "scaffold", "test-fixture"}

	found := false
	foundSchall03 := false
	foundISO9613 := false

	for _, s := range response {
		if !slices.Contains(validTiers, s.EvidenceTier) {
			t.Fatalf("%s: unexpected evidence tier %q", s.ID, s.EvidenceTier)
		}

		if s.ID == "rls19-road" {
			found = true

			if s.EvidenceTier != "normative" {
				t.Fatalf("rls19-road: unexpected evidence tier %q", s.EvidenceTier)
			}

			if len(s.Versions) == 0 {
				t.Fatal("rls19-road: expected at least one version")
			}

			if len(s.Versions[0].Profiles) == 0 {
				t.Fatal("rls19-road: expected at least one profile")
			}

			if len(s.Versions[0].Profiles[0].Parameters) == 0 {
				t.Fatal("rls19-road: expected parameters")
			}
		}

		if s.ID == "schall03" {
			foundSchall03 = true

			if len(s.Versions) == 0 {
				t.Fatal("schall03: expected at least one version")
			}

			if s.Context != "planning" {
				t.Fatalf("schall03: unexpected context %q", s.Context)
			}
		}

		if s.ID == "cnossos-road" && s.EvidenceTier != "scaffold" {
			t.Fatalf("cnossos-road: unexpected evidence tier %q", s.EvidenceTier)
		}

		if s.ID == "dummy-freefield" && s.EvidenceTier != "test-fixture" {
			t.Fatalf("dummy-freefield: unexpected evidence tier %q", s.EvidenceTier)
		}

		if s.ID == "iso9613" {
			foundISO9613 = true

			if len(s.Versions) == 0 {
				t.Fatal("iso9613: expected at least one version")
			}

			if s.Context != "planning" {
				t.Fatalf("iso9613: unexpected context %q", s.Context)
			}
		}
	}

	if !found {
		t.Fatal("expected rls19-road in standards list")
	}

	if !foundSchall03 {
		t.Fatal("expected schall03 in standards list")
	}

	if !foundISO9613 {
		t.Fatal("expected iso9613 in standards list")
	}
}

func TestStandardsEndpointReturnsUnavailableWithoutRegistry(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	handler := NewHandler(store, nil)
	req := newAPIRequest(http.MethodGet, "/api/v1/standards", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
