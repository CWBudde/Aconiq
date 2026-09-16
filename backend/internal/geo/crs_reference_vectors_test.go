package geo

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/wroge/wgs84"
)

// The fixture in testdata/proj-reference-vectors.json was produced once by
// PROJ 9.8.1; testdata/README.md records how. PROJ is not a build or CI
// dependency — the vectors are the fixture, not the tool.
//
// Every assertion below is ONE-WAY. A round trip only proves that two errors
// cancel, which is exactly how the defect these vectors were written for
// survived: github.com/wroge/wgs84 v1.1.7's inverse transverse Mercator was
// wrong by up to 8.5 m in UTM zone 32, and the existing degrees-first round
// trip ran inverse∘forward, where the error largely cancels. No tightening of a
// round-trip tolerance would have caught it.
const referenceVectorPath = "testdata/proj-reference-vectors.json"

type projReferenceVectors struct {
	ProjVersion     string               `json:"proj_version"`
	Generated       string               `json:"generated"`
	ProjectionCases []projProjectionCase `json:"projection_cases"`
	CRSCases        []projCoordinateCase `json:"crs_cases"`
}

// projProjectionCase pins the projection alone: geodetic coordinates on one
// ellipsoid against plane coordinates on the same ellipsoid, with no datum
// step on either side.
type projProjectionCase struct {
	Name       string  `json:"name"`
	EPSG       int     `json:"epsg"`
	ProjString string  `json:"proj_string"`
	Lon        float64 `json:"lon"`
	Lat        float64 `json:"lat"`
	Easting    float64 `json:"easting"`
	Northing   float64 `json:"northing"`
}

