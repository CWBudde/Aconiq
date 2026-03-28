# ISO 9613-2 Octave-Band Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the preview A-weighted approximation with the normative ISO 9613-2:1996 octave-band engineering method for TA Lärm industrial workflows.

**Architecture:** The module computes attenuation per octave band (63 Hz to 8 kHz) for each source-receiver path, then sums contributions with A-weighting (Eq. 5). Each attenuation term (A_div, A_atm, A_gr, A_bar) is implemented in its own file with table-driven tests. The existing single-value fallback uses 500 Hz terms.

**Tech Stack:** Go, `just test`, `just lint`, `just fmt`

**Reference:** ISO 9613-2:1996 in `interoperability/ISO9613-2/CD 4.48*.pdf`

---

### Task 1: Octave-Band Constants and Types

**Files:**

- Create: `backend/internal/standards/iso9613/octaveband.go`
- Test: `backend/internal/standards/iso9613/octaveband_test.go`

**Step 1: Write the failing test**

```go
package iso9613

import (
	"math"
	"testing"
)

func TestOctaveBandCount(t *testing.T) {
	t.Parallel()
	if NumBands != 8 {
		t.Fatalf("expected 8 bands, got %d", NumBands)
	}
}

func TestOctaveBandFrequencies(t *testing.T) {
	t.Parallel()
	expected := [NumBands]float64{63, 125, 250, 500, 1000, 2000, 4000, 8000}
	if OctaveBandFrequencies != expected {
		t.Fatalf("unexpected frequencies: %v", OctaveBandFrequencies)
	}
}

func TestAWeightingCorrections(t *testing.T) {
	t.Parallel()
	// 1 kHz band (index 4) must have A-weighting of 0.0 dB
	if AWeighting[4] != 0.0 {
		t.Fatalf("expected 0.0 at 1 kHz, got %v", AWeighting[4])
	}
}

func TestWavelength(t *testing.T) {
	t.Parallel()
	// λ = c / f, at 1000 Hz: 340/1000 = 0.34
	got := Wavelength(1000)
	if math.Abs(got-0.34) > 1e-9 {
		t.Fatalf("expected 0.34, got %v", got)
	}
}

func TestBandLevelsFromSingleValue(t *testing.T) {
	t.Parallel()
	levels := BandLevelsFromAWeighted(100.0)
	// All 8 bands should be set to 100.0 (the single A-weighted value)
	for i, v := range levels {
		if v != 100.0 {
			t.Fatalf("band %d: expected 100.0, got %v", i, v)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/standards/iso9613/... -run TestOctaveBand -v`
Expected: FAIL — types and constants not defined

**Step 3: Write the implementation**

```go
package iso9613

const (
	NumBands   = 8
	SpeedOfSound = 340.0 // m/s, reference value used by ISO 9613-2
)

// OctaveBandFrequencies contains the 8 standard midband frequencies (Hz).
var OctaveBandFrequencies = [NumBands]float64{63, 125, 250, 500, 1000, 2000, 4000, 8000}

// AWeighting contains the A-weighting corrections per octave band (dB).
// IEC 651 / IEC 61672-1 values at nominal midband frequencies.
var AWeighting = [NumBands]float64{-26.2, -16.1, -8.6, -3.2, 0.0, 1.2, 1.0, -1.1}

// BandLevels holds sound power or pressure levels for each octave band.
type BandLevels [NumBands]float64

// Wavelength returns the wavelength of sound at a given frequency (m).
func Wavelength(freqHz float64) float64 {
	return SpeedOfSound / freqHz
}

// BandLevelsFromAWeighted creates octave-band levels from a single A-weighted
// value by setting all bands to that value. This is the fallback per
// ISO 9613-2 Note 1: use 500 Hz attenuation terms when only A-weighted
// sound power is known.
func BandLevelsFromAWeighted(lwa float64) BandLevels {
	var levels BandLevels
	for i := range levels {
		levels[i] = lwa
	}
	return levels
}
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/standards/iso9613/... -run "TestOctaveBand|TestAWeighting|TestWavelength|TestBandLevels" -v`
Expected: PASS

**Step 5: Commit**

```
git add backend/internal/standards/iso9613/octaveband.go backend/internal/standards/iso9613/octaveband_test.go
git commit -m "feat(iso9613): add octave-band constants, types, and A-weighting"
```

---

### Task 2: Atmospheric Absorption with Table 2

**Files:**

- Create: `backend/internal/standards/iso9613/atmospheric.go`
- Create: `backend/internal/standards/iso9613/atmospheric_test.go`

**Step 1: Write the failing test**

