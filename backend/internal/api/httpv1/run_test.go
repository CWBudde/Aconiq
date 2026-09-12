package httpv1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

func seedRun(t *testing.T, store projectfs.Store, status string) project.Run {
	t.Helper()

	run, _, err := store.CreateRun(projectfs.CreateRunSpec{
		ScenarioID: "default",
		Standard:   project.StandardRef{ID: "rls19-road", Version: "2019", Profile: "default"},
		Status:     status,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	return run
}

func deleteRun(t *testing.T, handler http.Handler, runID string) *httptest.ResponseRecorder {
	t.Helper()

	req := newAPIRequest(http.MethodDelete, "/api/v1/runs/"+runID, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func TestDeleteRunEndpointRemovesTheRun(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Delete Run API")
	run := seedRun(t, store, project.RunStatusCompleted)
	handler := NewHandler(store, nil)

	rec := deleteRun(t, handler, run.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response deleteRunResponse

	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.RunID != run.ID {
		t.Errorf("unexpected run_id: %q", response.RunID)
	}

	if !slices.Contains(response.RemovedPaths, ".noise/runs/"+run.ID) {
		t.Errorf("expected the run directory in removed_paths, got %#v", response.RemovedPaths)
	}

	// Both lists must serialise as arrays, never null: the client iterates them.
	var raw map[string]json.RawMessage

	decodeResponse(t, rec.Body.Bytes(), &raw)

	for _, member := range []string{"removed_paths", "retained_paths"} {
		if string(raw[member]) == "null" {
			t.Errorf("%s must be an array, not null", member)
		}
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) != 0 {
		t.Errorf("expected the run to be gone from the manifest, got %d", len(proj.Runs))
	}

	if _, statErr := os.Stat(filepath.Join(store.Root(), ".noise", "runs", run.ID)); !os.IsNotExist(statErr) {
		t.Errorf("expected the run directory to be gone, stat said: %v", statErr)
	}
}

func TestDeleteRunEndpointRefusesARunningRun(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Delete Running Run")
	run := seedRun(t, store, project.RunStatusRunning)
	handler := NewHandler(store, nil)

	assertErrorCode(t, deleteRun(t, handler, run.ID), http.StatusConflict, errorCodeRunNotFinished)
}

func TestDeleteRunEndpointReportsAnUnknownRun(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Delete Unknown Run"), nil)

	assertErrorCode(t, deleteRun(t, handler, "run-nope"), http.StatusNotFound, errorCodeNotFound)
}

func TestDeleteRunEndpointRejectsATraversingIdentifier(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Delete Traversing Run"), nil)

	// %2e%2e%2f decodes to "../" only after routing, so the segment reaches the
	// handler as a single path value and must be refused there.
	req := newAPIRequest(http.MethodDelete, "/api/v1/runs/%2e%2e%2f%2e%2e%2fetc", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("a traversing run id was accepted: %s", rec.Body.String())
	}
}

// DELETE is not a safe method, so it inherits the client-header guard that
// forces a CORS preflight. This asserts the guard actually covers the new route.
func TestDeleteRunEndpointRequiresTheClientHeader(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Delete Run Header")
	run := seedRun(t, store, project.RunStatusCompleted)
	handler := NewHandler(store, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/runs/"+run.ID, nil)
	req.Host = testHost

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertErrorCode(t, rec, http.StatusForbidden, errorCodeClientHeaderRequired)
}

// The more specific log pattern must keep winning over /api/v1/runs/{id}.
func TestRunLogRouteStillWinsOverTheRunResource(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Run Log Routing")
	run := seedRun(t, store, project.RunStatusCompleted)
	handler := NewHandler(store, nil)

	req := newAPIRequest(http.MethodGet, "/api/v1/runs/"+run.ID+"/log", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected the log endpoint to answer 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// And a GET of the run resource itself is now a 405, not a 404: the resource
	// is routed, the method is simply not offered.
	assertErrorCode(t, getRun(t, handler, run.ID), http.StatusMethodNotAllowed, errorCodeMethodNotAllowed)
}

func getRun(t *testing.T, handler http.Handler, runID string) *httptest.ResponseRecorder {
	t.Helper()

	req := newAPIRequest(http.MethodGet, "/api/v1/runs/"+runID, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func TestOpenAPIDocumentsRunDelete(t *testing.T) {
	t.Parallel()

	spec := BuildOpenAPISpec("")

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("expected paths object")
	}

	item, ok := paths["/api/v1/runs/{id}"].(map[string]any)
	if !ok {
		t.Fatal("expected the /api/v1/runs/{id} path item")
	}

	assertOperationResponses(t, item, "delete", []string{"200", "400", "404", "405", "409", "413"})

	components, ok := spec["components"].(map[string]any)
	if !ok {
		t.Fatal("expected components object")
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("expected schemas object")
	}

	if _, exists := schemas["DeleteRunResponse"]; !exists {
		t.Error("expected a DeleteRunResponse schema")
	}

	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("encode path item: %v", err)
	}

	if !strings.Contains(string(encoded), errorCodeRunNotFinished) {
		t.Errorf("expected the %s code to be documented on the operation", errorCodeRunNotFinished)
	}
}

// 415 comes from requireContentType, which only an operation that parses a body
// ever calls. Stamping it on a bodyless DELETE would document a refusal the
// server cannot produce.
func TestTransportContractStampsUnsupportedMediaTypeOnlyWhereThereIsABody(t *testing.T) {
	t.Parallel()

	spec := BuildOpenAPISpec("")

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("expected paths object")
	}

	deleteResponses := operationResponses(t, paths, "/api/v1/runs/{id}", "delete")
	if _, exists := deleteResponses["415"]; exists {
		t.Error("a bodyless DELETE must not document a 415")
	}

	// 413 and the client header stay on every non-GET, body or not.
	if _, exists := deleteResponses["413"]; !exists {
		t.Error("expected a 413 on the delete operation")
	}

	postResponses := operationResponses(t, paths, "/api/v1/runs", "post")
	if _, exists := postResponses["415"]; !exists {
		t.Error("an operation that parses a body must still document a 415")
	}
}

func operationResponses(t *testing.T, paths map[string]any, path string, method string) map[string]any {
	t.Helper()

	item, ok := paths[path].(map[string]any)
	if !ok {
		t.Fatalf("expected the %s path item", path)
	}

	operation, ok := item[method].(map[string]any)
	if !ok {
		t.Fatalf("expected a %s operation on %s", method, path)
	}

	responses, ok := operation["responses"].(map[string]any)
	if !ok {
		t.Fatalf("expected responses on %s %s", method, path)
	}

	return responses
}
