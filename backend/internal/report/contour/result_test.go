package contour

import (
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/report/results"
)

// gridRaster builds a small south-up raster with a declared georeference and a
// west-to-east ramp, so a contour at a known level has somewhere to fall.
func gridRaster(t *testing.T, crs string, georef *results.Georeference) *results.Raster {
	t.Helper()

	const (
		width  = 4
		height = 3
	)

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     width,
		Height:    height,
		Bands:     2,
		NoData:    -9999,
		Unit:      "dB",
		BandNames: []string{"LrDay", "LrNight"},
		CRS:       crs,
		Geo:       georef,
	})
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	for band := range 2 {
		for y := range height {
			for x := range width {
				// 50..65 across, and the night band 10 dB quieter, so the two
				// bands cannot be confused for one another.
				value := 50 + float64(x)*5 - float64(band)*10
				if err := raster.Set(x, y, band, value); err != nil {
					t.Fatalf("set cell: %v", err)
				}
			}
		}
	}

	return raster
}

func southUp() *results.Georeference {
	return &results.Georeference{
		OriginX:    700000,
		OriginY:    5700000,
		PixelSizeM: 25,
		RowOrder:   results.RowOrderSouthUp,
	}
}

func TestFromRasterReturnsEveryBandAndSaysWhereTheyAre(t *testing.T) {
	t.Parallel()

	raster := gridRaster(t, "EPSG:25832", southUp())

	result, err := FromRaster(raster, Options{Interval: 5}, "EPSG:25832")
	if err != nil {
		t.Fatalf("from raster: %v", err)
	}

	if result.CRS != "EPSG:25832" {
		t.Errorf("crs = %q, want EPSG:25832", result.CRS)
	}

	if result.Interval != 5 {
		t.Errorf("interval = %v, want 5", result.Interval)
	}

	// One call covers every band; a caller showing one filters rather than
	// asking again, so both names have to be present.
	bands := map[string]int{}
	for _, line := range result.Lines {
		bands[line.BandName]++
	}

	if bands["LrDay"] == 0 || bands["LrNight"] == 0 {
		t.Errorf("expected lines for both bands, got %v", bands)
	}
}

func TestFromRasterDefaultsTheIntervalAndEchoesIt(t *testing.T) {
	t.Parallel()

	raster := gridRaster(t, "EPSG:25832", southUp())

	result, err := FromRaster(raster, Options{}, "EPSG:25832")
	if err != nil {
		t.Fatalf("from raster: %v", err)
	}

	// Echoed rather than left at zero: a caller that omitted it still has to
	// be able to label the legend with the step it actually got.
	if result.Interval != DefaultInterval {
		t.Errorf("interval = %v, want the %v default", result.Interval, DefaultInterval)
	}
}

func TestFromRasterMovesLinesIntoTheRequestedCRS(t *testing.T) {
	t.Parallel()

	metric := gridRaster(t, "EPSG:25832", southUp())

	here, err := FromRaster(metric, Options{Interval: 5}, "EPSG:25832")
	if err != nil {
		t.Fatalf("from raster, same crs: %v", err)
	}

	there, err := FromRaster(metric, Options{Interval: 5}, "EPSG:4326")
	if err != nil {
		t.Fatalf("from raster, wgs84: %v", err)
	}

	if there.CRS != "EPSG:4326" {
		t.Errorf("crs = %q, want EPSG:4326", there.CRS)
	}

	if len(here.Lines) != len(there.Lines) || len(here.Lines) == 0 {
		t.Fatalf("line counts differ: %d vs %d", len(here.Lines), len(there.Lines))
	}

	// Degrees, not metres. Asserting the magnitude rather than the value keeps
	// this a test of *which CRS*, and leaves the arithmetic to crstransform's
	// own reference vectors.
	first := there.Lines[0].Points[0]
	if first[0] < -180 || first[0] > 180 || first[1] < -90 || first[1] > 90 {
		t.Errorf("point %v is not in degrees", first)
	}

	if here.Lines[0].Points[0] == first {
		t.Error("the lines did not move at all")
	}
}