```go
package iso9613

import (
	"math"
	"testing"
)

func TestAtmosphericAbsorptionTable2Row1(t *testing.T) {
	t.Parallel()
	// Table 2 row 1: 10°C, 70% RH
	// α values: 0.1, 0.4, 1.0, 1.9, 3.7, 9.7, 32.8, 117.0 dB/km
	// At 100 m distance, A_atm = α * 100 / 1000 = α / 10
	tests := []struct {
		band     int
		alpha    float64
		distance float64
		expected float64
	}{
		{0, 0.1, 1000, 0.1},      // 63 Hz, 1 km
		{3, 1.9, 100, 0.19},      // 500 Hz, 100 m
		{4, 3.7, 200, 0.74},      // 1 kHz, 200 m
		{7, 117.0, 100, 11.7},    // 8 kHz, 100 m
	}

	for _, tc := range tests {
		alpha := LookupAlpha(10, 70, tc.band)
		got := AtmosphericAbsorption(alpha, tc.distance)
		if math.Abs(got-tc.expected) > 0.01 {
			t.Errorf("band %d, d=%.0fm: expected %.2f, got %.2f", tc.band, tc.distance, tc.expected, got)
		}
	}
}

func TestAtmosphericAbsorptionTable2AllRows(t *testing.T) {
	t.Parallel()
	// Verify the 500 Hz column (band index 3) for each table row
	tests := []struct {
		tempC    float64
		humidity float64
		alpha500 float64
	}{
		{10, 70, 1.9},
		{20, 70, 2.8},
		{30, 70, 3.1},
		{15, 20, 2.7},
		{15, 50, 2.2},
		{15, 80, 2.4},
	}

	for _, tc := range tests {
		alpha := LookupAlpha(tc.tempC, tc.humidity, 3)
		if math.Abs(alpha-tc.alpha500) > 0.05 {
			t.Errorf("T=%.0f RH=%.0f: expected α₅₀₀=%.1f, got %.1f", tc.tempC, tc.humidity, tc.alpha500, alpha)
		}
	}
}

func TestAtmosphericAbsorptionBandLevels(t *testing.T) {
	t.Parallel()
	// Full octave-band A_atm at 10°C, 70% RH, 200 m
	got := AtmosphericAbsorptionBands(10, 70, 200)
	// Expected: α * 200 / 1000 = α * 0.2
	expected := [NumBands]float64{0.02, 0.08, 0.20, 0.38, 0.74, 1.94, 6.56, 23.4}
	for i := range got {
		if math.Abs(got[i]-expected[i]) > 0.01 {
			t.Errorf("band %d: expected %.2f, got %.2f", i, expected[i], got[i])
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/standards/iso9613/... -run TestAtmosphericAbsorption -v`
Expected: FAIL

**Step 3: Write the implementation**

```go
package iso9613

// Table 2 from ISO 9613-2:1996: atmospheric attenuation coefficient α (dB/km)
// indexed by [temperature °C, humidity %] for each octave band.

type atmosphericRow struct {
	TempC    float64
	Humidity float64
	Alpha    [NumBands]float64
}

// table2 holds the 7 reference rows from ISO 9613-2 Table 2.
// Row order: 63, 125, 250, 500, 1000, 2000, 4000, 8000 Hz.
var table2 = []atmosphericRow{
	{10, 70, [NumBands]float64{0.1, 0.4, 1.0, 1.9, 3.7, 9.7, 32.8, 117.0}},
	{20, 70, [NumBands]float64{0.1, 0.3, 1.1, 2.8, 5.0, 9.0, 22.9, 76.6}},
	{30, 70, [NumBands]float64{0.1, 0.3, 1.0, 3.1, 7.4, 12.7, 23.1, 59.3}},
	{15, 20, [NumBands]float64{0.3, 0.6, 1.2, 2.7, 8.2, 28.2, 88.8, 202.0}},
	{15, 50, [NumBands]float64{0.1, 0.5, 1.2, 2.2, 4.2, 10.8, 36.2, 129.0}},
	{15, 80, [NumBands]float64{0.1, 0.3, 1.1, 2.4, 4.1, 8.3, 23.7, 82.8}},
}

// LookupAlpha returns the atmospheric attenuation coefficient α (dB/km)
// for a given temperature, humidity, and octave band index.
// For exact table matches it returns the tabulated value. For other
// conditions it uses nearest-row selection. band is 0-indexed.
func LookupAlpha(tempC, humidity float64, band int) float64 {
	if band < 0 || band >= NumBands {
		return 0
	}

	best := 0
	bestDist := 1e18
	for i, row := range table2 {
		dt := (tempC - row.TempC) / 10.0
		dh := (humidity - row.Humidity) / 50.0
		dist := dt*dt + dh*dh
		if dist < bestDist {
			bestDist = dist
			best = i
		}
	}

	return table2[best].Alpha[band]
}

// AtmosphericAbsorption computes A_atm (Eq. 8): α · d / 1000.
func AtmosphericAbsorption(alpha, distanceM float64) float64 {
	return alpha * distanceM / 1000.0
}

// AtmosphericAbsorptionBands computes A_atm for all 8 octave bands.
func AtmosphericAbsorptionBands(tempC, humidity, distanceM float64) BandLevels {
	var result BandLevels
	for i := 0; i < NumBands; i++ {
		alpha := LookupAlpha(tempC, humidity, i)
		result[i] = AtmosphericAbsorption(alpha, distanceM)
	}
	return result
}
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/standards/iso9613/... -run TestAtmosphericAbsorption -v`
Expected: PASS

