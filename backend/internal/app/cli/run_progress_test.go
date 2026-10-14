package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// progressSnapshot is what was on disk at the moment a progress report was
// written: the run is still computing, so this is what a reader polling the
// run would have seen.
type progressSnapshot struct {
	done, total int
	log         string
	progress    projectfs.RunProgress
}

// A run that is computing must be readable from outside the process: run.log
// carries every line logged so far and a "computed X of Y" line, and
// progress.json says the same in numbers. Before this, run.log held one line
// until finalizeRun wrote the whole log at the end, so an API client polling
// a long run saw nothing move.
//
// The observer fires inside the run, after each report has been written and
// before the run finishes — so everything it reads is a mid-run view, which is
// the property under test.
func TestRunWritesItsLogAndProgressWhileItComputes(t *testing.T) {
	t.Parallel()

	type progressCase struct {
		standard string
		fixture  []string
		extra    []string
		// logged is a line the module writes before its compute, which a
		// mid-run log must already hold.
		logged string
	}

	const scaffolds = 8

	cases := make([]progressCase, 0, 4+scaffolds)
	cases = append(cases, []progressCase{
		{"rls19-road", registryRunFixtures["rls19-road"], nil, "receivers="},
		{"iso9613", registryRunFixtures["iso9613"], nil, "receivers="},
		// Schall 03 used to buffer its log lines and append them after the
		// compute, so a progress line would have landed ahead of the lines
		// that preceded it; it logs through the run's log now, and the
		// "receivers=" check below is what holds it to that.
		{"schall03", registryRunFixtures["schall03"], nil, "receivers="},
		// BEB counts the facade receivers it computes levels for, which only
		// the package knows; its total arrives with the first report. It logs
		// no receiver count of its own before the compute.
		{"beb-exposure", registryRunFixtures["beb-exposure"], []string{"--experimental"}, "evidence_tier="},
	}...)

	// The scaffolds share one walk, but each reaches it through its own
	// table entry, and an entry left on the progress-less call is exactly the
	// regression this is here to catch.
	for _, standard := range []string{
		"cnossos-road", "cnossos-rail", "cnossos-industry", "cnossos-aircraft",
		"bub-road", "bub-rail", "bub-industry", "buf-aircraft",
	} {
		cases = append(cases, progressCase{standard, registryRunFixtures[standard], []string{"--experimental"}, "receivers="})
	}

	for _, tc := range cases {
		t.Run(tc.standard, func(t *testing.T) {
			t.Parallel()

			projectDir := t.TempDir()

			mustRunCLI(t, "--project", projectDir, "init", "--name", "Progress", "--crs", "EPSG:25832")
			mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, tc.fixture...))

			const runID = "run-progress-under-test"

			runDir := filepath.Join(projectDir, ".noise", "runs", runID)

			var (
				mu        sync.Mutex
				snapshots []progressSnapshot
			)

			ctx := withProgressObserver(t.Context(), func(done, total int) {
				logBytes, err := os.ReadFile(filepath.Join(runDir, "run.log"))
				if err != nil {
					t.Errorf("read run.log mid-run: %v", err)
				}

				progressBytes, err := os.ReadFile(filepath.Join(runDir, "progress.json"))
				if err != nil {
					t.Errorf("read progress.json mid-run: %v", err)
				}

				var progress projectfs.RunProgress

				err = json.Unmarshal(progressBytes, &progress)
				if err != nil {
					t.Errorf("decode progress.json mid-run: %v", err)
				}

				mu.Lock()
				defer mu.Unlock()

				snapshots = append(snapshots, progressSnapshot{done: done, total: total, log: string(logBytes), progress: progress})
			})

			args := append([]string{"--project", projectDir, "run", "--standard", tc.standard, "--run-id", runID, "--workers", "2"}, tc.extra...)

			err := runCLIContext(ctx, args...)
			if err != nil {
				t.Fatalf("run: %v", err)
			}

			if len(snapshots) < 2 {
				t.Fatalf("expected at least the first and the last report, got %d", len(snapshots))
			}

			first, last := snapshots[0], snapshots[len(snapshots)-1]
			if first.done != 0 || last.done != last.total || last.total == 0 {
				t.Fatalf("expected reports from 0 to total, got first=%d/%d last=%d/%d", first.done, first.total, last.done, last.total)
			}

			for _, snap := range snapshots {
				line := fmt.Sprintf("computed %d of %d receivers", snap.done, snap.total)
				if !strings.Contains(snap.log, line) {
					t.Fatalf("run.log mid-run lacks %q:\n%s", line, snap.log)
				}

				// The lines logged before the compute are on disk too, not
				// only the progress line: the log streams, it does not just
				// grow a feed at the end.
				if !strings.Contains(snap.log, tc.logged) || !strings.Contains(snap.log, "evidence_tier=") {
					t.Fatalf("run.log mid-run lacks the lines logged before the compute:\n%s", snap.log)
				}

				if strings.Contains(snap.log, "run completed") {
					t.Fatalf("observer ran after the run completed, so it proves nothing:\n%s", snap.log)
				}

				if snap.progress.Done != snap.done || snap.progress.Total != snap.total {
					t.Fatalf("progress.json = %d/%d, want %d/%d", snap.progress.Done, snap.progress.Total, snap.done, snap.total)
				}
			}

			if last.progress.ETASeconds == nil || *last.progress.ETASeconds != 0 {
				t.Fatalf("a finished walk must estimate zero seconds left, got %v", last.progress.ETASeconds)
			}

			// The finished log is the same lines the stream wrote, plus the
			// closing ones: finalizeRun replaces the file with the in-memory
			// record, which the streamed copy must be a prefix of.
			final, err := os.ReadFile(filepath.Join(runDir, "run.log"))
			if err != nil {
				t.Fatalf("read final run.log: %v", err)
			}

			if !strings.HasPrefix(string(final), last.log) {
				t.Fatalf("the final log does not continue the streamed one:\nstreamed:\n%s\nfinal:\n%s", last.log, final)
			}
		})
	}
}

