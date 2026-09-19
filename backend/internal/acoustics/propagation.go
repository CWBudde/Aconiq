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
