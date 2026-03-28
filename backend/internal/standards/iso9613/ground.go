package iso9613

import "math"

// Table 3 functions for ground attenuation contributions.
// These are from ISO 9613-2:1996, Table 3 notes.

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
// g is the ground factor for that region, h is hs or hr, dp is the
// projected source-receiver distance.
func sourceReceiverAtten(g, h, dp float64, band int) float64 {
	switch band {
	case 0: // 63 Hz
		return -1.5
	case 1: // 125 Hz
		return -1.5 + g*aPrime(h, dp)
	case 2: // 250 Hz
		return -1.5 + g*bPrime(h, dp)
	case 3: // 500 Hz
		return -1.5 + g*cPrime(h, dp)
	case 4: // 1000 Hz
		return -1.5 + g*dPrime(h, dp)
	case 5, 6, 7: // 2000, 4000, 8000 Hz
		return -1.5 * (1 - g)
	default:
		return 0
	}
}

// middleRegionAtten computes A_m from Table 3 for one band.
func middleRegionAtten(gm float64, band int, q float64) float64 {
	switch band {
	case 0: // 63 Hz
		return 3 * q
	default: // 125-8000 Hz
		return -3 * q * (1 - gm)
	}
}

// GroundEffectBands computes A_gr per octave band using the general method
// (Eq. 9, Table 3). gs, gr, gm are the ground factors for the source,
// receiver, and middle regions. hs and hr are source and receiver heights.
// dp is the projected source-receiver distance.
func GroundEffectBands(gs, gr, gm, hs, hr, dp float64) BandLevels {
	q := middleRegionQ(hs, hr, dp)

	var result BandLevels

	for i := range NumBands {
		as := sourceReceiverAtten(gs, hs, dp, i)
		ar := sourceReceiverAtten(gr, hr, dp, i)
		am := middleRegionAtten(gm, i, q)
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
