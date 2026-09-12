package cli

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// `aconiq delete-run` is the twin of DELETE /api/v1/runs/{id}. Both go through
// projectfs.DeleteRun, so this asserts the wiring — the flag, the refusals and
// the exit classification — rather than restating the store's own behaviour.
func TestDeleteRunCommandRemovesTheRun(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "Delete Run CLI", "--crs", "EPSG:25832")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	run, _, err := store.CreateRun(projectfs.CreateRunSpec{
		ScenarioID: "default",
		Standard:   project.StandardRef{ID: "rls19-road", Version: "2019", Profile: "default"},
		Status:     project.RunStatusCompleted,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "delete-run", "--run", run.ID)

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if slices.ContainsFunc(proj.Runs, func(r project.Run) bool { return r.ID == run.ID }) {
		t.Error("the run is still listed in the manifest")
	}

	if _, statErr := os.Stat(filepath.Join(projectDir, ".noise", "runs", run.ID)); !os.IsNotExist(statErr) {
		t.Errorf("expected the run directory to be gone, stat said: %v", statErr)
	}
}

func TestDeleteRunCommandRefusesAnUnfinishedRun(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "Delete Run CLI", "--crs", "EPSG:25832")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	run, _, err := store.CreateRun(projectfs.CreateRunSpec{
		ScenarioID: "default",
		Standard:   project.StandardRef{ID: "rls19-road", Version: "2019", Profile: "default"},
		Status:     project.RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	err = runCLI("--project", projectDir, "delete-run", "--run", run.ID)
	if !errors.Is(err, projectfs.ErrRunNotFinished) {
		t.Fatalf("expected ErrRunNotFinished, got %v", err)
	}
}

func TestDeleteRunCommandRequiresTheRunFlag(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "Delete Run CLI", "--crs", "EPSG:25832")

	err := runCLI("--project", projectDir, "delete-run")
	if err == nil {
		t.Fatal("expected delete-run without --run to fail")
	}
}