**Step 5: Commit**

```
git add backend/internal/standards/iso9613/atmospheric.go backend/internal/standards/iso9613/atmospheric_test.go
git commit -m "feat(iso9613): implement atmospheric absorption with Table 2 coefficients (Eq. 8)"
```

---

### Task 3: Ground Effect — Table 3 Formulas

**Files:**

- Create: `backend/internal/standards/iso9613/ground.go`
- Create: `backend/internal/standards/iso9613/ground_test.go`

**Step 1: Write the failing test**

```go
package iso9613

import (
	"math"
	"testing"
)

func TestGroundEffectFunctions(t *testing.T) {
	t.Parallel()
	// Verify a'(h), b'(h), c'(h), d'(h) at known values
	// Use h=5, dp=200 as a middle-of-range check
	tests := []struct {
		name string
		fn   func(float64, float64) float64
		h    float64
		dp   float64
	}{
		{"aPrime", aPrime, 5, 200},
		{"bPrime", bPrime, 5, 200},
		{"cPrime", cPrime, 5, 200},
		{"dPrime", dPrime, 5, 200},
	}

	for _, tc := range tests {
		got := tc.fn(tc.h, tc.dp)
		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Errorf("%s(h=%v, dp=%v) returned %v", tc.name, tc.h, tc.dp, got)
		}
		// All functions return values >= 0 for positive h
		if got < 0 {
			t.Errorf("%s(h=%v, dp=%v) = %v, expected >= 0", tc.name, tc.h, tc.dp, got)
		}
	}
}

func TestGroundEffectHardGround(t *testing.T) {
	t.Parallel()
	// Hard ground: G=0. Table 3 says A_s = A_r = -1.5 for all bands.
	// A_m = -3*q*(1-G_m) = -3*q for G_m=0
	hs, hr := 5.0, 4.0
	dp := 200.0
	result := GroundEffectBands(0, 0, 0, hs, hr, dp)

	// For hard ground, 63 Hz: A_s = -1.5, A_r = -1.5
	if math.Abs(result[0]-(-1.5+-1.5+middleRegionAtten(0, 0, hs, hr, dp))) > 0.1 {
		t.Errorf("63 Hz hard ground: got %v", result[0])
	}
}

func TestGroundEffectPorousGround(t *testing.T) {
	t.Parallel()
	// Porous ground: G=1
	hs, hr := 5.0, 4.0
	dp := 200.0
	result := GroundEffectBands(1, 1, 1, hs, hr, dp)

	// High frequency bands (2k, 4k, 8k): A_s = A_r = -1.5(1-G) = 0
	for _, band := range []int{5, 6, 7} {
		if math.Abs(result[band]) > 0.1 {
			t.Errorf("band %d porous ground: expected ~0, got %v", band, result[band])
		}
	}
}

func TestGroundEffectSimplified(t *testing.T) {
	t.Parallel()
	// Eq. 10: A_gr = 4.8 - (2*hm/d)*[17 + 300/d] >= 0
	// hm=3, d=200: 4.8 - (6/200)*(17+1.5) = 4.8 - 0.555 = 4.245
	got := GroundEffectSimplified(3, 200)
	expected := 4.8 - (2*3.0/200.0)*(17+300.0/200.0)
	if math.Abs(got-expected) > 0.01 {
		t.Errorf("simplified: expected %.3f, got %.3f", expected, got)
	}
}

func TestGroundEffectSimplifiedClampedToZero(t *testing.T) {
	t.Parallel()
	// Very high source: should clamp to 0
	got := GroundEffectSimplified(100, 50)
	if got != 0 {
		t.Errorf("expected 0 (clamped), got %v", got)
	}
}

func TestMiddleRegionFactor(t *testing.T) {
	t.Parallel()
	// q = 0 when dp <= 30*(hs+hr)
	q := middleRegionQ(5, 4, 200)
	if q != 0 {
		t.Errorf("expected q=0 for dp=200 <= 30*(5+4)=270, got %v", q)
	}

	// q > 0 when dp > 30*(hs+hr)
	q = middleRegionQ(2, 2, 200)
	if q <= 0 {
		t.Errorf("expected q>0 for dp=200 > 30*(2+2)=120, got %v", q)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/standards/iso9613/... -run "TestGroundEffect|TestMiddleRegion" -v`
