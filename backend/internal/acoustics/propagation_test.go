package acoustics

import (
	"math"
	"testing"
)

func TestGeometricDivergenceFollowsTheInverseSquareLaw(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		distanceM float64
		want      float64
	}{
		{
			// At the reference distance of 1 m the logarithm vanishes and only
			// the hemispherical constant is left. A change to the constant
			// breaks this case and nothing else would catch it.
			name:      "the reference distance reports the bare constant",
			distanceM: 1,
			want:      11,
		},
		{
			name:      "ten metres costs twenty decibels more",
			distanceM: 10,
			want:      31,
		},
		{
			name:      "one hundred metres costs forty decibels more",
			distanceM: 100,
			want:      51,
		},
		{
			// Doubling the distance is the inverse square law's 6.02 dB.
			name:      "doubling the distance adds six decibels",
			distanceM: 2,
			want:      11 + 20*math.Log10(2),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := GeometricDivergence(testCase.distanceM)
			if math.Abs(got-testCase.want) > 1e-12 {
				t.Fatalf("unexpected divergence: got %v want %v", got, testCase.want)
			}
		})
	}
}

func TestGeometricDivergenceIsUnguardedAtZero(t *testing.T) {
	t.Parallel()

	// The clamp to a minimum propagation distance belongs to the caller, so a
	// zero distance is a caller's bug and is reported rather than corrected.
	if got := GeometricDivergence(0); !math.IsInf(got, -1) {
		t.Fatalf("expected -Inf at zero distance, got %v", got)
	}
}

func TestAirAbsorptionScalesLinearlyWithDistance(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name               string
		coefficientDBPerKM float64
		distanceM          float64
		want               float64
	}{
		{
			name:               "one kilometre costs the whole coefficient",
			coefficientDBPerKM: 0.7,
			distanceM:          1000,
			want:               0.7,
		},
		{
			// Not 0.07: 100/1000 is not exactly a tenth in binary, so the
			// product lands one ulp below. Written out rather than rounded
			// away, because that last bit is hashed into every run digest.
			name:               "one hundred metres costs a tenth of it",
			coefficientDBPerKM: 0.7,
			distanceM:          100,
			want:               0.06999999999999999,
		},
		{
			name:               "a zero coefficient costs nothing at any distance",
			coefficientDBPerKM: 0,
			distanceM:          12345,
			want:               0,
		},
		{
			name:               "a zero distance costs nothing at any coefficient",
			coefficientDBPerKM: 0.7,
			distanceM:          0,
			want:               0,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := AirAbsorption(testCase.coefficientDBPerKM, testCase.distanceM)
			if got != testCase.want {
				t.Fatalf("AirAbsorption(%v, %v) = %v, want %v", testCase.coefficientDBPerKM, testCase.distanceM, got, testCase.want)
			}
		})
	}
}

// TestAirAbsorptionDividesBeforeMultiplying pins the parenthesisation rather
// than the value. a * (d / 1000) and (a * d) / 1000 differ in the last ulp for
// inputs like these, receiver tables persist float64 at full precision, and the
// run digests hash every byte — so a "simplification" here moves goldens. The
// per-module unit tests all compare with a 1e-9 tolerance and would not catch
// it; this is the only assertion that does.
func TestAirAbsorptionDividesBeforeMultiplying(t *testing.T) {
	t.Parallel()

	// 0.7 dB/km is the default air absorption of every scaffold that carried a
	// copy of this function, and 3 m is cnossos/road's default min_distance_m,
	// so this is a pair the shipped configuration really produces.
	// Variables, not constants: untyped constant arithmetic is folded at
	// arbitrary precision, so both associations would agree at compile time and
	// the test would prove nothing.
	coefficient := 0.7
	distance := 3.0

	divideFirst := coefficient * (distance / 1000.0)
	multiplyFirst := (coefficient * distance) / 1000.0

	if divideFirst == multiplyFirst {
		t.Fatalf("the chosen constants no longer discriminate the two associations; pick another pair")
	}

	if got := AirAbsorption(coefficient, distance); got != divideFirst {
		t.Fatalf("AirAbsorption divided after multiplying: got %v, want %v", got, divideFirst)
	}
}

func TestClampDistanceRaisesOnlyBelowTheMinimum(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		distanceM float64
		minimumM  float64
		want      float64
	}{
		{
			name:      "a distance below the minimum is raised to it",
			distanceM: 1,
			minimumM:  3,
			want:      3,
		},
		{
			name:      "a distance at the minimum is left alone",
			distanceM: 3,
			minimumM:  3,
			want:      3,
		},
		{
			name:      "a distance above the minimum is left alone",
			distanceM: 10,
			minimumM:  3,
			want:      10,
		},
		{
			// Every caller's Validate rejects a non-positive minimum, so this
			// only records that the clamp does not invent one of its own.
			name:      "a zero distance is raised to the minimum",
			distanceM: 0,
			minimumM:  20,
			want:      20,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := ClampDistance(testCase.distanceM, testCase.minimumM)
			if got != testCase.want {
				t.Fatalf("ClampDistance(%v, %v) = %v, want %v", testCase.distanceM, testCase.minimumM, got, testCase.want)
			}
		})
	}
}

// TestClampDistancePassesNaNThrough records the one input on which the three
// spellings this function replaced could have disagreed. math.Max(NaN, m)
// returns NaN and NaN < m is false, so both return NaN — the compare form kept
// here agrees with the math.Max form the two aircraft modules used.
func TestClampDistancePassesNaNThrough(t *testing.T) {
	t.Parallel()

	if got := ClampDistance(math.NaN(), 20); !math.IsNaN(got) {
		t.Fatalf("ClampDistance(NaN, 20) = %v, want NaN", got)
	}
}
