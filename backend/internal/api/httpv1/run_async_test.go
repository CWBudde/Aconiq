package httpv1

// A run outlives the request that starts it.
//
// POST /runs answers 202 once the run's row exists, and the run goes on under
// a context the server owns: a client polls GET /runs/{id} for its progress,
// cancels it by id, and the server closes the row of a child that dies
// without closing it — by id, since it minted the id.
//
// The executor here is a fake that does what the `aconiq run` child does with
// the manifest — creates its row under the id it was given, writes progress —
// and then blocks until it is released or interrupted.

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// newRunTestHandler is newHandlerWithOptions with a run supervisor the test
// holds, so it can wait for runs to settle before it asserts on what they
// wrote. The cleanup interrupts anything still running, so a failing test
// does not leave a goroutine writing into a removed temp dir.
func newRunTestHandler(t *testing.T, store projectfs.Store, opts handlerOptions) (http.Handler, *runSupervisor) {
	t.Helper()

	runs := newRunSupervisor()
	opts.runs = runs

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_ = runs.shutdown(ctx)
	})

	return newHandlerWithOptions(store, opts), runs
}

// startRun appends one running row, the way an `aconiq run` started from a
// terminal does: the server did not start it and does not own it.
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

// fakeChild stands in for the `aconiq run` child.
type fakeChild struct {
	store projectfs.Store
	// progressAt is when the fake's progress report claims to have been
	// written.
	progressAt time.Time
	// release ends a run that is blocked: nil completes it (closing its row
	// as the child would), an error makes it die without closing the row.
	release chan error
	// cause records why the run's context ended, if it did.
	cause chan error
}

func newFakeChild(store projectfs.Store) *fakeChild {
	return &fakeChild{
		store:      store,
		progressAt: time.Date(2026, 10, 13, 12, 0, 30, 0, time.UTC),
		release:    make(chan error, 1),
		cause:      make(chan error, 1),
	}
}

func (f *fakeChild) execute(ctx context.Context, runID string, _ createRunRequest) error {
	_, _, err := f.store.CreateRun(projectfs.CreateRunSpec{
		RunID:      runID,
		ScenarioID: "default",
		Standard:   project.StandardRef{ID: "rls19-road", Version: "2019", Profile: "default"},
		Status:     project.RunStatusRunning,
	})
	if err != nil {
		return err
	}

	started := f.progressAt.Add(-30 * time.Second)

	err = f.store.WriteRunProgress(runID, projectfs.NewRunProgress(3, 10, started, f.progressAt))
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		f.cause <- context.Cause(ctx)

		// Killed before it could record anything: the row stays open for
		// the server to close.
		return stderrors.New("signal: killed")
	case err := <-f.release:
		if err != nil {
			return err
		}

		return closeFakeRun(f.store, runID, project.RunStatusCompleted)
	}
}

// closeFakeRun writes a terminal status the way cli.finalizeRun does.
func closeFakeRun(store projectfs.Store, runID, status string) error {
	proj, err := store.Load()
	if err != nil {
		return err
	}

	for i := range proj.Runs {
		if proj.Runs[i].ID == runID {
			proj.Runs[i].Status = status
		}
	}

	return store.Save(proj)
}