Expected: FAIL

**Step 3: Write the implementation**

```go
package iso9613

import "math"

// Table 3 functions for ground attenuation contributions.

func aPrime(h, dp float64) float64 {
	return 1.5 + 3.0*math.Exp(-0.12*(h-5)*(h-5))*(1-math.Exp(-dp/50.0)) +
		5.7*math.Exp(-0.09*h*h)*(1-math.Exp(-2.8e-6*dp*dp))
}

func bPrime(h, dp float64) float64 {
	return 1.5 + 8.6*math.Exp(-0.09*h*h)*(1-math.Exp(-dp/50.0))
}

func cPrime(h, dp float64) float64 {
	return 1.5 + 14.0*math.Exp(-0.46*h*h)*(1-math.Exp(-dp/50.0))
}

func dPrime(h, dp float64) float64 {
	return 1.5 + 5.0*math.Exp(-0.9*h*h)*(1-math.Exp(-dp/50.0))
}

// middleRegionQ computes the weighting factor q for the middle region.
// q = 0 when dp ≤ 30*(hs + hr); otherwise q = 1 - 30*(hs+hr)/dp.
func middleRegionQ(hs, hr, dp float64) float64 {
	limit := 30 * (hs + hr)
	if dp <= limit {
		return 0
	}
	return 1 - limit/dp
}

// sourceReceiverAtten computes A_s or A_r from Table 3 for one band.
// G is the ground factor for that region, h is hs or hr, dp is the
// projected source-receiver distance.
func sourceReceiverAtten(G, h, dp float64, band int) float64 {
	switch band {
	case 0: // 63 Hz
		return -1.5
	case 1: // 125 Hz
		return -1.5 + G*aPrime(h, dp)
	case 2: // 250 Hz
		return -1.5 + G*bPrime(h, dp)
	case 3: // 500 Hz
		return -1.5 + G*cPrime(h, dp)
	case 4: // 1000 Hz
		return -1.5 + G*dPrime(h, dp)
	case 5, 6, 7: // 2000, 4000, 8000 Hz
		return -1.5 * (1 - G)
	default:
		return 0
	}
}

// middleRegionAtten computes A_m from Table 3 for one band.
func middleRegionAtten(Gm float64, band int, q float64) float64 {
	switch band {
	case 0: // 63 Hz
		return 3 * q
	case 1, 2, 3, 4: // 125–1000 Hz
		return -3 * q * (1 - Gm)
	case 5, 6, 7: // 2000, 4000, 8000 Hz
		return -3 * q * (1 - Gm)
	default:
		return 0
	}
}

// GroundEffectBands computes A_gr per octave band using the general method
// (Eq. 9, Table 3). Gs, Gr, Gm are the ground factors for the source,
// receiver, and middle regions. hs and hr are source and receiver heights.
// dp is the projected source-receiver distance.
func GroundEffectBands(Gs, Gr, Gm, hs, hr, dp float64) BandLevels {
	q := middleRegionQ(hs, hr, dp)
	var result BandLevels
	for i := 0; i < NumBands; i++ {
		as := sourceReceiverAtten(Gs, hs, dp, i)
		ar := sourceReceiverAtten(Gr, hr, dp, i)
		am := middleRegionAtten(Gm, i, q)
		result[i] = as + ar + am
	}
	return result
}

// GroundEffectSimplified computes A_gr using the simplified method (Eq. 10).
// Valid only for A-weighted levels over mostly porous, non-tonal ground.
// hm is the mean propagation height, d is the source-receiver distance.
func GroundEffectSimplified(hm, d float64) float64 {
	if d <= 0 {
		return 0
	}
	agr := 4.8 - (2*hm/d)*(17+300.0/d)
	if agr < 0 {
		return 0
	}
	return agr
}
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/standards/iso9613/... -run "TestGroundEffect|TestMiddleRegion" -v`
Expected: PASS

**Step 5: Commit**

```
git add backend/internal/standards/iso9613/ground.go backend/internal/standards/iso9613/ground_test.go
git commit -m "feat(iso9613): implement ground effect with Table 3 three-region model (Eq. 9)"
```

---

### Task 4: Barrier Screening Formulas

**Files:**

- Create: `backend/internal/standards/iso9613/barrier.go`
- Create: `backend/internal/standards/iso9613/barrier_test.go`

**Step 1: Write the failing test**

