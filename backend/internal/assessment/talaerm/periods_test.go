package talaerm

import (
	"fmt"
	"math"
	"testing"
)

func TestValidateTeilzeiten(t *testing.T) {
	t.Parallel()

	validDay16h := Teilzeit{DurationH: 16, LAeq: 55.0, KT: 0, KI: 0, KR: 0}
	validDay10h := Teilzeit{DurationH: 10, LAeq: 55.0, KT: 0, KI: 0, KR: 0}
	validDay6h := Teilzeit{DurationH: 6, LAeq: 50.0, KT: 3, KI: 0, KR: 0}
	validNight1h := Teilzeit{DurationH: 1, LAeq: 45.0, KT: 0, KI: 0, KR: 0}
	validNight8h := Teilzeit{DurationH: 8, LAeq: 42.0, KT: 0, KI: 0, KR: 0}

	tests := []struct {
		name       string
		period     AssessmentPeriod
		teilzeiten []Teilzeit
		wantErr    bool
	}{
		{
			name:       "valid single Teilzeit covering full day",
			period:     PeriodDay,
			teilzeiten: []Teilzeit{validDay16h},
		},
		{
			name:       "valid two Teilzeiten summing to 16h",
			period:     PeriodDay,
			teilzeiten: []Teilzeit{validDay10h, validDay6h},
		},
		{
			name:       "valid single Teilzeit for night 1h",
			period:     PeriodNight,
			teilzeiten: []Teilzeit{validNight1h},
		},
		{
			name:       "valid single Teilzeit for night 8h",
			period:     PeriodNight,
			teilzeiten: []Teilzeit{validNight8h},
		},
		{
			name:       "empty slice",
			period:     PeriodDay,
			teilzeiten: []Teilzeit{},
			wantErr:    true,
		},
		{
			name:   "DurationH zero",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 0, LAeq: 55.0, KT: 0, KI: 0, KR: 0},
			},
			wantErr: true,
		},
		{
			name:   "sum mismatch for day",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 10, LAeq: 55.0, KT: 0, KI: 0, KR: 0},
			},
			wantErr: true,
		},
		{
			name:   "NaN LAeq",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 16, LAeq: math.NaN(), KT: 0, KI: 0, KR: 0},
			},
			wantErr: true,
		},
		{
			name:   "KT invalid value 4",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 16, LAeq: 55.0, KT: 4, KI: 0, KR: 0},
			},
			wantErr: true,
		},
		{
			name:   "KI invalid value 2",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 16, LAeq: 55.0, KT: 0, KI: 2, KR: 0},
			},
			wantErr: true,
		},
		{
			name:   "KR invalid value 3",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 16, LAeq: 55.0, KT: 0, KI: 0, KR: 3},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTeilzeiten(tt.period, tt.teilzeiten)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTeilzeiten() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsErhoehtEmpfindlichkeitTime_Weekday(t *testing.T) {
	t.Parallel()

	tests := []struct {
		hour int
		want bool
	}{
		{6, true},
		{7, false},
		{10, false},
		{20, true},
		{21, true},
		{22, false},
		{0, false},
		{5, false},
		{12, false},
		{19, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("hour_%d", tt.hour), func(t *testing.T) {
			t.Parallel()

			got := IsErhoehtEmpfindlichkeitTime(tt.hour, true)
			if got != tt.want {
				t.Errorf("IsErhoehtEmpfindlichkeitTime(%d, weekday) = %v, want %v", tt.hour, got, tt.want)
			}
		})
	}
}

func TestIsErhoehtEmpfindlichkeitTime_Weekend(t *testing.T) {
	t.Parallel()

	tests := []struct {
		hour int
		want bool
	}{
		{6, true},
		{7, true},
		{8, true},
		{9, false},
		{10, false},
		{13, true},
		{14, true},
		{15, false},
		{20, true},
		{21, true},
		{22, false},
		{0, false},
		{12, false},
		{16, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("hour_%d", tt.hour), func(t *testing.T) {
			t.Parallel()

			got := IsErhoehtEmpfindlichkeitTime(tt.hour, false)
			if got != tt.want {
				t.Errorf("IsErhoehtEmpfindlichkeitTime(%d, weekend) = %v, want %v", tt.hour, got, tt.want)
			}
		})
	}
}

func TestNightAssessmentDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode string
		want float64
	}{
		{"full", 8.0},
		{"loudest_hour", 1.0},
		{"unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()

			got := NightAssessmentDuration(tt.mode)
			if got != tt.want {
				t.Errorf("NightAssessmentDuration(%q) = %g, want %g", tt.mode, got, tt.want)
			}
		})
	}
}
