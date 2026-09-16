package geo

import (
	"math"
	"sync"

	"github.com/wroge/wgs84"
)

// TransverseMercator is a transverse-Mercator projection implemented as the
// Krüger n-series in Karney's formulation, carried to sixth order in the third
// flattening n. It satisfies wgs84.Projection, so it can be substituted for the
// projection the wroge/wgs84 library ships while leaving that library's datums,
// Helmert transformations and per-CRS Area predicates in place.
//
// # Why this package carries its own projection
//
// github.com/wroge/wgs84 v1.1.7 computes the radius of curvature in the
// meridian in its INVERSE (system.go:40) as
//
//	R1 := sph.A() * (1 - sph.e2()) / math.Pow(1-sph.e2()*sin2(φ1), 3/2)
//
// `3/2` is an untyped integer constant expression in Go, so the exponent is 1,
// not 1.5. R1 comes out ~0.21% too large, and it divides the leading D²/2 term
// of the footpoint-latitude correction, so the latitude error is zero on the
// central meridian and grows quadratically with easting offset: 0.49 m at
// 48 km out, 8.5 m at the edge of UTM zone 32. The forward direction of the
// same file is clean — it agrees with PROJ to sub-millimetre — which is why a
// round trip that starts in degrees (inverse∘forward) largely cancels the
// defect and never saw it. Reported upstream as
// https://github.com/wroge/wgs84/issues/30.
//
// # Why not just write 1.5
//
// Because it does not finish the job. Rebuilt with the exponent corrected, the
// library's inverse is still wrong one-way against PROJ 9.8.1 by 0.0963 m at
// the western edge of UTM zone 32 and 0.0832 m at the eastern edge — Snyder's
// truncated series, and this time almost entirely in the LONGITUDE, which R1
// never touched. A round trip on that build shows 5-8 mm of it, understating
// the one-way error by more than an order of magnitude. Same trap, second time.
//
// The Krüger series below is exact to a few nanometres over the same range in
// both directions, and being exactly reversible it lets forward and inverse
// each be pinned against external reference vectors rather than against one
// another — see testdata/README.md.
//
// The coefficients are Karney, "Transverse Mercator with an accuracy of a few
// nanometers", J. Geodesy 85(8), 2011, eqs. 12 (α) and 13 (β), and agree term
// for term with GeographicLib's order-6 `alp`/`bet` tables.
//
// LatOrigin is 0 for every CRS this package builds, but it is honoured: the
// northing is measured from the meridian arc at LatOrigin, not from the equator.
type TransverseMercator struct {
	// LonOrigin is the central meridian in degrees.
	LonOrigin float64
	// LatOrigin is the latitude of the projection origin in degrees.
	LatOrigin float64
	// Scale is the scale factor on the central meridian (k0).
	Scale float64
	// FalseEasting is added to the projected easting, in metres.
	FalseEasting float64
	// FalseNorthing is added to the projected northing, in metres.
	FalseNorthing float64
}

var _ wgs84.Projection = TransverseMercator{}

// tmSeriesOrder is the truncation order of the Krüger series in the third
// flattening n. Order 6 is Karney's recommendation for double precision: the
// neglected terms are O(n⁷) ≈ 1e-19 of the rectifying radius, far below the
// rounding of a float64 metre.
const tmSeriesOrder = 6

// tmTauMaxIterations bounds the Newton solve that recovers the geodetic
// latitude from the conformal latitude. The iteration is quadratically
// convergent from a starting guess that is already correct to O(e²), so two
// steps suffice; the bound exists so a pathological spheroid cannot spin.
const tmTauMaxIterations = 8

// tmTauTolerance is the relative step size at which that solve stops. It is
// √ε/10; because convergence is quadratic, a step this small leaves a result
// accurate to about its square, which is below float64 resolution.
const tmTauTolerance = 1.5e-9

// tmSeries holds the parts of the projection that depend only on the spheroid.
// Computing them is several times the cost of projecting one point, and the
// transform path runs per vertex and per terrain lookup, so they are cached.
type tmSeries struct {
	// ecc is the first eccentricity of the spheroid.
	ecc float64
	// rectifyingRadius is Karney's A: the radius of the sphere whose
	// circumference equals the meridian circumference of the spheroid.
	rectifyingRadius float64
	// alpha maps the conformal to the rectifying latitude (forward).
	alpha [tmSeriesOrder]float64
	// beta is the inverse of alpha (inverse direction).
	beta [tmSeriesOrder]float64
}

