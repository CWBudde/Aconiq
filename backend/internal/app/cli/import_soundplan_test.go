package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

func TestImportSoundPlanWritesNormalizedModelAndReport(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	soundPlanDir := soundPlanInteropPath(t)

	mustRunCLI(t, "--project", projectDir, "init", "--name", "SoundPLAN", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--from-soundplan", soundPlanDir)

	modelPath := filepath.Join(projectDir, ".noise", "model", "model.normalized.geojson")
	reportPath := filepath.Join(projectDir, ".noise", "model", "soundplan-import-report.json")

	modelPayload, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read normalized model: %v", err)
	}

	var fc modelgeojson.FeatureCollection
	if err := json.Unmarshal(modelPayload, &fc); err != nil {
		t.Fatalf("decode normalized model: %v", err)
	}

	if len(fc.Features) < 395 {
		t.Fatalf("expected at least 395 features, got %d", len(fc.Features))
	}

	counts := make(map[string]int)
	for _, feature := range fc.Features {
		kind, _ := feature.Properties["kind"].(string)
		counts[kind]++
	}

	if counts["building"] != 315 {
		t.Fatalf("building count = %d, want 315", counts["building"])
	}

	if counts["receiver"] != 77 {
		t.Fatalf("receiver count = %d, want 77", counts["receiver"])
	}

	if counts["barrier"] != 1 {
		t.Fatalf("barrier count = %d, want 1", counts["barrier"])
	}

	if counts["source"] < 2 {
		t.Fatalf("source count = %d, want at least 2", counts["source"])
	}

	var sawDerivedRail bool
	var sawAddressedBuilding bool
	var sawBarrierAcoustics bool
	for _, feature := range fc.Features {
		kind, _ := feature.Properties["kind"].(string)
		switch kind {
		case "source":
			if value, ok := feature.Properties["soundplan_dominant_train_name"].(string); ok && value != "" {
				sawDerivedRail = true
			}

			if feature.Properties["traffic_day_trains_per_hour"] == nil {
				t.Fatal("expected traffic_day_trains_per_hour on imported rail source")
			}

		case "building":
			address, _ := feature.Properties["soundplan_address"].(string)
			if address == "Hauptstraße 4" {
				sawAddressedBuilding = true
				if _, ok := feature.Properties["soundplan_placeholder_height"]; ok {
					t.Fatal("expected parsed SoundPLAN building height, not placeholder height metadata")
				}
			}

		case "barrier":
			absorptionA, okA := feature.Properties["soundplan_barrier_absorption_a_db"].(float64)
			absorptionB, okB := feature.Properties["soundplan_barrier_absorption_b_db"].(float64)
			if okA && okB && absorptionA == 30 && absorptionB == 30 {
				sawBarrierAcoustics = true
			}
		}
	}

	if !sawDerivedRail {
		t.Fatal("expected at least one imported rail source with derived dominant train name")
	}

	if !sawAddressedBuilding {
		t.Fatal("expected at least one imported building with parsed SoundPLAN address metadata")
	}

	if !sawBarrierAcoustics {
		t.Fatal("expected imported SoundPLAN barrier acoustic properties from GeoWand :D! records")
	}

	reportPayload, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read soundplan import report: %v", err)
	}

	var report soundPlanImportReport
	if err := json.Unmarshal(reportPayload, &report); err != nil {
		t.Fatalf("decode soundplan import report: %v", err)
	}

	if report.Format != "soundplan" {
		t.Fatalf("format = %q, want soundplan", report.Format)
	}

	if report.CountsByKind["building"] != 315 {
		t.Fatalf("report building count = %d, want 315", report.CountsByKind["building"])
	}

	if report.TerrainSource != "GeoTmp.geo" {
		t.Fatalf("terrain source = %q, want GeoTmp.geo", report.TerrainSource)
	}

	if report.GridResolutionM != 5.0 {
		t.Fatalf("grid_resolution_m = %v, want 5.0", report.GridResolutionM)
	}

	if report.CalcAreaBounds == nil {
		t.Fatal("expected calc_area_bounds")
	}

	if len(report.GridMaps) != 4 {
		t.Fatalf("grid map count = %d, want 4", len(report.GridMaps))
	}

	if !report.GridMaps[0].DecodedValues {
		t.Fatal("expected decoded SoundPLAN grid-map values")
	}

	if report.GridMaps[0].ActiveCellCount != 5961 {
		t.Fatalf("active_cell_count = %d, want 5961", report.GridMaps[0].ActiveCellCount)
	}

	if len(report.StandardMappings) == 0 {
		t.Fatal("expected standard mappings in report")
	}

	if len(report.Warnings) == 0 {
		t.Fatal("expected non-empty import warnings")
	}

	for _, warning := range report.Warnings {
		if strings.Contains(warning, "building heights are not yet available from GeoObjs.geo attributes") {
			t.Fatalf("unexpected legacy GeoObjs height warning: %q", warning)
		}
	}

	if report.CountsByKind["source"] < 2 {
		t.Fatalf("report source count = %d, want at least 2", report.CountsByKind["source"])
	}
}

func soundPlanInteropPath(t *testing.T) string {
	t.Helper()

	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}

	path := filepath.Join(filepath.Dir(filePath), "..", "..", "..", "..", "interoperability", "Schienenprojekt - Schall 03")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("sample SoundPLAN project not available")
	}

	return path
}
