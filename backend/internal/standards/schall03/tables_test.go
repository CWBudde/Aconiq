package schall03

import (
	"math"
	"testing"
)

func TestAirAbsorptionAlphaValues(t *testing.T) {
	t.Parallel()

	expected := BeiblattSpectrum{0.1, 0.4, 1.0, 1.9, 3.7, 9.7, 32.8, 117.0}
	if AirAbsorptionAlpha != expected {
		t.Errorf("AirAbsorptionAlpha: expected %v, got %v", expected, AirAbsorptionAlpha)
	}
}

func TestAirAbsorptionAlphaMonotonicallyIncreasing(t *testing.T) {
	t.Parallel()

	for i := 1; i < len(AirAbsorptionAlpha); i++ {
		if AirAbsorptionAlpha[i] <= AirAbsorptionAlpha[i-1] {
			t.Errorf("AirAbsorptionAlpha[%d]=%g <= [%d]=%g: expected monotonically increasing",
				i, AirAbsorptionAlpha[i], i-1, AirAbsorptionAlpha[i-1])
		}
	}
}

func TestSpeedFactorBRolling(t *testing.T) {
	t.Parallel()

	rolling := SpeedFactorBTable[0]
	if rolling.Description != "Rollgeraeusche" {
		t.Fatalf("expected Rollgeraeusche, got %q", rolling.Description)
	}

	expected := BeiblattSpectrum{-5, -5, 0, 10, 25, 25, 25, 25}
	if rolling.B != expected {
		t.Errorf("rolling speed factor b: expected %v, got %v", expected, rolling.B)
	}
}

func TestSpeedFactorBAerodynamic(t *testing.T) {
	t.Parallel()

	aero := SpeedFactorBTable[1]
	for i, v := range aero.B {
		if v != 50 {
			t.Errorf("aerodynamic band %d: expected 50, got %g", i, v)
		}
	}
}

func TestSpeedFactorBAggregate(t *testing.T) {
	t.Parallel()

	agg := SpeedFactorBTable[2]
	for i, v := range agg.B {
		if v != -10 {
			t.Errorf("aggregate band %d: expected -10, got %g", i, v)
		}
	}
}

func TestSpeedFactorBDrive(t *testing.T) {
	t.Parallel()

	drive := SpeedFactorBTable[3]
	for i, v := range drive.B {
		if v != 20 {
			t.Errorf("drive band %d: expected 20, got %g", i, v)
		}
	}
}

func TestSpeedFactorBForTeilquelleRolling(t *testing.T) {
	t.Parallel()

	for _, m := range []int{1, 2, 3, 4} {
		b := SpeedFactorBForTeilquelle(m)
		if b[0] != -5 {
			t.Errorf("m=%d: expected b[0]=-5, got %g", m, b[0])
		}

		if b[4] != 25 {
			t.Errorf("m=%d: expected b[4]=25, got %g", m, b[4])
		}
	}
}

func TestSpeedFactorBForTeilquelleUnknown(t *testing.T) {
	t.Parallel()

	b := SpeedFactorBForTeilquelle(99)
	for i, v := range b {
		if v != 0 {
			t.Errorf("unknown m=99: expected b[%d]=0, got %g", i, v)
		}
	}
}

func TestBridgeCorrections(t *testing.T) {
	t.Parallel()

	if len(BridgeCorrectionTable) != 4 {
		t.Fatalf("expected 4 bridge types, got %d", len(BridgeCorrectionTable))
	}

	// Type 1: steel, direct
	b1 := BridgeCorrectionTable[0]
	if b1.KBr != 12 {
		t.Errorf("bridge type 1: expected KBr=12, got %g", b1.KBr)
	}

	if b1.KLM != -6 {
		t.Errorf("bridge type 1: expected KLM=-6, got %g", b1.KLM)
	}

	// Type 2: steel, ballast
	b2 := BridgeCorrectionTable[1]
	if b2.KBr != 6 || b2.KLM != -3 {
		t.Errorf("bridge type 2: expected KBr=6, KLM=-3, got KBr=%g, KLM=%g", b2.KBr, b2.KLM)
	}

	// Type 3: massive, ballast
	b3 := BridgeCorrectionTable[2]
	if b3.KBr != 3 || b3.KLM != -3 {
		t.Errorf("bridge type 3: expected KBr=3, KLM=-3, got KBr=%g, KLM=%g", b3.KBr, b3.KLM)
	}

	// Type 4: massive, feste Fahrbahn — K_LM not applicable
	b4 := BridgeCorrectionTable[3]
	if b4.KBr != 4 {
		t.Errorf("bridge type 4: expected KBr=4, got %g", b4.KBr)
	}

	if !math.IsNaN(b4.KLM) {
		t.Errorf("bridge type 4: expected KLM=NaN, got %g", b4.KLM)
	}
}

func TestCurveNoiseCorrectionForRadius(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		radiusM float64
		wantKL  float64
		wantKLA float64
	}{
		{"tight curve", 200, 8, -3},
		{"medium curve", 400, 3, -3},
		{"wide curve", 600, 0, 0},
		{"boundary 300", 300, 3, -3},
		{"boundary 500", 500, 0, 0},
		{"straight", 0, 0, 0},
		{"negative", -1, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kL, kLA := CurveNoiseCorrectionForRadius(tt.radiusM)
			if kL != tt.wantKL {
				t.Errorf("K_L: expected %g, got %g", tt.wantKL, kL)
			}

			if kLA != tt.wantKLA {
				t.Errorf("K_LA: expected %g, got %g", tt.wantKLA, kLA)
			}
		})
	}
}

func TestC1FahrbahnartTableCount(t *testing.T) {
	t.Parallel()

	if len(C1FahrbahnartTable) != 3 {
		t.Fatalf("expected 3 Fahrbahnart entries, got %d", len(C1FahrbahnartTable))
	}
}

func TestC1FesteFahrbahnValues(t *testing.T) {
	t.Parallel()

	ff := C1FahrbahnartTable[0]
	if ff.Name != "Feste Fahrbahn" {
		t.Fatalf("expected Feste Fahrbahn, got %q", ff.Name)
	}

	if len(ff.Corrections) != 2 {
		t.Fatalf("expected 2 corrections, got %d", len(ff.Corrections))
	}

	schiene := ff.Corrections[0]

	expectedSchiene := BeiblattSpectrum{0, 0, 0, 7, 3, 0, 0, 0}
	if schiene.C1 != expectedSchiene {
		t.Errorf("Feste Fahrbahn schiene: expected %v, got %v", expectedSchiene, schiene.C1)
	}

	reflexion := ff.Corrections[1]

	expectedReflexion := BeiblattSpectrum{1, 1, 1, 1, 1, 1, 1, 1}
	if reflexion.C1 != expectedReflexion {
		t.Errorf("Feste Fahrbahn reflexion: expected %v, got %v", expectedReflexion, reflexion.C1)
	}
}

func TestC2SurfaceConditionTableCount(t *testing.T) {
	t.Parallel()

	if len(C2SurfaceConditionTable) != 4 {
		t.Fatalf("expected 4 C2 entries, got %d", len(C2SurfaceConditionTable))
	}
}

func TestC2BuGValues(t *testing.T) {
	t.Parallel()

	bug := C2SurfaceConditionTable[0]

	expected := BeiblattSpectrum{0, 0, 0, -4, -5, -5, -4, 0}
	if bug.C2 != expected {
		t.Errorf("bueG: expected %v, got %v", expected, bug.C2)
	}
}