func postRun(t *testing.T, handler http.Handler) (*httptest.ResponseRecorder, runSummaryResponse) {
	t.Helper()

	req := newAPIRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{"standard_id": "rls19-road"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var summary runSummaryResponse
	if rec.Code == http.StatusAccepted {
		decodeResponse(t, rec.Body.Bytes(), &summary)
	}

	return rec, summary
}

func getRun(t *testing.T, handler http.Handler, runID string) runSummaryResponse {
	t.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAPIRequest(http.MethodGet, "/api/v1/runs/"+runID, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET run: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var summary runSummaryResponse

	decodeResponse(t, rec.Body.Bytes(), &summary)

	return summary
}

func cancelRun(handler http.Handler, runID string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAPIRequest(http.MethodPost, "/api/v1/runs/"+runID+"/cancel", nil))

	return rec
}

// The whole contract in one pass: 202 with the run while it computes, the same
// run with its progress from GET /runs/{id}, from the list and from the
// project status the SSE stream sends — and no progress once it has finished.
func TestCreateRunAnswersAcceptedWhileTheRunComputes(t *testing.T) {
	t.Parallel()

	store, _ := runGateFixture(t)
	child := newFakeChild(store)

	handler, runs := newRunTestHandler(t, store, handlerOptions{clock: time.Now, runExecutor: child.execute})

	rec, accepted := postRun(t, handler)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	if accepted.Status != project.RunStatusRunning {
		t.Fatalf("202 status = %q, want %q", accepted.Status, project.RunStatusRunning)
	}

	got := getRun(t, handler, accepted.ID)
	if got.Status != project.RunStatusRunning || got.Progress == nil {
		t.Fatalf("GET while running = %+v, want running with progress", got)
	}

	if got.Progress.Done != 3 || got.Progress.Total != 10 {
		t.Fatalf("progress = %d/%d, want 3/10", got.Progress.Done, got.Progress.Total)
	}

	// 3 receivers in 30 s leaves 7 at 10 s each.
	if got.Progress.ETASeconds == nil || *got.Progress.ETASeconds != 70 {
		t.Fatalf("eta = %v, want 70", got.Progress.ETASeconds)
	}

	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, newAPIRequest(http.MethodGet, "/api/v1/runs", nil))

	var list []runSummaryResponse

	decodeResponse(t, listRec.Body.Bytes(), &list)

	if len(list) != 1 || list[0].Progress == nil || list[0].Progress.Done != 3 {
		t.Fatalf("run list does not carry the progress: %s", listRec.Body.String())
	}

	// The SSE stream's payload is the project status; its dedupe key is the
	// encoded status, so progress in the status is what makes the stream move.
	event, _ := Handler{store: store, now: time.Now}.buildProjectStatusStreamEvent()

	status, ok := event["project"].(projectStatusResponse)
	if !ok || status.LastRun == nil || status.LastRun.Progress == nil || status.LastRun.Progress.Done != 3 {
		t.Fatalf("project status does not carry the last run's progress: %+v", event)
	}

	child.release <- nil

	runs.wg.Wait()

	done := getRun(t, handler, accepted.ID)
	if done.Status != project.RunStatusCompleted || done.Progress != nil {
		t.Fatalf("GET after completion = %+v, want completed with no progress", done)
	}
}

