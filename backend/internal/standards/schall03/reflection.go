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

// ReflectionGeometry holds the computed geometry for one specular reflection.
type ReflectionGeometry struct {
	ReflectionPoint geo.Point2D // point on wall where reflection occurs
	ImageSource     geo.Point2D // mirror source position
	DSO             float64     // source-to-reflection-point distance [m]
	DOR             float64     // reflection-point-to-receiver distance [m]
	Beta            float64     // angle between reflected path and wall normal [rad]
	LMin            float64     // smallest wall dimension (min of length, height) [m]
}

// ComputeReflectionGeometry determines the specular reflection geometry for a
// source, receiver, and wall.  Returns (geometry, true) if a valid reflection
// point exists on the wall segment and both source and receiver are on the same
// side of the wall.  Returns (zero, false) otherwise.
func ComputeReflectionGeometry(source, receiver geo.Point2D, wall ReflectingWall) (ReflectionGeometry, bool) {
	// 1. Mirror the source across the wall line.
	imageSource, ok := MirrorSource(source, wall)
	if !ok {
		return ReflectionGeometry{}, false
	}

	// 2. Check source and receiver are on the same side of the wall.
	if !sameSide(source, receiver, wall) {
		return ReflectionGeometry{}, false
	}

	// 3. Find where the line from imageSource to receiver intersects the wall
	//    segment.  This is the reflection point.
	reflPoint, ok := segmentLineIntersection(imageSource, receiver, wall.A, wall.B)
	if !ok {
		return ReflectionGeometry{}, false
	}

	// 4. Compute distances and angle.
	dSO := geo.Distance(source, reflPoint)
	dOR := geo.Distance(reflPoint, receiver)

	// Wall normal (perpendicular to wall direction, either orientation).
	dx := wall.B.X - wall.A.X
	dy := wall.B.Y - wall.A.Y
	wallLen := math.Sqrt(dx*dx + dy*dy)
	// Normal: (-dy, dx) normalized.
	nx := -dy / wallLen
	ny := dx / wallLen

	// β = angle between the reflected path direction at the reflection point
	// and the wall normal.
	toRecvX := receiver.X - reflPoint.X
	toRecvY := receiver.Y - reflPoint.Y
	toRecvLen := math.Sqrt(toRecvX*toRecvX + toRecvY*toRecvY)

	beta := 0.0
	if toRecvLen > 1e-9 {
		cosB := math.Abs(toRecvX*nx+toRecvY*ny) / toRecvLen
		cosB = math.Min(cosB, 1.0)
		beta = math.Acos(cosB)
	}

	lMin := math.Min(wallLen, wall.HeightM)

	return ReflectionGeometry{
		ReflectionPoint: reflPoint,
		ImageSource:     imageSource,
		DSO:             dSO,
		DOR:             dOR,
		Beta:            beta,
		LMin:            lMin,
	}, true
}

// sameSide returns true if points p and q are on the same side of the line
// defined by the wall segment (or if either is exactly on the line).
func sameSide(p, q geo.Point2D, wall ReflectingWall) bool {
	dx := wall.B.X - wall.A.X
	dy := wall.B.Y - wall.A.Y
	crossP := dx*(p.Y-wall.A.Y) - dy*(p.X-wall.A.X)
	crossQ := dx*(q.Y-wall.A.Y) - dy*(q.X-wall.A.X)

	return crossP*crossQ >= 0
}

// segmentLineIntersection finds the point where the line through p1→p2
// intersects the segment s1→s2.  Returns (point, true) if the intersection
// lies within the segment (0 ≤ t ≤ 1).
func segmentLineIntersection(p1, p2, s1, s2 geo.Point2D) (geo.Point2D, bool) {
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	sx := s2.X - s1.X
	sy := s2.Y - s1.Y

	denom := sx*dy - sy*dx
	if math.Abs(denom) < 1e-12 {
		return geo.Point2D{}, false // parallel
	}

	// Parameter along the segment s1→s2.
	t := ((p1.X-s1.X)*dy - (p1.Y-s1.Y)*dx) / denom

	if t < 0 || t > 1 {
		return geo.Point2D{}, false // outside segment
	}

	return geo.Point2D{
		X: s1.X + t*sx,
		Y: s1.Y + t*sy,
	}, true
}

