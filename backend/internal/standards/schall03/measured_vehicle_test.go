package schall03

import (
	"math"
	"testing"
)

func TestMeasuredVehicleValidation(t *testing.T) {
	t.Parallel()

	valid := MeasuredVehicle{
		Fz:     100,
		Name:   "Custom EMU",
		NAchs0: 4,
		Teilquellen: []Teilquelle{
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				AA: 80, DeltaA: BeiblattSpectrum{-5, -3, -1, 0, 1, 0, -2, -4},
			},
			{
				M: 2, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				AA: 78, DeltaA: BeiblattSpectrum{-4, -2, 0, 1, 2, 1, -1, -3},
			},
		},
		RoughnessSplit:        RoughnessSplitSmooth,
		MeasurementCorrection: 2.0,
	}

	err := valid.Validate()
	if err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestMeasuredVehicleRejectsFzBelowHundred(t *testing.T) {
	t.Parallel()

	mv := MeasuredVehicle{
		Fz:     10,
		Name:   "Overlap with Beiblatt",
		NAchs0: 4,
		Teilquellen: []Teilquelle{
			{M: 1, SourceType: SourceTypeRolling, HeightH: 1, AA: 80},
		},
		RoughnessSplit: RoughnessSplitSmooth,
	}

	err := mv.Validate()
	if err == nil {
		t.Fatal("expected error for Fz < 100")
	}
}

func TestMeasuredVehicleRejectsEmptyTeilquellen(t *testing.T) {
	t.Parallel()

	mv := MeasuredVehicle{
		Fz:             100,
		Name:           "No sources",
		NAchs0:         4,
		Teilquellen:    nil,
		RoughnessSplit: RoughnessSplitSmooth,
	}

	err := mv.Validate()
	if err == nil {
		t.Fatal("expected error for empty Teilquellen")
	}
}

func TestTable19RailContribution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method RoughnessSplitMethod
		wantDB float64
	}{
		{RoughnessSplitVerySmooth, -20},
		{RoughnessSplitSmooth, -7},
		{RoughnessSplitUnknown, -3},
	}

	for _, tc := range tests {
		got := Table19RailContributionDB(tc.method)
		if got != tc.wantDB {
			t.Errorf("Table19(%s): got %.1f, want %.1f", tc.method, got, tc.wantDB)
		}
	}
}

func TestTable19WheelContribution(t *testing.T) {
	t.Parallel()

	// For "smooth" (Regelfall): rail = -7 dB → fraction = 0.2
	// → wheel fraction = 0.8 → wheel dB = 10*lg(0.8) ≈ -0.97 dB.
	wheelDB := Table19WheelContributionDB(RoughnessSplitSmooth)
	expected := 10 * math.Log10(0.8) // ≈ -0.969

	if math.Abs(wheelDB-expected) > 0.01 {
		t.Errorf("wheel contribution for smooth: got %.3f, want %.3f", wheelDB, expected)
	}

	// For "unknown": rail = -3 dB → fraction = 10^(-0.3) ≈ 0.5012
	// → wheel = 1 - 0.5012 ≈ 0.4988 → wheel dB ≈ -3.02 dB.
	wheelDB = Table19WheelContributionDB(RoughnessSplitUnknown)
	railFraction := math.Pow(10, -3.0/10.0)
	expected = 10 * math.Log10(1-railFraction)

	if math.Abs(wheelDB-expected) > 0.001 {
		t.Errorf("wheel contribution for unknown: got %.3f, want %.3f", wheelDB, expected)
	}
}

func TestTable20Correction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		brake  string
		sites  int
		same   bool
		wantDB float64
	}{
		{"disc", 1, false, 2},
		{"disc", 3, false, 0},
		{"disc", 1, true, 3},
		{"composite", 1, false, 2},
		{"composite", 3, false, 1},
		{"composite", 1, true, 4},
		{"cast_iron", 1, false, 3},
		{"cast_iron", 3, false, 2},
		{"cast_iron", 1, true, 5},
	}

	for _, tc := range tests {
		got := Table20CorrectionDB(tc.brake, tc.sites, tc.same)
		if got != tc.wantDB {
			t.Errorf("Table20(%s, %d sites, same=%v): got %.0f, want %.0f",
				tc.brake, tc.sites, tc.same, got, tc.wantDB)
		}
	}
}

func TestMeasuredVehicleToFzKategorie(t *testing.T) {
	t.Parallel()

	mv := MeasuredVehicle{
		Fz:     100,
		Name:   "Custom EMU",
		NAchs0: 4,
		Teilquellen: []Teilquelle{
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				AA: 80, DeltaA: BeiblattSpectrum{-5, -3, -1, 0, 1, 0, -2, -4},
			},
		},
		RoughnessSplit:        RoughnessSplitSmooth,
		MeasurementCorrection: 2.0,
	}

	fz := mv.ToFzKategorie()

	if fz.Fz != 100 {
		t.Errorf("Fz: got %d, want 100", fz.Fz)
	}

	if fz.NAchs0 != 4 {
		t.Errorf("NAchs0: got %d, want 4", fz.NAchs0)
	}

	if len(fz.Teilquellen) != 1 {
		t.Fatalf("Teilquellen count: got %d, want 1", len(fz.Teilquellen))
	}
}

