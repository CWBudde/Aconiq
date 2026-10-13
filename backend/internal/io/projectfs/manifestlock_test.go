package projectfs

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
)

const (
	manifestLockHelperEnv     = "ACONIQ_PROJECTFS_LOCK_HELPER"
	manifestLockRunsPerWriter = 8
)

// TestHelperProcessWritesManifest is not a test: it is the writer process
// TestConcurrentWritersLoseNoRun starts. It does what an `aconiq run` does to
// the manifest — create a run, then close it — several times over.
func TestHelperProcessWritesManifest(t *testing.T) {
	if os.Getenv(manifestLockHelperEnv) != "1" {
		t.Skip("helper process for TestConcurrentWritersLoseNoRun")
	}

	// The project is the working directory, and the tag the pid: neither comes
	// from the environment, which gosec would trace into every path below.
	store, err := New(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = writeRunsAndCloseThem(store, "p"+strconv.Itoa(os.Getpid()))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(0)
}

// writeRunsAndCloseThem creates manifestLockRunsPerWriter runs under ids that
// carry tag, closing every other one through FailOpenRun, so both the create
// and the close read-modify-write are exercised against the other writers.
func writeRunsAndCloseThem(store Store, tag string) error {
	for i := range manifestLockRunsPerWriter {
		runID := fmt.Sprintf("run-%s-%d", tag, i)

		_, _, err := store.CreateRun(CreateRunSpec{
			RunID:      runID,
			ScenarioID: "default",
			Standard:   project.StandardRef{ID: "rls19-road", Version: "2019", Profile: "default"},
			Status:     project.RunStatusRunning,
		})
		if err != nil {
			return fmt.Errorf("create %s: %w", runID, err)
		}

		if i%2 == 1 {
			_, err = store.FailOpenRun(runID, time.Now, "closed by the lock test")
			if err != nil {
				return fmt.Errorf("close %s: %w", runID, err)
			}
		}
	}

	return nil
}

// Several processes, and several Store values inside this one, create and
// close runs on one project at the same time. Without the cross-process lock
// each one's load-append-save drops the rows another saved in between —
// measured at 19 of 40 runs lost with two `aconiq run` processes — so every
// row has to be there at the end, and every row closed has to read failed.
func TestConcurrentWritersLoseNoRun(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Lock Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	const (
		processes  = 4
		goroutines = 4
	)

	var wg sync.WaitGroup

	errs := make(chan error, processes+goroutines)

	for p := range processes {
		wg.Go(func() {
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcessWritesManifest$")

			cmd.Dir = store.Root()

			cmd.Env = append(os.Environ(), manifestLockHelperEnv+"=1")

			out, err := cmd.CombinedOutput()
			if err != nil {
				errs <- fmt.Errorf("writer process %d: %w\n%s", p, err, out)
			}
		})
	}

	for g := range goroutines {
		wg.Go(func() {
			// A Store value of its own, as a second handler or command in the
			// same process would have.
			own, err := New(store.Root())
			if err != nil {
				errs <- err

				return
			}

			err = writeRunsAndCloseThem(own, "g"+strconv.Itoa(g))
			if err != nil {
				errs <- err
			}
		})
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatal(err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	want := (processes + goroutines) * manifestLockRunsPerWriter
	if len(proj.Runs) != want {
		t.Fatalf("manifest holds %d runs, want %d: a concurrent writer lost rows", len(proj.Runs), want)
	}

	for _, run := range proj.Runs {
		var index int

		_, err := fmt.Sscanf(run.ID[len(run.ID)-1:], "%d", &index)
		if err != nil {
			t.Fatalf("unexpected run id %q", run.ID)
		}

		wantStatus := project.RunStatusRunning
		if index%2 == 1 {
			wantStatus = project.RunStatusFailed
		}

		if run.Status != wantStatus {
			t.Fatalf("run %s = %q, want %q: a close was lost", run.ID, run.Status, wantStatus)
		}
	}
}
