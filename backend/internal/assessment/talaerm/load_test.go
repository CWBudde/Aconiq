package talaerm

import (
	"math"
	"testing"
)

func TestComputeGesamtbelastung(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		vorbelastung    float64
		zusatzbelastung float64
		want            float64
	}{
		{name: "equal levels", vorbelastung: 50, zusatzbelastung: 50, want: 53.01},
		{name: "one dominant", vorbelastung: 60, zusatzbelastung: 50, want: 60.41},
		{name: "large difference", vorbelastung: 70, zusatzbelastung: 40, want: 70.00},
		{name: "symmetric 55", vorbelastung: 55, zusatzbelastung: 55, want: 58.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ComputeGesamtbelastung(tt.vorbelastung, tt.zusatzbelastung)
			if diff := math.Abs(got - tt.want); diff > 0.01 {
				t.Errorf("ComputeGesamtbelastung(%v, %v) = %v, want %v (diff %v)", tt.vorbelastung, tt.zusatzbelastung, got, tt.want, diff)
			}
		})
	}
}

func TestIsIrrelevant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		zusatzbelastung float64
		richtwert       int
		want            bool
	}{
		{name: "irrelevant day", zusatzbelastung: 49, richtwert: 55, want: true},
		{name: "not irrelevant day", zusatzbelastung: 50, richtwert: 55, want: false},
		{name: "exact boundary", zusatzbelastung: 49.0, richtwert: 55, want: true},
		{name: "large margin", zusatzbelastung: 40, richtwert: 55, want: true},
		{name: "irrelevant night", zusatzbelastung: 29, richtwert: 35, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := IsIrrelevant(tt.zusatzbelastung, tt.richtwert)
			if got != tt.want {
				t.Errorf("IsIrrelevant(%v, %v) = %v, want %v", tt.zusatzbelastung, tt.richtwert, got, tt.want)
			}
		})
	}
}

func TestCheckIrrelevanz(t *testing.T) {
	t.Parallel()

	t.Run("exact boundary", func(t *testing.T) {
		t.Parallel()

		result := CheckIrrelevanz(49.0, 29.0, Thresholds{Day: 55, Night: 35})
		if !result.DayIrrelevant {
			t.Error("expected day irrelevant")
		}

		if !result.NightIrrelevant {
			t.Error("expected night irrelevant")
		}

		if diff := math.Abs(result.DayMarginDB - 0.0); diff > 0.001 {
			t.Errorf("DayMarginDB = %v, want 0.0", result.DayMarginDB)
		}

		if diff := math.Abs(result.NightMarginDB - 0.0); diff > 0.001 {
			t.Errorf("NightMarginDB = %v, want 0.0", result.NightMarginDB)
		}
	})

	t.Run("large margin", func(t *testing.T) {
		t.Parallel()

		result := CheckIrrelevanz(40.0, 25.0, Thresholds{Day: 55, Night: 40})
		if !result.DayIrrelevant {
			t.Error("expected day irrelevant")
		}

		if diff := math.Abs(result.DayMarginDB - 9.0); diff > 0.001 {
			t.Errorf("DayMarginDB = %v, want 9.0", result.DayMarginDB)
		}
	})
}

func TestAssessLoad(t *testing.T) {
	t.Parallel()

	t.Run("with vorbelastung", func(t *testing.T) {
		t.Parallel()

		vDay := 55.0
		vNight := 40.0

		result, err := AssessLoad(LoadInput{
			ZusatzbelastungDay:   50,
			ZusatzbelastungNight: 35,
			VorbelastungDay:      &vDay,
			VorbelastungNight:    &vNight,
		}, Thresholds{Day: 55, Night: 40})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.GesamtbelastungDay == nil {
			t.Fatal("expected gesamtbelastung day to be computed")
		}

		if diff := math.Abs(*result.GesamtbelastungDay - 56.19); diff > 0.01 {
			t.Errorf("GesamtbelastungDay = %v, want ~56.19", *result.GesamtbelastungDay)
		}

		if result.GesamtbelastungNight == nil {
			t.Fatal("expected gesamtbelastung night to be computed")
		}

		if diff := math.Abs(*result.GesamtbelastungNight - 41.19); diff > 0.01 {
			t.Errorf("GesamtbelastungNight = %v, want ~41.19", *result.GesamtbelastungNight)
		}

		// zusatzDay=50 > 55-6=49, so NOT irrelevant
		if result.Irrelevanz.DayIrrelevant {
			t.Error("day should NOT be irrelevant (50 > 49)")
		}

		// zusatzNight=35 > 40-6=34, so NOT irrelevant
		if result.Irrelevanz.NightIrrelevant {
			t.Error("night should NOT be irrelevant (35 > 34)")
		}
	})

	t.Run("without vorbelastung", func(t *testing.T) {
		t.Parallel()

		result, err := AssessLoad(LoadInput{
			ZusatzbelastungDay:   48,
			ZusatzbelastungNight: 33,
			VorbelastungDay:      nil,
			VorbelastungNight:    nil,
		}, Thresholds{Day: 55, Night: 40})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.GesamtbelastungDay != nil {
			t.Error("expected gesamtbelastung day to be nil when vorbelastung is nil")
		}

		if result.GesamtbelastungNight != nil {
			t.Error("expected gesamtbelastung night to be nil when vorbelastung is nil")
		}

		// 48 <= 55-6=49, irrelevant
		if !result.Irrelevanz.DayIrrelevant {
			t.Error("day should be irrelevant (48 <= 49)")
		}

		// 33 <= 40-6=34, irrelevant
		if !result.Irrelevanz.NightIrrelevant {
			t.Error("night should be irrelevant (33 <= 34)")
		}
	})

	t.Run("NaN input returns error", func(t *testing.T) {
		t.Parallel()

		_, err := AssessLoad(LoadInput{
			ZusatzbelastungDay:   math.NaN(),
			ZusatzbelastungNight: 35,
		}, Thresholds{Day: 55, Night: 40})
		if err == nil {
			t.Error("expected error for NaN input")
		}
	})
}