func TestFromRasterRefusesAReceiverSetThatIsNotAGrid(t *testing.T) {
	t.Parallel()

	// No georeference: explicit receivers, placed individually, with no cell
	// size to place a contour on. `cli` can infer one from the receiver table;
	// this has no table to read, and guessing would invent a grid.
	raster := gridRaster(t, "EPSG:25832", nil)

	_, err := FromRaster(raster, Options{Interval: 5}, "EPSG:4326")
	if err == nil {
		t.Fatal("expected a refusal for a raster with no georeference")
	}

	if !strings.Contains(err.Error(), "georeference") {
		t.Errorf("refusal does not name the cause: %v", err)
	}
}

func TestFromRasterRefusesACRSItCannotTransform(t *testing.T) {
	t.Parallel()

	// The opposite of `cli.contoursInWGS84`'s policy, on purpose: a file a
	// reviewer opens in QGIS can carry vertices nobody moved, but a layer this
	// app draws under its own legend cannot be metres labelled as degrees.
	raster := gridRaster(t, "WKT:LOCAL_CS[\"site\"]", southUp())

	_, err := FromRaster(raster, Options{Interval: 5}, "EPSG:4326")
	if err == nil {
		t.Fatal("expected a refusal for a CRS with no EPSG code")
	}

	if !strings.Contains(err.Error(), "the raster") {
		t.Errorf("refusal does not say which end failed: %v", err)
	}

	// And the other end, so the message is not always blaming the raster.
	ok := gridRaster(t, "EPSG:25832", southUp())

	_, err = FromRaster(ok, Options{Interval: 5}, "WKT:LOCAL_CS[\"site\"]")
	if err == nil {
		t.Fatal("expected a refusal for an untransformable target")
	}

	if !strings.Contains(err.Error(), "the requested target") {
		t.Errorf("refusal does not say which end failed: %v", err)
	}
}

func TestFromRasterRefusesANilRaster(t *testing.T) {
	t.Parallel()

	_, err := FromRaster(nil, Options{}, "EPSG:4326")
	if err == nil {
		t.Fatal("expected a refusal for a nil raster")
	}
}

func TestReprojectLeavesAnIdentityAlone(t *testing.T) {
	t.Parallel()

	crs, err := geo.ParseCRS("EPSG:25832")
	if err != nil {
		t.Fatalf("parse crs: %v", err)
	}

	lines := []Line{{Level: 55, BandName: "LrDay", Points: [][2]float64{{1, 2}, {3, 4}}}}

	moved, err := Reproject(lines, crs, crs)
	if err != nil {
		t.Fatalf("reproject: %v", err)
	}

	if moved[0].Points[0] != [2]float64{1, 2} {
		t.Errorf("an identity transform moved a point: %v", moved[0].Points[0])
	}
}

func TestReprojectCarriesTheLevelAndBandThrough(t *testing.T) {
	t.Parallel()

	from, err := geo.ParseCRS("EPSG:25832")
	if err != nil {
		t.Fatalf("parse from: %v", err)
	}

	to, err := geo.ParseCRS("EPSG:4326")
	if err != nil {
		t.Fatalf("parse to: %v", err)
	}

	lines := []Line{{Level: 65, BandName: "LrNight", Points: [][2]float64{{700000, 5700000}}}}

	moved, err := Reproject(lines, from, to)
	if err != nil {
		t.Fatalf("reproject: %v", err)
	}

	// The geometry moves; what the line *means* must not.
	if moved[0].Level != 65 || moved[0].BandName != "LrNight" {
		t.Errorf("reprojection changed the line's identity: %+v", moved[0])
	}

	if moved[0].Points[0] == lines[0].Points[0] {
		t.Error("the point did not move")
	}
}
