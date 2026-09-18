package export

import (
	"testing"

	"github.com/aconiq/backend/internal/qa/golden"
	"github.com/aconiq/backend/internal/report/results"
)

// The raster centre→corner georeference contract.
//
// testdata/raster-parity/ is read from *two* trees: this test writes it, and
// frontend/src/map/raster-extent.parity.test.ts reads it to check that the
// browser's geoTransformFromGeoreference produces the same numbers
// GeoTransformFromGeoreference does here. Moving, renaming or reshaping
// anything under testdata/raster-parity/ breaks that test, which no Go tool
// will tell you about — grep frontend/ before you do.
//
// Go is canonical. Georeference says the conversion happens "once, in
// report/export, and nowhere else", and in Go it does; the browser cannot call
// it, so raster-extent.ts is a second implementation by necessity — as
// raster-bin.ts and receiver-csv.ts are — and this golden is what keeps it
// honest.
//
// What the fixture has to pin is the half-pixel arithmetic and the sign of the
// height term. Both are invisible in the output: a transform that walks half a
// pixel the wrong way, or omits the (height-1) term, still produces a
// well-formed affine transform that places the raster a cell or a grid-height
// away from where its receivers are. Only a comparison against the numbers Go
// actually returns catches that.
//
// This lives in export/ rather than beside the raster binary fixture in
// results/: export imports results, and results' raster_parity_test.go is in
// package results (not results_test), so a test generating this there could
// not reach GeoTransformFromGeoreference without closing an import cycle.
//
// Regenerate with `just update-golden`.

// geoTransformParityCase carries input *and* output, so the frontend mirror
// has something to feed its own implementation rather than a bare expectation
// it would have to restate the inputs for.
//
// The output is spelled in snake_case like every other JSON in this project —
// GeoTransform itself carries no tags because it never crosses a wire — and
// the TypeScript side maps it onto its own camelCase fields.
type geoTransformParityCase struct {
	Name       string               `json:"name"`
	Why        string               `json:"why"`
	Georef     results.Georeference `json:"georeference"`
	GridHeight int                  `json:"grid_height"`
	Want       geoTransformJSON     `json:"geo_transform"`
}

type geoTransformJSON struct {
	OriginX    float64 `json:"origin_x"`
	OriginY    float64 `json:"origin_y"`
	PixelSizeX float64 `json:"pixel_size_x"`
	PixelSizeY float64 `json:"pixel_size_y"`
}

func geoTransformParityInputs() []geoTransformParityCase {
	southUp := func(originX float64, originY float64, pixelSizeM float64) results.Georeference {
		return results.Georeference{
			OriginX:    originX,
			OriginY:    originY,
			PixelSizeM: pixelSizeM,
			RowOrder:   results.RowOrderSouthUp,
		}
	}

	return []geoTransformParityCase{
		{
			Name:       "raster binary parity fixture",
			Why:        "the same grid results' raster-parity fixture describes, so both goldens speak about one raster",
			Georef:     southUp(-50.5, 5600000.25, 12.5),
			GridHeight: 2,
		},
		{
			Name:       "single row",
			Why:        "the (height-1) term vanishes, so an off-by-one in it is invisible here and visible everywhere else",
			Georef:     southUp(100, 200, 25),
			GridHeight: 1,
		},
		{
			Name:       "negative origin",
			Why:        "a sign error in the half-pixel step is otherwise hidden by a positive UTM easting",
			Georef:     southUp(-1200.5, -300.25, 5),
			GridHeight: 4,
		},
		{
			Name:       "non-integral pixel size",
			Why:        "halving an odd pixel size is where a mirror that rounds to whole metres parts company",
			Georef:     southUp(412345.75, 5321000.5, 2.5),
			GridHeight: 7,
		},
		{
			Name:       "pixel size whose half is not exact in binary",
			Why:        "0.1/2 is not representable, so both sides must divide rather than multiply by 0.5 of a decimal literal",
			Georef:     southUp(10, 20, 0.1),
			GridHeight: 3,
		},
	}
}

func TestGeoTransformFromGeoreferenceParityFixture(t *testing.T) {
	t.Parallel()

	cases := geoTransformParityInputs()

	for index := range cases {
		transform, err := GeoTransformFromGeoreference(cases[index].Georef, cases[index].GridHeight)
		if err != nil {
			t.Fatalf("case %q: geo transform from georeference: %v", cases[index].Name, err)
		}

		cases[index].Want = geoTransformJSON{
			OriginX:    transform.OriginX,
			OriginY:    transform.OriginY,
			PixelSizeX: transform.PixelSizeX,
			PixelSizeY: transform.PixelSizeY,
		}
	}

	golden.AssertJSONSnapshot(t, "testdata/raster-parity/geotransform.golden.json", cases)
}