// --run-id names the run, and a name already taken is refused as user input
// before anything is written: two rows under one id would make every
// run-scoped endpoint answer about whichever it found first.
func TestRunIDFlagNamesTheRunAndRefusesADuplicate(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "RunID", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, registryRunFixtures["dummy-freefield"]...))
	mustRunCLI(t, "--project", projectDir, "run", "--run-id", "run-chosen-by-caller")

	if got := onlyRun(t, projectDir).ID; got != "run-chosen-by-caller" {
		t.Fatalf("run id = %q, want the one passed on the command line", got)
	}

	for _, id := range []string{"run-chosen-by-caller", "../escape", "-flag"} {
		err := runCLI("--project", projectDir, "run", "--run-id", id)

		var appErr *domainerrors.AppError
		if !errors.As(err, &appErr) || appErr.Kind != domainerrors.KindUserInput {
			t.Fatalf("--run-id %q: expected a user-input refusal, got %v", id, err)
		}
	}

	// Still one run: every refusal happened before CreateRun wrote a row.
	onlyRun(t, projectDir)
}

// The throttle: the first and the last report are always written, and in
// between at most one per runProgressInterval.
func TestRunProgressThrottlesBetweenTheFirstAndTheLastReport(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	const runID = "run-throttle"

	err = os.MkdirAll(filepath.Join(store.Root(), ".noise", "runs", runID), 0o750)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	clock := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

	var written []int

	log := newRunLog()
	progress := newRunProgress(withProgressObserver(context.Background(), func(done, _ int) {
		written = append(written, done)
	}), store, runID, log)
	progress.now = func() time.Time { return clock }

	progress.start(10)

	for done := 1; done <= 10; done++ {
		clock = clock.Add(400 * time.Millisecond)

		progress.report(done, 10)
	}

	// 0 at start; then 400 ms per receiver against a one-second interval
	// writes every third (1.2 s), and 10 is written because it is the last.
	want := []int{0, 3, 6, 9, 10}
	if fmt.Sprint(written) != fmt.Sprint(want) {
		t.Fatalf("written reports = %v, want %v", written, want)
	}

	if got := len(log.all()); got != len(want) {
		t.Fatalf("log holds %d progress lines, want %d", got, len(want))
	}

	stored, ok, err := store.ReadRunProgress(runID)
	if err != nil || !ok {
		t.Fatalf("read progress: ok=%v err=%v", ok, err)
	}

	if stored.Done != 10 || stored.Total != 10 || !stored.UpdatedAt.Equal(clock) {
		t.Fatalf("stored progress = %+v", stored)
	}
}
