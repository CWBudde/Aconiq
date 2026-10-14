package httpv1

// The runs this server is computing: started under a context the server owns
// rather than the request's, tracked by id, cancellable by id, and closed out
// by id when the process computing one dies without closing it itself.

import (
	"context"
	stderrors "errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

var (
	// errRunCancelRequested is the cause a run's context carries when a client
	// asked for it to stop.
	errRunCancelRequested = stderrors.New("run cancelled through the API")
	// errServerStopping is the cause every run's context carries when the
	// server shuts down with runs still in flight.
	errServerStopping = stderrors.New("the server shut down")
	// errRunNotStarted is the cause of a run the server gave up waiting for:
	// it never appeared in the manifest within runStartTimeout.
	errRunNotStarted = stderrors.New("the run did not register in time")
)

// runSupervisor owns the runs this server has started.
//
// # Why the request does not own them
//
// A run used to be computed inside its POST /runs request, under
// exec.CommandContext(r.Context(), …). So a browser reload cancelled the
// request and the computation with it, the server's write timeout was a
// ceiling on how long a run could take, and a child that died left its row
// reading "running" because the only process that would have closed it was
// the one that died. Here a run's context is detached from the request's
// cancellation: a request starts one and returns, and the run ends when it
// finishes, when a client cancels it by id, or when the server shuts down.
//
// The pointer matters for the reason Handler.manifest's does: mux.HandleFunc
// copies the Handler once per route, and every copy must see one registry.
type runSupervisor struct {
	mu     sync.Mutex
	active map[string]*supervisedRun
	closed bool
	wg     sync.WaitGroup
}

// supervisedRun is one run in flight.
type supervisedRun struct {
	cancel context.CancelCauseFunc
	// done is closed once the run's process has exited and the supervisor has
	// settled its row. err is the executor's answer, readable after done.
	done chan struct{}
	err  error
}

func newRunSupervisor() *runSupervisor {
	return &runSupervisor{active: make(map[string]*supervisedRun)}
}

// start runs execute in a goroutine of its own and returns the handle. When
// execute returns, settle is called with the cause the run's context was
// cancelled with (nil if it was not) and execute's error; only then is the
// run's done channel closed and its id forgotten.
//
// The run's context keeps the values of the request's, ctx, and none of its
// cancellation: context.WithoutCancel is the whole point of this type. What
// ends it instead is a cancel by id, or the supervisor shutting down.
//
// It refuses — false — once the supervisor has been shut down: a run started
// then would outlive the server that is meant to be watching it.
func (s *runSupervisor) start(
	ctx context.Context,
	runID string,
	execute func(ctx context.Context) error,
	settle func(runID string, cause error, err error),
) (*supervisedRun, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, false
	}

	runCtx, cancel := context.WithCancelCause(context.WithoutCancel(ctx))

	run := &supervisedRun{cancel: cancel, done: make(chan struct{})}
	s.active[runID] = run

	s.wg.Go(func() {
		err := execute(runCtx)

		cause := context.Cause(runCtx)

		cancel(nil)

		settle(runID, cause, err)

		run.err = err

		s.mu.Lock()
		delete(s.active, runID)
		s.mu.Unlock()

		close(run.done)
	})

	return run, true
}

// cancel asks one run to stop, and reports whether it is one of this
// server's. The run's process is interrupted, not killed: it stops at its next
// context check and records itself; see newRunProcess for the grace after which
// it is killed anyway.
func (s *runSupervisor) cancel(runID string, cause error) bool {
	s.mu.Lock()
	run, ok := s.active[runID]
	s.mu.Unlock()

	if !ok {
		return false
	}

	run.cancel(cause)

	return true
}

