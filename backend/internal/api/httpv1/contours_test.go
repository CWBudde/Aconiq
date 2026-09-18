package httpv1

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/report/contour"
	"github.com/aconiq/backend/internal/report/results"
)

// The fixture raster is deliberately larger than the 2x2 marching squares
// needs, and ramps west to east, so that every level between its ends really
// does cross a cell: a flat raster produces an empty answer that would pass a
// test asserting only the envelope. Two bands, because one request returns all
// of them and a single-band fixture cannot show that.
const (
	contourFixtureWidth  = 4
	contourFixtureHeight = 3
	contourFixtureOrigin = 500000.0
	contourFixtureNorth  = 5700000.0
	contourFixturePixelM = 10.0
	contourFixtureCRS    = "EPSG:25832"
)

// seedRunWithRaster gives a run the result raster a grid receiver mode would
// have written, and the manifest refs that point at it.
func seedRunWithRaster(t *testing.T, store projectfs.Store, georef *results.Georeference) project.Run {
	t.Helper()

	run := seedRun(t, store, project.RunStatusCompleted)

	raster, err := results.NewRaster(results.RasterMetadata{
		Width: contourFixtureWidth, Height: contourFixtureHeight, Bands: 2,
		NoData: -9999, Unit: "dB", BandNames: []string{"Lden", "Lnight"},
		CRS: contourFixtureCRS, Geo: georef,
	})
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	for band := range 2 {
		for y := range contourFixtureHeight {
			for x := range contourFixtureWidth {
				setErr := raster.Set(x, y, band, 50+float64(band)*10+float64(x)*4)
				if setErr != nil {
					t.Fatalf("set raster cell: %v", setErr)
				}
			}
		}
	}

	relBase := path.Join(".noise", "runs", run.ID, "results", "grid")

	_, err = results.SaveRaster(filepath.Join(store.Root(), filepath.FromSlash(relBase)), raster)
	if err != nil {
		t.Fatalf("save raster: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	proj.Artifacts = append(proj.Artifacts,
		project.ArtifactRef{
			ID: run.ID + "-raster-meta", RunID: run.ID,
			Kind: project.ArtifactKindRunResultRasterMetadata, Path: relBase + ".json",
		},
		project.ArtifactRef{
			ID: run.ID + "-raster-bin", RunID: run.ID,
			Kind: project.ArtifactKindRunResultRasterBinary, Path: relBase + ".bin",
		},
	)

	err = store.Save(proj)
	if err != nil {
		t.Fatalf("save project: %v", err)
	}

	return run
}

func contourFixtureGeoreference() *results.Georeference {
	return &results.Georeference{
		OriginX:    contourFixtureOrigin,
		OriginY:    contourFixtureNorth,
		PixelSizeM: contourFixturePixelM,
		RowOrder:   results.RowOrderSouthUp,
	}
}

func getContours(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAPIRequest(http.MethodGet, target, nil))

	return rec
}

func decodeContours(t *testing.T, rec *httptest.ResponseRecorder) contoursResponse {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp contoursResponse

	decodeResponse(t, rec.Body.Bytes(), &resp)

	return resp
}

// The happy path has to answer with lines that say where they are: a client
// handed bare coordinates guesses WGS84, and the default here really is WGS84,
// so the declaration is the only thing that tells a caller apart from a lucky
// guess when it asks for something else.
func TestRunContoursEndpointReturnsLinesInWGS84ByDefault(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Contours API")
	run := seedRunWithRaster(t, store, contourFixtureGeoreference())
	handler := NewHandler(store, nil)

	resp := decodeContours(t, getContours(t, handler, "/api/v1/runs/"+run.ID+"/contours"))

	if resp.CRS != defaultContourCRS {
		t.Errorf("crs = %q, want %q", resp.CRS, defaultContourCRS)
	}

	if resp.Interval != contour.DefaultInterval {
		t.Errorf("interval = %v, want %v", resp.Interval, contour.DefaultInterval)
	}

	if len(resp.Lines) == 0 {
		t.Fatal("expected contour lines from a ramped raster, got none")
	}

	bands := map[string]bool{}

	for _, line := range resp.Lines {
		bands[line.BandName] = true

		if len(line.Points) < 2 {
			t.Fatalf("contour at %v dB has %d points", line.Level, len(line.Points))
		}

		// Degrees, not metres: the fixture raster is in EPSG:25832, so an answer
		// that skipped the reprojection would carry six-figure eastings here.
		for _, point := range line.Points {
			if math.Abs(point[0]) > 180 || math.Abs(point[1]) > 90 {
				t.Fatalf("point %v is not a WGS84 coordinate", point)
			}
		}
	}

	for _, band := range []string{"Lden", "Lnight"} {
		if !bands[band] {
			t.Errorf("band %q is missing: one request must return every band", band)
		}
	}
}

