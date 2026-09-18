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
//
// The comparison is exact, and stays exact now that the propagation path also
// samples the DTM along its length — but that exactness rests on the grid. A
// terrain model interpolates bilinearly, v00·(1−fx)(1−fy) + v10·fx(1−fy) +
// v01·(1−fx)fy + v11·fx·fy, whose four weights sum to 1 only up to rounding: at
// a plateau of 0 m every product is exactly 0, while at 400 m an awkward
// fraction returns 400 ± one ULP (≈ 5.7e−14 m). Here every sample lands on a
// pixel centre or exactly half way between two (the DTM is 10 m and so is the
// integration step), where the weights are 0, 1 or ½ and the sum is exact. At
// 50 m pixels the same scene drifts by 1.4e−14 dB. So a failure here in the
// 1e−13 dB range means the grid changed, not the physics; anything larger is
// the real thing — the defect this test guards against was 7.0 dB.
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
				t.Errorf("receiver %q %s: %.6f dB at sea level, %.6f dB 400 m up (%.6g dB)",
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

	return runSchall03GroundSceneOn(t, groundZ, func(_, _ float64) float64 { return groundZ })
}

// runSchall03GroundSceneOn is runSchall03GroundScene over an arbitrary surface:
// surfaceZ gives the DTM elevation at a point in the project CRS, so the same
// scene can be run over a plateau, a slope or a ridge without a second copy of
// the pipeline. groundZ still sets the track's elevation_m.
func runSchall03GroundSceneOn(t *testing.T, groundZ float64, surfaceZ func(x, y float64) float64) map[string]map[string]float64 {
	t.Helper()

	projectDir := t.TempDir()

	modelPath := filepath.Join(projectDir, "model.geojson")

	err := os.WriteFile(modelPath, []byte(schall03GroundModelJSON(groundZ)), 0o600)
	if err != nil {
		t.Fatalf("write model: %v", err)
	}

	// The DTM covers x ∈ [-200, 750] and y ∈ [-350, 600] at 10 m resolution,
	// comfortably around the track and all three receivers. The resolution is
	// what lets a caller put relief *between* the track and a receiver without
	// disturbing the ground either of them stands on: the track (y = 0) and all
	// three receivers (y = 60, 250, −60; x = 200) sit exactly on pixel centres,
	// so their own elevations are read straight off the grid with no
	// interpolation across a neighbouring row.
	terrainPath := filepath.Join(projectDir, "dtm.tif")
	writeGeoTIFF(t, terrainPath, 96, 96, -200, 600, 10, surfaceZ)

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

// writeFlatGeoTIFF writes a DTM whose every pixel holds the same elevation — a
// level plateau, which is the flat ground the old h_m simplification assumed
// everywhere.
func writeFlatGeoTIFF(t *testing.T, path string, width, height int, originX, originY, pixelSize, elevation float64) {
	t.Helper()

	writeGeoTIFF(t, path, width, height, originX, originY, pixelSize, func(_, _ float64) float64 { return elevation })
}

// writeGeoTIFF writes an uncompressed single-strip float32 GeoTIFF sampling
// surfaceZ at each pixel centre. originX/originY are the upper-left *pixel
// centre* in the project CRS, which is the tie point the reader expects, so
// pixel (col, row) sits at (originX + col·pixelSize, originY − row·pixelSize).
func writeGeoTIFF(t *testing.T, path string, width, height int, originX, originY, pixelSize float64, surfaceZ func(x, y float64) float64) {
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
		x := originX + float64(i%width)*pixelSize
		y := originY - float64(i/width)*pixelSize

		order.PutUint32(buf[pixelOffset+i*bytesPerPixel:], math.Float32bits(float32(surfaceZ(x, y))))
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

// schall03HollowDepthM is the depth of the hollow the test below puts between
// the track and the `open` receiver. Three metres is a drainage swale or an old
// borrow pit: ordinary relief, chosen shallow so the measured delta is what a
// commonplace site costs rather than a worst case, and so that Gl. 14's ≥ 0 dB
// clamp does not fire and flatten the comparison.
const schall03HollowDepthM = 3.0

// TestRunSchall03FollowsTheTerrainBetweenSourceAndReceiver is the end-to-end
// statement of Gl. 15: h_m = S/d is the mean height of the path above the
// *terrain profile*, not above one level plane carrying the whole site.
//
// Both runs use the same model. The two DTMs agree exactly at the track and at
// every receiver — the hollow lives on the pixel rows y = −20, −30 and −40 m,
// and the track and all three receivers sit on rows of their own that bilinear
// interpolation never mixes it into — so the reference plane of deviation 10 is
// identical in both runs and the only thing that differs is the ground
// *between* the ends. That isolates the term under test: any level change is
// the h_m correction and nothing else.
//
// `near` (60 m out, behind the wall) and `far` (250 m out, behind the wall)
// have paths on the other side of the track and must not move at all. `open`
// is 60 m out on the hollow's side with nothing between it and the track: its
// path runs over the hollow, the mean ground under it drops, and h_m — the mean
// height of the path *above that ground* — grows. Gl. 14 carries h_m with a
// minus sign, so A_gr,B shrinks and the receiver gets louder. This is the
// direction that matters: the flat-ground reading claimed a ground attenuation
// the terrain does not provide.
func TestRunSchall03FollowsTheTerrainBetweenSourceAndReceiver(t *testing.T) {
	t.Parallel()

	const groundZ = 100.0

	level := runSchall03GroundSceneOn(t, groundZ, func(_, _ float64) float64 { return groundZ })

	hollow := runSchall03GroundSceneOn(t, groundZ, func(_, y float64) float64 {
		if y == -20 || y == -30 || y == -40 {
			return groundZ - schall03HollowDepthM
		}

		return groundZ
	})

	for _, id := range []string{"near", "far"} {
		for _, indicator := range []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight} {
			if hollow[id][indicator] != level[id][indicator] {
				t.Errorf("receiver %q %s moved although its path never reaches the hollow: %.6f dB against %.6f dB",
					id, indicator, hollow[id][indicator], level[id][indicator])
			}
		}
	}

	for _, indicator := range []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight} {
		flat := level["open"][indicator]

		corrected := hollow["open"][indicator]
		if corrected <= flat {
			t.Errorf("%s at `open`: %.6f dB over the hollow against %.6f dB over level ground — Gl. 14 requires the hollow to remove ground attenuation",
				indicator, corrected, flat)
		}

		// The measured magnitude, recorded here and in CHANGELOG.md. It is
		// asserted as a band rather than a point so that an unrelated change to
		// the emission chain does not rewrite this test, while losing the terrain
		// term altogether (delta 0) or double-counting it does.
		delta := corrected - flat
		if delta < 0.5 || delta > 2.0 {
			t.Errorf("%s at `open`: the hollow changed the level by %.4f dB, expected 0.5 to 2.0 dB", indicator, delta)
		}

		t.Logf("%s at `open`: level ground %.4f dB, %g m hollow %.4f dB (%.4f dB louder)",
			indicator, flat, schall03HollowDepthM, corrected, delta)
	}
}
