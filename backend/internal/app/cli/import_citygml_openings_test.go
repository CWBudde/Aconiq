package cli

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/citygmlimport"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// A CityGML LoD3 import writes the house's windows as window features beside
// it (P18 C1b): the two in its walls placed on their facades, proposed and
// not sensitive, each citing its gml:id and the file's hash; the skylight,
// the window in no facade plane and the door reported, not dropped. Once a
// person marks the south window sensitive and confirms it, the facade mode
// places at it what it places at the same window from a window list.
func TestImportCityGMLLoD3Windows(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	mustRunCLI(t, "--project", projectDir, "init", "--name", "LoD3", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, "p18c1b", "lod3-house.gml"))

	report := readCityGMLReport(t, projectDir)
	if o := report.Openings; o == nil || o.Windows != 4 || o.Imported != 2 || o.Doors != 1 || len(o.Skipped) != 2 {
		t.Fatalf("openings report = %+v, want 4 windows, 2 imported, 1 door, 2 skipped", report.Openings)
	}

	skipped := map[string]string{}
	for _, s := range report.Openings.Skipped {
		skipped[s.ID] = s.Reason
	}

	if !strings.Contains(skipped["skylight"], "RoofSurface") || !strings.Contains(skipped["w-adrift"], "no facade plane") {
		t.Errorf("skipped = %v, want the skylight and the window in no facade plane", skipped)
	}

	windows := map[string]modelgeojson.Window{}

	for _, feature := range projectWindows(t, projectDir) {
		window, err := modelgeojson.WindowOf(feature)
		if err != nil {
			t.Fatalf("WindowOf: %v", err)
		}

		windows[window.ID] = window

		evidence, _ := feature.Properties[modelgeojson.PropertyEvidence].([]any)
		entry, _ := evidence[0].(map[string]any)

		sha, _ := entry["content_sha256"].(string)
		if len(evidence) != 1 || entry["kind"] != "citygml-lod3" || entry["source_id"] != window.ID || len(sha) != 64 {
			t.Errorf("%s evidence = %#v, want its gml:id and the file's sha256", window.ID, evidence)
		}
	}

	south, east := windows["w-south"], windows["w-east"]
	if len(windows) != 2 || south.Facade.BuildingRef != "lod3-house" || south.Sensitive || south.ReviewStatus != geo.ReviewStatusProposed {
		t.Fatalf("windows = %+v, want w-south and w-east, proposed and not sensitive", windows)
	}

	want := geo.FacadeWindow{UM: 2, SillZM: 3.9, WidthM: 1.2, HeightM: 1.5}

	corner := geo.Point2D{X: 500000, Y: 5800000}
	if !closeFrame(south.Frame, want) || south.Facade.From != corner || south.Facade.To != (geo.Point2D{X: corner.X + 10, Y: corner.Y}) {
		t.Errorf("w-south = %+v, want %+v on the south edge, east from %v", south, want, corner)
	}

	if math.Abs(east.Frame.SillZM-1) > 1e-9 || math.Abs(east.Frame.WidthM-1) > 1e-9 || math.Abs(east.Frame.HeightM-1.2) > 1e-9 {
		t.Errorf("w-east = %+v, want a 1 m × 1.2 m window with its sill at 1 m", east)
	}

	confirmSouthWindowAndAddASource(t, projectDir)

	mustRunCLI(t, facadeRunArgs(projectDir, "iso9613",
		"--receiver-mode", receiverModeFacade, "--facade-building", "Musterstraße 1", "--facade-spacing-m", "4")...)

	_, resultsDir := facadeRunOutputs(t, projectDir)

	table := readReceiverTableJSON(t, filepath.Join(resultsDir, "receivers.json"))
	if len(table.Records) != 1 {
		t.Fatalf("receiver table has %d rows, want the south window's alone", len(table.Records))
	}

	// TA Lärm: 0.5 m out at the window centre, as for a-bed in
	// TestImportWindowsAndRunAtThem, which is the same window from a list.
	if r := table.Records[0]; r.ID != "facade/lod3-house/p0r0e0/w/w-south" ||
		math.Abs(r.X-500002.6) > 1e-6 || math.Abs(r.Y-5799999.5) > 1e-6 || math.Abs(r.HeightM-4.65) > 1e-9 {
		t.Errorf("row = %+v, want the south window's point", r)
	}
}

func readCityGMLReport(t *testing.T, projectDir string) citygmlimport.ImportReport {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(projectDir, ".noise", "model", "citygml-import-report.json"))
	if err != nil {
		t.Fatalf("read CityGML import report: %v", err)
	}

	var report citygmlimport.ImportReport

	err = json.Unmarshal(raw, &report)
	if err != nil {
		t.Fatalf("decode CityGML import report: %v", err)
	}

	return report
}

// confirmSouthWindowAndAddASource does what a person does in review: marks
// w-south sensitive and confirms it. It also adds the point source south of
// the house the TA Lärm run needs, which a CityGML file does not carry.
func confirmSouthWindowAndAddASource(t *testing.T, projectDir string) {
	t.Helper()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	raw, err := store.ReadModel()
	if err != nil {
		t.Fatalf("read model: %v", err)
	}

	model, err := modelgeojson.Normalize(raw, proj.CRS, "model")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	for i, feature := range model.Features {
		if feature.ID == "w-south" {
			model.Features[i].Properties[modelgeojson.PropertySensitive] = true
			model.Features[i].Properties[modelgeojson.PropertyReviewStatus] = geo.ReviewStatusConfirmed
		}
	}

	model.Features = append(model.Features, modelgeojson.Feature{
		ID: "stack", Kind: modelgeojson.FeatureKindSource, SourceType: "point",
		GeometryType: modelgeojson.GeometryTypePoint, Coordinates: []any{500050.0, 5799960.0},
		Properties: map[string]any{"iso9613_source_height_m": 10.0, "iso9613_sound_power_level_db": 100.0},
	})

	err = store.SaveModel(&proj, model, modelgeojson.Validate(model))
	if err != nil {
		t.Fatalf("save model: %v", err)
	}
}

func closeFrame(a, b geo.FacadeWindow) bool {
	return math.Abs(a.UM-b.UM) < 1e-9 && math.Abs(a.SillZM-b.SillZM) < 1e-9 &&
		math.Abs(a.WidthM-b.WidthM) < 1e-9 && math.Abs(a.HeightM-b.HeightM) < 1e-9
}
