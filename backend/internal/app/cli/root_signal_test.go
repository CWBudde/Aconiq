//go:build unix

package cli

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"
)

// Not parallel: the signal goes to the whole test process, and a parallel test
// holding its own signal registration would receive it too. Without the
// handler interruptContext installs, the SIGINT would end the test binary —
// so this fails loudly rather than quietly if the registration goes away.
func TestInterruptContextIsCancelledByTheFirstSignal(t *testing.T) {
	ctx, stop := interruptContext(t.Context())
	defer stop()

	err := syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	if err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the context was not cancelled by SIGINT")
	}

	// The run log prints the cause, so it has to name the signal on every Go
	// release the module builds with — not only where NotifyContext sets one.
	cause := context.Cause(ctx)
	if cause == nil || cause.Error() != "interrupt signal received" {
		t.Fatalf("expected the cause to name the signal, got %v", cause)
	}

	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("expected ctx.Err() to stay context.Canceled, got %v", ctx.Err())
	}
}
