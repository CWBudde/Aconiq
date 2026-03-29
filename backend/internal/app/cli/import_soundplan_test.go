package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	for _, feature := range fc.Features {
		kind, _ := feature.Properties["kind"].(string)
		if kind != "source" {
			continue
		}

		if value, ok := feature.Properties["soundplan_dominant_train_name"].(string); ok && value != "" {
			sawDerivedRail = true
		}

		if feature.Properties["traffic_day_trains_per_hour"] == nil {
			t.Fatal("expected traffic_day_trains_per_hour on imported rail source")
		}
	}

	if !sawDerivedRail {
		t.Fatal("expected at least one imported rail source with derived dominant train name")
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

	if len(report.StandardMappings) == 0 {
		t.Fatal("expected standard mappings in report")
	}

	if len(report.Warnings) == 0 {
		t.Fatal("expected non-empty import warnings")
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
