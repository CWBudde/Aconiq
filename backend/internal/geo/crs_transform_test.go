package geo

import (
	"fmt"
	"math"
	"testing"
)

// Reference coordinate pairs verified against PROJ cs2cs and/or known survey points.
// Convention: geographic CRS uses (lon, lat), projected CRS uses (easting, northing).

func TestEPSGTransform_WGS84_to_UTM32(t *testing.T) {
	t.Parallel()

	from, _ := ParseCRS("EPSG:4326")
	to, _ := ParseCRS("EPSG:25832")

	pipeline, err := BuildTransformPipeline(to, from)
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}

	// Hannover city center: lon=9.7320, lat=52.3759
	out, err := pipeline.ApplyPoint(Point2D{X: 9.7320, Y: 52.3759})
	if err != nil {
		t.Fatalf("transform: %v", err)
	}

	// Library output verified against cs2cs. Sub-meter tolerance.
	if math.Abs(out.X-549830) > 1.0 || math.Abs(out.Y-5803100) > 1.0 {
		t.Fatalf("unexpected UTM32 result: easting=%.2f, northing=%.2f", out.X, out.Y)
	}
}

func TestEPSGTransform_UTM32_to_WGS84(t *testing.T) {
	t.Parallel()

	from, _ := ParseCRS("EPSG:25832")
	to, _ := ParseCRS("EPSG:4326")

	pipeline, err := BuildTransformPipeline(to, from)
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}

	// Reverse of the forward test: take the UTM32 output and recover WGS84.
	out, err := pipeline.ApplyPoint(Point2D{X: 549830, Y: 5803100})
	if err != nil {
		t.Fatalf("transform: %v", err)
	}

	// Sub-arcsecond tolerance (~0.001 degree ≈ 100m, but roundtrip is much tighter).
	if math.Abs(out.X-9.7320) > 0.001 || math.Abs(out.Y-52.3759) > 0.001 {
		t.Fatalf("unexpected WGS84 result: lon=%.6f, lat=%.6f", out.X, out.Y)
	}
}

func TestEPSGTransform_UTM32_to_UTM33(t *testing.T) {
	t.Parallel()

	from, _ := ParseCRS("EPSG:25832")
	to, _ := ParseCRS("EPSG:25833")

	pipeline, err := BuildTransformPipeline(to, from)
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}

	// A point in UTM32 near the zone boundary (12°E).
	out, err := pipeline.ApplyPoint(Point2D{X: 700000, Y: 5800000})
	if err != nil {
		t.Fatalf("transform: %v", err)
	}

	// In UTM33, the easting should be lower (west of central meridian 15°E).
	if out.X <= 0 || out.Y <= 0 {
		t.Fatalf("unexpected result: easting=%.2f, northing=%.2f", out.X, out.Y)
	}

	if math.Abs(out.Y-5800000) > 10000 {
		t.Fatalf("northing shifted too much: %.2f", out.Y)
	}
}

func TestEPSGTransform_GaussKruger_to_UTM32(t *testing.T) {
	t.Parallel()

	from, _ := ParseCRS("EPSG:31467") // DHDN GK zone 3
	to, _ := ParseCRS("EPSG:25832")   // ETRS89 UTM 32N

	pipeline, err := BuildTransformPipeline(to, from)
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}

	// Hannover in GK3: R≈3549xxx, H≈5804xxx (zone prefix 3 in Rechtswert).
	out, err := pipeline.ApplyPoint(Point2D{X: 3549590, Y: 5804240})
	if err != nil {
		t.Fatalf("transform: %v", err)
	}

	// GK→UTM involves a Helmert datum shift (DHDN→ETRS89). Expect coordinates in
	// the UTM32 range with a datum-shift offset of ~100m from naive zone conversion.
	if out.X < 549000 || out.X > 550500 || out.Y < 5802000 || out.Y > 5804500 {
		t.Fatalf("GK3->UTM32 result out of expected range: easting=%.2f, northing=%.2f", out.X, out.Y)
	}
}

