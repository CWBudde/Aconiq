package httpv1

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// modelSaveSourceLabel is the source_path stamped into a model that arrived
// over the API. `aconiq import` records the input file here; the API has no
// file, so it records the channel instead, and provenance still says where the
// model came from.
const modelSaveSourceLabel = "api:model"

// modelSaveRequest is the body of POST /api/v1/model. Model stays raw: it is
// handed to modelgeojson unparsed, exactly as `aconiq import` hands it the
// input file, so the two channels accept the same documents.
type modelSaveRequest struct {
	// CRS of the model's coordinates, like `aconiq import --input-crs`. Empty
	// means the coordinates are already in the project CRS.
	CRS   string          `json:"crs,omitempty"`
	Model json.RawMessage `json:"model"`
}

type modelSaveResponse struct {
	NormalizedPath       string                    `json:"normalized_path"`
	DumpPath             string                    `json:"dump_path"`
	ValidationReportPath string                    `json:"validation_report_path"`
	FeatureCount         int                       `json:"feature_count"`
	Warnings             []validationIssueResponse `json:"warnings"`
}

// validationIssueResponse is one validation finding as the API reports it.
// The level is left out: the list an issue sits in already says whether it is
// an error or a warning.
type validationIssueResponse struct {
	Code      string `json:"code"`
	FeatureID string `json:"feature_id,omitempty"`
	Message   string `json:"message"`
}

// handleModelSave replaces the project model with the FeatureCollection in the
// request. It is the API-side twin of `aconiq import --input`: same
// normalisation, same validation gate, same files and manifest refs, through
// the same store method.
func (h Handler) handleModelSave(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req modelSaveRequest

	if !decodeJSONBody(w, r, maxModelSaveBodyBytes, &req) {
		return
	}

	if isJSONAbsent(req.Model) {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "model is required",
			Hint:    "Send a GeoJSON FeatureCollection as `model`; see docs/geojson-schema-v1.md for the feature schema.",
		})

		return
	}

	proj, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	model, err := modelgeojson.NormalizeWithCRS(req.Model, proj.CRS, req.CRS, modelSaveSourceLabel)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "model is not a valid GeoJSON FeatureCollection: " + err.Error(),
		})

		return
	}

	report := modelgeojson.Validate(model)
	if report.ErrorCount() > 0 {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeModelInvalid,
			Message: "model failed validation",
			Details: map[string]any{
				"errors": validationIssuesResponse(report.Errors),
			},
			Hint: "Fix the features listed in details.errors (each carries its feature_id) and send the model again. Nothing was written.",
		})

		return
	}

	// SaveModel returns domain errors whose message names the step, not the
	// file: the absolute path stays in the wrapped cause on the server side and
	// never reaches the envelope.
	err = h.store.SaveModel(&proj, model, report)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	paths := h.store.ModelArtifactPaths()

	writeJSON(w, http.StatusCreated, modelSaveResponse{
		NormalizedPath:       h.store.RelativePath(paths.Normalized),
		DumpPath:             h.store.RelativePath(paths.Dump),
		ValidationReportPath: h.store.RelativePath(paths.Validation),
		FeatureCount:         len(model.Features),
		Warnings:             validationIssuesResponse(report.Warnings),
	})
}

// isJSONAbsent reports whether a raw member was missing or explicitly null,
// which for a required object are the same omission.
func isJSONAbsent(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)

	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

// validationIssuesResponse always returns a list, never nil, so an empty
// warnings member serialises as `[]` rather than `null`.
func validationIssuesResponse(issues []modelgeojson.ValidationIssue) []validationIssueResponse {
	out := make([]validationIssueResponse, 0, len(issues))
	for _, issue := range issues {
		out = append(out, validationIssueResponse{
			Code:      issue.Code,
			FeatureID: issue.FeatureID,
			Message:   issue.Message,
		})
	}

	return out
}
