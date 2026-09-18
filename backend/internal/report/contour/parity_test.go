package contour

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/qa/golden"
	"github.com/aconiq/backend/internal/report/results"
)

// The cross-target contract of FromRaster.
//
// testdata/contour-parity/ is read from *two* trees: this test writes it, and
// frontend/src/wasm/contours.parity.test.ts reads it to check that the WASM
// kernel — which is what `window.aconiq.contours` and, through the same
// function, GET /api/v1/runs/{id}/contours answer with — traces the same lines
// this build does. Moving, renaming or reshaping anything under
// testdata/contour-parity/ breaks that test, which no Go tool will tell you
// about — grep frontend/ before you do.
//
// The claim the split-out package makes is that there is one marching-squares
// implementation and therefore one answer to where a 55 dB line falls. Until
// this fixture existed that was an intention: contours.test.ts drove the
// kernel's argument order and bounded the vertices by a bounding box, and
// nothing anywhere compared a vertex against what Go computes. A kernel built
// from a stale tree, or a JS-side reimplementation slipped in behind the same
// entry point, would have passed everything.
//
// Three goldens, all load-bearing:
//
//   - input.golden.json is the TypeScript side's *input*: the sidecar a run
//     would have written, the cell values behind it in receiver order, and the
//     request options. It is what lets the kernel be asked the same question
//     rather than a similar one.
//   - raster.golden.bin is that grid as results.SaveRaster writes it. The
//     frontend reads these bytes rather than encoding its own, because a second
//     writer of the byte contract is exactly what raster-bin.ts exists to
//     prevent.
//   - contours.golden.json is the expectation: the Result, vertices rounded to
//     1e-6.
//
// Regenerate all three with `just update-golden`.

// contourParityRequest is wasmkernel.ContourRequest minus the raster: the
// options both boundaries take, spelled in the wire's own snake_case so the
// frontend can hand the object to the kernel as it reads it.
type contourParityRequest struct {
	TargetCRS string  `json:"target_crs"`
	Interval  float64 `json:"interval"`
}

// contourParityInput is everything needed to put the identical question to the
// kernel: what the raster is, what is in it, and what to trace.
type contourParityInput struct {
	Metadata results.RasterMetadata `json:"metadata"`
	// Bands[b][i] is band b's value at cell index i, row-major from the
	// south-west corner — the order results.SaveRaster writes and
	// results.DecodeRaster reads back.
	Bands   [][]float64          `json:"bands"`
	Request contourParityRequest `json:"request"`
}

// contourParityFixture is shaped so that each way the two targets could drift
// changes the golden rather than hiding in it:
//
//   - **Two bands whose levels cannot overlap.** LrDay never drops below 52 dB
//     and LrNight never reaches 48, so a band mix-up does not merely relabel a
//     line — it attaches a name to a level that band cannot produce, and
//     TestContourParityFixture asserts the separation so a later edit cannot
//     quietly remove it.
//   - **A north-south gradient, not just a west-east one**, and a nodata cell
//     off the centre line. Row 0 is the southernmost row and the tracer flips
//     it against the geo-transform; a fixture symmetric about its horizontal
//     axis would trace identically upside down. See
//     TestContourParityFixtureCanSeeARowFlip.
//   - **A metric CRS traced into WGS84.** The request asks for EPSG:4326 over
//     an EPSG:25832 raster, so a skipped or misdirected reprojection lands the
//     lines six orders of magnitude away rather than producing numbers that
//     happen to agree.
//   - **A 4 dB interval, which is not DefaultInterval.** A target that ignored
//     the field and took the 5 dB default would trace a different set of levels
//     entirely.
//
// 5 wide by 4 high keeps a width/height swap visible too, as it is in
// results.TestRasterBinaryParityFixture, which owns the byte layout itself.
// Every value is a multiple of 0.5, so the grid crosses the WASM boundary and
// both languages' JSON with no rounding of its own.
func contourParityFixture() contourParityInput {
	const nodata = -9999.0

	return contourParityInput{
		Metadata: results.RasterMetadata{
			Width:     5,
			Height:    4,
			Bands:     2,
			NoData:    nodata,
			Unit:      "dB(A)",
			BandNames: []string{"LrDay", "LrNight"},
			CRS:       "EPSG:25832",
			Geo: &results.Georeference{
				// A real site in UTM zone 32 — the reprojection has to land
				// somewhere a reader can check, and a half-metre origin keeps
				// the half-pixel corner conversion from being invisible.
				OriginX:    673_250.5,
				OriginY:    5_647_100.25,
				PixelSizeM: 25,
				RowOrder:   results.RowOrderSouthUp,
			},
		},
		Bands: [][]float64{
			// LrDay, south row first: rising north (≈ +5 dB per row) faster
			// than east (≈ +1.5 dB per cell), so the lines run roughly
			// west-east and a row flip moves every vertex.
			{
				52.0, 53.5, 55.0, 56.5, 58.0,
				57.0, 59.0, 61.5, 63.0, 64.5,
				63.5, 66.0, 68.0, 69.5, 70.5,
				66.0, 69.0, 72.0, 74.0, 75.0,
			},
			// LrNight, a whole band below LrDay, with one nodata cell. The
			// hole is off-centre in both axes and it is load-bearing: the
			// three lines that reach it stop there, several vertices short of
			// the east edge they would otherwise run to. A tracer that read
			// -9999 as an ordinary value would instead run every level
			// through that cell.
			{
				30.0, 31.5, 33.0, 34.5, 35.5,
				34.0, 36.0, 38.0, nodata, 40.5,
				38.5, 41.0, 43.0, 44.0, 45.0,
				41.0, 43.5, 45.5, 46.5, 47.0,
			},
		},
		Request: contourParityRequest{
			TargetCRS: "EPSG:4326",
			Interval:  4,
		},
	}
}

