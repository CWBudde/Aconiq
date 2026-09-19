package httpv1

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
)

// TestConcurrentManifestWritersDoNotDropEachOthersChange drives the lost update
// three routes could produce.
//
// POST /api/v1/model, POST /api/v1/import/terrain and DELETE /api/v1/runs/{id}
// each Load the manifest, mutate the value and Save it back, with nothing
// between them. Two requests in flight therefore read the same snapshot and
// the second Save discards the first one's change:
//
//	model:  Load (run present) ....................... Save (run present again)
//	delete: .............. Load, remove run, Save
//
// and the deleted run is back, with a 204 already sent for it.
//
// The pairing matters. Both halves have to be observable afterwards, so the
// test deletes a different run each round and checks the run is gone while the
// model save also landed.
func TestConcurrentManifestWritersDoNotDropEachOthersChange(t *testing.T) {
	t.Parallel()

	const rounds = 40

	store := mustStore(t, "Manifest Concurrency")
	handler := NewHandler(store, nil)

	runIDs := make([]string, 0, rounds)
	for range rounds {
		runIDs = append(runIDs, mustCreateCompletedRun(t, store).ID)
	}

	for round, runID := range runIDs {
		var (
			wg         sync.WaitGroup
			modelRec   *httptest.ResponseRecorder
			deleteCode int
		)

		wg.Go(func() {
			modelRec = postModel(t, handler, `{"model": `+validModelFeatureCollection+`}`)
		})

		wg.Go(func() {
			req := newAPIRequest(http.MethodDelete, "/api/v1/runs/"+runID, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			deleteCode = rec.Code
		})

		wg.Wait()

		if modelRec.Code != http.StatusCreated {
			t.Fatalf("round %d: model save returned %d: %s", round, modelRec.Code, modelRec.Body.String())
		}

		if deleteCode != http.StatusNoContent && deleteCode != http.StatusOK {
			t.Fatalf("round %d: delete run returned %d", round, deleteCode)
		}

		proj, err := store.Load()
		if err != nil {
			t.Fatalf("round %d: load manifest: %v", round, err)
		}

		for _, run := range proj.Runs {
			if run.ID == runID {
				t.Fatalf("round %d: run %s was deleted with %d and then written back by the concurrent model save", round, runID, deleteCode)
			}
		}

		if !hasArtifact(proj.Artifacts, project.ArtifactIDModelNormalized) {
			t.Fatalf("round %d: the model save's artifact ref was dropped by the concurrent delete", round)
		}
	}
}

// TestConcurrentTerrainImportsKeepTheManifestReadable pairs the third writer
// with the one that does not save in process, so the manifest is being read by
// POST /runs' two Loads while it is being replaced.
func TestConcurrentTerrainImportsKeepTheManifestReadable(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Terrain Concurrency")
	handler := NewHandler(store, nil)

	var wg sync.WaitGroup

	for range 8 {
		wg.Go(func() {
			for range 5 {
				req := newAPIRequest(http.MethodPost, "/api/v1/model", strings.NewReader(`{"model": `+validModelFeatureCollection+`}`))
				req.Header.Set("Content-Type", "application/json")

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusCreated {
					t.Errorf("model save returned %d: %s", rec.Code, rec.Body.String())

					return
				}
			}
		})
	}

	wg.Wait()

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("manifest unreadable after concurrent saves: %v", err)
	}

	if !hasArtifact(proj.Artifacts, project.ArtifactIDModelNormalized) {
		t.Fatal("model artifact missing after concurrent saves")
	}
}

func hasArtifact(artifacts []project.ArtifactRef, id string) bool {
	for _, artifact := range artifacts {
		if artifact.ID == id {
			return true
		}
	}

	return false
}
