package soundplanimport

import (
	"path/filepath"
	"testing"
)

func TestParseGridMapMetadata_Layers(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	meta, err := ParseGridMapMetadata(filepath.Join(dir, "RRLK0022", "RRLK0022.GM"), 5961)
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

	if !meta.DecodedValues {
		t.Fatal("expected decoded_values=true")
	}

	if meta.ActiveCellCount != 5961 {
		t.Fatalf("active_cell_count = %d, want 5961", meta.ActiveCellCount)
	}

	if meta.RowCount != 74 {
		t.Fatalf("row_count = %d, want 74", meta.RowCount)
	}

	if len(meta.RowCellCounts) != 74 {
		t.Fatalf("row_cell_counts len = %d, want 74", len(meta.RowCellCounts))
	}

	if meta.RowCellCounts[0] != 42 {
		t.Fatalf("row_cell_counts[0] = %d, want 42", meta.RowCellCounts[0])
	}

	if meta.RowCellCounts[len(meta.RowCellCounts)-1] != 6 {
		t.Fatalf("last row cell count = %d, want 6", meta.RowCellCounts[len(meta.RowCellCounts)-1])
	}

	if len(meta.ValueStats) != 3 {
		t.Fatalf("value_stats len = %d, want 3", len(meta.ValueStats))
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

		if !item.DecodedValues {
			t.Fatalf("%s expected decoded values", item.ResultSubFolder)
		}

		if item.ActiveCellCount != item.PointsTotal {
			t.Fatalf("%s active_cell_count = %d, want %d", item.ResultSubFolder, item.ActiveCellCount, item.PointsTotal)
		}
	}
}
