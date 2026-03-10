package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/aconiq/backend/internal/qa/golden"
	"github.com/aconiq/backend/internal/report/results"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
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

	err = json.Unmarshal(receiverTablePayload, &receiverTable)
	if err != nil {
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
			"ldummy":   round6(record.Values[freefield.IndicatorLdummy]),
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

func TestRunRejectsUnknownRunParameter(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase8", "model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase8", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	err := runCLI("--project", projectDir, "run", "--standard", "dummy-freefield", "--param", "not_allowed=1")
	if err == nil {
		t.Fatal("expected run command error")
	}

	if !strings.Contains(err.Error(), `unknown run parameter "not_allowed"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCnossosRoadProducesOutputs(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase10", "road_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase10", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "cnossos-road")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) == 0 {
		t.Fatal("expected one run")
	}

	run := proj.Runs[len(proj.Runs)-1]
	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected completed run status, got %q", run.Status)
	}

	resultsDir := filepath.Join(projectDir, ".noise", "runs", run.ID, "results")
	for _, path := range []string{
		filepath.Join(resultsDir, "receivers.json"),
		filepath.Join(resultsDir, "receivers.csv"),
		filepath.Join(resultsDir, "cnossos-road.json"),
		filepath.Join(resultsDir, "cnossos-road.bin"),
		filepath.Join(resultsDir, "run-summary.json"),
	} {
		_, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected result file %s: %v", path, err)
		}
	}

	payload, err := os.ReadFile(filepath.Join(resultsDir, "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable

	err = json.Unmarshal(payload, &table)
	if err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	if len(table.IndicatorOrder) == 0 {
		t.Fatal("expected indicator order in receiver table")
	}

	expectedIndicators := map[string]bool{
		cnossosroad.IndicatorLden:   false,
		cnossosroad.IndicatorLnight: false,
	}
	for _, indicator := range table.IndicatorOrder {
		if _, ok := expectedIndicators[indicator]; ok {
			expectedIndicators[indicator] = true
		}
	}

	for indicator, found := range expectedIndicators {
		if !found {
			t.Fatalf("expected indicator %s in receiver table order", indicator)
		}
	}

	provenancePayload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var provenance struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(provenancePayload, &provenance); err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if provenance.Metadata["model_version"] != cnossosroad.BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", provenance.Metadata)
	}

	if provenance.Metadata["compliance_boundary"] != "baseline-preview-expanded-road-contract" {
		t.Fatalf("unexpected compliance boundary: %#v", provenance.Metadata)
	}
}

func TestRunCnossosRailProducesOutputs(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase11", "rail_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase11", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "cnossos-rail")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) == 0 {
		t.Fatal("expected one run")
	}

	run := proj.Runs[len(proj.Runs)-1]
	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected completed run status, got %q", run.Status)
	}

	resultsDir := filepath.Join(projectDir, ".noise", "runs", run.ID, "results")
	for _, path := range []string{
		filepath.Join(resultsDir, "receivers.json"),
		filepath.Join(resultsDir, "receivers.csv"),
		filepath.Join(resultsDir, "cnossos-rail.json"),
		filepath.Join(resultsDir, "cnossos-rail.bin"),
		filepath.Join(resultsDir, "run-summary.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected result file %s: %v", path, err)
		}
	}

	payload, err := os.ReadFile(filepath.Join(resultsDir, "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable
	if err := json.Unmarshal(payload, &table); err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	expectedIndicators := map[string]bool{
		cnossosrail.IndicatorLden:   false,
		cnossosrail.IndicatorLnight: false,
	}
	for _, indicator := range table.IndicatorOrder {
		if _, ok := expectedIndicators[indicator]; ok {
			expectedIndicators[indicator] = true
		}
	}

	for indicator, found := range expectedIndicators {
		if !found {
			t.Fatalf("expected indicator %s in receiver table order", indicator)
		}
	}

	provenancePayload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var provenance struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(provenancePayload, &provenance); err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if provenance.Metadata["model_version"] != cnossosrail.BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", provenance.Metadata)
	}

	if provenance.Metadata["compliance_boundary"] != "baseline-preview-expanded-rail-contract" {
		t.Fatalf("unexpected compliance boundary: %#v", provenance.Metadata)
	}
}

func TestRunBUBRoadProducesOutputs(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase14", "bub_road_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase14", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "bub-road")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) == 0 {
		t.Fatal("expected one run")
	}

	run := proj.Runs[len(proj.Runs)-1]
	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected completed run status, got %q", run.Status)
	}

	if run.Standard.Context != "mapping" {
		t.Fatalf("expected mapping context, got %q", run.Standard.Context)
	}

	resultsDir := filepath.Join(projectDir, ".noise", "runs", run.ID, "results")
	for _, path := range []string{
		filepath.Join(resultsDir, "receivers.json"),
		filepath.Join(resultsDir, "receivers.csv"),
		filepath.Join(resultsDir, "bub-road.json"),
		filepath.Join(resultsDir, "bub-road.bin"),
		filepath.Join(resultsDir, "run-summary.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected result file %s: %v", path, err)
		}
	}

	payload, err := os.ReadFile(filepath.Join(resultsDir, "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable
	if err := json.Unmarshal(payload, &table); err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	expectedIndicators := map[string]bool{
		bubroad.IndicatorLden:   false,
		bubroad.IndicatorLnight: false,
	}
	for _, indicator := range table.IndicatorOrder {
		if _, ok := expectedIndicators[indicator]; ok {
			expectedIndicators[indicator] = true
		}
	}

	for indicator, found := range expectedIndicators {
		if !found {
			t.Fatalf("expected indicator %s in receiver table order", indicator)
		}
	}

	provenancePayload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var provenance struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(provenancePayload, &provenance); err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if provenance.Metadata["model_version"] != bubroad.BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", provenance.Metadata)
	}

	if provenance.Metadata["compliance_boundary"] != "baseline-preview-expanded-bub-road-contract" {
		t.Fatalf("unexpected compliance boundary: %#v", provenance.Metadata)
	}
}

func TestRunCnossosAircraftProducesOutputs(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase13", "aircraft_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase13", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "cnossos-aircraft")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) == 0 {
		t.Fatal("expected one run")
	}

	run := proj.Runs[len(proj.Runs)-1]
	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected completed run status, got %q", run.Status)
	}

	resultsDir := filepath.Join(projectDir, ".noise", "runs", run.ID, "results")
	for _, path := range []string{
		filepath.Join(resultsDir, "receivers.json"),
		filepath.Join(resultsDir, "receivers.csv"),
		filepath.Join(resultsDir, "cnossos-aircraft.json"),
		filepath.Join(resultsDir, "cnossos-aircraft.bin"),
		filepath.Join(resultsDir, "run-summary.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected result file %s: %v", path, err)
		}
	}

	payload, err := os.ReadFile(filepath.Join(resultsDir, "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable
	if err := json.Unmarshal(payload, &table); err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	expectedIndicators := map[string]bool{
		cnossosaircraft.IndicatorLden:   false,
		cnossosaircraft.IndicatorLnight: false,
	}
	for _, indicator := range table.IndicatorOrder {
		if _, ok := expectedIndicators[indicator]; ok {
			expectedIndicators[indicator] = true
		}
	}

	for indicator, found := range expectedIndicators {
		if !found {
			t.Fatalf("expected indicator %s in receiver table order", indicator)
		}
	}

	provenancePayload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var provenance struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(provenancePayload, &provenance); err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if provenance.Metadata["model_version"] != cnossosaircraft.BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", provenance.Metadata)
	}

	if provenance.Metadata["compliance_boundary"] != "baseline-preview-expanded-cnossos-aircraft-contract" {
		t.Fatalf("unexpected compliance boundary: %#v", provenance.Metadata)
	}
}

func TestRunBUFAircraftProducesOutputs(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase15", "aircraft_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase15", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "buf-aircraft")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) == 0 {
		t.Fatal("expected one run")
	}

	run := proj.Runs[len(proj.Runs)-1]
	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected completed run status, got %q", run.Status)
	}

	if run.Standard.Context != "mapping" {
		t.Fatalf("expected mapping context, got %q", run.Standard.Context)
	}

	resultsDir := filepath.Join(projectDir, ".noise", "runs", run.ID, "results")
	for _, path := range []string{
		filepath.Join(resultsDir, "receivers.json"),
		filepath.Join(resultsDir, "receivers.csv"),
		filepath.Join(resultsDir, "buf-aircraft.json"),
		filepath.Join(resultsDir, "buf-aircraft.bin"),
		filepath.Join(resultsDir, "run-summary.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected result file %s: %v", path, err)
		}
	}

	payload, err := os.ReadFile(filepath.Join(resultsDir, "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable
	if err := json.Unmarshal(payload, &table); err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	expectedIndicators := map[string]bool{
		bufaircraft.IndicatorLden:   false,
		bufaircraft.IndicatorLnight: false,
	}
	for _, indicator := range table.IndicatorOrder {
		if _, ok := expectedIndicators[indicator]; ok {
			expectedIndicators[indicator] = true
		}
	}

	for indicator, found := range expectedIndicators {
		if !found {
			t.Fatalf("expected indicator %s in receiver table order", indicator)
		}
	}

	provenancePayload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var provenance struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(provenancePayload, &provenance); err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if provenance.Metadata["model_version"] != bufaircraft.BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", provenance.Metadata)
	}

	if provenance.Metadata["compliance_boundary"] != "baseline-preview-expanded-buf-aircraft-contract" {
		t.Fatalf("unexpected compliance boundary: %#v", provenance.Metadata)
	}
}

func TestRunBEBExposureProducesOutputs(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase16", "beb_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase16", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "beb-exposure")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) == 0 {
		t.Fatal("expected one run")
	}

	run := proj.Runs[len(proj.Runs)-1]
	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected completed run status, got %q", run.Status)
	}

	if run.Standard.Context != "mapping" {
		t.Fatalf("expected mapping context, got %q", run.Standard.Context)
	}

	resultsDir := filepath.Join(projectDir, ".noise", "runs", run.ID, "results")
	for _, path := range []string{
		filepath.Join(resultsDir, "buildings.json"),
		filepath.Join(resultsDir, "buildings.csv"),
		filepath.Join(resultsDir, "beb-exposure.json"),
		filepath.Join(resultsDir, "beb-exposure.bin"),
		filepath.Join(resultsDir, "beb-summary.json"),
		filepath.Join(resultsDir, "run-summary.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected result file %s: %v", path, err)
		}
	}

	payload, err := os.ReadFile(filepath.Join(resultsDir, "buildings.json"))
	if err != nil {
		t.Fatalf("read building table: %v", err)
	}

	var table results.ReceiverTable
	if err := json.Unmarshal(payload, &table); err != nil {
		t.Fatalf("decode building table: %v", err)
	}

	expectedIndicators := map[string]bool{
		bebexposure.IndicatorAffectedPersonsLden:     false,
		bebexposure.IndicatorAffectedDwellingsLnight: false,
	}
	for _, indicator := range table.IndicatorOrder {
		if _, ok := expectedIndicators[indicator]; ok {
			expectedIndicators[indicator] = true
		}
	}

	for indicator, found := range expectedIndicators {
		if !found {
			t.Fatalf("expected indicator %s in building table order", indicator)
		}
	}

	provenancePayload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var provenance struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(provenancePayload, &provenance); err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if provenance.Metadata["model_version"] != bebexposure.BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", provenance.Metadata)
	}

	if provenance.Metadata["compliance_boundary"] != "baseline-preview-expanded-beb-exposure-contract" {
		t.Fatalf("unexpected compliance boundary: %#v", provenance.Metadata)
	}
}

func TestRunCnossosIndustryProducesOutputs(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase12", "industry_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase12", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "cnossos-industry")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) == 0 {
		t.Fatal("expected one run")
	}

	run := proj.Runs[len(proj.Runs)-1]
	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected completed run status, got %q", run.Status)
	}

	resultsDir := filepath.Join(projectDir, ".noise", "runs", run.ID, "results")
	for _, path := range []string{
		filepath.Join(resultsDir, "receivers.json"),
		filepath.Join(resultsDir, "receivers.csv"),
		filepath.Join(resultsDir, "cnossos-industry.json"),
		filepath.Join(resultsDir, "cnossos-industry.bin"),
		filepath.Join(resultsDir, "run-summary.json"),
	} {
		_, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected result file %s: %v", path, err)
		}
	}

	payload, err := os.ReadFile(filepath.Join(resultsDir, "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable

	err = json.Unmarshal(payload, &table)
	if err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	expectedIndicators := map[string]bool{
		cnossosindustry.IndicatorLden:   false,
		cnossosindustry.IndicatorLnight: false,
	}
	for _, indicator := range table.IndicatorOrder {
		if _, ok := expectedIndicators[indicator]; ok {
			expectedIndicators[indicator] = true
		}
	}

	for indicator, found := range expectedIndicators {
		if !found {
			t.Fatalf("expected indicator %s in receiver table order", indicator)
		}
	}
}

func TestRunBEBExposureWithBUFAircraftUpstreamProducesOutputs(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase16", "beb_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase16BUF", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "beb-exposure", "--param", "upstream_mapping_standard=buf-aircraft")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	run := proj.Runs[len(proj.Runs)-1]
	resultsDir := filepath.Join(projectDir, ".noise", "runs", run.ID, "results")

	payload, err := os.ReadFile(filepath.Join(resultsDir, "beb-summary.json"))
	if err != nil {
		t.Fatalf("read beb summary: %v", err)
	}

	var summary struct {
		UpstreamMappingStandard string `json:"upstream_mapping_standard"`
	}
	if err := json.Unmarshal(payload, &summary); err != nil {
		t.Fatalf("decode beb summary: %v", err)
	}

	if summary.UpstreamMappingStandard != "buf-aircraft" {
		t.Fatalf("expected upstream_mapping_standard=buf-aircraft, got %q", summary.UpstreamMappingStandard)
	}
}

func TestRunRLS19RoadProducesOutputsAndProvenanceMetadata(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase17", "rls19_road_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase17", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "rls19-road")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) == 0 {
		t.Fatal("expected one run")
	}

	run := proj.Runs[len(proj.Runs)-1]
	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected completed run status, got %q", run.Status)
	}

	resultsDir := filepath.Join(projectDir, ".noise", "runs", run.ID, "results")
	for _, path := range []string{
		filepath.Join(resultsDir, "receivers.json"),
		filepath.Join(resultsDir, "receivers.csv"),
		filepath.Join(resultsDir, "rls19-road.json"),
		filepath.Join(resultsDir, "rls19-road.bin"),
		filepath.Join(resultsDir, "run-summary.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected result file %s: %v", path, err)
		}
	}

	payload, err := os.ReadFile(filepath.Join(resultsDir, "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable
	if err := json.Unmarshal(payload, &table); err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	expectedIndicators := map[string]bool{
		rls19road.IndicatorLrDay:   false,
		rls19road.IndicatorLrNight: false,
	}
	for _, indicator := range table.IndicatorOrder {
		if _, ok := expectedIndicators[indicator]; ok {
			expectedIndicators[indicator] = true
		}
	}

	for indicator, found := range expectedIndicators {
		if !found {
			t.Fatalf("expected indicator %s in receiver table order", indicator)
		}
	}

	provenancePayload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var provenance struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(provenancePayload, &provenance); err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if provenance.Metadata["data_pack_version"] != rls19road.BuiltinDataPackVersion {
		t.Fatalf("unexpected data_pack_version: %#v", provenance.Metadata)
	}

	if provenance.Metadata["reporting_precision_db"] != "0.1" {
		t.Fatalf("unexpected reporting_precision_db: %#v", provenance.Metadata)
	}
}

func TestRunRLS19RoadCustomReceiversProduceTableOnlyOutputs(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "rls19_custom_receivers.geojson")
	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "rls19-rd-1", "kind": "source", "source_type": "line"},
      "geometry": {"type": "LineString", "coordinates": [[0, 0], [120, 0]]}
    },
    {
      "type": "Feature",
      "properties": {"id": "rcv-1", "kind": "receiver", "height_m": 4},
      "geometry": {"type": "Point", "coordinates": [20, 15]}
    },
    {
      "type": "Feature",
      "properties": {"id": "rcv-2", "kind": "receiver", "height_m": 4},
      "geometry": {"type": "Point", "coordinates": [60, 25]}
    }
  ]
}`)
	if err := os.WriteFile(modelPath, payload, 0o644); err != nil {
		t.Fatalf("write custom model: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase30", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "rls19-road", "--receiver-mode", "custom")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	run := proj.Runs[len(proj.Runs)-1]
	if run.ReceiverMode != "custom" {
		t.Fatalf("expected custom receiver mode, got %q", run.ReceiverMode)
	}
	if run.ReceiverSetID != "explicit-manual" {
		t.Fatalf("expected explicit receiver set id, got %q", run.ReceiverSetID)
	}

	resultsDir := filepath.Join(projectDir, ".noise", "runs", run.ID, "results")
	assertFileExists(t, filepath.Join(resultsDir, "receivers.json"))
	assertFileExists(t, filepath.Join(resultsDir, "receivers.csv"))
	assertFileExists(t, filepath.Join(resultsDir, "run-summary.json"))

	if _, err := os.Stat(filepath.Join(resultsDir, "rls19-road.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no raster metadata for custom receiver run, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(resultsDir, "rls19-road.bin")); !os.IsNotExist(err) {
		t.Fatalf("expected no raster data for custom receiver run, got err=%v", err)
	}

	receiverPayload, err := os.ReadFile(filepath.Join(resultsDir, "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable
	if err := json.Unmarshal(receiverPayload, &table); err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}
	if len(table.Records) != 2 {
		t.Fatalf("expected 2 explicit receivers, got %d", len(table.Records))
	}
	if table.Records[0].ID != "rcv-1" || table.Records[1].ID != "rcv-2" {
		t.Fatalf("unexpected receiver ordering: %#v", table.Records)
	}

	summaryPayload, err := os.ReadFile(filepath.Join(resultsDir, "run-summary.json"))
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}

	var summary map[string]any
	if err := json.Unmarshal(summaryPayload, &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary["receiver_mode"] != "custom" {
		t.Fatalf("unexpected receiver_mode in summary: %#v", summary)
	}
	if _, ok := summary["grid_width"]; ok {
		t.Fatalf("did not expect grid_width in custom receiver summary: %#v", summary)
	}

	provenancePayload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var provenance project.ProvenanceManifest
	if err := json.Unmarshal(provenancePayload, &provenance); err != nil {
		t.Fatalf("decode provenance: %v", err)
	}
	if provenance.ReceiverMode != "custom" {
		t.Fatalf("expected custom receiver mode in provenance, got %q", provenance.ReceiverMode)
	}
}

func TestRunRLS19RoadCustomReceiversUsePerReceiverHeight(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "rls19_receiver_heights.geojson")
	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "rls19-rd-1", "kind": "source", "source_type": "line"},
      "geometry": {"type": "LineString", "coordinates": [[0, 0], [120, 0]]}
    },
    {
      "type": "Feature",
      "properties": {"id": "bar-1", "kind": "barrier", "height_m": 4},
      "geometry": {"type": "LineString", "coordinates": [[-20, 10], [140, 10]]}
    },
    {
      "type": "Feature",
      "properties": {"id": "rcv-low", "kind": "receiver", "height_m": 2},
      "geometry": {"type": "Point", "coordinates": [60, 50]}
    },
    {
      "type": "Feature",
      "properties": {"id": "rcv-high", "kind": "receiver", "height_m": 15},
      "geometry": {"type": "Point", "coordinates": [60, 50]}
    }
  ]
}`)
	if err := os.WriteFile(modelPath, payload, 0o644); err != nil {
		t.Fatalf("write custom model: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase17ReceiverHeight", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "rls19-road", "--receiver-mode", "custom")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	run := proj.Runs[len(proj.Runs)-1]
	receiverPayload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "runs", run.ID, "results", "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable
	if err := json.Unmarshal(receiverPayload, &table); err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	if len(table.Records) != 2 {
		t.Fatalf("expected 2 explicit receivers, got %d", len(table.Records))
	}

	if table.Records[0].Values[rls19road.IndicatorLrDay] >= table.Records[1].Values[rls19road.IndicatorLrDay] {
		t.Fatalf(
			"expected higher receiver to be louder in barrier scenario: low=%.4f high=%.4f",
			table.Records[0].Values[rls19road.IndicatorLrDay],
			table.Records[1].Values[rls19road.IndicatorLrDay],
		)
	}
}

func TestExtractRLS19RoadSourcesDirectionalGeometry(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "rd-main",
        "kind": "source",
        "source_type": "line",
        "rls19_directional_sources": [
          {
            "id": "northbound",
            "centerline": [[0, -2, 101], [120, -2, 103]],
            "traffic_day_pkw": 650,
            "traffic_night_pkw": 180,
            "speed_pkw_kph": 90
          },
          {
            "id": "southbound",
            "centerline": [[0, 2], [120, 2]],
            "centerline_elevations": [100, 102],
            "traffic_day_pkw": 350,
            "traffic_night_pkw": 70,
            "speed_pkw_kph": 70
          }
        ]
      },
      "geometry": {"type": "LineString", "coordinates": [[0, 0], [120, 0]]}
    }
  ]
}`)

	model, err := modelgeojson.Normalize(payload, "EPSG:25832", "directional.geojson")
	if err != nil {
		t.Fatalf("normalize model: %v", err)
	}

	sources, overrideCount, err := extractRLS19RoadSources(model, rls19RoadRunOptions{
		SurfaceType:      string(rls19road.SurfaceSMA),
		SpeedPkwKPH:      100,
		SpeedLkw1KPH:     80,
		SpeedLkw2KPH:     70,
		SpeedKradKPH:     100,
		TrafficDayPkw:    900,
		TrafficDayLkw1:   40,
		TrafficDayLkw2:   60,
		TrafficDayKrad:   10,
		TrafficNightPkw:  200,
		TrafficNightLkw1: 10,
		TrafficNightLkw2: 20,
		TrafficNightKrad: 2,
	}, []string{"line"})
	if err != nil {
		t.Fatalf("extract sources: %v", err)
	}

	if overrideCount != 1 {
		t.Fatalf("expected one override-bearing source feature, got %d", overrideCount)
	}

	if len(sources) != 2 {
		t.Fatalf("expected 2 directional sources, got %d", len(sources))
	}

	if sources[0].ID != "rd-main-northbound" || sources[1].ID != "rd-main-southbound" {
		t.Fatalf("unexpected source ids: %#v", []string{sources[0].ID, sources[1].ID})
	}

	if len(sources[0].CenterlineElevations) != 2 || len(sources[1].CenterlineElevations) != 2 {
		t.Fatalf("expected per-vertex elevations for both directional sources: %#v", sources)
	}

	if sources[0].CenterlineElevations[0] != 101 || sources[0].CenterlineElevations[1] != 103 {
		t.Fatalf("expected 3D geometry elevations on first direction, got %#v", sources[0].CenterlineElevations)
	}

	if sources[1].CenterlineElevations[0] != 100 || sources[1].CenterlineElevations[1] != 102 {
		t.Fatalf("expected centerline_elevations override on second direction, got %#v", sources[1].CenterlineElevations)
	}

	if sources[0].TrafficDay.PkwPerHour != 650 || sources[1].TrafficDay.PkwPerHour != 350 {
		t.Fatalf("expected direction-specific traffic split, got %#v", []float64{sources[0].TrafficDay.PkwPerHour, sources[1].TrafficDay.PkwPerHour})
	}

	if sources[0].Speeds.PkwKPH != 90 || sources[1].Speeds.PkwKPH != 70 {
		t.Fatalf("expected direction-specific speeds, got %#v", []float64{sources[0].Speeds.PkwKPH, sources[1].Speeds.PkwKPH})
	}
}

