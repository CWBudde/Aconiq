package results

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRasterIndexingAndRoundtrip(t *testing.T) {
	t.Parallel()

	bandNames := []string{"Lden", "Lnight"}

	raster, err := NewRaster(RasterMetadata{
		Width:     3,
		Height:    2,
		Bands:     2,
		NoData:    -9999,
		Units:     UniformUnits(bandNames, UnitDecibel),
		BandNames: bandNames,
	})
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	err = raster.Set(0, 0, 0, 50.5)
	if err != nil {
		t.Fatalf("set value: %v", err)
	}

	err = raster.Set(2, 1, 1, 42.0)
	if err != nil {
		t.Fatalf("set value: %v", err)
	}

	v0, err := raster.At(0, 0, 0)
	if err != nil {
		t.Fatalf("get value: %v", err)
	}

	if math.Abs(v0-50.5) > 1e-9 {
		t.Fatalf("unexpected value %.6f", v0)
	}

	dir := t.TempDir()

	paths, err := SaveRaster(filepath.Join(dir, "noise_map"), raster)
	if err != nil {
		t.Fatalf("save raster: %v", err)
	}

	loaded, err := LoadRaster(paths.MetadataPath)
	if err != nil {
		t.Fatalf("load raster: %v", err)
	}

	v1, err := loaded.At(2, 1, 1)
	if err != nil {
		t.Fatalf("loaded value: %v", err)
	}

	if math.Abs(v1-42.0) > 1e-9 {
		t.Fatalf("unexpected loaded value %.6f", v1)
	}
}

func TestRasterBoundsError(t *testing.T) {
	t.Parallel()

	raster, err := NewRaster(RasterMetadata{Width: 1, Height: 1, Bands: 1, NoData: -1})
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}
	{
		_, err := raster.At(2, 0, 0)
		if err == nil {
			t.Fatal("expected bounds error")
		}
	}
}

func TestGeoreferenceValidate(t *testing.T) {
	t.Parallel()

	valid := Georeference{OriginX: 500000, OriginY: 5600000, PixelSizeM: 10, RowOrder: RowOrderSouthUp}

	err := valid.Validate()
	if err != nil {
		t.Fatalf("valid georeference: %v", err)
	}

	cases := map[string]Georeference{
		"zero pixel size":     {OriginX: 1, OriginY: 1, PixelSizeM: 0, RowOrder: RowOrderSouthUp},
		"negative pixel size": {OriginX: 1, OriginY: 1, PixelSizeM: -10, RowOrder: RowOrderSouthUp},
		"non-finite origin x": {OriginX: math.Inf(1), OriginY: 1, PixelSizeM: 10, RowOrder: RowOrderSouthUp},
		"non-finite origin y": {OriginX: 1, OriginY: math.NaN(), PixelSizeM: 10, RowOrder: RowOrderSouthUp},
		"unset row order":     {OriginX: 1, OriginY: 1, PixelSizeM: 10},
		"unknown row order":   {OriginX: 1, OriginY: 1, PixelSizeM: 10, RowOrder: "north-up"},
	}

	for name, georef := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := georef.Validate()
			if err == nil {
				t.Fatalf("expected %s to be refused", name)
			}
		})
	}
}

// A raster cannot be created with a georeference that does not describe a
// grid: the sidecar is what every downstream GIS format reads its transform
// from, so a bad one is worse than none at all.
func TestNewRasterRefusesInvalidGeoreference(t *testing.T) {
	t.Parallel()

	_, err := NewRaster(RasterMetadata{
		Width: 1, Height: 1, Bands: 1, NoData: -1,
		Geo: &Georeference{OriginX: 0, OriginY: 0, PixelSizeM: 0, RowOrder: RowOrderSouthUp},
	})
	if err == nil {
		t.Fatal("expected a raster with a zero pixel size to be refused")
	}
}

// Metadata() hands back a value, and a pointer field would have made that a
// half-truth: mutating the returned georeference must not reach the raster.
func TestRasterMetadataCopiesGeoreference(t *testing.T) {
	t.Parallel()

	raster, err := NewRaster(RasterMetadata{
		Width: 2, Height: 2, Bands: 1, NoData: -1,
		Geo: &Georeference{OriginX: 100, OriginY: 200, PixelSizeM: 10, RowOrder: RowOrderSouthUp},
	})
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	meta := raster.Metadata()
	meta.Geo.OriginX = -1

	if raster.Metadata().Geo.OriginX != 100 {
		t.Fatal("mutating the returned metadata reached the raster's georeference")
	}
}

