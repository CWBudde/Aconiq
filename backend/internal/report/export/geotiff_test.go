package export

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/report/results"
)

func TestExportGeoTIFF(t *testing.T) {
	t.Parallel()

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     4,
		Height:    3,
		Bands:     1,
		NoData:    -9999,
		Unit:      "dB",
		BandNames: []string{"Lden"},
		CRS:       "EPSG:25832",
	})
	if err != nil {
		t.Fatalf("create raster: %v", err)
	}

	// Fill with test values.
	for y := range 3 {
		for x := range 4 {
			err := raster.Set(x, y, 0, 45.0+float64(x)+float64(y)*10)
			if err != nil {
				t.Fatalf("set raster: %v", err)
			}
		}
	}

	outDir := t.TempDir()
	basePath := filepath.Join(outDir, "test-raster")

	gt := GeoTransform{
		OriginX:    500000,
		OriginY:    5700030,
		PixelSizeX: 10.0,
		PixelSizeY: -10.0,
	}

	paths, err := ExportGeoTIFF(basePath, raster, gt, "EPSG:25832")
	if err != nil {
		t.Fatalf("export geotiff: %v", err)
	}

	if len(paths) != 1 {
		t.Fatalf("expected 1 output file, got %d", len(paths))
	}

	expectedPath := filepath.Join(outDir, "test-raster_Lden.tif")
	if paths[0] != expectedPath {
		t.Fatalf("path = %q, want %q", paths[0], expectedPath)
	}

	// Verify the file is a valid TIFF.
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read geotiff: %v", err)
	}

	// TIFF header: "II" (little-endian), magic 42.
	if len(data) < 8 {
		t.Fatalf("file too small: %d bytes", len(data))
	}

	if data[0] != 'I' || data[1] != 'I' {
		t.Fatalf("not little-endian TIFF")
	}

	magic := binary.LittleEndian.Uint16(data[2:])
	if magic != 42 {
		t.Fatalf("TIFF magic = %d, want 42", magic)
	}

	ifdOffset := binary.LittleEndian.Uint32(data[4:])
	if ifdOffset != 8 {
		t.Fatalf("IFD offset = %d, want 8", ifdOffset)
	}

	// Read IFD entry count.
	entryCount := binary.LittleEndian.Uint16(data[8:])
	if entryCount < 10 {
		t.Fatalf("expected >= 10 IFD entries, got %d", entryCount)
	}

	// Verify ImageWidth and ImageLength tags.
	assertIFDTag(t, data, 10, entryCount, tiffTagImageWidth, 4)
	assertIFDTag(t, data, 10, entryCount, tiffTagImageLength, 3)
}

func TestExportGeoTIFFMultiBand(t *testing.T) {
	t.Parallel()

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     2,
		Height:    2,
		Bands:     2,
		NoData:    -9999,
		Unit:      "dB",
		BandNames: []string{"Lden", "Lnight"},
	})
	if err != nil {
		t.Fatalf("create raster: %v", err)
	}

	for band := range 2 {
		for y := range 2 {
			for x := range 2 {
				_ = raster.Set(x, y, band, float64(band*10+y*2+x))
			}
		}
	}

	outDir := t.TempDir()
	gt := GeoTransform{OriginX: 0, OriginY: 20, PixelSizeX: 10, PixelSizeY: -10}

	paths, err := ExportGeoTIFF(filepath.Join(outDir, "multi"), raster, gt, "EPSG:4326")
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	if len(paths) != 2 {
		t.Fatalf("expected 2 files, got %d", len(paths))
	}

	for _, p := range paths {
		_, err := os.Stat(p)
		if err != nil {
			t.Fatalf("file %s missing: %v", p, err)
		}
	}
}

func TestExportGeoTIFFNilRaster(t *testing.T) {
	t.Parallel()

	_, err := ExportGeoTIFF("/tmp/test", nil, GeoTransform{}, "")
	if err == nil {
		t.Fatal("expected error for nil raster")
	}
}