// TestEPSGTransform_Roundtrip checks that a transform composed with its own
// inverse returns the point it started from. That is a weak property — it is
// satisfied by any pair of errors that cancel — so it is not where projection
// accuracy is established. TestTransverseMercatorForwardMatchesPROJ and
// TestTransverseMercatorInverseMatchesPROJ do that, one direction at a time,
// against PROJ 9.8.1 reference vectors.
//
// The tolerances below are still worth having tight, because a slack one costs
// nothing to leave in place and hides whatever it is wider than. They are
// measured with roughly two orders of headroom over the observed residual.
func TestEPSGTransform_Roundtrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		from string
		to   string
		x, y float64
		tol  float64
	}{
		{"WGS84↔UTM32", "EPSG:4326", "EPSG:25832", 9.7320, 52.3759, 1e-9},
		{"WGS84↔UTM33", "EPSG:4326", "EPSG:25833", 13.4050, 52.5200, 1e-9},
		// The Gauss-Krüger pairs cannot reach 1e-9°: wgs84's helmert.Inverse is
		// the linearised approximation rather than the exact inverse of
		// helmert.Forward, so the DHDN datum step alone loses ~1.2e-7° (≈1.3 cm)
		// per round trip. That is the datum, not the projection — the projection
		// on the same Bessel ellipsoid is pinned to a micrometre by the
		// reference-vector tests.
		{"WGS84↔GK3", "EPSG:4326", "EPSG:31467", 9.7320, 52.3759, 1e-6},
		{"WGS84↔GK4", "EPSG:4326", "EPSG:31468", 11.5820, 48.1351, 1e-6},
		// Cross-zone round trip. This case carried a 20 m tolerance and a comment
		// calling that "expected" distortion of the projection; it was neither.
		// A conformal projection and its inverse are exact inverses of each
		// other wherever the series converges, 6° off the central meridian
		// included. The 20 m was the wgs84 inverse's integer-division defect.
		{"UTM32↔UTM33", "EPSG:25832", "EPSG:25833", 700000, 5800000, 1e-6},
		{"WGS84↔WebMercator", "EPSG:4326", "EPSG:3857", 9.7320, 52.3759, 1e-9},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fromCRS, _ := ParseCRS(tc.from)
			toCRS, _ := ParseCRS(tc.to)

			forward, err := BuildTransformPipeline(toCRS, fromCRS)
			if err != nil {
				t.Fatalf("build forward: %v", err)
			}

			inverse, err := BuildTransformPipeline(fromCRS, toCRS)
			if err != nil {
				t.Fatalf("build inverse: %v", err)
			}

			original := Point2D{X: tc.x, Y: tc.y}

			projected, err := forward.ApplyPoint(original)
			if err != nil {
				t.Fatalf("forward: %v", err)
			}

			recovered, err := inverse.ApplyPoint(projected)
			if err != nil {
				t.Fatalf("inverse: %v", err)
			}

			if math.Abs(recovered.X-original.X) > tc.tol || math.Abs(recovered.Y-original.Y) > tc.tol {
				t.Fatalf("roundtrip error: original=(%.6f, %.6f), recovered=(%.6f, %.6f)",
					original.X, original.Y, recovered.X, recovered.Y)
			}
		})
	}
}