// The sidecar is the only place the georeference and the CRS survive, so the
// round trip is the contract every GIS export depends on.
func TestSaveRasterRoundTripsGeoreferenceAndCRS(t *testing.T) {
	t.Parallel()

	bandNames := []string{"Lden"}

	raster, err := NewRaster(RasterMetadata{
		Width: 2, Height: 3, Bands: 1, NoData: -9999,
		Units:     UniformUnits(bandNames, UnitDecibel),
		BandNames: bandNames,
		CRS:       "EPSG:25832",
		Geo:       &Georeference{OriginX: 500000, OriginY: 5600000, PixelSizeM: 10, RowOrder: RowOrderSouthUp},
	})
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	paths, err := SaveRaster(filepath.Join(t.TempDir(), "grid"), raster)
	if err != nil {
		t.Fatalf("save raster: %v", err)
	}

	loaded, err := LoadRaster(paths.MetadataPath)
	if err != nil {
		t.Fatalf("load raster: %v", err)
	}

	meta := loaded.Metadata()
	if meta.CRS != "EPSG:25832" {
		t.Fatalf("crs did not survive the round trip: %q", meta.CRS)
	}

	if meta.Geo == nil {
		t.Fatal("georeference did not survive the round trip")
	}

	if meta.Geo.OriginX != 500000 || meta.Geo.OriginY != 5600000 || meta.Geo.PixelSizeM != 10 {
		t.Fatalf("unexpected georeference %+v", *meta.Geo)
	}

	if meta.Geo.RowOrder != RowOrderSouthUp {
		t.Fatalf("unexpected row order %q", meta.Geo.RowOrder)
	}
}

// A raster with no georeference must not gain one by being written: absence is
// the signal that the receivers were not a grid.
func TestSaveRasterOmitsAbsentGeoreference(t *testing.T) {
	t.Parallel()

	raster, err := NewRaster(RasterMetadata{Width: 1, Height: 2, Bands: 1, NoData: -1})
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	paths, err := SaveRaster(filepath.Join(t.TempDir(), "scattered"), raster)
	if err != nil {
		t.Fatalf("save raster: %v", err)
	}

	encoded, err := os.ReadFile(paths.MetadataPath)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}

	if strings.Contains(string(encoded), "georeference") {
		t.Fatalf("sidecar carries a georeference it was never given:\n%s", encoded)
	}
}

func TestSetReceiverLeavesAMaskedCellAtNoData(t *testing.T) {
	t.Parallel()

	bands := []string{"LrDay", "LrNight"}

	raster, err := NewRaster(RasterMetadata{
		Width: 3, Height: 2, Bands: 2, NoData: -9999,
		Units: UniformUnits(bands, UnitDecibel), BandNames: bands,
	})
	if err != nil {
		t.Fatal(err)
	}

	layout := GridLayout{Width: 3, Height: 2, NoDataCells: []bool{false, false, false, false, true, false}}
	if layout.NoDataCount() != 1 {
		t.Fatalf("NoDataCount = %d, want 1", layout.NoDataCount())
	}

	for i := range 6 {
		err := raster.SetReceiver(layout, i, float64(50+i), float64(40+i))
		if err != nil {
			t.Fatalf("receiver %d: %v", i, err)
		}
	}

	// Receiver 4 is cell (1, 1): row-major from the origin.
	for band, want := range []float64{-9999, -9999} {
		got, _ := raster.At(1, 1, band)
		if got != want {
			t.Errorf("masked cell band %d = %v, want nodata", band, got)
		}
	}

	got, _ := raster.At(2, 1, 1)
	if got != 45 {
		t.Errorf("receiver 5 night = %v, want 45", got)
	}

	// A layout with no mask masks nothing, and an index past it is not masked.
	if (GridLayout{}).IsNoData(0) || layout.IsNoData(99) || layout.IsNoData(-1) {
		t.Error("IsNoData answered true outside the mask")
	}
}
