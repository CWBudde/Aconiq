package road

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// --- helpers ---

func sampleSource() RoadSource {
	return RoadSource{
		ID:          "road-1",
		SurfaceType: SurfaceSMA,
		Speeds: SpeedInput{
			PkwKPH: 100, Lkw1KPH: 80, Lkw2KPH: 70, KradKPH: 100,
		},
		Centerline: []geo.Point2D{
			{X: -50, Y: 0},
			{X: 50, Y: 0},
		},
		TrafficDay: TrafficInput{
			PkwPerHour: 900, Lkw1PerHour: 40, Lkw2PerHour: 60, KradPerHour: 10,
		},
		TrafficNight: TrafficInput{
			PkwPerHour: 200, Lkw1PerHour: 10, Lkw2PerHour: 20, KradPerHour: 2,
		},
	}
}

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// --- model validation tests ---

func TestRoadSourceValidate(t *testing.T) {
	t.Parallel()

	source := sampleSource()

	err := source.Validate()
	if err != nil {
		t.Fatalf("valid source failed validation: %v", err)
	}
}

func TestRoadSourceValidate_MissingID(t *testing.T) {
	t.Parallel()

	s := sampleSource()

	s.ID = ""

	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestRoadSourceValidate_ShortCenterline(t *testing.T) {
	t.Parallel()

	s := sampleSource()

	s.Centerline = []geo.Point2D{{X: 0, Y: 0}}

	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for short centerline")
	}
}

func TestRoadSourceValidate_InvalidSpeed(t *testing.T) {
	t.Parallel()

	s := sampleSource()

	s.Speeds.PkwKPH = 0

	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for zero speed")
	}
}

func TestRoadSourceValidate_NegativeTraffic(t *testing.T) {
	t.Parallel()

	s := sampleSource()

	s.TrafficDay.PkwPerHour = -1

	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for negative traffic")
	}
}

func TestRoadSourceValidate_InvalidSurface(t *testing.T) {
	t.Parallel()

	s := sampleSource()

	s.SurfaceType = "bogus"

	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for invalid surface type")
	}
}

func TestRoadSourceValidate_InvalidLaneCount(t *testing.T) {
	t.Parallel()

	s := sampleSource()
	s.LaneCount = -1

	err := s.Validate()
	if err == nil {
		t.Fatal("expected error for negative lane_count")
	}
}

func TestVehicleGroupString(t *testing.T) {
	t.Parallel()

	groups := AllVehicleGroups()

	names := []string{"Pkw", "Lkw1", "Lkw2", "Krad"}
	for i, vg := range groups {
		if vg.String() != names[i] {
			t.Fatalf("expected %s, got %s", names[i], vg.String())
		}
	}
}

func TestParseJunctionType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected JunctionType
		wantErr  bool
	}{
		{"none", JunctionNone, false},
		{"signalized", JunctionSignalized, false},
		{"roundabout", JunctionRoundabout, false},
		{"other", JunctionOther, false},
		{"NONE", JunctionNone, false},
		{"bogus", JunctionNone, true},
	}
	for _, tt := range tests {
		jt, err := ParseJunctionType(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("expected error for %q", tt.input)
			}

			continue
		}

		if err != nil {
			t.Fatalf("unexpected error for %q: %v", tt.input, err)
		}

		if jt != tt.expected {
			t.Fatalf("for %q: expected %v, got %v", tt.input, tt.expected, jt)
		}
	}
}

// --- table tests ---

func TestSurfaceCorrection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		surface  SurfaceType
		group    VehicleGroup
		speedKPH float64
		want     float64
	}{
		{name: "SMA alias low Pkw", surface: SurfaceSMA, group: Pkw, speedKPH: 60, want: -2.6},
		{name: "SMA alias high Lkw", surface: SurfaceSMA, group: Lkw1, speedKPH: 80, want: -2.0},
		{name: "Non-ribbed guss asphalt", surface: SurfaceGussasphaltStandard, group: Lkw1, speedKPH: 80, want: 0.0},
		{name: "SMA 5/8 Pkw low", surface: SurfaceSMA5_8, group: Pkw, speedKPH: 60, want: -2.6},
		{name: "SMA 5/8 Lkw low", surface: SurfaceSMA5_8, group: Lkw1, speedKPH: 60, want: -1.8},
		{name: "SMA 5/8 high not applicable", surface: SurfaceSMA5_8, group: Pkw, speedKPH: 80, want: 0.0},
		{name: "SMA 8/11 Pkw high", surface: SurfaceSMA8_11, group: Pkw, speedKPH: 80, want: -1.8},
		{name: "SMA 8/11 Lkw high", surface: SurfaceSMA8_11, group: Lkw1, speedKPH: 80, want: -2.0},
		{name: "AB Pkw low", surface: SurfaceAB, group: Pkw, speedKPH: 50, want: -2.7},
		{name: "AB Pkw high", surface: SurfaceAB, group: Pkw, speedKPH: 80, want: -1.9},
		{name: "AB Lkw low", surface: SurfaceAB, group: Lkw1, speedKPH: 50, want: -1.9},
		{name: "AB Lkw high", surface: SurfaceAB, group: Lkw1, speedKPH: 80, want: -2.1},
		{name: "OPA alias high Pkw", surface: SurfaceOPA, group: Pkw, speedKPH: 80, want: -4.5},
		{name: "OPA PA11 high Lkw", surface: SurfaceOPA11, group: Lkw1, speedKPH: 80, want: -4.4},
		{name: "OPA PA8 high Pkw", surface: SurfaceOPA8, group: Pkw, speedKPH: 80, want: -5.5},
		{name: "Concrete low Pkw", surface: SurfaceConcrete, group: Pkw, speedKPH: 40, want: -1.4},
		{name: "Concrete high Lkw", surface: SurfaceConcrete, group: Lkw1, speedKPH: 80, want: -2.3},
		{name: "Low-noise guss asphalt Pkw", surface: SurfaceGussasphalt, group: Pkw, speedKPH: 40, want: -2.0},
		{name: "Low-noise guss asphalt Lkw", surface: SurfaceGussasphalt, group: Lkw1, speedKPH: 80, want: -1.5},
		{name: "LOA low Pkw", surface: SurfaceLOA, group: Pkw, speedKPH: 40, want: -3.2},
		{name: "LOA low Lkw", surface: SurfaceLOA, group: Lkw1, speedKPH: 40, want: -1.0},
		{name: "LOA high not applicable", surface: SurfaceLOA, group: Pkw, speedKPH: 80, want: 0.0},
		{name: "SMA LA 8 high Pkw", surface: SurfaceSMALA8, group: Pkw, speedKPH: 80, want: -2.8},
		{name: "SMA LA 8 high Lkw", surface: SurfaceSMALA8, group: Lkw1, speedKPH: 80, want: -4.6},
		{name: "DSH-V low Pkw", surface: SurfaceDSHV, group: Pkw, speedKPH: 50, want: -3.9},
		{name: "DSH-V high Pkw", surface: SurfaceDSHV, group: Pkw, speedKPH: 80, want: -2.8},
		{name: "DSH-V low Lkw", surface: SurfaceDSHV, group: Lkw1, speedKPH: 50, want: -0.9},
		{name: "DSH-V high Lkw", surface: SurfaceDSHV, group: Lkw1, speedKPH: 80, want: -2.3},
		{name: "Paving even 30", surface: SurfacePavingEven, group: Pkw, speedKPH: 30, want: 1.0},
		{name: "Paving even 40", surface: SurfacePavingEven, group: Lkw1, speedKPH: 40, want: 2.0},
		{name: "Paving even 50", surface: SurfacePavingEven, group: Pkw, speedKPH: 50, want: 3.0},
		{name: "Paving rough alias 30", surface: SurfacePaving, group: Pkw, speedKPH: 30, want: 5.0},
		{name: "Paving rough 40", surface: SurfacePavingOther, group: Lkw1, speedKPH: 40, want: 6.0},
		{name: "Paving rough 50", surface: SurfacePavingOther, group: Pkw, speedKPH: 50, want: 7.0},
		{name: "Krad uses Pkw band", surface: SurfaceAB, group: Krad, speedKPH: 50, want: -2.7},
		{name: "Legacy damaged surface", surface: SurfaceUnpavedOrDamaged, group: Krad, speedKPH: 50, want: 3.0},
		{name: "Unknown surface", surface: "unknown", group: Pkw, speedKPH: 50, want: 0.0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SurfaceCorrection(tt.surface, tt.group, tt.speedKPH)
			if !almostEqual(got, tt.want, 0.000001) {
				t.Fatalf("SurfaceCorrection(%q, %s, %.0f): want %.6f, got %.6f", tt.surface, tt.group, tt.speedKPH, tt.want, got)
			}
		})
	}
}

func TestGradientCorrection(t *testing.T) {
	t.Parallel()

	// Flat road (g=0): no correction for any group at any speed.
	for _, vg := range AllVehicleGroups() {
		if got := GradientCorrection(0, vg, 100); got != 0 {
			t.Fatalf("flat road %s: expected 0, got %f", vg, got)
		}
	}

	// Steep uphill: Lkw2 correction must exceed Pkw correction.
	// Lkw2 at g=8, v=70: (8-2)/10*(70+10)/10 = 4.8
	// Pkw  at g=8, v=100: (8-2)/10*(100+70)/100 = 1.02
	lkw2Up := GradientCorrection(8, Lkw2, 70)
	pkwUp := GradientCorrection(8, Pkw, 100)
	if lkw2Up <= pkwUp {
		t.Fatalf("expected Lkw2 gradient correction > Pkw uphill: Lkw2=%f Pkw=%f", lkw2Up, pkwUp)
	}

	// Downhill (g=-6): RLS-19 Eqs. 7b/7c give a positive correction for Lkw
	// (engine braking increases noise). Correction must be > 0.
	lkw2Down := GradientCorrection(-6, Lkw2, 70)
	if lkw2Down <= 0 {
		t.Fatalf("expected positive downhill correction for Lkw2 (engine braking), got %f", lkw2Down)
	}

	// Clamped at +/-12.
	if GradientCorrection(15, Lkw2, 70) != GradientCorrection(12, Lkw2, 70) {
		t.Fatal("gradient should be clamped at +12%")
	}
}

func TestJunctionCorrection(t *testing.T) {
	t.Parallel()
	// No junction type: 0.
	if got := JunctionCorrection(JunctionNone, 10); got != 0 {
		t.Fatalf("no junction should be 0, got %f", got)
	}
	// Signalized close: highest correction.
	nearby := JunctionCorrection(JunctionSignalized, 10)
	far := JunctionCorrection(JunctionSignalized, 200)

	if nearby <= 0 {
		t.Fatalf("signalized close should be > 0, got %f", nearby)
	}

	if far != 0 {
		t.Fatalf("signalized far should be 0, got %f", far)
	}
}

