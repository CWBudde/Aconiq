//go:build unix

package httpv1

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// helperTrapExitCode is what the helper process exits with once it has caught
// SIGINT. A child that was killed instead never reaches it.
const helperTrapExitCode = 3

// TestHelperProcessTrapsInterrupt is not a test: it is the child that
// TestCancelledRunProcessIsInterruptedNotKilled starts, standing in for an
// `aconiq run` that finalizes its run when interrupted.
func TestHelperProcessTrapsInterrupt(t *testing.T) {
	if os.Getenv("ACONIQ_HTTPV1_HELPER_PROCESS") != "1" {
		t.Skip("helper process for TestCancelledRunProcessIsInterruptedNotKilled")
	}

	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)

	_, _ = os.Stdout.WriteString("ready\n")

	select {
	case <-interrupted:
		os.Exit(helperTrapExitCode)
	case <-time.After(30 * time.Second):
		os.Exit(1)
	}
}

// A run request whose HTTP context ends must interrupt its child, which then
// records its own run as failed. Killing it — what exec.CommandContext does by
// default — leaves the run reading "running" in the manifest.
func TestCancelledRunProcessIsInterruptedNotKilled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cmd := newRunProcess(ctx, os.Args[0], []string{"-test.run=^TestHelperProcessTrapsInterrupt$"})

	cmd.Env = append(os.Environ(), "ACONIQ_HTTPV1_HELPER_PROCESS=1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	err = cmd.Start()
	if err != nil {
		t.Fatalf("start helper: %v", err)
	}

	// Cancel only once the helper has installed its handler; before that, a
	// SIGINT would end it the default way and prove nothing.
	_, err = bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("wait for helper: %v", err)
	}

	cancel()

	err = cmd.Wait()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected the helper to exit with a status, got %v", err)
	}

	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		t.Fatalf("expected the helper to be interrupted, but it was killed by %v", status.Signal())
	}

	if exitErr.ExitCode() != helperTrapExitCode {
		t.Fatalf("expected exit code %d from the trapped interrupt, got %d", helperTrapExitCode, exitErr.ExitCode())
	}
}