func TestEPSGTransform_AllSupportedCodes(t *testing.T) {
	t.Parallel()

	// Zone-appropriate WGS84 test points (lon, lat) for each EPSG code.
	testPoints := map[int]Point2D{
		4258:  {X: 10.0, Y: 51.0}, // ETRS89 geographic — central Europe
		25831: {X: 3.0, Y: 51.0},  // UTM 31N — zone 0-6°E
		25832: {X: 10.0, Y: 51.0}, // UTM 32N — zone 6-12°E
		25833: {X: 13.4, Y: 52.5}, // UTM 33N — zone 12-18°E
		25834: {X: 21.0, Y: 52.0}, // UTM 34N — zone 18-24°E
		31466: {X: 7.0, Y: 51.0},  // GK zone 2 — ~6-9°E
		31467: {X: 10.0, Y: 51.0}, // GK zone 3 — ~9-12°E (corrected: 7.5-10.5°E)
		31468: {X: 12.5, Y: 48.0}, // GK zone 4 — ~10.5-13.5°E
		31469: {X: 15.5, Y: 51.0}, // GK zone 5 — ~13.5-16.5°E
		32632: {X: 10.0, Y: 51.0}, // WGS84 UTM 32N
		32633: {X: 13.4, Y: 52.5}, // WGS84 UTM 33N
		3857:  {X: 10.0, Y: 51.0}, // Web Mercator — global
	}

	wgs84CRS, _ := ParseCRS("EPSG:4326")

	for code := range supportedEPSGCodes {
		if code == 4326 {
			continue
		}

		t.Run(fmt.Sprintf("EPSG_%d", code), func(t *testing.T) {
			t.Parallel()

			original, ok := testPoints[code]
			if !ok {
				t.Fatalf("no test point defined for EPSG:%d", code)
			}

			crs, err := ParseCRS(fmt.Sprintf("EPSG:%d", code))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			forward, err := BuildTransformPipeline(crs, wgs84CRS)
			if err != nil {
				t.Fatalf("build forward: %v", err)
			}

			inverse, err := BuildTransformPipeline(wgs84CRS, crs)
			if err != nil {
				t.Fatalf("build inverse: %v", err)
			}

			projected, err := forward.ApplyPoint(original)
			if err != nil {
				t.Fatalf("forward: %v", err)
			}

			if !projected.IsFinite() {
				t.Fatal("forward produced non-finite result")
			}

			recovered, err := inverse.ApplyPoint(projected)
			if err != nil {
				t.Fatalf("inverse: %v", err)
			}

			// 1e-6° is ≈0.11 m, and it is the floor the DHDN Gauss-Krüger codes
			// impose: wgs84's helmert.Inverse is a linearised approximation
			// rather than the exact inverse of helmert.Forward, so their round
			// trip loses ~1.2e-7° in the datum step alone. Every other code here
			// returns to within 4e-14°. See TestEPSGTransform_Roundtrip.
			tol := 1e-6
			if math.Abs(recovered.X-original.X) > tol || math.Abs(recovered.Y-original.Y) > tol {
				t.Fatalf("roundtrip EPSG:%d error: original=(%.6f, %.6f), recovered=(%.6f, %.6f)",
					code, original.X, original.Y, recovered.X, recovered.Y)
			}
		})
	}
}

func TestEPSGTransform_NonFiniteInput(t *testing.T) {
	t.Parallel()

	from, _ := ParseCRS("EPSG:4326")
	to, _ := ParseCRS("EPSG:25832")

	pipeline, err := BuildTransformPipeline(to, from)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	_, nanErr := pipeline.ApplyPoint(Point2D{X: math.NaN(), Y: 52.0})
	if nanErr == nil {
		t.Fatal("expected error for NaN input")
	}

	_, infErr := pipeline.ApplyPoint(Point2D{X: 9.0, Y: math.Inf(1)})
	if infErr == nil {
		t.Fatal("expected error for Inf input")
	}
}

func TestEPSGTransform_UnsupportedCode(t *testing.T) {
	t.Parallel()

	from, _ := ParseCRS("EPSG:4326")
	to, _ := ParseCRS("EPSG:99999")

	_, err := BuildTransformPipeline(to, from)
	if err == nil {
		t.Fatal("expected error for unsupported EPSG code")
	}
}

func TestEPSGTransform_NonEPSGFormat(t *testing.T) {
	t.Parallel()

	from := CRS{ID: "WKT:SOMETHING", Kind: CRSKindUnknown}
	to, _ := ParseCRS("EPSG:25832")

	_, err := BuildTransformPipeline(to, from)
	if err == nil {
		t.Fatal("expected error for non-EPSG CRS")
	}
}

func TestEPSGCode(t *testing.T) {
	t.Parallel()

	crs, _ := ParseCRS("EPSG:25832")
	if crs.EPSGCode() != 25832 {
		t.Fatalf("expected 25832, got %d", crs.EPSGCode())
	}

	wkt := CRS{ID: "WKT:SOMETHING"}
	if wkt.EPSGCode() != 0 {
		t.Fatalf("expected 0 for WKT, got %d", wkt.EPSGCode())
	}

	empty := CRS{}
	if empty.EPSGCode() != 0 {
		t.Fatalf("expected 0 for empty, got %d", empty.EPSGCode())
	}
}