func TestComputeBaseEmission_Table3ReferenceValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		group    VehicleGroup
		speedKPH float64
		want     float64
	}{
		{name: "Pkw 30", group: Pkw, speedKPH: 30, want: 94.491511},
		{name: "Pkw 60", group: Pkw, speedKPH: 60, want: 102.747947},
		{name: "Pkw 80", group: Pkw, speedKPH: 80, want: 106.485034},
		{name: "Pkw 100", group: Pkw, speedKPH: 100, want: 109.419914},
		{name: "Pkw 130", group: Pkw, speedKPH: 130, want: 112.889260},
		{name: "Lkw1 30", group: Lkw1, speedKPH: 30, want: 101.398315},
		{name: "Lkw1 60", group: Lkw1, speedKPH: 60, want: 108.616963},
		{name: "Lkw1 80", group: Lkw1, speedKPH: 80, want: 113.545338},
		{name: "Lkw1 100", group: Lkw1, speedKPH: 100, want: 117.612203},
		{name: "Lkw1 130", group: Lkw1, speedKPH: 130, want: 122.490853},
		{name: "Lkw2 30", group: Lkw2, speedKPH: 30, want: 105.744984},
		{name: "Lkw2 60", group: Lkw2, speedKPH: 60, want: 110.758598},
		{name: "Lkw2 80", group: Lkw2, speedKPH: 80, want: 115.778537},
		{name: "Lkw2 100", group: Lkw2, speedKPH: 100, want: 120.235303},
		{name: "Lkw2 130", group: Lkw2, speedKPH: 130, want: 125.691501},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := computeBaseEmission(tt.speedKPH, tt.group)
			if !almostEqual(got, tt.want, 0.000001) {
				t.Fatalf("computeBaseEmission(%s, %.0f): want %.6f, got %.6f", tt.group, tt.speedKPH, tt.want, got)
			}
		})
	}
}

func TestComputeVehicleGroupEmissions_KradUsesPkwSpeedForBaseEmission(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.JunctionType = JunctionNone
	source.SurfaceType = SurfaceSMA
	source.GradientPercent = 0
	source.Speeds = SpeedInput{PkwKPH: 30, Lkw1KPH: 60, Lkw2KPH: 80, KradKPH: 130}

	emissions, err := ComputeVehicleGroupEmissions(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var krad VehicleGroupEmission
	for _, emission := range emissions {
		if emission.Group == Krad {
			krad = emission
			break
		}
	}

	if !almostEqual(krad.BaseLevel, 105.744984, 0.000001) {
		t.Fatalf("Krad base emission should use Pkw speed with Lkw2 coefficients: got %.6f", krad.BaseLevel)
	}

	if almostEqual(krad.BaseLevel, 125.691501, 0.000001) {
		t.Fatal("Krad base emission should not use KradKPH directly")
	}
}

func TestEmissionForPeriod_ImplementsEq4ForSingleVehicleGroup(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.JunctionType = JunctionNone
	source.SurfaceType = SurfaceSMA
	source.GradientPercent = 0
	source.ReflectionSurchargeDB = 0
	source.TrafficDay = TrafficInput{PkwPerHour: 900}

	result, err := ComputeEmission(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	base := computeVehicleSoundPower(source, Pkw)
	speed := effectiveVehicleSpeed(source.Speeds, Pkw)
	want := base + 10*math.Log10(source.TrafficDay.PkwPerHour/speed) - 30
	if !almostEqual(result.LmEDay, want, 0.000001) {
		t.Fatalf("single-group Eq. 4 emission: want %.6f, got %.6f", want, result.LmEDay)
	}
}

func TestEmissionForPeriod_PerGroupCountsMatchTotalShareForm(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.ReflectionSurchargeDB = 0

	direct := 0.0
	shareWeighted := 0.0
	totalCount := source.TrafficDay.TotalPerHour()
	if totalCount <= 0 {
		t.Fatal("sample source must have positive total traffic")
	}

	for _, vg := range AllVehicleGroups() {
		count := source.TrafficDay.CountForGroup(vg)
		if count <= 0 {
			continue
		}

		level := computeVehicleSoundPower(source, vg)
		speed := effectiveVehicleSpeed(source.Speeds, vg)
		term := math.Pow(10, level/10) / speed

		direct += count * term
		shareWeighted += (count / totalCount) * term
	}

	directLevel := 10*math.Log10(direct) - 30
	shareLevel := 10*math.Log10(totalCount) + 10*math.Log10(shareWeighted) - 30
	if !almostEqual(directLevel, shareLevel, 0.000001) {
		t.Fatalf("Eq. 4 direct-count and total-share forms should match: direct=%.6f share=%.6f", directLevel, shareLevel)
	}

	result, err := ComputeEmission(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !almostEqual(result.LmEDay, directLevel, 0.000001) {
		t.Fatalf("ComputeEmission should implement Eq. 4: want %.6f, got %.6f", directLevel, result.LmEDay)
	}
}

// --- emission tests ---

func TestComputeEmission_Valid(t *testing.T) {
	t.Parallel()

	source := sampleSource()

	result, err := ComputeEmission(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Day should be higher than night (more traffic).
	if result.LmEDay <= result.LmENight {
		t.Fatalf("expected day > night emission: day=%f night=%f", result.LmEDay, result.LmENight)
	}
	// Both should be finite and reasonable.
	if math.IsNaN(result.LmEDay) || math.IsInf(result.LmEDay, 0) {
		t.Fatal("day emission is not finite")
	}

	if math.IsNaN(result.LmENight) || math.IsInf(result.LmENight, 0) {
		t.Fatal("night emission is not finite")
	}
}

func TestEmission_IncreasesWithTraffic(t *testing.T) {
	t.Parallel()

	low := sampleSource()
	low.TrafficDay = TrafficInput{PkwPerHour: 200, Lkw1PerHour: 10, Lkw2PerHour: 10, KradPerHour: 2}

	lowResult, err := ComputeEmission(low)
	if err != nil {
		t.Fatalf("compute low emission: %v", err)
	}

	high := sampleSource()
	high.TrafficDay = TrafficInput{PkwPerHour: 1200, Lkw1PerHour: 80, Lkw2PerHour: 120, KradPerHour: 20}

	highResult, err := ComputeEmission(high)
	if err != nil {
		t.Fatalf("compute high emission: %v", err)
	}

	if highResult.LmEDay <= lowResult.LmEDay {
		t.Fatalf("higher traffic should increase emission: low=%f high=%f", lowResult.LmEDay, highResult.LmEDay)
	}
}

func TestEmission_IncreasesWithSpeed(t *testing.T) {
	t.Parallel()

	slow := sampleSource()
	slow.Speeds = SpeedInput{PkwKPH: 50, Lkw1KPH: 50, Lkw2KPH: 50, KradKPH: 50}

	slowResult, err := ComputeEmission(slow)
	if err != nil {
		t.Fatalf("compute slow emission: %v", err)
	}

	fast := sampleSource()
	fast.Speeds = SpeedInput{PkwKPH: 130, Lkw1KPH: 80, Lkw2KPH: 80, KradKPH: 130}

	fastResult, err := ComputeEmission(fast)
	if err != nil {
		t.Fatalf("compute fast emission: %v", err)
	}

	if fastResult.LmEDay <= slowResult.LmEDay {
		t.Fatalf("higher speed should increase emission: slow=%f fast=%f", slowResult.LmEDay, fastResult.LmEDay)
	}
}

func TestEmission_SurfaceAffectsLevel(t *testing.T) {
	t.Parallel()

	quiet := sampleSource()
	quiet.SurfaceType = SurfaceOPA

	quietResult, err := ComputeEmission(quiet)
	if err != nil {
		t.Fatalf("compute quiet surface: %v", err)
	}

	loud := sampleSource()
	loud.SurfaceType = SurfacePaving

	loudResult, err := ComputeEmission(loud)
	if err != nil {
		t.Fatalf("compute loud surface: %v", err)
	}

	if loudResult.LmEDay <= quietResult.LmEDay {
		t.Fatalf("paving should be louder than OPA: OPA=%f paving=%f", quietResult.LmEDay, loudResult.LmEDay)
	}
}

func TestEmission_GradientAffectsLevel(t *testing.T) {
	t.Parallel()

	flat := sampleSource()
	flat.GradientPercent = 0

	flatResult, err := ComputeEmission(flat)
	if err != nil {
		t.Fatalf("compute flat: %v", err)
	}

	steep := sampleSource()
	steep.GradientPercent = 8

	steepResult, err := ComputeEmission(steep)
	if err != nil {
		t.Fatalf("compute steep: %v", err)
	}

	if steepResult.LmEDay <= flatResult.LmEDay {
		t.Fatalf("steep uphill should increase emission: flat=%f steep=%f", flatResult.LmEDay, steepResult.LmEDay)
	}
}

func TestEmission_JunctionAffectsLevel(t *testing.T) {
	t.Parallel()

	noJunction := sampleSource()
	noJunction.JunctionType = JunctionNone

	noJunctionResult, err := ComputeEmission(noJunction)
	if err != nil {
		t.Fatalf("compute no junction: %v", err)
	}

	withJunction := sampleSource()
	withJunction.JunctionType = JunctionSignalized
	withJunction.JunctionDistanceM = 20

	withJunctionResult, err := ComputeEmission(withJunction)
	if err != nil {
		t.Fatalf("compute with junction: %v", err)
	}

	if withJunctionResult.LmEDay <= noJunctionResult.LmEDay {
		t.Fatalf("junction should increase emission: none=%f signalized=%f", noJunctionResult.LmEDay, withJunctionResult.LmEDay)
	}
}

func TestEmission_ReflectionSurcharge(t *testing.T) {
	t.Parallel()

	base := sampleSource()

	baseResult, err := ComputeEmission(base)
	if err != nil {
		t.Fatalf("compute base: %v", err)
	}

	withRefl := sampleSource()
	withRefl.ReflectionSurchargeDB = 2.0

	reflResult, err := ComputeEmission(withRefl)
	if err != nil {
		t.Fatalf("compute with reflection: %v", err)
	}

	// Reflection surcharge should increase level by exactly 2 dB.
	diff := reflResult.LmEDay - baseResult.LmEDay
	if !almostEqual(diff, 2.0, 0.01) {
		t.Fatalf("reflection surcharge should add 2 dB: got diff=%f", diff)
	}
}

func TestVehicleGroupEmissions(t *testing.T) {
	t.Parallel()

	source := sampleSource()

	emissions, err := ComputeVehicleGroupEmissions(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(emissions) != 4 {
		t.Fatalf("expected 4 vehicle groups, got %d", len(emissions))
	}

	for _, e := range emissions {
		if math.IsNaN(e.SoundPowerLevel) || math.IsInf(e.SoundPowerLevel, 0) {
			t.Fatalf("vehicle group %s has non-finite sound power", e.Group)
		}
	}
}

func TestEmission_ZeroTraffic(t *testing.T) {
	t.Parallel()

	s := sampleSource()
	s.TrafficDay = TrafficInput{}
	s.TrafficNight = TrafficInput{}

	result, err := ComputeEmission(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LmEDay > -900 {
		t.Fatalf("zero traffic should give very low level, got %f", result.LmEDay)
	}
}

// --- segment splitting tests ---

func TestSplitLineIntoSegments_BasicLine(t *testing.T) {
	t.Parallel()

	line := []geo.Point2D{{X: 0, Y: 0}, {X: 100, Y: 0}}

	segs := SplitLineIntoSegments(line, nil, 10)
	if len(segs) != 10 {
		t.Fatalf("expected 10 segments, got %d", len(segs))
	}
	// First segment midpoint should be near x=5.
	if !almostEqual(segs[0].MidPoint.X, 5, 0.01) {
		t.Fatalf("first midpoint X: expected 5, got %f", segs[0].MidPoint.X)
	}
	// All segments should have equal length.
	for i, seg := range segs {
		if !almostEqual(seg.LengthM, 10, 0.01) {
			t.Fatalf("segment %d length: expected 10, got %f", i, seg.LengthM)
		}
	}
	// MidZ should be zero when no elevations provided.
	for _, seg := range segs {
		if seg.MidZ != 0 {
			t.Fatalf("expected MidZ=0 without elevations, got %f", seg.MidZ)
		}
	}
}

func TestSplitLineIntoSegments_ShortLine(t *testing.T) {
	t.Parallel()

	line := []geo.Point2D{{X: 0, Y: 0}, {X: 3, Y: 0}}

	segs := SplitLineIntoSegments(line, nil, 10)
	if len(segs) != 1 {
		t.Fatalf("short line should produce 1 segment, got %d", len(segs))
	}

	if !almostEqual(segs[0].MidPoint.X, 1.5, 0.01) {
		t.Fatalf("midpoint X: expected 1.5, got %f", segs[0].MidPoint.X)
	}
}

func TestSplitLineIntoSegments_Polyline(t *testing.T) {
	t.Parallel()
	// L-shaped line: 50m east + 50m north = 100m total.
	line := []geo.Point2D{{X: 0, Y: 0}, {X: 50, Y: 0}, {X: 50, Y: 50}}

	segs := SplitLineIntoSegments(line, nil, 10)
	if len(segs) != 10 {
		t.Fatalf("expected 10 segments, got %d", len(segs))
	}

	totalLen := 0.0
	for _, seg := range segs {
		totalLen += seg.LengthM
	}

	if !almostEqual(totalLen, 100, 0.01) {
		t.Fatalf("total segment length: expected 100, got %f", totalLen)
	}
}

func TestSplitLineIntoSegments_Deterministic(t *testing.T) {
	t.Parallel()

	line := []geo.Point2D{{X: 0, Y: 0}, {X: 100, Y: 0}}
	segs1 := SplitLineIntoSegments(line, nil, 7)

	segs2 := SplitLineIntoSegments(line, nil, 7)
	if len(segs1) != len(segs2) {
		t.Fatal("segments should be deterministic")
	}

	for i := range segs1 {
		if segs1[i].MidPoint != segs2[i].MidPoint || segs1[i].LengthM != segs2[i].LengthM {
			t.Fatalf("segment %d differs between runs", i)
		}
	}
}

func TestRoadSourceEffectiveCenterline_AppliesLaneOffset(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.Centerline = []geo.Point2D{{X: 0, Y: 0}, {X: 100, Y: 0}}
	source.LaneCount = 2

	line := source.EffectiveCenterline()
	if len(line) != 2 {
		t.Fatalf("expected 2 points, got %d", len(line))
	}

	if !almostEqual(source.SourceLineOffsetM(), 1.75, 0.000001) {
		t.Fatalf("expected lane-count offset 1.75 m, got %.6f", source.SourceLineOffsetM())
	}

	if !almostEqual(line[0].Y, -1.75, 0.000001) || !almostEqual(line[1].Y, -1.75, 0.000001) {
		t.Fatalf("expected right-hand offset line at y=-1.75, got %#v", line)
	}
}

func TestPropagation_LaneCountAutoOffsetMatchesExplicitGeometry(t *testing.T) {
	t.Parallel()

	auto := sampleSource()
	auto.Centerline = []geo.Point2D{{X: 0, Y: 0}, {X: 100, Y: 0}}
	auto.LaneCount = 2

	explicit := sampleSource()
	explicit.Centerline = []geo.Point2D{{X: 0, Y: -1.75}, {X: 100, Y: -1.75}}

	cfg := DefaultPropagationConfig()
	receiver := geo.Point2D{X: 50, Y: -20}

	autoLevels, err := ComputeReceiverLevels(receiver, []RoadSource{auto}, nil, cfg)
	if err != nil {
		t.Fatalf("auto-offset propagation: %v", err)
	}

	explicitLevels, err := ComputeReceiverLevels(receiver, []RoadSource{explicit}, nil, cfg)
	if err != nil {
		t.Fatalf("explicit-offset propagation: %v", err)
	}

	if !almostEqual(autoLevels.LrDay, explicitLevels.LrDay, 0.000001) {
		t.Fatalf("expected equal day level for auto and explicit source line: auto=%.6f explicit=%.6f", autoLevels.LrDay, explicitLevels.LrDay)
	}

	if !almostEqual(autoLevels.LrNight, explicitLevels.LrNight, 0.000001) {
		t.Fatalf("expected equal night level for auto and explicit source line: auto=%.6f explicit=%.6f", autoLevels.LrNight, explicitLevels.LrNight)
	}
}

// --- propagation tests ---

func TestPropagation_DecreasesWithDistance(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()

	near, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 5}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("compute near: %v", err)
	}

	far, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 200}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("compute far: %v", err)
	}

	if near.LrDay <= far.LrDay {
		t.Fatalf("expected near > far: near=%f far=%f", near.LrDay, far.LrDay)
	}
}

func TestPropagation_DayHigherThanNight(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()

	levels, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 25}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}

	if levels.LrDay <= levels.LrNight {
		t.Fatalf("expected day > night: day=%f night=%f", levels.LrDay, levels.LrNight)
	}
}

