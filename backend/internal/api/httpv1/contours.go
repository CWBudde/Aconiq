package httpv1

import (
	stderrors "errors"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/report/contour"
	"github.com/aconiq/backend/internal/report/results"
)

// defaultContourCRS answers a request that named no CRS.
//
// WGS84 rather than the raster's own CRS because the caller that reaches for
// this endpoint is drawing a map, and a client that ignores the declared `crs`
// would place metres as if they were degrees. The default is the safer guess,
// not a licence to guess: the response says what the coordinates are in either
// way, which is the whole reason for the envelope below.
const defaultContourCRS = "EPSG:4326"

// contourIntervalParam names the dB step in the query string and in the error
// details that quote it back, so a client reading the refusal is told which
// parameter to fix under the name it sent.
const contourIntervalParam = "interval"

// contoursResponse is the body of GET /api/v1/runs/{id}/contours, and
// contourLineResponse is one contour in it.
//
// They are declared here rather than reused from internal/report/contour for
// the reason transform.go gives: httpv1 owns its wire contract, and a type
// belonging to a package with two other consumers — `aconiq export` and the
// WebAssembly kernel — must not be able to move an HTTP contract clients depend
// on as a side effect of serving one of them.
// TestContourDTOsMatchTheContourPackageContract is what stops the two drifting
// while they are meant to agree.
//
// This is an envelope and not a bare `application/geo+json` FeatureCollection,
// although a FeatureCollection is what a map would consume most directly. RFC
// 7946 §4 fixes GeoJSON coordinates to WGS84 and removed the `crs` member that
// once let a document say otherwise, so a FeatureCollection cannot declare the
// CRS it is actually in — and this endpoint answers in whichever CRS was asked
// for. An envelope that carries `crs` beside the lines can. It also keeps the
// response JSON, which the frontend's request helper parses every response as.
type contoursResponse struct {
	// CRS is always populated, and names the coordinates actually returned
	// rather than the string that was asked for.
	CRS string `json:"crs"`
	// Interval echoes the dB step the contours were generated at, which the
	// caller may have omitted and taken the default for; a legend that names
	// the step has to know which step it got.
	Interval float64               `json:"interval"`
	Lines    []contourLineResponse `json:"lines"`
}

type contourLineResponse struct {
	Level float64 `json:"level"`
	// BandName is how a caller tells the bands apart: one request returns every
	// band the raster carries, and a client showing one filters rather than
	// asking again.
	BandName string       `json:"band_name"`
	Points   [][2]float64 `json:"points"`
}

// handleRunContours contours a finished run's result raster.
//
// Go 1.22 routing gives this pattern precedence over /api/v1/runs/{id}, the
// same way /api/v1/runs/{id}/log has it, so registering it shadows nothing.
//
// The marching squares themselves live in internal/report/contour, which
// `aconiq export --format contour-geojson` and the WebAssembly kernel also
// call. Where a 55 dB line falls is read as an assessment, so this handler
// contributes no geometry of its own: it resolves the raster, hands it over,
// and reports what comes back.
func (h Handler) handleRunContours(w http.ResponseWriter, r *http.Request) {
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

	interval, targetCRS, ok := parseContourQuery(w, r)
	if !ok {
		return
	}

	proj, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)

		return
	}

	rasterPath, found := runRasterMetadataPath(proj, runID)
	if !found {
		writeMissingContourSource(w, proj, runID)

		return
	}

	raster, err := loadRunRaster(h.store.Root(), rasterPath)
	if err != nil {
		writeProjectFileError(w, err, "failed to read this run's result raster")

		return
	}

	result, err := contour.FromRaster(raster, contour.Options{Interval: interval}, targetCRS)
	if err != nil {
		writeContourRefusal(w, err)

		return
	}

	writeJSON(w, http.StatusOK, contoursResponse{
		CRS:      result.CRS,
		Interval: result.Interval,
		Lines:    contourLinesResponse(result.Lines),
	})
}

// parseContourQuery reads the two query parameters, applying the defaults for
// the ones that were left out. It reports whether the caller may continue;
// every refusal has already written an envelope.
func parseContourQuery(w http.ResponseWriter, r *http.Request) (float64, string, bool) {
	interval := contour.DefaultInterval

	if raw := strings.TrimSpace(r.URL.Query().Get(contourIntervalParam)); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		// ParseFloat accepts "NaN" and "Inf", and neither compares false against
		// `<= 0`, so both would otherwise reach the level loop and never
		// terminate it.
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed <= 0 {
			writeAPIError(w, http.StatusBadRequest, apiError{
				Code:    errorCodeBadRequest,
				Message: "interval must be a positive number of decibels",
				Details: map[string]any{contourIntervalParam: raw},
				Hint: fmt.Sprintf(
					"Send a positive dB step such as `interval=2.5`, or omit interval to take the default %g dB.",
					contour.DefaultInterval),
			})

			return 0, "", false
		}

		interval = parsed
	}

	requested := strings.TrimSpace(r.URL.Query().Get("crs"))
	if requested == "" {
		requested = defaultContourCRS
	}

	// Parsing here, rather than leaving the whole string to contour.FromRaster,
	// keeps a malformed query parameter a bad request in this package's own
	// words — the same words handleModelGet answers `?crs=` with. What FromRaster
	// still judges is whether contours can be moved into the CRS at all, which
	// is its call and not this handler's.
	parsed, err := geo.ParseCRS(requested)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "crs is not a recognised CRS identifier: " + err.Error(),
			Hint: "Use an EPSG identifier such as `EPSG:25832`, or omit crs to receive the contours in " +
				defaultContourCRS + ".",
		})

		return 0, "", false
	}

	return interval, parsed.ID, true
}

