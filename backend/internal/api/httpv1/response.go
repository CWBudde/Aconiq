package httpv1

import (
	stderrors "errors"
	"net/http"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/jsonio"
)

// apiError.Code values. They are part of the HTTP API contract: clients switch
// on them, and openapi.go documents them, so every emission site must use these
// spellings.
const (
	errorCodeBadRequest       = "bad_request"
	errorCodeNotFound         = "not_found"
	errorCodeInternalError    = "internal_error"
	errorCodeMethodNotAllowed = "method_not_allowed"
	// errorCodeExperimentalOptInRequired answers a run request that targets a
	// scaffold-tier standard without acknowledging what that tier means.
	errorCodeExperimentalOptInRequired = "experimental_opt_in_required"
	// errorCodeModelInvalid answers a model that parsed as GeoJSON but failed
	// schema validation; details.errors carries the findings per feature.
	errorCodeModelInvalid = "model_invalid"
	// errorCodeModelNotFound answers a read of a project that loaded but has no
	// model saved yet. It is deliberately not errorCodeNotFound: a client has to
	// be able to tell "no project" from "no model", and the two want different
	// hints.
	errorCodeModelNotFound = "model_not_found"
	// errorCodeRunHasNoRaster answers a contour request for a run that exists but
	// recorded no result raster. It is deliberately not errorCodeNotFound, for
	// the reason errorCodeModelNotFound is not: "no such run" and "this run was
	// not computed over a grid" send a client to two different remedies, and
	// only one of them is checking the id.
	errorCodeRunHasNoRaster = "run_has_no_raster"

	// errorCodeRunHasNoGrid answers a contour request for a run whose raster
	// exists but was computed over receivers placed individually. It is not
	// run_has_no_raster: there is one, it simply records no cell size, and the
	// remedy is a different receiver mode rather than a different run.
	errorCodeRunHasNoGrid = "run_has_no_grid"
	// errorCodeRunNotFinished answers a delete of a run that is still pending or
	// running: its directory is being written by a live `aconiq run`.
	errorCodeRunNotFinished = "run_not_finished"
	// errorCodeExportInsideRun answers a delete whose export bundle sits inside
	// the run directory: the bundle is kept, so the directory cannot go.
	errorCodeExportInsideRun = "export_inside_run"
	// errorCodeCRSNotProjectable answers a transform that cannot be carried out
	// for the CRS it was given: a geographic site whose centre falls outside the
	// ETRS89 / UTM zones this project supports, or a pair with no route between
	// them. One code, not two, because the remedy is the same — the CRS setup is
	// wrong — and details.reason separates the causes for a client that cares.
	errorCodeCRSNotProjectable = "crs_not_projectable"
	// errorCodeUpstreamError answers a request this API could not complete
	// because a third-party server it depends on refused or failed. One code for
	// all of them, because the remedy is never in the project — but
	// details.upstream_status carries the status the server actually sent, since
	// "you were blocked", "you were rate limited" and "it is down" send a reader
	// to three different remedies and used to arrive as one sentence.
	errorCodeUpstreamError = "upstream_error"
	// errorCodeLGLNTooManyTiles answers an LGLN import whose bounding box
	// intersects more tiles than one request may download.
	errorCodeLGLNTooManyTiles = "lgln_too_many_tiles"
	// errorCodeLGLNUnavailable answers an LGLN import the LGLN service could
	// not serve: unreachable, failing, or answering with something unusable.
	errorCodeLGLNUnavailable = "lgln_unavailable"

	// The transport-level controls in security.go. They are refusals to route,
	// not endpoint answers, so they can appear on any path.
	errorCodeForbiddenHost              = "forbidden_host"
	errorCodeUnauthorized               = "unauthorized"
	errorCodeClientHeaderRequired       = "client_header_required"
	errorCodeUnsupportedMediaType       = "unsupported_media_type"
	errorCodeRequestTooLarge            = "request_too_large"
	errorCodeForbiddenPath              = "forbidden_path"
	errorCodeOverpassEndpointNotAllowed = "overpass_endpoint_not_allowed"
)

type errorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
	Hint    string         `json:"hint,omitempty"`
}

func requireMethod(w http.ResponseWriter, r *http.Request, expected string) bool {
	if r.Method == expected {
		return true
	}

	writeAPIError(w, http.StatusMethodNotAllowed, apiError{
		Code:    errorCodeMethodNotAllowed,
		Message: "unsupported HTTP method",
		Details: map[string]any{
			"method":   r.Method,
			"expected": expected,
			"path":     r.URL.Path,
		},
	})

	return false
}

func writeDomainError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	apiErr := apiError{
		Code:    errorCodeInternalError,
		Message: "request failed",
	}

	var appErr *domainerrors.AppError
	if stderrors.As(err, &appErr) {
		if appErr.Msg != "" {
			apiErr.Message = appErr.Msg
		}

		apiErr.Details = map[string]any{
			"operation": appErr.Op,
			"kind":      appErr.Kind,
		}

		switch appErr.Kind {
		case domainerrors.KindUserInput:
			status = http.StatusBadRequest
			apiErr.Code = "user_input_error"
		case domainerrors.KindValidation:
			status = http.StatusBadRequest
			apiErr.Code = "validation_error"
		case domainerrors.KindNotFound:
			status = http.StatusNotFound
			apiErr.Code = errorCodeNotFound
			apiErr.Hint = "Initialize the project first with `aconiq init`."
		default:
			status = http.StatusInternalServerError
			apiErr.Code = errorCodeInternalError
		}
	}

	writeAPIError(w, status, apiErr)
}

func writeAPIError(w http.ResponseWriter, status int, apiErr apiError) {
	writeJSON(w, status, errorResponse{Error: apiErr})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	encoded, err := jsonio.Marshal(payload)
	if err != nil {
		// The canned body carries its own newline, because the success path's
		// came from jsonio.Marshal. Both branches still end the same way.
		encoded = []byte("{\"error\":{\"code\":\"internal_error\",\"message\":\"failed to encode response\"}}\n")
		status = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}
