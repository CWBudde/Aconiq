package httpv1

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// The route serves the file as written, not a re-encoding of it: provenance is
// the record a reader checks a run against, and a server that re-marshals it
// could drop a field the struct of the day does not know.
func TestRunProvenanceServesTheFileVerbatim(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Provenance")

	run, _, err := store.CreateRun(projectfs.CreateRunSpec{
		Standard: project.StandardRef{ID: "dummy-freefield", Version: "v1"},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	want, err := os.ReadFile(filepath.Join(store.Root(), filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	handler := NewHandler(store, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAPIRequest(http.MethodGet, "/api/v1/runs/"+run.ID+"/provenance", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content type = %q, want application/json", got)
	}

	if !bytes.Equal(rec.Body.Bytes(), want) {
		t.Fatalf("body differs from provenance.json:\n got %s\nwant %s", rec.Body.Bytes(), want)
	}
}

func TestRunProvenanceReturnsNotFoundForUnknownRun(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Provenance"), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAPIRequest(http.MethodGet, "/api/v1/runs/run-nope/provenance", nil))

	assertErrorCode(t, rec, http.StatusNotFound, errorCodeNotFound)
}

// A run row whose file has gone is a damaged project, but it is the run's
// provenance that is missing, not the server that failed.
func TestRunProvenanceReturnsNotFoundWhenTheFileIsMissing(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Provenance")

	run, _, err := store.CreateRun(projectfs.CreateRunSpec{
		Standard: project.StandardRef{ID: "dummy-freefield", Version: "v1"},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	err = os.Remove(filepath.Join(store.Root(), filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("remove provenance: %v", err)
	}

	handler := NewHandler(store, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAPIRequest(http.MethodGet, "/api/v1/runs/"+run.ID+"/provenance", nil))

	assertErrorCode(t, rec, http.StatusNotFound, errorCodeNotFound)
}

// The manifest is untrusted data, as for the run log: a provenance_path that
// leaves the project root must not become an HTTP read of what it names.
func TestRunProvenanceRefusesPathOutsideProjectRoot(t *testing.T) {
	t.Parallel()

	for _, provenancePath := range []string{"../../../../etc/passwd", "/etc/passwd"} {
		store := mustStore(t, "Traversal")

		proj, err := store.Load()
		if err != nil {
			t.Fatalf("load project: %v", err)
		}

		proj.Runs = append(proj.Runs, project.Run{
			ID:             "run-evil",
			Status:         project.RunStatusCompleted,
			ProvenancePath: provenancePath,
			Standard:       project.StandardRef{ID: "rls19-road", Version: "2019"},
		})

		err = store.Save(proj)
		if err != nil {
			t.Fatalf("save project: %v", err)
		}

		handler := NewHandler(store, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, newAPIRequest(http.MethodGet, "/api/v1/runs/run-evil/provenance", nil))

		assertErrorCode(t, rec, http.StatusForbidden, errorCodeForbiddenPath)

		if strings.Contains(rec.Body.String(), "root:") {
			t.Fatal("response leaked the contents of the traversed file")
		}
	}
}

func TestRunProvenanceRejectsOtherMethods(t *testing.T) {
	t.Parallel()

	handler := NewHandler(mustStore(t, "Provenance"), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAPIRequest(http.MethodPost, "/api/v1/runs/run-1/provenance", nil))

	assertErrorCode(t, rec, http.StatusMethodNotAllowed, errorCodeMethodNotAllowed)
}

func TestOpenAPIDocumentsRunProvenance(t *testing.T) {
	t.Parallel()

	spec := BuildOpenAPISpec("http://127.0.0.1:8080")

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("spec has no paths")
	}

	item, ok := paths["/api/v1/runs/{id}/provenance"].(map[string]any)
	if !ok {
		t.Fatal("spec does not document /api/v1/runs/{id}/provenance")
	}

	get, ok := item["get"].(map[string]any)
	if !ok || get["operationId"] != "getRunProvenance" {
		t.Fatalf("provenance GET is not documented as getRunProvenance: %v", item)
	}

	components, _ := spec["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)

	if _, ok := schemas["RunProvenance"]; !ok {
		t.Fatal("spec has no RunProvenance schema")
	}
}