func TestPropagation_MultipleSources(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()

	single, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 25}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("compute single: %v", err)
	}

	// Two identical sources: +3 dB.
	double, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 25}, []RoadSource{source, source}, nil, cfg)
	if err != nil {
		t.Fatalf("compute double: %v", err)
	}

	diff := double.LrDay - single.LrDay
	if !almostEqual(diff, 3.0, 0.2) {
		t.Fatalf("doubling sources should add ~3 dB: got diff=%f", diff)
	}
}

func TestPropagation_Deterministic(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()
	receiver := geo.Point2D{X: 0, Y: 50}

	r1, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}

	r2, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}

	if r1.LrDay != r2.LrDay || r1.LrNight != r2.LrNight {
		t.Fatalf("results should be deterministic: run1=%+v run2=%+v", r1, r2)
	}
}

func TestPropagation_InvalidConfig(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := PropagationConfig{SegmentLengthM: -1, MinDistanceM: 3}

	_, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 10}, []RoadSource{source}, nil, cfg)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestPropagation_NoSources(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()

	_, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 10}, nil, nil, cfg)
	if err == nil {
		t.Fatal("expected error for no sources")
	}
}

// --- compute orchestration tests ---

func TestComputeReceiverOutputs(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()
	receivers := []geo.PointReceiver{
		{ID: "r1", Point: geo.Point2D{X: 0, Y: 10}, HeightM: 4},
		{ID: "r2", Point: geo.Point2D{X: 0, Y: 50}, HeightM: 4},
		{ID: "r3", Point: geo.Point2D{X: 0, Y: 100}, HeightM: 4},
	}

	outputs, err := ComputeReceiverOutputs(receivers, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("compute outputs: %v", err)
	}

	if len(outputs) != 3 {
		t.Fatalf("expected 3 outputs, got %d", len(outputs))
	}

	// Verify monotonic decrease with distance.
	for i := 1; i < len(outputs); i++ {
		if outputs[i].Indicators.LrDay >= outputs[i-1].Indicators.LrDay {
			t.Fatalf("level should decrease with distance: at=%d prev=%f next=%f",
				i, outputs[i-1].Indicators.LrDay, outputs[i].Indicators.LrDay)
		}
	}
}

func TestComputeReceiverOutputs_UsesPerReceiverHeight(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()
	barrier := sampleBarrier()
	receivers := []geo.PointReceiver{
		{ID: "low", Point: geo.Point2D{X: 0, Y: 50}, HeightM: 2.0},
		{ID: "high", Point: geo.Point2D{X: 0, Y: 50}, HeightM: 15.0},
	}

	outputs, err := ComputeReceiverOutputs(receivers, []RoadSource{source}, []Barrier{barrier}, cfg)
	if err != nil {
		t.Fatalf("compute outputs: %v", err)
	}

	if len(outputs) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(outputs))
	}

	if outputs[0].Indicators.LrDay >= outputs[1].Indicators.LrDay {
		t.Fatalf(
			"expected higher receiver to reduce shielding and increase level: low=%.4f high=%.4f",
			outputs[0].Indicators.LrDay,
			outputs[1].Indicators.LrDay,
		)
	}
}

func TestComputeReceiverOutputs_EmptyReceivers(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()

	_, err := ComputeReceiverOutputs(nil, []RoadSource{sampleSource()}, nil, cfg)
	if err == nil {
		t.Fatal("expected error for empty receivers")
	}
}

// --- descriptor tests ---

func TestDescriptorValidates(t *testing.T) {
	t.Parallel()

	descriptor := Descriptor()

	err := descriptor.Validate()
	if err != nil {
		t.Fatalf("descriptor should validate: %v", err)
	}

	if descriptor.ID != StandardID {
		t.Fatalf("unexpected ID: %s", descriptor.ID)
	}

	if descriptor.DefaultVersion != "2019" {
		t.Fatalf("unexpected version: %s", descriptor.DefaultVersion)
	}
}

// --- shielding tests ---

func sampleBarrier() Barrier {
	// Barrier running east-west at y=10, between source (y=0) and receiver (y>10).
	return Barrier{
		ID:       "wall-1",
		Geometry: []geo.Point2D{{X: -100, Y: 10}, {X: 100, Y: 10}},
		HeightM:  4.0,
	}
}

func TestBarrierValidate(t *testing.T) {
	t.Parallel()

	b := sampleBarrier()

	err := b.Validate()
	if err != nil {
		t.Fatalf("valid barrier failed: %v", err)
	}

	b.HeightM = 0

	err = b.Validate()
	if err == nil {
		t.Fatal("expected error for zero height")
	}
}

func TestComputeShielding_NoBarriers(t *testing.T) {
	t.Parallel()

	result := ComputeShielding(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 0, Y: 50}, 4.0,
		nil,
	)
	if result.Shielded {
		t.Fatal("expected no shielding without barriers")
	}
}

func TestComputeShielding_BarrierBetween(t *testing.T) {
	t.Parallel()

	barrier := sampleBarrier()
	result := ComputeShielding(
		geo.Point2D{X: 0, Y: 0}, 0.5, // source at y=0, height 0.5m
		geo.Point2D{X: 0, Y: 50}, 4.0, // receiver at y=50, height 4m
		[]Barrier{barrier}, // barrier at y=10, height 4m
	)

	if !result.Shielded {
		t.Fatal("expected shielding from barrier")
	}

	if result.InsertionLoss <= 0 {
		t.Fatalf("expected positive insertion loss, got %f", result.InsertionLoss)
	}

	if result.BarrierID != "wall-1" {
		t.Fatalf("expected barrier ID wall-1, got %q", result.BarrierID)
	}
}

func TestComputeShielding_BarrierNotCrossing(t *testing.T) {
	t.Parallel()

	// Barrier parallel to source-receiver path, does not cross it.
	barrier := Barrier{
		ID:       "parallel",
		Geometry: []geo.Point2D{{X: 5, Y: -10}, {X: 5, Y: 100}},
		HeightM:  4.0,
	}

	result := ComputeShielding(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 0, Y: 50}, 4.0,
		[]Barrier{barrier},
	)

	if result.Shielded {
		t.Fatal("expected no shielding from parallel barrier")
	}
}

