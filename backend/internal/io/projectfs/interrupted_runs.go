package projectfs

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
)

// interruptedRunNote is appended to the log of a run this package closes out,
// so that the reason is where a reader looks for it rather than only in the
// status field.
const interruptedRunNote = "run interrupted: the process computing it exited without recording a result"

// FailInterruptedRuns closes out every run the manifest still records as
// running or pending, and reports how many it closed.
//
// # Why a run can be left open at all
//
// The terminal status is written by the `aconiq run` subprocess, at the end of
// its own work. Nothing else writes it. So a subprocess that dies before that
// point — killed, out of memory, or cancelled with its HTTP request, which is
// what a browser reload during a run does — leaves a row saying "running"
// that nothing will ever come back to.
//
// The UI then reads that row and is right to draw a spinner: the manifest is
// the only thing it can ask, and the manifest says the run is in progress.
// Two dead runs spinning forever is what this function exists to prevent, and
// the fix belongs on the server rather than in the UI, because "is this pid
// alive" is not a question an HTTP client can ask.
//
// # Why "every open run" is the right set
//
// A run only ever executes inside a live handleRunCreate request, which holds
// the request open for the whole computation. So at the moment the server
// starts there is no run of its own in flight, and any row still open belongs
// to a previous process. The one case this over-reaches is a bare `aconiq run`
// invoked from a terminal against the same project while the server restarts;
// that run keeps computing and writes its own terminal status when it
// finishes, so the cost is a row that reads "failed" for a few minutes and
// then corrects itself.
func (s Store) FailInterruptedRuns(now func() time.Time, keep func(project.Run) bool) (int, error) {
	proj, err := s.Load()
	if err != nil {
		return 0, fmt.Errorf("load project: %w", err)
	}

	closed := 0

	for i := range proj.Runs {
		run := proj.Runs[i]

		if run.Status != project.RunStatusRunning && run.Status != project.RunStatusPending {
			continue
		}

		if keep != nil && keep(run) {
			continue
		}

		proj.Runs[i].Status = project.RunStatusFailed
		proj.Runs[i].FinishedAt = now().UTC()
		closed++

		// The status alone says "failed", which reads like the computation
		// refused something. Say what actually happened, in the file the UI
		// already shows for a run. A log that cannot be appended to is not
		// worth failing the reconciliation over — the status is the part that
		// stops the spinner.
		_ = s.appendRunLogNote(run.ID, interruptedRunNote)
	}

	if closed == 0 {
		return 0, nil
	}

	err = s.Save(proj)
	if err != nil {
		return 0, fmt.Errorf("save project: %w", err)
	}

	return closed, nil
}

// RunIDs is the set of run ids a manifest snapshot held, so that a caller can
// tell the rows it is responsible for from the rows that were already there.
func RunIDs(proj project.Project) map[string]struct{} {
	ids := make(map[string]struct{}, len(proj.Runs))
	for _, run := range proj.Runs {
		ids[run.ID] = struct{}{}
	}

	return ids
}

// appendRunLogNote adds one line to a run's log, creating it if the run died
// before anything was written.
func (s Store) appendRunLogNote(runID, note string) error {
	path := filepath.Join(s.runsDir(), runID, "run.log")

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open run log: %w", err)
	}

	defer func() { _ = file.Close() }()

	_, err = fmt.Fprintf(file, "%s\n", note)
	if err != nil {
		return fmt.Errorf("append to run log: %w", err)
	}

	return nil
}
