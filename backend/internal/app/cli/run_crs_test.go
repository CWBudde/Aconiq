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
	// points. That round trip used to lose ~0.5 m at this easting, which mostly
	// cancelled between two points 100 m apart and left 4.9e-6 dB here; since
	// PLAN.md 1.6 closed it the round trip is lossless to nanometres and the
	// disagreement is 8.1e-11 dB. The tolerance stays where it was rather than
	// following the measurement down: what it is for is the claim that one
	// spelling of a site must not change its levels, and 0.001 dB is already
	// two orders below the reporting precision that claim is about.
	const tolerance = 0.001

	if math.Abs(geographicLevel-projectedLevel) > tolerance {
		t.Fatalf("the same site reported %.6f dB in EPSG:4326 and %.6f dB in EPSG:25832 (%.6f dB apart); one spelling of a site must not change its levels",
			geographicLevel, projectedLevel, math.Abs(geographicLevel-projectedLevel))
	}
}

// The compute CRS travels with the numbers. A reader of provenance.json must
// be able to tell which CRS the levels were computed in without going back to
// the project manifest, which can be edited after the run.
// The CRS a run computed in has to reach both files, for two different
// readers.
//
// provenance.json is the audit record. run-summary.json is what the local API
// serves — provenance is not an ArtifactRef, so `GET /api/v1/artifacts/{id}`
// never reaches it — and it is therefore the only place a consumer holding a
// receiver table can learn which CRS its x/y are in. Browser mode writes the
// same two keys into its own summary, so the map projects a stored run the
// same way on both targets.
func TestRunRecordsTheComputeCRSInProvenanceAndTheRunSummary(t *testing.T) {
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

			wanted := map[string]string{"project_crs": tc.wantProjectCRS, "compute_crs": tc.wantComputeCRS}

			for _, name := range []string{"provenance.json", filepath.Join("results", "run-summary.json")} {
				payload := readSingleRunFile(t, projectDir, name)

				for key, want := range wanted {
					needle := fmt.Sprintf("%q: %q", key, want)
					if !strings.Contains(payload, needle) {
						t.Fatalf("%s does not record %s; got:\n%s", name, needle, payload)
					}
				}
			}
		})
	}
}

// readSingleRunFile returns the contents of one file under the project's only
// run directory, failing the test when the project holds anything but one run.
func readSingleRunFile(t *testing.T, projectDir string, name string) string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(projectDir, ".noise", "runs", "*", name))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one %s, got %v (%v)", name, matches, err)
	}

	payload, err := os.ReadFile(filepath.Clean(matches[0]))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}

	return string(payload)
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

// readReceiverTable returns every receiver row's x, y and level.
func readReceiverTable(t *testing.T, projectDir string) ([][3]float64, error) {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(projectDir, ".noise", "runs", "*", "results", "receivers.csv"))
	if err != nil || len(matches) != 1 {
		return nil, fmt.Errorf("expected exactly one receivers.csv under %s, got %v (%w)", projectDir, matches, err)
	}

	file, err := os.Open(matches[0])
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", matches[0], err)
	}

	defer func() { _ = file.Close() }()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", matches[0], err)
	}

	out := make([][3]float64, 0, len(rows)-1)

	for _, row := range rows[1:] {
		x, _ := strconv.ParseFloat(row[1], 64)
		y, _ := strconv.ParseFloat(row[2], 64)
		level, _ := strconv.ParseFloat(row[len(row)-1], 64)

		out = append(out, [3]float64{x, y, level})
	}

	return out, nil
}

