package acoustics

import "math"

// SilenceThresholdDB is the cut-off at or below which an incoming level is read
// as the silence sentinel rather than as energy.
//
// It pairs with SilenceDB: SilenceThresholdDB is what recognises a silence
// sentinel arriving from a caller, SilenceDB is what is emitted when there is
// no energy to report. The gap between them exists so that a sentinel that has
// been attenuated on its way through a propagation path — -999 dB minus some
// shielding — is still recognised as silence rather than summed as a real, very
// quiet contribution.
const SilenceThresholdDB = -900.0

// EnergySum adds dB levels energetically, in the order given.
//
// The silence contract:
//
//   - A NaN or infinite term is skipped, as is any term at or below
//     SilenceThresholdDB. Note that the threshold is inclusive: exactly -900 dB
//     is treated as silence and dropped.
//   - If nothing is left to sum — an empty slice, an all-silent slice, or a
//     total that is not positive — the result is SilenceDB, because 10 lg is
//     undefined there.
//   - Otherwise the result is 10 lg of the summed energy.
//
// The result depends only on the order of the slice, so it is as deterministic
// as the caller's ordering.
//
// # Why this is not compensated, yet
//
// The accumulation is a plain `+=`, and that is a known deviation from
// docs/policies/determinism.md §3 ("Summation strategy"), not an oversight. The
// policy calls a reduction sensitive when its term count scales with model
// size, and several call sites here are exactly that: rls19/road and the
// cnossos/bub propagation models build one contribution per source, per
// reflection path and per line-source subsegment before calling this, so the
// slice grows with the model.
//
// Switching the loop below to numeric.CompensatedSum was measured, and it does
// change results: on the four CLI digest fixtures that exercise this helper it
// moves receiver levels by at most 2.8e-14 dB — one to two ulps — which is
// numerically negligible but not nothing, because receiver tables persist
// float64 at full round-trip precision and the run digests hash every byte. It
// therefore moves five golden snapshots (rls19-road, cnossos-road,
// cnossos-rail, bub-rail, bub-road).
//
// That makes the switch a behaviour change rather than a refactor, and it has
// to be taken, with its goldens re-cut, as its own piece of work. Until then the
// arithmetic here is byte for byte what the seven module-local copies this
// helper replaced performed. See PLAN.md Priority 7.
func EnergySum(levels []float64) float64 {
	total := 0.0

	for _, level := range levels {
		if math.IsNaN(level) || math.IsInf(level, 0) || level <= SilenceThresholdDB {
			continue
		}

		total += math.Pow(10, level/10)
	}

	if total <= 0 {
		return SilenceDB
	}

	return 10 * math.Log10(total)
}
