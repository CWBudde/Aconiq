package httpv1

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/io/projectfs"
)

func TestImportTerrainEndpoint(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Terrain Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	fixedTime := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	handler := newHandlerWithOptions(store, handlerOptions{
		clock:        func() time.Time { return fixedTime },
		corsDisabled: true,
	})

	// Build a minimal synthetic GeoTIFF (2x2, float32, 10m pixels, origin 100,200).
	geotiffData := buildTestGeoTIFF(t)

	body, contentType := createMultipartFile(t, "file", "terrain.tif", geotiffData)

	req := newAPIRequest(http.MethodPost, "/api/v1/import/terrain", body)
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var info terrainInfoResponse
	decodeResponse(t, rec.Body.Bytes(), &info)

	if info.GridSize[0] != 2 || info.GridSize[1] != 2 {
		t.Errorf("expected grid 2x2, got %v", info.GridSize)
	}

	if info.PixelSize[0] != 10 || info.PixelSize[1] != 10 {
		t.Errorf("expected pixel size [10, 10], got %v", info.PixelSize)
	}

	// Verify artifact was registered.
	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	found := false

	for _, a := range proj.Artifacts {
		if a.ID == "artifact-terrain" {
			found = true

			if a.Kind != "model.terrain_geotiff" {
				t.Errorf("unexpected artifact kind: %q", a.Kind)
			}
		}
	}

	if !found {
		t.Error("artifact-terrain not found in project manifest")
	}
}

func TestImportTerrainRejectsBadExtension(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Terrain Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	handler := newHandlerWithOptions(store, handlerOptions{corsDisabled: true})

	body, contentType := createMultipartFile(t, "file", "terrain.png", []byte("not a tiff"))

	req := newAPIRequest(http.MethodPost, "/api/v1/import/terrain", body)
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestImportTerrainRejectsInvalidGeoTIFF(t *testing.T) {
	t.Parallel()

	store, err := projectfs.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = store.Init("Terrain Test", "EPSG:25832")
	if err != nil {
		t.Fatalf("init project: %v", err)
	}

	handler := newHandlerWithOptions(store, handlerOptions{corsDisabled: true})

	body, contentType := createMultipartFile(t, "file", "terrain.tif", []byte("not valid tiff data"))

	req := newAPIRequest(http.MethodPost, "/api/v1/import/terrain", body)
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

type terrainInfoResponse struct {
	Bounds    [4]float64 `json:"bounds"`
	PixelSize [2]float64 `json:"pixel_size"`
	GridSize  [2]int     `json:"grid_size"`
}

func buildTestGeoTIFF(t *testing.T) []byte {
	t.Helper()

	order := binary.LittleEndian
	width, height := 2, 2
	pixels := []float32{10, 20, 30, 40}
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

	writeTag(256, 3, 1, uint32(width))              // ImageWidth
	writeTag(257, 3, 1, uint32(height))             // ImageLength
	writeTag(258, 3, 1, 32)                         // BitsPerSample
	writeTag(259, 3, 1, 1)                          // Compression = None
	writeTag(273, 4, 1, uint32(pixelOffset))        // StripOffsets
	writeTag(279, 4, 1, uint32(pixelDataSize))      // StripByteCounts
	writeTag(339, 3, 1, 3)                          // SampleFormat = Float
	writeTag(33550, 12, 3, uint32(scaleDataOffset)) // ModelPixelScale
	writeTag(33922, 12, 6, uint32(tpDataOffset))    // ModelTiepoint

	// Next IFD = 0.
	order.PutUint32(buf[pos:], 0)

	// Scale data: 10m x 10m.
	order.PutUint64(buf[scaleDataOffset:], math.Float64bits(10))
	order.PutUint64(buf[scaleDataOffset+8:], math.Float64bits(10))
	order.PutUint64(buf[scaleDataOffset+16:], 0)

	// Tiepoint: origin at (100, 200).
	order.PutUint64(buf[tpDataOffset:], 0)
	order.PutUint64(buf[tpDataOffset+8:], 0)
	order.PutUint64(buf[tpDataOffset+16:], 0)
	order.PutUint64(buf[tpDataOffset+24:], math.Float64bits(100))
	order.PutUint64(buf[tpDataOffset+32:], math.Float64bits(200))
	order.PutUint64(buf[tpDataOffset+40:], 0)

	return buf
}

func createMultipartFile(t *testing.T, fieldName, fileName string, data []byte) (io.Reader, string) {
	t.Helper()

	var b bytes.Buffer

	w := multipart.NewWriter(&b)

	part, err := w.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	_, err = part.Write(data)
	if err != nil {
		t.Fatalf("write form file: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return &b, w.FormDataContentType()
}
