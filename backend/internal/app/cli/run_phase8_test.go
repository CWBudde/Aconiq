package cli

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/soundplan/soundplan/backend/internal/domain/project"
	"github.com/soundplan/soundplan/backend/internal/io/projectfs"
	"github.com/soundplan/soundplan/backend/internal/qa/golden"
	"github.com/soundplan/soundplan/backend/internal/report/results"
)

func TestRunDummyFreefieldPhase8Golden(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase8", "model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase8", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(
		t,
		"--project", projectDir,
		"run",
		"--standard", "dummy-freefield",
		"--param", "grid_resolution_m=10",
		"--param", "grid_padding_m=0",
		"--param", "source_emission_db=90",
		"--param", "receiver_height_m=4",
		"--param", "chunk_size=3",
		"--param", "workers=2",
	)

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	if len(proj.Runs) == 0 {
		t.Fatal("expected at least one run")
	}

	run := proj.Runs[len(proj.Runs)-1]
	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected run status %q, got %q", project.RunStatusCompleted, run.Status)
	}

	receiverTablePath := filepath.Join(projectDir, ".noise", "runs", run.ID, "results", "receivers.json")
	receiverTablePayload, err := os.ReadFile(receiverTablePath)
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var receiverTable results.ReceiverTable
	if err := json.Unmarshal(receiverTablePayload, &receiverTable); err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	rasterPath := filepath.Join(projectDir, ".noise", "runs", run.ID, "results", "ldummy.json")
	raster, err := results.LoadRaster(rasterPath)
	if err != nil {
		t.Fatalf("load raster: %v", err)
	}

	meta := raster.Metadata()
	recordsSnapshot := make([]map[string]any, 0, len(receiverTable.Records))
	for _, record := range receiverTable.Records {
		recordsSnapshot = append(recordsSnapshot, map[string]any{
			"id":       record.ID,
			"x":        round6(record.X),
			"y":        round6(record.Y),
			"height_m": round6(record.HeightM),
			"ldummy":   round6(record.Values[dummyIndicatorName]),
		})
	}

	rasterValues := raster.Values()
	rasterValuesRounded := make([]float64, 0, len(rasterValues))
	for _, value := range rasterValues {
		rasterValuesRounded = append(rasterValuesRounded, round6(value))
	}

	snapshot := map[string]any{
		"standard": run.Standard,
		"raster": map[string]any{
			"width":      meta.Width,
			"height":     meta.Height,
			"bands":      meta.Bands,
			"band_names": meta.BandNames,
			"unit":       meta.Unit,
			"values":     rasterValuesRounded,
		},
		"receivers": recordsSnapshot,
	}

	golden.AssertJSONSnapshot(t, testdataPath(t, "phase8-dummy-freefield.golden.json"), snapshot)
}

func mustRunCLI(t *testing.T, args ...string) {
	t.Helper()

	cmd := newRootCommand()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("noise %v: %v", args, err)
	}
}

func round6(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}

func testdataPath(t *testing.T, parts ...string) string {
	t.Helper()

	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}

	base := filepath.Join(filepath.Dir(filePath), "testdata")
	all := append([]string{base}, parts...)
	return filepath.Join(all...)
}