func TestExportGeoTIFFRowFlip(t *testing.T) {
	t.Parallel()

	// Create a 2x2 raster where row 0 (bottom) has value 10, row 1 (top) has value 20.
	raster, err := results.NewRaster(results.RasterMetadata{
		Width: 2, Height: 2, Bands: 1, NoData: -9999, Unit: "dB", BandNames: []string{"test"},
	})
	if err != nil {
		t.Fatalf("create raster: %v", err)
	}

	_ = raster.Set(0, 0, 0, 10.0) // bottom-left
	_ = raster.Set(1, 0, 0, 11.0) // bottom-right
	_ = raster.Set(0, 1, 0, 20.0) // top-left
	_ = raster.Set(1, 1, 0, 21.0) // top-right

	outDir := t.TempDir()
	gt := GeoTransform{OriginX: 0, OriginY: 20, PixelSizeX: 10, PixelSizeY: -10}

	paths, err := ExportGeoTIFF(filepath.Join(outDir, "flip"), raster, gt, "")
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Find image data offset from StripOffsets tag.
	ifdOffset := int(binary.LittleEndian.Uint32(data[4:]))
	entryCount := int(binary.LittleEndian.Uint16(data[ifdOffset:]))

	var imageOffset int

	for i := range entryCount {
		pos := ifdOffset + 2 + i*12
		tag := binary.LittleEndian.Uint16(data[pos:])

		if tag == tiffTagStripOffsets {
			imageOffset = int(binary.LittleEndian.Uint32(data[pos+8:]))
			break
		}
	}

	if imageOffset == 0 {
		t.Fatal("could not find StripOffsets")
	}

	// Read first row in TIFF (should be the top row = raster row 1 = values 20, 21).
	v0 := math.Float64frombits(binary.LittleEndian.Uint64(data[imageOffset:]))
	v1 := math.Float64frombits(binary.LittleEndian.Uint64(data[imageOffset+8:]))

	if v0 != 20.0 || v1 != 21.0 {
		t.Fatalf("first TIFF row = [%v, %v], want [20, 21]", v0, v1)
	}

	// Read second row in TIFF (should be the bottom row = raster row 0 = values 10, 11).
	v2 := math.Float64frombits(binary.LittleEndian.Uint64(data[imageOffset+16:]))
	v3 := math.Float64frombits(binary.LittleEndian.Uint64(data[imageOffset+24:]))

	if v2 != 10.0 || v3 != 11.0 {
		t.Fatalf("second TIFF row = [%v, %v], want [10, 11]", v2, v3)
	}
}

func TestExportCOG(t *testing.T) {
	t.Parallel()

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     300,
		Height:    200,
		Bands:     1,
		NoData:    -9999,
		Unit:      "dB",
		BandNames: []string{"Lden"},
		CRS:       "EPSG:25832",
	})
	if err != nil {
		t.Fatalf("create raster: %v", err)
	}

	for y := range 200 {
		for x := range 300 {
			err := raster.Set(x, y, 0, 45.0+float64(x)*0.1+float64(y)*0.05)
			if err != nil {
				t.Fatalf("set raster: %v", err)
			}
		}
	}

	outDir := t.TempDir()
	basePath := filepath.Join(outDir, "test-cog")

	gt := GeoTransform{
		OriginX:    500000,
		OriginY:    5700020,
		PixelSizeX: 10.0,
		PixelSizeY: -10.0,
	}

	paths, err := ExportCOG(basePath, raster, gt, "EPSG:25832")
	if err != nil {
		t.Fatalf("export cog: %v", err)
	}

	if len(paths) != 1 {
		t.Fatalf("expected 1 output file, got %d", len(paths))
	}

	expectedPath := filepath.Join(outDir, "test-cog_Lden.cog.tif")
	if paths[0] != expectedPath {
		t.Fatalf("path = %q, want %q", paths[0], expectedPath)
	}

	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read cog: %v", err)
	}

	// Verify TIFF header.
	if len(data) < 8 {
		t.Fatalf("file too small: %d bytes", len(data))
	}

	if data[0] != 'I' || data[1] != 'I' {
		t.Fatal("not little-endian TIFF")
	}

	magic := binary.LittleEndian.Uint16(data[2:])
	if magic != 42 {
		t.Fatalf("TIFF magic = %d, want 42", magic)
	}

	ifdOffset := int(binary.LittleEndian.Uint32(data[4:]))
	if ifdOffset != 8 {
		t.Fatalf("IFD offset = %d, want 8", ifdOffset)
	}

	// Read IFD and verify tile tags.
	entryCount := binary.LittleEndian.Uint16(data[ifdOffset:])
	ifdStart := ifdOffset + 2

	assertIFDTag(t, data, ifdStart, entryCount, tiffTagTileWidth, 256)
	assertIFDTag(t, data, ifdStart, entryCount, tiffTagTileLength, 256)
	assertIFDTag(t, data, ifdStart, entryCount, tiffTagImageWidth, 300)
	assertIFDTag(t, data, ifdStart, entryCount, tiffTagImageLength, 200)

	// Verify TileOffsets tag exists (count = tilesX * tilesY = 2*1 = 2).
	assertIFDTagExists(t, data, ifdStart, entryCount, tiffTagTileOffsets)
	assertIFDTagExists(t, data, ifdStart, entryCount, tiffTagTileByteCounts)
}