// tmSeriesCache memoises tmSeries by spheroid. The key is the pair the
// wgs84.Spheroid interface exposes, which is all newTMSeries reads.
var tmSeriesCache sync.Map

type tmSeriesKey struct {
	semiMajorAxis     float64
	inverseFlattening float64
}

// seriesFor returns the cached Krüger series for a spheroid.
func seriesFor(spheroid wgs84.Spheroid) tmSeries {
	key := tmSeriesKey{semiMajorAxis: spheroid.A(), inverseFlattening: spheroid.Fi()}
	if cached, ok := tmSeriesCache.Load(key); ok {
		series, _ := cached.(tmSeries)
		return series
	}

	series := newTMSeries(key.semiMajorAxis, key.inverseFlattening)
	tmSeriesCache.Store(key, series)

	return series
}

// newTMSeries evaluates the spheroid-dependent constants of the Krüger series.
//
// The α and β polynomials are written as Horner forms in the third flattening
// n = f/(2-f) so each rational coefficient can be read straight off Karney's
// eqs. 12 and 13.
func newTMSeries(semiMajorAxis, inverseFlattening float64) tmSeries {
	flattening := 0.0
	if inverseFlattening != 0 {
		flattening = 1 / inverseFlattening
	}

	n := flattening / (2 - flattening)
	n2 := n * n

	series := tmSeries{
		ecc: math.Sqrt(flattening * (2 - flattening)),
		// A = a/(1+n) · (1 + n²/4 + n⁴/64 + n⁶/256)
		rectifyingRadius: semiMajorAxis / (1 + n) *
			(1 + n2*(1.0/4+n2*(1.0/64+n2*(1.0/256)))),
	}

	series.alpha = [tmSeriesOrder]float64{
		n * (1.0/2 + n*(-2.0/3+n*(5.0/16+n*(41.0/180+n*(-127.0/288+n*(7891.0/37800)))))),
		n2 * (13.0/48 + n*(-3.0/5+n*(557.0/1440+n*(281.0/630+n*(-1983433.0/1935360))))),
		n2 * n * (61.0/240 + n*(-103.0/140+n*(15061.0/26880+n*(167603.0/181440)))),
		n2 * n2 * (49561.0/161280 + n*(-179.0/168+n*(6601661.0/7257600))),
		n2 * n2 * n * (34729.0/80640 + n*(-3418889.0/1995840)),
		n2 * n2 * n2 * (212378941.0 / 319334400),
	}

	series.beta = [tmSeriesOrder]float64{
		n * (1.0/2 + n*(-2.0/3+n*(37.0/96+n*(-1.0/360+n*(-81.0/512+n*(96199.0/604800)))))),
		n2 * (1.0/48 + n*(1.0/15+n*(-437.0/1440+n*(46.0/105+n*(-1118711.0/3870720))))),
		n2 * n * (17.0/480 + n*(-37.0/840+n*(-209.0/4480+n*(5569.0/90720)))),
		n2 * n2 * (4397.0/161280 + n*(-11.0/504+n*(-830251.0/7257600))),
		n2 * n2 * n * (4583.0/161280 + n*(-108847.0/3991680)),
		n2 * n2 * n2 * (20648693.0 / 638668800),
	}

	return series
}

// originXi returns the rectifying latitude ξ of latOrigin on the central
// meridian, in Karney's normalised units. Multiplied by Scale·A it is the
// meridian arc the northing is measured from.
func (s tmSeries) originXi(latOriginDeg float64) float64 {
	if latOriginDeg == 0 {
		return 0
	}

	// On the central meridian η' is zero, so ξ' reduces to atan of the
	// conformal-latitude tangent and the cosh factors are all 1.
	xiPrime := math.Atan(s.taupFromTau(math.Tan(latOriginDeg * math.Pi / 180)))

	xi := xiPrime
	for j := 1; j <= tmSeriesOrder; j++ {
		xi += s.alpha[j-1] * math.Sin(float64(2*j)*xiPrime)
	}

	return xi
}

// eAtanhE evaluates e·atanh(e·x), the kernel of the conformal-latitude relation.
func (s tmSeries) eAtanhE(x float64) float64 {
	return s.ecc * math.Atanh(s.ecc*x)
}

