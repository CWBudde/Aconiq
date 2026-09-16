package geo_test

import (
	"math"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

func TestUTMZoneForLongitude(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		lon  float64
		want int
	}{
		{"zone 32 covers most of Germany", 10.0, 32},
		{"zone 32 starts at 6E", 6.0, 32},
		{"just west of 6E is zone 31", 5.999, 31},
		{"zone 33 starts at 12E", 12.0, 33},
		{"Vienna is zone 33", 16.37, 33},
		{"the antimeridian does not run off the end", 180.0, 60},
		{"the far west is zone 1", -180.0, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := geo.UTMZoneForLongitude(tc.lon)
			if got != tc.want {
				t.Fatalf("UTMZoneForLongitude(%v) = %d, want %d", tc.lon, got, tc.want)
			}
		})
	}
}

func TestComputeCRSForGeographicPicksTheZone(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		lon  float64
		lat  float64
		want string
	}{
		{"Hamburg", 10.0, 53.55, "EPSG:25832"},
		{"Cologne", 6.96, 50.94, "EPSG:25832"},
		{"Vienna", 16.37, 48.21, "EPSG:25833"},
		{"the western edge of the supported range", 0.5, 50.0, "EPSG:25831"},
		{"the eastern edge of the supported range", 23.5, 54.0, "EPSG:25834"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := geo.ComputeCRSForGeographic(tc.lon, tc.lat)
			if err != nil {
				t.Fatalf("ComputeCRSForGeographic(%v, %v): %v", tc.lon, tc.lat, err)
			}

			if got.ID != tc.want {
				t.Fatalf("ComputeCRSForGeographic(%v, %v) = %s, want %s", tc.lon, tc.lat, got.ID, tc.want)
			}

			if got.Kind != geo.CRSKindProjected {
				t.Fatalf("compute CRS %s is %s; a compute CRS that is not projected defeats its own purpose", got.ID, got.Kind)
			}
		})
	}
}

// A site the CRS table cannot carry is refused rather than projected into the
// nearest zone it does carry. Silently computing hundreds of kilometres from
// the site is the outcome this whole function exists to prevent, so a wrong
// answer here is worse than no answer.
func TestComputeCRSForGeographicRefusesWhatItCannotProject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		lon      float64
		lat      float64
		wantHint string
	}{
		{"west of zone 31", -3.7, 40.4, "zone 30"},
		{"east of zone 34", 30.5, 50.4, "zone 36"},
		{"southern hemisphere", 18.4, -33.9, "southern hemisphere"},
		{"outside the lon/lat range", 500000, 5933000, "outside the lon/lat range"},
		{"not a number", math.NaN(), 50.0, "finite"},
		{"infinite", 10.0, math.Inf(1), "finite"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := geo.ComputeCRSForGeographic(tc.lon, tc.lat)
			if err == nil {
				t.Fatalf("ComputeCRSForGeographic(%v, %v) = %s, want an error", tc.lon, tc.lat, got.ID)
			}

			if !strings.Contains(err.Error(), tc.wantHint) {
				t.Fatalf("error %q does not name %q, so it does not tell the user what to change", err.Error(), tc.wantHint)
			}
		})
	}
}

// The zone boundaries are where an off-by-one puts a model in the wrong CRS,
// and a 500 km false easting makes that a quiet error rather than a loud one.
func TestComputeCRSForGeographicIsStableAcrossAZoneBoundary(t *testing.T) {
	t.Parallel()

	const boundary = 12.0

	west, err := geo.ComputeCRSForGeographic(boundary-1e-9, 50.0)
	if err != nil {
		t.Fatalf("just west of %v: %v", boundary, err)
	}

	east, err := geo.ComputeCRSForGeographic(boundary+1e-9, 50.0)
	if err != nil {
		t.Fatalf("just east of %v: %v", boundary, err)
	}

	if west.ID != "EPSG:25832" || east.ID != "EPSG:25833" {
		t.Fatalf("across the %v boundary got %s and %s, want EPSG:25832 and EPSG:25833", boundary, west.ID, east.ID)
	}
}