func TestComputeShielding_LowBarrier(t *testing.T) {
	t.Parallel()

	// Barrier lower than line of sight — should not shield.
	lowBarrier := Barrier{
		ID:       "low",
		Geometry: []geo.Point2D{{X: -100, Y: 10}, {X: 100, Y: 10}},
		HeightM:  0.1, // very low
	}

	result := ComputeShielding(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 0, Y: 50}, 4.0,
		[]Barrier{lowBarrier},
	)

	if result.Shielded {
		t.Fatal("expected no shielding from very low barrier")
	}
}

func TestComputeShielding_TallBarrier(t *testing.T) {
	t.Parallel()

	tall := Barrier{
		ID:       "tall",
		Geometry: []geo.Point2D{{X: -100, Y: 10}, {X: 100, Y: 10}},
		HeightM:  10.0,
	}

	short := Barrier{
		ID:       "short",
		Geometry: []geo.Point2D{{X: -100, Y: 10}, {X: 100, Y: 10}},
		HeightM:  4.0,
	}

	tallResult := ComputeShielding(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 0, Y: 50}, 4.0,
		[]Barrier{tall},
	)

	shortResult := ComputeShielding(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 0, Y: 50}, 4.0,
		[]Barrier{short},
	)

	if tallResult.InsertionLoss <= shortResult.InsertionLoss {
		t.Fatalf("taller barrier should have more attenuation: tall=%f short=%f",
			tallResult.InsertionLoss, shortResult.InsertionLoss)
	}
}

func TestComputeShielding_MultipleBarriers(t *testing.T) {
	t.Parallel()

	// Two barriers: the taller one should determine the result.
	shortBarrier := Barrier{
		ID:       "short",
		Geometry: []geo.Point2D{{X: -100, Y: 10}, {X: 100, Y: 10}},
		HeightM:  4.0,
	}

	tallBarrier := Barrier{
		ID:       "tall",
		Geometry: []geo.Point2D{{X: -100, Y: 20}, {X: 100, Y: 20}},
		HeightM:  8.0,
	}

	result := ComputeShielding(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 0, Y: 50}, 4.0,
		[]Barrier{shortBarrier, tallBarrier},
	)

	if !result.Shielded {
		t.Fatal("expected shielding")
	}

	if result.BarrierID != "tall" {
		t.Fatalf("expected tall barrier to dominate, got %q", result.BarrierID)
	}
}

func TestPropagation_WithBarrier(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()
	receiver := geo.Point2D{X: 0, Y: 50}

	// Free-field level.
	freeField, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("free field: %v", err)
	}

	// With barrier between source and receiver.
	barrier := sampleBarrier()

	shielded, err := ComputeReceiverLevels(receiver, []RoadSource{source}, []Barrier{barrier}, cfg)
	if err != nil {
		t.Fatalf("with barrier: %v", err)
	}

	// Barrier should reduce the level.
	reduction := freeField.LrDay - shielded.LrDay
	if reduction <= 0 {
		t.Fatalf("barrier should reduce level: free=%f shielded=%f", freeField.LrDay, shielded.LrDay)
	}

	// Reduction should be meaningful (several dB for a 4m wall at 10m from source).
	if reduction < 2 {
		t.Fatalf("expected at least 2 dB reduction, got %f", reduction)
	}
}

func TestPathDifference(t *testing.T) {
	t.Parallel()

	// Barrier exactly at line of sight: delta should be ~0.
	// Source at (0, h=0.5), barrier at (10, h=4), receiver at (50, h=4).
	// Line of sight from source to receiver: at x=10, height = 0.5 + (4-0.5)*10/50 = 1.2.
	// Barrier height 1.2 → delta ≈ 0.
	delta := pathDifference(10, 0.5, 40, 4.0, 1.2)
	if math.Abs(delta) > 0.01 {
		t.Fatalf("barrier at line of sight should have delta ~0, got %f", delta)
	}

	// Barrier well above line of sight: positive delta.
	delta = pathDifference(10, 0.5, 40, 4.0, 8.0)
	if delta <= 0 {
		t.Fatalf("tall barrier should have positive delta, got %f", delta)
	}

	// Barrier below line of sight: non-positive delta.
	delta = pathDifference(10, 0.5, 40, 4.0, 0.1)
	if delta > 0 {
		t.Fatalf("low barrier should have non-positive delta, got %f", delta)
	}
}

func TestMaekawaInsertionLoss(t *testing.T) {
	t.Parallel()

	// Zero delta: no loss.
	if maekawaInsertionLoss(0) != 0 {
		t.Fatal("zero delta should give zero loss")
	}

	// Positive delta: positive loss.
	loss := maekawaInsertionLoss(0.5)
	if loss <= 0 {
		t.Fatalf("positive delta should give positive loss, got %f", loss)
	}

	// Loss increases with delta.
	lossSmall := maekawaInsertionLoss(0.1)

	lossLarge := maekawaInsertionLoss(1.0)
	if lossLarge <= lossSmall {
		t.Fatalf("loss should increase with delta: small=%f large=%f", lossSmall, lossLarge)
	}

	// Capped at 20 dB.
	lossCapped := maekawaInsertionLoss(100)
	if lossCapped > 20 {
		t.Fatalf("loss should be capped at 20, got %f", lossCapped)
	}
}

// --- energySumDB tests ---

func TestEnergySumDB(t *testing.T) {
	t.Parallel()

	// Two equal levels: +3 dB.
	result := energySumDB([]float64{60, 60})
	if !almostEqual(result, 63.01, 0.01) {
		t.Fatalf("60+60 dB: expected ~63.01, got %f", result)
	}

	// Empty: -999.
	result = energySumDB(nil)
	if result > -900 {
		t.Fatalf("empty sum: expected -999, got %f", result)
	}

	// Single value passes through.
	result = energySumDB([]float64{55.0})
	if !almostEqual(result, 55.0, 0.01) {
		t.Fatalf("single value: expected 55, got %f", result)
	}
}

// --- topography tests ---

// sampleTieflageSource returns a source at Z=100 (road in cut, terrain at Z=105.5).
// Geometry matches TEST-20 I6 (simplified single Teilstück at midpoint X=100, Y=50).
func sampleTieflageSource() RoadSource {
	s := sampleSource()
	s.ElevationM = 100.0 // road surface at Z=100, terrain at Z=105.5

	return s
}

// tieflageSlopeCrest returns the Böschungskante for TEST-20 I6:
// slope crest at Y=62.8, Z=105.5 (terrain level), running along X-axis.
func tieflageSlopeCrest() TerrainEdge {
	return TerrainEdge{
		ID: "boeschungskante-i6",
		Geometry: []geo.Point3D{
			{X: -200, Y: 62.8, Z: 105.5},
			{X: 400, Y: 62.8, Z: 105.5},
		},
	}
}

// tieflageSlopeFoot returns the Böschungsfuß for TEST-20 I6:
// slope foot at Y=55.3, Z=100 (road level), running along X-axis.
func tieflageSlopeFoot() TerrainEdge {
	return TerrainEdge{
		ID: "boeschungsfuss-i6",
		Geometry: []geo.Point3D{
			{X: -200, Y: 55.3, Z: 100.0},
			{X: 400, Y: 55.3, Z: 100.0},
		},
	}
}

func TestTerrainEdgeValidate(t *testing.T) {
	t.Parallel()

	e := tieflageSlopeCrest()

	err := e.Validate()
	if err != nil {
		t.Fatalf("valid edge failed: %v", err)
	}

	// Too few points.
	e2 := TerrainEdge{ID: "x", Geometry: []geo.Point3D{{X: 0, Y: 0, Z: 0}}}

	err = e2.Validate()
	if err == nil {
		t.Fatal("expected error for single-point edge")
	}

	// Missing ID.
	e3 := TerrainEdge{Geometry: []geo.Point3D{{}, {}}}

	err = e3.Validate()
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestComputeTerrainAvgZ_NoProfiles(t *testing.T) {
	t.Parallel()

	avg := computeTerrainAvgZ(
		geo.Point2D{X: 0, Y: 0},
		geo.Point2D{X: 100, Y: 0},
		nil,
	)

	if avg != 0 {
		t.Fatalf("expected 0 for no terrain profiles, got %f", avg)
	}
}

func TestComputeTerrainAvgZ_FlatTerrain(t *testing.T) {
	t.Parallel()

	// Single terrain edge at a constant Z=105.5 crossing the path.
	edge := TerrainEdge{
		ID:       "flat-edge",
		Geometry: []geo.Point3D{{X: -10, Y: 50, Z: 105.5}, {X: 200, Y: 50, Z: 105.5}},
	}
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: edge}}}

	avg := computeTerrainAvgZ(
		geo.Point2D{X: 100, Y: 0},
		geo.Point2D{X: 100, Y: 100},
		[]TerrainProfile{profile},
	)

	// Edge crosses at midpoint of path; terrain before = 105.5, after = 105.5.
	if !almostEqual(avg, 105.5, 0.01) {
		t.Fatalf("expected terrain avg ~105.5, got %f", avg)
	}
}

// TestComputeMeanHeight_Tieflage verifies the h_m formula against TEST-20 I6 IO1.
// I6 IO1: source Z=100.5, receiver Z=108.3, terrain=105.5 → expected h_m ≈ -0.10.
func TestComputeMeanHeight_Tieflage(t *testing.T) {
	t.Parallel()

	foot := tieflageSlopeFoot()
	crest := tieflageSlopeCrest()
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	// Source midpoint (TEST-20 Teilstück midpoint), receiver IO1.
	source := geo.Point2D{X: 100, Y: 50}
	receiver := geo.Point2D{X: 0, Y: 100}
	sourceZ := 100.5   // road at 100, + 0.5 m source height
	receiverZ := 108.3 // IO1 absolute Z

	hm := computeMeanHeight(source, receiver, sourceZ, receiverZ, []TerrainProfile{profile})

	// TEST-20 reference: h_m = -0.104
	if !almostEqual(hm, -0.104, 0.05) {
		t.Fatalf("Tieflage I6 IO1: expected h_m ≈ -0.10, got %f", hm)
	}
}

// TestComputeMeanHeight_TieflageFarReceiver verifies h_m for TEST-20 I6 IO2.
// I6 IO2: source Z=100.5, receiver Z=120.5 → expected h_m ≈ 5.99.
func TestComputeMeanHeight_TieflageFarReceiver(t *testing.T) {
	t.Parallel()

	foot := tieflageSlopeFoot()
	crest := tieflageSlopeCrest()
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	source := geo.Point2D{X: 100, Y: 50}
	receiver := geo.Point2D{X: 0, Y: 100}
	sourceZ := 100.5
	receiverZ := 120.5 // IO2 absolute Z

	hm := computeMeanHeight(source, receiver, sourceZ, receiverZ, []TerrainProfile{profile})

	// TEST-20 reference: h_m = 5.988
	if !almostEqual(hm, 5.988, 0.05) {
		t.Fatalf("Tieflage I6 IO2: expected h_m ≈ 5.99, got %f", hm)
	}
}