// runRasterMetadataPath finds the manifest-declared sidecar of a run's result
// raster. The binary is not looked for: results.LoadRaster resolves it from the
// sidecar's own `data_file`, so a manifest that lists one and not the other
// still loads, and a manifest that lists neither is the absence reported here.
func runRasterMetadataPath(proj project.Project, runID string) (string, bool) {
	for _, artifact := range proj.Artifacts {
		if artifact.RunID == runID && artifact.Kind == project.ArtifactKindRunResultRasterMetadata {
			return artifact.Path, true
		}
	}

	return "", false
}

// writeMissingContourSource separates a run that does not exist from one that
// exists and produced no raster.
//
// Collapsing the two into `not_found` would tell a client that its run id was
// wrong when the run is sitting in the manifest, which is the same confusion
// errorCodeModelNotFound exists to avoid beside errorCodeNotFound: the two want
// different remedies, and only one of them is "check the id".
func writeMissingContourSource(w http.ResponseWriter, proj project.Project, runID string) {
	runExists := slices.ContainsFunc(proj.Runs, func(run project.Run) bool {
		return run.ID == runID
	})

	if !runExists {
		writeAPIError(w, http.StatusNotFound, apiError{
			Code:    errorCodeNotFound,
			Message: fmt.Sprintf("run %q not found", runID),
		})

		return
	}

	writeAPIError(w, http.StatusNotFound, apiError{
		Code:    errorCodeRunHasNoRaster,
		Message: fmt.Sprintf("run %q recorded no result raster, so there is nothing to contour", runID),
		Details: map[string]any{"run_id": runID},
		Hint: "Contours come from a grid: re-run the scenario with an auto-grid receiver mode, which is what " +
			"produces a raster. A run over individually placed receivers records a receiver table only.",
	})
}

// loadRunRaster resolves a manifest-declared raster sidecar inside the project
// root and loads it.
//
// The containment check is here and not inside results.LoadRaster because that
// function is the CLI's loader too, and the CLI hands it paths it built itself.
// Over HTTP the path arrives from .noise/project.json, which travels with a
// project a user may have received from someone else, so it is untrusted input
// for exactly the reason readProjectFile spells out. LoadRaster reads with a
// plain os.ReadFile, so nothing downstream would refuse an absolute path or a
// ".." escape; filepath.IsLocal refuses both before the join.
//
// This stops short of readProjectFile's os.OpenInRoot, which also defeats a
// symlink planted inside the project, because LoadRaster opens two files by
// path and takes no root. That is the gap to close if this endpoint ever grows
// past a trusted local project directory.
func loadRunRaster(root string, relPath string) (*results.Raster, error) {
	rel := filepath.FromSlash(strings.ReplaceAll(relPath, `\`, "/"))
	if !filepath.IsLocal(rel) {
		return nil, fmt.Errorf("%w: %s", errPathOutsideProject, relPath)
	}

	raster, err := results.LoadRaster(filepath.Join(root, rel))
	if err != nil {
		return nil, fmt.Errorf("load run raster %s: %w", rel, err)
	}

	return raster, nil
}

// writeContourRefusal answers a refusal from contour.FromRaster.
//
// The message is the package's own text, verbatim: its refusals already name
// the remedy, and this package restating them would make two answers to one
// question. What is decided here is only the status and the code, and the two
// causes genuinely differ.
//
// A raster with no georeference is the *resource*, not the request: the run
// was computed over receivers somebody placed individually, so no interval and
// no CRS would make this call succeed, and repeating it is pointless. That is
// a 409, the shape `run_not_finished` already uses.
//
// A CRS with no EPSG code is the *request*: send a different `crs` and the same
// run answers. It reuses `crs_not_projectable`, which POST /api/v1/transform
// already answers with for the same cause.
//
// They are told apart by sentinel and never by message text, which would break
// silently the first time the wording improved.
func writeContourRefusal(w http.ResponseWriter, err error) {
	switch {
	case stderrors.Is(err, contour.ErrNotAGrid):
		writeAPIError(w, http.StatusConflict, apiError{
			Code:    errorCodeRunHasNoGrid,
			Message: err.Error(),
			Hint:    "Re-run the scenario with `--receiver-mode auto-grid` to produce a grid contours can be drawn from.",
		})
	case stderrors.Is(err, contour.ErrCRSNotTransformable):
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeCRSNotProjectable,
			Message: err.Error(),
		})
	default:
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: err.Error(),
		})
	}
}

// contourLinesResponse always returns a list, never nil, so a run whose raster
// holds no contour at any level serialises as `[]` rather than `null`.
func contourLinesResponse(lines []contour.Line) []contourLineResponse {
	out := make([]contourLineResponse, 0, len(lines))
	for _, line := range lines {
		out = append(out, contourLineResponse{
			Level:    line.Level,
			BandName: line.BandName,
			Points:   line.Points,
		})
	}

	return out
}
