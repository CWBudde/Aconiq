package results

import (
	"math"
	"path/filepath"
	"testing"
)

func TestRasterIndexingAndRoundtrip(t *testing.T) {
	t.Parallel()

	raster, err := NewRaster(RasterMetadata{
		Width:     3,
		Height:    2,
		Bands:     2,
		NoData:    -9999,
		Unit:      "dB",
		BandNames: []string{"Lden", "Lnight"},
	})
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	if err := raster.Set(0, 0, 0, 50.5); err != nil {
		t.Fatalf("set value: %v", err)
	}

	if err := raster.Set(2, 1, 1, 42.0); err != nil {
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

	raster, err := NewRaster(RasterMetadata{Width: 1, Height: 1, Bands: 1, NoData: -1, Unit: "dB"})
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	if _, err := raster.At(2, 0, 0); err == nil {
		t.Fatal("expected bounds error")
	}
}
