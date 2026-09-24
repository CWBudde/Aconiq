package cli

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/report/results"
)

// The grid is 11x11 at 10 m over the drawn area [0,100]², and the building is
// a block [20,80]² around a courtyard [40,60]². Both are aligned with the
// grid, so the edges carry receivers:
//
//   - strictly inside the block and outside the courtyard: x,y ∈ {30..70},
//     minus the closed courtyard {40,50,60}² — 25 − 9 = 16 cells masked;
//   - on the facade (x or y = 20 or 80) and on the courtyard edge: not masked;
//   - in the courtyard: not masked, it is open air.
const (
	buildingMaskGridSide    = 11
	buildingMaskMaskedCells = 16
)

func buildingMaskModel(sources string, withBuilding bool) string {
	building := ""
	if withBuilding {
		building = `,
    {
      "type": "Feature",
      "properties": {"id": "block", "kind": "building", "height_m": 12},
      "geometry": {"type": "Polygon", "coordinates": [
        [[20,20],[80,20],[80,80],[20,80],[20,20]],
        [[40,40],[60,40],[60,60],[40,60],[40,40]]
      ]}
    }`
	}

	return `{
  "type": "FeatureCollection",
  "features": [` + sources + `,
    {
      "type": "Feature",
      "properties": {"id": "area", "kind": "calc-area"},
      "geometry": {"type": "Polygon", "coordinates": [[[0,0],[100,0],[100,100],[0,100],[0,0]]]}
    }` + building + `
  ]
}`
}

const buildingMaskRoadSource = `
    {
      "type": "Feature",
      "properties": {"id": "road", "kind": "source", "source_type": "line"},
      "geometry": {"type": "LineString", "coordinates": [[-100,-40],[200,-40]]}
    }`

const buildingMaskPointSource = `
    {
      "type": "Feature",
      "properties": {"id": "stack", "kind": "source", "source_type": "point",
        "iso9613_source_height_m": 10, "iso9613_sound_power_level_db": 100},
      "geometry": {"type": "Point", "coordinates": [50, -40]}
    }`

