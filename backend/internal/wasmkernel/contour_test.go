package wasmkernel_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/report/contour"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/aconiq/backend/internal/wasmkernel"
)

const (
	contourWidth  = 8
	contourHeight = 8
	contourCRS    = "EPSG:25832"
)

// contourRasterMeta describes the fixture grid. georeferenced says whether it
// carries a georeference, because a raster without one is how the domain
// refusal below is provoked.
func contourRasterMeta(georeferenced bool) results.RasterMetadata {
	meta := results.RasterMetadata{
		Width:     contourWidth,
		Height:    contourHeight,
		Bands:     2,
		NoData:    -999,
		Unit:      "dB(A)",
		BandNames: []string{"lr_day", "lr_night"},
		CRS:       contourCRS,
	}

	if georeferenced {
		meta.Geo = &results.Georeference{
			OriginX:    681_000,
			OriginY:    5_646_000,
			PixelSizeM: 10,
			RowOrder:   results.RowOrderSouthUp,
		}
	}

	return meta
}

// contourRaster fills the fixture with a plane sloping across both axes, so
// every 5 dB level in the range crosses the grid and each band traces at a
// different set of levels.
func contourRaster(t *testing.T, meta results.RasterMetadata) *results.Raster {
	t.Helper()

	raster, err := results.NewRaster(meta)
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	for band := range meta.Bands {
		for y := range meta.Height {
			for x := range meta.Width {
				value := 40 + float64(x+y) - 10*float64(band)
				if err := raster.Set(x, y, band, value); err != nil {
					t.Fatalf("set (%d,%d,%d): %v", x, y, band, err)
				}
			}
		}
	}

	return raster
}

// contourPayload encodes the raster the way a run writes it, through
// results.SaveRaster, so the bytes this test hands the kernel are the bytes the
// browser reads back out of a run's `.bin` rather than a second encoding of the
// same contract.
func contourPayload(t *testing.T, raster *results.Raster) []byte {
	t.Helper()

	persisted, err := results.SaveRaster(filepath.Join(t.TempDir(), "grid"), raster)
	if err != nil {
		t.Fatalf("save raster: %v", err)
	}

	payload, err := os.ReadFile(persisted.DataPath)
	if err != nil {
		t.Fatalf("read raster payload: %v", err)
	}

	return payload
}

func contoursJSON(t *testing.T, payload []byte, req wasmkernel.ContourRequest) wasmkernel.ContourResult {
	t.Helper()

	in, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	out, err := wasmkernel.Contours(payload, in)
	if err != nil {
		t.Fatalf("contours: %v", err)
	}

	var result wasmkernel.ContourResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	return result
}

func contoursError(t *testing.T, payload []byte, req wasmkernel.ContourRequest) string {
	t.Helper()

	in, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, err := wasmkernel.Contours(payload, in); err != nil {
		return err.Error()
	}

	t.Fatal("Contours accepted a request it should have refused")

	return ""
}

// The whole entry point in one pass: sidecar metadata in the JSON, values in
// the bytes, lines for every band out the other side.
func TestContoursTraceEveryBand(t *testing.T) {
	t.Parallel()

	meta := contourRasterMeta(true)
	payload := contourPayload(t, contourRaster(t, meta))

	result := contoursJSON(t, payload, wasmkernel.ContourRequest{
		Raster:    meta,
		TargetCRS: contourCRS,
	})

	if result.CRS != contourCRS {
		t.Errorf("crs = %q, want %q echoed back", result.CRS, contourCRS)
	}

	// An omitted interval takes the EU END default, and the response says so:
	// a legend naming the step has to know which step it got.
	if result.Interval != wasmkernel.DefaultContourInterval {
		t.Errorf("interval = %v, want the %v default", result.Interval, wasmkernel.DefaultContourInterval)
	}

	bands := map[string]int{}
	for _, line := range result.Lines {
		bands[line.BandName]++
	}

	for _, want := range meta.BandNames {
		if bands[want] == 0 {
			t.Errorf("no lines for band %q; got %v", want, bands)
		}
	}

	// Target equal to the source CRS means no reprojection, so the vertices
	// must still be the metres the georeference places the grid on. A silent
	// fall-through to degrees would put them near zero.
	for _, line := range result.Lines {
		for _, point := range line.Points {
			if point[0] < 680_000 || point[0] > 682_000 || point[1] < 5_645_000 || point[1] > 5_647_000 {
				t.Fatalf("vertex %v is outside the fixture's extent", point)
			}
		}
	}
}