// taupFromTau converts tan(geodetic latitude) to tan(conformal latitude).
// Karney eq. 7, in the numerically stable form GeographicLib uses.
func (s tmSeries) taupFromTau(tau float64) float64 {
	tau1 := math.Hypot(1, tau)
	sigma := math.Sinh(s.eAtanhE(tau / tau1))

	return math.Hypot(1, sigma)*tau - sigma*tau1
}

// tauFromTaup inverts taupFromTau by Newton's method. To lowest order in e²,
// taup = (1-e²)·tau, which is the starting guess.
func (s tmSeries) tauFromTaup(taup float64) float64 {
	oneMinusE2 := 1 - s.ecc*s.ecc
	if oneMinusE2 == 0 {
		return taup
	}

	tau := taup / oneMinusE2
	tolerance := tmTauTolerance * math.Max(1, math.Abs(taup))

	for range tmTauMaxIterations {
		taupOfTau := s.taupFromTau(tau)
		delta := (taup - taupOfTau) * (1 + oneMinusE2*tau*tau) /
			(oneMinusE2 * math.Hypot(1, tau) * math.Hypot(1, taupOfTau))
		tau += delta

		if math.Abs(delta) < tolerance {
			break
		}
	}

	return tau
}

// FromLonLat projects geodetic coordinates on s onto the plane.
// It implements wgs84.Projection.
func (p TransverseMercator) FromLonLat(lon, lat float64, s wgs84.Spheroid) (east, north float64) {
	series := seriesFor(s)

	lambda := normalizeLonDelta(lon-p.LonOrigin) * math.Pi / 180
	taup := series.taupFromTau(math.Tan(lat * math.Pi / 180))

	cosLambda := math.Cos(lambda)
	xiPrime := math.Atan2(taup, cosLambda)
	etaPrime := math.Asinh(math.Sin(lambda) / math.Hypot(taup, cosLambda))

	xi, eta := xiPrime, etaPrime

	for j := 1; j <= tmSeriesOrder; j++ {
		angle := float64(2 * j)
		xi += series.alpha[j-1] * math.Sin(angle*xiPrime) * math.Cosh(angle*etaPrime)
		eta += series.alpha[j-1] * math.Cos(angle*xiPrime) * math.Sinh(angle*etaPrime)
	}

	scaled := p.Scale * series.rectifyingRadius
	east = p.FalseEasting + scaled*eta
	north = p.FalseNorthing + scaled*(xi-series.originXi(p.LatOrigin))

	return east, north
}

// ToLonLat unprojects plane coordinates back to geodetic coordinates on s.
// It implements wgs84.Projection.
func (p TransverseMercator) ToLonLat(east, north float64, s wgs84.Spheroid) (lon, lat float64) {
	series := seriesFor(s)

	scaled := p.Scale * series.rectifyingRadius
	eta := (east - p.FalseEasting) / scaled
	xi := (north-p.FalseNorthing)/scaled + series.originXi(p.LatOrigin)

	xiPrime, etaPrime := xi, eta

	for j := 1; j <= tmSeriesOrder; j++ {
		angle := float64(2 * j)
		xiPrime -= series.beta[j-1] * math.Sin(angle*xi) * math.Cosh(angle*eta)
		etaPrime -= series.beta[j-1] * math.Cos(angle*xi) * math.Sinh(angle*eta)
	}

	sinXiPrime, cosXiPrime := math.Sincos(xiPrime)
	sinhEtaPrime := math.Sinh(etaPrime)

	// tan of the conformal latitude; the denominator is cosh²η' - sin²ξ'
	// rewritten so it stays positive and loses no precision near the poles.
	taup := sinXiPrime / math.Hypot(sinhEtaPrime, cosXiPrime)

	lat = math.Atan(series.tauFromTaup(taup)) * 180 / math.Pi
	lon = p.LonOrigin + math.Atan2(sinhEtaPrime, cosXiPrime)*180/math.Pi

	return lon, lat
}

// normalizeLonDelta folds a longitude difference into (-180, 180] degrees so a
// point just across the antimeridian from the central meridian does not project
// as if it were most of the way around the world.
func normalizeLonDelta(delta float64) float64 {
	folded := math.Mod(delta, 360)
	switch {
	case folded > 180:
		return folded - 360
	case folded <= -180:
		return folded + 360
	default:
		return folded
	}
}
