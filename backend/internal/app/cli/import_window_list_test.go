package cli

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
)

// Three windows of house a: a sensitive bedroom on the south facade (edge 0,
// (0,0) → (10,0)), a staircase window beside it that is context only, and a
// sensitive window on the north facade a person rejected.
const houseAWindows = `id,building,edge,u_m,sill_z_m,width_m,height_m,sensitive,review_status
a-bed,a,0,2,3.9,1.2,1.5,true,
a-stairs,a,0,6,1.2,0.8,1.0,false,confirmed
a-gone,a,2,1,0.9,1.0,1.4,true,rejected
`

func writeWindowList(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)

	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("write window list: %v", err)
	}

	return path
}

// projectWindows returns the windows of the project's model, which must
// validate clean.
func projectWindows(t *testing.T, projectDir string) []modelgeojson.Feature {
	t.Helper()

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	raw, err := store.ReadModel()
	if err != nil {
		t.Fatalf("read model: %v", err)
	}

	model, err := modelgeojson.Normalize(raw, "EPSG:25832", "model")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	var windows []modelgeojson.Feature

	for _, feature := range model.Features {
		if feature.Kind == modelgeojson.FeatureKindWindow {
			windows = append(windows, feature)
		}
	}

	report := modelgeojson.Validate(model)
	if !report.Valid || len(report.Warnings) != 0 {
		t.Fatalf("model with windows does not validate clean: %+v", report)
	}

	return windows
}

// A window list becomes window features, and a second import of the same list
// replaces them rather than adding copies. The facade mode then places the
// sensitive window's point in place of the stations on its facade, under each
// standard's rule, and the window's review status gates it like any point.
func TestImportWindowsAndRunAtThem(t *testing.T) {
	t.Parallel()

	projectDir := facadeProject(t, facadeModel(facadeRoad, facadeHouses))
	list := writeWindowList(t, projectDir, "windows.csv", houseAWindows)

	mustRunCLI(t, "--project", projectDir, "import", "--windows", list)
	mustRunCLI(t, "--project", projectDir, "import", "--windows", list)

	if windows := projectWindows(t, projectDir); len(windows) != 3 {
		t.Fatalf("model holds %d windows after two imports of three, want 3", len(windows))
	}

	// House a: the bedroom's point on edge 0, the north facade's stations
	// (its only window is rejected), the free west facade's; the party wall
	// none. 1 + 3 × 3 + 2 × 3 = 16 points, all proposed.
	args := facadeRunArgs(projectDir, "rls19-road",
		"--receiver-mode", receiverModeFacade, "--facade-building", "a", "--facade-spacing-m", "4")
	requireErrorContaining(t, runCLI(args...), "16 proposed", "--allow-unconfirmed")

	mustRunCLI(t, append(args, "--allow-unconfirmed")...)

	metadata, resultsDir := facadeRunOutputs(t, projectDir)
	if metadata["facade_window_count"] != "1" || metadata["facade_proposed_count"] != "16" {
		t.Errorf("provenance = %v, want one window point among 16 proposed", metadata)
	}

	table := readReceiverTableJSON(t, filepath.Join(resultsDir, "receivers.json"))
	if len(table.Records) != 16 {
		t.Fatalf("receiver table has %d rows, want 16", len(table.Records))
	}

	found := false

	for _, record := range table.Records {
		if record.ID != "facade/a/p0r0e0/w/a-bed" {
			continue
		}

		found = true

		// RLS-19: 5 cm out at the window centre, 0.2 m above its head.
		if math.Abs(record.X-2.6) > 1e-9 || math.Abs(record.Y+0.05) > 1e-9 || math.Abs(record.HeightM-5.6) > 1e-9 {
			t.Errorf("bedroom point = (%v, %v) at %v m", record.X, record.Y, record.HeightM)
		}

		if record.Storey == nil || *record.Storey != 1 || record.PlacementRule != geo.PlacementRLS19Facade {
			t.Errorf("bedroom row = %+v, want storey 1 under rls19-facade", record)
		}
	}

	if !found {
		t.Fatal("no row for the bedroom window")
	}

	// TA Lärm through ISO 9613-2: the bedroom window alone, 0.5 m out at its
	// centre.
	isoProject := facadeProject(t, facadeModel(buildingMaskPointSource, facadeHouses))
	mustRunCLI(t, "--project", isoProject, "import", "--windows", writeWindowList(t, isoProject, "w.csv", houseAWindows))
	mustRunCLI(t, facadeRunArgs(isoProject, "iso9613",
		"--receiver-mode", receiverModeFacade, "--facade-building", "a", "--facade-spacing-m", "4", "--allow-unconfirmed")...)

	metadata, resultsDir = facadeRunOutputs(t, isoProject)
	if metadata["facade_placement_rule"] != geo.PlacementTALaermWindow {
		t.Errorf("provenance = %v", metadata)
	}

	table = readReceiverTableJSON(t, filepath.Join(resultsDir, "receivers.json"))
	if len(table.Records) != 1 {
		t.Fatalf("iso9613 receiver table has %d rows, want the bedroom window's", len(table.Records))
	}

	if r := table.Records[0]; math.Abs(r.Y+0.5) > 1e-9 || math.Abs(r.HeightM-4.65) > 1e-9 || r.PlacementRule != geo.PlacementTALaermWindow {
		t.Errorf("iso9613 row = %+v", r)
	}
}

func TestImportWindowsRefuses(t *testing.T) {
	t.Parallel()

	projectDir := facadeProject(t, facadeModel(facadeRoad, facadeHouses))
	header := "id,building,edge,u_m,sill_z_m,width_m,height_m\n"

	for name, tc := range map[string]struct {
		file, content string
		parts         []string
	}{
		"unknown building":    {"w.csv", header + "w1,nope,0,2,1,1,1\n", []string{"row 1", "nope"}},
		"no such edge":        {"w.csv", header + "w1,a,7,2,1,1,1\n", []string{"row 1", "edge 7"}},
		"past the corner":     {"w.csv", header + "w1,a,0,9.5,1,1.2,1\n", []string{"row 1", "past the end"}},
		"another kind's id":   {"w.csv", header + "road,a,0,2,1,1,1\n", []string{"row 1", `"road"`, "source"}},
		"unknown extension":   {"w.txt", header + "w1,a,0,2,1,1,1\n", []string{".csv or .json"}},
		"unreadable row":      {"w.csv", header + "w1,a,0,two,1,1,1\n", []string{"row 1", "u_m"}},
		"slash in window id":  {"w.csv", header + "w/1,a,0,2,1,1,1\n", []string{"row 1", "'/'"}},
		"bad review status":   {"w.csv", header[:len(header)-1] + ",review_status\nw1,a,0,2,1,1,1,maybe\n", []string{"row 1", "maybe"}},
		"zero window height":  {"w.csv", header + "w1,a,0,2,1,1,0\n", []string{"row 1", "height_m"}},
		"window list missing": {"missing.csv", "", []string{"window list"}},
	} {
		path := filepath.Join(t.TempDir(), tc.file)
		if tc.content != "" {
			path = writeWindowList(t, t.TempDir(), tc.file, tc.content)
		}

		err := runCLI("--project", projectDir, "import", "--windows", path)
		if err == nil {
			t.Errorf("%s: import succeeded", name)

			continue
		}

		for _, part := range tc.parts {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("%s: %q does not mention %q", name, err, part)
			}
		}
	}

	if windows := projectWindows(t, projectDir); len(windows) != 0 {
		t.Errorf("refused imports left %d windows behind", len(windows))
	}
}
