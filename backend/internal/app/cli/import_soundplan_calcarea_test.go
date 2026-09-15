package cli

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/soundplanimport"
)

// The customer bundle under interoperability/ is gitignored, so everything here
// is built from synthetic SoundPLAN input: a bundle value for the model builder,
// and hand-written .geo records for the end-to-end path.

// calcAreaBundle returns a minimal bundle whose only geometry is the
// calculation area described by points.
func calcAreaBundle(points []soundplanimport.Point3D) *soundplanimport.ProjectBundle {
	bundle := &soundplanimport.ProjectBundle{
		Project: &soundplanimport.Project{Title: "Synthetic"},
	}

	if points != nil {
		bundle.CalcArea = &soundplanimport.CalcArea{Points: points}
	}

	return bundle
}

// closedCalcAreaPoints is a rectangle whose last vertex repeats the first.
func closedCalcAreaPoints() []soundplanimport.Point3D {
	return []soundplanimport.Point3D{
		{X: 2000, Y: 3000, Z: 210},
		{X: 2400, Y: 3000, Z: 210},
		{X: 2400, Y: 3300, Z: 210},
		{X: 2000, Y: 3300, Z: 210},
		{X: 2000, Y: 3000, Z: 210},
	}
}

// onlyCalcAreaFeature returns the model's single calc-area feature.
func onlyCalcAreaFeature(t *testing.T, model modelgeojson.Model) modelgeojson.Feature {
	t.Helper()

	var (
		found modelgeojson.Feature
		count int
	)

	for _, feature := range model.Features {
		if feature.Kind == modelgeojson.FeatureKindCalcArea {
			found = feature
			count++
		}
	}

	if count != 1 {
		t.Fatalf("expected exactly one calc-area feature, got %d", count)
	}

	return found
}

// calcAreaRing returns the outer ring of a calc-area feature's polygon.
func calcAreaRing(t *testing.T, feature modelgeojson.Feature) [][2]float64 {
	t.Helper()

	rings, ok := feature.Coordinates.([]any)
	if !ok || len(rings) != 1 {
		t.Fatalf("expected exactly one polygon ring, got %#v", feature.Coordinates)
	}

	rawRing, ok := rings[0].([]any)
	if !ok {
		t.Fatalf("expected a coordinate array, got %#v", rings[0])
	}

	ring := make([][2]float64, 0, len(rawRing))

	for _, rawPoint := range rawRing {
		point, pointOK := rawPoint.([]any)
		if !pointOK || len(point) != 2 {
			t.Fatalf("expected a 2D coordinate, got %#v", rawPoint)
		}

		x, xOK := point[0].(float64)

		y, yOK := point[1].(float64)
		if !xOK || !yOK {
			t.Fatalf("expected float64 coordinates, got %#v", point)
		}

		ring = append(ring, [2]float64{x, y})
	}

	return ring
}

func TestSoundPlanImportEmitsTheCalculationArea(t *testing.T) {
	t.Parallel()

	model, report := buildSoundPlanModelAndReport(calcAreaBundle(closedCalcAreaPoints()), "EPSG:25832", "synthetic")

	feature := onlyCalcAreaFeature(t, model)

	if feature.GeometryType != modelgeojson.GeometryTypePolygon {
		t.Fatalf("geometry type = %q, want Polygon", feature.GeometryType)
	}

	// A calculation area is a footprint on the ground, so height_m must stay
	// absent rather than be defaulted the way buildings and barriers are.
	if feature.HeightM != nil {
		t.Fatalf("calc-area feature must carry no height_m, got %v", *feature.HeightM)
	}

	if feature.ID == "" {
		t.Fatal("calc-area feature requires an id")
	}

	ring := calcAreaRing(t, feature)
	if len(ring) != len(closedCalcAreaPoints()) {
		t.Fatalf("ring has %d coordinates, want %d", len(ring), len(closedCalcAreaPoints()))
	}

	if ring[0] != ring[len(ring)-1] {
		t.Fatalf("ring is not closed: %v vs %v", ring[0], ring[len(ring)-1])
	}

	if report.CountsByKind[modelgeojson.FeatureKindCalcArea] != 1 {
		t.Fatalf("report calc-area count = %d, want 1", report.CountsByKind[modelgeojson.FeatureKindCalcArea])
	}

	validation := modelgeojson.Validate(model)
	if validation.ErrorCount() > 0 {
		t.Fatalf("model does not validate: %#v", validation.Errors)
	}
}