// A finer interval has to reach the generator, not just the echo: a response
// that reported 2.5 dB while drawing 5 dB lines would put a legend and a map
// out of step with each other.
func TestRunContoursEndpointHonoursTheRequestedInterval(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Contours Interval")
	run := seedRunWithRaster(t, store, contourFixtureGeoreference())
	handler := NewHandler(store, nil)

	base := decodeContours(t, getContours(t, handler, "/api/v1/runs/"+run.ID+"/contours"))
	fine := decodeContours(t, getContours(t, handler, "/api/v1/runs/"+run.ID+"/contours?interval=2.5"))

	if fine.Interval != 2.5 {
		t.Errorf("interval = %v, want 2.5", fine.Interval)
	}

	if len(fine.Lines) <= len(base.Lines) {
		t.Errorf("halving the interval produced %d lines, not more than the default's %d",
			len(fine.Lines), len(base.Lines))
	}
}

// Asking for the raster's own CRS must return the raster's own coordinates,
// untouched rather than round-tripped through WGS84 and back.
func TestRunContoursEndpointHonoursTheRequestedCRS(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Contours CRS")
	run := seedRunWithRaster(t, store, contourFixtureGeoreference())
	handler := NewHandler(store, nil)

	resp := decodeContours(t, getContours(t,
		handler, "/api/v1/runs/"+run.ID+"/contours?crs="+contourFixtureCRS))

	if resp.CRS != contourFixtureCRS {
		t.Fatalf("crs = %q, want %q", resp.CRS, contourFixtureCRS)
	}

	spanM := contourFixtureWidth * contourFixturePixelM

	for _, line := range resp.Lines {
		for _, point := range line.Points {
			if point[0] < contourFixtureOrigin || point[0] > contourFixtureOrigin+spanM {
				t.Fatalf("easting %v falls outside the fixture raster's %v m span", point[0], spanM)
			}
		}
	}
}

func TestRunContoursEndpointRejectsAnUnknownRun(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Contours Unknown Run")
	seedRunWithRaster(t, store, contourFixtureGeoreference())

	rec := getContours(t, NewHandler(store, nil), "/api/v1/runs/no-such-run/contours")

	assertErrorCode(t, rec, http.StatusNotFound, errorCodeNotFound)
}

// A run that exists but was computed over individually placed receivers is not
// a missing run, and must not be reported as one: the id is right, the receiver
// mode is what has to change.
func TestRunContoursEndpointSeparatesAMissingRasterFromAMissingRun(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Contours No Raster")
	run := seedRun(t, store, project.RunStatusCompleted)

	rec := getContours(t, NewHandler(store, nil), "/api/v1/runs/"+run.ID+"/contours")

	assertErrorCode(t, rec, http.StatusNotFound, errorCodeRunHasNoRaster)

	var response errorResponse

	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Hint == "" {
		t.Error("the refusal must say which receiver mode produces a raster")
	}
}

// A raster that records no georeference is refused in the contour package's own
// words, because the same refusal reaches a reader through `aconiq export` and
// the browser kernel.
//
// A conflict, not a bad request: the run was computed over receivers somebody
// placed individually, so no interval and no CRS would make this call succeed
// and repeating it is pointless. It is told apart from the CRS refusal by
// `contour.ErrNotAGrid`, never by matching the message.
func TestRunContoursEndpointRefusesARasterThatIsNotAGrid(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Contours No Georeference")
	run := seedRunWithRaster(t, store, nil)

	rec := getContours(t, NewHandler(store, nil), "/api/v1/runs/"+run.ID+"/contours")

	assertErrorCode(t, rec, http.StatusConflict, errorCodeRunHasNoGrid)

	var response errorResponse
	decodeResponse(t, rec.Body.Bytes(), &response)

	if response.Error.Hint == "" {
		t.Error("the refusal must name the receiver mode that produces a grid")
	}
}

