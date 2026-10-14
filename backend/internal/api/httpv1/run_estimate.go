package httpv1

// POST /api/v1/runs/estimate: what a run would cost in time, before it is
// started.
//
// # Why a child process, as a run is
//
// The estimate is `aconiq run --estimate --json`, spawned the way
// newCLIProcessRunExecutor spawns `aconiq run`, rather than a call into the
// standards code from this process. Three reasons, in order of weight:
//
//   - One pipeline. The estimate has to price the run the same request would
//     start — the same parameter resolution, the same model load and CRS
//     projection, the same receiver grid and the same prepared scene — and
//     refuse what that run would refuse, in its words. In the child it does,
//     because it is the run command's own code up to the scene. In process it
//     would be a second assembly of the CLI's run pipeline inside httpv1.
//   - Memory. A probe prepares the scene a run prepares, and on a city extract
//     that is the better part of a gigabyte held for a few seconds. In a child
//     it is returned when the child exits; in `aconiq serve` it would be heap
//     the server keeps.
//   - Consistency with run creation, for which the child process is already
//     the one way a request reaches the compute code.
//
// Unlike a run, an estimate is answered synchronously and writes nothing: the
// child creates no run row, so there is nothing for the run supervisor to
// track or settle. Its context is the request's: a client that gives up —
// the run dialog cancels a stale estimate whenever a setting changes —
// interrupts the child.

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
)

// estimateExecutor prices one run request and returns the estimate JSON.
type estimateExecutor func(ctx context.Context, req createRunRequest) (json.RawMessage, error)

// estimateOutputMode is the "mode" the run command's JSON output names an
// estimate with.
const estimateOutputMode = "estimate"

// useChildProcessExecutors gives the handler the `aconiq` child-process
// executors it was not given one of: the production path, which tests replace.
func useChildProcessExecutors(h *Handler, projectRoot string) {
	if h.estimateExecutor == nil {
		h.estimateExecutor = newCLIProcessEstimateExecutor(projectRoot)
	}

	if h.runExecutor == nil {
		h.runExecutor = newCLIProcessRunExecutor(projectRoot)
	}
}

// estimateCommandArgs is argv for `aconiq run --estimate`: the request's own
// flags, global --json so the answer is machine-read, and no --run-id because
// no run is created.
func estimateCommandArgs(projectRoot string, req createRunRequest) []string {
	args := appendRunRequestArgs([]string{"--project", projectRoot, "--json", "run"}, req)

	return append(args, "--estimate")
}

func newCLIProcessEstimateExecutor(projectRoot string) estimateExecutor {
	return func(ctx context.Context, req createRunRequest) (json.RawMessage, error) {
		executable, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("resolve executable: %w", err)
		}

		cmd := newRunProcess(ctx, executable, estimateCommandArgs(projectRoot, req))

		var stdout, stderr tailBuffer

		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		estimate, message := readEstimateOutput(stdout.Bytes())

		if err != nil {
			if message == "" {
				message = childErrorMessage(nil, stderr.Bytes())
			}

			var exitErr *exec.ExitError
			if stderrors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
				return nil, domainerrors.New(domainerrors.KindUserInput, "httpv1.estimateExecutor", message, err)
			}

			if message == "" {
				message = err.Error()
			}

			return nil, fmt.Errorf("estimate run: %s", message)
		}

		if estimate == nil {
			return nil, stderrors.New("estimate run: the run command printed no estimate")
		}

		return estimate, nil
	}
}

// readEstimateOutput reads the child's stdout, which under --json is a
// stream of JSON values: the structured log records, and the command's own
// output. It returns the estimate from the output that carries one, and the
// error the last log record that has one reported.
func readEstimateOutput(stdout []byte) (json.RawMessage, string) {
	var (
		estimate json.RawMessage
		message  string
	)

	// The command output is indented over many lines; the log records are one
	// line each. A decoder over the whole stream reads both.
	decoder := json.NewDecoder(bytes.NewReader(stdout))

	for {
		var value struct {
			Mode     string          `json:"mode"`
			Estimate json.RawMessage `json:"estimate"`
			Error    string          `json:"error"`
		}

		if decoder.Decode(&value) != nil {
			break
		}

		if value.Mode == estimateOutputMode && len(value.Estimate) > 0 {
			estimate = value.Estimate
		}

		if value.Error != "" {
			message = value.Error
		}
	}

	return estimate, message
}

func (h Handler) handleRunEstimate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, apiError{
			Code:    errorCodeMethodNotAllowed,
			Message: fmt.Sprintf("method %s is not allowed for %s", r.Method, r.URL.Path),
		})

		return
	}

	var req createRunRequest

	if !decodeJSONBody(w, r, maxRunCreateBodyBytes, &req) {
		return
	}

	err := req.validate()
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: err.Error(),
		})

		return
	}

	// The same gate a run passes: an estimate of a run that would be refused
	// is not an answer to anything.
	if gateErr, blocked := h.experimentalOptInGate(req); blocked {
		writeAPIError(w, http.StatusBadRequest, gateErr)

		return
	}

	_, err = h.store.Load()
	if err != nil {
		writeDomainError(w, err)

		return
	}

	estimate, err := h.estimateExecutor(r.Context(), req)
	if err != nil {
		writeRunCreateError(w, err)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(estimate)
}