// The parser hands back whatever CalcArea.geo contains, and a bundle may leave
// the ring open. GeoJSON does not, so the importer closes it.
func TestSoundPlanImportClosesAnOpenCalculationArea(t *testing.T) {
	t.Parallel()

	open := closedCalcAreaPoints()[:4]

	model, _ := buildSoundPlanModelAndReport(calcAreaBundle(open), "EPSG:25832", "synthetic")

	ring := calcAreaRing(t, onlyCalcAreaFeature(t, model))
	if len(ring) != len(open)+1 {
		t.Fatalf("ring has %d coordinates, want %d", len(ring), len(open)+1)
	}

	if ring[0] != ring[len(ring)-1] {
		t.Fatalf("ring is not closed: %v vs %v", ring[0], ring[len(ring)-1])
	}

	validation := modelgeojson.Validate(model)
	if validation.ErrorCount() > 0 {
		t.Fatalf("model does not validate: %#v", validation.Errors)
	}
}

// The import report compares Z as well, so a footprint that closes in plan but
// not in elevation must not gain a zero-length closing segment.
func TestSoundPlanImportDoesNotRepeatAVertexClosedOnlyInPlan(t *testing.T) {
	t.Parallel()

	points := closedCalcAreaPoints()
	points[len(points)-1].Z += 5

	model, _ := buildSoundPlanModelAndReport(calcAreaBundle(points), "EPSG:25832", "synthetic")

	ring := calcAreaRing(t, onlyCalcAreaFeature(t, model))
	if len(ring) != len(points) {
		t.Fatalf("ring has %d coordinates, want %d", len(ring), len(points))
	}
}

func TestSoundPlanImportWithoutACalculationAreaEmitsNone(t *testing.T) {
	t.Parallel()

	model, report := buildSoundPlanModelAndReport(calcAreaBundle(nil), "EPSG:25832", "synthetic")

	for _, feature := range model.Features {
		if feature.Kind == modelgeojson.FeatureKindCalcArea {
			t.Fatalf("unexpected calc-area feature %q", feature.ID)
		}
	}

	if report.CountsByKind[modelgeojson.FeatureKindCalcArea] != 0 {
		t.Fatalf("report calc-area count = %d, want 0", report.CountsByKind[modelgeojson.FeatureKindCalcArea])
	}

	if report.CalcArea != nil {
		t.Fatalf("unexpected calc_area metadata: %#v", report.CalcArea)
	}

	for _, warning := range report.Warnings {
		if strings.Contains(warning, "CalcArea.geo") {
			t.Fatalf("a bundle without a calculation area must warn about nothing: %q", warning)
		}
	}
}

// A degenerate area cannot form a polygon ring. Emitting it would fail
// validation and reject the entire bundle, so the import drops it and says so.
func TestSoundPlanImportSkipsADegenerateCalculationArea(t *testing.T) {
	t.Parallel()

	degenerate := closedCalcAreaPoints()[:2]

	model, report := buildSoundPlanModelAndReport(calcAreaBundle(degenerate), "EPSG:25832", "synthetic")

	for _, feature := range model.Features {
		if feature.Kind == modelgeojson.FeatureKindCalcArea {
			t.Fatalf("unexpected calc-area feature %q", feature.ID)
		}
	}

	warned := false

	for _, warning := range report.Warnings {
		if strings.Contains(warning, "cannot form a polygon ring") {
			warned = true
		}
	}

	if !warned {
		t.Fatalf("expected a warning about the degenerate calculation area, got %#v", report.Warnings)
	}
}

// ---------------------------------------------------------------------------
// Synthetic SoundPLAN bundle on disk
// ---------------------------------------------------------------------------

// geoRecordLen is the length of a `:G ` coordinate record in both CalcArea.geo
// and GeoRail.geo: a three-byte tag, three filler bytes, and four float64s.
const geoRecordLen = 6 + 4*8

// geoPointRecord encodes one `:G ` record carrying four float64 fields.
func geoPointRecord(values ...float64) []byte {
	record := make([]byte, geoRecordLen)
	copy(record, ":G ")

	for i, value := range values {
		binary.LittleEndian.PutUint64(record[6+i*8:], math.Float64bits(value))
	}

	return record
}

// writeSyntheticCalcAreaFile writes a CalcArea.geo holding one closed ring.
func writeSyntheticCalcAreaFile(t *testing.T, path string, minX, minY, maxX, maxY float64) {
	t.Helper()

	corners := [][2]float64{{minX, minY}, {maxX, minY}, {maxX, maxY}, {minX, maxY}, {minX, minY}}

	payload := make([]byte, 0, len(corners)*geoRecordLen)
	for _, corner := range corners {
		payload = append(payload, geoPointRecord(corner[0], corner[1], 200)...)
	}

	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write CalcArea.geo: %v", err)
	}
}

