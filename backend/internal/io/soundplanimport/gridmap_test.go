package soundplanimport

import (
	"path/filepath"
	"testing"
)

func TestParseGridMapMetadata_Layers(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	meta, err := ParseGridMapMetadata(filepath.Join(dir, "RRLK0022", "RRLK0022.GM"))
	if err != nil {
		t.Fatalf("ParseGridMapMetadata: %v", err)
	}

	if meta.FileSizeBytes <= 0 {
		t.Fatal("expected positive GM file size")
	}

	if len(meta.Layers) != 3 {
		t.Fatalf("layer count = %d, want 3", len(meta.Layers))
	}

	if meta.Layers[0].Name != "Ground elevation" || meta.Layers[0].Unit != "m" {
		t.Fatalf("layer[0] = %+v, want Ground elevation|m", meta.Layers[0])
	}

	if meta.Layers[1].Name != "Tag" || meta.Layers[1].Unit != "dB(A)" {
		t.Fatalf("layer[1] = %+v, want Tag|dB(A)", meta.Layers[1])
	}

	if meta.Layers[2].Name != "Nacht" || meta.Layers[2].Unit != "dB(A)" {
		t.Fatalf("layer[2] = %+v, want Nacht|dB(A)", meta.Layers[2])
	}
}

func TestLoadGridMapMetadata(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	runs, err := ListRuns(dir)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}

	gridMaps := LoadGridMapMetadata(dir, runs)
	if len(gridMaps) != 4 {
		t.Fatalf("grid map count = %d, want 4", len(gridMaps))
	}

	for _, item := range gridMaps {
		if item.ResultSubFolder == "" {
			t.Fatal("expected result subfolder")
		}

		if item.PointsTotal != 5961 {
			t.Fatalf("%s points_total = %d, want 5961", item.ResultSubFolder, item.PointsTotal)
		}

		if len(item.AssessmentPeriods) != 2 {
			t.Fatalf("%s assessment period count = %d, want 2", item.ResultSubFolder, len(item.AssessmentPeriods))
		}

		if len(item.Layers) != 3 {
			t.Fatalf("%s layer count = %d, want 3", item.ResultSubFolder, len(item.Layers))
		}
	}
}
