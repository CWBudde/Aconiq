package road

import (
	"errors"
	"fmt"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// Building represents a solid structure in plan view that acts as both a
// noise barrier (shielding the direct path) and a sound reflector (adding
// reflected propagation paths via the image-source method).
//
// A Reflector models a single facade line; a Building models the full footprint
// polygon. Multiple buildings can compose street corridors, perpendicular
// structures, or courtyards ("Hinterhof").
type Building struct {
	ID        string        `json:"id"`
	Footprint []geo.Point2D `json:"footprint"` // polygon vertices (auto-closed)
	HeightM   float64       `json:"height_m"`  // top of building above ground [m]

	// ReflectionLossDB is the energy loss per reflection [dB].
	// Defaults to 1.0 dB when zero or unset.
	ReflectionLossDB float64 `json:"reflection_loss_db,omitempty"`
}

// Validate checks a building definition.
func (b Building) Validate() error {
	if b.ID == "" {
		return errors.New("building id is required")
	}

	if len(b.Footprint) < 3 {
		return fmt.Errorf("building %q footprint must contain at least 3 vertices", b.ID)
	}

	for i, pt := range b.Footprint {
		if !pt.IsFinite() {
			return fmt.Errorf("building %q footprint point[%d] is not finite", b.ID, i)
		}
	}

	if !isFinite(b.HeightM) || b.HeightM <= 0 {
		return fmt.Errorf("building %q height_m must be finite and > 0", b.ID)
	}

	if !isFinite(b.ReflectionLossDB) || b.ReflectionLossDB < 0 {
		return fmt.Errorf("building %q reflection_loss_db must be finite and >= 0", b.ID)
	}

	return nil
}

// asBarrier converts the building footprint into a Barrier for shielding
// calculations. The polygon is auto-closed so all wall edges are checked.
func (b Building) asBarrier() Barrier {
	return Barrier{
		ID:       b.ID,
		Geometry: closedPolygon(b.Footprint),
		HeightM:  b.HeightM,
	}
}

// asReflector converts the building footprint into a Reflector for
// image-source reflection calculations.
func (b Building) asReflector() Reflector {
	return Reflector{
		ID:               b.ID,
		Geometry:         closedPolygon(b.Footprint),
		HeightM:          b.HeightM,
		ReflectionLossDB: b.ReflectionLossDB,
	}
}

// closedPolygon returns pts with the first vertex appended at the end when
// the polygon is not already closed, ensuring all wall edges are included.
func closedPolygon(pts []geo.Point2D) []geo.Point2D {
	if len(pts) < 2 {
		return pts
	}

	first, last := pts[0], pts[len(pts)-1]
	dx := first.X - last.X
	dy := first.Y - last.Y

	if math.Sqrt(dx*dx+dy*dy) < 1e-10 {
		return pts // already closed
	}

	closed := make([]geo.Point2D, len(pts)+1)
	copy(closed, pts)
	closed[len(pts)] = first

	return closed
}
