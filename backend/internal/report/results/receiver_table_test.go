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
