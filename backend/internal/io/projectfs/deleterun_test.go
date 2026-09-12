package projectfs

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
)

func mustDeleteRunFixture(t *testing.T, status string) (Store, project.Run) {
	t.Helper()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Delete Run", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	run, _, err := store.CreateRun(CreateRunSpec{
		ScenarioID: "default",
		Standard:   project.StandardRef{ID: "rls19-road", Version: "2019", Profile: "default"},
		Status:     status,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	return store, run
}

// addRunArtifact registers a ref under runID and writes a file at path.
func addRunArtifact(t *testing.T, store Store, runID string, id string, kind string, relPath string) {
	t.Helper()

	abs := filepath.Join(store.Root(), filepath.FromSlash(relPath))

	err := os.MkdirAll(filepath.Dir(abs), 0o750)
	if err != nil {
		t.Fatalf("create artifact directory: %v", err)
	}

	err = os.WriteFile(abs, []byte("{}\n"), 0o600)
	if err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	proj.Artifacts = append(proj.Artifacts, project.ArtifactRef{
		ID:    id,
		RunID: runID,
		Kind:  kind,
		Path:  relPath,
	})

	err = store.Save(proj)
	if err != nil {
		t.Fatalf("save project: %v", err)
	}
}

func TestDeleteRunRemovesRunDirectoryAndRefs(t *testing.T) {
	t.Parallel()

	store, run := mustDeleteRunFixture(t, project.RunStatusCompleted)

	addRunArtifact(t, store, run.ID, "artifact-result", project.ArtifactKindRunResultSummary,
		".noise/runs/"+run.ID+"/results/run-summary.json")

	// An artifact belonging to a different run must survive untouched.
	addRunArtifact(t, store, "run-other", "artifact-other", project.ArtifactKindRunResultSummary,
		".noise/runs/run-other/results/run-summary.json")

	result, err := store.DeleteRun(run.ID)
	if err != nil {
		t.Fatalf("delete run: %v", err)
	}

	if result.RunID != run.ID {
		t.Errorf("unexpected run id in result: %q", result.RunID)
	}

	if !slices.Contains(result.RemovedPaths, ".noise/runs/"+run.ID) {
		t.Errorf("expected the run directory among the removed paths, got %#v", result.RemovedPaths)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if slices.ContainsFunc(proj.Runs, func(r project.Run) bool { return r.ID == run.ID }) {
		t.Error("the run is still listed in the manifest")
	}

	for _, ref := range proj.Artifacts {
		if ref.RunID == run.ID {
			t.Errorf("artifact ref %q survived the delete", ref.ID)
		}
	}

	if !slices.ContainsFunc(proj.Artifacts, func(r project.ArtifactRef) bool { return r.ID == "artifact-other" }) {
		t.Error("another run's artifact ref was dropped too")
	}

	if _, statErr := os.Stat(filepath.Join(store.Root(), ".noise", "runs", run.ID)); !os.IsNotExist(statErr) {
		t.Errorf("expected the run directory to be gone, stat said: %v", statErr)
	}
}

// An export bundle may already have been delivered to a client. Its ref is
// dropped with the run, but its bytes stay, and the result says so.
func TestDeleteRunKeepsExportBundlesOnDisk(t *testing.T) {
	t.Parallel()

	store, run := mustDeleteRunFixture(t, project.RunStatusCompleted)

	bundlePath := ".noise/exports/bundle-1/export-summary.json"
	addRunArtifact(t, store, run.ID, "artifact-export", "export.bundle", bundlePath)

	result, err := store.DeleteRun(run.ID)
	if err != nil {
		t.Fatalf("delete run: %v", err)
	}

	if !slices.Contains(result.RetainedPaths, bundlePath) {
		t.Errorf("expected the bundle among the retained paths, got %#v", result.RetainedPaths)
	}

	if _, statErr := os.Stat(filepath.Join(store.Root(), filepath.FromSlash(bundlePath))); statErr != nil {
		t.Errorf("the export bundle should have been kept: %v", statErr)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if slices.ContainsFunc(proj.Artifacts, func(r project.ArtifactRef) bool { return r.ID == "artifact-export" }) {
		t.Error("the export ref should have been dropped even though the file stays")
	}
}

func TestDeleteRunRefusesARunThatIsStillWriting(t *testing.T) {
	t.Parallel()

	for _, status := range []string{project.RunStatusPending, project.RunStatusRunning} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()

			store, run := mustDeleteRunFixture(t, status)

			_, err := store.DeleteRun(run.ID)
			if !errors.Is(err, ErrRunNotFinished) {
				t.Fatalf("expected ErrRunNotFinished, got %v", err)
			}

			proj, loadErr := store.Load()
			if loadErr != nil {
				t.Fatalf("load project: %v", loadErr)
			}

			if len(proj.Runs) != 1 {
				t.Fatalf("the refusal must change nothing, got %d runs", len(proj.Runs))
			}

			if _, statErr := os.Stat(filepath.Join(store.Root(), ".noise", "runs", run.ID)); statErr != nil {
				t.Errorf("the run directory must still be there: %v", statErr)
			}
		})
	}
}

func TestDeleteRunReportsAnUnknownRun(t *testing.T) {
	t.Parallel()

	store, _ := mustDeleteRunFixture(t, project.RunStatusCompleted)

	_, err := store.DeleteRun("run-does-not-exist")
	if !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("expected ErrRunNotFound, got %v", err)
	}
}

// The identifier arrives from outside — a URL path segment or a CLI flag — so
// it is rejected before it can become a path.
func TestDeleteRunRejectsATraversingIdentifier(t *testing.T) {
	t.Parallel()

	store, _ := mustDeleteRunFixture(t, project.RunStatusCompleted)

	for _, runID := range []string{"", "../../etc", "..", "a/b", "/abs", "-flag", ".hidden"} {
		_, err := store.DeleteRun(runID)

		var appErr *domainerrors.AppError
		if !errors.As(err, &appErr) || appErr.Kind != domainerrors.KindUserInput {
			t.Errorf("run id %q: expected a user-input refusal, got %v", runID, err)
		}
	}
}
