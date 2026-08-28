// Package numeric holds the shared floating-point reduction helpers the
// standards modules use where docs/policies/determinism.md §3 requires a stable
// summation strategy.
package numeric

import "math"

// CompensatedSum accumulates float64 terms using Neumaier's variant of the
// Kahan compensated summation algorithm.
//
// A plain `sum += term` loses the low bits of every term that is small relative
// to the running total.  Aconiq's acoustic reductions hit exactly that shape:
// a line source is integrated subsegment by subsegment, so the term count grows
// with model size while the terms themselves span many orders of magnitude
// (a distant subsegment contributes a small fraction of a nearby one, and a
// silenced period contributes 1e-100).
//
// Neumaier's variant is used rather than plain Kahan because it stays exact
// when a term is larger than the running total, which plain Kahan does not.
//
// The result depends only on the order in which terms are added, so a
// CompensatedSum is as deterministic as the loop that feeds it: worker count
// cannot change it as long as the partition order is fixed, which is what
// docs/policies/determinism.md requires.
//
// The zero value is an empty sum and is ready to use.
type CompensatedSum struct {
	sum          float64
	compensation float64
}

// Add accumulates one term.
func (c *CompensatedSum) Add(term float64) {
	t := c.sum + term

	if math.Abs(c.sum) >= math.Abs(term) {
		// The low bits lost are those of term.
		c.compensation += (c.sum - t) + term
	} else {
		// term dominates, so the low bits lost are those of the running sum.
		c.compensation += (term - t) + c.sum
	}

	c.sum = t
}

// Sum returns the accumulated total including the compensation term.
func (c *CompensatedSum) Sum() float64 {
	return c.sum + c.compensation
}

// Sum adds terms in the given order using CompensatedSum. It is the
// convenience form for reductions over a slice that is already materialised;
// prefer the accumulator inside a loop that generates its terms.
func Sum(terms ...float64) float64 {
	var acc CompensatedSum

	for _, term := range terms {
		acc.Add(term)
	}

	return acc.Sum()
}
