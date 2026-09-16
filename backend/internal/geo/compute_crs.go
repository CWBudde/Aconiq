package geo

import (
	"fmt"
	"math"
)

// The ETRS89 / UTM zone codes this project supports, indexed by zone number.
// EPSG:25831..25834 cover 0°E..24°E, which spans the whole DACH region the
// normative modules are written for.
const (
	minSupportedUTMZone = 31
	maxSupportedUTMZone = 34
	etrs89UTMZoneBase   = 25800
)

// UTMZoneForLongitude returns the UTM zone a longitude falls in. The result is
// mathematical, not a support claim: ComputeCRSForGeographic decides whether
// the zone has a CRS this project can transform into.
func UTMZoneForLongitude(lonDeg float64) int {
	zone := int(math.Floor((lonDeg+180)/6)) + 1

	// A longitude of exactly +180 lands one past the last zone.
	zone = min(zone, 60)
	zone = max(zone, 1)

	return zone
}

// ComputeCRSForGeographic picks the metric CRS a geographic model has to be
// projected into before anything computes distances in it.
//
// It exists because the whole compute path is planar and metric: geo.Distance
// is math.Hypot over stored coordinates, so a model held in degrees collapses
// every propagation distance to the modules' minimum-distance clamp and each
// receiver reports the source's emission level verbatim. Projecting first is
// what makes those distances metres again.
//
// The target is always ETRS89 / UTM, for both EPSG:4326 and EPSG:4258 inputs.
// The two datums agree to well under a metre in Europe and the difference is a
// near-rigid shift of the whole model, so it moves no distance between two
// features by an acoustically meaningful amount — whereas maintaining a second
// WGS84 / UTM table would only cover zones 32 and 33 (EPSG:32632, 32633) and
// refuse zones 31 and 34 that ETRS89 supports.
//
// A site outside zones 31-34 or south of the equator is refused rather than
// projected into a zone the CRS table does not carry. Computing at the wrong
// place is the one outcome that is not available here.
func ComputeCRSForGeographic(lonDeg, latDeg float64) (CRS, error) {
	if math.IsNaN(lonDeg) || math.IsInf(lonDeg, 0) || math.IsNaN(latDeg) || math.IsInf(latDeg, 0) {
		return CRS{}, fmt.Errorf("model centre (%v, %v) is not a finite coordinate", lonDeg, latDeg)
	}

	if lonDeg < -180 || lonDeg > 180 || latDeg < -90 || latDeg > 90 {
		return CRS{}, fmt.Errorf("model centre (%.6f, %.6f) is outside the lon/lat range, so the project CRS is not the geographic CRS it declares", lonDeg, latDeg)
	}

	if latDeg < 0 {
		return CRS{}, fmt.Errorf("model centre latitude %.6f is in the southern hemisphere; only the northern ETRS89 / UTM zones are available", latDeg)
	}

	zone := UTMZoneForLongitude(lonDeg)
	if zone < minSupportedUTMZone || zone > maxSupportedUTMZone {
		return CRS{}, fmt.Errorf(
			"model centre longitude %.6f falls in UTM zone %d; a geographic project CRS can only be computed in zones %d-%d (EPSG:%d-%d). Set the project CRS to a metric CRS this model is actually in",
			lonDeg, zone, minSupportedUTMZone, maxSupportedUTMZone,
			etrs89UTMZoneBase+minSupportedUTMZone, etrs89UTMZoneBase+maxSupportedUTMZone,
		)
	}

	return ParseCRS(fmt.Sprintf("EPSG:%d", etrs89UTMZoneBase+zone))
}
