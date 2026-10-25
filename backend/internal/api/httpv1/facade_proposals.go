package httpv1

import (
	"net/http"
	"strings"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// proposeWindowsRequest is the body of POST /api/v1/facades/windows, and
// proposeWindowsResponse is what it answers with. The windows are placements
// in the shape POST /api/v1/facades/window takes.
type proposeWindowsRequest struct {
	ExpectedHash      string                 `json:"expected_hash"`
	TransmissionBasis string                 `json:"transmission_basis"`
	Windows           []windowPlacementInput `json:"windows"`
}

type proposeWindowsResponse struct {
	Hash      string                    `json:"hash"`
	WindowIDs []string                  `json:"window_ids"`
	Warnings  []validationIssueResponse `json:"warnings"`
}

// transmissionBasisField names the request member, as the refusals spell it.
const transmissionBasisField = "transmission_basis"

// handleFacadeWindowsPropose adds windows proposed from an image to the saved
// model (P18 D2): the write `aconiq mcp`'s propose_windows serves. Unlike
// POST /api/v1/facades/window it reads the project and writes it, because an
// agent has no model of its own to put the feature in. Every rule is
// facadeset.ProposeWindows's: proposed and not sensitive, with evidence that
// names the image, and only new ids, so a window a person reviewed is never
// touched. The write is a compare-and-write against expected_hash, which is
// required: a proposal is placed on the model the agent read, and nothing
// else.
func (h Handler) handleFacadeWindowsPropose(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req proposeWindowsRequest
	if !decodeJSONBody(w, r, maxModelSaveBodyBytes, &req) || !validProposal(w, req) {
		return
	}

	defer h.lockManifest()()

	proj, model, ok := h.savedModel(w)
	if !ok {
		return
	}

	placements := make([]facadeset.WindowPlacement, len(req.Windows))
	for i, in := range req.Windows {
		placements[i] = in.placement()
	}

	proposed, ids, err := facadeset.ProposeWindows(model, placements, req.TransmissionBasis)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: err.Error(),
			Hint:    "Propose new windows only, proposed and not sensitive, each with evidence naming the image: kind, source_id and content_sha256.",
		})

		return
	}

	report := modelgeojson.Validate(proposed)
	if report.ErrorCount() > 0 {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeModelInvalid,
			Message: "the model with the proposed windows failed validation",
			Details: map[string]any{"errors": validationIssuesResponse(report.Errors)},
		})

		return
	}

	hash, err := h.store.SaveModelIf(&proj, proposed, report, req.ExpectedHash)
	if writeHashMismatch(w, err) {
		return
	}

	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, proposeWindowsResponse{
		Hash:      hash,
		WindowIDs: ids,
		Warnings:  validationIssuesResponse(report.Warnings),
	})
}

// validProposal refuses a proposal that names no model it was placed on or
// no transmission decision, and reports whether it did not. Unlike a model
// save, expected_hash must be a hash: empty would make the write
// unconditional, and none names no model to place a window on.
func validProposal(w http.ResponseWriter, req proposeWindowsRequest) bool {
	if req.ExpectedHash == "" || req.ExpectedHash == projectfs.ExpectNone || !validExpectedHash(req.ExpectedHash) {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "expected_hash is required: the hash of the model the windows were proposed on, as 64 lowercase hex digits",
			Hint:    "Send the hash GET /api/v1/model or the project status returned, unchanged.",
		})

		return false
	}

	if strings.TrimSpace(req.TransmissionBasis) == "" {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: transmissionBasisField + " is required: the per-engagement decision the image was read under",
			Hint:    "aconiq mcp sends the value of its --photo-transmission flag.",
		})

		return false
	}

	return true
}

// savedModel reads the project and its saved model, with the model's import
// record, for a write that changes the model rather than replacing it. It
// answers a failure itself and reports whether it did not.
func (h Handler) savedModel(w http.ResponseWriter) (project.Project, modelgeojson.Model, bool) {
	proj, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return project.Project{}, modelgeojson.Model{}, false
	}

	raw, err := h.store.ReadModel()
	if writeModelNotFound(w, err) {
		return project.Project{}, modelgeojson.Model{}, false
	}

	if err == nil {
		var model modelgeojson.Model

		model, err = modelgeojson.Normalize(raw, proj.CRS, h.store.RelativePath(h.store.ModelArtifactPaths().Normalized))
		if err == nil {
			err = h.store.KeepImportRecord(&model)
		}

		if err == nil {
			return proj, model, true
		}
	}

	writeDomainError(w, err)

	return project.Project{}, modelgeojson.Model{}, false
}
