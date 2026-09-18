package results

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/qa/golden"
)

// The raster binary byte contract.
//
// testdata/raster-parity/ is read from *two* trees: this test writes it, and
// frontend/src/model/raster-bin.parity.test.ts reads it to check that the
// browser builder emits the same bytes SaveRaster does here. Moving, renaming
// or reshaping anything under testdata/raster-parity/ breaks that test, which
// no Go tool will tell you about — grep frontend/ before you do.
//
// Go is canonical, as it is for the receiver CSV. What the fixture has to pin
// is the part a mirror gets wrong: the **index order**. A raster is band-major
// and row-major within a band — `(band*height + y)*width + x` — so a builder
// that walks receivers and writes them in arrival order produces a file of
// exactly the right length, full of finite values, with every cell in the
// wrong place. Only a byte comparison catches that.
//
// Two goldens, both load-bearing:
//
//   - raster.golden.json is the frontend's *input*: the metadata and the cell
//     values, in receiver order rather than raster order, which is what the
//     browser actually has.
//   - raster.golden.bin is the frontend's *expectation*, byte for byte.
//
// Regenerate both with `just update-golden`.

// rasterParityInput is what the browser holds after a run: a grid shape, a
// georeference, and one level per receiver per band in *receiver* order.
type rasterParityInput struct {
	Metadata RasterMetadata `json:"metadata"`
	// Bands[b][i] is band b's value at receiver index i. Receiver index is
	// row-major from the south-west corner, which is the order both
	// geo.GridReceiverSet.Generate and the browser's buildReceiverGrid emit.
	Bands [][]float64 `json:"bands"`
}

// rasterParityFixture is deliberately asymmetric — 3 wide by 2 high, two
// bands — so that a width/height swap and a band/row swap both change the
// bytes. A square single-band raster would pass either way.
func rasterParityFixture() rasterParityInput {
	bandNames := []string{"LrDay", "LrNight"}

	return rasterParityInput{
		Metadata: RasterMetadata{
			Width:     3,
			Height:    2,
			Bands:     2,
			NoData:    -9999,
			Units:     UniformUnits(bandNames, UnitDecibel),
			BandNames: bandNames,
			CRS:       "EPSG:25832",
			Geo: &Georeference{
				// Values a float64 round-trips exactly through both
				// languages' shortest-digit printing, and a negative origin,
				// because a sign error is otherwise invisible in UTM easting.
				OriginX:    -50.5,
				OriginY:    5600000.25,
				PixelSizeM: 12.5,
				RowOrder:   RowOrderSouthUp,
			},
		},
		Bands: [][]float64{
			// Deliberately not monotonic, so a transposed write does not
			// happen to produce an ordered file that still looks plausible.
			{62.4, 55.125, 71, -0.5, 48.75, 60},
			{55.1, 47.875, 63.5, -9999, 41.25, 52},
		},
	}
}

func TestRasterBinaryParityFixture(t *testing.T) {
	t.Parallel()

	fixture := rasterParityFixture()

	golden.AssertJSONSnapshot(t, "testdata/raster-parity/raster.golden.json", fixture)

	raster, err := NewRaster(fixture.Metadata)
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	for band, values := range fixture.Bands {
		for index, value := range values {
			x := index % fixture.Metadata.Width
			y := index / fixture.Metadata.Width

			err := raster.Set(x, y, band, value)
			if err != nil {
				t.Fatalf("set band %d index %d: %v", band, index, err)
			}
		}
	}

	persistence, err := SaveRaster(filepath.Join(t.TempDir(), "raster"), raster)
	if err != nil {
		t.Fatalf("save raster: %v", err)
	}

	encoded, err := os.ReadFile(persistence.DataPath)
	if err != nil {
		t.Fatalf("read raster binary: %v", err)
	}

	golden.AssertBytesSnapshot(t, "testdata/raster-parity/raster.golden.bin", encoded)

	// The sidecar's own shape is the frontend's second target: browser mode
	// writes a metadata object, not a file, and it has to carry the same keys.
	sidecar, err := os.ReadFile(persistence.MetadataPath)
	if err != nil {
		t.Fatalf("read raster sidecar: %v", err)
	}

	var decoded map[string]any

	err = json.Unmarshal(sidecar, &decoded)
	if err != nil {
		t.Fatalf("decode raster sidecar: %v", err)
	}

	// created_at is the clock; everything else is the contract.
	delete(decoded, "created_at")

	golden.AssertJSONSnapshot(t, "testdata/raster-parity/raster_sidecar.golden.json", decoded)
}
