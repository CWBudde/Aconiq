package projectfs

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
)

// The startup sweep runs on every `aconiq serve`. On a project with nothing to
// close it must not write anything — not even the lock file — or a project on
// a read-only mount cannot be served at all.
func TestSweepWithNothingToCloseWritesNothing(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits do not make a folder read-only on Windows")
	}

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Read-only", "EPSG:25832")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	control := filepath.Join(store.Root(), ".noise")

	err = os.Chmod(control, 0o500)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(control, 0o750) })

	// Root ignores permission bits; the test would prove nothing there.
	probe := filepath.Join(control, "probe")
	if f, createErr := os.Create(probe); createErr == nil {
		_ = f.Close()
		_ = os.Remove(probe)

		t.Skip("running with privileges that ignore a read-only directory")
	}

	closed, err := store.FailInterruptedRuns(time.Now, nil)
	if err != nil {
		t.Fatalf("sweep of a read-only project with nothing open: %v", err)
	}

	if closed != 0 {
		t.Fatalf("closed %d runs, want 0", closed)
	}

	_, err = os.Stat(filepath.Join(control, manifestLockFileName))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the sweep created the lock file (or failed to look for it): %v", err)
	}
}

// When there is something to close, the sweep still closes it — the look
// before the lock is an optimisation for the empty case, not a change in what
// it does.
func TestSweepClosesWhatIsOpen(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Sweep", "EPSG:25832")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	run, _, err := store.CreateRun(CreateRunSpec{
		ScenarioID: "default",
		Standard:   project.StandardRef{ID: "rls19-road", Version: "2019", Profile: "default"},
		Status:     project.RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	closed, err := store.FailInterruptedRuns(time.Now, nil)
	if err != nil || closed != 1 {
		t.Fatalf("sweep: closed=%d err=%v, want 1 and nil", closed, err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if proj.Runs[0].ID != run.ID || proj.Runs[0].Status != project.RunStatusFailed {
		t.Fatalf("run after the sweep: %+v", proj.Runs[0])
	}
}