// The run belongs to the server, not to the request that started it: the
// client going away — a browser reload — must not reach the computation.
func TestRunOutlivesTheRequestThatStartedIt(t *testing.T) {
	t.Parallel()

	store, _ := runGateFixture(t)
	child := newFakeChild(store)

	handler, runs := newRunTestHandler(t, store, handlerOptions{clock: time.Now, runExecutor: child.execute})

	ctx, cancel := context.WithCancel(t.Context())

	req := newAPIRequest(http.MethodPost, "/api/v1/runs", strings.NewReader(`{"standard_id": "rls19-road"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var accepted runSummaryResponse

	decodeResponse(t, rec.Body.Bytes(), &accepted)

	cancel()

	select {
	case cause := <-child.cause:
		t.Fatalf("the request's end reached the run: %v", cause)
	case <-time.After(100 * time.Millisecond):
	}

	if got := statusOf(t, store, accepted.ID); got != project.RunStatusRunning {
		t.Fatalf("status = %q after the request ended, want %q", got, project.RunStatusRunning)
	}

	child.release <- nil

	runs.wg.Wait()
}

// Cancel interrupts the run it names. The fake does not stop in time, so the
// server closes the row itself — and says in the log that it was a
// cancellation, not a refusal.
func TestCancelInterruptsTheRunItNames(t *testing.T) {
	t.Parallel()

	store, _ := runGateFixture(t)
	child := newFakeChild(store)

	handler, runs := newRunTestHandler(t, store, handlerOptions{clock: time.Now, runExecutor: child.execute})

	_, accepted := postRun(t, handler)

	rec := cancelRun(handler, accepted.ID)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("cancel: expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	if cause := <-child.cause; !stderrors.Is(cause, errRunCancelRequested) {
		t.Fatalf("run context ended with %v, want the cancel request", cause)
	}

	runs.wg.Wait()

	if got := statusOf(t, store, accepted.ID); got != project.RunStatusFailed {
		t.Fatalf("status after cancel = %q, want %q", got, project.RunStatusFailed)
	}

	logBytes, err := os.ReadFile(filepath.Join(store.Root(), ".noise", "runs", accepted.ID, "run.log"))
	if err != nil {
		t.Fatalf("read run log: %v", err)
	}

	if !strings.Contains(string(logBytes), "run cancelled") {
		t.Fatalf("run log does not say the run was cancelled:\n%s", logBytes)
	}

	// Now finished: a second cancel has nothing to stop.
	rec = cancelRun(handler, accepted.ID)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), errorCodeRunNotRunning) {
		t.Fatalf("cancel of a finished run: expected 409 %s, got %d: %s", errorCodeRunNotRunning, rec.Code, rec.Body.String())
	}
}

// A run this server did not start is refused, not signalled, and an unknown
// one is a 404.
func TestCancelRefusesARunThisServerDidNotStart(t *testing.T) {
	t.Parallel()

	store, _ := runGateFixture(t)
	foreign := startRun(t, store)

	handler, _ := newRunTestHandler(t, store, handlerOptions{clock: time.Now, runExecutor: newFakeChild(store).execute})

	rec := cancelRun(handler, foreign.ID)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), errorCodeRunNotOwned) {
		t.Fatalf("expected 409 %s, got %d: %s", errorCodeRunNotOwned, rec.Code, rec.Body.String())
	}

	if got := statusOf(t, store, foreign.ID); got != project.RunStatusRunning {
		t.Fatalf("a refused cancel touched the run: %q", got)
	}

	rec = cancelRun(handler, "run-does-not-exist")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown run, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, newAPIRequest(http.MethodGet, "/api/v1/runs/"+foreign.ID+"/cancel", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET on cancel, got %d", rec.Code)
	}
}

// A child that dies without closing its row — killed, out of memory — gets
// that row, and only that row, closed by the server. The run started from a
// terminal beside it is still computing and must be left alone; the old
// reconciliation by manifest difference could not tell the two apart.
func TestChildThatDiesLeavesItsOwnRunFailedAndNoOtherRun(t *testing.T) {
	t.Parallel()

	store, _ := runGateFixture(t)
	foreign := startRun(t, store)
	child := newFakeChild(store)

	handler, runs := newRunTestHandler(t, store, handlerOptions{clock: time.Now, runExecutor: child.execute})

	_, accepted := postRun(t, handler)

	// A child that panicked leaves a goroutine dump on stderr, and the
	// executor's error carries it.
	child.release <- stderrors.New("execute run command: panic: boom\n\ngoroutine 1 [running]:\nmain.main()")

	runs.wg.Wait()

	if got := statusOf(t, store, accepted.ID); got != project.RunStatusFailed {
		t.Fatalf("status of the dead child's run = %q, want %q", got, project.RunStatusFailed)
	}

	logBytes, err := os.ReadFile(filepath.Join(store.Root(), ".noise", "runs", accepted.ID, "run.log"))
	if err != nil {
		t.Fatalf("read run log: %v", err)
	}

	if !strings.Contains(string(logBytes), "panic: boom)") || strings.Contains(string(logBytes), "goroutine 1") {
		t.Fatalf("run log should carry the first line of the error and no dump:\n%s", logBytes)
	}

	if got := statusOf(t, store, foreign.ID); got != project.RunStatusRunning {
		t.Fatalf("status of the run beside it = %q, want it untouched (%q)", got, project.RunStatusRunning)
	}
}

// A child that refuses before it creates a row is answered synchronously, in
// its own words, as before: a user-input refusal is a 400 and no run exists.
func TestRefusalBeforeTheRowIsAnsweredSynchronously(t *testing.T) {
	t.Parallel()

	store, _ := runGateFixture(t)

	handler, runs := newRunTestHandler(t, store, handlerOptions{
		clock: time.Now,
		runExecutor: func(context.Context, string, createRunRequest) error {
			return domainerrors.New(domainerrors.KindUserInput, "cli.run", `scenario "nope" does not exist`, nil)
		},
	})

	rec, _ := postRun(t, handler)

	runs.wg.Wait()

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `scenario \"nope\" does not exist`) {
		t.Fatalf("expected 400 with the child's refusal, got %d: %s", rec.Code, rec.Body.String())
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(proj.Runs) != 0 {
		t.Fatalf("a refused run left %d rows", len(proj.Runs))
	}
}

// Shutting down interrupts every run in flight, closes the rows their children
// left open, and refuses new runs from then on.
func TestShutdownInterruptsRunsAndRefusesNewOnes(t *testing.T) {
	t.Parallel()

	store, _ := runGateFixture(t)
	child := newFakeChild(store)

	handler, runs := newRunTestHandler(t, store, handlerOptions{clock: time.Now, runExecutor: child.execute})

	_, accepted := postRun(t, handler)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	err := runs.shutdown(ctx)
	if err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	if cause := <-child.cause; !stderrors.Is(cause, errServerStopping) {
		t.Fatalf("run context ended with %v, want the shutdown", cause)
	}

	if got := statusOf(t, store, accepted.ID); got != project.RunStatusFailed {
		t.Fatalf("status after shutdown = %q, want %q", got, project.RunStatusFailed)
	}

	rec, _ := postRun(t, handler)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), errorCodeServerStopping) {
		t.Fatalf("POST after shutdown: expected 503 %s, got %d: %s", errorCodeServerStopping, rec.Code, rec.Body.String())
	}
}

