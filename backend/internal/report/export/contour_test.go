package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExportContourGeoJSON(t *testing.T) {
	t.Parallel()

	contours := []ContourLine{
		{
			Level:    50,
			BandName: "Lden",
			Points:   [][2]float64{{100, 200}, {110, 210}, {120, 200}},
		},
		{
			Level:    55,
			BandName: "Lden",
			Points:   [][2]float64{{105, 205}, {115, 215}},
		},
	}

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "contours.geojson")

	err := ExportContourGeoJSON(outPath, contours)
	if err != nil {
		t.Fatalf("export contour geojson: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var fc map[string]any

	err = json.Unmarshal(data, &fc)
	if err != nil {
		t.Fatalf("decode geojson: %v", err)
	}

	if fc["type"] != "FeatureCollection" {
		t.Fatalf("type = %v, want FeatureCollection", fc["type"])
	}

	features, ok := fc["features"].([]any)
	if !ok || len(features) != 2 {
		t.Fatalf("expected 2 features, got %v", fc["features"])
	}

	// Verify first feature.
	f0 := features[0].(map[string]any)
	props := f0["properties"].(map[string]any)

	if props["level_db"].(float64) != 50 {
		t.Fatalf("first feature level = %v, want 50", props["level_db"])
	}

	if props["band_name"].(string) != "Lden" {
		t.Fatalf("first feature band = %v, want Lden", props["band_name"])
	}
}

func TestExportContourGeoJSONEmpty(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "empty.geojson")

	err := ExportContourGeoJSON(outPath, nil)
	if err != nil {
		t.Fatalf("export empty contours: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var fc map[string]any

	err = json.Unmarshal(data, &fc)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	features := fc["features"].([]any)
	if len(features) != 0 {
		t.Fatalf("expected 0 features, got %d", len(features))
	}
}
