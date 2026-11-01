package mcpserver_test

import (
	"maps"
	"path"
	"path/filepath"
	"slices"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/results"
)

// nonDeterministicTools are the read tools whose answer is a measurement:
// estimate_run times a probe of its kernel, so two calls differ by design
// (ADR-002, Deterministic outputs; its tool description says so).
var nonDeterministicTools = []string{"estimate_run"}

// seedGrid records a 4×3 two-band result raster for the run, as an auto-grid
// run persists one, so get_contours has lines to trace.
func (f fixture) seedGrid(t *testing.T, runID string) {
	t.Helper()

	bands := []string{"LrDay", "LrNight"}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width: 4, Height: 3, Bands: len(bands), NoData: -9999,
		Units: results.UniformUnits(bands, results.UnitDecibel), BandNames: bands, CRS: "EPSG:25832",
		Geo: &results.Georeference{OriginX: 500000, OriginY: 5700000, PixelSizeM: 10, RowOrder: results.RowOrderSouthUp},
	})
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	for band := range bands {
		for y := range 3 {
			for x := range 4 {
				err = raster.Set(x, y, band, 50+float64(band)*10+float64(x)*4)
				if err != nil {
					t.Fatalf("set raster cell: %v", err)
				}
			}
		}
	}

	relBase := path.Join(".noise", "runs", runID, "results", "grid")

	_, err = results.SaveRaster(filepath.Join(f.store.Root(), filepath.FromSlash(relBase)), raster)
	if err != nil {
		t.Fatalf("save raster: %v", err)
	}

	proj, err := f.store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	proj.Artifacts = append(
		proj.Artifacts,
		project.ArtifactRef{ID: runID + "-raster-meta", RunID: runID, Kind: project.ArtifactKindRunResultRasterMetadata, Path: relBase + ".json", CreatedAt: fixedNow},
		project.ArtifactRef{ID: runID + "-raster-bin", RunID: runID, Kind: project.ArtifactKindRunResultRasterBinary, Path: relBase + ".bin", CreatedAt: fixedNow},
	)

	err = f.store.Save(proj)
	if err != nil {
		t.Fatalf("save project: %v", err)
	}
}

// Two identical calls of a read tool against an unchanged project answer
// byte-identical results (ADR-002, Deterministic outputs). The project holds
// a model, a finished run with a receiver table and a result raster, so each
// tool answers with content rather than a refusal. A read tool added later
// fails the coverage check until it joins the table.
func TestReadToolsAreDeterministic(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)

	if result := f.call(t, "save_model", saveArgs(validModel, "none")); result.IsError {
		t.Fatalf("seed model: %s", resultText(t, result))
	}

	run := f.seedRunWithStatus(t, project.RunStatusCompleted)
	f.seedArtifact(t, run.ID, project.ArtifactKindRunResultReceiverTableJSON,
		".noise/runs/"+run.ID+"/results/receivers.json", receiverTable(t, 3))
	f.seedGrid(t, run.ID)

	runID := map[string]any{"run_id": run.ID}
	calls := map[string]map[string]any{
		"project_status":  {},
		"list_standards":  {},
		"get_model":       {},
		"transform":       {"source_crs": "EPSG:25832", "target_crs": "EPSG:4326", "coordinates": []float64{500000, 5700000, 500050, 5700050}},
		"list_runs":       {},
		"get_run":         runID,
		"get_run_log":     runID,
		"get_run_results": runID,
		"get_contours":    {"run_id": run.ID, "interval": 5},
	}

	readOnly := listTools(t, newFixture(t, false).session)
	reads := slices.DeleteFunc(slices.Sorted(maps.Keys(readOnly)), func(name string) bool {
		return slices.Contains(nonDeterministicTools, name)
	})

	if covered := slices.Sorted(maps.Keys(calls)); !slices.Equal(covered, reads) {
		t.Fatalf("determinism covers %v; the deterministic read tools are %v", covered, reads)
	}

	for tool, args := range calls {
		first := f.call(t, tool, args)
		if first.IsError {
			t.Fatalf("%s answered an error, which proves nothing about its content: %s", tool, resultText(t, first))
		}

		second := f.call(t, tool, args)
		if a, b := resultText(t, first), resultText(t, second); a != b {
			t.Fatalf("%s differs between two calls:\n%s\n%s", tool, a, b)
		}
	}

	// get_model in another CRS goes through the projection, and is held to
	// the same rule.
	crs := map[string]any{"crs": "EPSG:4326"}
	if a, b := resultText(t, f.call(t, "get_model", crs)), resultText(t, f.call(t, "get_model", crs)); a != b {
		t.Fatalf("get_model in EPSG:4326 differs between two calls:\n%s\n%s", a, b)
	}
}
