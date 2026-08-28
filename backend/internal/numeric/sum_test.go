package numeric_test

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/numeric"
)

// TestCompensatedSumRecoversCancelledTerms uses the classic case in which the
// naive running sum loses every small term to rounding against a large one.
func TestCompensatedSumRecoversCancelledTerms(t *testing.T) {
	t.Parallel()

	terms := []float64{1, 1e100, 1, -1e100}

	naive := 0.0
	for _, term := range terms {
		naive += term
	}

	if naive == 2 {
		t.Fatal("the naive sum is exact here, so this case proves nothing")
	}

	var acc numeric.CompensatedSum
	for _, term := range terms {
		acc.Add(term)
	}

	if got := acc.Sum(); got != 2 {
		t.Errorf("CompensatedSum.Sum() = %v, want 2 (naive sum was %v)", got, naive)
	}
}

// TestCompensatedSumMatchesLongRunningTotal checks the case the acoustics code
// actually meets: many same-signed terms of very different magnitude, as when a
// long line source is integrated subsegment by subsegment.
func TestCompensatedSumMatchesLongRunningTotal(t *testing.T) {
	t.Parallel()

	const (
		terms = 100_000
		small = 1e-9
	)

	var acc numeric.CompensatedSum

	acc.Add(1)

	naive := 1.0

	for range terms {
		acc.Add(small)

		naive += small
	}

	want := 1 + terms*small

	if got := math.Abs(acc.Sum() - want); got > 1e-15 {
		t.Errorf("CompensatedSum error = %v, want <= 1e-15", got)
	}

	if math.Abs(naive-want) <= math.Abs(acc.Sum()-want) {
		t.Errorf("naive error %v did not exceed compensated error %v",
			math.Abs(naive-want), math.Abs(acc.Sum()-want))
	}
}

// TestCompensatedSumZeroValueIsUsable pins that the accumulator needs no
// constructor, so it can be declared inline in a reduction loop.
func TestCompensatedSumZeroValueIsUsable(t *testing.T) {
	t.Parallel()

	var acc numeric.CompensatedSum

	if got := acc.Sum(); got != 0 {
		t.Errorf("zero CompensatedSum.Sum() = %v, want 0", got)
	}
}

// TestSumMatchesAccumulator pins the convenience wrapper to the accumulator.
func TestSumMatchesAccumulator(t *testing.T) {
	t.Parallel()

	terms := []float64{1, 1e100, 1, -1e100}

	if got := numeric.Sum(terms...); got != 2 {
		t.Errorf("Sum(%v) = %v, want 2", terms, got)
	}

	if got := numeric.Sum(); got != 0 {
		t.Errorf("Sum() = %v, want 0", got)
	}
}