// TestComputeMeanHeight_Hochlage verifies the h_m formula against TEST-20 I7 IO1.
// I7 IO1: road at Z=105, terrain at Z=100, receiver Z=102.6 → expected h_m ≈ 2.24.
func TestComputeMeanHeight_Hochlage(t *testing.T) {
	t.Parallel()

	// I7: Böschungskante at Y=55.3, Z=105 (top of embankment = road level)
	//      Böschungsfuß  at Y=62.8, Z=100 (bottom = terrain level)
	crest := TerrainEdge{
		ID:       "boeschungskante-i7",
		Geometry: []geo.Point3D{{X: -200, Y: 55.3, Z: 105.0}, {X: 400, Y: 55.3, Z: 105.0}},
	}
	foot := TerrainEdge{
		ID:       "boeschungsfuss-i7",
		Geometry: []geo.Point3D{{X: -200, Y: 62.8, Z: 100.0}, {X: 400, Y: 62.8, Z: 100.0}},
	}
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	source := geo.Point2D{X: 100, Y: 50}
	receiver := geo.Point2D{X: 0, Y: 75}
	sourceZ := 105.5   // road at 105, + 0.5 m
	receiverZ := 102.6 // IO1 absolute Z

	hm := computeMeanHeight(source, receiver, sourceZ, receiverZ, []TerrainProfile{profile})

	// TEST-20 reference: h_m = 2.237
	if !almostEqual(hm, 2.237, 0.05) {
		t.Fatalf("Hochlage I7 IO1: expected h_m ≈ 2.24, got %f", hm)
	}
}

// TestTerrainEdgeShielding_Tieflage verifies that the Böschungskante shields IO1
// in a Tieflage scenario (road in cut below terrain).
func TestTerrainEdgeShielding_Tieflage(t *testing.T) {
	t.Parallel()

	foot := tieflageSlopeFoot()
	crest := tieflageSlopeCrest()
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	// IO1 is behind the Böschungskante — should be shielded.
	result := computeTerrainEdgeShielding(
		geo.Point2D{X: 100, Y: 50}, 100.5,
		geo.Point2D{X: 0, Y: 100}, 108.3,
		[]TerrainProfile{profile},
	)

	if !result.Shielded {
		t.Fatal("IO1 behind Böschungskante should be shielded")
	}

	if result.InsertionLoss <= 0 {
		t.Fatalf("expected positive insertion loss, got %f", result.InsertionLoss)
	}
}

// TestTerrainEdgeShielding_NoShielding verifies that IO2 (high receiver) is not
// shielded in TEST-20 I6 because the line of sight clears the Böschungskante.
func TestTerrainEdgeShielding_NoShielding(t *testing.T) {
	t.Parallel()

	foot := tieflageSlopeFoot()
	crest := tieflageSlopeCrest()
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	// IO2 at Z=120.5: line of sight clears the 105.5 crest.
	result := computeTerrainEdgeShielding(
		geo.Point2D{X: 100, Y: 50}, 100.5,
		geo.Point2D{X: 0, Y: 100}, 120.5,
		[]TerrainProfile{profile},
	)

	if result.Shielded {
		t.Fatalf("IO2 line-of-sight clears Böschungskante — should not be shielded")
	}
}

// TestPropagation_Tieflage verifies that a road in a cut is louder for a
// high receiver (not shielded) than for a low receiver (shielded by crest).
func TestPropagation_Tieflage(t *testing.T) {
	t.Parallel()

	source := sampleTieflageSource()
	foot := tieflageSlopeFoot()
	crest := tieflageSlopeCrest()
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	cfg := DefaultPropagationConfig()
	cfg.ReceiverTerrainZ = 105.5
	cfg.Terrain = []TerrainProfile{profile}

	// IO1: low receiver (2.8 m above terrain), shielded by Böschungskante.
	cfg.ReceiverHeightM = 2.8

	shieldedLevel, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 100}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("Tieflage shielded: %v", err)
	}

	// IO2: high receiver (15 m above terrain), not shielded.
	cfg.ReceiverHeightM = 15.0

	unshieldedLevel, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 100}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("Tieflage unshielded: %v", err)
	}

	// The higher receiver (not shielded) is closer in effective distance
	// but the shielded one has extra D_z. Net result: unshielded should be louder.
	if unshieldedLevel.LrDay <= shieldedLevel.LrDay {
		t.Fatalf("unshielded (high receiver) should be louder: unshielded=%f shielded=%f",
			unshieldedLevel.LrDay, shieldedLevel.LrDay)
	}
}

// TestPropagation_Hochlage verifies that a road on an embankment is louder
// at a receiver not blocked by the embankment edge than one that is blocked.
func TestPropagation_Hochlage(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.ElevationM = 105.0 // road elevated 5 m above terrain

	// I7: Böschungskante at road level Y=55.3, Böschungsfuß at Y=62.8 terrain level.
	crest := TerrainEdge{
		ID:       "crest-i7",
		Geometry: []geo.Point3D{{X: -200, Y: 55.3, Z: 105.0}, {X: 400, Y: 55.3, Z: 105.0}},
	}
	foot := TerrainEdge{
		ID:       "foot-i7",
		Geometry: []geo.Point3D{{X: -200, Y: 62.8, Z: 100.0}, {X: 400, Y: 62.8, Z: 100.0}},
	}
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	cfg := DefaultPropagationConfig()
	cfg.ReceiverTerrainZ = 100.0
	cfg.Terrain = []TerrainProfile{profile}

	// IO1: low receiver (2.6 m above terrain at Z=102.6) — shielded by embankment.
	cfg.ReceiverHeightM = 2.6

	shieldedLevel, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 75}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("Hochlage shielded: %v", err)
	}

	// IO2: receiver at road height (5 m above terrain at Z=105) — just clears.
	cfg.ReceiverHeightM = 5.0
	cfg.ReceiverTerrainZ = 100.0

	higherLevel, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 75}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("Hochlage higher: %v", err)
	}

	// Higher receiver should not be shielded and thus louder.
	if higherLevel.LrDay <= shieldedLevel.LrDay {
		t.Fatalf("higher receiver should be louder: higher=%f shielded=%f",
			higherLevel.LrDay, shieldedLevel.LrDay)
	}
}

// TestPropagation_Ansteigende verifies that a rising road (Ansteigende Straße)
// produces valid results using per-vertex CenterlineElevations.
func TestPropagation_Ansteigende(t *testing.T) {
	t.Parallel()

	// Rising road: from Z=100 at X=250 to Z=122 at X=30 (matches TEST-20 I8 geometry).
	source := sampleSource()
	source.Centerline = []geo.Point2D{{X: 250, Y: 50}, {X: 30, Y: 50}}
	source.CenterlineElevations = []float64{100.0, 122.0}

	cfg := DefaultPropagationConfig()
	cfg.ReceiverTerrainZ = 100.0
	cfg.ReceiverHeightM = 5.5

	levels, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 110}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("ansteigende: %v", err)
	}

	// Day > night (traffic pattern).
	if levels.LrDay <= levels.LrNight {
		t.Fatalf("ansteigende: expected day > night: day=%f night=%f", levels.LrDay, levels.LrNight)
	}

	// Both should be finite (not -999 empty-sum sentinel).
	if levels.LrDay < -500 {
		t.Fatalf("ansteigende: day level unexpectedly low: %f", levels.LrDay)
	}
}

// TestPropagation_Ansteigende_HigherEndLouder verifies that the rising end of the
// road is louder for a nearby receiver on that end than the lower end.
func TestPropagation_Ansteigende_HigherEndLouder(t *testing.T) {
	t.Parallel()

	// Rising road from X=0,Z=100 to X=200,Z=120.
	source := sampleSource()
	source.Centerline = []geo.Point2D{{X: 0, Y: 0}, {X: 200, Y: 0}}
	source.CenterlineElevations = []float64{100.0, 120.0}

	cfgLow := DefaultPropagationConfig()
	cfgLow.ReceiverTerrainZ = 100.0
	cfgLow.ReceiverHeightM = 4.0

	// Receiver close to low end of road.
	levelLow, err := ComputeReceiverLevels(geo.Point2D{X: 10, Y: 20}, []RoadSource{source}, nil, cfgLow)
	if err != nil {
		t.Fatalf("low end: %v", err)
	}

	cfgHigh := DefaultPropagationConfig()
	cfgHigh.ReceiverTerrainZ = 120.0
	cfgHigh.ReceiverHeightM = 4.0

	// Receiver close to high end of road.
	levelHigh, err := ComputeReceiverLevels(geo.Point2D{X: 190, Y: 20}, []RoadSource{source}, nil, cfgHigh)
	if err != nil {
		t.Fatalf("high end: %v", err)
	}

	// Both finite and meaningful.
	if levelLow.LrDay < -500 || levelHigh.LrDay < -500 {
		t.Fatalf("levels unexpectedly low: low=%f high=%f", levelLow.LrDay, levelHigh.LrDay)
	}
}

// TestSplitLineIntoSegments_WithElevations verifies Z interpolation.
func TestSplitLineIntoSegments_WithElevations(t *testing.T) {
	t.Parallel()

	// Road rising from Z=100 to Z=120 over 100 m.
	line := []geo.Point2D{{X: 0, Y: 0}, {X: 100, Y: 0}}
	elevations := []float64{100.0, 120.0}

	segs := SplitLineIntoSegments(line, elevations, 10)
	if len(segs) != 10 {
		t.Fatalf("expected 10 segments, got %d", len(segs))
	}

	// First segment midpoint at distance 5 m → Z should be 100 + 5/100*20 = 101.
	if !almostEqual(segs[0].MidZ, 101.0, 0.01) {
		t.Fatalf("first segment MidZ: expected 101.0, got %f", segs[0].MidZ)
	}

	// Last segment midpoint at distance 95 m → Z should be 100 + 95/100*20 = 119.
	if !almostEqual(segs[len(segs)-1].MidZ, 119.0, 0.01) {
		t.Fatalf("last segment MidZ: expected 119.0, got %f", segs[len(segs)-1].MidZ)
	}
}

// TestPropagation_WegfuehrendeStrasse verifies that an angled (wegführende)
// road produces decreasing levels as the road leads away from the receiver.
// This is handled naturally by the Teilstueckverfahren geometry (no special
// topography features needed for a flat angled road).
func TestPropagation_WegfuehrendeStrasse(t *testing.T) {
	t.Parallel()

	// Road leading diagonally away: from (100,60) to (550,330) — TEST-20 I9 geometry.
	source := sampleSource()
	source.Centerline = []geo.Point2D{{X: 100, Y: 60}, {X: 550, Y: 330}}

	cfg := DefaultPropagationConfig()

	// IO1: close receiver.
	close1, err := ComputeReceiverLevels(geo.Point2D{X: 50, Y: 30}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("wegführende close: %v", err)
	}

	// IO2: farther receiver.
	far1, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 0}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("wegführende far: %v", err)
	}

	// Closer receiver should be louder.
	if close1.LrDay <= far1.LrDay {
		t.Fatalf("close receiver should be louder for wegführende: close=%f far=%f",
			close1.LrDay, far1.LrDay)
	}
}