```go
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
	// Line of sight above barrier: z is negative
	geom := BarrierGeometry{Dss: 49, Dsr: 50, E: 0, A: 0, D: 100}
	z := pathDifference(geom)
	if z >= 0 {
		t.Errorf("expected negative z for line-of-sight, got %v", z)
	}
}

func TestBarrierKmet(t *testing.T) {
	t.Parallel()
	// K_met = exp(-(1/2000)*sqrt(dss*dsr*d/(2z))) for z > 0
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
	// Single diffraction (e=0): C3 = 1
	c3 := c3Factor(0, 1000)
	if c3 != 1.0 {
		t.Errorf("C3 single: expected 1.0, got %v", c3)
	}
}

func TestBarrierC3Double(t *testing.T) {
	t.Parallel()
	// Double diffraction: C3 = [1+(5λ/e)^2] / [(1/3)+(5λ/e)^2]
	// At 1000 Hz, λ=0.34, e=5: 5λ/e = 0.34
	// C3 = (1+0.1156) / (0.3333+0.1156) = 1.1156/0.4489 ≈ 2.485
	c3 := c3Factor(5, 1000)
	expected := (1 + math.Pow(5*0.34/5, 2)) / (1.0/3.0 + math.Pow(5*0.34/5, 2))
	if math.Abs(c3-expected) > 0.01 {
		t.Errorf("C3 double: expected %.3f, got %.3f", expected, c3)
	}
}

func TestBarrierAttenuationDz(t *testing.T) {
	t.Parallel()
	// D_z = 10*lg(3 + C2/λ * C3 * z * Kmet)
	// With z=5, freq=1000, single diffraction, C2=20
	geom := BarrierGeometry{Dss: 50, Dsr: 60, E: 0, A: 0, D: 100}
	dz := BarrierDz(geom, 5.0, 1000, 20)
	if dz < 0 || dz > 20 {
		t.Errorf("D_z out of range: %v", dz)
	}
}

func TestBarrierAttenuationCapSingle(t *testing.T) {
	t.Parallel()
	// D_z capped at 20 for single diffraction
	geom := BarrierGeometry{Dss: 50, Dsr: 60, E: 0, A: 0, D: 100}
	dz := BarrierDz(geom, 100, 8000, 20) // very large z, high freq
	if dz > 20 {
		t.Errorf("D_z exceeded 20 dB cap for single diffraction: %v", dz)
	}
}

func TestBarrierAttenuationCapDouble(t *testing.T) {
	t.Parallel()
	// D_z capped at 25 for double diffraction
	geom := BarrierGeometry{Dss: 40, Dsr: 50, E: 10, A: 0, D: 90}
	dz := BarrierDz(geom, 100, 8000, 20) // very large z, high freq
	if dz > 25 {
		t.Errorf("D_z exceeded 25 dB cap for double diffraction: %v", dz)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/standards/iso9613/... -run TestBarrier -v`
Expected: FAIL

**Step 3: Write the implementation**

```go
package iso9613

import "math"

// BarrierGeometry holds pre-computed diffraction path geometry.
type BarrierGeometry struct {
	Dss float64 // distance from source to first diffraction edge (m)
	Dsr float64 // distance from last diffraction edge to receiver (m)
	E   float64 // distance between first and last diffraction edge, 0 for single (m)
	A   float64 // component distance parallel to barrier edge (m)
	D   float64 // direct source-to-receiver distance (m)
}

// IsDouble returns true if this represents double diffraction (e > 0).
func (g BarrierGeometry) IsDouble() bool {
	return g.E > 0
}

// pathDifference computes z from Eq. 16 (single) or Eq. 17 (double).
func pathDifference(g BarrierGeometry) float64 {
	pathSum := g.Dss + g.Dsr + g.E
	return math.Sqrt(pathSum*pathSum+g.A*g.A) - g.D
}

// c3Factor computes C_3 from Eq. 15.
// For single diffraction (e=0), C_3 = 1.
// For double diffraction, C_3 = [1+(5λ/e)²] / [(1/3)+(5λ/e)²].
func c3Factor(e, freqHz float64) float64 {
	if e <= 0 {
		return 1
	}
	lambda := Wavelength(freqHz)
	ratio := 5 * lambda / e
	r2 := ratio * ratio
	return (1 + r2) / (1.0/3.0 + r2)
}

// kMet computes K_met from Eq. 18.
func kMet(g BarrierGeometry, z float64) float64 {
	if z <= 0 {
		return 1
	}
	return math.Exp(-(1.0 / 2000.0) * math.Sqrt(g.Dss*g.Dsr*g.D/(2*z)))
}

// BarrierDz computes the barrier attenuation D_z (Eq. 14) for one octave band.
// c2 is 20 when ground reflections are included, 40 when handled by image sources.
func BarrierDz(g BarrierGeometry, z, freqHz, c2 float64) float64 {
	if z <= 0 {
		return 0
	}

	lambda := Wavelength(freqHz)
	c3 := c3Factor(g.E, freqHz)
	km := kMet(g, z)

	dz := 10 * math.Log10(3+(c2/lambda)*c3*z*km)

	maxDz := 20.0
	if g.IsDouble() {
		maxDz = 25.0
	}

	if dz > maxDz {
		return maxDz
	}

	return dz
}

// BarrierAttenuationBands computes A_bar per octave band (Eq. 12).
// groundAtten is A_gr for the unscreened path (subtracted per Eq. 12).
// Returns zero bands if geometry is nil (no barrier).
func BarrierAttenuationBands(g *BarrierGeometry, groundAtten BandLevels, c2 float64) BandLevels {
	var result BandLevels
	if g == nil {
		return result
	}

	z := pathDifference(*g)

	for i := 0; i < NumBands; i++ {
		dz := BarrierDz(*g, z, OctaveBandFrequencies[i], c2)
		abar := dz - groundAtten[i]
		if abar < 0 {
			abar = 0
		}
		result[i] = abar
	}

	return result
}
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/standards/iso9613/... -run TestBarrier -v`
Expected: PASS

