package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/results"
)

// Two terraced houses sharing the wall x = 10, north of a road at y = -20.
// House a has building:levels and no height_source, which is what a project
// imported before the importer wrote `levels` carries; house b's height was
// assumed.
const facadeHouses = `
    {"type":"Feature","properties":{"id":"a","kind":"building","height_m":9,"building:levels":"3"},
     "geometry":{"type":"Polygon","coordinates":[[[0,0],[10,0],[10,8],[0,8],[0,0]]]}},
    {"type":"Feature","properties":{"id":"b","kind":"building","height_m":9,"height_source":"assumed"},
     "geometry":{"type":"Polygon","coordinates":[[[10,0],[20,0],[20,8],[10,8],[10,0]]]}}`

const facadeRoad = `
    {"type":"Feature","properties":{"id":"road","kind":"source","source_type":"line"},
     "geometry":{"type":"LineString","coordinates":[[-100,-20],[200,-20]]}}`

const facadePointSource = `
    {"type":"Feature","properties":{"id":"src","kind":"source","source_type":"point"},
     "geometry":{"type":"Point","coordinates":[5,-20]}}`

// facadeReceiver is a receiver on house a's south facade (canonical edge 0,
// (0,0) → (10,0)), 5 cm out, with the facade properties given.
func facadeReceiver(id string, properties string) string {
	return `
    {"type":"Feature","properties":{"id":"` + id + `","kind":"receiver","height_m":3` + properties + `},
     "geometry":{"type":"Point","coordinates":[5,-0.05]}}`
}

const facadeKeyOfHouseAEdge0 = `"facade":{"building_ref":"a","part":0,"ring":0,"edge":0,"from":[0,0],"to":[10,0]}`

func facadeProperties(reviewStatus string) string {
	return `,"placement_rule":"rls19-facade",` + facadeKeyOfHouseAEdge0 +
		`,"storey":0,"slab_height_source":"levels","review_status":"` + reviewStatus + `"`
}

func facadeModel(features ...string) string {
	return `{"type":"FeatureCollection","features":[` + strings.Join(features, ",") + `]}`
}

// facadeProject initialises a project holding model and returns its directory.
func facadeProject(t *testing.T, model string) string {
	t.Helper()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")

	err := os.WriteFile(modelPath, []byte(model), 0o600)
	if err != nil {
		t.Fatalf("write model: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Facade", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	return projectDir
}

func facadeRunArgs(projectDir string, standard string, extra ...string) []string {
	return append([]string{"--project", projectDir, "run", "--standard", standard}, extra...)
}

// facadeRunOutputs returns the project's single run's provenance metadata and its
// results directory.
func facadeRunOutputs(t *testing.T, projectDir string) (map[string]string, string) {
	t.Helper()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) != 1 {
		t.Fatalf("project has %d runs, want 1", len(proj.Runs))
	}

	runDir := filepath.Join(projectDir, ".noise", "runs", proj.Runs[0].ID)

	payload, err := os.ReadFile(filepath.Join(runDir, "provenance.json"))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var provenance struct {
		Metadata map[string]string `json:"metadata"`
	}

	err = json.Unmarshal(payload, &provenance)
	if err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	return provenance.Metadata, filepath.Join(runDir, "results")
}

func projectRunCount(t *testing.T, projectDir string) int {
	t.Helper()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	return len(proj.Runs)
}

func requireErrorContaining(t *testing.T, err error, parts ...string) {
	t.Helper()

	if err == nil {
		t.Fatalf("run succeeded, want a refusal containing %q", parts)
	}

	for _, part := range parts {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("refusal %q does not contain %q", err, part)
		}
	}
}

