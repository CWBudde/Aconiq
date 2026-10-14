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
// The terminal status is written by the `aconiq run` process, at the end of
// its own work. Nothing else writes it while that process lives. So one that
// dies before that point — killed, out of memory, or still busy when the
// server that started it gave up waiting — leaves a row saying "running" that
// nothing will ever come back to.
//
// The UI then reads that row and is right to draw a spinner: the manifest is
// the only thing it can ask, and the manifest says the run is in progress.
// Two dead runs spinning forever is what this function exists to prevent, and
// the fix belongs on the server rather than in the UI, because "is this pid
// alive" is not a question an HTTP client can ask.
//
// # Why "every open run" is the right set at startup
//
// `aconiq serve` supervises every run it starts and closes the row of any
// child that dies without closing it — FailOpenRun, by id. So at the moment a
// server starts there is no run of its own in flight, and any row still open
// belongs to a previous process. The one case this over-reaches is a bare
// `aconiq run` invoked from a terminal against the same project while the
// server restarts; that run keeps computing and writes its own terminal
// status when it finishes, so the cost is a row that reads "failed" for a few
// minutes and then corrects itself.
func (s Store) FailInterruptedRuns(now func() time.Time, keep func(project.Run) bool) (int, error) {
	return s.failOpenRuns(now, keep, interruptedRunNote)
}

// FailOpenRun closes one run that is still recorded as running or pending,
// with note as the last line of its log, and reports whether there was such a
// row to close.
//
// It is the supervisor's half of FailInterruptedRuns: a server that started
// the run knows its id, and so closes exactly that one rather than sweeping.
// A run that already closed itself — which is what a run does when it is
// interrupted and stops in time — is left exactly as it wrote itself.
func (s Store) FailOpenRun(runID string, now func() time.Time, note string) (bool, error) {
	err := validateRunID("projectfs.FailOpenRun", runID)
	if err != nil {
		return false, err
	}

	if note == "" {
		note = interruptedRunNote
	}

	closed, err := s.failOpenRuns(now, func(run project.Run) bool { return run.ID != runID }, note)

	return closed > 0, err
}

// failOpenRuns closes the open runs keep does not keep.
//
// It looks before it locks. The startup sweep runs on every `aconiq serve`,
// and on almost every one there is nothing to close — so taking the lock
// first, which creates `.noise/project.json.lock`, would refuse to serve a
// project on a read-only mount (an archived study, a shared drive) for a write
// it was never going to make. Only when an open run is found is the lock taken
// and the manifest re-read, because another writer may have closed it in the
// meantime and the decision has to be made on the copy that is saved.
func (s Store) failOpenRuns(now func() time.Time, keep func(project.Run) bool, note string) (int, error) {
	proj, err := s.Load()
	if err != nil {
		return 0, fmt.Errorf("load project: %w", err)
	}

	if len(openRunsToClose(proj, keep)) == 0 {
		return 0, nil
	}

	unlock, err := s.LockManifest()
	if err != nil {
		return 0, fmt.Errorf("lock project: %w", err)
	}
	defer unlock()

	proj, err = s.Load()
	if err != nil {
		return 0, fmt.Errorf("load project: %w", err)
	}

	toClose := openRunsToClose(proj, keep)
	if len(toClose) == 0 {
		return 0, nil
	}

	at := now().UTC()

	for _, i := range toClose {
		proj.Runs[i].Status = project.RunStatusFailed
		proj.Runs[i].FinishedAt = at
	}

	err = s.Save(proj)
	if err != nil {
		return 0, fmt.Errorf("save project: %w", err)
	}

	// The status alone says "failed", which reads like the computation
	// refused something. Say what actually happened, in the file the UI
	// already shows for a run. A log that cannot be appended to is not worth
	// failing the reconciliation over — the status is the part that stops the
	// spinner.
	for _, i := range toClose {
		_ = s.appendRunLogNote(proj.Runs[i].ID, at.Format(time.RFC3339)+" "+note)
	}

	return len(toClose), nil
}

// openRunsToClose is the indexes of the runs still running or pending that
// keep does not keep.
func openRunsToClose(proj project.Project, keep func(project.Run) bool) []int {
	var indexes []int

	for i, run := range proj.Runs {
		if run.Status != project.RunStatusRunning && run.Status != project.RunStatusPending {
			continue
		}

		if keep != nil && keep(run) {
			continue
		}

		indexes = append(indexes, i)
	}

	return indexes
}

// RunIDs is the set of run ids a manifest snapshot held.
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
