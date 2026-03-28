package talaerm_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/assessment/talaerm"
	"github.com/aconiq/backend/internal/qa/golden"
)

func TestGoldenFullAssessment(t *testing.T) {
	peakDay81 := 81.0

	inputs := []talaerm.ReceiverInput{
		{
			// 1. Industrial pass: thresholds 70/70, zusatz 65/65 → passes.
			ReceiverID:   "IO-01",
			AreaCategory: talaerm.AreaIndustrial,
			Zusatzbelastung: talaerm.PeriodLevels{
				Day:   65,
				Night: 65,
			},
		},
		{
			// 2. Commercial fail: thresholds 65/50, zusatz 66/51 → exceeds both.
			ReceiverID:   "IO-02",
			AreaCategory: talaerm.AreaCommercial,
			Zusatzbelastung: talaerm.PeriodLevels{
				Day:   66,
				Night: 51,
			},
		},
		{
			// 3. Urban irrelevant: thresholds 63/45, zusatz 50/32 →
			// irrelevant (50<=57, 32<=39).
			ReceiverID:   "IO-03",
			AreaCategory: talaerm.AreaUrban,
			Zusatzbelastung: talaerm.PeriodLevels{
				Day:   50,
				Night: 32,
			},
		},
		{
			// 4. Mixed with Vorbelastung: thresholds 60/45,
			// zusatz 52/38, vorbelastung 58/43.
			ReceiverID:   "IO-04",
			AreaCategory: talaerm.AreaMixed,
			Zusatzbelastung: talaerm.PeriodLevels{
				Day:   52,
				Night: 38,
			},
			Vorbelastung: &talaerm.PeriodLevels{
				Day:   58,
				Night: 43,
			},
		},
		{
			// 5. Residential with Messabschlag: thresholds 55/40,
			// zusatz 56/41, IsMeasurementBased=true → after -3: 53/38, passes.
			ReceiverID:   "IO-05",
			AreaCategory: talaerm.AreaResidential,
			Zusatzbelastung: talaerm.PeriodLevels{
				Day:   56,
				Night: 41,
			},
			IsMeasurementBased: true,
		},
		{
			// 6. Pure residential peak fail: thresholds 50/35,
			// zusatz 40/25 (passes Lr), PeakDay=81.0 (limit=80), peak fails.
			ReceiverID:   "IO-06",
			AreaCategory: talaerm.AreaPureResidential,
			Zusatzbelastung: talaerm.PeriodLevels{
				Day:   40,
				Night: 25,
			},
			PeakDay: &peakDay81,
		},
		{
			// 7. Healthcare pass: thresholds 45/35, zusatz 40/30 → passes.
			ReceiverID:   "IO-07",
			AreaCategory: talaerm.AreaHealthcare,
			Zusatzbelastung: talaerm.PeriodLevels{
				Day:   40,
				Night: 30,
			},
		},
		{
			// 8. Gemengelage fail: custom thresholds 58/43,
			// zusatz 59/44 → exceeds both.
			ReceiverID:   "IO-08",
			AreaCategory: talaerm.AreaCommercial,
			Zusatzbelastung: talaerm.PeriodLevels{
				Day:   59,
				Night: 44,
			},
			Gemengelage: &talaerm.Gemengelage{
				EffectiveDay:   58,
				EffectiveNight: 43,
			},
		},
	}

	results := make([]talaerm.ReceiverAssessment, 0, len(inputs))

	for _, input := range inputs {
		result, err := talaerm.AssessReceiver(input)
		if err != nil {
			t.Fatalf("AssessReceiver(%s): %v", input.ReceiverID, err)
		}

		results = append(results, result)
	}

	skipped := []talaerm.SkippedReceiver{
		{ReceiverID: "IO-SKIP", Reason: "no area category assigned"},
	}

	generatedAt := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)

	envelope := talaerm.BuildExportEnvelope(results, skipped, "iso9613-2", generatedAt)

	// Verify JSON round-trip produces deterministic output.
	gotJSON, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	snapshotPath := filepath.Join("testdata", "full_assessment.golden.json")
	golden.AssertJSONSnapshot(t, snapshotPath, json.RawMessage(gotJSON))
}
