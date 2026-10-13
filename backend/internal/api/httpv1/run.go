package httpv1

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// deleteRunResponse is the body of DELETE /api/v1/runs/{id}.
//
// The delete answers 200 with a body rather than 204: the UI has to be able to
// say that the export bundle was kept, which a bodyless response cannot, and
// the frontend's request helper reads every response as JSON.
type deleteRunResponse struct {
	RunID string `json:"run_id"`
	// RemovedPaths and RetainedPaths are always present, as [] rather than null,
	// so a client can iterate without a nil check.
	RemovedPaths  []string `json:"removed_paths"`
	RetainedPaths []string `json:"retained_paths"`
}

// handleRun serves the single-run resource. Go 1.22 routing gives the more
// specific /api/v1/runs/{id}/log precedence over this pattern, so registering it
// does not shadow the log endpoint.
func (h Handler) handleRun(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleRunGet(w, r)
	case http.MethodDelete:
		h.handleRunDelete(w, r)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, apiError{
			Code:    errorCodeMethodNotAllowed,
			Message: fmt.Sprintf("method %s is not allowed for %s", r.Method, r.URL.Path),
		})
	}
}

// handleRunGet answers one run, with its progress while it computes. It is
// what a client polls after POST /runs answered 202.
func (h Handler) handleRunGet(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")

	proj, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	for _, run := range proj.Runs {
		if run.ID == runID {
			writeJSON(w, http.StatusOK, h.summarizeRunWithProgress(proj, run))

			return
		}
	}

	writeAPIError(w, http.StatusNotFound, apiError{
		Code:    errorCodeNotFound,
		Message: fmt.Sprintf("run %q not found", runID),
	})
}

// handleRunCancel asks a run this server started to stop.
//
// The answer is 202, not 200: the child is interrupted and stops at its next
// context check — before every Teilstück for RLS-19, about a second on the
// Linden extract, but between receivers for the standards that check only
// there — so the run is still `running` when this returns. It ends `failed`,
// with "run cancelled" in its log — either the child's own line, or the
// server's, if the child had to be killed after runInterruptGrace.
//
// A run this server did not start is refused rather than signalled: its
// process is not this server's child, and a pid read off disk is not a handle
// anyone should send signals to.
func (h Handler) handleRunCancel(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	runID := r.PathValue("id")

	// An unreadable manifest is reported as what it is, a storage failure,
	// and nothing is signalled: the caller learns the cancel did not happen
	// rather than being told the run does not exist while it computes on.
	proj, run, found, err := h.findRun(runID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	if !found {
		writeAPIError(w, http.StatusNotFound, apiError{
			Code:    errorCodeNotFound,
			Message: fmt.Sprintf("run %q not found", runID),
		})

		return
	}

	if run.Status != project.RunStatusRunning && run.Status != project.RunStatusPending {
		writeAPIError(w, http.StatusConflict, apiError{
			Code:    errorCodeRunNotRunning,
			Message: fmt.Sprintf("run %q is %s and has nothing left to stop", runID, run.Status),
		})

		return
	}

	if !h.runs.cancel(runID, errRunCancelRequested) {
		writeAPIError(w, http.StatusConflict, apiError{
			Code:    errorCodeRunNotOwned,
			Message: fmt.Sprintf("run %q was not started by this server", runID),
			Hint:    "Stop it where it was started: Ctrl-C in the terminal running `aconiq run`.",
		})

		return
	}

	// Read again: a child that was between receivers may already have closed
	// its row, and the caller is better served by that than by the row as it
	// was before the signal. The signal has gone either way, so a read that
	// fails here falls back to the row read above rather than to an error.
	if latest, latestRun, ok, _ := h.findRun(runID); ok {
		proj, run = latest, latestRun
	}

	writeJSON(w, http.StatusAccepted, h.summarizeRunWithProgress(proj, run))
}

func (h Handler) handleRunDelete(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	if runID == "" {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: messageRunIDRequired,
		})

		return
	}

	unlock := h.lockManifest()

	result, err := h.store.DeleteRun(runID)

	unlock()

	if err != nil {
		writeDeleteRunError(w, runID, err)
		return
	}

	writeJSON(w, http.StatusOK, deleteRunResponse{
		RunID:         result.RunID,
		RemovedPaths:  result.RemovedPaths,
		RetainedPaths: result.RetainedPaths,
	})
}

// writeDeleteRunError answers the store's named refusals in their own
// words. Both would otherwise land in writeDomainError's generic buckets, where
// "still running" would read as a bad request and "no such run" would advise
// the caller to initialise a project that is already there.
func writeDeleteRunError(w http.ResponseWriter, runID string, err error) {
	switch {
	case stderrors.Is(err, projectfs.ErrRunNotFound):
		writeAPIError(w, http.StatusNotFound, apiError{
			Code:    errorCodeNotFound,
			Message: fmt.Sprintf("run %q not found", runID),
		})
	case stderrors.Is(err, projectfs.ErrExportInsideRun):
		writeAPIError(w, http.StatusConflict, apiError{
			Code:    errorCodeExportInsideRun,
			Message: fmt.Sprintf("run %q holds export bundles inside its own directory and cannot be deleted", runID),
			Hint:    "Move the export bundle out of .noise/runs/, then delete the run.",
		})
	case stderrors.Is(err, projectfs.ErrRunNotFinished):
		writeAPIError(w, http.StatusConflict, apiError{
			Code:    errorCodeRunNotFinished,
			Message: fmt.Sprintf("run %q is still being written and cannot be deleted", runID),
			Hint:    "Wait for the run to finish or fail, then delete it.",
		})
	default:
		writeDomainError(w, err)
	}
}

