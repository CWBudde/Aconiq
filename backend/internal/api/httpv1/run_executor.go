package httpv1

// The `aconiq run` subprocess: how a run request becomes argv, and how the
// process that computes it is launched, interrupted and waited on.
//
// It sits beside handler.go rather than in it because it is the one place the
// API leaves its own process, and because the handler file is at its length
// limit.

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
)

func newCLIProcessRunExecutor(projectRoot string) runExecutor {
	return func(ctx context.Context, runID string, req createRunRequest) error {
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable: %w", err)
		}

		return awaitRunProcess(newRunProcess(ctx, executable, runCommandArgs(projectRoot, runID, req)))
	}
}

// awaitRunProcess runs a prepared `aconiq run` child to its end and, when it
// fails, answers with what it said was wrong.
//
// Only stderr is kept. The run command logs there, and a failed run's last
// record carries its error; stdout is the command's result, which the server
// reads from the project the child writes rather than from the child.
func awaitRunProcess(cmd *exec.Cmd) error {
	var stderr tailBuffer

	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return nil
	}

	return childProcessError(err, stderr.Bytes(), "httpv1.runExecutor", "execute run command")
}

// childProcessError is the error a failed `aconiq run` child is answered
// with: a user error in the child's own words when it refused the request
// (exit status 2, and its log says so), otherwise an error carrying the
// child's reason, or how the process exited when it gave none.
func childProcessError(err error, stderr []byte, op string, prefix string) error {
	failure := readChildFailure(stderr)

	var exitErr *exec.ExitError
	if stderrors.As(err, &exitErr) && exitErr.ExitCode() == 2 && failure.mayBeUserError {
		return domainerrors.New(domainerrors.KindUserInput, op, failure.message, err)
	}

	message := failure.message
	if message == "" {
		message = err.Error()
	}

	return fmt.Errorf("%s: %s", prefix, message)
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
	return appendRunRequestArgs([]string{"--project", projectRoot, "run", "--run-id", runID}, req)
}

// appendRunRequestArgs appends the flags that say what a request asks for —
// standard, scenario, model, parameters, inputs — which a run and an estimate
// of it share.
func appendRunRequestArgs(args []string, req createRunRequest) []string {
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

	for _, building := range req.FacadeBuildings {
		args = append(args, "--facade-building="+building)
	}

	if req.FacadeSpacingM != nil {
		args = append(args, "--facade-spacing-m", strconv.FormatFloat(*req.FacadeSpacingM, 'g', -1, 64))
	}

	if req.FacadeIncludeCourtyard {
		args = append(args, "--facade-courtyard")
	}

	if req.Experimental {
		args = append(args, "--experimental")
	}

	if req.AllowUnconfirmed {
		args = append(args, "--allow-unconfirmed")
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
