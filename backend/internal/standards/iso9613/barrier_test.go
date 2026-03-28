package iso9613

import (
	"math"
	"testing"
)

func TestBarrierPathDifferenceSingle(t *testing.T) {
	t.Parallel()
	// Single diffraction: z = sqrt((dss+dsr)^2 + a^2) - d (Eq. 16)
	// dss=50, dsr=60, a=0, d=100: z = sqrt(110^2) - 100 = 10
	geom := BarrierGeometry{Dss: 50, Dsr: 60, E: 0, A: 0, D: 100}

	z := pathDifference(geom)
	if math.Abs(z-10.0) > 0.01 {
		t.Errorf("single diffraction z: expected 10.0, got %v", z)
	}
}

func TestBarrierPathDifferenceDouble(t *testing.T) {
	t.Parallel()
	// Double diffraction: z = sqrt((dss+dsr+e)^2 + a^2) - d (Eq. 17)
	// dss=40, dsr=50, e=10, a=0, d=90: z = sqrt(100^2) - 90 = 10
	geom := BarrierGeometry{Dss: 40, Dsr: 50, E: 10, A: 0, D: 90}

	z := pathDifference(geom)
	if math.Abs(z-10.0) > 0.01 {
		t.Errorf("double diffraction z: expected 10.0, got %v", z)
	}
}

func TestBarrierPathDifferenceNegative(t *testing.T) {
	t.Parallel()

	geom := BarrierGeometry{Dss: 49, Dsr: 50, E: 0, A: 0, D: 100}

	z := pathDifference(geom)
	if z >= 0 {
		t.Errorf("expected negative z for line-of-sight, got %v", z)
	}
}

func TestBarrierKmet(t *testing.T) {
	t.Parallel()

	geom := BarrierGeometry{Dss: 50, Dsr: 60, E: 0, A: 0, D: 100}
	z := 10.0
	k := kMet(geom, z)

	expected := math.Exp(-(1.0 / 2000.0) * math.Sqrt(50*60*100/(2*10)))
	if math.Abs(k-expected) > 1e-6 {
		t.Errorf("K_met: expected %v, got %v", expected, k)
	}
}

func TestBarrierKmetNegativeZ(t *testing.T) {
	t.Parallel()

	k := kMet(BarrierGeometry{Dss: 50, Dsr: 50, D: 100}, -1)
	if k != 1 {
		t.Errorf("K_met for z<0: expected 1, got %v", k)
	}
}

func TestBarrierC3Single(t *testing.T) {
	t.Parallel()

	c3 := c3Factor(0, 1000)
	if c3 != 1.0 {
		t.Errorf("C3 single: expected 1.0, got %v", c3)
	}
}

func TestBarrierC3Double(t *testing.T) {
	t.Parallel()
	// At 1000 Hz, λ=0.34, e=5: 5λ/e = 5*0.34/5 = 0.34
	// C3 = (1+0.34^2) / (1/3+0.34^2) = 1.1156/0.4489 ≈ 2.485
	c3 := c3Factor(5, 1000)
	lambda := 340.0 / 1000.0
	ratio := 5 * lambda / 5
	r2 := ratio * ratio

	expected := (1 + r2) / (1.0/3.0 + r2)
	if math.Abs(c3-expected) > 0.01 {
		t.Errorf("C3 double: expected %.3f, got %.3f", expected, c3)
	}
}

func TestBarrierAttenuationDzPositive(t *testing.T) {
	t.Parallel()

	geom := BarrierGeometry{Dss: 50, Dsr: 60, E: 0, A: 0, D: 100}

	dz := BarrierDz(geom, 5.0, 1000, 20)
	if dz < 0 || dz > 20 {
		t.Errorf("D_z out of range: %v", dz)
	}
}

func TestBarrierAttenuationCapSingle(t *testing.T) {
	t.Parallel()

	geom := BarrierGeometry{Dss: 50, Dsr: 60, E: 0, A: 0, D: 100}

	dz := BarrierDz(geom, 100, 8000, 20)
	if dz > 20 {
		t.Errorf("D_z exceeded 20 dB cap for single diffraction: %v", dz)
	}
}

func TestBarrierAttenuationCapDouble(t *testing.T) {
	t.Parallel()

	geom := BarrierGeometry{Dss: 40, Dsr: 50, E: 10, A: 0, D: 90}

	dz := BarrierDz(geom, 100, 8000, 20)
	if dz > 25 {
		t.Errorf("D_z exceeded 25 dB cap for double diffraction: %v", dz)
	}
}

func TestBarrierAttenuationNegativeZ(t *testing.T) {
	t.Parallel()

	geom := BarrierGeometry{Dss: 49, Dsr: 50, E: 0, A: 0, D: 100}

	dz := BarrierDz(geom, -1, 1000, 20)
	if dz != 0 {
		t.Errorf("D_z for z<0: expected 0, got %v", dz)
	}
}

func TestBarrierAttenuationBandsNilGeometry(t *testing.T) {
	t.Parallel()

	result := BarrierAttenuationBands(nil, BandLevels{}, 20)
	for i, v := range result {
		if v != 0 {
			t.Errorf("band %d: expected 0 for nil geometry, got %v", i, v)
		}
	}
}
