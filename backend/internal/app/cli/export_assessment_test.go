package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/report/results"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

func TestMaybeBuild16BImSchVAssessment(t *testing.T) {
	t.Parallel()

	bundleDir := t.TempDir()
	modelPath := filepath.Join(bundleDir, "model.normalized.geojson")
	receiverPath := filepath.Join(bundleDir, "receivers.json")

	modelGeoJSON := `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {
        "id": "rx-1",
        "kind": "receiver",
        "height_m": 4,
        "bimschv16_area_category": "allgemeines Wohngebiet"
      },
      "geometry": { "type": "Point", "coordinates": [100, 200] }
    }
  ],
  "crs": { "type": "name", "properties": { "name": "EPSG:25832" } }
}`
	if err := os.WriteFile(modelPath, []byte(modelGeoJSON), 0o600); err != nil {
		t.Fatalf("write model: %v", err)
	}

	table := results.ReceiverTable{
		IndicatorOrder: []string{rls19road.IndicatorLrDay, rls19road.IndicatorLrNight},
		Unit:           "dB",
		Records: []results.ReceiverRecord{
			{ID: "rx-1", X: 100, Y: 200, HeightM: 4, Values: map[string]float64{
				rls19road.IndicatorLrDay:   61.1,
				rls19road.IndicatorLrNight: 50.0,
			}},
		},
	}
	if err := results.SaveReceiverTableJSON(receiverPath, table); err != nil {
		t.Fatalf("write receiver table: %v", err)
	}

	assessmentPath, built, err := maybeBuild16BImSchVAssessment(bundleDir, modelPath, receiverPath, "EPSG:25832", rls19road.StandardID, time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("build assessment: %v", err)
	}

	if !built {
		t.Fatal("expected assessment to be built")
	}

	payload, err := os.ReadFile(assessmentPath)
	if err != nil {
		t.Fatalf("read assessment: %v", err)
	}

	text := string(payload)
	if !strings.Contains(text, `"law": "16. BImSchV"`) || !strings.Contains(text, `"receiver_id": "rx-1"`) {
		t.Fatalf("unexpected assessment payload: %s", text)
	}
}
