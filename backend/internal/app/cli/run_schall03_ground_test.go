package cli

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// schall03TranslationM is the vertical shift the scene is tested against. It is
// the same figure the unit test in internal/standards/schall03 uses, and it is
// big enough that the defect it guards against would move every affected term
// by orders of magnitude.
const schall03TranslationM = 400.0

// TestRunSchall03IsInvariantWhenTheWholeSceneMovesUphill drives the defect
// through the CLI, end to end, because that is where the two datums actually
// meet: `elevation_m` reaches the track as an absolute Z (the SoundPLAN import
// writes the rail's ZTrack into it), while a receiver's `height_m` is a height
// above the ground it stands on, and only the imported DTM knows where that
// ground is.
//
// The two scenes below are the same site: one at sea level, one lifted 400 m —
// track, receiver and the ground beneath the receiver all together. Nothing
// about the acoustics changes, so neither may the level.
func TestRunSchall03IsInvariantWhenTheWholeSceneMovesUphill(t *testing.T) {
	t.Parallel()

	atSeaLevel := runSchall03GroundScene(t, 0)
	onThePlateau := runSchall03GroundScene(t, schall03TranslationM)

	if len(atSeaLevel) == 0 {
		t.Fatal("no receiver records")
	}

	assertSchall03WallStillScreens(t, atSeaLevel)
	assertSchall03WallStillScreens(t, onThePlateau)

	if len(onThePlateau) != len(atSeaLevel) {
		t.Fatalf("receiver count changed: %d vs %d", len(atSeaLevel), len(onThePlateau))
	}

	for id, sea := range atSeaLevel {
		plateau, ok := onThePlateau[id]
		if !ok {
			t.Fatalf("receiver %q is missing from the translated run", id)
		}

		for indicator, level := range sea {
			lifted := plateau[indicator]
			if lifted != level {
				t.Errorf("receiver %q %s: %.6f dB at sea level, %.6f dB 400 m up (%.6f dB)",
					id, indicator, level, lifted, lifted-level)
			}
		}
	}
}

// assertSchall03WallStillScreens checks that the scene's wall is doing
// something. `near` and `open` are the same 60 m from the track and differ only
// in which side of the wall they stand on, so a screened level that is not well
// below the open one means shielding has stopped being applied — and the
// translation invariance above would then be comparing two unshielded runs and
// passing for the wrong reason.
//
// The measured gap is about 9.8 dB; the margin below is set low enough that
// ordinary changes to the barrier chain do not trip it, and high enough that
// losing the barrier entirely does.
func assertSchall03WallStillScreens(t *testing.T, levels map[string]map[string]float64) {
	t.Helper()

	const minShieldingDB = 5.0

	for _, indicator := range []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight} {
		screened := levels["near"][indicator]
		open := levels["open"][indicator]

		if open-screened < minShieldingDB {
			t.Errorf("%s: screened receiver %.3f dB vs open receiver %.3f dB — the wall shields only %.3f dB, want at least %.1f dB",
				indicator, screened, open, open-screened, minShieldingDB)
		}
	}
}

// runSchall03GroundScene runs one scenario whose track sits at the absolute
// elevation groundZ, over a flat DTM at that same elevation, and returns the
// receiver indicators by receiver ID.
func runSchall03GroundScene(t *testing.T, groundZ float64) map[string]map[string]float64 {
	t.Helper()

	projectDir := t.TempDir()

	modelPath := filepath.Join(projectDir, "model.geojson")

	err := os.WriteFile(modelPath, []byte(schall03GroundModelJSON(groundZ)), 0o600)
	if err != nil {
		t.Fatalf("write model: %v", err)
	}

	// The DTM covers x, y ∈ [-200, 600] at 50 m resolution, comfortably around
	// the track and all three receivers.
	terrainPath := filepath.Join(projectDir, "dtm.tif")
	writeFlatGeoTIFF(t, terrainPath, 16, 16, -200, 600, 50, groundZ)

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Ground", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath, "--terrain", terrainPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "schall03", "--receiver-mode", "custom")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) == 0 {
		t.Fatal("expected one run")
	}

	run := proj.Runs[len(proj.Runs)-1]

	// Every receiver must sit on sampled ground; a miss would fall back to the
	// mean of the hits and the test would be measuring that fallback instead.
	logPayload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "runs", run.ID, "run.log"))
	if err != nil {
		t.Fatalf("read run.log: %v", err)
	}

	if !strings.Contains(string(logPayload), "schall03_receiver_terrain_samples=3/3") {
		t.Fatalf("the DTM did not cover every receiver:\n%s", logPayload)
	}

	payload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "runs", run.ID, "results", "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable

	err = json.Unmarshal(payload, &table)
	if err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	levels := make(map[string]map[string]float64, len(table.Records))

	for _, record := range table.Records {
		levels[record.ID] = maps.Clone(record.Values)
	}

	// A silent receiver would make the comparison vacuous.
	for id, values := range levels {
		for _, indicator := range []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight} {
			value, ok := values[indicator]
			if !ok {
				t.Fatalf("receiver %q carries no %s", id, indicator)
			}

			if value <= 0 {
				t.Fatalf("receiver %q %s = %v; the scene produced no level to compare", id, indicator, value)
			}
		}
	}

	return levels
}

