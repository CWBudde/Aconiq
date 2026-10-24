package facadeset_test

import (
	"math"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/facadeset"
)

// A photo of a 10 m × 8 m facade, its corners clicked bottom-left first.
func photoTransform() facadeset.PhotoTransform {
	return facadeset.PhotoTransform{
		ImagePoints:  [][]float64{{100, 900}, {865.4, 846.2}, {885.1, 114.9}, {123.5, 139.4}},
		FacadePoints: [][]float64{{0, 0}, {10, 0}, {10, 8}, {0, 8}},
	}
}

func apply(m [9]float64, x, y float64) (float64, float64) {
	w := m[6]*x + m[7]*y + m[8]

	return (m[0]*x + m[1]*y + m[2]) / w, (m[3]*x + m[4]*y + m[5]) / w
}

func TestRectifyMapsTheCornersBothWays(t *testing.T) {
	t.Parallel()

	tr := photoTransform()

	got, err := facadeset.Rectify(tr)
	if err != nil {
		t.Fatalf("Rectify: %v", err)
	}

	for i := range tr.ImagePoints {
		img, fac := tr.ImagePoints[i], tr.FacadePoints[i]

		u, z := apply(got.Matrix, img[0], img[1])
		if math.Abs(u-fac[0]) > 1e-9 || math.Abs(z-fac[1]) > 1e-9 {
			t.Errorf("corner %d: pixel %v maps to (%g, %g), want %v", i, img, u, z, fac)
		}

		x, y := apply(got.Inverse, fac[0], fac[1])
		if math.Abs(x-img[0]) > 1e-6 || math.Abs(y-img[1]) > 1e-6 {
			t.Errorf("corner %d: facade point %v maps back to (%g, %g), want %v", i, fac, x, y, img)
		}
	}

	again, err := facadeset.Rectify(photoTransform())
	if err != nil || again != got {
		t.Errorf("a second rectification of the same evidence gave %v (%v), the first %v", again, err, got)
	}
}

func TestRectifyRefuses(t *testing.T) {
	t.Parallel()

	tr := photoTransform()
	tr.ImagePoints = tr.ImagePoints[:3]

	_, err := facadeset.Rectify(tr)
	if err == nil || !strings.Contains(err.Error(), "image_points must hold 4 points") {
		t.Errorf("err = %v, want the missing corner named", err)
	}
}