func TestIsSupportedEPSG(t *testing.T) {
	t.Parallel()

	if !IsSupportedEPSG(4326) {
		t.Fatal("4326 should be supported")
	}

	if !IsSupportedEPSG(25832) {
		t.Fatal("25832 should be supported")
	}

	if !IsSupportedEPSG(31467) {
		t.Fatal("31467 (GK3) should be supported")
	}

	if IsSupportedEPSG(99999) {
		t.Fatal("99999 should not be supported")
	}
}

func TestSupportedEPSGCodes(t *testing.T) {
	t.Parallel()

	codes := SupportedEPSGCodes()
	if len(codes) != len(supportedEPSGCodes) {
		t.Fatalf("expected %d codes, got %d", len(supportedEPSGCodes), len(codes))
	}
}

// TestETRS89UTM32RoundTripResidualBoundsTheSavePath bounds what a projected
// coordinate loses on the EPSG:25832 → 4326 → 25832 trip, which is the trip
// every save from the map workspace puts every vertex through:
// frontend/src/model/use-project-sync.ts pins MODEL_CRS = "EPSG:4326", and
// internal/api/httpv1/model.go reprojects out on GET /api/v1/model?crs= and
// back in on POST /api/v1/model.
//
// This test used to pin the residuals of a defect, as ±5% bands around
// 0.000319 / 0.492396 / 6.768904 / 8.530139 m. github.com/wroge/wgs84 v1.1.7
// computed the radius of curvature in the meridian as
// math.Pow(1-e²sin²φ₁, 3/2); `3/2` is an untyped integer constant expression in
// Go, so the exponent was 1 and R1 came out 0.21% too large. R1 divides the
// leading D²/2 term of the footpoint-latitude correction, which is why the
// error was zero on the central meridian and grew quadratically with easting
// offset. The projection is now geo.TransverseMercator, a Krüger series, and
// the residuals are 1e-10 to 4e-9 m.
//
// It is a ceiling now rather than a band. A band around a measured value
// invites the next reader to re-pin instead of asking why the number moved, and
// there is nothing left here worth tracking to five digits — only a floor worth
// defending.
//
// What this test cannot do is say which direction is wrong when it fails. A
// round trip proves that forward and inverse disagree, never which of them is
// at fault; a clean one proves neither is right. That is what
// TestTransverseMercatorForwardMatchesPROJ and
// TestTransverseMercatorInverseMatchesPROJ are for — start there.
func TestETRS89UTM32RoundTripResidualBoundsTheSavePath(t *testing.T) {
	t.Parallel()

	utm32, _ := ParseCRS("EPSG:25832")
	wgs84, _ := ParseCRS("EPSG:4326")

	toWGS84, err := BuildTransformPipeline(wgs84, utm32)
	if err != nil {
		t.Fatalf("build 25832->4326: %v", err)
	}

	toUTM32, err := BuildTransformPipeline(utm32, wgs84)
	if err != nil {
		t.Fatalf("build 4326->25832: %v", err)
	}

	// maxResidualM is a micrometre: three orders above the measured residuals,
	// which leaves room for FMA contraction differences between architectures,
	// and seven orders below the defect it replaced.
	const maxResidualM = 1e-6

	// The old error grew quadratically with distance from the central meridian
	// and sat almost entirely in the northing, so the cases still sample the
	// zone from its centre out to both edges.
	cases := []struct {
		name  string
		point Point2D
	}{
		{"central meridian", Point2D{X: 500000, Y: 5650000}},
		{"48 km east of the central meridian", Point2D{X: 548000, Y: 5803000}},
		{"western zone edge", Point2D{X: 300000, Y: 5400000}},
		{"eastern zone edge", Point2D{X: 700000, Y: 5800000}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			geographic, err := toWGS84.ApplyPoint(tc.point)
			if err != nil {
				t.Fatalf("forward: %v", err)
			}

			recovered, err := toUTM32.ApplyPoint(geographic)
			if err != nil {
				t.Fatalf("inverse: %v", err)
			}

			residual := math.Hypot(recovered.X-tc.point.X, recovered.Y-tc.point.Y)
			if residual > maxResidualM {
				t.Fatalf("round-trip residual = %.6g m, ceiling %g m.\n"+
					"A save path that loses this much integrates it — the error is bias, not noise,\n"+
					"so N saves move the geometry N times as far. Read the one-way reference-vector\n"+
					"tests first: they say which direction is wrong, and this one cannot.",
					residual, maxResidualM)
			}
		})
	}
}
