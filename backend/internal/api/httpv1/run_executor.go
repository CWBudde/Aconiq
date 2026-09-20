package httpv1

// The `aconiq run` subprocess: how a run request becomes argv, and how the
// process that computes it is launched and waited on.
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
