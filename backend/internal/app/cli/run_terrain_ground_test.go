package cli

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/report/results"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

// flatGeoTIFF writes a DTM of constant elevation covering the model used by
// the datum tests below: 20 × 20 cells of 50 m, upper-left pixel centre at
// (-100, 400), so the grid spans roughly x ∈ [-125, 825], y ∈ [-575, 425].
//
// It is the smallest thing that makes an elevated site reachable through the
// CLI: `aconiq import --terrain` is the only way ReceiverTerrainZ becomes
// non-zero on a run.
func flatGeoTIFF(t *testing.T, dir string, elevationM float64) string {
	t.Helper()

	const (
		width      = 20
		height     = 20
		pixelSize  = 50.0
		originX    = -100.0
		originY    = 400.0
		sampleSize = 4
	)

	order := binary.LittleEndian
	pixelDataSize := width * height * sampleSize

	pixelOffset := 8
	numTags := 9
	ifdOffset := pixelOffset + pixelDataSize
	ifdSize := 2 + numTags*12 + 4
	scaleDataOffset := ifdOffset + ifdSize
	tiepointOffset := scaleDataOffset + 24

	buf := make([]byte, tiepointOffset+48)

	buf[0], buf[1] = 'I', 'I'
	order.PutUint16(buf[2:], 42)
	order.PutUint32(buf[4:], uint32(ifdOffset))

	for i := range width * height {
		order.PutUint32(buf[pixelOffset+i*sampleSize:], math.Float32bits(float32(elevationM)))
	}

	pos := ifdOffset
	order.PutUint16(buf[pos:], uint16(numTags))
	pos += 2

	writeTag := func(tag, dtype uint16, count, value uint32) {
		order.PutUint16(buf[pos:], tag)
		order.PutUint16(buf[pos+2:], dtype)
		order.PutUint32(buf[pos+4:], count)
		order.PutUint32(buf[pos+8:], value)
		pos += 12
	}

	writeTag(256, 3, 1, width)                      // ImageWidth
	writeTag(257, 3, 1, height)                     // ImageLength
	writeTag(258, 3, 1, 32)                         // BitsPerSample
	writeTag(259, 3, 1, 1)                          // Compression = none
	writeTag(273, 4, 1, uint32(pixelOffset))        // StripOffsets
	writeTag(279, 4, 1, uint32(pixelDataSize))      // StripByteCounts
	writeTag(339, 3, 1, 3)                          // SampleFormat = float
	writeTag(33550, 12, 3, uint32(scaleDataOffset)) // ModelPixelScale
	writeTag(33922, 12, 6, uint32(tiepointOffset))  // ModelTiepoint

	order.PutUint32(buf[pos:], 0) // next IFD

	order.PutUint64(buf[scaleDataOffset:], math.Float64bits(pixelSize))
	order.PutUint64(buf[scaleDataOffset+8:], math.Float64bits(pixelSize))
	order.PutUint64(buf[scaleDataOffset+16:], 0)

	order.PutUint64(buf[tiepointOffset+24:], math.Float64bits(originX))
	order.PutUint64(buf[tiepointOffset+32:], math.Float64bits(originY))

	path := filepath.Join(dir, "terrain.tif")

	err := os.WriteFile(path, buf, 0o600)
	if err != nil {
		t.Fatalf("write terrain GeoTIFF: %v", err)
	}

	return path
}

// runRLS19AtDatum runs one rls19-road project whose road surface and whose DTM
// both sit at datumZ, and returns LrDay at the single explicit receiver.
func runRLS19AtDatum(t *testing.T, datumZ float64) float64 {
	t.Helper()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")

	model := `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "rd-1", "kind": "source", "source_type": "line", "elevation_m": ` +
		jsonFloat(datumZ) + `},
      "geometry": {"type": "LineString", "coordinates": [[0, 0], [120, 0]]}
    },
    {
      "type": "Feature",
      "properties": {"id": "rcv-1", "kind": "receiver", "height_m": 4},
      "geometry": {"type": "Point", "coordinates": [60, 200]}
    }
  ]
}`

	err := os.WriteFile(modelPath, []byte(model), 0o600)
	if err != nil {
		t.Fatalf("write model: %v", err)
	}

	terrainPath := flatGeoTIFF(t, projectDir, datumZ)

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Datum", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "import", "--terrain", terrainPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "rls19-road", "--receiver-mode", "custom")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	run := proj.Runs[len(proj.Runs)-1]

	payload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "runs", run.ID, "results", "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable

	err = json.Unmarshal(payload, &table)
	if err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	if len(table.Records) != 1 {
		t.Fatalf("expected 1 receiver, got %d", len(table.Records))
	}

	level, ok := table.Records[0].Values[rls19road.IndicatorLrDay]
	if !ok {
		t.Fatalf("receiver record carries no %s: %#v", rls19road.IndicatorLrDay, table.Records[0])
	}

	return level
}

// jsonFloat spells a float for embedding in the model literal above.
func jsonFloat(v float64) string {
	encoded, err := json.Marshal(v)
	if err != nil {
		return "0"
	}

	return string(encoded)
}

// The whole run path, from `aconiq import --terrain` to the receiver table: a
// road and a receiver on flat ground 400 m above sea level must produce the
// level they produce at sea level. The site's altitude is not an acoustic
// property.
//
// Before the fix the upland run came out 4.4 dB louder, because h_m was the
// path's height above the datum rather than above the ground, which drove
// D_gr = 4.8 − (2·h_m/s_gr)·(17 + 300/s_gr) negative and clamped it to 0.
func TestRunRLS19RoadIsInvariantUnderTheSiteDatum(t *testing.T) {
	t.Parallel()

	seaLevel := runRLS19AtDatum(t, 0)
	upland := runRLS19AtDatum(t, 400)

	if math.Abs(seaLevel-upland) > 0.05 {
		t.Fatalf("the same site 400 m up is %.2f dB louder (%.2f vs %.2f): the ground attenuation was lost with the datum",
			upland-seaLevel, upland, seaLevel)
	}
}