func TestExportCOGNilRaster(t *testing.T) {
	t.Parallel()

	_, err := ExportCOG("/tmp/test", nil, GeoTransform{}, "")
	if err == nil {
		t.Fatal("expected error for nil raster")
	}
}

func TestExportCOGOverviews(t *testing.T) {
	t.Parallel()

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     600,
		Height:    400,
		Bands:     1,
		NoData:    -9999,
		Unit:      "dB",
		BandNames: []string{"Lden"},
		CRS:       "EPSG:25832",
	})
	if err != nil {
		t.Fatalf("create raster: %v", err)
	}

	for y := range 400 {
		for x := range 600 {
			err := raster.Set(x, y, 0, 50.0+float64(x)*0.01+float64(y)*0.02)
			if err != nil {
				t.Fatalf("set raster: %v", err)
			}
		}
	}

	outDir := t.TempDir()
	basePath := filepath.Join(outDir, "cog-overview")

	gt := GeoTransform{
		OriginX:    500000,
		OriginY:    5700040,
		PixelSizeX: 10.0,
		PixelSizeY: -10.0,
	}

	paths, err := ExportCOG(basePath, raster, gt, "EPSG:25832")
	if err != nil {
		t.Fatalf("export cog: %v", err)
	}

	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read cog: %v", err)
	}

	// Follow IFD chain and count IFDs (should have > 1 for overviews).
	ifdCount := 0
	nextIFD := int(binary.LittleEndian.Uint32(data[4:]))

	for nextIFD != 0 {
		ifdCount++
		entryCount := int(binary.LittleEndian.Uint16(data[nextIFD:]))
		// Next IFD pointer is after all entries.
		nextIFDPos := nextIFD + 2 + entryCount*12
		nextIFD = int(binary.LittleEndian.Uint32(data[nextIFDPos:]))
	}

	if ifdCount < 2 {
		t.Fatalf("expected at least 2 IFDs (full-res + overview), got %d", ifdCount)
	}

	t.Logf("COG has %d IFDs (1 full-res + %d overviews)", ifdCount, ifdCount-1)
}

// assertIFDTagExists checks that a tag exists in the IFD.
func assertIFDTagExists(t *testing.T, data []byte, ifdStart int, entryCount uint16, tagID uint16) {
	t.Helper()

	for i := range int(entryCount) {
		pos := ifdStart + i*12
		tag := binary.LittleEndian.Uint16(data[pos:])

		if tag == tagID {
			return
		}
	}

	t.Fatalf("tag %d not found in IFD", tagID)
}

// assertIFDTag checks that a tag exists in the IFD with the expected value.
func assertIFDTag(t *testing.T, data []byte, ifdStart int, entryCount uint16, tagID uint16, expectedValue uint32) {
	t.Helper()

	for i := range int(entryCount) {
		pos := ifdStart + i*12
		tag := binary.LittleEndian.Uint16(data[pos:])

		if tag == tagID {
			val := binary.LittleEndian.Uint32(data[pos+8:])
			if val != expectedValue {
				t.Fatalf("tag %d value = %d, want %d", tagID, val, expectedValue)
			}

			return
		}
	}

	t.Fatalf("tag %d not found in IFD", tagID)
}
