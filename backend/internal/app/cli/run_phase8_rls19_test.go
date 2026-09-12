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
	"github.com/aconiq/backend/internal/report/results"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

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

	err = json.Unmarshal(provenancePayload, &provenance)
	if err != nil {
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

	err := os.WriteFile(modelPath, payload, 0o600)
	if err != nil {
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

	_, err = os.Stat(filepath.Join(resultsDir, "rls19-road.json"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected no raster metadata for custom receiver run, got err=%v", err)
	}

	_, err = os.Stat(filepath.Join(resultsDir, "rls19-road.bin"))
	if !os.IsNotExist(err) {
		t.Fatalf("expected no raster data for custom receiver run, got err=%v", err)
	}

	receiverPayload, err := os.ReadFile(filepath.Join(resultsDir, "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable

	err = json.Unmarshal(receiverPayload, &table)
	if err != nil {
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

	err = json.Unmarshal(summaryPayload, &summary)
	if err != nil {
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

	err = json.Unmarshal(provenancePayload, &provenance)
	if err != nil {
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

	err := os.WriteFile(modelPath, payload, 0o600)
	if err != nil {
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

	err = json.Unmarshal(receiverPayload, &table)
	if err != nil {
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

func TestExtractRLS19RoadSourcesRejectsMixedDirectionalSurfaceTypes(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "rd-mixed-surface",
        "kind": "source",
        "source_type": "line",
        "surface_type": "SMA",
        "rls19_directional_sources": [
          {
            "id": "northbound",
            "centerline": [[0, -2], [120, -2]],
            "surface_type": "OPA"
          },
          {
            "id": "southbound",
            "centerline": [[0, 2], [120, 2]],
            "surface_type": "Beton"
          }
        ]
      },
      "geometry": {"type": "LineString", "coordinates": [[0, 0], [120, 0]]}
    }
  ]
}`)

	model, err := modelgeojson.Normalize(payload, "EPSG:25832", "directional-mixed-surface.geojson")
	if err != nil {
		t.Fatalf("normalize model: %v", err)
	}

	_, _, err = extractRLS19RoadSources(model, rls19RoadRunOptions{
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
	if err == nil {
		t.Fatal("expected error for mixed directional surface types")
	}

	if !strings.Contains(err.Error(), "shared surface_type") {
		t.Fatalf("expected mixed-surface guidance in error, got %v", err)
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

	err := os.WriteFile(modelPath, payload, 0o600)
	if err != nil {
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

	err = json.Unmarshal(summaryPayload, &summary)
	if err != nil {
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
	modelPath := testdataPath(t, "phase18", "schall03_normative_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase18", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "schall03", "--param", "grid_resolution_m=25")

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

	err = json.Unmarshal(provenancePayload, &provenance)
	if err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if provenance.Metadata["schall03_engine"] != schall03.EngineNormative {
		t.Fatalf("expected the normative engine to be resolved: %#v", provenance.Metadata)
	}

	if provenance.Metadata["model_version"] != schall03.NormativeModelVersion {
		t.Fatalf("unexpected model_version: %#v", provenance.Metadata)
	}

	if provenance.Metadata["compliance_boundary"] != schall03.ComplianceBoundaryNormative {
		t.Fatalf("unexpected compliance boundary: %#v", provenance.Metadata)
	}

	// The normative chain reads Beiblatt 1/2 directly; recording a data-pack
	// version here would assert a coefficient source the run never opened.
	if _, stale := provenance.Metadata["data_pack_version"]; stale {
		t.Fatalf("normative run still records a data_pack_version: %#v", provenance.Metadata)
	}
}

// A model that carries only the preview rail_* vocabulary has no Zugart and no
// Fz composition, so there is nothing for the Anlage-2 tables to compute from.
// The run must say so rather than quietly falling back to invented spectra.
func TestRunSchall03AutoRefusesModelWithoutNormativeTrackData(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase18", "schall03_rail_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase18", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	err := runCLI("--project", projectDir, "run", "--standard", "schall03")
	if err == nil {
		t.Fatal("expected the run to fail without normative track data")
	}

	for _, want := range []string{"schall03_operations", "schall03_engine=preview"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error message does not mention %q: %v", want, err)
		}
	}
}

// The preview data pack stays reachable, but only on purpose and only under a
// compliance boundary that says what it is.
func TestRunSchall03PreviewEngineIsOptInAndLabelled(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase18", "schall03_rail_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Phase18", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "schall03", "--param", "schall03_engine=preview")

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

	provenancePayload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.ProvenancePath)))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}

	var provenance struct {
		Metadata map[string]string `json:"metadata"`
	}

	err = json.Unmarshal(provenancePayload, &provenance)
	if err != nil {
		t.Fatalf("decode provenance: %v", err)
	}

	if provenance.Metadata["schall03_engine"] != schall03.EnginePreview {
		t.Fatalf("expected the preview engine to be resolved: %#v", provenance.Metadata)
	}

	if provenance.Metadata["model_version"] != schall03.PreviewModelVersion {
		t.Fatalf("unexpected model_version: %#v", provenance.Metadata)
	}

	if provenance.Metadata["compliance_boundary"] != schall03.ComplianceBoundaryPreview {
		t.Fatalf("unexpected compliance boundary: %#v", provenance.Metadata)
	}

	logPayload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(run.LogPath)))
	if err != nil {
		t.Fatalf("read run.log: %v", err)
	}

	if !strings.Contains(string(logPayload), "WARNING schall03_engine=preview") {
		t.Fatalf("run.log carries no preview warning:\n%s", logPayload)
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

// TestRunRLS19RoadParkingOnlyModelCompletes drives a model whose only source is
// an RLS-19 §3.4 Parkplatz. Before the parking vocabulary existed, such a model
// was refused twice over — the extractor demanded a line source and
// ComputeReceiverLevels demanded any source at all — so P1–P4 of the
// conformance declaration described code no run could reach.
func TestRunRLS19RoadParkingOnlyModelCompletes(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase17", "rls19_parking_model.geojson")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Parking", "--crs", "EPSG:25832")
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
		t.Fatalf("expected completed run status, got %q", run.Status)
	}

	summaryPayload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "runs", run.ID, "results", "run-summary.json"))
	if err != nil {
		t.Fatalf("read run summary: %v", err)
	}

	var summary map[string]any

	err = json.Unmarshal(summaryPayload, &summary)
	if err != nil {
		t.Fatalf("decode run summary: %v", err)
	}

	// source_count counts line sources, so without its own key a successful
	// Parkplatz-only run would report zero sources.
	if got := summary["parking_source_count"]; got != float64(1) {
		t.Errorf("parking_source_count = %v, want 1", got)
	}

	if got := summary["source_count"]; got != float64(0) {
		t.Errorf("source_count = %v, want 0", got)
	}

	// The automatic grid must be padded around the lot's whole footprint. With
	// only the centroid in the extent, a 200 m x 100 m lot at the default 20 m
	// padding would produce a 5x5 grid sitting entirely inside the source
	// instead of the 25x15 that covers it.
	if got := summary["grid_width"]; got != float64(25) {
		t.Errorf("grid_width = %v, want 25 (the polygon extent plus padding)", got)
	}

	if got := summary["grid_height"]; got != float64(15) {
		t.Errorf("grid_height = %v, want 15 (the polygon extent plus padding)", got)
	}

	logPayload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "runs", run.ID, "run.log"))
	if err != nil {
		t.Fatalf("read run log: %v", err)
	}

	if !strings.Contains(string(logPayload), "rls19_parking_sources=1") {
		t.Errorf("run log does not record the parking source count:\n%s", logPayload)
	}
}

// TestRunRLS19RoadParkingRaisesReceiverLevel proves the parking sources reach
// ComputeReceiverLevels rather than merely being parsed: the same receiver must
// read higher with the lot in the model than without it.
func TestRunRLS19RoadParkingRaisesReceiverLevel(t *testing.T) {
	t.Parallel()

	withParking := runRLS19ParkingScenario(t, true)
	withoutParking := runRLS19ParkingScenario(t, false)

	if !(withParking > withoutParking) {
		t.Errorf("a Parkplatz must raise the receiver level: with %.3f dB, without %.3f dB",
			withParking, withoutParking)
	}
}

// runRLS19ParkingScenario runs a one-road model, optionally with a Parkplatz
// beside it, and returns LrDay at the explicit receiver.
func runRLS19ParkingScenario(t *testing.T, includeParking bool) float64 {
	t.Helper()

	parking := ""
	if includeParking {
		parking = `{
      "type": "Feature",
      "properties": {
        "id": "lot", "kind": "source", "source_type": "area",
        "rls19_parking_num_spaces": 300,
        "rls19_parking_type": "lkw-omnibus",
        "rls19_parking_facility_type": "tank-rastanlage"
      },
      "geometry": {"type": "Polygon", "coordinates": [[[0,20],[100,20],[100,60],[0,60],[0,20]]]}
    },`
	}

	model := `{
  "type": "FeatureCollection",
  "features": [
    ` + parking + `
    {
      "type": "Feature",
      "properties": {"id": "road", "kind": "source", "source_type": "line"},
      "geometry": {"type": "LineString", "coordinates": [[0,0],[100,0]]}
    },
    {
      "type": "Feature",
      "properties": {"id": "rec-1", "kind": "receiver", "height_m": 4},
      "geometry": {"type": "Point", "coordinates": [50, 120]}
    }
  ]
}`

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")

	err := os.WriteFile(modelPath, []byte(model), 0o600)
	if err != nil {
		t.Fatalf("write model: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Parking", "--crs", "EPSG:25832")
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

	payload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "runs", run.ID, "results", "receivers.json"))
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	var table results.ReceiverTable

	err = json.Unmarshal(payload, &table)
	if err != nil {
		t.Fatalf("decode receiver table: %v", err)
	}

	for _, record := range table.Records {
		if record.ID == "rec-1" {
			return record.Values[rls19road.IndicatorLrDay]
		}
	}

	t.Fatal("receiver rec-1 not found in the result table")

	return 0
}

// TestRunRLS19RoadRefusesAModelWithNeitherSourceKind pins the runner's combined
// emptiness check, which replaced the road extractor's own.
func TestRunRLS19RoadRefusesAModelWithNeitherSourceKind(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")

	err := os.WriteFile(modelPath, []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "rec-1", "kind": "receiver", "height_m": 4},
      "geometry": {"type": "Point", "coordinates": [50, 120]}
    }
  ]
}`), 0o600)
	if err != nil {
		t.Fatalf("write model: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Empty", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	runErr := runCLI("--project", projectDir, "run", "--standard", "rls19-road")
	if runErr == nil {
		t.Fatal("a model with neither a line source nor a parking area must be refused")
	}

	if !strings.Contains(runErr.Error(), "line source or parking area") {
		t.Errorf("error %q should name both source kinds", runErr)
	}
}