// The facade mode generates its receivers from the two houses. Every point is
// proposed, so a normative run is refused without the opt-in — before a run
// exists — and with it the run names each row's building, facade and storey,
// leaves the party wall bare, and records the counts.
func TestFacadeModeRunsOverGeneratedReceiversOnlyWithTheOptIn(t *testing.T) {
	t.Parallel()

	projectDir := facadeProject(t, facadeModel(facadeRoad, facadeHouses))
	args := facadeRunArgs(projectDir, "rls19-road",
		"--receiver-mode", receiverModeFacade,
		"--facade-building", "a", "--facade-building", "b", "--facade-spacing-m", "4")

	// Per house: two 10 m facades at 3 stations and one free 8 m facade at 2;
	// the shared wall carries none. Three storeys each: 2 × 8 × 3 = 48 points,
	// all proposed, and house b's 24 have an assumed slab height.
	requireErrorContaining(t, runCLI(args...), "48 proposed", "24 with an assumed slab height", "--allow-unconfirmed")

	if n := projectRunCount(t, projectDir); n != 0 {
		t.Fatalf("a refused run left %d runs behind", n)
	}

	mustRunCLI(t, append(args, "--allow-unconfirmed")...)

	metadata, resultsDir := facadeRunOutputs(t, projectDir)
	for key, want := range map[string]string{
		"receiver_mode":               receiverModeFacade,
		"facade_placement_rule":       geo.PlacementRLS19Facade,
		"facade_buildings":            `["a","b"]`,
		"facade_spacing_m":            "4",
		"facade_include_courtyard":    "false",
		"facade_proposed_count":       "48",
		"facade_assumed_height_count": "24",
		"allow_unconfirmed":           "true",
	} {
		if metadata[key] != want {
			t.Errorf("provenance %s = %q, want %q", key, metadata[key], want)
		}
	}

	summary := readRunSummary(t, filepath.Join(resultsDir, "run-summary.json"))
	if summary["facade_proposed_count"] != float64(48) || summary["facade_assumed_height_count"] != float64(24) ||
		summary["receiver_mode"] != receiverModeFacade {
		t.Errorf("run summary = %v", summary)
	}

	table := readReceiverTableJSON(t, filepath.Join(resultsDir, "receivers.json"))
	if len(table.Records) != 48 {
		t.Fatalf("receiver table has %d rows, want 48", len(table.Records))
	}

	for _, record := range table.Records {
		if record.Facade == nil || record.Storey == nil || record.PlacementRule != geo.PlacementRLS19Facade {
			t.Fatalf("row %s names no facade: %+v", record.ID, record)
		}

		if record.X > 9.9 && record.X < 10.1 && record.Y > 0 && record.Y < 8 {
			t.Fatalf("row %s stands on the party wall at (%v, %v)", record.ID, record.X, record.Y)
		}

		if !strings.HasPrefix(record.ID, "facade/"+record.Facade.BuildingRef+"/") {
			t.Fatalf("row id %q does not name its building %q", record.ID, record.Facade.BuildingRef)
		}
	}

	csvPayload, err := os.ReadFile(filepath.Join(resultsDir, "receivers.csv"))
	if err != nil {
		t.Fatalf("read receiver csv: %v", err)
	}

	header, _, _ := strings.Cut(string(csvPayload), "\n")
	if !strings.HasSuffix(header, ",building_ref,part,ring,edge,storey,placement_rule") {
		t.Fatalf("csv header %q carries no facade columns", header)
	}
}