// projCoordinateCase pins the whole EPSG:4326 <-> EPSG:<code> pipeline that
// BuildTransformPipeline builds, datum shift included.
type projCoordinateCase struct {
	Name string  `json:"name"`
	EPSG int     `json:"epsg"`
	Lon  float64 `json:"lon"`
	Lat  float64 `json:"lat"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

func loadReferenceVectors(t *testing.T) projReferenceVectors {
	t.Helper()

	raw, err := os.ReadFile(referenceVectorPath)
	if err != nil {
		t.Fatalf("read reference vectors: %v", err)
	}

	var vectors projReferenceVectors
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatalf("decode reference vectors: %v", err)
	}

	if len(vectors.ProjectionCases) == 0 || len(vectors.CRSCases) == 0 {
		t.Fatal("reference vector fixture is empty")
	}

	return vectors
}

// spheroidsForEllipsoid maps the PROJ +ellps name in a fixture row to the
// wgs84 datum this package projects on. Datum satisfies wgs84.Spheroid.
func spheroidForEllipsoid(t *testing.T, name string) wgs84.Spheroid {
	t.Helper()

	switch name {
	case "GRS80":
		return wgs84.ETRS89()
	case "WGS84":
		return wgs84.WGS84()
	case "bessel":
		return wgs84.DHDN2001()
	default:
		t.Fatalf("no spheroid for +ellps=%s", name)
		return nil
	}
}

// parseProjString reads the projection back out of the PROJ definition the
// fixture row was generated with, so the test asserts against the parameters
// PROJ actually used rather than against a second copy of them.
func parseProjString(t *testing.T, def string) (TransverseMercator, string) {
	t.Helper()

	projection := TransverseMercator{}
	ellipsoid := ""

	for token := range strings.FieldsSeq(def) {
		key, value, ok := strings.Cut(strings.TrimPrefix(token, "+"), "=")
		if !ok {
			continue
		}

		if key == "ellps" {
			ellipsoid = value
			continue
		}

		number, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}

		switch key {
		case "lon_0":
			projection.LonOrigin = number
		case "lat_0":
			projection.LatOrigin = number
		case "k", "k_0":
			projection.Scale = number
		case "x_0":
			projection.FalseEasting = number
		case "y_0":
			projection.FalseNorthing = number
		}
	}

	if !strings.Contains(def, "+proj=tmerc") || ellipsoid == "" || projection.Scale == 0 {
		t.Fatalf("not a usable transverse Mercator definition: %q", def)
	}

	return projection, ellipsoid
}

// projectionToleranceM is what the Krüger series is allowed to differ from PROJ
// by in the plane. Karney's order-6 series and PROJ's extended transverse
// Mercator agree to nanometres inside a zone; the bound is set at a micrometre
// so it cannot be met by accident and cannot fail on FMA contraction.
const projectionToleranceM = 1e-6

// projectionToleranceDeg is the same bound on the geographic side. 1e-11 deg is
// roughly 1.1 micrometres of latitude.
const projectionToleranceDeg = 1e-11

func TestTransverseMercatorForwardMatchesPROJ(t *testing.T) {
	t.Parallel()

	vectors := loadReferenceVectors(t)

	for _, tc := range vectors.ProjectionCases {
		t.Run(fmt.Sprintf("EPSG%d/%s", tc.EPSG, tc.Name), func(t *testing.T) {
			t.Parallel()

			projection, ellipsoid := parseProjString(t, tc.ProjString)
			spheroid := spheroidForEllipsoid(t, ellipsoid)

			east, north := projection.FromLonLat(tc.Lon, tc.Lat, spheroid)

			if math.Abs(east-tc.Easting) > projectionToleranceM ||
				math.Abs(north-tc.Northing) > projectionToleranceM {
				t.Fatalf("forward (%.6f, %.6f) = (%.9f, %.9f), PROJ 9.8.1 says (%.9f, %.9f); "+
					"deviation (%.3g, %.3g) m exceeds %g m",
					tc.Lon, tc.Lat, east, north, tc.Easting, tc.Northing,
					east-tc.Easting, north-tc.Northing, projectionToleranceM)
			}
		})
	}
}

func TestTransverseMercatorInverseMatchesPROJ(t *testing.T) {
	t.Parallel()

	vectors := loadReferenceVectors(t)

	for _, tc := range vectors.ProjectionCases {
		t.Run(fmt.Sprintf("EPSG%d/%s", tc.EPSG, tc.Name), func(t *testing.T) {
			t.Parallel()

			projection, ellipsoid := parseProjString(t, tc.ProjString)
			spheroid := spheroidForEllipsoid(t, ellipsoid)

			lon, lat := projection.ToLonLat(tc.Easting, tc.Northing, spheroid)

			if math.Abs(lon-tc.Lon) > projectionToleranceDeg ||
				math.Abs(lat-tc.Lat) > projectionToleranceDeg {
				t.Fatalf("inverse (%.9f, %.9f) = (%.12f, %.12f), PROJ 9.8.1 says (%.12f, %.12f); "+
					"deviation (%.3g, %.3g) deg exceeds %g deg",
					tc.Easting, tc.Northing, lon, lat, tc.Lon, tc.Lat,
					lon-tc.Lon, lat-tc.Lat, projectionToleranceDeg)
			}
		})
	}
}

// TestTransverseMercatorSpheroidsMatchPROJ guards the assumption the two tests
// above rest on: that the wgs84 datums carry the same ellipsoid constants the
// PROJ +ellps names do. If one ever diverges, that has to read as its own
// failure rather than as a projection error of unexplained size.
func TestTransverseMercatorSpheroidsMatchPROJ(t *testing.T) {
	t.Parallel()

	cases := []struct {
		ellipsoid         string
		semiMajorAxis     float64
		inverseFlattening float64
	}{
		{"GRS80", 6378137, 298.257222101},
		{"WGS84", 6378137, 298.257223563},
		{"bessel", 6377397.155, 299.1528128},
	}

	for _, tc := range cases {
		t.Run(tc.ellipsoid, func(t *testing.T) {
			t.Parallel()

			spheroid := spheroidForEllipsoid(t, tc.ellipsoid)
			if spheroid.A() != tc.semiMajorAxis || spheroid.Fi() != tc.inverseFlattening {
				t.Fatalf("+ellps=%s is a=%.4f 1/f=%.9f in PROJ but a=%.4f 1/f=%.9f here",
					tc.ellipsoid, tc.semiMajorAxis, tc.inverseFlattening, spheroid.A(), spheroid.Fi())
			}
		})
	}
}

// The four constants below bound the whole pipeline — projection and datum
// together — against PROJ. They are looser than the projection bounds because
// the datum halves genuinely differ, and that difference is the library's, not
// this package's: wgs84 carries one Helmert set for DHDN where PROJ 9.8.1
// prefers the BETA2007 grid, and it models ETRS89 -> WGS84 as an ellipsoid
// change where PROJ treats it as a null shift.
//
// Every number is measured, with roughly a factor of two of headroom; the
// per-code table is in testdata/README.md. Nothing here excuses the projection:
// the DHDN projection itself is pinned to a micrometre by the projection cases
// above, on the same Bessel ellipsoid.
const (
	// crsToleranceM covers EPSG:25831-25834, 32632, 32633 and 3857, where the
	// worst measured disagreement is 0.1 mm of northing.
	crsToleranceM = 1e-3
	// crsToleranceDeg is the geographic-side twin of crsToleranceM; the worst
	// measured disagreement is 9.2e-10 deg.
	crsToleranceDeg = 1e-8
	// dhdnToleranceM covers EPSG:31466-31469. Worst measured: 1.84 m at Aachen.
	dhdnToleranceM = 4.0
	// dhdnToleranceDeg is its geographic twin. Worst measured: 2.6e-5 deg.
	dhdnToleranceDeg = 6e-5
)

// isDHDN reports whether an EPSG code is one of the DHDN Gauss-Krüger zones,
// whose datum shift PROJ and wgs84 disagree about by metres.
func isDHDN(code int) bool {
	return code >= 31466 && code <= 31469
}

func crsToleranceFor(code int, geographicTarget bool) float64 {
	switch {
	case isDHDN(code) && geographicTarget:
		return dhdnToleranceDeg
	case isDHDN(code):
		return dhdnToleranceM
	case geographicTarget:
		return crsToleranceDeg
	default:
		return crsToleranceM
	}
}

func TestEPSGTransformForwardMatchesPROJ(t *testing.T) {
	t.Parallel()

	vectors := loadReferenceVectors(t)

	source, err := ParseCRS("EPSG:4326")
	if err != nil {
		t.Fatalf("parse EPSG:4326: %v", err)
	}

	for _, tc := range vectors.CRSCases {
		t.Run(fmt.Sprintf("EPSG%d/%s", tc.EPSG, tc.Name), func(t *testing.T) {
			t.Parallel()

			target, err := ParseCRS(fmt.Sprintf("EPSG:%d", tc.EPSG))
			if err != nil {
				t.Fatalf("parse target: %v", err)
			}

			pipeline, err := BuildTransformPipeline(target, source)
			if err != nil {
				t.Fatalf("build pipeline: %v", err)
			}

			got, err := pipeline.ApplyPoint(Point2D{X: tc.Lon, Y: tc.Lat})
			if err != nil {
				t.Fatalf("apply: %v", err)
			}

			tolerance := crsToleranceFor(tc.EPSG, target.Kind == CRSKindGeographic)
			if math.Abs(got.X-tc.X) > tolerance || math.Abs(got.Y-tc.Y) > tolerance {
				t.Fatalf("4326 -> %s of (%.6f, %.6f) = (%.6f, %.6f), PROJ 9.8.1 says (%.6f, %.6f); "+
					"deviation (%.3g, %.3g) exceeds %g",
					target.ID, tc.Lon, tc.Lat, got.X, got.Y, tc.X, tc.Y,
					got.X-tc.X, got.Y-tc.Y, tolerance)
			}
		})
	}
}

func TestEPSGTransformInverseMatchesPROJ(t *testing.T) {
	t.Parallel()

	vectors := loadReferenceVectors(t)

	target, err := ParseCRS("EPSG:4326")
	if err != nil {
		t.Fatalf("parse EPSG:4326: %v", err)
	}

	for _, tc := range vectors.CRSCases {
		t.Run(fmt.Sprintf("EPSG%d/%s", tc.EPSG, tc.Name), func(t *testing.T) {
			t.Parallel()

			source, err := ParseCRS(fmt.Sprintf("EPSG:%d", tc.EPSG))
			if err != nil {
				t.Fatalf("parse source: %v", err)
			}

			pipeline, err := BuildTransformPipeline(target, source)
			if err != nil {
				t.Fatalf("build pipeline: %v", err)
			}

			got, err := pipeline.ApplyPoint(Point2D{X: tc.X, Y: tc.Y})
			if err != nil {
				t.Fatalf("apply: %v", err)
			}

			tolerance := crsToleranceFor(tc.EPSG, true)
			if math.Abs(got.X-tc.Lon) > tolerance || math.Abs(got.Y-tc.Lat) > tolerance {
				t.Fatalf("%s -> 4326 of (%.6f, %.6f) = (%.9f, %.9f), PROJ 9.8.1 says (%.9f, %.9f); "+
					"deviation (%.3g, %.3g) deg exceeds %g deg",
					source.ID, tc.X, tc.Y, got.X, got.Y, tc.Lon, tc.Lat,
					got.X-tc.Lon, got.Y-tc.Lat, tolerance)
			}
		})
	}
}
