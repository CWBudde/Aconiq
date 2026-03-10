package terrain

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func buildMinimalGeoTIFF(width, height int, pixels []float32, originX, originY, pixelSizeX, pixelSizeY float64) []byte {
	order := binary.LittleEndian
	bytesPerPixel := 4
	pixelDataSize := width * height * bytesPerPixel

	pixelOffset := 8
	numTags := 9
	ifdOffset := pixelOffset + pixelDataSize
	ifdSize := 2 + numTags*12 + 4
	scaleDataOffset := ifdOffset + ifdSize
	tpDataOffset := scaleDataOffset + 24
	totalSize := tpDataOffset + 48

	buf := make([]byte, totalSize)

	// Header.
	buf[0] = 'I'
	buf[1] = 'I'
	order.PutUint16(buf[2:], 42)
	order.PutUint32(buf[4:], uint32(ifdOffset))

	// Pixel data.
	for i, v := range pixels {
		order.PutUint32(buf[pixelOffset+i*4:], math.Float32bits(v))
	}

	// IFD.
	pos := ifdOffset
	order.PutUint16(buf[pos:], uint16(numTags))
	pos += 2

	writeTag := func(tag, dtype uint16, count uint32, value uint32) {
		order.PutUint16(buf[pos:], tag)
		order.PutUint16(buf[pos+2:], dtype)
		order.PutUint32(buf[pos+4:], count)
		order.PutUint32(buf[pos+8:], value)
		pos += 12
	}

	writeTag(tagImageWidth, tiffTypeShort, 1, uint32(width))
	writeTag(tagImageLength, tiffTypeShort, 1, uint32(height))
	writeTag(tagBitsPerSample, tiffTypeShort, 1, 32)
	writeTag(tagCompression, tiffTypeShort, 1, compressionNone)
	writeTag(tagStripOffsets, tiffTypeLong, 1, uint32(pixelOffset))
	writeTag(tagStripByteCounts, tiffTypeLong, 1, uint32(pixelDataSize))
	writeTag(tagSampleFormat, tiffTypeShort, 1, sampleFormatFloat)
	writeTag(tagModelPixelScale, tiffTypeDouble, 3, uint32(scaleDataOffset))
	writeTag(tagModelTiepoint, tiffTypeDouble, 6, uint32(tpDataOffset))

	// Next IFD = 0.
	order.PutUint32(buf[pos:], 0)

	// Scale data.
	order.PutUint64(buf[scaleDataOffset:], math.Float64bits(pixelSizeX))
	order.PutUint64(buf[scaleDataOffset+8:], math.Float64bits(pixelSizeY))
	order.PutUint64(buf[scaleDataOffset+16:], 0)

	// Tiepoint data.
	order.PutUint64(buf[tpDataOffset:], 0)    // pixI
	order.PutUint64(buf[tpDataOffset+8:], 0)  // pixJ
	order.PutUint64(buf[tpDataOffset+16:], 0) // pixK
	order.PutUint64(buf[tpDataOffset+24:], math.Float64bits(originX))
	order.PutUint64(buf[tpDataOffset+32:], math.Float64bits(originY))
	order.PutUint64(buf[tpDataOffset+40:], 0) // geoZ

	return buf
}

//nolint:unparam // pixelSizeX is always 10 in current tests but the parameter is part of the GeoTIFF contract
func writeTestGeoTIFF(t *testing.T, width, height int, pixels []float32, originX, originY, pixelSizeX, pixelSizeY float64) string {
	t.Helper()

	data := buildMinimalGeoTIFF(width, height, pixels, originX, originY, pixelSizeX, pixelSizeY)
	path := filepath.Join(t.TempDir(), "test.tif")

	err := os.WriteFile(path, data, 0o644)
	if err != nil {
		t.Fatalf("write test GeoTIFF: %v", err)
	}

	return path
}

