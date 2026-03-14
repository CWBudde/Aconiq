package schall03

import (
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

func TestTrainOperationFromZugart(t *testing.T) {
	t.Parallel()

	op, err := NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if op.TrainType != "ICE-1-Zug" {
		t.Errorf("TrainType = %q, want %q", op.TrainType, "ICE-1-Zug")
	}

	if op.SpeedKPH != 250 {
		t.Errorf("SpeedKPH = %v, want 250", op.SpeedKPH)
	}

	if len(op.FzComposition) != 2 {
		t.Fatalf("FzComposition length = %d, want 2", len(op.FzComposition))
	}

	if op.FzComposition[0].Fz != 1 || op.FzComposition[0].Count != 2 {
		t.Errorf("FzComposition[0] = {%d, %d}, want {1, 2}", op.FzComposition[0].Fz, op.FzComposition[0].Count)
	}

	if op.FzComposition[1].Fz != 2 || op.FzComposition[1].Count != 12 {
		t.Errorf("FzComposition[1] = {%d, %d}, want {2, 12}", op.FzComposition[1].Fz, op.FzComposition[1].Count)
	}

	if op.TrainsPerHourDay != 4 {
		t.Errorf("TrainsPerHourDay = %v, want 4", op.TrainsPerHourDay)
	}

	if op.TrainsPerHourNight != 2 {
		t.Errorf("TrainsPerHourNight = %v, want 2", op.TrainsPerHourNight)
	}
}

func TestTrainOperationFromZugartUnknown(t *testing.T) {
	t.Parallel()

	_, err := NewTrainOperationFromZugart("NonExistent-Zug", 4, 2)
	if err == nil {
		t.Fatal("expected error for unknown Zugart, got nil")
	}
}

func TestTrainOperationCustomComposition(t *testing.T) {
	t.Parallel()

	op := TrainOperation{
		TrainType:          "custom",
		FzComposition:      []FzCount{{Fz: 7, Count: 1}, {Fz: 9, Count: 5}},
		SpeedKPH:           160,
		TrainsPerHourDay:   6,
		TrainsPerHourNight: 3,
	}

	err := op.Validate()
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestSpeedDetermination(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		streckeMax  float64
		fahrzeugMax float64
		isStation   bool
		want        float64
	}{
		{
			name:       "normal",
			streckeMax: 200, fahrzeugMax: 250, isStation: false,
			want: 200,
		},
		{
			name:       "vehicle slower",
			streckeMax: 200, fahrzeugMax: 160, isStation: false,
			want: 160,
		},
		{
			name:       "minimum 50",
			streckeMax: 30, fahrzeugMax: 160, isStation: false,
			want: 50,
		},
		{
			name:       "station min 70",
			streckeMax: 120, fahrzeugMax: 160, isStation: true,
			want: 120,
		},
		{
			name:       "station low speed",
			streckeMax: 50, fahrzeugMax: 160, isStation: true,
			want: 70,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := resolveEffectiveSpeed(tc.streckeMax, tc.fahrzeugMax, tc.isStation)
			if got != tc.want {
				t.Errorf("resolveEffectiveSpeed(%v, %v, %v) = %v, want %v",
					tc.streckeMax, tc.fahrzeugMax, tc.isStation, got, tc.want)
			}
		})
	}
}

func TestTrainOperationValidateErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		op   TrainOperation
	}{
		{
			name: "zero speed",
			op: TrainOperation{
				TrainType:     "custom",
				FzComposition: []FzCount{{Fz: 1, Count: 1}},
				SpeedKPH:      0,
			},
		},
		{
			name: "no Fz entries",
			op: TrainOperation{
				TrainType:     "custom",
				FzComposition: nil,
				SpeedKPH:      100,
			},
		},
		{
			name: "invalid Fz number",
			op: TrainOperation{
				TrainType:     "custom",
				FzComposition: []FzCount{{Fz: 99, Count: 1}},
				SpeedKPH:      100,
			},
		},
		{
			name: "negative count",
			op: TrainOperation{
				TrainType:     "custom",
				FzComposition: []FzCount{{Fz: 1, Count: -1}},
				SpeedKPH:      100,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.op.Validate()
			if err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

func TestTrackSegmentValidate(t *testing.T) {
	t.Parallel()

	seg := TrackSegment{
		ID: "seg-1",
		TrackCenterline: []geo.Point2D{
			{X: 0, Y: 0},
			{X: 100, Y: 0},
		},
		ElevationM:    0,
		Fahrbahn:      FahrbahnartSchwellengleis,
		Surface:       SurfaceCondNone,
		BridgeType:    0,
		CurveRadiusM:  0,
		StreckeMaxKPH: 200,
		Operations: []TrainOperation{
			{
				TrainType:          "custom",
				FzComposition:      []FzCount{{Fz: 7, Count: 1}, {Fz: 9, Count: 5}},
				SpeedKPH:           160,
				TrainsPerHourDay:   6,
				TrainsPerHourNight: 3,
			},
		},
	}

	err := seg.Validate()
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	// Test missing ID.
	bad := seg
	bad.ID = ""

	err = bad.Validate()
	if err == nil {
		t.Error("expected error for empty ID, got nil")
	}

	// Test too few centerline points.
	bad = seg
	bad.TrackCenterline = []geo.Point2D{{X: 0, Y: 0}}

	err = bad.Validate()
	if err == nil {
		t.Error("expected error for single centerline point, got nil")
	}

	// Test no operations.
	bad = seg
	bad.Operations = nil

	err = bad.Validate()
	if err == nil {
		t.Error("expected error for no operations, got nil")
	}
}
