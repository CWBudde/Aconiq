package bimschv16

import (
	"strings"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/report/results"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

func TestThresholdsForCategory(t *testing.T) {
	t.Parallel()

	cases := []struct {
		category AreaCategory
		day      int
		night    int
	}{
		{AreaHospitalsSchoolsCareHomes, 57, 47},
		{AreaResidential, 59, 49},
		{AreaMixed, 64, 54},
		{AreaCommercial, 69, 59},
	}

	for _, tc := range cases {
		got, err := ThresholdsForCategory(tc.category)
		if err != nil {
			t.Fatalf("thresholds for %s: %v", tc.category, err)
		}
		if got.Day != tc.day || got.Night != tc.night {
			t.Fatalf("%s: got %d/%d want %d/%d", tc.category, got.Day, got.Night, tc.day, tc.night)
		}
	}
}

func TestParseAreaCategory(t *testing.T) {
	t.Parallel()

	cases := map[string]AreaCategory{
		"allgemeines Wohngebiet": AreaResidential,
		"Kleinsiedlungsgebiet":   AreaResidential,
		"Mischgebiet":            AreaMixed,
		"Gewerbegebiet":          AreaCommercial,
		"Krankenhaus":            AreaHospitalsSchoolsCareHomes,
	}

	for raw, want := range cases {
		got, err := ParseAreaCategory(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		if got != want {
			t.Fatalf("parse %q: got %q want %q", raw, got, want)
		}
	}
}

func TestRoundForThresholdComparison(t *testing.T) {
	t.Parallel()

	if got := RoundForThresholdComparison(59.01); got != 60 {
		t.Fatalf("got %d want 60", got)
	}
	if got := RoundForThresholdComparison(59.0); got != 59 {
		t.Fatalf("got %d want 59", got)
	}
}

func TestAssessReceiverRoadOnly(t *testing.T) {
	t.Parallel()

	result, err := AssessReceiver(ReceiverInput{
		ReceiverID:   "r1",
		AreaCategory: AreaResidential,
		Road:         &PeriodLevels{Day: 58.2, Night: 48.0},
	})
	if err != nil {
		t.Fatalf("assess receiver: %v", err)
	}

	if result.EligibleForNoiseProtectionMeasures {
		t.Fatal("did not expect exceedance")
	}
	if result.Road == nil || result.Combined == nil {
		t.Fatal("expected road and combined assessments")
	}
	if result.Road.DayRounded != 59 || result.Road.NightRounded != 48 {
		t.Fatalf("unexpected rounded values: %+v", result.Road)
	}
	if !strings.Contains(result.SummaryDE, "Beurteilungspegel Straße 59/48 dB") {
		t.Fatalf("unexpected summary: %s", result.SummaryDE)
	}
}

func TestAssessReceiverCombinedRoadAndRail(t *testing.T) {
	t.Parallel()

	result, err := AssessReceiver(ReceiverInput{
		ReceiverID:   "r2",
		AreaCategory: AreaMixed,
		Road:         &PeriodLevels{Day: 61.0, Night: 51.0},
		Rail:         &PeriodLevels{Day: 61.0, Night: 51.0},
	})
	if err != nil {
		t.Fatalf("assess receiver: %v", err)
	}

	if result.Combined == nil {
		t.Fatal("expected combined assessment")
	}
	if result.Combined.DayRounded <= result.Road.DayRounded {
		t.Fatalf("expected combined day > single-source day: combined=%d road=%d", result.Combined.DayRounded, result.Road.DayRounded)
	}
	if !result.EligibleForNoiseProtectionMeasures {
		t.Fatal("expected exceedance-based eligibility")
	}
	if !strings.Contains(result.SummaryDE, "kombinierter Beurteilungspegel Straße + Schiene") {
		t.Fatalf("unexpected summary: %s", result.SummaryDE)
	}
}

func TestBuildExportEnvelope(t *testing.T) {
	t.Parallel()

	model := modelgeojson.Model{
		Features: []modelgeojson.Feature{
			{ID: "rx-1", Kind: "receiver", Properties: map[string]any{"bimschv16_area_category": "allgemeines Wohngebiet"}},
			{ID: "rx-2", Kind: "receiver", Properties: map[string]any{}},
		},
	}
	table := results.ReceiverTable{
		IndicatorOrder: []string{rls19road.IndicatorLrDay, rls19road.IndicatorLrNight},
		Unit:           "dB",
		Records: []results.ReceiverRecord{
			{ID: "rx-1", Values: map[string]float64{rls19road.IndicatorLrDay: 60.1, rls19road.IndicatorLrNight: 49.2}},
		},
	}

	envelope, err := BuildExportEnvelope(model, table, rls19road.StandardID, time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}

	if envelope.AssessedCount != 1 || envelope.ExceedingCount != 1 {
		t.Fatalf("unexpected counts: %+v", envelope)
	}
	if len(envelope.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(envelope.Results))
	}
	if len(envelope.Skipped) != 1 || envelope.Skipped[0].ReceiverID != "rx-2" {
		t.Fatalf("unexpected skipped list: %+v", envelope.Skipped)
	}
}