// TestGroundCorrection_ProperFormula verifies the D_gr formula against TEST-20 I1.
func TestGroundCorrection_ProperFormula(t *testing.T) {
	t.Parallel()

	// I1 IO1: h_m=15.5, s_gr=101.98 → D_gr = 0 (formula yields negative).
	dgr1 := computeGroundCorrection(101.98, 15.5)
	if dgr1 != 0 {
		t.Fatalf("I1 IO1: expected D_gr=0 (clamped), got %f", dgr1)
	}

	// I1 IO2: h_m=3.0, s_gr=111.803 → D_gr ≈ 3.74.
	dgr2 := computeGroundCorrection(111.803, 3.0)
	if !almostEqual(dgr2, 3.74, 0.05) {
		t.Fatalf("I1 IO2: expected D_gr≈3.74, got %f", dgr2)
	}

	// I6 IO1: h_m=-0.104, s_gr=111.939 → D_gr ≈ 4.84.
	dgr3 := computeGroundCorrection(111.939, -0.104)
	if !almostEqual(dgr3, 4.84, 0.05) {
		t.Fatalf("I6 IO1: expected D_gr≈4.84, got %f", dgr3)
	}
}

// --- building / courtyard tests ---

func TestBuilding_Validate(t *testing.T) {
	t.Parallel()

	valid := Building{
		ID:        "bldg-1",
		Footprint: []geo.Point2D{{X: 0, Y: 10}, {X: 10, Y: 10}, {X: 10, Y: 15}, {X: 0, Y: 15}},
		HeightM:   8.0,
	}

	err := valid.Validate()
	if err != nil {
		t.Fatalf("expected valid building, got %v", err)
	}

	// Missing ID.
	b := valid
	b.ID = ""

	err = b.Validate()
	if err == nil {
		t.Fatal("expected error for missing ID")
	}

	// Too few vertices (need at least 3 for a polygon).
	b = valid
	b.Footprint = []geo.Point2D{{X: 0, Y: 0}, {X: 10, Y: 0}}

	err = b.Validate()
	if err == nil {
		t.Fatal("expected error for fewer than 3 footprint vertices")
	}

	// Zero height.
	b = valid
	b.HeightM = 0

	err = b.Validate()
	if err == nil {
		t.Fatal("expected error for zero height")
	}

	// Negative reflection loss.
	b = valid
	b.ReflectionLossDB = -1

	err = b.Validate()
	if err == nil {
		t.Fatal("expected error for negative reflection loss")
	}
}

func TestBuilding_AsBarrier_ClosedPolygon(t *testing.T) {
	t.Parallel()

	// 4-vertex rectangle — asBarrier should close it to 5 points.
	b := Building{
		ID:        "rect",
		Footprint: []geo.Point2D{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 5}, {X: 0, Y: 5}},
		HeightM:   8.0,
	}

	barrier := b.asBarrier()

	if barrier.HeightM != 8.0 {
		t.Fatalf("expected height 8.0, got %f", barrier.HeightM)
	}

	// Closed polygon: 4 vertices + closing vertex = 5 points.
	if len(barrier.Geometry) != 5 {
		t.Fatalf("expected 5 geometry points (closed), got %d", len(barrier.Geometry))
	}

	// Closing point equals first point.
	first, last := barrier.Geometry[0], barrier.Geometry[4]
	if first.X != last.X || first.Y != last.Y {
		t.Fatalf("barrier polygon not closed: first=%v last=%v", first, last)
	}
}

func TestBuilding_AsBarrier_AlreadyClosedPolygon(t *testing.T) {
	t.Parallel()

	// Pre-closed polygon must not add a duplicate point.
	b := Building{
		ID: "pre-closed",
		Footprint: []geo.Point2D{
			{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 5}, {X: 0, Y: 5}, {X: 0, Y: 0},
		},
		HeightM: 8.0,
	}

	barrier := b.asBarrier()

	if len(barrier.Geometry) != 5 {
		t.Fatalf("expected 5 points for pre-closed polygon, got %d", len(barrier.Geometry))
	}
}

func TestBuilding_AsReflector_Properties(t *testing.T) {
	t.Parallel()

	b := Building{
		ID:               "facade",
		Footprint:        []geo.Point2D{{X: 0, Y: 20}, {X: 10, Y: 20}, {X: 10, Y: 25}, {X: 0, Y: 25}},
		HeightM:          6.0,
		ReflectionLossDB: 2.0,
	}

	refl := b.asReflector()

	if refl.HeightM != 6.0 {
		t.Fatalf("expected reflector height 6.0, got %f", refl.HeightM)
	}

	if refl.ReflectionLossDB != 2.0 {
		t.Fatalf("expected reflection loss 2.0, got %f", refl.ReflectionLossDB)
	}

	// Closed polygon: 4 + 1 closing = 5 points.
	if len(refl.Geometry) != 5 {
		t.Fatalf("expected 5 geometry points, got %d", len(refl.Geometry))
	}
}

// TestBuilding_ShieldsDirectPath verifies that a building standing between
// source and receiver reduces the receiver level compared to the same geometry
// acting only as a reflector (no barrier shielding).
//
// Note: the image-source method can produce phantom reflections off interior
// faces of a building polygon (e.g. north↔south bounce inside a thin building
// body). Both test scenarios use the same reflector geometry, so phantom paths
// cancel out. The difference is purely from the barrier shielding component.
//
// Geometry: source near (0,0), receiver at (0,100), building at y=20..25 (h=10m).
// The building's south wall at y=20 crosses the direct source-receiver path.
func TestBuilding_ShieldsDirectPath(t *testing.T) {
	t.Parallel()

	source := RoadSource{
		ID:           "src",
		Centerline:   []geo.Point2D{{X: -1, Y: 0}, {X: 1, Y: 0}},
		SurfaceType:  SurfaceSMA,
		Speeds:       SpeedInput{PkwKPH: 100, Lkw1KPH: 80, Lkw2KPH: 70, KradKPH: 100},
		TrafficDay:   TrafficInput{PkwPerHour: 900, Lkw1PerHour: 40, Lkw2PerHour: 60, KradPerHour: 10},
		TrafficNight: TrafficInput{PkwPerHour: 200, Lkw1PerHour: 10, Lkw2PerHour: 20, KradPerHour: 2},
	}
	receiver := geo.Point2D{X: 0, Y: 100}

	bldg := Building{
		ID:        "blocker",
		Footprint: []geo.Point2D{{X: -5, Y: 20}, {X: 5, Y: 20}, {X: 5, Y: 25}, {X: -5, Y: 25}},
		HeightM:   10.0,
	}

	// Reflector-only: same geometry, same reflected paths, but no barrier shielding.
	cfgReflOnly := DefaultPropagationConfig()
	cfgReflOnly.Reflectors = []Reflector{bldg.asReflector()}

	lvlReflOnly, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgReflOnly)
	if err != nil {
		t.Fatalf("refl-only: %v", err)
	}

	// Building: same reflections PLUS barrier shielding of the direct path.
	cfgBuilding := DefaultPropagationConfig()
	cfgBuilding.Buildings = []Building{bldg}

	lvlBuilding, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgBuilding)
	if err != nil {
		t.Fatalf("building: %v", err)
	}

	// Building (shielding active) must be quieter than reflector-only (no shielding).
	if lvlBuilding.LrDay >= lvlReflOnly.LrDay {
		t.Fatalf("building with shielding should be lower than reflector-only: "+
			"building=%.2f refl-only=%.2f", lvlBuilding.LrDay, lvlReflOnly.LrDay)
	}
}

// TestBuilding_ParallelFacade_IncreasesLevel verifies that a building facade
// parallel to the road (house-front scenario) increases the receiver level by
// adding a reflected path.
//
// Geometry: road along x-axis, receiver at (0,30), building at y=45..50.
// The building's south face at y=45 reflects road sound back to the receiver.
func TestBuilding_ParallelFacade_IncreasesLevel(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 30}

	cfg := DefaultPropagationConfig()

	lvlFree, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("free field: %v", err)
	}

	// Building with south facade at y=45 (behind receiver at y=30).
	bldg := Building{
		ID:        "house-front",
		Footprint: []geo.Point2D{{X: -20, Y: 45}, {X: 20, Y: 45}, {X: 20, Y: 50}, {X: -20, Y: 50}},
		HeightM:   8.0,
	}
	cfg.Buildings = []Building{bldg}

	lvlWithBldg, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("with building: %v", err)
	}

	if lvlWithBldg.LrDay <= lvlFree.LrDay {
		t.Fatalf("parallel facade should increase level: free=%.2f with-bldg=%.2f",
			lvlFree.LrDay, lvlWithBldg.LrDay)
	}
}

// TestBuilding_Courtyard_IncreasesLevel verifies that a U-shaped courtyard
// (buildings on north, east, and west) raises the receiver level compared to
// an open field receiver at the same position. This is the "Hinterhof" scenario.
//
// Geometry:
//
//	Road: x=-50..50, y=0 (sampleSource)
//	Receiver: (0, 40) — inside the courtyard opening
//	North building: south face at y=60 (reflects back)
//	East building:  west face at x=20  (reflects inward)
//	West building:  east face at x=-20 (reflects inward)
func TestBuilding_Courtyard_IncreasesLevel(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 40}

	cfg := DefaultPropagationConfig()

	lvlFree, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("free field: %v", err)
	}

	cfg.Buildings = []Building{
		// North wall of the courtyard — reflects sound back toward road.
		{
			ID:        "north-wall",
			Footprint: []geo.Point2D{{X: -20, Y: 60}, {X: 20, Y: 60}, {X: 20, Y: 65}, {X: -20, Y: 65}},
			HeightM:   10.0,
		},
		// East wall of the courtyard — reflects inward.
		{
			ID:        "east-wall",
			Footprint: []geo.Point2D{{X: 20, Y: 0}, {X: 25, Y: 0}, {X: 25, Y: 65}, {X: 20, Y: 65}},
			HeightM:   10.0,
		},
		// West wall of the courtyard — reflects inward.
		{
			ID:        "west-wall",
			Footprint: []geo.Point2D{{X: -25, Y: 0}, {X: -20, Y: 0}, {X: -20, Y: 65}, {X: -25, Y: 65}},
			HeightM:   10.0,
		},
	}

	lvlCourtyard, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("courtyard: %v", err)
	}

	if lvlCourtyard.LrDay <= lvlFree.LrDay {
		t.Fatalf("courtyard should increase level: free=%.2f courtyard=%.2f",
			lvlFree.LrDay, lvlCourtyard.LrDay)
	}
}

// --- reflection tests ---

func TestReflector_Validate(t *testing.T) {
	t.Parallel()

	valid := Reflector{
		ID:       "wall-1",
		Geometry: []geo.Point2D{{X: 15, Y: -10}, {X: 15, Y: 10}},
		HeightM:  8.0,
	}

	err := valid.Validate()
	if err != nil {
		t.Fatalf("expected valid reflector, got %v", err)
	}

	// Missing ID.
	r := valid
	r.ID = ""

	err = r.Validate()
	if err == nil {
		t.Fatal("expected error for missing ID")
	}

	// Too few points.
	r = valid
	r.Geometry = []geo.Point2D{{X: 0, Y: 0}}

	err = r.Validate()
	if err == nil {
		t.Fatal("expected error for single-point geometry")
	}

	// Zero height.
	r = valid
	r.HeightM = 0

	err = r.Validate()
	if err == nil {
		t.Fatal("expected error for zero height")
	}

	// Negative reflection loss.
	r = valid
	r.ReflectionLossDB = -1

	err = r.Validate()
	if err == nil {
		t.Fatal("expected error for negative reflection loss")
	}
}

