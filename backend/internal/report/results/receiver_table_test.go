package results

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReceiverTableSaveJSONAndCSV(t *testing.T) {
	t.Parallel()

	table := ReceiverTable{
		IndicatorOrder: []string{"Lden", "Lnight"},
		Unit:           "dB",
		Records: []ReceiverRecord{
			{ID: "r1", X: 10, Y: 20, HeightM: 4, Values: map[string]float64{"Lden": 55.2, "Lnight": 45.1}},
			{ID: "r2", X: 12, Y: 21, HeightM: 4, Values: map[string]float64{"Lden": 56.0, "Lnight": 46.4}},
		},
	}

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "receivers.json")
	csvPath := filepath.Join(dir, "receivers.csv")

	err := SaveReceiverTableJSON(jsonPath, table)
	if err != nil {
		t.Fatalf("save receiver json: %v", err)
	}

	err = SaveReceiverTableCSV(csvPath, table)
	if err != nil {
		t.Fatalf("save receiver csv: %v", err)
	}

	csvPayload, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}

	if !strings.Contains(string(csvPayload), "id,x,y,height_m,Lden,Lnight") {
		t.Fatalf("unexpected csv header: %s", string(csvPayload))
	}
}

func TestLoadReceiverTableJSON(t *testing.T) {
	t.Parallel()

	table := ReceiverTable{
		IndicatorOrder: []string{"Lden", "Lnight"},
		Unit:           "dB",
		Records: []ReceiverRecord{
			{ID: "r1", X: 10, Y: 20, HeightM: 4, Values: map[string]float64{"Lden": 55.2, "Lnight": 45.1}},
			{ID: "r2", X: 12, Y: 21, HeightM: 4, Values: map[string]float64{"Lden": 56.0, "Lnight": 46.4}},
		},
	}

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "receivers.json")

	err := SaveReceiverTableJSON(jsonPath, table)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadReceiverTableJSON(jsonPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(loaded.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(loaded.Records))
	}

	if loaded.Records[0].ID != "r1" {
		t.Fatalf("first record id = %q, want r1", loaded.Records[0].ID)
	}

	if loaded.Records[0].Values["Lden"] != 55.2 {
		t.Fatalf("r1 Lden = %v, want 55.2", loaded.Records[0].Values["Lden"])
	}

	if loaded.Unit != "dB" {
		t.Fatalf("unit = %q, want dB", loaded.Unit)
	}
}

func TestLoadReceiverTableJSONNotFound(t *testing.T) {
	t.Parallel()

	_, err := LoadReceiverTableJSON("/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReceiverTableValidation(t *testing.T) {
	t.Parallel()

	table := ReceiverTable{
		IndicatorOrder: []string{"Lden"},
		Records: []ReceiverRecord{
			{ID: "r1", X: 1, Y: 2, HeightM: 4, Values: map[string]float64{}},
		},
	}

	err := table.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}
