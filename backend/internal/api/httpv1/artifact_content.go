package httpv1

import (
	"fmt"
	"net/http"
	"strings"
)

func (h Handler) handleArtifactContent(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	artifactID := r.PathValue("id")
	if artifactID == "" {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    errorCodeBadRequest,
			Message: "artifact id is required",
		})

		return
	}

	proj, err := h.store.Load()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	var artifactPath string

	for _, a := range proj.Artifacts {
		if a.ID == artifactID {
			artifactPath = a.Path
			break
		}
	}

	if artifactPath == "" {
		writeAPIError(w, http.StatusNotFound, apiError{
			Code:    errorCodeNotFound,
			Message: fmt.Sprintf("artifact %q not found", artifactID),
		})

		return
	}

	raw, readErr := readProjectFile(h.store.Root(), artifactPath)
	if readErr != nil {
		writeProjectFileError(w, readErr, "failed to read artifact file")
		return
	}

	w.Header().Set("Content-Type", artifactContentType(artifactPath))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

// artifactContentTypes maps an artifact's extension to what it actually is.
//
// The default used to be `application/json` for everything but `.html` and
// `.md`, which made the endpoint lie about two artifacts it was already
// serving: `run.result.raster_binary` is a headerless float64 array and
// `export.report_pdf` is a PDF, both announced as JSON. A browser handed that
// header renders bytes as text; a fetch() that trusts it and calls .json()
// throws on the first byte.
//
// The extensions are matched longest-first, because `.cog.tif` has to beat
// `.tif` to the answer.
const (
	contentTypeTIFF   = "image/tiff"
	contentTypeBinary = "application/octet-stream"
	contentTypeText   = "text/plain; charset=utf-8"
)

var artifactContentTypes = []struct {
	suffix      string
	contentType string
}{
	{".html", "text/html; charset=utf-8"},
	{".md", "text/markdown; charset=utf-8"},
	{".geojson", "application/geo+json"},
	{".json", "application/json; charset=utf-8"},
	{".csv", "text/csv; charset=utf-8"},
	{".typ", contentTypeText},
	{".log", contentTypeText},
	{".pdf", "application/pdf"},
	{".cog.tif", contentTypeTIFF},
	{".tif", contentTypeTIFF},
	{".tiff", contentTypeTIFF},
	{".gpkg", "application/geopackage+sqlite3"},
	{".bin", contentTypeBinary},
}

// artifactContentType answers for a path, falling back to octet-stream.
//
// Unknown is `application/octet-stream`, not JSON: an unrecognised artifact is
// far more likely to be the next binary format than the next JSON one, and
// bytes labelled octet-stream are downloaded rather than mis-parsed.
func artifactContentType(artifactPath string) string {
	lower := strings.ToLower(artifactPath)

	best := ""
	bestType := contentTypeBinary

	for _, candidate := range artifactContentTypes {
		if strings.HasSuffix(lower, candidate.suffix) && len(candidate.suffix) > len(best) {
			best = candidate.suffix
			bestType = candidate.contentType
		}
	}

	return bestType
}
