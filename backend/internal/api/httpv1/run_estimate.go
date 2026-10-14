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
// track or settle. Its context is the request's, and the server takes one
// estimate at a time: a client that gives up — the run dialog cancels a stale
// estimate whenever a setting changes — or a newer estimate kills the child.

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// estimateExecutor prices one run request and returns the estimate JSON.
type estimateExecutor func(ctx context.Context, req createRunRequest) (json.RawMessage, error)

// estimateOutputMode is the "mode" the run command's JSON output names an
// estimate with.
const estimateOutputMode = "estimate"

// maxEstimateOutputBytes bounds the estimate's stdout. The document is a few
// kilobytes; one that is not has stopped being an estimate.
const maxEstimateOutputBytes = 1 << 20

// estimateKillGrace bounds how long a killed estimate's output is drained.
const estimateKillGrace = time.Second

// estimateSlot is the one estimate this server computes at a time.
//
// The run dialog asks again whenever a setting changes, and the probe it
// starts holds the machine's cores' worth of attention for seconds and a
// city scene's worth of memory. Two at once would each measure a machine the
// other is loading, so a new estimate cancels the one in flight — whose
// answer was about to be stale anyway — rather than queueing behind it.
type estimateSlot struct {
	mu         sync.Mutex
	generation uint64
	cancel     context.CancelFunc
}

// claim cancels the estimate in flight, if any, and returns the context the
// new one runs under and its release.
func (s *estimateSlot) claim(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	s.generation++
	generation := s.generation
	s.cancel = cancel

	return ctx, func() {
		cancel()

		s.mu.Lock()
		defer s.mu.Unlock()

		if s.generation == generation {
			s.cancel = nil
		}
	}
}

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

		// Killed rather than interrupted, and not waited for: an estimate
		// writes nothing, so there is no record for a grace period to let it
		// finish. WaitDelay only bounds how long its output pipes are drained.
		cmd.Cancel = func() error {
			err := cmd.Process.Kill()
			if err != nil && !stderrors.Is(err, os.ErrProcessDone) {
				return fmt.Errorf("kill estimate process: %w", err)
			}

			return nil
		}
		cmd.WaitDelay = estimateKillGrace

		return awaitEstimateProcess(cmd)
	}
}

// awaitEstimateProcess runs a prepared `aconiq run --estimate --json` child to
// its end and returns the estimate it printed.
//
// The two streams are read for different things. stdout is the command's
// result and nothing else — one JSON document — so it is kept from the front,
// up to maxEstimateOutputBytes, and decoded whole. stderr is the command's log,
// whose last record names the error when the child fails, so it is kept from
// the back.
func awaitEstimateProcess(cmd *exec.Cmd) (json.RawMessage, error) {
	stdout := headBuffer{limit: maxEstimateOutputBytes}

	var stderr tailBuffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, childProcessError(err, stderr.Bytes(), "httpv1.estimateExecutor", "estimate run")
	}

	output, overflowed := stdout.Bytes()
	if overflowed {
		return nil, fmt.Errorf("estimate run: the run command printed more than %d bytes", maxEstimateOutputBytes)
	}

	estimate, err := readEstimateOutput(output)
	if err != nil {
		return nil, fmt.Errorf("estimate run: %w", err)
	}

	return estimate, nil
}

// readEstimateOutput decodes the child's stdout — under --json exactly one
// JSON document, the command's output — and returns the estimate it carries.
// Anything else on stdout, a second value or a stray line, is refused rather
// than skipped: it would mean the stream is no longer only the result.
func readEstimateOutput(stdout []byte) (json.RawMessage, error) {
	var output struct {
		Mode     string          `json:"mode"`
		Estimate json.RawMessage `json:"estimate"`
	}

	decoder := json.NewDecoder(bytes.NewReader(stdout))

	err := decoder.Decode(&output)
	if err != nil {
		return nil, fmt.Errorf("decode the run command's output: %w", err)
	}

	_, err = decoder.Token()
	if !stderrors.Is(err, io.EOF) {
		return nil, stderrors.New("the run command printed more than one JSON document")
	}

	if output.Mode != estimateOutputMode || len(output.Estimate) == 0 {
		return nil, stderrors.New("the run command printed no estimate")
	}

	return output.Estimate, nil
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

	ctx, release := h.estimates.claim(r.Context())
	defer release()

	estimate, err := h.estimateExecutor(ctx, req)
	if err != nil {
		// Cancelled while its own client was still there: a newer estimate
		// took the slot.
		if ctx.Err() != nil && r.Context().Err() == nil {
			writeAPIError(w, http.StatusConflict, apiError{
				Code:    errorCodeEstimateSuperseded,
				Message: "a newer estimate replaced this one before it finished",
			})

			return
		}

		writeRunCreateError(w, err)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(estimate)
}
