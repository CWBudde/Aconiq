package httpv1

// The `aconiq run` subprocess: how a run request becomes argv, how the process
// that computes it is launched and waited on, and what is left to clean up
// when it dies without saying so.
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

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

func newCLIProcessRunExecutor(projectRoot string) runExecutor {
	return func(ctx context.Context, req createRunRequest) error {
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable: %w", err)
		}

		// G702 reports the taint from the request body to argv. The flow is real,
		// but every field that reaches args is constrained by
		// createRunRequest.validate before the handler calls this executor:
		// identifiers and parameter names must match a fixed pattern, and paths
		// must be relative and inside the project. There is no shell — argv is
		// passed as a slice.
		//nolint:gosec // request fields are validated by createRunRequest.validate
		cmd := exec.CommandContext(ctx, executable, runCommandArgs(projectRoot, req)...)

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

// runCommandArgs turns a validated request into argv for `aconiq run`. Optional
// fields are omitted rather than passed empty, so the run command applies its
// own defaults.
func runCommandArgs(projectRoot string, req createRunRequest) []string {
	args := []string{"--project", projectRoot, "run"}

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

// closeRunInterruptedBy marks this request's run failed after its subprocess
// died without recording one.
//
// The terminal status is written by the `aconiq run` child and by nothing
// else, so a child that was killed, cancelled together with its HTTP request —
// which is what a browser reload during a run does — or shot by the OOM killer
// leaves a row reading "running" that nothing will ever come back to. The UI
// then draws a spinner forever, and is right to: the manifest is the only
// thing it can ask.
//
// # Why ownership is worked out by difference, and why that needs a count
//
// The child mints the run id, and this process never learns it: it passes argv
// and reads an exit code. So the only handle on "which row is mine" is the
// difference between the manifest as this request found it and the manifest
// now — the rows that are open and were not there before.
//
// That difference identifies one request's run only while it holds exactly one
// row. Two overlapping POSTs both snapshot a manifest without either run in
// it, so each would see the other's row in its own difference, and a failure
// in one would mark a run that is still computing as failed. Losing a live run
// is much worse than leaving a spinner, so anything but exactly one row closes
// nothing. What is left over is reconciled the next time `aconiq serve`
// starts, which is the same sweep that catches a server killed mid-run.
func (h Handler) closeRunInterruptedBy(before project.Project) {
	after, err := h.store.Load()
	if err != nil {
		h.logger.Error("cannot reconcile the interrupted run: loading the project failed",
			"error", err)

		return
	}

	existing := projectfs.RunIDs(before)

	var opened []string

	for _, run := range after.Runs {
		if run.Status != project.RunStatusRunning && run.Status != project.RunStatusPending {
			continue
		}

		if _, existed := existing[run.ID]; existed {
			continue
		}

		opened = append(opened, run.ID)
	}

	if len(opened) != 1 {
		// Nothing opened: the child failed before it created a row, which is
		// every refusal the run command makes up front. More than one: a
		// concurrent request, and no way to tell the rows apart.
		if len(opened) > 1 {
			h.logger.Warn("leaving interrupted runs open: concurrent run requests make ownership ambiguous",
				"candidates", opened)
		}

		return
	}

	interrupted := opened[0]

	_, err = h.store.FailInterruptedRuns(h.now, func(run project.Run) bool {
		return run.ID != interrupted
	})
	if err != nil {
		// The request is about to be answered with the executor's error, which
		// is the one the caller asked about. This failure is the reason the run
		// will keep reading "running" until the server restarts, and there is
		// nowhere in that response to say so.
		h.logger.Error("interrupted run left open: closing it in the manifest failed",
			"run_id", interrupted, "error", err)
	}
}
