package facadeset

import (
	"fmt"

	"github.com/aconiq/backend/internal/geo"
)

// PhotoTransform is the four point pairs a person sets by clicking a
// facade's corners in a photo of it (P18 D1): pixels, with y running down,
// and where those corners are in the facade's frame. It is the `transform` a
// window rectified from the photo carries in its evidence.
type PhotoTransform = geo.PhotoTransform

// Rectification is the homography a PhotoTransform solves to, as the
// elevation view draws a photo with it: Matrix takes a pixel to (u, z) on the
// facade and Inverse takes it back, each nine numbers, row-major, scaled so
// that w is positive over the photographed facade.
type Rectification struct {
	Matrix  [9]float64 `json:"matrix"`
	Inverse [9]float64 `json:"inverse"`
}

// Rectify solves a photo's point pairs through geo.FacadeHomography — the
// solve window validation checks a window's evidence with — so the browser
// draws a photo onto the facade with the same matrix in both modes and solves
// none of its own. The photo itself never reaches it: only the pairs do.
func Rectify(t PhotoTransform) (Rectification, error) {
	h, err := t.Homography()
	if err != nil {
		return Rectification{}, fmt.Errorf("the photo cannot be rectified: %w", err)
	}

	inv, err := h.Inverse()
	if err != nil {
		return Rectification{}, fmt.Errorf("the photo cannot be rectified: %w", err)
	}

	return Rectification{Matrix: h, Inverse: inv}, nil
}