// The facade mode belongs to the standards a geometric placement rule exists
// for, to buildings the rule can place on, and to a complete request.
func TestFacadeModeRefusals(t *testing.T) {
	t.Parallel()

	projectDir := facadeProject(t, facadeModel(facadeRoad, facadeHouses))

	for _, tc := range []struct {
		name  string
		args  []string
		parts []string
	}{
		{
			"standard without a rule", facadeRunArgs(projectDir, "cnossos-road",
				"--receiver-mode", receiverModeFacade, "--facade-building", "a", "--facade-spacing-m", "4", "--experimental"),
			[]string{"cnossos-road", "iso9613", "rls19-road", "schall03"},
		},
		{
			// TA Lärm's rule places at windows only, and house a has none.
			"window rule without windows", facadeRunArgs(projectDir, "iso9613",
				"--receiver-mode", receiverModeFacade, "--facade-building", "a", "--facade-spacing-m", "4"),
			[]string{"talaerm-window", "aconiq import --windows"},
		},
		{
			"no spacing", facadeRunArgs(projectDir, "rls19-road",
				"--receiver-mode", receiverModeFacade, "--facade-building", "a"),
			[]string{"--facade-spacing-m"},
		},
		{
			"no building", facadeRunArgs(projectDir, "rls19-road",
				"--receiver-mode", receiverModeFacade, "--facade-spacing-m", "4"),
			[]string{"--facade-building"},
		},
		{
			"unknown building", facadeRunArgs(projectDir, "rls19-road",
				"--receiver-mode", receiverModeFacade, "--facade-building", "nope", "--facade-spacing-m", "4", "--allow-unconfirmed"),
			[]string{"nope"},
		},
		{
			"facade flags outside facade mode", facadeRunArgs(projectDir, "rls19-road",
				"--receiver-mode", receiverModeAutoGrid, "--facade-building", "a"),
			[]string{"--receiver-mode facade"},
		},
	} {
		requireErrorContaining(t, runCLI(tc.args...), tc.parts...)
	}

	if n := projectRunCount(t, projectDir); n != 0 {
		t.Fatalf("refused runs left %d runs behind", n)
	}
}

// Custom receivers carrying facade properties are held to the same rule: a
// normative standard refuses proposed points without the opt-in. Their rows
// name the facade.
func TestCustomFacadeReceiversFollowTheGate(t *testing.T) {
	t.Parallel()

	projectDir := facadeProject(t, facadeModel(facadeRoad, facadeHouses,
		facadeReceiver("r-proposed", facadeProperties("proposed"))))

	requireErrorContaining(t,
		runCLI(facadeRunArgs(projectDir, "rls19-road", "--receiver-mode", receiverModeCustom)...),
		"1 proposed", "--allow-unconfirmed")

	mustRunCLI(t, facadeRunArgs(projectDir, "rls19-road", "--receiver-mode", receiverModeCustom, "--allow-unconfirmed")...)

	metadata, resultsDir := facadeRunOutputs(t, projectDir)
	if metadata["facade_proposed_count"] != "1" || metadata["facade_assumed_height_count"] != "" {
		t.Fatalf("provenance metadata = %v", metadata)
	}

	table := readReceiverTableJSON(t, filepath.Join(resultsDir, "receivers.json"))
	if len(table.Records) != 1 || table.Records[0].Facade == nil || table.Records[0].Facade.BuildingRef != "a" ||
		*table.Records[0].Storey != 0 {
		t.Fatalf("receiver table = %+v", table.Records)
	}
}

// A test fixture assesses nothing, so it runs over proposed points as they are.
func TestCustomFacadeReceiversRunUnderATestFixtureWithoutTheOptIn(t *testing.T) {
	t.Parallel()

	projectDir := facadeProject(t, facadeModel(facadePointSource, facadeHouses,
		facadeReceiver("r-proposed", facadeProperties("proposed"))))

	mustRunCLI(t, facadeRunArgs(projectDir, "dummy-freefield", "--receiver-mode", receiverModeCustom)...)

	metadata, resultsDir := facadeRunOutputs(t, projectDir)
	if metadata["facade_proposed_count"] != "1" {
		t.Fatalf("provenance metadata = %v", metadata)
	}

	table := readReceiverTableJSON(t, filepath.Join(resultsDir, "receivers.json"))
	if len(table.Records) != 1 || table.Records[0].Facade == nil {
		t.Fatalf("receiver table = %+v", table.Records)
	}
}

