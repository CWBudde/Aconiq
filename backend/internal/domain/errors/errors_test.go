package errors_test

import (
	stderrors "errors"
	"fmt"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
)

// errSentinel is a plain error used as a wrapped cause.
var errSentinel = stderrors.New("boom")

func TestErrorMessageFormats(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  *domainerrors.AppError
		want string
	}{
		{
			name: "op and message only",
			err:  domainerrors.New(domainerrors.KindUserInput, "cli.import", "missing --from", nil),
			want: "cli.import: missing --from",
		},
		{
			name: "op, message and cause",
			err:  domainerrors.New(domainerrors.KindInternal, "io.projectfs", "write manifest", errSentinel),
			want: "io.projectfs: write manifest: boom",
		},
		{
			name: "cause without message",
			err:  domainerrors.New(domainerrors.KindInternal, "io.projectfs", "", errSentinel),
			want: "io.projectfs: boom",
		},
		{
			name: "empty message and no cause",
			err:  domainerrors.New(domainerrors.KindValidation, "geo.validate", "", nil),
			want: "geo.validate: ",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNilReceiverIsSafe(t *testing.T) {
	t.Parallel()

	var err *domainerrors.AppError

	if got := err.Error(); got != "<nil>" {
		t.Fatalf("nil Error() = %q, want %q", got, "<nil>")
	}

	if got := err.Unwrap(); got != nil {
		t.Fatalf("nil Unwrap() = %v, want nil", got)
	}
}

func TestNewRetainsFields(t *testing.T) {
	t.Parallel()

	err := domainerrors.New(domainerrors.KindNotFound, "cli.status", "no run", errSentinel)

	if err.Kind != domainerrors.KindNotFound {
		t.Fatalf("Kind = %q, want %q", err.Kind, domainerrors.KindNotFound)
	}

	if err.Op != "cli.status" {
		t.Fatalf("Op = %q, want cli.status", err.Op)
	}

	if err.Msg != "no run" {
		t.Fatalf("Msg = %q, want %q", err.Msg, "no run")
	}

	if !stderrors.Is(err, errSentinel) {
		t.Fatal("expected errors.Is to reach the wrapped cause")
	}
}

// TestIsUserErrorClassifiesKinds pins the classification that decides the CLI
// exit code: cli.Execute returns 2 for a user error and 1 otherwise (see
// internal/app/cli/root.go).
func TestIsUserErrorClassifiesKinds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		kind domainerrors.Kind
		want bool
	}{
		{kind: domainerrors.KindUserInput, want: true},
		{kind: domainerrors.KindValidation, want: true},
		{kind: domainerrors.KindNotFound, want: true},
		{kind: domainerrors.KindInternal, want: false},
		{kind: domainerrors.Kind("something-new"), want: false},
	}

	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			t.Parallel()

			err := domainerrors.New(tc.kind, "op", "msg", nil)
			if got := domainerrors.IsUserError(err); got != tc.want {
				t.Fatalf("IsUserError(%q) = %t, want %t", tc.kind, got, tc.want)
			}
		})
	}
}

func TestIsUserErrorOnNonAppErrors(t *testing.T) {
	t.Parallel()

	if domainerrors.IsUserError(nil) {
		t.Fatal("IsUserError(nil) = true, want false")
	}

	if domainerrors.IsUserError(errSentinel) {
		t.Fatal("IsUserError(plain error) = true, want false")
	}

	if domainerrors.IsUserError(fmt.Errorf("wrap: %w", errSentinel)) {
		t.Fatal("IsUserError(wrapped plain error) = true, want false")
	}
}

// TestIsUserErrorThroughWrapping is the case that matters in practice: the
// codebase wraps errors with %w on the way up the call stack, so the
// classification has to survive an arbitrary number of wrappers.
func TestIsUserErrorThroughWrapping(t *testing.T) {
	t.Parallel()

	userErr := domainerrors.New(domainerrors.KindUserInput, "geo.parse", "bad CRS", nil)

	wrapped := fmt.Errorf("import model: %w", userErr)
	wrapped = fmt.Errorf("run scenario: %w", wrapped)
	wrapped = fmt.Errorf("cli: %w", wrapped)

	if !domainerrors.IsUserError(wrapped) {
		t.Fatal("expected a triple-wrapped user error to stay a user error")
	}

	var appErr *domainerrors.AppError
	if !stderrors.As(wrapped, &appErr) {
		t.Fatal("expected errors.As to find the AppError")
	}

	if appErr.Kind != domainerrors.KindUserInput {
		t.Fatalf("recovered Kind = %q, want %q", appErr.Kind, domainerrors.KindUserInput)
	}
}

// TestIsUserErrorUsesOutermostAppError documents the resolution rule when an
// internal failure wraps a user-facing one: errors.As stops at the first
// AppError in the chain, so the outermost AppError wins and an internal error
// wrapping a user error is reported as internal (exit code 1).
func TestIsUserErrorUsesOutermostAppError(t *testing.T) {
	t.Parallel()

	inner := domainerrors.New(domainerrors.KindUserInput, "geo.parse", "bad CRS", nil)
	outer := domainerrors.New(domainerrors.KindInternal, "engine.run", "worker failed", inner)

	if domainerrors.IsUserError(outer) {
		t.Fatal("internal error wrapping a user error must classify as internal")
	}

	// The reverse nesting keeps the user classification.
	flipped := domainerrors.New(
		domainerrors.KindUserInput,
		"cli.run",
		"invalid scenario",
		domainerrors.New(domainerrors.KindInternal, "engine.run", "worker failed", nil),
	)

	if !domainerrors.IsUserError(flipped) {
		t.Fatal("user error wrapping an internal error must classify as user input")
	}
}

func TestUnwrapChainReachesCause(t *testing.T) {
	t.Parallel()

	err := domainerrors.New(domainerrors.KindInternal, "op", "msg", fmt.Errorf("layer: %w", errSentinel))

	if got := stderrors.Unwrap(err); got == nil {
		t.Fatal("Unwrap() = nil, want the wrapped cause")
	}

	if !stderrors.Is(err, errSentinel) {
		t.Fatal("expected errors.Is to reach the sentinel through two layers")
	}

	bare := domainerrors.New(domainerrors.KindInternal, "op", "msg", nil)
	if got := stderrors.Unwrap(bare); got != nil {
		t.Fatalf("Unwrap() on a causeless error = %v, want nil", got)
	}
}
