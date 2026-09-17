package acoustics

import (
	"math"
	"testing"
)

func TestEnergySumFollowsTheSilenceContract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		levels []float64
		want   float64
	}{
		{
			name:   "nothing to sum reports the silence sentinel",
			levels: nil,
			want:   SilenceDB,
		},
		{
			name:   "an all-silent slice reports the silence sentinel",
			levels: []float64{SilenceDB, SilenceDB, -1200},
			want:   SilenceDB,
		},
		{
			// The threshold is inclusive: exactly -900 dB is read as a silence
			// sentinel, not as 1e-90 of energy. This is the boundary the seven
			// module-local copies this helper replaced all used.
			name:   "the threshold itself is silence",
			levels: []float64{-900},
			want:   SilenceDB,
		},
		{
			// Just above the threshold is a real, absurdly quiet level and is
			// summed, so the single term passes through unchanged.
			name:   "just above the threshold is energy",
			levels: []float64{-899.9},
			want:   -899.9,
		},
		{
			name:   "a single level passes through",
			levels: []float64{55},
			want:   55,
		},
		{
			name:   "two equal levels gain 3 dB",
			levels: []float64{60, 60},
			want:   63.01029995663981,
		},
		{
			// NaN and both infinities are skipped, so what is left is the pair
			// above and the answer must be identical to it.
			name:   "non-finite terms are skipped",
			levels: []float64{60, math.NaN(), math.Inf(1), 60, math.Inf(-1)},
			want:   63.01029995663981,
		},
		{
			name:   "silence mixed into real levels does not shift them",
			levels: []float64{60, SilenceDB, 60, -1000},
			want:   63.01029995663981,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := EnergySum(testCase.levels)
			if math.Abs(got-testCase.want) > 1e-12 {
				t.Fatalf("EnergySum(%v) = %v, want %v", testCase.levels, got, testCase.want)
			}
		})
	}
}

// TestEnergySumLosesTermsAnUncompensatedSumMustLose pins the one place where
// the helper's arithmetic is knowingly weaker than docs/policies/determinism.md
// §3 asks for, so that the deviation has a test rather than only a comment.
//
// It builds the case the policy names: many small terms accumulated onto a
// total large enough that each one falls off the bottom of the mantissa. A
// plain `sum += term` loses every single one of them; Neumaier compensation
// would not. See EnergySum's doc comment for why the compensated version is not
// in place yet — when it is, this test inverts, and that inversion is the
// signal that five golden snapshots have to move with it.
func TestEnergySumLosesTermsAnUncompensatedSumMustLose(t *testing.T) {
	t.Parallel()

	// 160 dB is 1e16 of energy, whose ulp is 2. Adding 1e0 to it rounds back to
	// 1e16 every time, so summing one loud level and two thousand quiet ones
	// uncompensated returns exactly the loud level.
	levels := make([]float64, 0, 2001)
	levels = append(levels, 160)

	for range 2000 {
		levels = append(levels, 0)
	}

	got := EnergySum(levels)

	if got != 160 {
		t.Fatalf("EnergySum = %v, want exactly 160: the uncompensated total drops every quiet term", got)
	}

	// What a compensated reduction over the same terms in the same order would
	// have returned instead. The gap is ~8.7e-13 dB.
	compensated := 10 * math.Log10(1e16+2000)
	if compensated <= got {
		t.Fatalf("the fixture no longer demonstrates the loss: compensated %v, uncompensated %v", compensated, got)
	}
}