// And a CRS with no EPSG code is the other way round: the request is what is
// wrong, a different `crs` on the same run answers, and it reuses the code
// POST /api/v1/transform already gives for the same cause.
func TestRunContoursEndpointRefusesATargetCRSItCannotTransformInto(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Contours Untransformable Target")
	run := seedRunWithRaster(t, store, contourFixtureGeoreference())

	rec := getContours(t, NewHandler(store, nil),
		"/api/v1/runs/"+run.ID+"/contours?crs=WKT:LOCAL_CS%5B%22site%22%5D")

	assertErrorCode(t, rec, http.StatusBadRequest, errorCodeCRSNotProjectable)
}

func TestRunContoursEndpointRejectsAnUnusableInterval(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Contours Bad Interval")
	run := seedRunWithRaster(t, store, contourFixtureGeoreference())
	handler := NewHandler(store, nil)

	// NaN and Inf parse as floats but would never terminate the level loop, so
	// they belong in the same refusal as text and zero.
	for _, query := range []string{"abc", "0", "-5", "NaN", "Inf"} {
		rec := getContours(t, handler, "/api/v1/runs/"+run.ID+"/contours?interval="+query)

		assertErrorCode(t, rec, http.StatusBadRequest, errorCodeBadRequest)
	}
}

func TestRunContoursEndpointRejectsAnUnparseableCRS(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Contours Bad CRS")
	run := seedRunWithRaster(t, store, contourFixtureGeoreference())

	rec := getContours(t, NewHandler(store, nil), "/api/v1/runs/"+run.ID+"/contours?crs=nonsense")

	assertErrorCode(t, rec, http.StatusBadRequest, errorCodeBadRequest)
}

func TestRunContoursEndpointRejectsANonGETMethod(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Contours Method")
	run := seedRunWithRaster(t, store, contourFixtureGeoreference())

	rec := httptest.NewRecorder()
	NewHandler(store, nil).ServeHTTP(rec, newAPIRequest(http.MethodPost, "/api/v1/runs/"+run.ID+"/contours", nil))

	assertErrorCode(t, rec, http.StatusMethodNotAllowed, errorCodeMethodNotAllowed)
}

// httpv1 declares its own DTOs so that the contour package cannot move an HTTP
// contract as a side effect of serving the exporter or the browser kernel. They
// are meant to agree today, though, and nothing but this test says so.
func TestContourDTOsMatchTheContourPackageContract(t *testing.T) {
	t.Parallel()

	points := [][2]float64{{9.0, 51.0}, {9.01, 51.01}}

	apiPayload, err := json.Marshal(contoursResponse{
		CRS:      "EPSG:4326",
		Interval: 5,
		Lines:    []contourLineResponse{{Level: 55, BandName: "Lden", Points: points}},
	})
	if err != nil {
		t.Fatalf("marshal API response: %v", err)
	}

	packagePayload, err := json.Marshal(contour.Result{
		CRS:      "EPSG:4326",
		Interval: 5,
		Lines:    []contour.Line{{Level: 55, BandName: "Lden", Points: points}},
	})
	if err != nil {
		t.Fatalf("marshal contour result: %v", err)
	}

	if string(apiPayload) != string(packagePayload) {
		t.Errorf("response shapes differ:\n api:     %s\n package: %s", apiPayload, packagePayload)
	}
}

func TestOpenAPIDocumentsRunContours(t *testing.T) {
	t.Parallel()

	spec := BuildOpenAPISpec("")

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("expected paths object")
	}

	item, ok := paths["/api/v1/runs/{id}/contours"].(map[string]any)
	if !ok {
		t.Fatal("expected /api/v1/runs/{id}/contours path item")
	}

	if _, exists := item["get"]; !exists {
		t.Fatal("expected a get operation on /api/v1/runs/{id}/contours")
	}

	components, ok := spec["components"].(map[string]any)
	if !ok {
		t.Fatal("expected components object")
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("expected schemas object")
	}

	for _, name := range []string{"ContourResult", "ContourLine"} {
		if _, exists := schemas[name]; !exists {
			t.Errorf("expected %s schema in the openapi document", name)
		}
	}
}
