package httpv1

import "testing"

// artifactContentType is what a browser and a generated client both believe.
// The default used to be application/json for everything that was not HTML or
// Markdown, which made the endpoint lie about two artifacts it already served:
// a raster .bin is a headerless float64 array and a report .pdf is a PDF.
func TestArtifactContentTypeFollowsTheFormat(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		".noise/runs/r1/results/rls19-road.json":     "application/json; charset=utf-8",
		".noise/exports/e1/report.html":              "text/html; charset=utf-8",
		".noise/exports/e1/report.md":                "text/markdown; charset=utf-8",
		".noise/exports/e1/report.typ":               "text/plain; charset=utf-8",
		".noise/exports/e1/report.pdf":               "application/pdf",
		".noise/runs/r1/results/receivers.csv":       "text/csv; charset=utf-8",
		".noise/runs/r1/results/rls19-road.bin":      "application/octet-stream",
		".noise/exports/e1/formats/raster_LrDay.tif": "image/tiff",
		".noise/exports/e1/formats/contours.geojson": "application/geo+json",
		".noise/exports/e1/formats/receivers.gpkg":   "application/geopackage+sqlite3",
		".noise/exports/e1/formats/x.unknown-suffix": "application/octet-stream",
	}

	for path, want := range cases {
		if got := artifactContentType(path); got != want {
			t.Errorf("%s: got %q, want %q", path, got, want)
		}
	}

	// `.cog.tif` has to beat `.tif`, which is why the match is longest-first
	// rather than first-wins over the table's order.
	if got := artifactContentType(".noise/exports/e1/formats/raster_LrDay.cog.tif"); got != "image/tiff" {
		t.Errorf("cog: got %q, want image/tiff", got)
	}

	// Case is not part of the answer: an import can produce .TIF.
	if got := artifactContentType("/tmp/RASTER.TIF"); got != "image/tiff" {
		t.Errorf("uppercase: got %q, want image/tiff", got)
	}
}

// The spec is hand-built, so nothing but a test keeps it honest about what the
// endpoint actually sends.
func TestOpenAPIDeclaresTheBinaryArtifactMediaTypes(t *testing.T) {
	t.Parallel()

	payload := BuildOpenAPISpec("http://127.0.0.1:8080")

	paths, _ := payload["paths"].(map[string]any)
	artifact, _ := paths["/api/v1/artifacts/{id}/content"].(map[string]any)
	get, _ := artifact["get"].(map[string]any)
	responses, _ := get["responses"].(map[string]any)
	ok, _ := responses["200"].(map[string]any)
	content, _ := ok["content"].(map[string]any)

	for _, mediaType := range []string{
		"application/json",
		"application/geo+json",
		"image/tiff",
		"application/geopackage+sqlite3",
		"application/pdf",
		"application/octet-stream",
	} {
		if _, declared := content[mediaType]; !declared {
			t.Errorf("openapi does not declare %s for the artifact content endpoint", mediaType)
		}
	}
}
