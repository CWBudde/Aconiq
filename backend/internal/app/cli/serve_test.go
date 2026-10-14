package cli

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"testing"
)

// recordingStopper stands in for the serve handler's run supervisor.
type recordingStopper struct {
	stopped int
}

func (r *recordingStopper) StopRuns(context.Context) error {
	r.stopped++

	return nil
}

func discardState() commandState {
	return commandState{Logger: slog.New(slog.DiscardHandler)}
}

// A listener that dies under a running server must not leave its runs behind:
// they are children no request owns, and the process is about to return.
// The runs are stopped, and the listener's error is still what is reported.
func TestListenerErrorStopsTheRunsAndKeepsTheError(t *testing.T) {
	t.Parallel()

	listenErr := errors.New("accept tcp 127.0.0.1:8080: too many open files")
	runs := &recordingStopper{}

	err := stopAfterListenerError(&http.Server{}, runs, listenErr, "127.0.0.1:8080", discardState())

	if runs.stopped != 1 {
		t.Fatalf("runs stopped %d times, want once", runs.stopped)
	}

	if !errors.Is(err, listenErr) {
		t.Fatalf("expected the listener's error to be reported, got %v", err)
	}
}

// A server closed on purpose reports nothing, but its runs are still stopped.
func TestClosedServerStopsTheRunsAndReportsNothing(t *testing.T) {
	t.Parallel()

	runs := &recordingStopper{}

	err := stopAfterListenerError(&http.Server{}, runs, http.ErrServerClosed, "127.0.0.1:8080", discardState())
	if err != nil {
		t.Fatalf("expected no error for a closed server, got %v", err)
	}

	if runs.stopped != 1 {
		t.Fatalf("runs stopped %d times, want once", runs.stopped)
	}
}

// End to end through runServe: a port that is already taken fails the
// listener, and the command reports that failure rather than hanging or
// swallowing it.
func TestServeReportsAListenerThatCannotBind(t *testing.T) {
	t.Parallel()

	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	defer func() { _ = taken.Close() }()

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Serve", "--crs", "EPSG:25832")

	err = runCLI("--project", projectDir, "serve", "--listen", taken.Addr().String())
	if err == nil {
		t.Fatal("expected serve to fail on a port that is taken")
	}
}