// buildingMaskRun runs one auto-grid run over the drawn area and returns its
// run directory.
func buildingMaskRun(t *testing.T, model string, standard string) string {
	t.Helper()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")

	err := os.WriteFile(modelPath, []byte(model), 0o600)
	if err != nil {
		t.Fatalf("write model: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "init", "--name", "BuildingMask", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run",
		"--standard", standard,
		"--param", "grid_resolution_m=10", "--param", "grid_padding_m=0")

	return latestRunDir(t, projectDir)
}

func TestGridReceiversInsideABuildingAreNoDataInTheRaster(t *testing.T) {
	t.Parallel()

	cases := []struct {
		standard string
		sources  string
		raster   string
	}{
		{"rls19-road", buildingMaskRoadSource, "rls19-road.json"},
		{"iso9613", buildingMaskPointSource, "iso9613.json"},
	}

	for _, tc := range cases {
		t.Run(tc.standard, func(t *testing.T) {
			t.Parallel()

			runDir := buildingMaskRun(t, buildingMaskModel(tc.sources, true), tc.standard)
			resultsDir := filepath.Join(runDir, "results")

			table := readReceiverTableJSON(t, filepath.Join(resultsDir, "receivers.json"))
			if len(table.Records) != buildingMaskGridSide*buildingMaskGridSide {
				t.Fatalf("receiver table has %d rows, want the whole %dx%d grid", len(table.Records), buildingMaskGridSide, buildingMaskGridSide)
			}

			raster, err := results.LoadRaster(filepath.Join(resultsDir, tc.raster))
			if err != nil {
				t.Fatalf("load raster: %v", err)
			}

			meta := raster.Metadata()
			if meta.Width != buildingMaskGridSide || meta.Height != buildingMaskGridSide {
				t.Fatalf("raster is %dx%d, want %dx%d", meta.Width, meta.Height, buildingMaskGridSide, buildingMaskGridSide)
			}

			masked := 0

			for i, record := range table.Records {
				x, y := i%meta.Width, i/meta.Width
				wantMasked := strictlyInsideTheBlock(record.X, record.Y)

				if wantMasked {
					masked++
				}

				for band, name := range meta.BandNames {
					tableValue := record.Values[name]
					if math.IsNaN(tableValue) || math.IsInf(tableValue, 0) {
						t.Fatalf("receiver %s %s = %v: a masked receiver is still computed", record.ID, name, tableValue)
					}

					cell, err := raster.At(x, y, band)
					if err != nil {
						t.Fatalf("raster at (%d,%d,%d): %v", x, y, band, err)
					}

					switch {
					case wantMasked && cell != meta.NoData:
						t.Errorf("receiver %s at (%v,%v) stands inside the block, raster %s = %v, want nodata", record.ID, record.X, record.Y, name, cell)
					case !wantMasked && cell != tableValue:
						t.Errorf("receiver %s at (%v,%v): raster %s = %v, table says %v", record.ID, record.X, record.Y, name, cell, tableValue)
					}
				}
			}

			if masked != buildingMaskMaskedCells {
				t.Fatalf("the geometry puts %d receivers inside the block, want %d — the fixture no longer tests what it says", masked, buildingMaskMaskedCells)
			}

			summary := readRunSummary(t, filepath.Join(resultsDir, "run-summary.json"))
			if got := summary[gridMaskedCellsKey]; got != float64(buildingMaskMaskedCells) {
				t.Errorf("run-summary %s = %v, want %d", gridMaskedCellsKey, got, buildingMaskMaskedCells)
			}

			logPayload, err := os.ReadFile(filepath.Join(runDir, "run.log"))
			if err != nil {
				t.Fatalf("read run.log: %v", err)
			}

			if !strings.Contains(string(logPayload), fmt.Sprintf("grid_masked_cells=%d ", buildingMaskMaskedCells)) {
				t.Errorf("run.log does not record the masked cells:\n%s", logPayload)
			}
		})
	}
}

// ISO 9613-2 does not model buildings in this pipeline, so the building's
// only effect on its run is the mask — which must reach the raster and
// nothing else. That is the property the design rests on: the receiver table,
// and so output_hash, are those of the unmasked run.
func TestMaskingABuildingLeavesTheReceiverTableAndOutputHashAlone(t *testing.T) {
	t.Parallel()

	withBuilding := filepath.Join(buildingMaskRun(t, buildingMaskModel(buildingMaskPointSource, true), "iso9613"), "results")
	without := filepath.Join(buildingMaskRun(t, buildingMaskModel(buildingMaskPointSource, false), "iso9613"), "results")

	for _, name := range []string{"receivers.csv", "receivers.json"} {
		a, err := os.ReadFile(filepath.Join(withBuilding, name))
		if err != nil {
			t.Fatal(err)
		}

		b, err := os.ReadFile(filepath.Join(without, name))
		if err != nil {
			t.Fatal(err)
		}

		if string(a) != string(b) {
			t.Errorf("%s differs between the masked and the unmasked run", name)
		}
	}

	masked := readRunSummary(t, filepath.Join(withBuilding, "run-summary.json"))
	unmasked := readRunSummary(t, filepath.Join(without, "run-summary.json"))

	if masked["output_hash"] != unmasked["output_hash"] {
		t.Errorf("output_hash moved: %v masked, %v unmasked", masked["output_hash"], unmasked["output_hash"])
	}

	if _, ok := unmasked[gridMaskedCellsKey]; ok {
		t.Errorf("a run with nothing masked must not carry %s", gridMaskedCellsKey)
	}
}

func strictlyInsideTheBlock(x, y float64) bool {
	inBlock := x > 20 && x < 80 && y > 20 && y < 80
	inCourtyard := x >= 40 && x <= 60 && y >= 40 && y <= 60

	return inBlock && !inCourtyard
}