// A different target CRS is what the map actually asks for, and the response
// has to name the CRS the points are really in — not the one they came from.
func TestContoursReprojectToTheRequestedCRS(t *testing.T) {
	t.Parallel()

	meta := contourRasterMeta(true)
	payload := contourPayload(t, contourRaster(t, meta))

	result := contoursJSON(t, payload, wasmkernel.ContourRequest{
		Raster:    meta,
		TargetCRS: "EPSG:4326",
		Interval:  10,
	})

	if result.CRS != "EPSG:4326" {
		t.Errorf("crs = %q, want EPSG:4326", result.CRS)
	}

	if result.Interval != 10 {
		t.Errorf("interval = %v, want the requested 10", result.Interval)
	}

	if len(result.Lines) == 0 {
		t.Fatal("no contour lines at a 10 dB interval over a 14 dB range")
	}

	for _, line := range result.Lines {
		for _, point := range line.Points {
			if point[0] < 11 || point[0] > 12 || point[1] < 50 || point[1] > 52 {
				t.Fatalf("vertex %v is not the degrees the fixture's extent projects to", point)
			}
		}
	}
}

func TestContoursRefuseInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := wasmkernel.Contours(nil, []byte("not json"))
	if err == nil {
		t.Fatal("Contours accepted input that is not JSON")
	}

	if !strings.Contains(err.Error(), "invalid input JSON") {
		t.Errorf("refusal %q does not name the JSON boundary", err.Error())
	}
}

// The payload carries no header, so its length is the only thing that says it
// matches the sidecar it arrived with. A short one read against the declared
// shape would report cells from the wrong band.
func TestContoursRefuseAPayloadThatDoesNotMatchTheShape(t *testing.T) {
	t.Parallel()

	meta := contourRasterMeta(true)
	payload := contourPayload(t, contourRaster(t, meta))

	message := contoursError(t, payload[:len(payload)-8], wasmkernel.ContourRequest{
		Raster:    meta,
		TargetCRS: contourCRS,
	})

	if !strings.Contains(message, "raster binary size mismatch") {
		t.Errorf("refusal %q does not carry DecodeRaster's size refusal", message)
	}
}

// The refusal has to reach the browser in the generator's own words. `aconiq
// export` and the API print contour.FromRaster's sentence, and a reader
// comparing browser mode against a bundle must not be shown a second wording
// of the same failure.
func TestContoursRefusalsReachTheCallerVerbatim(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		meta      results.RasterMetadata
		targetCRS string
	}{
		{name: "no georeference", meta: contourRasterMeta(false), targetCRS: contourCRS},
		{name: "untransformable target", meta: contourRasterMeta(true), targetCRS: "WKT:local"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raster := contourRaster(t, tc.meta)
			payload := contourPayload(t, raster)

			_, want := contour.FromRaster(raster, contour.Options{}, tc.targetCRS)
			if want == nil {
				t.Fatal("contour.FromRaster accepted what this case exists to refuse")
			}

			got := contoursError(t, payload, wasmkernel.ContourRequest{
				Raster:    tc.meta,
				TargetCRS: tc.targetCRS,
			})

			if got != want.Error() {
				t.Errorf("kernel refused with %q, want contour.FromRaster's own %q", got, want.Error())
			}
		})
	}
}
