package httpv1

import (
	stderrors "errors"
	"fmt"
	"net/http"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/io/osmimport"
	overpass "github.com/cwbudde/go-overpass"
)

type importOSMRequest struct {
	South            float64 `json:"south"`
	West             float64 `json:"west"`
	North            float64 `json:"north"`
	East             float64 `json:"east"`
	OverpassEndpoint string  `json:"overpass_endpoint,omitempty"`
}

func (h Handler) handleImportOSM(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req importOSMRequest

	if !decodeJSONBody(w, r, maxImportOSMBodyBytes, &req) {
		return
	}

	if endpointErr := validateOverpassEndpoint(req.OverpassEndpoint); endpointErr != nil {
		writeAPIError(w, http.StatusBadRequest, *endpointErr)
		return
	}

	if req.South >= req.North {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "south must be less than north",
		})

		return
	}

	if req.West >= req.East {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "west must be less than east",
		})

		return
	}

	if req.South < -90 || req.North > 90 || req.West < -180 || req.East > 180 {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "bounding box coordinates out of WGS84 range (lat: -90..90, lon: -180..180)",
		})

		return
	}

	fc, err := osmimport.Fetch(r.Context(), osmimport.Config{
		BBox: osmimport.BBox{
			South: req.South,
			West:  req.West,
			North: req.North,
			East:  req.East,
		},
		OverpassEndpoint: req.OverpassEndpoint,
	})
	if err != nil {
		var appErr *domainerrors.AppError
		if stderrors.As(err, &appErr) {
			writeDomainError(w, err)

			return
		}

		writeAPIError(w, http.StatusBadGateway, overpassAPIError(err))

		return
	}

	writeJSON(w, http.StatusOK, fc)
}

// overpassAPIError turns a failed Overpass query into an envelope that says
// which kind of failure it was.
//
// go-overpass carries the upstream status in a *overpass.ServerError and it
// survives both the retry wrapper and osmimport's own wrapping, so the status
// was always reachable here — it was simply never read, and every cause came
// back as the single sentence "Overpass API request failed". A blocked User-Agent,
// a rate limit and an outage are three different problems with three different
// remedies, and a reader who cannot tell them apart retries the one case that
// will never succeed on its own.
//
// It is a function rather than inline code because the 502 branch is otherwise
// unreachable from a test: the endpoint allowlist admits neither 127.0.0.1 nor
// http, so no httptest server can stand in for Overpass.
func overpassAPIError(err error) apiError {
	apiErr := apiError{
		Code:    errorCodeUpstreamError,
		Message: "Overpass API request failed",
	}

	var serverErr *overpass.ServerError
	if !stderrors.As(err, &serverErr) {
		apiErr.Hint = "The Overpass server could not be reached. Check the network connection, or try again later."

		return apiErr
	}

	status := serverErr.StatusCode
	apiErr.Message = fmt.Sprintf("Overpass API request failed with status %d", status)
	apiErr.Details = map[string]any{"upstream_status": status}

	switch {
	case status == http.StatusTooManyRequests || status == http.StatusGatewayTimeout:
		apiErr.Hint = "The Overpass server is busy or the query timed out. Wait a moment, then try again with a smaller bounding box."
	case status >= http.StatusInternalServerError:
		apiErr.Hint = "The Overpass server reported a fault of its own. Try again later, or use a different server."
	case status >= http.StatusBadRequest:
		apiErr.Hint = "The Overpass server refused the request itself rather than failing on it. Its usage policy may have changed, or this client may be blocked."
	}

	return apiErr
}
