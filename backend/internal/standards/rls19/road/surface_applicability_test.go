package road

import (
	"strings"
	"testing"
)

// Tests for RoadSource.validateSurfaceApplicability: the refusal of a
// surface/speed pairing for which Tabelle 4a grants no correction.
//
// These live apart from road_test.go because that file is close to revive's
// file-length-limit, and because the surface/speed applicability rule is a
// topic of its own — as parking_test.go and standarddata_test.go already are.

// outOfBandSource returns a source whose OPA surface has no Tabelle 4a cell at
// the 50 km/h every group runs at.
func outOfBandSource() RoadSource {
	source := sampleSource()
	source.SurfaceType = SurfaceOPA
	source.Speeds = SpeedInput{PkwKPH: 50, Lkw1KPH: 50, Lkw2KPH: 50, KradKPH: 50}

	return source
}

func TestRoadSourceValidate_SurfaceOutOfBand(t *testing.T) {
	t.Parallel()

	err := outOfBandSource().Validate()
	if err == nil {
		t.Fatal("expected an error for OPA at 50 km/h")
	}

	// The message is the deliverable here: the existing Validate tests assert
	// only that an error occurred, which would not catch a refusal that fails
	// to tell the modeller which field to change.
	for _, want := range []string{`road source "road-1"`, `surface_type "OPA"`, "Pkw", "pkw_kph", "50", "above 60 km/h"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// TestRoadSourceValidate_SurfaceOutOfBandExemptsZeroTrafficGroup pins the
// exemption that keeps computable models computable: emissionForPeriod skips a
// group with no traffic, so no cell is ever read for it.
func TestRoadSourceValidate_SurfaceOutOfBandExemptsZeroTrafficGroup(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.SurfaceType = SurfaceConcrete // Lkw cell is crossed out at or below 60
	source.Speeds = SpeedInput{PkwKPH: 100, Lkw1KPH: 40, Lkw2KPH: 40, KradKPH: 100}
	source.TrafficDay.Lkw1PerHour = 0
	source.TrafficDay.Lkw2PerHour = 0
	source.TrafficNight.Lkw1PerHour = 0
	source.TrafficNight.Lkw2PerHour = 0

	if err := source.Validate(); err != nil {
		t.Fatalf("a group with no traffic must not be refused: %v", err)
	}
}

// TestRoadSourceValidate_SurfaceOutOfBandCatchesNightOnlyTraffic proves the
// exemption is day-or-night, not day-only: a group that runs only at night
// still reaches SurfaceCorrection in the night period.
func TestRoadSourceValidate_SurfaceOutOfBandCatchesNightOnlyTraffic(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.SurfaceType = SurfaceConcrete
	source.Speeds = SpeedInput{PkwKPH: 100, Lkw1KPH: 40, Lkw2KPH: 40, KradKPH: 100}
	source.TrafficDay.Lkw1PerHour = 0
	source.TrafficDay.Lkw2PerHour = 0
	source.TrafficNight.Lkw1PerHour = 1
	source.TrafficNight.Lkw2PerHour = 0

	err := source.Validate()
	if err == nil {
		t.Fatal("expected an error for a group carrying night-only traffic")
	}

	if !strings.Contains(err.Error(), "Lkw1") {
		t.Errorf("error %q should name Lkw1", err)
	}
}

// TestRoadSourceValidate_SurfaceOutOfBandExemptsKrad isolates the §3.3.3
// Anmerkung carve-out: Kräder run at the Pkw speed, where OPA has no cell, but
// carry D_SD = 0 by prescription and so read no cell at all.
func TestRoadSourceValidate_SurfaceOutOfBandExemptsKrad(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.SurfaceType = SurfaceOPA
	source.Speeds = SpeedInput{PkwKPH: 50, Lkw1KPH: 100, Lkw2KPH: 100, KradKPH: 50}
	source.TrafficDay = TrafficInput{PkwPerHour: 0, Lkw1PerHour: 40, Lkw2PerHour: 60, KradPerHour: 10}
	source.TrafficNight = TrafficInput{PkwPerHour: 0, Lkw1PerHour: 10, Lkw2PerHour: 20, KradPerHour: 2}

	if err := source.Validate(); err != nil {
		t.Fatalf("Kraeder must not be refused on a crossed-out cell: %v", err)
	}
}

func TestRoadSourceValidate_SurfaceNotSpecifiedIsSilent(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.SurfaceType = SurfaceNotSpecified
	source.Speeds = SpeedInput{PkwKPH: 50, Lkw1KPH: 50, Lkw2KPH: 50, KradKPH: 50}

	if err := source.Validate(); err != nil {
		t.Fatalf("an unstated surface is the zero-correction reference, not a pairing: %v", err)
	}
}

// TestRoadSourceValidate_DefinedZeroIsSilent is the counterpart the whole
// change turns on: "Nicht geriffelter Gussasphalt" is zero in all four cells,
// and a defined zero must never be confused with a crossed-out one.
func TestRoadSourceValidate_DefinedZeroIsSilent(t *testing.T) {
	t.Parallel()

	for _, speed := range []float64{50, 100} {
		source := sampleSource()
		source.SurfaceType = SurfaceGussasphaltStandard
		source.Speeds = SpeedInput{PkwKPH: speed, Lkw1KPH: speed, Lkw2KPH: speed, KradKPH: speed}

		if err := source.Validate(); err != nil {
			t.Fatalf("the all-zeros row must be accepted at %g km/h: %v", speed, err)
		}
	}
}

// TestRoadSourceValidate_PavingAndLegacyNeverOutOfBand covers the two entry
// shapes that carry no NaN: Tabelle 4b is indexed by speed alone, and the
// legacy damaged-surface row is per vehicle group.
func TestRoadSourceValidate_PavingAndLegacyNeverOutOfBand(t *testing.T) {
	t.Parallel()

	surfaces := []SurfaceType{SurfacePaving, SurfacePavingEven, SurfacePavingOther, SurfaceUnpavedOrDamaged}

	for _, surface := range surfaces {
		for _, speed := range []float64{30, 40, 50, 100} {
			source := sampleSource()
			source.SurfaceType = surface
			source.Speeds = SpeedInput{PkwKPH: speed, Lkw1KPH: speed, Lkw2KPH: speed, KradKPH: speed}

			if err := source.Validate(); err != nil {
				t.Errorf("%q at %g km/h must be accepted: %v", surface, speed, err)
			}
		}
	}
}

// TestRoadSourceValidate_SurfaceBandReportsTheClamp pins that the declared
// speed leads and the clamped value is named only when it differs. The clamp
// cannot change the verdict — every group clamps to [30, 130] and the band
// boundary is 60 — so the parenthesis is explanatory, never load-bearing.
func TestRoadSourceValidate_SurfaceBandReportsTheClamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		surface  SurfaceType
		speedKPH float64
		want     []string
	}{
		{
			name:     "below the clamp floor",
			surface:  SurfaceOPA,
			speedKPH: 20,
			want:     []string{"20 km/h", "clamped to 30 km/h", "above 60 km/h"},
		},
		{
			name:     "above the clamp ceiling",
			surface:  SurfaceLOA,
			speedKPH: 200,
			want:     []string{"200 km/h", "clamped to 130 km/h", "at 60 km/h and below"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			source := sampleSource()
			source.SurfaceType = tt.surface
			source.Speeds = SpeedInput{
				PkwKPH: tt.speedKPH, Lkw1KPH: tt.speedKPH,
				Lkw2KPH: tt.speedKPH, KradKPH: tt.speedKPH,
			}

			err := source.Validate()
			if err == nil {
				t.Fatalf("expected an error for %q at %g km/h", tt.surface, tt.speedKPH)
			}

			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}

	// An unclamped speed must not carry the parenthesis at all.
	source := sampleSource()
	source.SurfaceType = SurfaceOPA
	source.Speeds = SpeedInput{PkwKPH: 50, Lkw1KPH: 50, Lkw2KPH: 50, KradKPH: 50}

	err := source.Validate()
	if err == nil {
		t.Fatal("expected an error for OPA at 50 km/h")
	}

	if strings.Contains(err.Error(), "clamped") {
		t.Errorf("error %q should not mention clamping for an in-range speed", err)
	}
}

func TestRoadSourceValidate_SurfaceBandBoundaryIsInclusiveBelow(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.SurfaceType = SurfaceOPA
	source.Speeds = SpeedInput{PkwKPH: 60, Lkw1KPH: 100, Lkw2KPH: 100, KradKPH: 60}

	if err := source.Validate(); err == nil {
		t.Fatal("60 km/h belongs to the <= 60 column, where OPA has no cell")
	}

	source.Speeds.PkwKPH = 61
	source.Speeds.KradKPH = 61

	if err := source.Validate(); err != nil {
		t.Fatalf("61 km/h is above the boundary and tabulated: %v", err)
	}
}

// TestRoadSourceValidate_SpeedErrorPrecedesSurfaceError pins the sequential
// first-error contract, and with it the decision to run the applicability
// check last: it reads speeds and traffic counts the earlier steps validate.
func TestRoadSourceValidate_SpeedErrorPrecedesSurfaceError(t *testing.T) {
	t.Parallel()

	source := outOfBandSource()
	source.Speeds.Lkw1KPH = 0

	err := source.Validate()
	if err == nil {
		t.Fatal("expected an error for a zero speed")
	}

	if !strings.Contains(err.Error(), "lkw1_kph") {
		t.Errorf("error %q should report the zero speed", err)
	}

	if strings.Contains(err.Error(), "Tabelle 4a") {
		t.Errorf("error %q should report the speed before the surface pairing", err)
	}
}

// TestComputeEmission_RejectsOutOfBandSurface confirms the refusal reaches the
// compute entry point rather than stopping at Validate.
func TestComputeEmission_RejectsOutOfBandSurface(t *testing.T) {
	t.Parallel()

	result, err := ComputeEmission(outOfBandSource())
	if err == nil {
		t.Fatal("expected ComputeEmission to refuse an out-of-band surface")
	}

	if result != (EmissionResult{}) {
		t.Errorf("expected a zero EmissionResult on refusal, got %+v", result)
	}
}