// MaxReflectionOrder is the maximum number of bounces per Schall 03.
const MaxReflectionOrder = 3

// ReflectionPath describes one validated multi-order reflection path.
type ReflectionPath struct {
	Order      int                  // reflection order (1 = single bounce, 2 = double, etc.)
	Walls      []int                // indices into the walls slice, one per bounce
	Geometries []ReflectionGeometry // geometry per bounce
	TotalDist  float64              // total reflected path length [m]
	DRho       float64              // cumulative absorption loss D_ρ [dB]
}

// EnumerateReflectionPaths finds all valid reflection paths from source to
// receiver via the given walls, up to maxOrder bounces.  Paths that fail the
// Fresnel check (Gl. 27) are excluded.  Consecutive bounces off the same wall
// are excluded.
func EnumerateReflectionPaths(source, receiver geo.Point2D, walls []ReflectingWall, maxOrder int) []ReflectionPath {
	if maxOrder > MaxReflectionOrder {
		maxOrder = MaxReflectionOrder
	}

	var result []ReflectionPath

	// 1st order: source → wall[i] → receiver.
	type candidate struct {
		imageSource geo.Point2D
		walls       []int
		geometries  []ReflectionGeometry
		totalDist   float64
		dRho        float64
	}

	var current []candidate

	for i, w := range walls {
		rg, ok := ComputeReflectionGeometry(source, receiver, w)
		if !ok {
			continue
		}

		if !FresnelCheck(rg.LMin, rg.Beta, rg.DSO, rg.DOR) {
			continue
		}

		path := ReflectionPath{
			Order:      1,
			Walls:      []int{i},
			Geometries: []ReflectionGeometry{rg},
			TotalDist:  rg.DSO + rg.DOR,
			DRho:       Table18AbsorptionLoss(w.Surface),
		}
		result = append(result, path)

		current = append(current, candidate{
			imageSource: rg.ImageSource,
			walls:       []int{i},
			geometries:  []ReflectionGeometry{rg},
			totalDist:   rg.DSO + rg.DOR,
			dRho:        Table18AbsorptionLoss(w.Surface),
		})
	}

	// Higher orders: iteratively extend paths.
	for order := 2; order <= maxOrder; order++ {
		var next []candidate

		for _, c := range current {
			lastWallIdx := c.walls[len(c.walls)-1]

			for j, w := range walls {
				if j == lastWallIdx {
					continue // no consecutive same-wall bounces
				}

				// Check if the previous image source can see the receiver via wall j.
				rg, ok := ComputeReflectionGeometry(c.imageSource, receiver, w)
				if !ok {
					continue
				}

				if !FresnelCheck(rg.LMin, rg.Beta, rg.DSO, rg.DOR) {
					continue
				}

				// Mirror the previous image source across wall j for next order.
				newImage, ok := MirrorSource(c.imageSource, w)
				if !ok {
					continue
				}

				newWalls := make([]int, len(c.walls)+1)
				copy(newWalls, c.walls)
				newWalls[len(c.walls)] = j

				newGeoms := make([]ReflectionGeometry, len(c.geometries)+1)
				copy(newGeoms, c.geometries)
				newGeoms[len(c.geometries)] = rg

				dRho := c.dRho + Table18AbsorptionLoss(w.Surface)
				totalDist := rg.DSO + rg.DOR

				path := ReflectionPath{
					Order:      order,
					Walls:      newWalls,
					Geometries: newGeoms,
					TotalDist:  totalDist,
					DRho:       dRho,
				}
				result = append(result, path)

				next = append(next, candidate{
					imageSource: newImage,
					walls:       newWalls,
					geometries:  newGeoms,
					totalDist:   totalDist,
					dRho:        dRho,
				})
			}
		}

		current = next
	}

	return result
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
