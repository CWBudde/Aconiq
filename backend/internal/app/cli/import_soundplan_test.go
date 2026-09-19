package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/qa/fixtures"
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

	if len(fc.Features) < 348 {
		t.Fatalf("expected at least 348 features, got %d", len(fc.Features))
	}

	counts := make(map[string]int)

	for _, feature := range fc.Features {
		kind, _ := feature.Properties["kind"].(string)
		counts[kind]++
	}

	if counts["building"] != 315 {
		t.Fatalf("building count = %d, want 315", counts["building"])
	}

	// 13 immission points, expanded to one receiver per floor: 2 or 3 floors
	// each, 30 in total, which is exactly the row count of RREC*.abs. The 77
	// this used to assert was the map label layer.
	if counts["receiver"] != 30 {
		t.Fatalf("receiver count = %d, want 30", counts["receiver"])
	}

	assertSoundPlanReceiverColumns(t, fc)

	if counts["barrier"] != 1 {
		t.Fatalf("barrier count = %d, want 1", counts["barrier"])
	}

	if counts["source"] < 2 {
		t.Fatalf("source count = %d, want at least 2", counts["source"])
	}

	var (
		sawDerivedRail       bool
		sawDerivedTraction   bool
		sawAddressedBuilding bool
		sawBarrierAcoustics  bool
	)

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

			if value, ok := feature.Properties["rail_traction_type"].(string); ok && value == "mixed" {
				sawDerivedTraction = true
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

	if !sawDerivedTraction {
		t.Fatal("expected at least one imported rail source with derived non-placeholder traction metadata")
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

	if report.CalcArea == nil {
		t.Fatal("expected calc_area metadata")
	}

	if len(report.CalcArea.Points) == 0 {
		t.Fatal("expected at least one calc_area point")
	}

	if !report.CalcArea.IsClosed {
		t.Fatal("expected calc_area polygon to be closed")
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

// assertSoundPlanReceiverColumns checks that every imported receiver carries
// the SoundPLAN identity the comparison keys on, and that the identities are
// what the reference project says they are: 13 immission points, each expanded
// into a column of floors, no two receivers sharing an ID.
func assertSoundPlanReceiverColumns(t *testing.T, fc modelgeojson.FeatureCollection) {
	t.Helper()

	objIDs := make(map[float64]bool)
	ids := make(map[string]bool)

	for _, feature := range fc.Features {
		if kind, _ := feature.Properties["kind"].(string); kind != "receiver" {
			continue
		}

		id, _ := feature.Properties["id"].(string)
		if id == "" {
			t.Fatalf("receiver without an id: %#v", feature.Properties)
		}

		if ids[id] {
			t.Fatalf("receiver id %q is not unique", id)
		}

		ids[id] = true

		objID, ok := feature.Properties["soundplan_obj_id"].(float64)
		if !ok || objID <= 0 {
			t.Fatalf("receiver %s carries no soundplan_obj_id", id)
		}

		objIDs[objID] = true

		if _, ok := feature.Properties["soundplan_floor"].(float64); !ok {
			t.Fatalf("receiver %s carries no soundplan_floor", id)
		}

		if name, ok := feature.Properties["soundplan_receiver_name"].(string); !ok || name == "" {
			t.Fatalf("receiver %s carries no soundplan_receiver_name", id)
		}

		heightM, ok := feature.Properties["height_m"].(float64)
		if !ok || heightM <= 0 {
			t.Fatalf("receiver %s height_m = %v, want > 0", id, feature.Properties["height_m"])
		}
	}

	if len(objIDs) != 13 {
		t.Fatalf("distinct soundplan_obj_id = %d, want 13", len(objIDs))
	}
}

func soundPlanInteropPath(t *testing.T) string {
	t.Helper()

	return fixtures.SoundPLANProjectDir(t)
}

// TestImportSoundPlanRegistersAllFourModelArtifacts pins what the manifest
// holds after a SoundPLAN import.
//
// Nothing asserted this before, which is why the importer could write the
// three model files through its own non-atomic writer while claiming, in
// SaveModel's doc comment, that the store was "the one persistence path for a
// model". The reroute is only safe if the four IDs, kinds and relative paths
// come out identical, so this test is the net under it.
func TestImportSoundPlanRegistersAllFourModelArtifacts(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "SoundPLAN", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--from-soundplan", soundPlanInteropPath(t))

	payload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "project.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest struct {
		Artifacts []struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
			Path string `json:"path"`
		} `json:"artifacts"`
	}

	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}

	want := map[string][2]string{
		"artifact-model-normalized":        {"model.normalized_geojson", ".noise/model/model.normalized.geojson"},
		"artifact-model-dump":              {"model.dump_json", ".noise/model/model.dump.json"},
		"artifact-model-validation":        {"model.validation_report", ".noise/model/validation-report.json"},
		"artifact-soundplan-import-report": {"model.soundplan_import_report", ".noise/model/soundplan-import-report.json"},
	}

	got := make(map[string][2]string, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		got[artifact.ID] = [2]string{artifact.Kind, artifact.Path}
	}

	for id, wantPair := range want {
		gotPair, ok := got[id]
		if !ok {
			t.Fatalf("manifest has no artifact %q; it holds %v", id, got)
		}

		if gotPair != wantPair {
			t.Fatalf("artifact %q is {kind: %q, path: %q}, want {kind: %q, path: %q}",
				id, gotPair[0], gotPair[1], wantPair[0], wantPair[1])
		}
	}

	for _, pair := range want {
		if _, statErr := os.Stat(filepath.Join(projectDir, filepath.FromSlash(pair[1]))); statErr != nil {
			t.Fatalf("artifact file missing: %v", statErr)
		}
	}
}
