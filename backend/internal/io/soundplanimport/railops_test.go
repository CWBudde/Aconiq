package soundplanimport

import (
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/standards/schall03"
)

func TestLoadRailOperationSummaries(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	proj, err := ParseProjectFile(filepath.Join(dir, "Project.sp"))
	if err != nil {
		t.Fatalf("ParseProjectFile: %v", err)
	}

	runs, err := ListRuns(dir)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}

	summaries, resultDir, err := LoadRailOperationSummaries(dir, proj, runs)
	if err != nil {
		t.Fatalf("LoadRailOperationSummaries: %v", err)
	}

	if filepath.Base(resultDir) != "RSPS0011" {
		t.Fatalf("selected result dir = %q, want RSPS0011", filepath.Base(resultDir))
	}

	if len(summaries) != 5 {
		t.Fatalf("got %d summaries, want 5", len(summaries))
	}

	found := false
	for _, summary := range summaries {
		if summary.Railname != "Hauptstrecke Gleis 1" {
			continue
		}

		found = true

		if summary.TrafficDayPH <= 0 {
			t.Fatalf("TrafficDayPH = %f, want > 0", summary.TrafficDayPH)
		}

		if summary.TrafficNightPH <= 0 {
			t.Fatalf("TrafficNightPH = %f, want > 0", summary.TrafficNightPH)
		}

		if summary.AverageSpeedKPH <= 0 {
			t.Fatalf("AverageSpeedKPH = %f, want > 0", summary.AverageSpeedKPH)
		}

		if summary.TrainClass != schall03.TrainClassMixed && summary.TrainClass != schall03.TrainClassPassenger && summary.TrainClass != schall03.TrainClassFreight {
			t.Fatalf("unexpected train class %q", summary.TrainClass)
		}

		if summary.TractionType != schall03.TractionMixed && summary.TractionType != schall03.TractionElectric && summary.TractionType != schall03.TractionDiesel {
			t.Fatalf("unexpected traction type %q", summary.TractionType)
		}

		if summary.DominantTrainName == "" {
			t.Fatal("expected DominantTrainName")
		}
	}

	if !found {
		t.Fatal("expected summary for Hauptstrecke Gleis 1")
	}
}

func TestClassifyTrainClassAndTractionType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		trainType    TrainType
		wantClass    string
		wantTraction string
	}{
		{
			name:         "ICE  (v<=250)",
			trainType:    TrainType{ZugArt: 8, Name: "ICE  (v<=250)"},
			wantClass:    schall03.TrainClassPassenger,
			wantTraction: schall03.TractionElectric,
		},
		{
			name:         "Güterzug (Fernv.)",
			trainType:    TrainType{ZugArt: 6, Name: "Güterzug (Fernv.)"},
			wantClass:    schall03.TrainClassFreight,
			wantTraction: schall03.TractionMixed,
		},
		{
			name:         "Straßenbahn",
			trainType:    TrainType{ZugArt: 17, Name: "Straßenbahn"},
			wantClass:    schall03.TrainClassPassenger,
			wantTraction: schall03.TractionElectric,
		},
		{
			name:         "IC-Zug-V-Lok",
			trainType:    TrainType{Name: "IC-Zug-V-Lok"},
			wantClass:    schall03.TrainClassPassenger,
			wantTraction: schall03.TractionDiesel,
		},
	}

	for _, tt := range tests {
		if got := classifyTrainClass(tt.name, tt.trainType); got != tt.wantClass {
			t.Errorf("classifyTrainClass(%q, %+v) = %q, want %q", tt.name, tt.trainType, got, tt.wantClass)
		}

		if got := classifyTractionType(tt.name, tt.trainType); got != tt.wantTraction {
			t.Errorf("classifyTractionType(%q, %+v) = %q, want %q", tt.name, tt.trainType, got, tt.wantTraction)
		}
	}
}
