package httpv1

// What POST /api/v1/runs does with the row its subprocess left open.
//
// The subprocess writes the terminal status and nothing else does, so one that
// dies leaves a row reading "running" forever. The handler closes it by
// difference against the manifest it found — and the case worth guarding is
// the one where that difference names more than one row, because a concurrent
// request's run is still computing and must not be marked failed.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// startRun appends one running row, the way the `aconiq run` child does at the
// start of its own work.
func startRun(t *testing.T, store projectfs.Store) project.Run {
	t.Helper()

	run, _, err := store.CreateRun(projectfs.CreateRunSpec{
		ScenarioID: "default",
		Standard: project.StandardRef{
			ID:      "rls19-road",
			Version: "2019",
			Profile: "default",
		},
		Status: project.RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	return run
}

// statusOf reads a run's status back out of the manifest.
func statusOf(t *testing.T, store projectfs.Store, runID string) string {
	t.Helper()

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	for _, run := range proj.Runs {
		if run.ID == runID {
			return run.Status
		}
	}

	t.Fatalf("run %s is not in the manifest", runID)

	return ""
}

// postFailingRun drives one POST /api/v1/runs whose executor does `body` and
// then fails, the way a killed subprocess does.
func postFailingRun(t *testing.T, store projectfs.Store, body func()) {
	t.Helper()

	handler := newHandlerWithOptions(store, handlerOptions{
		clock: time.Now,
		runExecutor: func(_ context.Context, _ createRunRequest) error {
			body()

			return context.Canceled
		},
	})

	req := newAPIRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{"standard_id": "rls19-road"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code < 400 {
		t.Fatalf("expected the failed executor to be reported, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestFailedRunRequestClosesTheRunItStarted(t *testing.T) {
	t.Parallel()

	store, _ := runGateFixture(t)

	var started project.Run

	postFailingRun(t, store, func() { started = startRun(t, store) })

	if got := statusOf(t, store, started.ID); got != project.RunStatusFailed {
		t.Fatalf("status of the interrupted run = %q, want %q", got, project.RunStatusFailed)
	}
}

// The row was already open when the request arrived, so it belongs to an
// earlier process; `aconiq serve` reconciles that one at startup, not this
// handler.
func TestFailedRunRequestLeavesAnAlreadyOpenRunAlone(t *testing.T) {
	t.Parallel()

	store, _ := runGateFixture(t)

	earlier := startRun(t, store)

	var started project.Run

	postFailingRun(t, store, func() { started = startRun(t, store) })

	if got := statusOf(t, store, earlier.ID); got != project.RunStatusRunning {
		t.Fatalf("status of the pre-existing run = %q, want it untouched (%q)", got, project.RunStatusRunning)
	}

	if got := statusOf(t, store, started.ID); got != project.RunStatusFailed {
		t.Fatalf("status of the interrupted run = %q, want %q", got, project.RunStatusFailed)
	}
}

// Two overlapping POSTs both snapshot a manifest holding neither run, so each
// one's difference names both rows. Closing them would mark a run that is
// still computing as failed, which costs a user the work; leaving them open
// costs a spinner until the next `aconiq serve`. The handler must take the
// second.
func TestFailedRunRequestLeavesAConcurrentRunAlone(t *testing.T) {
	t.Parallel()

	store, _ := runGateFixture(t)

	var mine, concurrent project.Run

	postFailingRun(t, store, func() {
		mine = startRun(t, store)
		concurrent = startRun(t, store)
	})

	for _, run := range []project.Run{mine, concurrent} {
		if got := statusOf(t, store, run.ID); got != project.RunStatusRunning {
			t.Fatalf("run %s = %q; an ambiguous difference must close nothing (%q)", run.ID, got, project.RunStatusRunning)
		}
	}
}
