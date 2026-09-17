package httpv1

import (
	stderrors "errors"
	"net/http"

	"github.com/aconiq/backend/internal/geo/crstransform"
)

// transformRequest is the body of POST /api/v1/transform, and transformResponse
// is what it answers with.
//
// They are declared here rather than reused from crstransform because httpv1
// owns its wire contract: crstransform's doc comment holds the door open to
// handing a Float64Array across the JavaScript boundary instead of JSON, and
// that is a browser-boundary decision which must not be able to silently move
// an HTTP contract clients depend on. TestTransformDTOsMatchTheProjectorContract
// is what stops the two drifting while they are meant to agree.
type transformRequest struct {
	SourceCRS string `json:"source_crs"`
	// TargetCRS may be omitted or set to "auto", which asks the server to make
	// the same zone decision `aconiq run` makes.
	TargetCRS   string    `json:"target_crs,omitempty"`
	Coordinates []float64 `json:"coordinates"`
}

type transformResponse struct {
	SourceCRS string `json:"source_crs"`
	// TargetCRS is the CRS the coordinates are actually in, which for an "auto"
	// request is the resolved zone rather than the string that was asked for.
	TargetCRS string `json:"target_crs"`
	// Applied is false only when nothing moved, and then Coordinates are the
	// input values verbatim — not a round trip that happens to land close.
	Applied     bool      `json:"applied"`
	Coordinates []float64 `json:"coordinates"`
}

// handleTransform projects a batch of coordinates between two CRS.
//
// It exists so that API mode has a projector at all. Browser mode has one in
// process — the WebAssembly kernel — and `aconiq run` projects before anything
// reads a coordinate, but a client talking to this API had neither, so a
// projected model could be imported and then not drawn. Pulling the 4 MB kernel
// into a mode that never otherwise loads it, purely to draw a map, would buy the
// map at the cost of the mode's premise.
//
// The endpoint is a pure function of its body: it never touches the store, so it
// answers before `aconiq init` has run, and it cannot read or write the model.
// That is deliberate, and it is what keeps it on the right side of the invariant
// that governs the map — the map is a projection *of* the model, never a source
// *for* it.
//
// POST rather than GET, although the operation reads nothing: a district-sized
// batch does not fit in a URL. The cost is that securityMiddleware treats it as
// state-changing, so it requires the X-Aconiq-Client header and forces a
// preflight, which is a price worth paying for a bounded body.
func (h Handler) handleTransform(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req transformRequest
	if !decodeJSONBody(w, r, maxTransformBodyBytes, &req) {
		return
	}

	resp, err := crstransform.Transform(crstransform.Request{
		SourceCRS:   req.SourceCRS,
		TargetCRS:   req.TargetCRS,
		Coordinates: req.Coordinates,
	})
	if err != nil {
		writeTransformRefusal(w, req, err)

		return
	}

	writeJSON(w, http.StatusOK, transformResponse{
		SourceCRS:   resp.SourceCRS,
		TargetCRS:   resp.TargetCRS,
		Applied:     resp.Applied,
		Coordinates: resp.Coordinates,
	})
}

// writeTransformRefusal turns a refused projection into an error envelope.
//
// Message is the projector's own text, unprefixed. Every other handler in this
// package prefixes — handleModelSave writes "model is not a valid GeoJSON
// FeatureCollection: " in front of its cause — and this one must not: the
// geographic refusal is quoted verbatim from geo.ComputeCRSForGeographic so that
// browser mode, API mode and `aconiq run` refuse the same site in the same
// words. Context goes in Details, never in front of the text, and
// TestTransformRefusesAnUnprojectableSiteInTheProjectorsWords is what keeps it
// that way.
func writeTransformRefusal(w http.ResponseWriter, req transformRequest, err error) {
	var refusal *crstransform.Error
	if !stderrors.As(err, &refusal) {
		writeAPIError(w, http.StatusInternalServerError, apiError{
			Code:    errorCodeInternalError,
			Message: err.Error(),
		})

		return
	}

	apiErr := apiError{
		Code:    errorCodeBadRequest,
		Message: refusal.Error(),
		Details: map[string]any{"reason": string(refusal.Reason)},
	}

	switch refusal.Reason {
	case crstransform.ReasonOddCoordinateCount:
		apiErr.Details["coordinate_values"] = len(req.Coordinates)
		apiErr.Hint = "coordinates are flat and interleaved: x0, y0, x1, y1, …"
	case crstransform.ReasonSourceCRS:
		apiErr.Details["source_crs"] = req.SourceCRS
	case crstransform.ReasonTargetCRS:
		apiErr.Details["target_crs"] = req.TargetCRS
	case crstransform.ReasonGeographicRefused:
		apiErr.Code = errorCodeCRSNotProjectable
		apiErr.Details["source_crs"] = req.SourceCRS
		apiErr.Hint = "only ETRS89 / UTM zones 31 to 34 are supported"
	case crstransform.ReasonPipelineUnavailable:
		apiErr.Code = errorCodeCRSNotProjectable
		apiErr.Details["source_crs"] = req.SourceCRS
		apiErr.Details["target_crs"] = req.TargetCRS
	case crstransform.ReasonCoordinate:
		apiErr.Details["coordinate_index"] = refusal.Index
	}

	writeAPIError(w, http.StatusBadRequest, apiErr)
}
