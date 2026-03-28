package talaerm

import (
	"math"
	"testing"
)

func TestComputeLr(t *testing.T) {
	const tolerance = 0.01

	tests := []struct {
		name       string
		period     AssessmentPeriod
		teilzeiten []Teilzeit
		cmet       float64
		wantLr     float64
		wantErr    bool
	}{
		{
			name:   "single Teilzeit full day no surcharges",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 16, LAeq: 60, KT: 0, KI: 0, KR: 0},
			},
			cmet:   0,
			wantLr: 60.0,
		},
		{
			name:   "single Teilzeit full day with Cmet",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 16, LAeq: 60, KT: 0, KI: 0, KR: 0},
			},
			cmet:   2.0,
			wantLr: 58.0,
		},
		{
			name:   "single Teilzeit with all surcharges",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 16, LAeq: 50, KT: 3, KI: 6, KR: 6},
			},
			cmet:   0,
			wantLr: 65.0,
		},
		{
			name:   "two equal Teilzeiten same levels",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 8, LAeq: 60, KT: 0, KI: 0, KR: 0},
				{DurationH: 8, LAeq: 60, KT: 0, KI: 0, KR: 0},
			},
			cmet:   0,
			wantLr: 60.0,
		},
		{
			name:   "two Teilzeiten different levels",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 8, LAeq: 70, KT: 0, KI: 0, KR: 0},
				{DurationH: 8, LAeq: 60, KT: 0, KI: 0, KR: 0},
			},
			cmet:   0,
			wantLr: 67.40,
		},
		{
			name:   "night loudest hour",
			period: PeriodNight,
			teilzeiten: []Teilzeit{
				{DurationH: 1, LAeq: 55, KT: 0, KI: 0, KR: 0},
			},
			cmet:   0,
			wantLr: 55.0,
		},
		{
			name:   "night full 8h",
			period: PeriodNight,
			teilzeiten: []Teilzeit{
				{DurationH: 8, LAeq: 55, KT: 0, KI: 0, KR: 0},
			},
			cmet:   0,
			wantLr: 55.0,
		},
		{
			name:   "Teilzeit with KR surcharge in one sub-period",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 1, LAeq: 50, KT: 0, KI: 0, KR: 6},
				{DurationH: 15, LAeq: 50, KT: 0, KI: 0, KR: 0},
			},
			cmet:   0,
			wantLr: 50.74,
		},
		{
			name:   "validation error sum mismatch",
			period: PeriodDay,
			teilzeiten: []Teilzeit{
				{DurationH: 10, LAeq: 60},
			},
			cmet:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeLr(tt.period, tt.teilzeiten, tt.cmet)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if math.Abs(got-tt.wantLr) > tolerance {
				t.Errorf("ComputeLr = %g, want %g (tolerance %g)", got, tt.wantLr, tolerance)
			}
		})
	}
}

func TestComputeLrSimple(t *testing.T) {
	got := ComputeLrSimple(60, 2, 3, 0, 6)
	want := 67.0

	if got != want {
		t.Errorf("ComputeLrSimple = %g, want %g", got, want)
	}
}

func TestComputeImpulsSurcharge(t *testing.T) {
	tests := []struct {
		name   string
		lAFTeq float64
		lAeq   float64
		wantKI float64
	}{
		{name: "diff 3 gives KI=3", lAFTeq: 63, lAeq: 60, wantKI: 3},
		{name: "diff 0 gives KI=0", lAFTeq: 60, lAeq: 60, wantKI: 0},
		{name: "diff 7 gives KI=6", lAFTeq: 67, lAeq: 60, wantKI: 6},
		{name: "diff 1.4 gives KI=0", lAFTeq: 61.4, lAeq: 60, wantKI: 0},
		{name: "diff 4.5 gives KI=6", lAFTeq: 64.5, lAeq: 60, wantKI: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeImpulsSurcharge(tt.lAFTeq, tt.lAeq)
			if got != tt.wantKI {
				t.Errorf("ComputeImpulsSurcharge(%g, %g) = %g, want %g", tt.lAFTeq, tt.lAeq, got, tt.wantKI)
			}
		})
	}
}