// A manifest that cannot be read is a storage failure, not a missing run: GET
// and cancel answer 500, and cancel signals nothing, so the caller is not told
// its run does not exist while it goes on computing.
func TestUnreadableManifestIsNotReportedAsAMissingRun(t *testing.T) {
	t.Parallel()

	store, _ := runGateFixture(t)
	child := newFakeChild(store)

	handler, runs := newRunTestHandler(t, store, handlerOptions{clock: time.Now, runExecutor: child.execute})

	_, accepted := postRun(t, handler)

	// A directory where the manifest should be: Load fails with an I/O error,
	// which is neither "not initialised" nor "no such run".
	manifest := filepath.Join(store.Root(), ".noise", "project.json")

	saved, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	err = os.Remove(manifest)
	if err != nil {
		t.Fatalf("remove manifest: %v", err)
	}

	err = os.Mkdir(manifest, 0o750)
	if err != nil {
		t.Fatalf("replace manifest: %v", err)
	}

	rec := cancelRun(handler, accepted.ID)
	assertErrorCode(t, rec, http.StatusInternalServerError, errorCodeInternalError)

	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, newAPIRequest(http.MethodGet, "/api/v1/runs/"+accepted.ID, nil))
	assertErrorCode(t, getRec, http.StatusInternalServerError, errorCodeInternalError)

	select {
	case cause := <-child.cause:
		t.Fatalf("a cancel answered with an error still signalled the run: %v", cause)
	default:
	}

	// Put the manifest back so the run can close itself.
	err = os.Remove(manifest)
	if err != nil {
		t.Fatalf("remove directory: %v", err)
	}

	err = os.WriteFile(manifest, saved, 0o600)
	if err != nil {
		t.Fatalf("restore manifest: %v", err)
	}

	child.release <- nil

	runs.wg.Wait()
}

// A project that was never initialised is answered before any child starts,
// with the 404 not_found envelope every other route gives it. Starting the
// child instead turned its exit code 2 into a 400 carrying stderr.
func TestCreateRunInAnUninitialisedProjectIsNotFound(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	executed := false

	handler, runs := newRunTestHandler(t, store, handlerOptions{
		clock: time.Now,
		runExecutor: func(context.Context, string, createRunRequest) error {
			executed = true

			return nil
		},
	})

	rec, _ := postRun(t, handler)

	runs.wg.Wait()

	assertErrorCode(t, rec, http.StatusNotFound, errorCodeNotFound)

	if executed {
		t.Fatal("a run was started in a project that does not exist")
	}
}

func TestGetRunAnswersNotFoundForAnUnknownRun(t *testing.T) {
	t.Parallel()

	store, _ := runGateFixture(t)
	handler := NewHandler(store, nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAPIRequest(http.MethodGet, "/api/v1/runs/run-nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