func TestContourParityFixture(t *testing.T) {
	t.Parallel()

	fixture := contourParityFixture()

	golden.AssertJSONSnapshot(t, "testdata/contour-parity/input.golden.json", fixture)

	raster := contourParityRaster(t, fixture)

	// Through SaveRaster rather than through a local encoder: these are the
	// bytes a run writes, and the frontend reads this file instead of
	// reconstructing the payload from input.golden.json's values.
	persistence, err := results.SaveRaster(filepath.Join(t.TempDir(), "raster"), raster)
	if err != nil {
		t.Fatalf("save raster: %v", err)
	}

	payload, err := os.ReadFile(persistence.DataPath)
	if err != nil {
		t.Fatalf("read raster binary: %v", err)
	}

	golden.AssertBytesSnapshot(t, "testdata/contour-parity/raster.golden.bin", payload)

	result, err := FromRaster(raster, Options{Interval: fixture.Request.Interval}, fixture.Request.TargetCRS)
	if err != nil {
		t.Fatalf("contours from raster: %v", err)
	}

	assertContourParityIsWorthPinning(t, result)

	golden.AssertJSONSnapshot(t, "testdata/contour-parity/contours.golden.json", roundContourResult(result))
}

// contourParityRaster fills a raster from the fixture's cell values.
func contourParityRaster(t *testing.T, fixture contourParityInput) *results.Raster {
	t.Helper()

	raster, err := results.NewRaster(fixture.Metadata)
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	for band, values := range fixture.Bands {
		for index, value := range values {
			err := raster.Set(index%fixture.Metadata.Width, index/fixture.Metadata.Width, band, value)
			if err != nil {
				t.Fatalf("set band %d index %d: %v", band, index, err)
			}
		}
	}

	return raster
}

// assertContourParityIsWorthPinning checks the properties the fixture was
// built to carry, so that a later edit to the numbers cannot leave a golden
// that still compares cleanly while testing nothing.
//
// A golden alone cannot do this: regenerating it makes any fixture agree with
// itself.
func assertContourParityIsWorthPinning(t *testing.T, result Result) {
	t.Helper()

	if result.CRS != "EPSG:4326" || result.Interval != 4 {
		t.Fatalf("result echoes crs %q interval %g, want EPSG:4326 / 4", result.CRS, result.Interval)
	}

	minDay := math.Inf(1)
	maxNight := math.Inf(-1)

	for _, line := range result.Lines {
		switch line.BandName {
		case "LrDay":
			minDay = math.Min(minDay, line.Level)
		case "LrNight":
			maxNight = math.Max(maxNight, line.Level)
		default:
			t.Fatalf("line at level %g carries band name %q, which is neither band", line.Level, line.BandName)
		}

		// Longitude and latitude near the site, not eastings and northings. A
		// reprojection that never ran would leave the vertices in the 10^5/10^6
		// range the georeference places them in.
		for _, point := range line.Points {
			if point[0] < 10 || point[0] > 12 || point[1] < 50 || point[1] > 52 {
				t.Fatalf("vertex %v of the %s line at %g is not lon/lat near the fixture's site", point, line.BandName, line.Level)
			}
		}
	}

	if math.IsInf(minDay, 1) || math.IsInf(maxNight, -1) {
		t.Fatalf("expected lines from both bands, got day-min %g night-max %g", minDay, maxNight)
	}

	if minDay <= maxNight {
		t.Fatalf("the bands' levels overlap (day from %g, night to %g): a band mix-up would no longer be visible", minDay, maxNight)
	}
}

// TestContourParityFixtureCanSeeARowFlip guards the second property directly:
// row 0 is the southernmost row, and the tracer flips it against the
// geo-transform, so a fixture that reads the same upside down would pin nothing
// about row order.
func TestContourParityFixtureCanSeeARowFlip(t *testing.T) {
	t.Parallel()

	fixture := contourParityFixture()
	width, height := fixture.Metadata.Width, fixture.Metadata.Height

	for band, values := range fixture.Bands {
		mirrored := false

		for y := range height {
			for x := range width {
				if values[y*width+x] != values[(height-1-y)*width+x] {
					mirrored = true
				}
			}
		}

		if !mirrored {
			t.Fatalf("band %d is symmetric about its horizontal axis, so a row flip would go unnoticed", band)
		}
	}
}

// roundContourResult rounds vertices to 1e-6 for the snapshot.
//
// It is the goldens' precision, not a tolerance — the same 1e-6 the CLI parity
// goldens are written at. Both targets run this identical code, so they should
// agree bit for bit; they are nonetheless separate builds of it, and Go's math
// library is assembly on amd64 and portable Go under GOARCH=wasm, so the
// transverse-Mercator series behind the reprojection may land an ulp apart.
// Rounding what is written means the frontend can compare against a stated
// number rather than against a float64 it has to reproduce exactly.
func roundContourResult(result Result) Result {
	rounded := result
	rounded.Lines = make([]Line, len(result.Lines))

	for i, line := range result.Lines {
		moved := line
		moved.Points = make([][2]float64, len(line.Points))

		for j, point := range line.Points {
			moved.Points[j] = [2]float64{contourParityRound6(point[0]), contourParityRound6(point[1])}
		}

		rounded.Lines[i] = moved
	}

	return rounded
}

func contourParityRound6(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}
