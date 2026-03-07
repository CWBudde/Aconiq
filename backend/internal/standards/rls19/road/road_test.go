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
	// SMA/AB should be 0 for Pkw.
	if got := SurfaceCorrection(SurfaceSMA, Pkw); got != 0 {
		t.Fatalf("SMA/Pkw correction: expected 0, got %f", got)
	}
	// OPA should be negative (quieter).
	if got := SurfaceCorrection(SurfaceOPA, Pkw); got >= 0 {
		t.Fatalf("OPA/Pkw correction should be negative, got %f", got)
	}
	// Paving should be positive (louder).
	if got := SurfaceCorrection(SurfacePaving, Pkw); got <= 0 {
		t.Fatalf("Paving/Pkw correction should be positive, got %f", got)
	}
	// Unknown surface returns 0.
	if got := SurfaceCorrection("unknown", Pkw); got != 0 {
		t.Fatalf("unknown surface should return 0, got %f", got)
	}
}

func TestGradientCorrection(t *testing.T) {
	t.Parallel()
	// Flat road: no correction for any group.
	for _, vg := range AllVehicleGroups() {
		if got := GradientCorrection(0, vg); got != 0 {
			t.Fatalf("flat road %s: expected 0, got %f", vg, got)
		}
	}
	// Steep uphill: heavy trucks get largest correction.
	lkw2Up := GradientCorrection(8, Lkw2)

	pkwUp := GradientCorrection(8, Pkw)
	if lkw2Up <= pkwUp {
		t.Fatalf("expected Lkw2 gradient correction > Pkw: Lkw2=%f Pkw=%f", lkw2Up, pkwUp)
	}
	// Downhill: Lkw2 gets negative correction.
	lkw2Down := GradientCorrection(-6, Lkw2)
	if lkw2Down >= 0 {
		t.Fatalf("expected negative downhill correction for Lkw2, got %f", lkw2Down)
	}
	// Clamped at +/-12.
	if GradientCorrection(15, Lkw2) != GradientCorrection(12, Lkw2) {
		t.Fatal("gradient should be clamped at 12%")
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

	segs := SplitLineIntoSegments(line, 10)
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
}

func TestSplitLineIntoSegments_ShortLine(t *testing.T) {
	t.Parallel()

	line := []geo.Point2D{{X: 0, Y: 0}, {X: 3, Y: 0}}

	segs := SplitLineIntoSegments(line, 10)
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

	segs := SplitLineIntoSegments(line, 10)
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
	segs1 := SplitLineIntoSegments(line, 7)

	segs2 := SplitLineIntoSegments(line, 7)
	if len(segs1) != len(segs2) {
		t.Fatal("segments should be deterministic")
	}

	for i := range segs1 {
		if segs1[i].MidPoint != segs2[i].MidPoint || segs1[i].LengthM != segs2[i].LengthM {
			t.Fatalf("segment %d differs between runs", i)
		}
	}
}

// --- propagation tests ---

func TestPropagation_DecreasesWithDistance(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()

	near, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 5}, []RoadSource{source}, cfg)
	if err != nil {
		t.Fatalf("compute near: %v", err)
	}

	far, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 200}, []RoadSource{source}, cfg)
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

	levels, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 25}, []RoadSource{source}, cfg)
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

	single, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 25}, []RoadSource{source}, cfg)
	if err != nil {
		t.Fatalf("compute single: %v", err)
	}

	// Two identical sources: +3 dB.
	double, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 25}, []RoadSource{source, source}, cfg)
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

	r1, err := ComputeReceiverLevels(receiver, []RoadSource{source}, cfg)
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}

	r2, err := ComputeReceiverLevels(receiver, []RoadSource{source}, cfg)
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

	_, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 10}, []RoadSource{source}, cfg)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestPropagation_NoSources(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()

	_, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 10}, nil, cfg)
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

	outputs, err := ComputeReceiverOutputs(receivers, []RoadSource{source}, cfg)
	if err != nil {
		t.Fatalf("compute outputs: %v", err)
	}

	if len(outputs) != 3 {
		t.Fatalf("expected 3 outputs, got %d", len(outputs))
	}

	// Verify monotonic decrease with distance.
	for i := 1; i < len(outputs); i++ {
		if outputs[i].Indicators.LrDay >= outputs[i-1].Indicators.LrDay {
			t.Fatalf("level should decrease with distance: receiver[%d]=%f receiver[%d]=%f",
				i, outputs[i-1].Indicators.LrDay, i+1, outputs[i].Indicators.LrDay)
		}
	}
}

func TestComputeReceiverOutputs_EmptyReceivers(t *testing.T) {
	t.Parallel()

	cfg := DefaultPropagationConfig()

	_, err := ComputeReceiverOutputs(nil, []RoadSource{sampleSource()}, cfg)
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
