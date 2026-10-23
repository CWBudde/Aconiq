package httpv1

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// modelSaveSourceLabel is the source_path stamped into a model that arrived
// over the API. `aconiq import` records the input file here; the API has no
// file, so it records the channel instead, and provenance still says where the
// model came from.
const modelSaveSourceLabel = "api:model"

// modelReadSourceLabel is the source_path a reprojected read carries. It never
// reaches disk — GET builds a model in memory purely to run the transform — but
// the normaliser records one, and naming the channel keeps that honest.
const modelReadSourceLabel = "api:model:read"

// modelSaveRequest is the body of POST /api/v1/model. Model stays raw: it is
// handed to modelgeojson unparsed, exactly as `aconiq import` hands it the
// input file, so the two channels accept the same documents.
type modelSaveRequest struct {
	// CRS of the model's coordinates, like `aconiq import --input-crs`. Empty
	// means the coordinates are already in the project CRS.
	CRS   string          `json:"crs,omitempty"`
	Model json.RawMessage `json:"model"`
	// ExpectedHash makes the save a compare-and-write: the hash of the model
	// the client last read, or "none" when it expects no model at all. Empty
	// replaces whatever is there, as the save always has.
	ExpectedHash string `json:"expected_hash,omitempty"`
}

type modelSaveResponse struct {
	NormalizedPath       string `json:"normalized_path"`
	DumpPath             string `json:"dump_path"`
	ValidationReportPath string `json:"validation_report_path"`
	FeatureCount         int    `json:"feature_count"`
	// Hash is the receipt for what was just written: the SHA-256 of the
	// normalized file. A client keeps it beside its local draft and compares it
	// as a string against the hash on the project status later. It never
	// recomputes one — a re-serialised model is not the same bytes.
	Hash     string                    `json:"hash"`
	Warnings []validationIssueResponse `json:"warnings"`
}

// modelGetResponse is the body of GET /api/v1/model.
type modelGetResponse struct {
	// CRS names the coordinates actually returned, so a client never has to
	// infer it from what it asked for.
	CRS          string          `json:"crs"`
	ProjectCRS   string          `json:"project_crs"`
	Hash         string          `json:"hash"`
	FeatureCount int             `json:"feature_count"`
	Model        json.RawMessage `json:"model"`
}

// handleModel dispatches by method. requireMethod answers for a single expected
// method, so a path that serves two needs a switch, the way handleRuns does.
func (h Handler) handleModel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleModelGet(w, r)
	case http.MethodPost:
		h.handleModelSave(w, r)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, apiError{
			Code:    errorCodeMethodNotAllowed,
			Message: fmt.Sprintf("method %s is not allowed for %s", r.Method, r.URL.Path),
		})
	}
}

// handleModelGet returns the stored model, optionally reprojected into the CRS
// named by `?crs=`. It is what lets a reloaded workspace hydrate from the
// project rather than from whatever the browser happened to keep.
func (h Handler) handleModelGet(w http.ResponseWriter, r *http.Request) {
	proj, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	requested := strings.TrimSpace(r.URL.Query().Get("crs"))
	responseCRS := proj.CRS

	if requested != "" {
		parsed, parseErr := geo.ParseCRS(requested)
		if parseErr != nil {
			writeAPIError(w, http.StatusBadRequest, apiError{
				Code:    errorCodeBadRequest,
				Message: "crs is not a recognised CRS identifier: " + parseErr.Error(),
				Hint:    "Use an EPSG identifier such as `EPSG:4326`, or omit crs to receive the model in the project CRS.",
			})

			return
		}

		responseCRS = parsed.ID
	}

	// One read, hashed in place: reading the bytes and hashing the path
	// separately would let a concurrent save pair this model with the next
	// one's receipt.
	raw, hash, err := h.store.ReadModelWithHash()
	if err != nil {
		if writeModelNotFound(w, err) {
			return
		}

		writeDomainError(w, err)

		return
	}

	payload, featureCount, err := modelInCRS(raw, proj.CRS, responseCRS)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apiError{
			Code:    errorCodeInternalError,
			Message: "failed to render the stored model: " + err.Error(),
		})

		return
	}

	writeJSON(w, http.StatusOK, modelGetResponse{
		CRS:          responseCRS,
		ProjectCRS:   proj.CRS,
		Hash:         hash,
		FeatureCount: featureCount,
		Model:        payload,
	})
}

// modelInCRS renders the stored model in target. When the target is the project
// CRS the stored bytes are returned untouched — cheaper, and byte-identical to
// what is on disk, which is what the hash is a receipt for.
//
// A transform reuses the save path in reverse rather than adding transform code
// of its own: the stored file's CRS takes the "import" slot and the requested
// one the "project" slot, so a read reprojects through exactly the pipeline a
// write reprojected through.
func modelInCRS(raw []byte, projectCRS string, target string) (json.RawMessage, int, error) {
	if strings.EqualFold(strings.TrimSpace(target), strings.TrimSpace(projectCRS)) {
		var stored modelgeojson.FeatureCollection

		err := json.Unmarshal(raw, &stored)
		if err != nil {
			return nil, 0, fmt.Errorf("decode stored model: %w", err)
		}

		return json.RawMessage(raw), len(stored.Features), nil
	}

	model, err := modelgeojson.NormalizeWithCRS(raw, target, projectCRS, modelReadSourceLabel)
	if err != nil {
		return nil, 0, fmt.Errorf("reproject stored model into %s: %w", target, err)
	}

	encoded, err := json.Marshal(model.ToFeatureCollection())
	if err != nil {
		return nil, 0, fmt.Errorf("encode reprojected model: %w", err)
	}

	return encoded, len(model.Features), nil
}

