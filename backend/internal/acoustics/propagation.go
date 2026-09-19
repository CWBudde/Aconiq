package acoustics

import "math"

// GeometricDivergence is the spreading loss of a point source radiating into a
// free hemisphere: A_div = 20 lg(d / d0) + 11 dB, with the reference distance
// d0 = 1 m. It is ISO 9613-2 equation (7), and the same term every mapping
// scaffold in the repository applies to its own effective distance.
//
// It is deliberately unguarded. Each module clamps the distance to its own
// configured minimum before calling — MinDistanceM, or MinSlantDistanceM for
// the aircraft modules — so a non-positive distance arriving here is a caller's
// bug and is reported as -Inf or NaN rather than silently corrected to
// something plausible.
func GeometricDivergence(distanceM float64) float64 {
	return 20*math.Log10(distanceM) + 11.0
}

// AirAbsorption is the atmospheric attenuation of a single broadband
// coefficient applied over a distance: A_atm = alpha * d, with alpha in dB/km
// and d in metres. Six mapping scaffolds spelled it out identically and
// rls19/road spelled it out against a package constant instead of a config
// field, which is why the coefficient is a parameter here rather than a config
// struct: this package cannot import internal/standards.
//
// The parenthesisation is load-bearing. a * (d / 1000) and (a * d) / 1000
// differ in the last ulp, receiver tables persist float64 at full round-trip
// precision, and the run digests hash every byte of them. The form below is
// the one all seven call sites had.
//
// It is unguarded for the same reason GeometricDivergence is: the caller has
// already clamped the distance.
func AirAbsorption(coefficientDBPerKM, distanceM float64) float64 {
	return coefficientDBPerKM * (distanceM / 1000.0)
}

// ClampDistance raises a propagation distance to a configured minimum, which is
// what keeps GeometricDivergence away from its singularity at d = 0.
//
// Eleven call sites did this, under three names and in three spellings: a
// direct compare, the same with a dead local, and math.Max. The compare is the
// form kept, because five of the seven named copies used it and because it is
// the one that does not quietly normalise a negative zero. The three agree on
// every input a module's Validate admits — each requires its minimum finite and
// strictly positive — so choosing between them moves no number.
func ClampDistance(distanceM, minimumM float64) float64 {
	if distanceM < minimumM {
		return minimumM
	}

	return distanceM
}
