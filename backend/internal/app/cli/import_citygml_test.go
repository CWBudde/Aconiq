package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

func TestImportCityGMLWritesNormalizedBuildingModel(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := testdataPath(t, "phase34b", "buildings.citygml")

	mustRunCLI(t, "--project", projectDir, "init", "--name", "CityGML", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)

	payload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "model", "model.normalized.geojson"))
	if err != nil {
		t.Fatalf("read normalized model: %v", err)
	}

	var fc modelgeojson.FeatureCollection
	err = json.Unmarshal(payload, &fc)
	if err != nil {
		t.Fatalf("decode normalized model: %v", err)
	}

	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}

	feature := fc.Features[0]
	if feature.Properties["kind"] != "building" {
		t.Fatalf("expected building kind, got %#v", feature.Properties["kind"])
	}

	if feature.Properties["height_m"] != 12.0 {
		t.Fatalf("expected height 12, got %#v", feature.Properties["height_m"])
	}

	if feature.Geometry.Type != "Polygon" {
		t.Fatalf("expected polygon geometry, got %q", feature.Geometry.Type)
	}
}