func TestMirrorPoint_PerpendicularWall(t *testing.T) {
	t.Parallel()

	// Mirror (0, 0) across the vertical wall at x=10.
	img := mirrorPoint(
		geo.Point2D{X: 0, Y: 0},
		geo.Point2D{X: 10, Y: -5},
		geo.Point2D{X: 10, Y: 5},
	)
	if !almostEqual(img.X, 20, 1e-9) || !almostEqual(img.Y, 0, 1e-9) {
		t.Fatalf("expected (20, 0), got (%f, %f)", img.X, img.Y)
	}
}

func TestMirrorPoint_AngledWall(t *testing.T) {
	t.Parallel()

	// Mirror (0, 2) across the line y=x (wall from origin to (1,1)).
	// Mirror of (0,2) across y=x is (2,0).
	img := mirrorPoint(
		geo.Point2D{X: 0, Y: 2},
		geo.Point2D{X: 0, Y: 0},
		geo.Point2D{X: 1, Y: 1},
	)
	if !almostEqual(img.X, 2, 1e-6) || !almostEqual(img.Y, 0, 1e-6) {
		t.Fatalf("expected (2, 0), got (%f, %f)", img.X, img.Y)
	}
}

// TestComputeReflectedPaths_SingleReflection verifies the image-source path
// distance for a simple reflection off a perpendicular wall.
//
// Geometry: source (0,0), receiver (10,0), wall at x=15 (y ∈ [-20,20]).
// Image source: (30, 0); reflected plan distance = 20 m.
func TestComputeReflectedPaths_SingleReflection(t *testing.T) {
	t.Parallel()

	wall := Reflector{
		ID:       "wall",
		Geometry: []geo.Point2D{{X: 15, Y: -20}, {X: 15, Y: 20}},
		HeightM:  8,
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 10, Y: 0}, 4.0,
		[]Reflector{wall},
	)

	if len(paths) != 1 {
		t.Fatalf("expected 1 reflected path, got %d", len(paths))
	}
	// Image source = (30,0), receiver = (10,0) → plan dist = 20.
	if !almostEqual(paths[0].planDistM, 20.0, 1e-6) {
		t.Fatalf("expected plan dist 20.0 m, got %f", paths[0].planDistM)
	}
	// Default reflection loss = 1.0 dB.
	if !almostEqual(paths[0].lossDB, 1.0, 1e-9) {
		t.Fatalf("expected reflection loss 1.0 dB, got %f", paths[0].lossDB)
	}
}

// TestComputeReflectedPaths_WallSegmentMissed verifies that no reflection is
// returned when the reflected ray crosses the infinite wall line but misses
// the actual wall segment.
//
// Geometry: source (0,0), receiver (10,0), wall segment from (15,5) to (15,20).
// Reflection point would land at (15,0) — below the wall segment → no hit.
func TestComputeReflectedPaths_WallSegmentMissed(t *testing.T) {
	t.Parallel()

	wall := Reflector{
		ID:       "partial-wall",
		Geometry: []geo.Point2D{{X: 15, Y: 5}, {X: 15, Y: 20}},
		HeightM:  8,
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 10, Y: 0}, 4.0,
		[]Reflector{wall},
	)

	if len(paths) != 0 {
		t.Fatalf("expected 0 reflected paths, got %d", len(paths))
	}
}

// TestComputeReflectedPaths_NoReflectors verifies empty input returns no paths.
func TestComputeReflectedPaths_NoReflectors(t *testing.T) {
	t.Parallel()

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 10, Y: 0}, 4.0,
		nil,
	)
	if len(paths) != 0 {
		t.Fatalf("expected 0 paths for nil reflectors, got %d", len(paths))
	}
}

// TestComputeReflectedPaths_DoubleReflection_Corner verifies that two
// perpendicular walls generate single reflections off each wall plus one
// valid second-order reflection (A→B only; B→A is geometrically invalid here).
//
// Geometry:
//
//	Wall A: x=15 (y ∈ [-20,20])
//	Wall B: y=12 (x ∈ [-20,20])
//	Source: (0, 0), Receiver: (5, 5)
//
// Expected paths: 1st-A, 1st-B, 2nd-A-then-B (total 3).
// 2nd-B-then-A is invalid because the back-leg check fails.
func TestComputeReflectedPaths_DoubleReflection_Corner(t *testing.T) {
	t.Parallel()

	wallA := Reflector{
		ID:       "wall-a",
		Geometry: []geo.Point2D{{X: 15, Y: -20}, {X: 15, Y: 20}},
		HeightM:  8,
	}
	wallB := Reflector{
		ID:       "wall-b",
		Geometry: []geo.Point2D{{X: -20, Y: 12}, {X: 20, Y: 12}},
		HeightM:  8,
	}
	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 5, Y: 5}, 4.0,
		[]Reflector{wallA, wallB},
	)

	if len(paths) != 3 {
		t.Fatalf("expected 3 reflected paths (1st-A, 1st-B, 2nd-A-then-B), got %d", len(paths))
	}

	// The double-reflection plan distance = dist2D(S''=(30,24), R=(5,5)) = sqrt(986).
	expectedDouble := math.Sqrt(986.0)
	maxDist := 0.0
	maxLoss := 0.0

	for _, p := range paths {
		if p.planDistM > maxDist {
			maxDist = p.planDistM
			maxLoss = p.lossDB
		}
	}

	if !almostEqual(maxDist, expectedDouble, 0.01) {
		t.Fatalf("expected double-reflection plan dist ≈ %f, got %f", expectedDouble, maxDist)
	}
	// Double reflection loss = 1.0 + 1.0 = 2.0 dB.
	if !almostEqual(maxLoss, 2.0, 1e-9) {
		t.Fatalf("expected double-reflection loss 2.0 dB, got %f", maxLoss)
	}
}

// TestComputeReceiverLevels_ReflectionIncreasesLevel verifies that adding a
// reflector wall behind the road increases the receiver level.
//
// Geometry: road along x-axis (-50 to 50), receiver at (0, 50),
// reflector wall at y=-10 (behind road). Every segment has a valid reflection.
func TestComputeReceiverLevels_ReflectionIncreasesLevel(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 50}

	cfg := DefaultPropagationConfig()

	lvlNoRefl, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("no reflector: %v", err)
	}

	cfg.Reflectors = []Reflector{{
		ID:       "back-wall",
		Geometry: []geo.Point2D{{X: -200, Y: -10}, {X: 200, Y: -10}},
		HeightM:  8,
	}}

	lvlWithRefl, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("with reflector: %v", err)
	}

	if lvlWithRefl.LrDay <= lvlNoRefl.LrDay {
		t.Fatalf("reflection should increase level: no-refl=%.2f dB, with-refl=%.2f dB",
			lvlNoRefl.LrDay, lvlWithRefl.LrDay)
	}
}

// TestComputeReceiverLevels_ReflectionCustomLoss verifies that a higher
// reflection loss results in a smaller level increase than default.
func TestComputeReceiverLevels_ReflectionCustomLoss(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 50}

	cfgDefault := DefaultPropagationConfig()
	cfgDefault.Reflectors = []Reflector{{
		ID:       "back-wall",
		Geometry: []geo.Point2D{{X: -200, Y: -10}, {X: 200, Y: -10}},
		HeightM:  8,
		// ReflectionLossDB = 0 → uses default 1.0 dB
	}}

	cfgHighLoss := DefaultPropagationConfig()
	cfgHighLoss.Reflectors = []Reflector{{
		ID:               "back-wall",
		Geometry:         []geo.Point2D{{X: -200, Y: -10}, {X: 200, Y: -10}},
		HeightM:          8,
		ReflectionLossDB: 5.0, // absorptive surface
	}}

	lvlDefault, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgDefault)
	if err != nil {
		t.Fatalf("default loss: %v", err)
	}

	lvlHighLoss, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgHighLoss)
	if err != nil {
		t.Fatalf("high loss: %v", err)
	}

	if lvlHighLoss.LrDay >= lvlDefault.LrDay {
		t.Fatalf("higher reflection loss should reduce level increase: default=%.2f high=%.2f",
			lvlDefault.LrDay, lvlHighLoss.LrDay)
	}
}

// K5: Reflection height condition tests.
//
// A reflector only produces a valid reflected path when the wall is tall enough
// that the reflected ray does not pass over it. At the reflection point P the
// ray height is interpolated linearly between sourceZ and receiverZ:
//
//	t          = dist2D(imageSource, P) / dist2D(imageSource, receiver)
//	heightAtP  = sourceZ + (receiverZ − sourceZ) · t
//
// The reflection is valid only when wall.HeightM >= heightAtP.

// TestComputeReflectedPaths_HeightTooShort_NoReflection verifies that a
// geometrically plausible wall that is too short to intercept the ray produces
// no reflected path.
//
// Geometry: source (0,0) z=0.5 m, receiver (10,0) z=4.0 m,
// wall at x=15 (y ∈ [−5, 5]), height=2.0 m.
//
// Image source S′=(30,0).  Reflection point P=(15,0).
// t = dist(S′,P)/dist(S′,R) = 15/20 = 0.75.
// heightAtP = 0.5 + 3.5·0.75 = 3.125 m.
// 2.0 < 3.125 → ray passes over the wall → no reflection.
func TestComputeReflectedPaths_HeightTooShort_NoReflection(t *testing.T) {
	t.Parallel()

	wall := Reflector{
		ID:       "short-wall",
		Geometry: []geo.Point2D{{X: 15, Y: -5}, {X: 15, Y: 5}},
		HeightM:  2.0, // shorter than the 3.125 m ray height at reflection point
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 10, Y: 0}, 4.0,
		[]Reflector{wall},
	)

	if len(paths) != 0 {
		t.Fatalf("expected 0 reflected paths (wall too short), got %d", len(paths))
	}
}

// TestComputeReflectedPaths_HeightJustEnough_Reflection verifies that the same
// wall geometry with sufficient height produces one reflected path.
//
// Same geometry as above but wall height=5.0 m >= 3.125 m → reflection valid.
func TestComputeReflectedPaths_HeightJustEnough_Reflection(t *testing.T) {
	t.Parallel()

	wall := Reflector{
		ID:       "tall-wall",
		Geometry: []geo.Point2D{{X: 15, Y: -5}, {X: 15, Y: 5}},
		HeightM:  5.0, // 5.0 >= 3.125 → valid reflection
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 10, Y: 0}, 4.0,
		[]Reflector{wall},
	)

	if len(paths) != 1 {
		t.Fatalf("expected 1 reflected path (wall tall enough), got %d", len(paths))
	}

	// Image source S′=(30,0), receiver=(10,0) → plan dist = 20 m.
	if !almostEqual(paths[0].planDistM, 20.0, 1e-6) {
		t.Fatalf("expected plan dist 20.0 m, got %f", paths[0].planDistM)
	}
}

