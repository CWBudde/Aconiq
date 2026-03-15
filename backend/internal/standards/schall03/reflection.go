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