// shutdown interrupts every run in flight and waits for them to settle, until
// ctx ends. It refuses new runs from the moment it is called. The error names
// the runs still in flight when ctx ended; those are reconciled by the next
// `aconiq serve`, whose startup sweep closes any row left open.
func (s *runSupervisor) shutdown(ctx context.Context) error {
	// Under the lock start takes, so a run is either refused or registered —
	// and then interrupted here — never started in between.
	s.mu.Lock()
	s.closed = true

	for _, run := range s.active {
		run.cancel(errServerStopping)
	}
	s.mu.Unlock()

	finished := make(chan struct{})

	go func() {
		s.wg.Wait()
		close(finished)
	}()

	select {
	case <-finished:
		return nil
	case <-ctx.Done():
		s.mu.Lock()
		pending := make([]string, 0, len(s.active))

		for id := range s.active {
			pending = append(pending, id)
		}
		s.mu.Unlock()

		slices.Sort(pending)

		return fmt.Errorf("runs still stopping when the shutdown deadline passed: %v: %w", pending, ctx.Err())
	}
}

// settleRun closes this run's row if the process that computed it exited
// without closing it — killed, crashed, or still busy when its interrupt grace
// ran out — and logs what the executor said, since no response is waiting
// for it any more.
//
// A run that stopped in time has closed its own row, completed or failed, and
// FailOpenRun leaves it exactly as it wrote itself. The server knows the id
// now: it minted it. That replaces the old reconciliation by manifest
// difference, which could not tell two concurrent requests' rows apart and so
// left both open rather than risk failing a live one.
func (h Handler) settleRun(runID string, cause error, err error) {
	if err != nil {
		h.logger.Warn("run process ended with an error", "run_id", runID, "error", err)
	}

	unlock := h.lockManifest()
	defer unlock()

	closed, failErr := h.store.FailOpenRun(runID, h.now, settleNote(cause, err))
	if failErr != nil {
		// Nobody is waiting on a response for this run any more. This is the
		// reason it will keep reading "running" until the server restarts,
		// and the log is the only place left to say so.
		h.logger.Error("run left open: closing it in the manifest failed",
			"run_id", runID, "error", failErr)

		return
	}

	if closed {
		h.logger.Warn("closed a run its process left open", "run_id", runID, "cause", cause)
	}
}

// settleNote is the last line of a run the server had to close itself: what
// a reader of the log needs in order not to mistake it for a refusal.
func settleNote(cause error, err error) string {
	switch {
	case stderrors.Is(cause, errRunCancelRequested):
		return "run cancelled: stopped before it recorded a result"
	case stderrors.Is(cause, errServerStopping):
		return "run interrupted: the server that started it shut down before it recorded a result"
	case stderrors.Is(cause, errRunNotStarted):
		return "run interrupted: it did not register with the server in time"
	case err != nil:
		return "run interrupted: the process computing it exited without recording a result (" + noteReason(err) + ")"
	default:
		return ""
	}
}

// maxNoteReasonLen bounds how much of the executor's error reaches the log
// line. The error carries the child's stderr, and a child that panicked or
// was sent SIGQUIT left a goroutine dump there — tens of kilobytes that belong
// in the server's log, which has the whole error, and not in one run.log line.
const maxNoteReasonLen = 200

// noteReason is the first line of err, cut to maxNoteReasonLen.
func noteReason(err error) string {
	reason, _, _ := strings.Cut(err.Error(), "\n")

	runes := []rune(reason)
	if len(runes) > maxNoteReasonLen {
		return string(runes[:maxNoteReasonLen]) + "…"
	}

	return reason
}

// runStartTimeout bounds how long POST /runs waits for the child to create
// its row before giving up on it. Creating the row is the child's first write
// — standard resolution, a manifest load and a provenance file, all before a
// model is even read — so this is a ceiling on a stall, not on a run.
const runStartTimeout = 30 * time.Second

// runStartPollInterval is how often POST /runs looks for the row while it
// waits. The child creates it within milliseconds of starting, so the answer
// arrives one or two polls in.
const runStartPollInterval = 20 * time.Millisecond