// writeModelNotFound answers a missing model in its own words, and reports
// whether it did.
//
// writeDomainError would turn the same KindNotFound into "Initialize the
// project first", which is wrong here — the project loaded — and which a client
// cannot tell apart from a genuinely missing project. A dedicated code lets the
// frontend distinguish "no project" from "no model yet".
func writeModelNotFound(w http.ResponseWriter, err error) bool {
	var appErr *domainerrors.AppError
	if !stderrors.As(err, &appErr) || appErr.Kind != domainerrors.KindNotFound {
		return false
	}

	writeAPIError(w, http.StatusNotFound, apiError{
		Code:    errorCodeModelNotFound,
		Message: "the project has no model yet",
		Hint:    "Save one with POST /api/v1/model, or import one with `aconiq import`.",
	})

	return true
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
	var req modelSaveRequest

	if !decodeJSONBody(w, r, maxModelSaveBodyBytes, &req) {
		return
	}

	if !validExpectedHash(req.ExpectedHash) {
		writeInvalidExpectedHash(w)
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

	// Held for the rest of the handler, not just across Load and SaveModel:
	// the normalisation and validation between them can return early, and a
	// lock released on some paths and not others is the bug this prevents.
	defer h.lockManifest()()

	// The cross-process half is SaveModel's own: it re-reads the manifest
	// under the file lock before it saves, so an `aconiq run` child writing
	// meanwhile is not overwritten. This load is for the project CRS only.
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

	model, err = h.keepStoredCoordinates(model, proj.CRS, req.CRS)
	if err != nil {
		writeDomainError(w, err)
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
	// The receipt comes from the save itself: SaveModelIf hashes the bytes it
	// committed while it holds the project lock. Hashing the file after it
	// returns would race any writer that replaces the model in between, and
	// hand this client a receipt for someone else's model.
	hash, err := h.store.SaveModelIf(&proj, model, report, req.ExpectedHash)
	if writeHashMismatch(w, err) {
		return
	}

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
		Hash:                 hash,
		Warnings:             validationIssuesResponse(report.Warnings),
	})
}

// keptCoordinateToleranceM is how far a vertex may arrive from where it is
// stored and still count as not moved: 1 µm, far above the nanometres an
// EPSG:4326 round trip moves it (PLAN.md §1.6) and far below anything a
// person moves on a map.
const keptCoordinateToleranceM = 1e-6

// keepStoredCoordinates puts the stored coordinates back on every feature a
// save through another CRS did not move (modelgeojson.KeepStoredCoordinates).
// The map draws a projected project in EPSG:4326, and the nanometres the trip
// back adds would renumber the facade keys of a footprint whose vertices tie
// on x. A save in the project CRS, or into a project with no model yet, is
// taken as it arrived.
func (h Handler) keepStoredCoordinates(model modelgeojson.Model, projectCRS, requestCRS string) (modelgeojson.Model, error) {
	if requestCRS == "" || requestCRS == projectCRS {
		return model, nil
	}

	raw, err := h.store.ReadModel()
	if err != nil {
		var appErr *domainerrors.AppError
		if stderrors.As(err, &appErr) && appErr.Kind == domainerrors.KindNotFound {
			return model, nil
		}

		return modelgeojson.Model{}, fmt.Errorf("read the stored model: %w", err)
	}

	// A stored model that no longer parses is about to be replaced; there is
	// nothing in it to keep.
	stored, parseErr := modelgeojson.Normalize(raw, projectCRS, modelSaveSourceLabel)
	if parseErr == nil {
		model = modelgeojson.KeepStoredCoordinates(model, stored, keptCoordinateToleranceM)
	}

	return model, nil
}

// expectedHashField names the precondition in a JSON body and in a multipart
// form alike.
const expectedHashField = "expected_hash"

// validExpectedHash accepts what a compare-and-write precondition may say:
// nothing, "none", or a hash in the one spelling the server issues — bare
// lowercase hex. Anything else is a client bug, refused rather than compared,
// because it could never match and would read as a conflict.
func validExpectedHash(expected string) bool {
	if expected == "" || expected == projectfs.ExpectNone {
		return true
	}

	if len(expected) != sha256.Size*2 {
		return false
	}

	for _, c := range expected {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}

	return true
}

func writeInvalidExpectedHash(w http.ResponseWriter) {
	writeAPIError(w, http.StatusBadRequest, apiError{
		Code:    errorCodeBadRequest,
		Message: "expected_hash must be a SHA-256 as 64 lowercase hex digits, or \"none\"",
		Hint:    "Send the hash the server last issued (the save receipt or the project status), unchanged.",
	})
}

// writeHashMismatch answers a refused compare-and-write with 409 and the hash
// that is on disk, and reports whether err was one.
func writeHashMismatch(w http.ResponseWriter, err error) bool {
	var mismatch *projectfs.HashMismatchError
	if !stderrors.As(err, &mismatch) {
		return false
	}

	writeAPIError(w, http.StatusConflict, apiError{
		Code:    errorCodeHashMismatch,
		Message: fmt.Sprintf("the %s changed since expected_hash was read; nothing was written", mismatch.Subject),
		Details: map[string]any{
			expectedHashField: mismatch.Expected,
			"current_hash":    mismatch.Current,
		},
		Hint: "Read the current " + mismatch.Subject + " again, reapply the change and send it with the new hash.",
	})

	return true
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
