package cli

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// projectedToGeographic converts an EPSG:25832 coordinate to EPSG:4326, so a
// test can state one physical layout once and express it in both CRS.
func projectedToGeographic(t *testing.T, point geo.Point2D) geo.Point2D {
	t.Helper()

	from, err := geo.ParseCRS("EPSG:25832")
	if err != nil {
		t.Fatalf("parse EPSG:25832: %v", err)
	}

	to, err := geo.ParseCRS("EPSG:4326")
	if err != nil {
		t.Fatalf("parse EPSG:4326: %v", err)
	}

	pipeline, err := geo.BuildTransformPipeline(to, from)
	if err != nil {
		t.Fatalf("build transform: %v", err)
	}

	out, err := pipeline.ApplyPoint(point)
	if err != nil {
		t.Fatalf("transform %v: %v", point, err)
	}

	return out
}

func writeSourceAndReceiverModel(t *testing.T, path string, source, receiver geo.Point2D) {
	t.Helper()

	payload := fmt.Sprintf(
		`{"type":"FeatureCollection","features":[
  {"type":"Feature","properties":{"id":"src-1","kind":"source","source_type":"point"},
   "geometry":{"type":"Point","coordinates":[%s,%s]}},
  {"type":"Feature","properties":{"id":"rec-1","kind":"receiver","height_m":4},
   "geometry":{"type":"Point","coordinates":[%s,%s]}}
]}`,
		strconv.FormatFloat(source.X, 'f', -1, 64), strconv.FormatFloat(source.Y, 'f', -1, 64),
		strconv.FormatFloat(receiver.X, 'f', -1, 64), strconv.FormatFloat(receiver.Y, 'f', -1, 64),
	)

	err := os.WriteFile(path, []byte(payload), 0o600)
	if err != nil {
		t.Fatalf("write model %s: %v", path, err)
	}
}

// runFreefieldLevel drives init → import → run in a fresh project and returns
// the single receiver's level.
func runFreefieldLevel(t *testing.T, crs string, source, receiver geo.Point2D) float64 {
	t.Helper()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")

	writeSourceAndReceiverModel(t, modelPath, source, receiver)

	mustRunCLI(t, "--project", projectDir, "init", "--name", "ComputeCRS", "--crs", crs)
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "dummy-freefield", "--receiver-mode", "custom")

	return singleReceiverLevel(t, projectDir)
}

func singleReceiverLevel(t *testing.T, projectDir string) float64 {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(projectDir, ".noise", "runs", "*", "results", "receivers.csv"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected exactly one receivers.csv under %s, got %v (%v)", projectDir, matches, err)
	}

	file, err := os.Open(matches[0])
	if err != nil {
		t.Fatalf("open %s: %v", matches[0], err)
	}

	defer func() { _ = file.Close() }()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read %s: %v", matches[0], err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected a header and one receiver row, got %d rows", len(rows))
	}

	level, err := strconv.ParseFloat(rows[1][len(rows[1])-1], 64)
	if err != nil {
		t.Fatalf("parse level %q: %v", rows[1][len(rows[1])-1], err)
	}

	return level
}

// The defect this pins: every standards module measures distance with
// geo.Distance, which is math.Hypot over the coordinates it is handed. Before
// the compute projection existed, a project in EPSG:4326 — the `aconiq init`
// default — measured a 100 m separation as 0.0009 units, fell under the
// module's 1 m clamp, and reported the source's 90 dB emission level verbatim
// at every receiver regardless of distance. The same layout in EPSG:25832
// reported 49.98 dB. A 40 dB disagreement between two spellings of one site is
// what this test refuses to let back in.
func TestRunAgreesBetweenGeographicAndProjectedProjectCRS(t *testing.T) {
	source := geo.Point2D{X: 566000, Y: 5934000}
	receiver := geo.Point2D{X: 566000, Y: 5934100}

	projectedLevel := runFreefieldLevel(t, "EPSG:25832", source, receiver)
	geographicLevel := runFreefieldLevel(t, "EPSG:4326",
		projectedToGeographic(t, source), projectedToGeographic(t, receiver))

	// 100 m from a 90 dB point source under 90 - 20 log10(d).
	const want = 50.0

	if math.Abs(projectedLevel-want) > 0.001 {
		t.Fatalf("projected run reported %.6f dB, want %.3f dB", projectedLevel, want)
	}

	// The tolerance covers one EPSG:25832 → 4326 → 25832 round trip of both
	// points. PLAN.md 1.6 pins that round trip's absolute residual at roughly
	// 0.5 m at this easting, but the residual is a bias pointing the same way
	// for two points 100 m apart, so it very nearly cancels in the distance
	// between them: the disagreement measured here is 4.9e-6 dB. The tolerance
	// is set two orders of magnitude above that rather than at the measurement,
	// which leaves room for the projection library to change without leaving
	// room for the defect to come back.
	const tolerance = 0.001

	if math.Abs(geographicLevel-projectedLevel) > tolerance {
		t.Fatalf("the same site reported %.6f dB in EPSG:4326 and %.6f dB in EPSG:25832 (%.6f dB apart); one spelling of a site must not change its levels",
			geographicLevel, projectedLevel, math.Abs(geographicLevel-projectedLevel))
	}
}