// runAutoGrid drives a run that builds its own receiver grid, which is the
// path the original defect report named.
func runAutoGrid(t *testing.T, crs string, source geo.Point2D) [][3]float64 {
	t.Helper()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")

	payload := fmt.Sprintf(`{"type":"FeatureCollection","features":[
  {"type":"Feature","properties":{"id":"src-1","kind":"source","source_type":"point"},
   "geometry":{"type":"Point","coordinates":[%s,%s]}}
]}`, strconv.FormatFloat(source.X, 'f', -1, 64), strconv.FormatFloat(source.Y, 'f', -1, 64))

	err := os.WriteFile(modelPath, []byte(payload), 0o600)
	if err != nil {
		t.Fatalf("write model: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "init", "--name", "AutoGrid", "--crs", crs)
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "dummy-freefield",
		"--param", "grid_resolution_m=10", "--param", "grid_padding_m=20")

	rows, err := readReceiverTable(t, projectDir)
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	return rows
}

// The auto receiver grid is where the defect was first reported:
// buildReceiversFromPoints subtracts grid_padding_m and steps by
// grid_resolution_m in the project's own units, so a geographic project
// subtracted 20 degrees of padding and spaced receivers 10 degrees apart —
// scattering them across three continents and reporting the levels computed
// there as if they were the site's.
//
// The custom-receiver test cannot see that come back, because it supplies the
// receivers itself. This one compares the grid a geographic project builds
// against the grid its projected twin builds: same count, same metric spacing,
// same levels.
func TestAutoGridAgreesBetweenGeographicAndProjectedProjectCRS(t *testing.T) {
	source := geo.Point2D{X: 566000, Y: 5934000}

	projected := runAutoGrid(t, "EPSG:25832", source)
	geographic := runAutoGrid(t, "EPSG:4326", projectedToGeographic(t, source))

	if len(projected) == 0 {
		t.Fatal("the projected run produced no receivers")
	}

	if len(geographic) != len(projected) {
		t.Fatalf("the geographic project built %d grid receivers and its projected twin built %d; "+
			"a padding in degrees produces a different grid, which is the original defect",
			len(geographic), len(projected))
	}

	// 20 m padding either side of a single source, stepped at 10 m: a 5x5 grid.
	const wantReceivers = 25

	if len(projected) != wantReceivers {
		t.Fatalf("grid holds %d receivers, want %d — the extent or the step is not in metres", len(projected), wantReceivers)
	}

	// The step between the first two cells, in the projected run, is the
	// grid_resolution_m the run asked for. In degrees this would be 10 units
	// of longitude.
	const wantStepM = 10.0

	gotStep := projected[1][0] - projected[0][0]
	if math.Abs(gotStep-wantStepM) > 1e-6 {
		t.Fatalf("grid step is %.6f, want %.6f m", gotStep, wantStepM)
	}

	// The two grids are compared by shape rather than by absolute position.
	// Both are built around the same source, but the geographic project's
	// source has been through one EPSG:25832 → 4326 → 25832 round trip, and
	// any offset that trip introduces is shared by every cell. It used to be
	// roughly a metre and is now zero to the printed digits (PLAN.md 1.6), but
	// the comparison stays congruence-based on purpose: congruence is the
	// property that distinguishes a grid stepped in metres from one stepped in
	// degrees, and absolute position would make this test fail for a reason
	// that belongs to the transform rather than to the grid.
	for i := range projected {
		if math.Abs(geographic[i][2]-projected[i][2]) > 0.01 {
			t.Fatalf("receiver %d: %.6f dB in EPSG:4326 against %.6f dB in EPSG:25832",
				i, geographic[i][2], projected[i][2])
		}

		offsetX := (geographic[i][0] - geographic[0][0]) - (projected[i][0] - projected[0][0])
		offsetY := (geographic[i][1] - geographic[0][1]) - (projected[i][1] - projected[0][1])

		if math.Abs(offsetX) > 0.05 || math.Abs(offsetY) > 0.05 {
			t.Fatalf("receiver %d sits %.4f, %.4f m from where the projected grid puts it relative to cell 0; "+
				"the two grids are not the same shape, which is what a padding or a step in degrees produces",
				i, offsetX, offsetY)
		}
	}

	// And the shared offset really is shared: state its size so a change in
	// the projection shows up here as a number rather than as a mystery.
	t.Logf("whole-grid offset from the 25832 → 4326 → 25832 round trip: %.4f m east, %.4f m north",
		geographic[0][0]-projected[0][0], geographic[0][1]-projected[0][1])
}
