package schall03

import (
	"errors"
	"fmt"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// WallSurfaceType identifies the acoustic surface category of a reflecting
// wall per Table 18.
type WallSurfaceType int

const (
	// WallSurfaceHard is "Ebene und harte Wände" — D_ρ = 0 dB.
	WallSurfaceHard WallSurfaceType = iota
	// WallSurfaceBuilding is "Gebäudewände mit Fenstern und kleinen Anbauten" — D_ρ = −1 dB.
	WallSurfaceBuilding
	// WallSurfaceAbsorbing is "Absorbierende Schallschutzwände" — D_ρ = −4 dB.
	WallSurfaceAbsorbing
	// WallSurfaceHighlyAbsorbing is "Hoch absorbierende Schallschutzwände" — D_ρ = −8 dB.
	WallSurfaceHighlyAbsorbing
)

// table18 maps WallSurfaceType to D_ρ absorption loss in dB (Table 18).
var table18 = [4]float64{0, -1, -4, -8}

// Table18AbsorptionLoss returns D_ρ in dB for the given wall surface type.
func Table18AbsorptionLoss(surface WallSurfaceType) float64 {
	if surface < 0 || int(surface) >= len(table18) {
		return 0
	}

	return table18[surface]
}

// ReflectingWall describes one reflecting surface as a 2D line segment
// with a height and acoustic surface type.
type ReflectingWall struct {
	A       geo.Point2D     // first endpoint
	B       geo.Point2D     // second endpoint
	HeightM float64         // wall height above ground [m]
	Surface WallSurfaceType // acoustic surface category (Table 18)
}

// Validate checks the wall for geometric and physical validity.
func (w ReflectingWall) Validate() error {
	if !w.A.IsFinite() || !w.B.IsFinite() {
		return errors.New("ReflectingWall: endpoints must be finite")
	}

	if geo.Distance(w.A, w.B) < 1e-9 {
		return errors.New("ReflectingWall: wall has zero length")
	}

	if math.IsNaN(w.HeightM) || math.IsInf(w.HeightM, 0) || w.HeightM <= 0 {
		return errors.New("ReflectingWall: HeightM must be finite and > 0")
	}

	if w.Surface < WallSurfaceHard || w.Surface > WallSurfaceHighlyAbsorbing {
		return fmt.Errorf("ReflectingWall: unknown surface type %d", w.Surface)
	}

	return nil
}

// Length returns the wall segment length in metres.
func (w ReflectingWall) Length() float64 {
	return geo.Distance(w.A, w.B)
}

// MirrorSource computes the image (mirror) source position by reflecting
// the source point across the infinite line defined by the wall segment.
// Returns (imagePoint, true) on success.
// Returns (zero, false) if the wall is degenerate (zero length).
func MirrorSource(source geo.Point2D, wall ReflectingWall) (geo.Point2D, bool) {
	// Wall direction vector.
	dx := wall.B.X - wall.A.X
	dy := wall.B.Y - wall.A.Y
	lenSq := dx*dx + dy*dy

	if lenSq < 1e-18 {
		return geo.Point2D{}, false
	}

	// Project source onto the wall line: parameter t along A→B.
	t := ((source.X-wall.A.X)*dx + (source.Y-wall.A.Y)*dy) / lenSq

	// Foot of perpendicular from source onto the wall line.
	footX := wall.A.X + t*dx
	footY := wall.A.Y + t*dy

	// Mirror = source + 2*(foot - source) = 2*foot - source.
	return geo.Point2D{
		X: 2*footX - source.X,
		Y: 2*footY - source.Y,
	}, true
}

// FresnelCheck implements Gl. 27 to determine whether a reflecting surface is
// large enough for a valid specular reflection.  The check uses the lowest
// octave band frequency (63 Hz, λ ≈ 5.397 m) as the most restrictive case.
// If the wall passes at 63 Hz it passes for all higher-frequency bands.
//
//	l_min · cos(β) > √(2λ / (1/d_so + 1/d_or))
//
// lMin:  smallest dimension of the reflector [m] (typically wall length or height)
// beta:  angle between source→receiver line and reflector normal [rad]
// dSO:   source-to-reflector distance [m]
// dOR:   reflector-to-receiver distance [m]
func FresnelCheck(lMin, beta, dSO, dOR float64) bool {
	const lambda63 = speedOfSound / 63.0 // ≈ 5.397 m

	if dSO <= 0 || dOR <= 0 {
		return false
	}

	lhs := lMin * math.Cos(beta)
	rhs := math.Sqrt(2.0 * lambda63 / (1.0/dSO + 1.0/dOR))

	return lhs > rhs
}