// --- Tests ---

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.tif"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_ValidGeoTIFF(t *testing.T) {
	// 3x3 grid, origin at (100, 200), 10m pixel size.
	// Elevations: 100..108
	pixels := []float32{100, 101, 102, 103, 104, 105, 106, 107, 108}
	path := writeTestGeoTIFF(t, 3, 3, pixels, 100, 200, 10, 10)

	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	bounds := m.Bounds()
	// Origin is pixel center at (100, 200); pixel size 10m.
	// MinX = 100 - 5 = 95, MaxX = 120 + 5 = 125
	// MaxY = 200 + 5 = 205, MinY = 180 - 5 = 175

	if bounds[0] != 95 || bounds[2] != 125 {
		t.Errorf("expected X bounds [95, 125], got [%f, %f]", bounds[0], bounds[2])
	}

	if bounds[1] != 175 || bounds[3] != 205 {
		t.Errorf("expected Y bounds [175, 205], got [%f, %f]", bounds[1], bounds[3])
	}
}

func TestElevationAt_PixelCenters(t *testing.T) {
	// 3x3 grid, origin at (100, 200), 10m pixel size.
	// Row 0: 100, 101, 102
	// Row 1: 103, 104, 105
	// Row 2: 106, 107, 108
	pixels := []float32{100, 101, 102, 103, 104, 105, 106, 107, 108}
	path := writeTestGeoTIFF(t, 3, 3, pixels, 100, 200, 10, 10)

	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Query at pixel center (0,0) = world (100, 200).
	elev, ok := m.ElevationAt(100, 200)
	if !ok {
		t.Fatal("expected point inside bounds")
	}

	if elev != 100 {
		t.Errorf("expected 100.0 at pixel (0,0), got %f", elev)
	}

	// Query at pixel center (1,1) = world (110, 190).
	elev, ok = m.ElevationAt(110, 190)
	if !ok {
		t.Fatal("expected point inside bounds")
	}

	if elev != 104 {
		t.Errorf("expected 104.0 at pixel (1,1), got %f", elev)
	}
}

func TestElevationAt_BilinearInterpolation(t *testing.T) {
	// 2x2 grid:
	// (0,0)=10  (1,0)=20
	// (0,1)=30  (1,1)=40
	// Origin at (0, 10), pixel size 10m.
	pixels := []float32{10, 20, 30, 40}
	path := writeTestGeoTIFF(t, 2, 2, pixels, 0, 10, 10, 10)

	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Query at the center of the 4 pixels = world (5, 5).
	// Bilinear interpolation of 10, 20, 30, 40 at (0.5, 0.5):
	// = 10*0.5*0.5 + 20*0.5*0.5 + 30*0.5*0.5 + 40*0.5*0.5 = 25
	elev, ok := m.ElevationAt(5, 5)
	if !ok {
		t.Fatal("expected point inside bounds")
	}

	if math.Abs(elev-25.0) > 0.001 {
		t.Errorf("expected 25.0 at center, got %f", elev)
	}

	// Query at (2.5, 7.5) — between pixel (0,0) and (0,1) in X, between row 0 and 1 in Y.
	// px = (2.5 - 0) / 10 = 0.25, py = (10 - 7.5) / 10 = 0.25
	// Bilinear: 10*(0.75)*(0.75) + 20*(0.25)*(0.75) + 30*(0.75)*(0.25) + 40*(0.25)*(0.25)
	// = 5.625 + 3.75 + 5.625 + 2.5 = 17.5
	elev, ok = m.ElevationAt(2.5, 7.5)
	if !ok {
		t.Fatal("expected point inside bounds")
	}

	if math.Abs(elev-17.5) > 0.001 {
		t.Errorf("expected 17.5, got %f", elev)
	}
}

func TestElevationAt_OutsideBounds(t *testing.T) {
	pixels := []float32{10, 20, 30, 40}
	path := writeTestGeoTIFF(t, 2, 2, pixels, 0, 10, 10, 10)

	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Well outside bounds.
	_, ok := m.ElevationAt(1000, 1000)
	if ok {
		t.Error("expected point outside bounds")
	}
}

func TestLoad_InvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.tif")

	err := os.WriteFile(path, []byte("not a tiff"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(path)
	if err == nil {
		t.Fatal("expected error for invalid file")
	}
}
