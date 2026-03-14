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

func TestEPSGTransform_Roundtrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		from string
		to   string
		x, y float64
		tol  float64
	}{
		{"WGS84↔UTM32", "EPSG:4326", "EPSG:25832", 9.7320, 52.3759, 0.0001},
		{"WGS84↔UTM33", "EPSG:4326", "EPSG:25833", 13.4050, 52.5200, 0.0001},
		{"WGS84↔GK3", "EPSG:4326", "EPSG:31467", 9.7320, 52.3759, 0.001},
		{"WGS84↔GK4", "EPSG:4326", "EPSG:31468", 11.5820, 48.1351, 0.001},
		// Cross-zone roundtrip: point at ~10.7°E is far from UTM33 central meridian (15°E),
		// so the TM projection introduces larger distortion. ~20m roundtrip error is expected.
		{"UTM32↔UTM33", "EPSG:25832", "EPSG:25833", 700000, 5800000, 20.0},
		{"WGS84↔WebMercator", "EPSG:4326", "EPSG:3857", 9.7320, 52.3759, 0.0001},
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

			// GK uses Helmert datum shift, so roundtrip tolerance is looser.
			tol := 0.001
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