**Step 5: Commit**

```
git add backend/internal/standards/iso9613/barrier.go backend/internal/standards/iso9613/barrier_test.go
git commit -m "feat(iso9613): implement barrier diffraction formulas (Eq. 12–18)"
```

---

### Task 5: Meteorological Correction

**Files:**

- Modify: `backend/internal/standards/iso9613/propagation.go`
- Create: `backend/internal/standards/iso9613/meteorological_test.go`

**Step 1: Write the failing test**

```go
package iso9613

import (
	"math"
	"testing"
)

func TestMeteorologicalCorrectionShortDistance(t *testing.T) {
	t.Parallel()
	// C_met = 0 when dp <= 10*(hs+hr)
	// hs=5, hr=4, dp=80 <= 10*9=90
	got := MeteorologicalCorrection(3.0, 5, 4, 80)
	if got != 0 {
		t.Errorf("expected 0 for short distance, got %v", got)
	}
}

func TestMeteorologicalCorrectionLongDistance(t *testing.T) {
	t.Parallel()
	// C_met = C0 * [1 - 10*(hs+hr)/dp]
	// C0=3, hs=5, hr=4, dp=200: C_met = 3 * [1 - 90/200] = 3 * 0.55 = 1.65
	got := MeteorologicalCorrection(3.0, 5, 4, 200)
	if math.Abs(got-1.65) > 0.01 {
		t.Errorf("expected 1.65, got %v", got)
	}
}

func TestMeteorologicalCorrectionZeroC0(t *testing.T) {
	t.Parallel()
	// C0=0 (pure downwind): C_met always 0
	got := MeteorologicalCorrection(0, 5, 4, 500)
	if got != 0 {
		t.Errorf("expected 0 for C0=0, got %v", got)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/standards/iso9613/... -run TestMeteorological -v`
Expected: FAIL

**Step 3: Add MeteorologicalCorrection to propagation.go**

Add to `propagation.go`:

```go
// MeteorologicalCorrection computes C_met from Eq. 21–22.
// c0 depends on local meteorological statistics; default 0 for pure downwind.
func MeteorologicalCorrection(c0, hs, hr, dp float64) float64 {
	limit := 10 * (hs + hr)
	if dp <= limit {
		return 0
	}
	return c0 * (1 - limit/dp)
}
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/standards/iso9613/... -run TestMeteorological -v`
Expected: PASS

**Step 5: Commit**

```
git add backend/internal/standards/iso9613/propagation.go backend/internal/standards/iso9613/meteorological_test.go
git commit -m "feat(iso9613): implement meteorological correction C_met (Eq. 21–22)"
```

---

### Task 6: Rewrite Propagation Chain for Octave Bands

**Files:**

- Modify: `backend/internal/standards/iso9613/propagation.go`
- Modify: `backend/internal/standards/iso9613/model.go`
- Modify: `backend/internal/standards/iso9613/emission.go`

**Step 1: Update model types**

Update `PointSource` in `model.go`:

```go
type PointSource struct {
	ID                      string      `json:"id"`
	Point                   geo.Point2D `json:"point"`
	SourceHeightM           float64     `json:"source_height_m"`
	SoundPowerLevelDB       float64     `json:"sound_power_level_db"`
	OctaveBandLevels        *BandLevels `json:"octave_band_levels,omitempty"`
	DirectivityCorrectionDB float64     `json:"directivity_correction_db,omitempty"`
	TonalityCorrectionDB    float64     `json:"tonality_correction_db,omitempty"`
	ImpulsivityCorrectionDB float64     `json:"impulsivity_correction_db,omitempty"`
}
```

Add `C0` and barrier geometry to `PropagationConfig`:

