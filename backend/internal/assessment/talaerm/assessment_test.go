package talaerm

import (
	"strings"
	"testing"
)

func TestAssessReceiver_SimplePass(t *testing.T) {
	result, err := AssessReceiver(ReceiverInput{
		ReceiverID:      "IO1",
		AreaCategory:    AreaCommercial,
		Zusatzbelastung: PeriodLevels{Day: 55, Night: 40},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Exceeds {
		t.Error("expected Exceeds=false for levels well below thresholds")
	}

	if result.Richtwerte.Day != 65 || result.Richtwerte.Night != 50 {
		t.Errorf("unexpected Richtwerte: %+v", result.Richtwerte)
	}

	if result.ZusatzbelastungDay.LevelRounded != 55 {
		t.Errorf("expected day rounded=55, got %d", result.ZusatzbelastungDay.LevelRounded)
	}

	if result.ZusatzbelastungNight.LevelRounded != 40 {
		t.Errorf("expected night rounded=40, got %d", result.ZusatzbelastungNight.LevelRounded)
	}

	if result.AreaCategoryCode != "b" {
		t.Errorf("expected code=b, got %s", result.AreaCategoryCode)
	}
}

func TestAssessReceiver_SimpleFail(t *testing.T) {
	result, err := AssessReceiver(ReceiverInput{
		ReceiverID:      "IO2",
		AreaCategory:    AreaResidential,
		Zusatzbelastung: PeriodLevels{Day: 56, Night: 41},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Exceeds {
		t.Error("expected Exceeds=true")
	}

	if !result.ZusatzbelastungDay.Exceeds {
		t.Error("expected day exceeds")
	}

	if result.ZusatzbelastungDay.LevelRounded != 56 {
		t.Errorf("expected day rounded=56, got %d", result.ZusatzbelastungDay.LevelRounded)
	}

	if result.ZusatzbelastungDay.Richtwert != 55 {
		t.Errorf("expected day richtwert=55, got %d", result.ZusatzbelastungDay.Richtwert)
	}

	if result.ZusatzbelastungDay.ExceedanceDB != 1 {
		t.Errorf("expected day exceedance=1, got %d", result.ZusatzbelastungDay.ExceedanceDB)
	}

	if !result.ZusatzbelastungNight.Exceeds {
		t.Error("expected night exceeds")
	}

	if result.ZusatzbelastungNight.LevelRounded != 41 {
		t.Errorf("expected night rounded=41, got %d", result.ZusatzbelastungNight.LevelRounded)
	}
}

func TestAssessReceiver_Irrelevant(t *testing.T) {
	result, err := AssessReceiver(ReceiverInput{
		ReceiverID:      "IO3",
		AreaCategory:    AreaMixed,
		Zusatzbelastung: PeriodLevels{Day: 48, Night: 33},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Irrelevanz.DayIrrelevant {
		t.Error("expected day irrelevant (48 <= 60-6=54)")
	}

	if !result.Irrelevanz.NightIrrelevant {
		t.Error("expected night irrelevant (33 <= 45-6=39)")
	}

	if result.Exceeds {
		t.Error("expected Exceeds=false when both periods are irrelevant")
	}

	if !strings.Contains(result.SummaryDE, "Irrelevanzkriterium") {
		t.Error("expected summary to mention Irrelevanzkriterium")
	}
}

func TestAssessReceiver_Messabschlag(t *testing.T) {
	result, err := AssessReceiver(ReceiverInput{
		ReceiverID:         "IO4",
		AreaCategory:       AreaResidential,
		Zusatzbelastung:    PeriodLevels{Day: 55.5, Night: 40.5},
		IsMeasurementBased: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After -3 dB: effective day=52.5, night=37.5.
	// Richtwerte: 55/40. ceil(52.5)=53 <= 55, ceil(37.5)=38 <= 40.
	if result.Exceeds {
		t.Error("expected Exceeds=false after Messabschlag")
	}

	if result.ZusatzbelastungDay.LevelRounded != 53 {
		t.Errorf("expected day rounded=53 (ceil(52.5)), got %d", result.ZusatzbelastungDay.LevelRounded)
	}

	if result.ZusatzbelastungNight.LevelRounded != 38 {
		t.Errorf("expected night rounded=38 (ceil(37.5)), got %d", result.ZusatzbelastungNight.LevelRounded)
	}

	if !result.MeasurementDeduction {
		t.Error("expected MeasurementDeduction=true")
	}

	if !strings.Contains(result.SummaryDE, "Messabschlag") {
		t.Error("expected summary to mention Messabschlag")
	}
}

func TestAssessReceiver_Gemengelage(t *testing.T) {
	// Nr. 6.7: Gemengelage thresholds must not exceed Kern-/Dorf-/Mischgebiet (60/45).
	// Use 58/43 as effective values and levels that exceed them.
	result, err := AssessReceiver(ReceiverInput{
		ReceiverID:      "IO5",
		AreaCategory:    AreaMixed,
		Zusatzbelastung: PeriodLevels{Day: 59, Night: 44},
		Gemengelage:     &Gemengelage{EffectiveDay: 58, EffectiveNight: 43},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Richtwerte.Day != 58 || result.Richtwerte.Night != 43 {
		t.Errorf("expected Richtwerte 58/43 from Gemengelage, got %d/%d",
			result.Richtwerte.Day, result.Richtwerte.Night)
	}

	if !result.ZusatzbelastungDay.Exceeds {
		t.Error("expected day exceeds (ceil(59)=59 > 58)")
	}

	if !result.ZusatzbelastungNight.Exceeds {
		t.Error("expected night exceeds (ceil(44)=44 > 43)")
	}

	if !result.Exceeds {
		t.Error("expected Exceeds=true")
	}
}

func TestAssessReceiver_PeakExceedance(t *testing.T) {
	peakDay := 96.0

	result, err := AssessReceiver(ReceiverInput{
		ReceiverID:      "IO6",
		AreaCategory:    AreaCommercial,
		Zusatzbelastung: PeriodLevels{Day: 55, Night: 40},
		PeakDay:         &peakDay,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Peak limit day: 65+30=95. 96 > 95 → exceeds.
	if result.Peak == nil {
		t.Fatal("expected peak assessment to be present")
	}

	if result.Peak.LimitDay != 95 {
		t.Errorf("expected peak limit day=95, got %d", result.Peak.LimitDay)
	}

	if !result.Peak.DayExceeds {
		t.Error("expected peak day exceeds (96 > 95)")
	}

	if !result.Exceeds {
		t.Error("expected Exceeds=true (peak fails even though Lr passes)")
	}
}

func TestAssessReceiver_IrrelevantButPeakFails(t *testing.T) {
	peakDay := 91.0

	result, err := AssessReceiver(ReceiverInput{
		ReceiverID:      "IO7",
		AreaCategory:    AreaMixed,
		Zusatzbelastung: PeriodLevels{Day: 48, Night: 33},
		PeakDay:         &peakDay,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Irrelevanz.DayIrrelevant || !result.Irrelevanz.NightIrrelevant {
		t.Error("expected both periods to be irrelevant")
	}

	// Peak limit day: 60+30=90. 91 > 90 → exceeds.
	if result.Peak == nil {
		t.Fatal("expected peak assessment")
	}

	if !result.Peak.DayExceeds {
		t.Error("expected peak day exceeds (91 > 90)")
	}

	if !result.Exceeds {
		t.Error("expected Exceeds=true (peak is independent of irrelevance)")
	}
}

func TestAssessReceiver_AllCategories(t *testing.T) {
	tests := []struct {
		category AreaCategory
		code     string
		day      int
		night    int
	}{
		{AreaIndustrial, "a", 70, 70},
		{AreaCommercial, "b", 65, 50},
		{AreaUrban, "c", 63, 45},
		{AreaMixed, "d", 60, 45},
		{AreaResidential, "e", 55, 40},
		{AreaPureResidential, "f", 50, 35},
		{AreaHealthcare, "g", 45, 35},
	}

	for _, tc := range tests {
		t.Run(string(tc.category), func(t *testing.T) {
			result, err := AssessReceiver(ReceiverInput{
				ReceiverID:      "IO-" + tc.code,
				AreaCategory:    tc.category,
				Zusatzbelastung: PeriodLevels{Day: 30, Night: 20},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.AreaCategoryCode != tc.code {
				t.Errorf("expected code=%s, got %s", tc.code, result.AreaCategoryCode)
			}

			if result.Richtwerte.Day != tc.day || result.Richtwerte.Night != tc.night {
				t.Errorf("expected Richtwerte %d/%d, got %d/%d",
					tc.day, tc.night, result.Richtwerte.Day, result.Richtwerte.Night)
			}

			if result.Exceeds {
				t.Error("expected Exceeds=false for very low levels")
			}
		})
	}
}

func TestAssessReceiver_EmptyReceiverID(t *testing.T) {
	_, err := AssessReceiver(ReceiverInput{
		ReceiverID:      "",
		AreaCategory:    AreaCommercial,
		Zusatzbelastung: PeriodLevels{Day: 50, Night: 40},
	})
	if err == nil {
		t.Fatal("expected error for empty receiver ID")
	}

	if !strings.Contains(err.Error(), "receiver id") {
		t.Errorf("expected error about receiver id, got: %v", err)
	}
}

func TestAssessReceiver_InvalidCategory(t *testing.T) {
	_, err := AssessReceiver(ReceiverInput{
		ReceiverID:      "IO-invalid",
		AreaCategory:    AreaCategory("nonsense"),
		Zusatzbelastung: PeriodLevels{Day: 50, Night: 40},
	})
	if err == nil {
		t.Fatal("expected error for invalid category")
	}

	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected error about unsupported category, got: %v", err)
	}
}

func TestAssessReceiver_GemengelageExceedsMax(t *testing.T) {
	tests := []struct {
		name string
		gem  Gemengelage
	}{
		{
			name: "day exceeds",
			gem:  Gemengelage{EffectiveDay: 61, EffectiveNight: 45},
		},
		{
			name: "night exceeds",
			gem:  Gemengelage{EffectiveDay: 60, EffectiveNight: 46},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AssessReceiver(ReceiverInput{
				ReceiverID:      "IO-gem",
				AreaCategory:    AreaMixed,
				Zusatzbelastung: PeriodLevels{Day: 50, Night: 40},
				Gemengelage:     &tc.gem,
			})
			if err == nil {
				t.Fatal("expected error for Gemengelage exceeding maximum")
			}

			if !strings.Contains(err.Error(), "gemengelage") {
				t.Errorf("expected error about gemengelage, got: %v", err)
			}
		})
	}
}

func TestAssessReceiver_Vorbelastung(t *testing.T) {
	result, err := AssessReceiver(ReceiverInput{
		ReceiverID:      "IO-vb",
		AreaCategory:    AreaCommercial,
		Zusatzbelastung: PeriodLevels{Day: 55, Night: 40},
		Vorbelastung:    &PeriodLevels{Day: 60, Night: 45},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Gesamtbelastung == nil {
		t.Fatal("expected Gesamtbelastung to be computed")
	}

	// Gesamtbelastung should be > max(Vorbelastung, Zusatzbelastung).
	if result.Gesamtbelastung.Day <= 60 {
		t.Errorf("expected Gesamtbelastung day > 60, got %.1f", result.Gesamtbelastung.Day)
	}

	if !strings.Contains(result.SummaryDE, "Gesamtbelastung") {
		t.Error("expected summary to mention Gesamtbelastung")
	}
}

func TestAssessReceiver_SummaryContainsVerdict(t *testing.T) {
	result, err := AssessReceiver(ReceiverInput{
		ReceiverID:      "IO-sum",
		AreaCategory:    AreaResidential,
		Zusatzbelastung: PeriodLevels{Day: 50, Night: 35},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.SummaryDE, "eingehalten") {
		t.Error("expected summary to contain 'eingehalten' for passing receiver")
	}

	if strings.Contains(result.SummaryDE, "überschritten") {
		t.Error("summary should not contain 'überschritten' for passing receiver")
	}
}

func TestAssessReceiver_CeilRounding(t *testing.T) {
	// Test that fractional levels are rounded up.
	result, err := AssessReceiver(ReceiverInput{
		ReceiverID:      "IO-ceil",
		AreaCategory:    AreaResidential,
		Zusatzbelastung: PeriodLevels{Day: 54.1, Night: 39.1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ceil(54.1) = 55, ceil(39.1) = 40. Both equal Richtwert, not exceeding.
	if result.ZusatzbelastungDay.LevelRounded != 55 {
		t.Errorf("expected day rounded=55 (ceil(54.1)), got %d", result.ZusatzbelastungDay.LevelRounded)
	}

	if result.ZusatzbelastungNight.LevelRounded != 40 {
		t.Errorf("expected night rounded=40 (ceil(39.1)), got %d", result.ZusatzbelastungNight.LevelRounded)
	}

	// 55 <= 55 and 40 <= 40: not exceeding.
	if result.Exceeds {
		t.Error("expected Exceeds=false (rounded equals richtwert, not exceeding)")
	}
}