func TestSection9SignificanceCheck(t *testing.T) {
	t.Parallel()

	reference := FzKategorie{
		Fz: 7, Name: "E-Lok", NAchs0: 4,
		Teilquellen: []Teilquelle{
			{M: 1, AA: 80, DeltaA: BeiblattSpectrum{0, 0, 0, 0, 0, 0, 0, 0}},
		},
	}

	// Significant: total level differs by 3 dB (>= 2 dB threshold).
	measured := FzKategorie{
		Fz: 100, Name: "Measured E-Lok", NAchs0: 4,
		Teilquellen: []Teilquelle{
			{M: 1, AA: 83, DeltaA: BeiblattSpectrum{0, 0, 0, 0, 0, 0, 0, 0}},
		},
	}

	sig, desc := Section9SignificanceCheck(measured, reference)
	if !sig {
		t.Errorf("expected significant deviation, got: %s", desc)
	}

	// Not significant: total level differs by 1 dB (< 2 dB threshold).
	measured2 := FzKategorie{
		Fz: 101, Name: "Similar E-Lok", NAchs0: 4,
		Teilquellen: []Teilquelle{
			{M: 1, AA: 81, DeltaA: BeiblattSpectrum{0, 0, 0, 0, 0, 0, 0, 0}},
		},
	}

	sig2, _ := Section9SignificanceCheck(measured2, reference)
	if sig2 {
		t.Error("expected no significant deviation for 1 dB total difference")
	}

	// Significant: octave band differs by 5 dB (>= 4 dB threshold).
	measured3 := FzKategorie{
		Fz: 102, Name: "Band-shifted E-Lok", NAchs0: 4,
		Teilquellen: []Teilquelle{
			{M: 1, AA: 80, DeltaA: BeiblattSpectrum{5, 0, 0, 0, 0, 0, 0, 0}},
		},
	}

	sig3, _ := Section9SignificanceCheck(measured3, reference)
	if !sig3 {
		t.Error("expected significant deviation for 5 dB octave band difference")
	}
}

func TestComputeStreckeEmissionWithMeasuredVehicle(t *testing.T) {
	t.Parallel()

	// Create a measured vehicle (Fz 100) with known spectra.
	mv := MeasuredVehicle{
		Fz:     100,
		Name:   "Test Measured Loco",
		NAchs0: 4,
		Teilquellen: []Teilquelle{
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				AA: 85, DeltaA: BeiblattSpectrum{-8, -5, -2, 0, 1, 0, -3, -6},
			},
			{
				M: 5, SourceType: SourceTypeAerodynamic, HeightH: 3, HeightM: 5,
				AA: 70, DeltaA: BeiblattSpectrum{0, 0, 0, 0, 0, 0, 0, 0},
			},
		},
		RoughnessSplit:        RoughnessSplitSmooth,
		MeasurementCorrection: 2.0,
	}

	input := StreckeEmissionInput{
		Vehicles: []VehicleInput{
			{Fz: 100, NPerHour: 4},
		},
		SpeedKPH:         160,
		Fahrbahn:         FahrbahnartSchwellengleis,
		Surface:          SurfaceCondNone,
		MeasuredVehicles: []MeasuredVehicle{mv},
	}

	result, err := ComputeStreckeEmission(input)
	if err != nil {
		t.Fatalf("compute emission: %v", err)
	}

	// Height 1 (rolling noise) should be present.
	h1, ok := result.PerHeight[1]
	if !ok {
		t.Fatal("expected height 1 in result")
	}

	// Height 3 (aerodynamic) should be present.
	h3, ok := result.PerHeight[3]
	if !ok {
		t.Fatal("expected height 3 in result")
	}

	// Rolling noise (AA=85) should produce higher levels than aerodynamic (AA=70)
	// at their respective mid-band peaks.
	if h1[3] <= h3[3] { // compare 500 Hz band
		t.Errorf("expected rolling h1[3]=%.1f > aerodynamic h3[3]=%.1f", h1[3], h3[3])
	}
}

func TestComputeStreckeEmissionRejectsUnknownMeasuredFz(t *testing.T) {
	t.Parallel()

	input := StreckeEmissionInput{
		Vehicles: []VehicleInput{
			{Fz: 200, NPerHour: 4}, // Fz 200 not provided
		},
		SpeedKPH: 100,
		Fahrbahn: FahrbahnartSchwellengleis,
	}

	_, err := ComputeStreckeEmission(input)
	if err == nil {
		t.Fatal("expected error for unknown Fz 200")
	}
}

func TestComputeStreckeEmissionMixedStandardAndMeasured(t *testing.T) {
	t.Parallel()

	mv := MeasuredVehicle{
		Fz:     100,
		Name:   "Custom Loco",
		NAchs0: 4,
		Teilquellen: []Teilquelle{
			{
				M: 1, SourceType: SourceTypeRolling, HeightH: 1, HeightM: 0,
				AA: 82, DeltaA: BeiblattSpectrum{-5, -3, -1, 0, 1, 0, -2, -4},
			},
		},
		RoughnessSplit:        RoughnessSplitSmooth,
		MeasurementCorrection: 2.0,
	}

	input := StreckeEmissionInput{
		Vehicles: []VehicleInput{
			{Fz: 7, NPerHour: 2},   // Standard E-Lok
			{Fz: 100, NPerHour: 3}, // Measured custom loco
		},
		SpeedKPH:         100,
		Fahrbahn:         FahrbahnartSchwellengleis,
		Surface:          SurfaceCondNone,
		MeasuredVehicles: []MeasuredVehicle{mv},
	}

	result, err := ComputeStreckeEmission(input)
	if err != nil {
		t.Fatalf("compute emission: %v", err)
	}

	// Should produce results for height 1 at minimum (both have rolling noise at h=1).
	if _, ok := result.PerHeight[1]; !ok {
		t.Fatal("expected height 1 in result")
	}
}
