package httpv1

// The `aconiq run` subprocess: how a run request becomes argv, and how the
// process that computes it is launched, interrupted and waited on.
//
// It sits beside handler.go rather than in it because it is the one place the
// API leaves its own process, and because the handler file is at its length
// limit.

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
)

func newCLIProcessRunExecutor(projectRoot string) runExecutor {
	return func(ctx context.Context, runID string, req createRunRequest) error {
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable: %w", err)
		}

		cmd := newRunProcess(ctx, executable, runCommandArgs(projectRoot, runID, req))

		var stderr bytes.Buffer

		cmd.Stdout = io.Discard
		cmd.Stderr = &stderr

		err = cmd.Run()
		if err != nil {
			var exitErr *exec.ExitError
			if stderrors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
				return domainerrors.New(domainerrors.KindUserInput, "httpv1.runExecutor", strings.TrimSpace(stderr.String()), err)
			}

			message := strings.TrimSpace(stderr.String())
			if message == "" {
				message = err.Error()
			}

			return fmt.Errorf("execute run command: %s", message)
		}

		return nil
	}
}

// runInterruptGrace is how long an interrupted `aconiq run` child has to record
// its own run as failed before it is killed. RLS-19 stops before its next
// Teilstück and the other standards before their next receiver, so this
// bounds that much work plus the manifest write; a child still busy after it
// is killed, and the server closes its row instead (settleRun).
const runInterruptGrace = 10 * time.Second

// newRunProcess prepares the `aconiq run` child for one run.
//
// When ctx ends — a client cancelled the run, or the server is shutting down —
// the child is interrupted rather than killed, which is what
// exec.CommandContext would do by default. On SIGINT `aconiq run` stops at its
// next context check and records its run as failed itself, with "run cancelled" in its
// log; a killed child leaves its row reading "running" for settleRun to close.
// Only a child still running runInterruptGrace later is killed.
func newRunProcess(ctx context.Context, executable string, args []string) *exec.Cmd {
	// G702 reports the taint from the request body to argv. The flow is real,
	// but every field that reaches args is constrained by
	// createRunRequest.validate before the handler calls this executor:
	// identifiers and parameter names must match a fixed pattern, and paths
	// must be relative and inside the project. The run id is minted by the
	// server. There is no shell — argv is passed as a slice.
	//nolint:gosec // request fields are validated by createRunRequest.validate
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Cancel = func() error {
		return interruptProcess(cmd.Process)
	}
	cmd.WaitDelay = runInterruptGrace

	return cmd
}

// interruptProcess asks a process to stop. Where a process cannot be sent
// os.Interrupt — Windows — it is killed instead, which is the behaviour
// before interrupting existed.
func interruptProcess(process *os.Process) error {
	err := process.Signal(os.Interrupt)
	if err == nil || stderrors.Is(err, os.ErrProcessDone) {
		return nil
	}

	err = process.Kill()
	if err != nil {
		return fmt.Errorf("kill run process: %w", err)
	}

	return nil
}

// runCommandArgs turns a validated request into argv for `aconiq run`. Optional
// fields are omitted rather than passed empty, so the run command applies its
// own defaults. The run id is the server's, passed through the hidden
// --run-id, so the row the child creates is one the server can name.
func runCommandArgs(projectRoot string, runID string, req createRunRequest) []string {
	args := []string{"--project", projectRoot, "run", "--run-id", runID}

	for _, flag := range []struct {
		name  string
		value string
	}{
		{"--scenario", req.ScenarioID},
		{"--standard", req.StandardID},
		{"--standard-version", req.StandardVersion},
		{"--standard-profile", req.StandardProfile},
		{"--model", req.ModelPath},
		{"--receiver-mode", req.ReceiverMode},
	} {
		if flag.value != "" {
			args = append(args, flag.name, flag.value)
		}
	}

	if req.Experimental {
		args = append(args, "--experimental")
	}

	paramKeys := make([]string, 0, len(req.Params))
	for key := range req.Params {
		paramKeys = append(paramKeys, key)
	}

	slices.Sort(paramKeys)

	for _, key := range paramKeys {
		args = append(args, "--param", fmt.Sprintf("%s=%s", key, req.Params[key]))
	}

	for _, inputPath := range req.InputPaths {
		args = append(args, "--input", inputPath)
	}

	return args
}
