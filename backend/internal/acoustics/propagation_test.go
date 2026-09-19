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