// writeSyntheticGeoRailFile writes a GeoRail.geo holding one named track with
// one segment: an `:O&` object group, a `:D1` name record, and `:G ` vertices.
func writeSyntheticGeoRailFile(t *testing.T, path string, name string, vertices [][2]float64) {
	t.Helper()

	payload := []byte(":O&")

	nameRecord := make([]byte, 15, 15+len(name))
	copy(nameRecord, ":D1")
	nameRecord[14] = byte(len(name))
	payload = append(payload, nameRecord...)
	payload = append(payload, name...)

	for _, vertex := range vertices {
		payload = append(payload, geoPointRecord(vertex[0], vertex[1], 210, 208)...)
	}

	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write GeoRail.geo: %v", err)
	}
}

// syntheticSoundPlanBundleDir writes a SoundPLAN project directory holding one
// rail track and, when withCalcArea is set, a calculation area 2 km away from
// it. The two extents cannot be confused for one another.
func syntheticSoundPlanBundleDir(t *testing.T, withCalcArea bool) string {
	t.Helper()

	dir := t.TempDir()

	projectFile := "[PROJECT]\nTITLE=Synthetic Bundle\nVERSION=1\n"
	if err := os.WriteFile(filepath.Join(dir, "Project.sp"), []byte(projectFile), 0o600); err != nil {
		t.Fatalf("write Project.sp: %v", err)
	}

	writeSyntheticGeoRailFile(t, filepath.Join(dir, "GeoRail.geo"), "Synthetic Gleis 1", [][2]float64{
		{0, 0}, {200, 100}, {400, 400},
	})

	if withCalcArea {
		writeSyntheticCalcAreaFile(t, filepath.Join(dir, "CalcArea.geo"),
			syntheticCalcAreaMinX, syntheticCalcAreaMinY, syntheticCalcAreaMaxX, syntheticCalcAreaMaxY)
	}

	return dir
}

// The synthetic calculation area sits well clear of the synthetic rail track,
// which spans [0,400]x[0,400].
const (
	syntheticCalcAreaMinX = 2000.0
	syntheticCalcAreaMinY = 3000.0
	syntheticCalcAreaMaxX = 2200.0
	syntheticCalcAreaMaxY = 3200.0
)

// TestSoundPlanImportThenRunUsesTheBundlesCalculationArea is the payoff: the
// emitted feature travels through the normalized model into `aconiq run`, so an
// imported project's receiver grid covers the extent the SoundPLAN author drew
// instead of the extent the imported sources happen to span.
func TestSoundPlanImportThenRunUsesTheBundlesCalculationArea(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	bundleDir := syntheticSoundPlanBundleDir(t, true)

	mustRunCLI(t, "--project", projectDir, "init", "--name", "SoundPLAN CalcArea", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--from-soundplan", bundleDir)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "schall03",
		"--param", "grid_resolution_m=50", "--param", "grid_padding_m=0",
		"--param", "schall03_engine=preview")

	runDir := latestRunDir(t, projectDir)

	logPayload, err := os.ReadFile(filepath.Join(runDir, "run.log"))
	if err != nil {
		t.Fatalf("read run.log: %v", err)
	}

	if !strings.Contains(string(logPayload), "grid_extent="+gridExtentCalcArea) {
		t.Fatalf("run.log does not record the calculation area as the grid extent:\n%s", logPayload)
	}

	minX, minY, maxX, maxY := receiverTableBounds(t, filepath.Join(runDir, "results", "receivers.csv"))
	if minX != syntheticCalcAreaMinX || minY != syntheticCalcAreaMinY ||
		maxX != syntheticCalcAreaMaxX || maxY != syntheticCalcAreaMaxY {
		t.Fatalf("receivers cover [%.1f,%.1f]x[%.1f,%.1f], not the bundle's calculation area", minX, maxX, minY, maxY)
	}
}

// The same bundle without CalcArea.geo must behave exactly as it did before the
// feature existed: no calc-area feature, and a grid over the source extent.
func TestSoundPlanImportThenRunFallsBackToTheSourceExtent(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	bundleDir := syntheticSoundPlanBundleDir(t, false)

	mustRunCLI(t, "--project", projectDir, "init", "--name", "SoundPLAN NoCalcArea", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--from-soundplan", bundleDir)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "schall03",
		"--param", "grid_resolution_m=50", "--param", "grid_padding_m=0",
		"--param", "schall03_engine=preview")

	runDir := latestRunDir(t, projectDir)

	logPayload, err := os.ReadFile(filepath.Join(runDir, "run.log"))
	if err != nil {
		t.Fatalf("read run.log: %v", err)
	}

	if !strings.Contains(string(logPayload), "grid_extent="+gridExtentSource) {
		t.Fatalf("run.log does not record the source extent as the grid extent:\n%s", logPayload)
	}
}
