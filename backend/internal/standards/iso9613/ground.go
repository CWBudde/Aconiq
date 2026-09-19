package iso9613

import (
	"math"

	"github.com/aconiq/backend/internal/geo"
)

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
// ISO 9613-2:1996 Table 3 gives A_m = -3q at 63 Hz and A_m = -3q(1 - G_m)
// from 125 Hz to 8 kHz; both expressions are negative for q > 0.
func middleRegionAtten(gm float64, band int, q float64) float64 {
	switch band {
	case 0: // 63 Hz
		return -3 * q
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

// ResolveRegionFactors resolves the ground factors of one propagation path
// from the project's ground zones, falling back to the global factor over
// ground no zone covers.
//
// The results are returned in GroundEffectBands' own argument order — source,
// receiver, middle — rather than in path order, so that the two calls cannot
// be wired up transposed.
//
// ISO 9613-2:1996 Abschnitt 7.3.1 splits the projected path into three
// regions: the source region reaching 30·h_s from the source towards the
// receiver, the receiver region reaching 30·h_r back from the receiver, and
// the middle region between them. Each region's G is the porous fraction of
// its ground, so mixed ground resolves to the **length-weighted mean** of the
// zone factors over that region's own span, not to the factor under one probe
// point — a probe at the source would let a 1 m puddle at its feet speak for
// 30·h_s of ground.
//
// Where the two end regions meet or overlap there is no middle region, and
// middleRegionQ returns q = 0 for exactly the same inputs, so A_m vanishes and
// the returned gm is unused; it carries the fallback rather than a number that
// looks computed.
//
// With no zones every region resolves to fallback and the result is what
// passing the global factor three times produced.
func ResolveRegionFactors(source, receiver geo.Point2D, hs, hr float64, zones []GroundZone, fallback float64) (gs, gr, gm float64) {
	if len(zones) == 0 {
		return fallback, fallback, fallback
	}

	polygons := make([][][]geo.Point2D, 0, len(zones))
	for _, zone := range zones {
		polygons = append(polygons, zone.Polygon)
	}

	spans := geo.SegmentPolygonSpans(source, receiver, polygons)

	dp := geo.Distance(source, receiver)
	if dp <= 0 {
		atSource := weightedFactor(spans, zones, 0, 1, fallback)

		return atSource, atSource, fallback
	}

	// Regions are expressed in the same parameter t the spans use, so the
	// weighting needs no second unit: t = distance along the path / d_p.
	sourceRegionEnd := math.Min(30*hs, dp) / dp
	receiverRegionStart := 1 - math.Min(30*hr, dp)/dp

	gs = weightedFactor(spans, zones, 0, sourceRegionEnd, fallback)
	gr = weightedFactor(spans, zones, receiverRegionStart, 1, fallback)

	gm = fallback
	if receiverRegionStart > sourceRegionEnd {
		gm = weightedFactor(spans, zones, sourceRegionEnd, receiverRegionStart, fallback)
	}

	return gs, gr, gm
}

// weightedFactor returns the length-weighted mean ground factor over the
// parameter interval [from, to] of a path the spans describe.
//
// Spans are visited in path order and each contributes its own overlap, so the
// sum does not depend on the order the zones happen to sit in — only on which
// zone covers which stretch, which SegmentPolygonSpans resolves by slice order.
func weightedFactor(spans []geo.SegmentSpan, zones []GroundZone, from, to float64, fallback float64) float64 {
	if to-from <= 0 {
		return fallback
	}

	var weighted, covered float64

	for _, span := range spans {
		overlap := math.Min(span.End, to) - math.Max(span.Start, from)
		if overlap <= 0 {
			continue
		}

		factor := fallback
		if span.Polygon >= 0 {
			factor = zones[span.Polygon].GroundFactor
		}

		weighted += overlap * factor
		covered += overlap
	}

	if covered <= 0 {
		return fallback
	}

	return weighted / covered
}