// TestComputeReflectedPaths_DoubleReflection_SecondWallTooShort verifies that
// when the second wall in a two-bounce path is too short to reflect the ray at
// its reflection point, the double reflection is suppressed. Single reflections
// that independently satisfy the height condition remain valid.
//
// Geometry: source (0,0) z=0.5, receiver (5,5) z=4.0.
// wallA at x=15 (y ∈ [−20,20]), height=8 m (tall enough for 1st bounce).
// wallB at y=12 (x ∈ [−20,20]), height=1 m (too short for both 1st-order
// off wallB and for the 2nd leg of A→B).
//
// Expected: only 1 path (1st-order off wallA). The 1st-order off wallB and
// the 2nd-order A→B path are suppressed by the height condition.
func TestComputeReflectedPaths_DoubleReflection_SecondWallTooShort(t *testing.T) {
	t.Parallel()

	wallA := Reflector{
		ID:       "wall-a",
		Geometry: []geo.Point2D{{X: 15, Y: -20}, {X: 15, Y: 20}},
		HeightM:  8, // tall: 1st-order off A valid (heightAtP ≈ 2.6 m)
	}
	wallB := Reflector{
		ID:       "wall-b",
		Geometry: []geo.Point2D{{X: -20, Y: 12}, {X: 20, Y: 12}},
		HeightM:  1, // short: 1st-order off B invalid (heightAtP ≈ 2.7 m) and
		// 2nd-order A→B invalid (heightAtP2 ≈ 2.7 m)
	}

	paths := computeReflectedPaths(
		geo.Point2D{X: 0, Y: 0}, 0.5,
		geo.Point2D{X: 5, Y: 5}, 4.0,
		[]Reflector{wallA, wallB},
	)

	if len(paths) != 1 {
		t.Fatalf("expected 1 reflected path (1st-order off wall-a only), got %d", len(paths))
	}
}

// TestComputeReceiverLevels_ShortReflector_NoLevelIncrease verifies that a
// reflector too short to intercept any reflected ray does not increase the
// receiver level compared to free-field propagation.
//
// Geometry: road along x-axis, receiver at (0,50), reflector at y=−10 with
// height=0.6 m. The reflected ray height at the wall is ~0.52 m (very close to
// source height) for nearby segments, but increases for distant segments.
// We use a very short wall (0.6 m) so at least for a close segment the height
// condition can fail; for a fully opaque wall (height=30 m) it must fail for all.
func TestComputeReceiverLevels_ShortReflector_NoLevelIncrease(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	receiver := geo.Point2D{X: 0, Y: 50}

	cfgFree := DefaultPropagationConfig()

	lvlFree, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgFree)
	if err != nil {
		t.Fatalf("free field: %v", err)
	}

	// A reflector that is far shorter than any ray height at the reflection point.
	cfgShort := DefaultPropagationConfig()
	cfgShort.Reflectors = []Reflector{{
		ID:       "too-short",
		Geometry: []geo.Point2D{{X: -200, Y: -10}, {X: 200, Y: -10}},
		HeightM:  0.1, // essentially at ground level — no ray can reflect off this
	}}

	lvlShort, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfgShort)
	if err != nil {
		t.Fatalf("short reflector: %v", err)
	}

	// A wall so short that no reflected ray can reach it → level must not increase.
	if lvlShort.LrDay > lvlFree.LrDay+0.01 {
		t.Fatalf("short reflector should not increase level: free=%.2f dB, short=%.2f dB",
			lvlFree.LrDay, lvlShort.LrDay)
	}
}

// TestPropagation_ShieldedNoGroundEffect verifies that when shielded (D_z > 0)
// the ground effect (D_gr) is replaced by D_z, not added on top.
func TestPropagation_ShieldedNoGroundEffect(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()
	receiver := geo.Point2D{X: 0, Y: 50}

	// Free-field level (D_gr applies).
	freeField, err := ComputeReceiverLevels(receiver, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("free field: %v", err)
	}

	// Level with a very tall barrier (effectively full shielding, D_z >> D_gr).
	tallBarrier := Barrier{
		ID:       "tall",
		Geometry: []geo.Point2D{{X: -100, Y: 10}, {X: 100, Y: 10}},
		HeightM:  20.0,
	}

	withBarrier, err := ComputeReceiverLevels(receiver, []RoadSource{source}, []Barrier{tallBarrier}, cfg)
	if err != nil {
		t.Fatalf("with barrier: %v", err)
	}

	// Barrier should significantly attenuate.
	reduction := freeField.LrDay - withBarrier.LrDay
	if reduction < 10 {
		t.Fatalf("tall barrier should attenuate by >10 dB, got %f dB", reduction)
	}
}

// --- RLS-19 Section 3.3.6: Längsneigungskorrektur (Eqs. 7a / 7b / 7c) ---
//
// Normative formulas are speed-dependent.  The existing GradientCorrection
// function accepts no speed parameter and therefore cannot implement the
// correct formulas.  These tests encode the exact Eq. 7a/7b/7c values and
// are expected to FAIL until GradientCorrection is updated.

func TestGradientCorrection_Eq7a_Pkw_Uphill(t *testing.T) {
	t.Parallel()

	// Eq. 7a for g > +2: D = (g-2)/10 * (v_Pkw+70)/100
	// g=8, v=100: (8-2)/10 * (100+70)/100 = 0.6 * 1.7 = 1.020
	want := (8.0-2.0)/10.0 * (100.0+70.0)/100.0
	got := GradientCorrection(8, Pkw, 100)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(8, Pkw, 100): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Eq7a_Pkw_Uphill_LowSpeed(t *testing.T) {
	t.Parallel()

	// g=4, v=50: just above threshold → (4-2)/10 * (50+70)/100 = 0.2 * 1.2 = 0.240
	want := (4.0-2.0)/10.0 * (50.0+70.0)/100.0
	got := GradientCorrection(4, Pkw, 50)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(4, Pkw, 50): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Eq7a_Pkw_Downhill(t *testing.T) {
	t.Parallel()

	// Eq. 7a for g < -6: D = (g+6)/(-6) * (90-min(v_Pkw,70))/20
	// g=-8, v=100: (-8+6)/(-6) * (90-70)/20 = (1/3) * 1 = 0.333...
	want := (-8.0+6.0)/(-6.0) * (90.0-70.0)/20.0
	got := GradientCorrection(-8, Pkw, 100)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(-8, Pkw, 100): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Eq7a_Pkw_Flat(t *testing.T) {
	t.Parallel()

	// |g| <= 2 for Pkw → 0 at any speed.
	for _, v := range []float64{30, 60, 100, 130} {
		got := GradientCorrection(1, Pkw, v)
		if got != 0 {
			t.Fatalf("GradientCorrection(1, Pkw, %.0f): want 0, got %.4f", v, got)
		}
	}
}

func TestGradientCorrection_Eq7b_Lkw1_Uphill(t *testing.T) {
	t.Parallel()

	// Eq. 7b for g > +2: D = (g-2)/10 * v_Lkw1/10
	// g=8, v=80: (8-2)/10 * 80/10 = 0.6 * 8 = 4.800
	want := (8.0-2.0)/10.0 * 80.0/10.0
	got := GradientCorrection(8, Lkw1, 80)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(8, Lkw1, 80): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Eq7b_Lkw1_Downhill(t *testing.T) {
	t.Parallel()

	// Eq. 7b for g < -4: D = (g+4)/(-8) * (v_Lkw1-20)/10
	// g=-6, v=80: (-6+4)/(-8) * (80-20)/10 = 0.25 * 6 = 1.500
	want := (-6.0+4.0)/(-8.0) * (80.0-20.0)/10.0
	got := GradientCorrection(-6, Lkw1, 80)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(-6, Lkw1, 80): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Eq7c_Lkw2_Uphill(t *testing.T) {
	t.Parallel()

	// Eq. 7c for g > +2: D = (g-2)/10 * (v_Lkw2+10)/10
	// g=8, v=70: (8-2)/10 * (70+10)/10 = 0.6 * 8 = 4.800
	want := (8.0-2.0)/10.0 * (70.0+10.0)/10.0
	got := GradientCorrection(8, Lkw2, 70)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(8, Lkw2, 70): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Eq7c_Lkw2_Downhill(t *testing.T) {
	t.Parallel()

	// Eq. 7c for g < -4: D = (g+4)/(-8) * (v_Lkw2-10)/10
	// g=-6, v=70: (-6+4)/(-8) * (70-10)/10 = 0.25 * 6 = 1.500
	want := (-6.0+4.0)/(-8.0) * (70.0-10.0)/10.0
	got := GradientCorrection(-6, Lkw2, 70)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("GradientCorrection(-6, Lkw2, 70): want %.4f, got %.4f", want, got)
	}
}

func TestGradientCorrection_Clamped_At12(t *testing.T) {
	t.Parallel()

	// Gradients beyond ±12% use ±12% values.
	if GradientCorrection(15, Lkw2, 70) != GradientCorrection(12, Lkw2, 70) {
		t.Fatal("gradient should be clamped at +12%")
	}

	if GradientCorrection(-15, Pkw, 100) != GradientCorrection(-12, Pkw, 100) {
		t.Fatal("gradient should be clamped at -12%")
	}
}

// --- RLS-19 Section 3.3.7: Knotenpunktkorrektur (Eq. 8 / Tabelle 5) ---
//
// Eq. 8: D_{KKT}(x) = K_KT * max(1 - x/120, 0)
// Tabelle 5: signalized K_KT=3, roundabout K_KT=2, other K_KT=0.
// The step-table currently in JunctionCorrection does not implement this formula.

func TestJunctionCorrection_Eq8_Signalized_At0m(t *testing.T) {
	t.Parallel()

	// x=0: K_KT * (1 - 0) = 3 * 1 = 3.0
	got := JunctionCorrection(JunctionSignalized, 0)
	if !almostEqual(got, 3.0, 0.001) {
		t.Fatalf("JunctionCorrection(Signalized, 0): want 3.000, got %.4f", got)
	}
}

func TestJunctionCorrection_Eq8_Signalized_ContinuousAt60m(t *testing.T) {
	t.Parallel()

	// x=60: 3 * (1 - 60/120) = 3 * 0.5 = 1.5
	got := JunctionCorrection(JunctionSignalized, 60)
	if !almostEqual(got, 1.5, 0.001) {
		t.Fatalf("JunctionCorrection(Signalized, 60): want 1.500, got %.4f", got)
	}
}

func TestJunctionCorrection_Eq8_Signalized_At120m(t *testing.T) {
	t.Parallel()

	// x=120: 3 * (1 - 1) = 0
	got := JunctionCorrection(JunctionSignalized, 120)
	if !almostEqual(got, 0.0, 0.001) {
		t.Fatalf("JunctionCorrection(Signalized, 120): want 0.000, got %.4f", got)
	}
}

func TestJunctionCorrection_Eq8_Roundabout_At40m(t *testing.T) {
	t.Parallel()

	// x=40: 2 * (1 - 40/120) = 2 * (2/3) ≈ 1.333
	want := 2.0 * (1.0 - 40.0/120.0)
	got := JunctionCorrection(JunctionRoundabout, 40)
	if !almostEqual(got, want, 0.001) {
		t.Fatalf("JunctionCorrection(Roundabout, 40): want %.4f, got %.4f", want, got)
	}
}

func TestJunctionCorrection_Eq8_Other_AlwaysZero(t *testing.T) {
	t.Parallel()

	// K_KT=0 for sonstige Knotenpunkte → always 0.
	for _, x := range []float64{0, 10, 50, 120} {
		got := JunctionCorrection(JunctionOther, x)
		if got != 0 {
			t.Fatalf("JunctionCorrection(Other, %.0f): want 0, got %.4f", x, got)
		}
	}
}
