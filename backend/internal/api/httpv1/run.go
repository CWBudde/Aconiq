package httpv1

import (
	stderrors "errors"
	"fmt"
	"net/http"

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

	result, err := h.store.DeleteRun(runID)
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