```go
type PropagationConfig struct {
	GroundFactor            float64
	AirTemperatureC         float64
	RelativeHumidityPercent float64
	MeteorologyAssumption   string
	BarrierGeometry         *BarrierGeometry // nil = no barrier
	C0                      float64          // meteorological correction factor
	MinDistanceM            float64
}
```

Remove `BarrierAttenuationDB` from `PropagationConfig` (replaced by `BarrierGeometry`).

**Step 2: Rewrite emission.go for octave bands**

```go
// EffectiveBandLevels returns the octave-band sound power levels for a source.
// If OctaveBandLevels is set, uses those directly plus corrections.
// Otherwise, fills all bands from SoundPowerLevelDB (500 Hz fallback).
func EffectiveBandLevels(source PointSource) BandLevels {
	var levels BandLevels
	if source.OctaveBandLevels != nil {
		levels = *source.OctaveBandLevels
	} else {
		levels = BandLevelsFromAWeighted(source.SoundPowerLevelDB)
	}

	dc := source.DirectivityCorrectionDB
	for i := range levels {
		levels[i] += dc
	}
	return levels
}
```

Keep `ComputeEmission` for backward compatibility but add `EffectiveBandLevels`.

**Step 3: Rewrite propagation.go attenuation chain**

Replace the existing approximate functions with the octave-band chain:

```go
// BandAttenuation computes per-octave-band attenuation for one source-receiver path.
func BandAttenuation(receiver geo.PointReceiver, source PointSource, cfg PropagationConfig) (BandLevels, float64) {
	distance := effectiveDistance(sourceDistance(receiver, source, cfg), cfg)
	hs := source.SourceHeightM
	hr := receiver.HeightM
	dp := geo.Distance(receiver.Point, source.Point) // projected distance

	adiv := geometricDivergence(distance)
	aatm := AtmosphericAbsorptionBands(cfg.AirTemperatureC, cfg.RelativeHumidityPercent, distance)
	agr := GroundEffectBands(cfg.GroundFactor, cfg.GroundFactor, cfg.GroundFactor, hs, hr, dp)
	abar := BarrierAttenuationBands(cfg.BarrierGeometry, agr, 20)

	var totalAtten BandLevels
	for i := 0; i < NumBands; i++ {
		totalAtten[i] = adiv + aatm[i] + agr[i] + abar[i]
	}
	return totalAtten, distance
}

// ComputeDownwindLevel computes L_AT(DW) for one receiver from all sources (Eq. 5).
func ComputeDownwindLevel(receiver geo.PointReceiver, sources []PointSource, cfg PropagationConfig) float64 {
	sum := 0.0
	for _, source := range sources {
		bandLevels := EffectiveBandLevels(source)
		atten, _ := BandAttenuation(receiver, source, cfg)

		for j := 0; j < NumBands; j++ {
			lft := bandLevels[j] - atten[j]
			sum += math.Pow(10, 0.1*(lft+AWeighting[j]))
		}
	}

	if sum <= 0 {
		return -999
	}
	return 10 * math.Log10(sum)
}
```

**Step 4: Run all existing tests, fix any breakage**

Run: `cd backend && go test ./internal/standards/iso9613/... -v`
Expected: Some tests may need updating for the new API. Fix them.

**Step 5: Commit**

```
git add backend/internal/standards/iso9613/
git commit -m "feat(iso9613): rewrite propagation chain for octave-band processing (Eq. 3–5)"
```

---

### Task 7: Update Compute and Export for New Indicators

**Files:**

- Modify: `backend/internal/standards/iso9613/compute.go`
- Modify: `backend/internal/standards/iso9613/export.go`
- Modify: `backend/internal/standards/iso9613/indicators.go`
- Modify: `backend/internal/standards/iso9613/model.go`

**Step 1: Update indicators and output types**

In `model.go`, update `ReceiverIndicators`:

```go
type ReceiverIndicators struct {
	LpAeqDW float64     // A-weighted downwind level (Eq. 5)
	LpAeqLT float64     // long-term average (Eq. 6), only when C_met > 0
	BandLevels *BandLevels // per-octave-band levels, optional
}
```

In `indicators.go`, add `IndicatorLpAeqLT`:

```go
const (
	IndicatorLpAeqDW = "LpAeq_DW"
	IndicatorLpAeqLT = "LpAeq_LT"
)
```

Keep `IndicatorLpAeq` as an alias for `IndicatorLpAeqDW` for backward compatibility.

**Step 2: Update compute.go**

Update `ComputeReceiverLevel` to use the octave-band chain and return both DW and LT levels. Update `ComputeReceiverOutputs` to populate the new indicators.

**Step 3: Update export.go**

Update `ExportResultBundle` to include `LpAeq_DW` and conditionally `LpAeq_LT` in receiver tables and rasters.

**Step 4: Run all tests**

Run: `cd backend && go test ./internal/standards/iso9613/... -v`
Expected: PASS