// schall03GroundModelJSON is one straight electrified line, a Schallschutzwand
// beside it and three receivers, all resting on ground at the absolute
// elevation groundZ. `elevation_m` is the absolute Z of the Schienenoberkante,
// so it moves with the ground.
//
// The wall runs along y = 20, between the track and the positive-y side, so
// which side a receiver stands on decides whether it is screened. `near` is
// 60 m out behind the wall and `far` is 250 m out behind it, where Gl. 14's
// `A_gr,B` is well clear of its ≥ 0 dB clamp and the defect had the most room
// to act. `open` is the unscreened control: 60 m out on the source side, the
// same distance as `near` and with nothing between it and the track, so the
// two differ by the shielding alone.
func schall03GroundModelJSON(groundZ float64) string {
	return fmt.Sprintf(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "rail-1",
        "kind": "source",
        "source_type": "line",
        "elevation_m": %[1]g,
        "schall03_fahrbahn": "schwellengleis",
        "schall03_strecke_max_kph": 160,
        "schall03_operations": [
          {"zugart": "Nahverkehrszug-ET", "trains_per_hour_day": 4, "trains_per_hour_night": 1},
          {"zugart": "Gueterzug-E-Lok", "speed_kph": 100, "trains_per_hour_day": 1, "trains_per_hour_night": 2}
        ]
      },
      "geometry": {"type": "LineString", "coordinates": [[0, 0], [400, 0]]}
    },
    {
      "type": "Feature",
      "properties": {"id": "wall-1", "kind": "barrier", "height_m": 4, "schall03_reflective": false},
      "geometry": {"type": "LineString", "coordinates": [[0, 20], [400, 20]]}
    },
    {
      "type": "Feature",
      "properties": {"id": "near", "kind": "receiver", "height_m": 3.5},
      "geometry": {"type": "Point", "coordinates": [200, 60]}
    },
    {
      "type": "Feature",
      "properties": {"id": "far", "kind": "receiver", "height_m": 3.5},
      "geometry": {"type": "Point", "coordinates": [200, 250]}
    },
    {
      "type": "Feature",
      "properties": {"id": "open", "kind": "receiver", "height_m": 3.5},
      "geometry": {"type": "Point", "coordinates": [200, -60]}
    }
  ]
}`, groundZ)
}

// writeFlatGeoTIFF writes an uncompressed single-strip float32 GeoTIFF whose
// every pixel holds the same elevation — a level plateau, which is exactly the
// flat ground Anlage 2's h_m simplification assumes. originX/originY are the
// upper-left corner in the project CRS.
func writeFlatGeoTIFF(t *testing.T, path string, width, height int, originX, originY, pixelSize, elevation float64) {
	t.Helper()

	order := binary.LittleEndian

	const (
		bytesPerPixel = 4
		numTags       = 9
		pixelOffset   = 8
	)

	pixelDataSize := width * height * bytesPerPixel
	ifdOffset := pixelOffset + pixelDataSize
	ifdSize := 2 + numTags*12 + 4
	scaleDataOffset := ifdOffset + ifdSize
	tiepointOffset := scaleDataOffset + 24

	buf := make([]byte, tiepointOffset+48)

	buf[0] = 'I'
	buf[1] = 'I'
	order.PutUint16(buf[2:], 42)
	order.PutUint32(buf[4:], uint32(ifdOffset))

	for i := range width * height {
		order.PutUint32(buf[pixelOffset+i*bytesPerPixel:], math.Float32bits(float32(elevation)))
	}

	pos := ifdOffset
	order.PutUint16(buf[pos:], uint16(numTags))
	pos += 2

	writeTag := func(tag, dataType uint16, count, value uint32) {
		order.PutUint16(buf[pos:], tag)
		order.PutUint16(buf[pos+2:], dataType)
		order.PutUint32(buf[pos+4:], count)
		order.PutUint32(buf[pos+8:], value)
		pos += 12
	}

	writeTag(256, 3, 1, uint32(width))              // ImageWidth
	writeTag(257, 3, 1, uint32(height))             // ImageLength
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

	err := os.WriteFile(path, buf, 0o600)
	if err != nil {
		t.Fatalf("write GeoTIFF: %v", err)
	}
}

// TestRunSchall03RefusesATerrainThatCoversNoReceiver pins the second half of
// the datum fix. A DTM that reaches not one receiver leaves the run with no
// ground to measure against, and taking Z = 0 there would put the whole scene
// back on the sea-level datum this work removed — silently, on a project that
// imported terrain precisely so that would not happen. The run is refused
// instead, and as a user error, because the cause is always a CRS or an extent
// the user can correct.
func TestRunSchall03RefusesATerrainThatCoversNoReceiver(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	modelPath := filepath.Join(projectDir, "model.geojson")

	err := os.WriteFile(modelPath, []byte(schall03GroundModelJSON(0)), 0o600)
	if err != nil {
		t.Fatalf("write model: %v", err)
	}

	// A DTM at real EPSG:25832 coordinates, while the model sits near the
	// origin — the shape a CRS mismatch takes in practice.
	terrainPath := filepath.Join(projectDir, "dtm.tif")
	writeFlatGeoTIFF(t, terrainPath, 16, 16, 500000, 5600000, 50, 120)

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Ground", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath, "--terrain", terrainPath)

	err = runCLI("--project", projectDir, "run", "--standard", "schall03", "--receiver-mode", "custom")
	if err == nil {
		t.Fatal("the run was accepted although the DTM covers no receiver")
	}

	var appErr *domainerrors.AppError

	if !errors.As(err, &appErr) {
		t.Fatalf("error is not an AppError: %v", err)
	}

	// KindUserInput is what exits the CLI with code 2.
	if appErr.Kind != domainerrors.KindUserInput {
		t.Errorf("error kind = %q, want %q", appErr.Kind, domainerrors.KindUserInput)
	}

	for _, want := range []string{"covers none of the", "CRS", "aconiq import --terrain"} {
		if !strings.Contains(appErr.Error(), want) {
			t.Errorf("the refusal does not mention %q:\n%v", want, appErr)
		}
	}
}

// TestRunSchall03WarnsWhenElevationHasNoGroundUnderIt covers the case the
// refusal above cannot reach: no DTM at all, and a track carrying a non-zero
// elevation_m. The reading the run then takes — elevation_m as a height above
// the receiver's ground — is right for a hand-written bridge deck and wrong for
// a SoundPLAN import, where it is an absolute Z and no terrain artifact is
// produced to correct it. Both scenes must keep running, so the run says which
// reading it took instead of refusing.
func TestRunSchall03WarnsWhenElevationHasNoGroundUnderIt(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	modelPath := filepath.Join(projectDir, "model.geojson")

	err := os.WriteFile(modelPath, []byte(schall03GroundModelJSON(120)), 0o600)
	if err != nil {
		t.Fatalf("write model: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Ground", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "schall03", "--receiver-mode", "custom")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) == 0 {
		t.Fatal("expected one run")
	}

	run := proj.Runs[len(proj.Runs)-1]

	logPayload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "runs", run.ID, "run.log"))
	if err != nil {
		t.Fatalf("read run.log: %v", err)
	}

	for _, want := range []string{
		"WARNING no terrain model",
		"height above the ground under each receiver",
		"SoundPLAN",
		"aconiq import --terrain",
	} {
		if !strings.Contains(string(logPayload), want) {
			t.Errorf("run.log does not mention %q:\n%s", want, logPayload)
		}
	}
}
