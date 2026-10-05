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
//
// It turns GET /api/v1/runs/abc from a 404 into a 405, which is the more
// correct answer: the resource is routed, the method is not offered.
func (h Handler) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeAPIError(w, http.StatusMethodNotAllowed, apiError{
			Code:    errorCodeMethodNotAllowed,
			Message: fmt.Sprintf("method %s is not allowed for %s", r.Method, r.URL.Path),
		})

		return
	}

	h.handleRunDelete(w, r)
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
		summaries = append(summaries, summarizeRun(proj, v))
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