// A rejected point never enters a run, with or without the opt-in, and a
// confirmed one needs none.
func TestRejectedFacadeReceiversNeverEnterARun(t *testing.T) {
	t.Parallel()

	projectDir := facadeProject(t, facadeModel(facadeRoad, facadeHouses,
		facadeReceiver("r-confirmed", facadeProperties("confirmed")),
		facadeReceiver("r-rejected", facadeProperties("rejected"))))

	mustRunCLI(t, facadeRunArgs(projectDir, "rls19-road", "--receiver-mode", receiverModeCustom)...)

	metadata, resultsDir := facadeRunOutputs(t, projectDir)
	if metadata["facade_proposed_count"] != "" || metadata["facade_rejected_count"] != "1" {
		t.Fatalf("provenance metadata = %v", metadata)
	}

	table := readReceiverTableJSON(t, filepath.Join(resultsDir, "receivers.json"))
	if len(table.Records) != 1 || table.Records[0].ID != "r-confirmed" {
		t.Fatalf("receiver table = %+v", table.Records)
	}
}

// A facade link that no longer holds blocks a normative run, and the opt-in
// does not lift it: re-attaching is a reviewed step. A point placed by another
// standard's rule is not an Immissionsort of this one.
func TestBrokenFacadeLinksBlockANormativeRun(t *testing.T) {
	t.Parallel()

	confirmed := facadeProperties("confirmed")

	for _, tc := range []struct {
		name       string
		properties string
		parts      []string
	}{
		{
			"unresolved", strings.Replace(confirmed, `"building_ref":"a"`, `"building_ref":"gone"`, 1),
			[]string{"r1", modelgeojson.CodeReceiverFacadeUnresolved},
		},
		{
			"moved", strings.Replace(confirmed, `"from":[0,0]`, `"from":[0,0.5]`, 1),
			[]string{"r1", modelgeojson.CodeReceiverFacadeMoved},
		},
		{
			"foreign rule", strings.Replace(confirmed, `"rls19-facade"`, `"talaerm-window"`, 1),
			[]string{"r1", "talaerm-window", "rls19-road"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			projectDir := facadeProject(t, facadeModel(facadeRoad, facadeHouses, facadeReceiver("r1", tc.properties)))

			requireErrorContaining(t,
				runCLI(facadeRunArgs(projectDir, "rls19-road", "--receiver-mode", receiverModeCustom, "--allow-unconfirmed")...),
				tc.parts...)

			// A test fixture assesses nothing, so it runs.
			fixtureDir := facadeProject(t, facadeModel(facadePointSource, facadeHouses, facadeReceiver("r1", tc.properties)))
			mustRunCLI(t, facadeRunArgs(fixtureDir, "dummy-freefield", "--receiver-mode", receiverModeCustom)...)
		})
	}
}

// The facade attributes reach a receiver table row only as a complete set,
// which is the only shape results.ReceiverTable accepts.
func TestFacadeAttributesReachTheTableOnlyComplete(t *testing.T) {
	t.Parallel()

	storey := 1
	set := receiverSet{facade: map[string]facadeset.Attributes{
		"full":    {PlacementRule: geo.PlacementRLS19Facade, Facade: &geo.FacadeKey{BuildingRef: "a"}, Storey: &storey},
		"no-rule": {Facade: &geo.FacadeKey{BuildingRef: "a"}, Storey: &storey},
	}}

	full := set.tableRecord(results.ReceiverRecord{ID: "full"})
	if full.Facade == nil || full.Facade.BuildingRef != "a" || *full.Storey != 1 || full.PlacementRule != geo.PlacementRLS19Facade {
		t.Fatalf("full = %+v", full)
	}

	partial := set.tableRecord(results.ReceiverRecord{ID: "no-rule"})
	if partial.Facade != nil || partial.Storey != nil || partial.PlacementRule != "" {
		t.Fatalf("partial = %+v", partial)
	}
}

// mustCustomReceiverSet resolves model's custom receiver set as a run would.
func mustCustomReceiverSet(t *testing.T, model modelgeojson.Model) receiverSet {
	t.Helper()

	set, err := customReceiverSet(receiverSet{mode: receiverModeCustom, metadata: map[string]string{}}, model)
	if err != nil {
		t.Fatalf("resolve custom receivers: %v", err)
	}

	return set
}
