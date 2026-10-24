package geo_test

import (
	"math"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// facadeCorners is a 10 m × 8 m facade's frame corners, in the order a person
// clicks them: bottom-left, bottom-right, top-right, top-left, seen from
// outside.
var facadeCorners = [4]geo.Point2D{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 8}, {X: 0, Y: 8}}

// photographed is where a camera standing off to one side and below the
// facade sees a point (u, z) of it: a projective map with image y running
// down, as a photo's pixels do.
func photographed(p geo.Point2D) geo.Point2D {
	w := 1 + 0.004*p.X + 0.0005*p.Y

	return geo.Point2D{X: (80*p.X + 3*p.Y + 100) / w, Y: (-2*p.X - 95*p.Y + 900) / w}
}

func photoCorners() [4]geo.Point2D {
	var image [4]geo.Point2D
	for i, c := range facadeCorners {
		image[i] = photographed(c)
	}

	return image
}

func near(a, b geo.Point2D, tol float64) bool {
	return math.Abs(a.X-b.X) <= tol && math.Abs(a.Y-b.Y) <= tol
}

func TestFacadeHomographyMapsThePairsAndThePlaneBetweenThem(t *testing.T) {
	t.Parallel()

	image := photoCorners()

	h, err := geo.FacadeHomography(image, facadeCorners)
	if err != nil {
		t.Fatalf("FacadeHomography: %v", err)
	}

	for i := range image {
		got, ok := h.Apply(image[i])
		if !ok || !near(got, facadeCorners[i], 1e-9) {
			t.Errorf("corner %d: %v maps to %v, want %v", i, image[i], got, facadeCorners[i])
		}
	}

	// Four pairs fix a homography, so every other point of the facade lands
	// where the camera saw it too: a window centre, an eave corner.
	inv, err := h.Inverse()
	if err != nil {
		t.Fatalf("Inverse: %v", err)
	}

	for _, p := range []geo.Point2D{{X: 3.1, Y: 4.7}, {X: 9.4, Y: 0.6}, {X: 5, Y: 8}} {
		got, ok := h.Apply(photographed(p))
		if !ok || !near(got, p, 1e-9) {
			t.Errorf("facade point %v: the photo's pixel maps back to %v", p, got)
		}

		px, ok := inv.Apply(p)
		if !ok || !near(px, photographed(p), 1e-6) {
			t.Errorf("facade point %v: the inverse puts it at pixel %v, the camera at %v", p, px, photographed(p))
		}
	}
}

// The same evidence always gives the same window: solving the same four
// pairs again gives the same matrix, bit for bit.
func TestFacadeHomographyIsBitIdenticalOnEverySolve(t *testing.T) {
	t.Parallel()

	first, err := geo.FacadeHomography(photoCorners(), facadeCorners)
	if err != nil {
		t.Fatal(err)
	}

	for range 3 {
		again, err := geo.FacadeHomography(photoCorners(), facadeCorners)
		if err != nil {
			t.Fatal(err)
		}

		if again != first {
			t.Fatalf("second solve %v differs from the first %v", again, first)
		}
	}
}

func TestFacadeHomographyRefuses(t *testing.T) {
	t.Parallel()

	image := photoCorners()
	with := func(i int, p geo.Point2D) [4]geo.Point2D {
		out := image
		out[i] = p

		return out
	}

	var mirrored [4]geo.Point2D
	for i, p := range image {
		mirrored[i] = geo.Point2D{X: 2000 - p.X, Y: p.Y}
	}

	crossed := image
	crossed[2], crossed[3] = crossed[3], crossed[2]

	cases := map[string]struct {
		image, facade [4]geo.Point2D
		want          string
	}{
		"not finite": {with(1, geo.Point2D{X: math.NaN(), Y: 3}), facadeCorners, "not finite"},
		"repeated":   {with(1, image[0]), facadeCorners, "convex quadrilateral"},
		"collinear facade": {
			image, [4]geo.Point2D{{X: 0, Y: 0}, {X: 5, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 8}}, "convex quadrilateral",
		},
		"crossed order": {crossed, facadeCorners, "convex quadrilateral"},
		"mirrored":      {mirrored, facadeCorners, "mirrored"},
	}

	for name, tc := range cases {
		_, err := geo.FacadeHomography(tc.image, tc.facade)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to say %q", name, err, tc.want)
		}
	}
}

func TestPhotoTransformHomography(t *testing.T) {
	t.Parallel()

	image := photoCorners()

	pairs := func(ps [4]geo.Point2D) [][]float64 {
		out := make([][]float64, 0, len(ps))
		for _, p := range ps {
			out = append(out, []float64{p.X, p.Y})
		}

		return out
	}

	got, err := geo.PhotoTransform{ImagePoints: pairs(image), FacadePoints: pairs(facadeCorners)}.Homography()
	if err != nil {
		t.Fatalf("Homography: %v", err)
	}

	want, err := geo.FacadeHomography(image, facadeCorners)
	if err != nil || got != want {
		t.Errorf("transform solves to %v, the corners to %v (%v)", got, want, err)
	}

	cases := map[string]struct {
		t    geo.PhotoTransform
		want string
	}{
		"three image points": {
			geo.PhotoTransform{ImagePoints: pairs(image)[:3], FacadePoints: pairs(facadeCorners)},
			"image_points must hold 4 points",
		},
		"a triple": {
			geo.PhotoTransform{
				ImagePoints:  pairs(image),
				FacadePoints: [][]float64{{0, 0, 1}, {10, 0}, {10, 8}, {0, 8}},
			},
			"facade_points[0] must be [x, y]",
		},
	}

	for name, tc := range cases {
		_, err := tc.t.Homography()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to say %q", name, err, tc.want)
		}
	}
}
