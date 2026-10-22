package cli

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/io/soundplanimport"
	exportfmt "github.com/aconiq/backend/internal/report/export"
)

// soundPLANBundle returns the soundplan directory of the project's only
// export bundle, and its decoded mapping report.
func soundPLANBundle(t *testing.T, projectDir string) (string, exportfmt.SoundPLANReport) {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(projectDir, ".noise", "exports", "*", "formats", "soundplan", exportfmt.SoundPLANReportFile))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one SoundPLAN mapping report, got %v (%v)", matches, err)
	}

	payload, err := os.ReadFile(filepath.Clean(matches[0]))
	if err != nil {
		t.Fatal(err)
	}

	var report exportfmt.SoundPLANReport

	err = json.Unmarshal(payload, &report)
	if err != nil {
		t.Fatalf("decode %s: %v", matches[0], err)
	}

	return filepath.Dir(matches[0]), report
}

// firstShapePoint reads the first record of a point Shapefile.
func firstShapePoint(t *testing.T, path string) geo.Point2D {
	t.Helper()

	shp, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	// 100-byte header, 8-byte record header, 4-byte shape type, then X and Y.
	const at = 100 + 8 + 4

	return geo.Point2D{
		X: math.Float64frombits(binary.LittleEndian.Uint64(shp[at:])),
		Y: math.Float64frombits(binary.LittleEndian.Uint64(shp[at+8:])),
	}
}

func TestExportSoundPLANCarriesTheModelAndTheTerrain(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")
	dtmPath := filepath.Join(projectDir, "dtm.tif")

	writeSourceAndReceiverModel(t, modelPath, geo.Point2D{X: 500000, Y: 5800000}, geo.Point2D{X: 500050, Y: 5800000})
	writeFlatGeoTIFF(t, dtmPath, 4, 3, 499990, 5800020, 10, 55.25)

	mustRunCLI(t, "--project", projectDir, "init", "--name", "SoundPLAN", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath, "--terrain", dtmPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "dummy-freefield", "--receiver-mode", "custom")
	mustRunCLI(t, "--project", projectDir, "export", "--format", "soundplan-shp")

	dir, report := soundPLANBundle(t, projectDir)

	if report.CRS != "EPSG:25832" || report.SourceCRS != "" {
		t.Errorf("crs %q from %q, want EPSG:25832 unprojected", report.CRS, report.SourceCRS)
	}

	layers := make([]string, 0, len(report.Layers))
	for _, l := range report.Layers {
		layers = append(layers, l.Layer)
	}

	if strings.Join(layers, ",") != "receivers,sources_point" {
		t.Errorf("layers %v", layers)
	}

	if got := firstShapePoint(t, filepath.Join(dir, "sources_point.shp")); got != (geo.Point2D{X: 500000, Y: 5800000}) {
		t.Errorf("source at %v", got)
	}

	if report.Terrain == nil || report.Terrain.Points != 12 {
		t.Fatalf("terrain %+v, want all 12 DTM cells", report.Terrain)
	}

	points, err := soundplanimport.ParseHoehenTxtFile(filepath.Join(dir, exportfmt.SoundPLANTerrainFile))
	if err != nil {
		t.Fatal(err)
	}

	if first := points[0]; first.X != 499990 || first.Y != 5800020 || first.Z != 55.25 {
		t.Errorf("first terrain point %+v, want the north-west cell centre at 55.25 m", first)
	}
}

// A geographic project computes in the UTM zone the run chose, and that is
// where SoundPLAN gets the model: it would measure degrees as metres.
func TestExportSoundPLANProjectsAGeographicProjectWhereTheRunComputed(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")

	// Hannover, UTM zone 32.
	writeSourceAndReceiverModel(t, modelPath, geo.Point2D{X: 9.7320, Y: 52.3759}, geo.Point2D{X: 9.7330, Y: 52.3759})

	mustRunCLI(t, "--project", projectDir, "init", "--name", "SoundPLAN", "--crs", "EPSG:4326")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "dummy-freefield", "--receiver-mode", "custom")
	mustRunCLI(t, "--project", projectDir, "export", "--format", "soundplan-shp")

	dir, report := soundPLANBundle(t, projectDir)

	if report.CRS != "EPSG:25832" || report.SourceCRS != "EPSG:4326" {
		t.Fatalf("crs %q from %q, want EPSG:25832 from EPSG:4326", report.CRS, report.SourceCRS)
	}

	got := firstShapePoint(t, filepath.Join(dir, "sources_point.shp"))
	if got.X < 540000 || got.X > 560000 || got.Y < 5790000 || got.Y > 5815000 {
		t.Errorf("source at %v, not in Hannover's UTM 32 coordinates", got)
	}
}

// --target-crs reprojects the bundle's model GeoJSON in place; the SoundPLAN
// layers still come out in the CRS the run computed in, not in the target.
func TestExportSoundPLANIgnoresTheTargetCRSOfTheBundleModel(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")

	writeSourceAndReceiverModel(t, modelPath, geo.Point2D{X: 550000, Y: 5803000}, geo.Point2D{X: 550050, Y: 5803000})

	mustRunCLI(t, "--project", projectDir, "init", "--name", "SoundPLAN", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "dummy-freefield", "--receiver-mode", "custom")
	mustRunCLI(t, "--project", projectDir, "export", "--target-crs", "EPSG:4326", "--format", "soundplan-shp")

	dir, report := soundPLANBundle(t, projectDir)

	if report.CRS != "EPSG:25832" || report.SourceCRS != "EPSG:4326" {
		t.Fatalf("crs %q from %q", report.CRS, report.SourceCRS)
	}

	// Through 4326 and back: the round trip of Priority 1.6, millimetres.
	got := firstShapePoint(t, filepath.Join(dir, "sources_point.shp"))
	if math.Abs(got.X-550000) > 0.01 || math.Abs(got.Y-5803000) > 0.01 {
		t.Errorf("source at %v, want (550000, 5803000)", got)
	}
}

func TestExportSoundPLANRefusesWebMercator(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")

	writeSourceAndReceiverModel(t, modelPath, geo.Point2D{X: 1083000, Y: 6865000}, geo.Point2D{X: 1083050, Y: 6865000})

	mustRunCLI(t, "--project", projectDir, "init", "--name", "SoundPLAN", "--crs", "EPSG:3857")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "dummy-freefield", "--receiver-mode", "custom")

	err := runCLI("--project", projectDir, "export", "--format", "soundplan-shp")
	if err == nil || !strings.Contains(err.Error(), "not a metric conformal CRS") {
		t.Fatalf("export over Web Mercator: %v", err)
	}

	// The CRS is the user's to change, so the refusal exits 2, not 1.
	if !domainerrors.IsUserError(err) {
		t.Errorf("refusal is not classified as user input: %v", err)
	}
}
