package httpv1

import (
	"context"
	stderrors "errors"
	"net/http"
	"path/filepath"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/lglnimport"
)

// maxImportLGLNBodyBytes caps the LGLN import request: four numbers.
const maxImportLGLNBodyBytes = 4 << 10

type importLGLNRequest struct {
	South float64 `json:"south"`
	West  float64 `json:"west"`
	North float64 `json:"north"`
	East  float64 `json:"east"`
}

type importLGLNTile struct {
	ID   string `json:"id"`
	Date string `json:"date"`
}

// importLGLNResponse is a GeoJSON FeatureCollection with three foreign
// members: which tiles the buildings came from, the source note the licence
// requires, and the parser's skip counts.
type importLGLNResponse struct {
	Type        string                        `json:"type"`
	Features    []modelgeojson.GeoJSONFeature `json:"features"`
	Tiles       []importLGLNTile              `json:"tiles"`
	Attribution string                        `json:"attribution"`
	Skipped     map[string]int                `json:"skipped"`
}

// handleImportLGLN answers POST /api/v1/import/lgln: the LGLN LoD2 buildings
// whose footprint centroid lies in the box, in WGS84. Like /import/osm it
// saves nothing; the client merges the result into its model. The tiles it
// downloads are cached under .noise/cache/lgln, so a second request for the
// same area does not download them again.
func (h Handler) handleImportLGLN(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req importLGLNRequest

	if !decodeJSONBody(w, r, maxImportLGLNBodyBytes, &req) {
		return
	}

	bb := lglnimport.BBox{West: req.West, South: req.South, East: req.East, North: req.North}

	cacheDir := filepath.Join(h.store.Root(), ".noise", "cache", "lgln")

	result, err := h.lgln.Load(r.Context(), bb, cacheDir)
	if err != nil {
		writeAPIError(w, lglnErrorStatus(err), lglnAPIError(err))

		return
	}

	tiles := make([]importLGLNTile, 0, len(result.Tiles))
	for _, t := range result.Tiles {
		tiles = append(tiles, importLGLNTile{ID: t.ID, Date: t.Updated})
	}

	writeJSON(w, http.StatusOK, importLGLNResponse{
		Type:        modelgeojson.TypeFeatureCollection,
		Features:    result.Collection.Features,
		Tiles:       tiles,
		Attribution: lglnimport.Attribution(result.Tiles),
		Skipped:     result.Skipped,
	})
}

func lglnErrorStatus(err error) int {
	switch {
	case stderrors.Is(err, lglnimport.ErrInvalidBBox),
		stderrors.Is(err, lglnimport.ErrOutsideCoverage),
		stderrors.Is(err, lglnimport.ErrTooManyTiles),
		stderrors.Is(err, context.Canceled):
		return http.StatusBadRequest
	case isLGLNUpstreamError(err):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// isLGLNUpstreamError reports a failure that lies with the LGLN service
// rather than with the request or with this server.
func isLGLNUpstreamError(err error) bool {
	return stderrors.Is(err, lglnimport.ErrUnavailable) ||
		stderrors.Is(err, lglnimport.ErrHostNotAllowed) ||
		stderrors.Is(err, lglnimport.ErrTileTooLarge) ||
		stderrors.Is(err, lglnimport.ErrInvalidTile) ||
		stderrors.Is(err, context.DeadlineExceeded)
}

// lglnAPIError turns a failed LGLN import into an envelope. Too many tiles and
// a box outside Lower Saxony are the caller's to fix; everything the service
// did wrong is lgln_unavailable; a tile that does not parse is ours.
func lglnAPIError(err error) apiError {
	var tooMany *lglnimport.TooManyTilesError

	switch {
	case stderrors.As(err, &tooMany):
		return apiError{
			Code:    errorCodeLGLNTooManyTiles,
			Message: err.Error(),
			Details: map[string]any{"found_at_least": tooMany.Found, "max": tooMany.Max},
			Hint:    "Choose a smaller bounding box: one request covers at most nine 1 km tiles.",
		}
	case stderrors.Is(err, lglnimport.ErrOutsideCoverage):
		return apiError{
			Code:    errorCodeBadRequest,
			Message: err.Error(),
			Hint:    "LGLN publishes LoD2 buildings for Lower Saxony only.",
		}
	case stderrors.Is(err, lglnimport.ErrInvalidBBox):
		return apiError{Code: errorCodeBadRequest, Message: err.Error()}
	case stderrors.Is(err, context.Canceled):
		return apiError{Code: errorCodeBadRequest, Message: "request cancelled"}
	case isLGLNUpstreamError(err):
		return apiError{
			Code:    errorCodeLGLNUnavailable,
			Message: err.Error(),
			Hint:    "The LGLN download service could not be used. Check the network connection, or try again later.",
		}
	default:
		return apiError{Code: errorCodeInternalError, Message: err.Error()}
	}
}