type artifactRefResponse struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

type runSummaryResponse struct {
	ID            string                `json:"id"`
	ScenarioID    string                `json:"scenario_id"`
	Context       string                `json:"context,omitempty"`
	StandardID    string                `json:"standard_id"`
	Version       string                `json:"version"`
	Profile       string                `json:"profile,omitempty"`
	ReceiverMode  string                `json:"receiver_mode,omitempty"`
	ReceiverSetID string                `json:"receiver_set_id,omitempty"`
	Status        string                `json:"status"`
	StartedAt     time.Time             `json:"started_at"`
	FinishedAt    time.Time             `json:"finished_at"`
	LogPath       string                `json:"log_path"`
	Artifacts     []artifactRefResponse `json:"artifacts"`
	// Progress is present while the run is pending or running and its module
	// has reported at least once. Absent means "not known", never "nothing
	// done".
	Progress *runProgressResponse `json:"progress,omitempty"`
}

// runProgressResponse is how far a running run's compute has got, as the
// process computing it last wrote to progress.json.
type runProgressResponse struct {
	Done      int       `json:"done"`
	Total     int       `json:"total"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// ETASeconds extrapolates the rate so far; absent until a receiver has
	// completed.
	ETASeconds *float64 `json:"eta_seconds,omitempty"`
}

// summarizeRunWithProgress is summarizeRun plus the run's progress report,
// for a run that is still open. A finished run carries none: its progress is
// its status, and reading a file per finished run on every poll of the list
// would cost more than it could say.
func (h Handler) summarizeRunWithProgress(proj project.Project, run project.Run) runSummaryResponse {
	summary := summarizeRun(proj, run)
	summary.Progress = h.runProgress(run)

	return summary
}

// runProgress reads an open run's progress report, or nil. A report that
// cannot be read is absent rather than an error: it is a report about the
// run, and the run's own answer must not fail for want of it.
func (h Handler) runProgress(run project.Run) *runProgressResponse {
	if run.Status != project.RunStatusRunning && run.Status != project.RunStatusPending {
		return nil
	}

	progress, ok, err := h.store.ReadRunProgress(run.ID)
	if err != nil || !ok {
		return nil
	}

	return &runProgressResponse{
		Done:       progress.Done,
		Total:      progress.Total,
		StartedAt:  progress.StartedAt,
		UpdatedAt:  progress.UpdatedAt,
		ETASeconds: progress.ETASeconds,
	}
}

type runLogResponse struct {
	RunID string   `json:"run_id"`
	Lines []string `json:"lines"`
}

func (h Handler) handleRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleRunsList(w, r)
	case http.MethodPost:
		h.handleRunCreate(w, r)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, apiError{
			Code:    errorCodeMethodNotAllowed,
			Message: fmt.Sprintf("method %s is not allowed for %s", r.Method, r.URL.Path),
		})
	}
}

func (h Handler) handleRunsList(w http.ResponseWriter, _ *http.Request) {
	proj, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	summaries := make([]runSummaryResponse, 0, len(proj.Runs))
	for _, v := range slices.Backward(proj.Runs) {
		summaries = append(summaries, h.summarizeRunWithProgress(proj, v))
	}

	writeJSON(w, http.StatusOK, summaries)
}

func summarizeRun(proj project.Project, run project.Run) runSummaryResponse {
	artifacts := make([]artifactRefResponse, 0)

	for _, a := range proj.Artifacts {
		if a.RunID != run.ID {
			continue
		}

		artifacts = append(artifacts, artifactRefResponse{
			ID:        a.ID,
			Kind:      a.Kind,
			Path:      a.Path,
			CreatedAt: a.CreatedAt,
		})
	}

	return runSummaryResponse{
		ID:            run.ID,
		ScenarioID:    run.ScenarioID,
		Context:       run.Standard.Context,
		StandardID:    run.Standard.ID,
		Version:       run.Standard.Version,
		Profile:       run.Standard.Profile,
		ReceiverMode:  run.ReceiverMode,
		ReceiverSetID: run.ReceiverSetID,
		Status:        run.Status,
		StartedAt:     run.StartedAt,
		FinishedAt:    run.FinishedAt,
		LogPath:       run.LogPath,
		Artifacts:     artifacts,
	}
}

func (h Handler) handleRunLog(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	runID := r.PathValue("id")
	if runID == "" {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: messageRunIDRequired,
		})

		return
	}

	proj, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	var logPath string

	for _, run := range proj.Runs {
		if run.ID == runID {
			logPath = run.LogPath
			break
		}
	}

	if logPath == "" {
		writeAPIError(w, http.StatusNotFound, apiError{
			Code:    errorCodeNotFound,
			Message: fmt.Sprintf("run %q not found", runID),
		})

		return
	}

	raw, readErr := readProjectFile(h.store.Root(), logPath)
	if readErr != nil {
		writeProjectFileError(w, readErr, "failed to read run log")
		return
	}

	lines := splitLogLines(string(raw))
	writeJSON(w, http.StatusOK, runLogResponse{
		RunID: runID,
		Lines: lines,
	})
}

func splitLogLines(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines
}
