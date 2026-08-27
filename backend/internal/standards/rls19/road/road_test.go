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

// TestMultipleReflectionSurcharge verifies the Mehrfachreflexionszuschlag
// formula D_refl = min(2·h_Beb/w, 1.6) per RLS-19 Eq. 9.
func TestMultipleReflectionSurcharge(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		h, w   float64
		wantDB float64
	}{
		{"no buildings", 0, 10, 0},
		{"no width given", 5, 0, 0},
		{"both zero", 0, 0, 0},
		{"below clamp h=1 w=10", 1, 10, 0.2},
		{"below clamp h=5 w=10", 5, 10, 1.0},
		{"at clamp h=8 w=10", 8, 10, 1.6},
		{"above clamp capped to 1.6", 12, 10, 1.6},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := MultipleReflectionSurcharge(tc.h, tc.w)
			if !almostEqual(got, tc.wantDB, 1e-9) {
				t.Fatalf("MultipleReflectionSurcharge(h=%g, w=%g) = %g, want %g", tc.h, tc.w, got, tc.wantDB)
			}
		})
	}
}

func TestEmission_ReflectionSurcharge(t *testing.T) {
	t.Parallel()

	base := sampleSource()

	baseResult, err := ComputeEmission(base)
	if err != nil {
		t.Fatalf("compute base: %v", err)
	}

	// h=5, w=10 → D_refl = min(2·5/10, 1.6) = 1.0 dB.
	withRefl := sampleSource()
	withRefl.BuildingHeightM = 5
	withRefl.StreetWidthM = 10

	reflResult, err := ComputeEmission(withRefl)
	if err != nil {
		t.Fatalf("compute with reflection: %v", err)
	}

	diff := reflResult.LmEDay - baseResult.LmEDay
	if !almostEqual(diff, 1.0, 0.01) {
		t.Fatalf("reflection surcharge should add 1.0 dB: got diff=%f", diff)
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