// The compute CRS travels with the numbers. A reader of provenance.json must
// be able to tell which CRS the levels were computed in without going back to
// the project manifest, which can be edited after the run.
func TestRunRecordsTheComputeCRSInProvenance(t *testing.T) {
	source := geo.Point2D{X: 566000, Y: 5934000}
	receiver := geo.Point2D{X: 566000, Y: 5934100}

	cases := []struct {
		name           string
		crs            string
		wantProjectCRS string
		wantComputeCRS string
		geographic     bool
	}{
		{"a geographic project is projected", "EPSG:4326", "EPSG:4326", "EPSG:25832", true},
		{"a projected project is left alone", "EPSG:25832", "EPSG:25832", "EPSG:25832", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projectDir := t.TempDir()
			modelPath := filepath.Join(projectDir, "model.geojson")

			src, rec := source, receiver
			if tc.geographic {
				src, rec = projectedToGeographic(t, source), projectedToGeographic(t, receiver)
			}

			writeSourceAndReceiverModel(t, modelPath, src, rec)

			mustRunCLI(t, "--project", projectDir, "init", "--name", "ComputeCRS", "--crs", tc.crs)
			mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
			mustRunCLI(t, "--project", projectDir, "run", "--standard", "dummy-freefield", "--receiver-mode", "custom")

			matches, err := filepath.Glob(filepath.Join(projectDir, ".noise", "runs", "*", "provenance.json"))
			if err != nil || len(matches) != 1 {
				t.Fatalf("expected one provenance.json, got %v (%v)", matches, err)
			}

			payload, err := os.ReadFile(matches[0])
			if err != nil {
				t.Fatalf("read provenance: %v", err)
			}

			for key, want := range map[string]string{"project_crs": tc.wantProjectCRS, "compute_crs": tc.wantComputeCRS} {
				needle := fmt.Sprintf("%q: %q", key, want)
				if !strings.Contains(string(payload), needle) {
					t.Fatalf("provenance does not record %s; got:\n%s", needle, payload)
				}
			}
		})
	}
}

// A site the CRS table cannot carry is refused. The run fails rather than
// computing somewhere else, and the message names the project CRS so the user
// knows which setting to change.
func TestRunRefusesAGeographicProjectOutsideTheSupportedZones(t *testing.T) {
	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")

	// Madrid: UTM zone 30, which EPSG:25831..25834 does not reach.
	writeSourceAndReceiverModel(t, modelPath,
		geo.Point2D{X: -3.7038, Y: 40.4168}, geo.Point2D{X: -3.7038, Y: 40.4177})

	mustRunCLI(t, "--project", projectDir, "init", "--name", "OutOfZone", "--crs", "EPSG:4326")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	err := runCLI("--project", projectDir, "run", "--standard", "dummy-freefield", "--receiver-mode", "custom")
	if err == nil {
		t.Fatal("a geographic project outside zones 31-34 ran to completion; it must be refused rather than computed elsewhere")
	}

	if !strings.Contains(err.Error(), "EPSG:4326") {
		t.Fatalf("error %q does not name the project CRS the user has to change", err.Error())
	}
}

func TestResolveComputeModelLeavesAProjectedModelUntouched(t *testing.T) {
	t.Parallel()

	model := modelgeojson.Model{
		ProjectCRS: "EPSG:25832",
		Features: []modelgeojson.Feature{{
			ID:           "src-1",
			Kind:         modelgeojson.FeatureKindSource,
			GeometryType: modelgeojson.GeometryTypePoint,
			Coordinates:  []any{566000.0, 5934000.0},
		}},
	}

	out, projection, err := resolveComputeModel(model, "EPSG:25832")
	if err != nil {
		t.Fatalf("resolveComputeModel: %v", err)
	}

	if projection.Applied {
		t.Fatal("a projected project reported a compute projection; the ordinary German project must run through unchanged")
	}

	point, _ := out.Features[0].Coordinates.([]any)

	x, _ := point[0].(float64)
	if x != 566000.0 {
		t.Fatalf("x = %v, want 566000", x)
	}
}

// A CRS whose kind cannot be determined has no EPSG code to transform through.
// Refusing it would break projects that work today, so it is left alone.
func TestResolveComputeModelLeavesAnUnclassifiedCRSAlone(t *testing.T) {
	t.Parallel()

	for _, crs := range []string{"WKT:LOCAL_CS[\"site\"]", "", "EPSG:1"} {
		model := modelgeojson.Model{
			ProjectCRS: crs,
			Features: []modelgeojson.Feature{{
				ID:           "src-1",
				Kind:         modelgeojson.FeatureKindSource,
				GeometryType: modelgeojson.GeometryTypePoint,
				Coordinates:  []any{10.0, 53.55},
			}},
		}

		_, projection, err := resolveComputeModel(model, crs)
		if err != nil {
			t.Fatalf("resolveComputeModel(%q): %v", crs, err)
		}

		if projection.Applied {
			t.Fatalf("CRS %q was reprojected; its kind is not known to be geographic", crs)
		}
	}
}