func TestRunRLS19RoadPerSourceAcousticsRecordedInSummary(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "rls19_per_source.geojson")
	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "rd-override",
        "kind": "source",
        "source_type": "line",
        "surface_type": "OPA",
        "traffic_day_pkw": 300
      },
      "geometry": {"type": "LineString", "coordinates": [[0, 0], [120, 0]]}
    },
    {
      "type": "Feature",
      "properties": {"id": "rd-default", "kind": "source", "source_type": "line"},
      "geometry": {"type": "LineString", "coordinates": [[0, 50], [120, 50]]}
    }
  ]
}`)
	if err := os.WriteFile(modelPath, payload, 0o644); err != nil {
		t.Fatalf("write model: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase31", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "rls19-road")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	run := proj.Runs[len(proj.Runs)-1]
	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected completed run, got %q", run.Status)
	}

	summaryPayload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "runs", run.ID, "results", "run-summary.json"))
	if err != nil {
		t.Fatalf("read run-summary: %v", err)
	}

	var summary map[string]any
	if err := json.Unmarshal(summaryPayload, &summary); err != nil {
		t.Fatalf("decode run-summary: %v", err)
	}

	// One source had per-source acoustic overrides; the other used run-wide defaults.
	overrideCount, ok := summary["sources_with_feature_acoustics_overrides"]
	if !ok {
		t.Fatalf("expected sources_with_feature_acoustics_overrides in summary, got: %v", summary)
	}

	// JSON numbers decode as float64.
	if overrideCount.(float64) != 1 {
		t.Fatalf("expected sources_with_feature_acoustics_overrides=1, got %v", overrideCount)
	}

	// The log must also record the override count.
	logPayload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "runs", run.ID, "run.log"))
	if err != nil {
		t.Fatalf("read run.log: %v", err)
	}

	if !strings.Contains(string(logPayload), "rls19_sources_with_feature_overrides=1") {
		t.Fatalf("run.log missing rls19_sources_with_feature_overrides=1:\n%s", logPayload)
	}
}

func TestRunSchall03ProducesOutputsAndProvenanceMetadata(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase18", "schall03_rail_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase18", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "schall03")

	store, err := projectfs.New(projectDir)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	proj, err := store.Load()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(proj.Runs) == 0 {
		t.Fatal("expected one run")
	}

	run := proj.Runs[len(proj.Runs)-1]
	if run.Status != project.RunStatusCompleted {
		t.Fatalf("expected completed run status, got %q", run.Status)
	}

	resultsDir := filepath.Join(projectDir, ".noise", "runs", run.ID, "results")
	for _, path := range []string{
		filepath.Join(resultsDir, "receivers.json"),
		filepath.Join(resultsDir, "receivers.csv"),
		filepath.Join(resultsDir, "schall03.json"),
		filepath.Join(resultsDir, "schall03.bin"),
		filepath.Join(resultsDir, "run-summary.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected result file %s: %v", path, err)
		}
	}

	payload, err := os.ReadFile(filepath.Join(resultsDir, "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable
	if err := json.Unmarshal(payload, &table); err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	expectedIndicators := map[string]bool{
		schall03.IndicatorLrDay:   false,
		schall03.IndicatorLrNight: false,
	}
	for _, indicator := range table.IndicatorOrder {
		if _, ok := expectedIndicators[indicator]; ok {
			expectedIndicators[indicator] = true
		}
	}

	for indicator, found := range expectedIndicators {
		if !found {
			t.Fatalf("expected indicator %s in receiver table order", indicator)
		}
	}

	provenancePayload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var provenance struct {
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(provenancePayload, &provenance); err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if provenance.Metadata["model_version"] != schall03.BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", provenance.Metadata)
	}

	if provenance.Metadata["data_pack_version"] != schall03.BuiltinDataPackVersion {
		t.Fatalf("unexpected data_pack_version: %#v", provenance.Metadata)
	}

	if provenance.Metadata["compliance_boundary"] != "baseline-preview-no-normative-tables" {
		t.Fatalf("unexpected compliance boundary: %#v", provenance.Metadata)
	}
}

func mustRunCLI(t *testing.T, args ...string) {
	t.Helper()

	err := runCLI(args...)
	if err != nil {
		t.Fatalf("noise %v: %v", args, err)
	}
}

func runCLI(args ...string) error {
	cmd := newRootCommand()
	cmd.SetArgs(args)

	err := cmd.Execute()
	if err != nil {
		return fmt.Errorf("noise %v: %w", args, err)
	}

	return nil
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