**Step 5: Commit**

```
git add backend/internal/standards/iso9613/
git commit -m "feat(iso9613): add LpAeq_DW and LpAeq_LT indicators with octave-band output"
```

---

### Task 8: Update CLI Integration and Descriptor

**Files:**

- Modify: `backend/internal/standards/iso9613/model.go` (Descriptor)
- Modify: `backend/internal/app/cli/run_options.go` (iso9613RunOptions, PropagationConfig)
- Modify: `backend/internal/app/cli/run_pipeline.go` (compute call)
- Modify: `backend/internal/app/cli/run_persist.go` (output persistence)
- Modify: `backend/internal/app/cli/run_extract.go` (source extraction)

**Step 1: Update descriptor parameters**

- Remove `barrier_attenuation_db` parameter
- Add `c0_met` parameter (default 0)
- Update `model_version` to `"iso9613-octaveband-v1"`
- Update `compliance_boundary` to `"iso9613-engineering-octaveband"`
- Add supported indicators: `["LpAeq_DW", "LpAeq_LT"]`

**Step 2: Update CLI options and pipeline**

- Remove `BarrierAttenuationDB` from `iso9613RunOptions`
- Add `C0Met` to `iso9613RunOptions`
- Update `PropagationConfig()` method
- Update `parseISO9613RunOptions` to parse `c0_met`
- Update the compute call in `run_pipeline.go` to use `ComputeReceiverOutputs`
- Update `persistISO9613RunOutputs` for new indicator names

**Step 3: Run full test suite**

Run: `cd backend && go test ./... -count=1`
Expected: PASS

Run: `just ci`
Expected: PASS (lint, fmt, test, tidy)

**Step 4: Commit**

```
git add backend/
git commit -m "feat(iso9613): wire octave-band propagation into CLI and update descriptor"
```

---

### Task 9: Comprehensive Test Coverage

**Files:**

- Modify: `backend/internal/standards/iso9613/iso9613_test.go`

**Step 1: Add integration tests**

Write table-driven tests that exercise the full chain:

- Single point source at known distance → verify L_AT(DW) against hand calculation
- Two sources at different distances → verify energetic sum
- Hard ground (G=0) vs porous ground (G=1) → verify ground effect changes levels
- With vs without barrier geometry → verify barrier reduces levels
- With C0 > 0 → verify L_AT(LT) < L_AT(DW)
- Determinism: same inputs produce identical outputs

**Step 2: Add backward compatibility test**

Verify that a source with only `SoundPowerLevelDB` (no octave bands) still produces a valid result using the 500 Hz fallback.

**Step 3: Run full suite**

Run: `cd backend && go test ./internal/standards/iso9613/... -v -count=1`
Expected: PASS

Run: `just ci`
Expected: PASS

**Step 4: Commit**

```
git add backend/internal/standards/iso9613/
git commit -m "test(iso9613): add comprehensive octave-band integration tests"
```

---

### Task 10: Update Acceptance Scenarios and Golden Tests

**Files:**

- Modify: `backend/internal/qa/acceptance/testdata/iso9613/point_preview.scenario.json`
- Modify: `backend/internal/qa/acceptance/testdata/iso9613/point_contextual.scenario.json`
- Modify: `backend/internal/app/cli/testdata/phase19/iso9613_industry_model.geojson`

**Step 1: Update acceptance test expected values**

The octave-band chain produces different numeric results than the preview approximation. Update golden values in acceptance scenarios to match the new normative calculations.

**Step 2: Update golden snapshots**

Run: `just update-golden`
Review diffs carefully — the level changes should be explainable by the switch from approximate to normative formulas.

**Step 3: Run full CI**

Run: `just ci`
Expected: PASS

**Step 4: Commit**

```
git add backend/
git commit -m "test(iso9613): update acceptance scenarios for octave-band propagation"
```

---

### Task 11: Update Documentation

**Files:**

- Modify: `backend/internal/standards/iso9613/indicators.go` (provenance metadata)
- Modify: `docs/phase19-iso9613-baseline.md`

**Step 1: Update provenance metadata**

Update `ProvenanceMetadata` to reflect the new model version and compliance boundary.

**Step 2: Update baseline documentation**

Update `docs/phase19-iso9613-baseline.md` to document:

- Octave-band processing is now implemented
- Table 2 atmospheric absorption with interpolation simplification
- Table 3 general ground effect with single-G limitation
- Barrier formulas implemented (geometry detection deferred)
- C_met implemented (C_0 = 0 default)
- Known deviations: no ISO 9613-1, single global G, no reflections, no line/area sources

**Step 3: Commit**

```
git add backend/internal/standards/iso9613/indicators.go docs/phase19-iso9613-baseline.md
git commit -m "docs(iso9613): update baseline and provenance for octave-band implementation"
```
