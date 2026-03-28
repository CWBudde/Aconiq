package talaerm

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildExportEnvelope_MixedResults(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	results := []ReceiverAssessment{
		{
			ReceiverID:       "R1",
			AreaCategory:     AreaCommercial,
			AreaCategoryCode: "b",
			Exceeds:          false,
			Irrelevanz: IrrelevanzResult{
				DayIrrelevant:   false,
				NightIrrelevant: false,
			},
		},
		{
			ReceiverID:       "R2",
			AreaCategory:     AreaResidential,
			AreaCategoryCode: "e",
			Exceeds:          true,
			Irrelevanz: IrrelevanzResult{
				DayIrrelevant:   false,
				NightIrrelevant: false,
			},
		},
		{
			ReceiverID:       "R3",
			AreaCategory:     AreaMixed,
			AreaCategoryCode: "d",
			Exceeds:          false,
			Irrelevanz: IrrelevanzResult{
				DayIrrelevant:   true,
				NightIrrelevant: true,
			},
		},
	}

	skipped := []SkippedReceiver{
		{ReceiverID: "R4", Reason: "missing area category"},
	}

	env := BuildExportEnvelope(results, skipped, "schall03-2014", now)

	if env.Regulation != "TA Lärm" {
		t.Errorf("Regulation = %q, want %q", env.Regulation, "TA Lärm")
	}

	if env.Edition != RegulationEdition {
		t.Errorf("Edition = %q, want %q", env.Edition, RegulationEdition)
	}

	if env.ReceiverCount != 4 {
		t.Errorf("ReceiverCount = %d, want 4", env.ReceiverCount)
	}

	if env.AssessedCount != 3 {
		t.Errorf("AssessedCount = %d, want 3", env.AssessedCount)
	}

	if env.ExceedingCount != 1 {
		t.Errorf("ExceedingCount = %d, want 1", env.ExceedingCount)
	}

	if env.IrrelevantCount != 1 {
		t.Errorf("IrrelevantCount = %d, want 1", env.IrrelevantCount)
	}

	if len(env.Results) != 3 {
		t.Errorf("len(Results) = %d, want 3", len(env.Results))
	}

	if len(env.Skipped) != 1 {
		t.Errorf("len(Skipped) = %d, want 1", len(env.Skipped))
	}

	// Verify category counts: b=1, e=1, d=1.
	wantCats := map[string]int{"b": 1, "e": 1, "d": 1}
	if len(env.CategoryCounts) != len(wantCats) {
		t.Fatalf("CategoryCounts length = %d, want %d", len(env.CategoryCounts), len(wantCats))
	}

	for k, v := range wantCats {
		if env.CategoryCounts[k] != v {
			t.Errorf("CategoryCounts[%q] = %d, want %d", k, env.CategoryCounts[k], v)
		}
	}
}

func TestBuildExportEnvelope_Empty(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	env := BuildExportEnvelope(nil, nil, "test-standard", now)

	if env.ReceiverCount != 0 {
		t.Errorf("ReceiverCount = %d, want 0", env.ReceiverCount)
	}

	if env.AssessedCount != 0 {
		t.Errorf("AssessedCount = %d, want 0", env.AssessedCount)
	}

	if env.ExceedingCount != 0 {
		t.Errorf("ExceedingCount = %d, want 0", env.ExceedingCount)
	}

	if env.IrrelevantCount != 0 {
		t.Errorf("IrrelevantCount = %d, want 0", env.IrrelevantCount)
	}
}

func TestBuildExportEnvelope_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 3, 20, 14, 0, 0, 0, time.UTC)

	results := []ReceiverAssessment{
		{
			ReceiverID:       "IO-1",
			AreaCategory:     AreaHealthcare,
			AreaCategoryCode: "g",
			Exceeds:          true,
			Irrelevanz: IrrelevanzResult{
				DayIrrelevant:   true,
				NightIrrelevant: true,
			},
		},
	}

	env := BuildExportEnvelope(results, nil, "iso9613-2", now)

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded ExportEnvelope

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if decoded.Regulation != env.Regulation {
		t.Errorf("Regulation round-trip: got %q, want %q", decoded.Regulation, env.Regulation)
	}

	if decoded.Edition != env.Edition {
		t.Errorf("Edition round-trip: got %q, want %q", decoded.Edition, env.Edition)
	}

	if !decoded.GeneratedAt.Equal(env.GeneratedAt) {
		t.Errorf("GeneratedAt round-trip: got %v, want %v", decoded.GeneratedAt, env.GeneratedAt)
	}

	if decoded.ReceiverCount != env.ReceiverCount {
		t.Errorf("ReceiverCount round-trip: got %d, want %d", decoded.ReceiverCount, env.ReceiverCount)
	}

	if decoded.ExceedingCount != env.ExceedingCount {
		t.Errorf("ExceedingCount round-trip: got %d, want %d", decoded.ExceedingCount, env.ExceedingCount)
	}

	if decoded.IrrelevantCount != env.IrrelevantCount {
		t.Errorf("IrrelevantCount round-trip: got %d, want %d", decoded.IrrelevantCount, env.IrrelevantCount)
	}

	if len(decoded.Results) != len(env.Results) {
		t.Errorf("Results length round-trip: got %d, want %d", len(decoded.Results), len(env.Results))
	}
}
