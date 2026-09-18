package schall03

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/numeric"
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
	A       geo.Point2D     `json:"a"`        // first endpoint
	B       geo.Point2D     `json:"b"`        // second endpoint
	HeightM float64         `json:"height_m"` // wall height above ground [m]
	Surface WallSurfaceType `json:"surface"`  // acoustic surface category (Table 18)
	// ObstacleID names the physical obstacle this facade belongs to, in the
	// same namespace as BarrierSegment.ObstacleID.  A building is reflector
	// and obstacle at once, and a mirrored ray crosses its own reflector by
	// construction, so the identity has to travel with the path: without it
	// the facade diffracts the very reflection it produced.
	//
	// An empty ObstacleID means "this wall belongs to no named obstacle", so
	// a scene built before obstacle identity existed keeps its exact previous
	// behaviour: nothing is excluded from its reflected paths.
	ObstacleID string `json:"obstacle_id,omitempty"`
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

// ReflectedSubsegmentContrib computes the linear acoustic power contribution
// from one track subsegment to a receiver along a reflected path.
//
// It runs the same kernel as normativeSubsegmentContrib (subsegmentContrib in
// compute.go, Gl. 6, 8-16) but:
//   - Uses the reflected path distance instead of the direct distance
//   - Applies the cumulative absorption loss D_ρ from wall reflections (Gl. 28)
//   - No barrier diffraction (use ReflectedSubsegmentContribWithBarriers for barrier support)
//
// dp:              horizontal reflected path distance [m]
// stepLen:         subsegment length [m]
// sinDelta2:       sin²(δ) directivity for the reflected path direction
// waterFractionW:  fraction of reflected path over water [0, 1]
// dRho:            cumulative absorption loss from reflections [dB] (negative or zero).
func ReflectedSubsegmentContrib(
	emission *StreckeEmissionResult,
	elevationM float64,
	receiver ReceiverInput,
	dp, stepLen, sinDelta2, waterFractionW float64,
	dRho, groundOffsetM float64,
) float64 {
	// Gl. 28: the image source level includes D_ρ; the rest of the chain is the
	// direct-path kernel, here without barriers.
	return subsegmentContrib(
		emission, elevationM, receiver, geo.Point2D{},
		dp, stepLen, sinDelta2, waterFractionW, dRho, groundOffsetM,
		nil,
	)
}

// ReflectedSubsegmentContribWithBarriers is like ReflectedSubsegmentContrib but
// includes barrier attenuation along the reflected path.  imageSource must be
// the fully unfolded origin of that path — the source mirrored across every wall
// it bounces off, which ReflectionPath.EffectiveSource returns — so that the
// diffraction check runs along the ray dp measures.
//
// excludeObstacleIDs names the obstacles the path bounced off; their panels are
// dropped from the barrier set before the diffraction check, see
// barriersExcludingObstacles.
func ReflectedSubsegmentContribWithBarriers(
	emission *StreckeEmissionResult,
	elevationM float64,
	receiver ReceiverInput,
	imageSource geo.Point2D,
	dp, stepLen, sinDelta2, waterFractionW float64,
	dRho, groundOffsetM float64,
	barriers []BarrierSegment,
	excludeObstacleIDs []string,
) float64 {
	barriers = barriersExcludingObstacles(barriers, excludeObstacleIDs)
	if len(barriers) == 0 {
		return ReflectedSubsegmentContrib(emission, elevationM, receiver, dp, stepLen, sinDelta2, waterFractionW, dRho, groundOffsetM)
	}

	// Gl. 28 for D_ρ, and the diffraction check along the unfolded ray
	// imageSource → receiver.
	return subsegmentContrib(
		emission, elevationM, receiver, imageSource,
		dp, stepLen, sinDelta2, waterFractionW, dRho, groundOffsetM,
		barriers,
	)
}

// barriersExcludingObstacles drops the panels of the obstacles a mirrored path
// bounced off.
//
// A reflected ray crosses its own reflector by construction — that crossing is
// what ComputeReflectionGeometry solves for — so counting it as a diffraction
// edge would invent a shielding loss exactly where the standard sees a
// reflection.  A building is obstacle and reflector at once
// (appendSchall03Building emits both from one footprint under one ObstacleID),
// which is why the identity travels with the path.
//
// The exclusion is per obstacle, not per facade: another wall of the *same*
// building does not shield that building's own mirrored path.  That direction
// over-predicts the level and is declared in the conformance document.
//
// A panel with an empty ObstacleID belongs to no named obstacle and is never
// excluded, so a scene built before obstacle identity existed is unchanged.
func barriersExcludingObstacles(barriers []BarrierSegment, ids []string) []BarrierSegment {
	if len(ids) == 0 || len(barriers) == 0 {
		return barriers
	}

	excluded := false

	for _, b := range barriers {
		if b.ObstacleID != "" && slices.Contains(ids, b.ObstacleID) {
			excluded = true

			break
		}
	}

	if !excluded {
		return barriers
	}

	kept := make([]BarrierSegment, 0, len(barriers))

	for _, b := range barriers {
		if b.ObstacleID == "" || !slices.Contains(ids, b.ObstacleID) {
			kept = append(kept, b)
		}
	}

	return kept
}

// reflectionPathObstacleIDs returns the obstacles a path bounced off, in bounce
// order and without repeats.  Walls that belong to no named obstacle contribute
// nothing, so the result is empty for a scene without obstacle identity and
// barriersExcludingObstacles then returns the barrier slice untouched.
func reflectionPathObstacleIDs(rp ReflectionPath, walls []ReflectingWall) []string {
	var ids []string

	for _, idx := range rp.Walls {
		if idx < 0 || idx >= len(walls) {
			continue
		}

		id := walls[idx].ObstacleID
		if id == "" || slices.Contains(ids, id) {
			continue
		}

		ids = append(ids, id)
	}

	return ids
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

// EffectiveSource is the fully unfolded origin of the path — the image source of
// the *last* bounce, which is what a barrier along the path must be checked
// against.  For order 1 it is Geometries[0].ImageSource; for higher orders it is
// not, because Geometries[0].ImageSource is only the first mirror.
//
// EnumerateReflectionPaths mirrors the running image source once per bounce
// (each ReflectionGeometry is solved from the previous image source), so the
// last ImageSource is the source reflected across every wall of the path in
// order.  That is also the point TotalDist measures from: TotalDist equals
// geo.Distance(EffectiveSource(), receiver).  Propagation distance and
// diffraction ray therefore describe one and the same ray.
//
// A path with no geometries cannot occur — the enumerator appends a geometry per
// bounce and never emits an order-0 path — but the method is exported, so an
// empty path returns the zero point rather than panicking.
func (p ReflectionPath) EffectiveSource() geo.Point2D {
	if len(p.Geometries) == 0 {
		return geo.Point2D{}
	}

	return p.Geometries[len(p.Geometries)-1].ImageSource
}

// candidate holds in-progress state for multi-order reflection path enumeration.
type candidate struct {
	imageSource geo.Point2D
	walls       []int
	geometries  []ReflectionGeometry
	totalDist   float64
	dRho        float64
}

// EnumerateReflectionPaths finds all valid reflection paths from source to
// receiver via the given walls, up to maxOrder bounces.  Paths that fail the
// Fresnel check (Gl. 27) are excluded.  Consecutive bounces off the same wall
// are excluded.
func EnumerateReflectionPaths(source, receiver geo.Point2D, walls []ReflectingWall, maxOrder int) []ReflectionPath {
	if maxOrder > MaxReflectionOrder {
		maxOrder = MaxReflectionOrder
	}

	var (
		result  []ReflectionPath
		current []candidate
	)

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
			extendReflectionCandidates(c, receiver, walls, order, &result, &next)
		}

		current = next
	}

	return result
}

// extendReflectionCandidates extends a single candidate by one additional bounce
// off each eligible wall, appending valid paths to result and new candidates to next.
func extendReflectionCandidates(c candidate, receiver geo.Point2D, walls []ReflectingWall, order int, result *[]ReflectionPath, next *[]candidate) {
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

		*result = append(*result, ReflectionPath{
			Order:      order,
			Walls:      newWalls,
			Geometries: newGeoms,
			TotalDist:  totalDist,
			DRho:       dRho,
		})

		*next = append(*next, candidate{
			imageSource: newImage,
			walls:       newWalls,
			geometries:  newGeoms,
			totalDist:   totalDist,
			dRho:        dRho,
		})
	}
}

// ComputeReflectedLineSourceLpAeq integrates reflected path contributions along
// a track centerline for one emission result, returning the total reflected
// contribution in dB.
func ComputeReflectedLineSourceLpAeq(
	emission *StreckeEmissionResult,
	centerline []geo.Point2D,
	elevationM float64,
	receiver ReceiverInput,
	waterFractionW float64,
	walls []ReflectingWall,
) float64 {
	return reflectedLineSourceLpAeq(emission, centerline, elevationM, receiver, waterFractionW, walls, nil)
}

// reflectedLineSourceLpAeq is ComputeReflectedLineSourceLpAeq with the run's
// terrain model in hand.  See reflectedPathGroundOffset for which ground a
// mirrored path is measured against.
func reflectedLineSourceLpAeq(
	emission *StreckeEmissionResult,
	centerline []geo.Point2D,
	elevationM float64,
	receiver ReceiverInput,
	waterFractionW float64,
	walls []ReflectingWall,
	dtm terrain.Model,
) float64 {
	if len(walls) == 0 {
		return math.Inf(-1)
	}

	var total numeric.CompensatedSum

	eachSubsegment(centerline, func(pt geo.Point2D, stepLen, tvX, tvY, tvLen float64) {
		paths := EnumerateReflectionPaths(pt, receiver.Point, walls, MaxReflectionOrder)
		if len(paths) == 0 {
			return
		}

		groundOffsetM := reflectedPathGroundOffset(dtm, pt, receiver)

		for _, rp := range paths {
			reflDist, sd2 := reflectedRayTerms(pt, rp, tvX, tvY, tvLen)

			total.Add(ReflectedSubsegmentContrib(
				emission, elevationM, receiver,
				reflDist, stepLen, sd2, waterFractionW, rp.DRho, groundOffsetM,
			))
		}
	})

	return lineSourceLevel(total.Sum())
}

// reflectedPathGroundOffset resolves the ground under a *mirrored* path.
//
// It is measured along the real subsegment→receiver line, not along the
// unfolded image-source ray.  The unfolded ray's origin is a construction, not
// a place: it can land inside a hillside, on the far side of a valley, or
// outside the DTM entirely, and the terrain it would cross is terrain the sound
// never travels over.  The sound does travel from this subsegment to this
// receiver — by a detour — and the ground along that corridor is the only
// physical ground the path has.
//
// RLS-19 draws the same line at rls19/road/propagation.go (hmRefl): declared
// terrain edges are left off the mirrored leg, and h_m is taken from the ground
// each real end stands on.  Schall 03 additionally keeps the rise along that
// real corridor, because here the DTM is the only ground description there is
// and dropping it would put a reflected path back on the flat plane while the
// direct path from the same subsegment stands on the hill.
func reflectedPathGroundOffset(dtm terrain.Model, source geo.Point2D, receiver ReceiverInput) float64 {
	return resolvePathGroundOffset(dtm, source, receiver)
}

// reflectedRayTerms returns the propagation distance and sin²(δ) for one
// enumerated reflection path leaving the subsegment midpoint pt.
//
// The distance is the total reflected path length, clamped to the 1 m
// near-field floor.
//
// The directivity uses a different ray: Gl. 28's D_Ir is the directivity of the
// point source "in der Richtung des Spiegelschallempfängers" — the direction
// from the source towards the mirrored receiver, which is the same ray as
// source → first reflection point (Bild 8: R lies on Q–IO_i).  The
// source→own-image direction is perpendicular to the wall by construction and
// does not depend on the receiver at all.
func reflectedRayTerms(pt geo.Point2D, rp ReflectionPath, tvX, tvY, tvLen float64) (float64, float64) {
	firstGeom := rp.Geometries[0]
	rvX := firstGeom.ReflectionPoint.X - pt.X
	rvY := firstGeom.ReflectionPoint.Y - pt.Y
	dp := math.Sqrt(rvX*rvX + rvY*rvY)

	if dp < 1 {
		dp = 1
	}

	sinDelta2 := normativeSinDelta2(rvX, rvY, dp, tvX, tvY, tvLen)

	reflDist := rp.TotalDist
	if reflDist < 1 {
		reflDist = 1
	}

	return reflDist, sinDelta2
}

// ComputeReflectedLineSourceLpAeqWithBarriers is like
// ComputeReflectedLineSourceLpAeq but includes barrier attenuation on reflected
// paths.
//
// For each reflected path the barrier check runs from the path's
// EffectiveSource — the image source of the *last* bounce, i.e. the source
// mirrored across every wall of the path.  That is the origin of the fully
// unfolded ray, and the one the path's TotalDist is measured from, so the
// diffraction check and the propagation distance describe the same ray at every
// order.  The first bounce's image source would only do for order 1; for two or
// three bounces it is a ray the sound never travels.
//
// The obstacles the path bounced off are excluded from the barrier set — a
// facade must not diffract its own reflection.
func ComputeReflectedLineSourceLpAeqWithBarriers(
	emission *StreckeEmissionResult,
	centerline []geo.Point2D,
	elevationM float64,
	receiver ReceiverInput,
	waterFractionW float64,
	walls []ReflectingWall,
	barriers []BarrierSegment,
) float64 {
	return reflectedLineSourceLpAeqWithBarriers(
		emission, centerline, elevationM, receiver, waterFractionW, walls, barriers, nil,
	)
}

// reflectedLineSourceLpAeqWithBarriers is
// ComputeReflectedLineSourceLpAeqWithBarriers with the run's terrain model in
// hand.
func reflectedLineSourceLpAeqWithBarriers(
	emission *StreckeEmissionResult,
	centerline []geo.Point2D,
	elevationM float64,
	receiver ReceiverInput,
	waterFractionW float64,
	walls []ReflectingWall,
	barriers []BarrierSegment,
	dtm terrain.Model,
) float64 {
	if len(walls) == 0 {
		return math.Inf(-1)
	}

	if len(barriers) == 0 {
		return reflectedLineSourceLpAeq(emission, centerline, elevationM, receiver, waterFractionW, walls, dtm)
	}

	var total numeric.CompensatedSum

	eachSubsegment(centerline, func(pt geo.Point2D, stepLen, tvX, tvY, tvLen float64) {
		paths := EnumerateReflectionPaths(pt, receiver.Point, walls, MaxReflectionOrder)
		if len(paths) == 0 {
			return
		}

		groundOffsetM := reflectedPathGroundOffset(dtm, pt, receiver)

		for _, rp := range paths {
			reflDist, sd2 := reflectedRayTerms(pt, rp, tvX, tvY, tvLen)

			total.Add(ReflectedSubsegmentContribWithBarriers(
				emission, elevationM, receiver,
				rp.EffectiveSource(),
				reflDist, stepLen, sd2, waterFractionW, rp.DRho, groundOffsetM,
				barriers,
				reflectionPathObstacleIDs(rp, walls),
			))
		}
	})

	return lineSourceLevel(total.Sum())
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
// dOR:   reflector-to-receiver distance [m].
func FresnelCheck(lMin, beta, dSO, dOR float64) bool {
	const lambda63 = speedOfSound / 63.0 // ≈ 5.397 m

	if dSO <= 0 || dOR <= 0 {
		return false
	}

	lhs := lMin * math.Cos(beta)
	rhs := math.Sqrt(2.0 * lambda63 / (1.0/dSO + 1.0/dOR))

	return lhs > rhs
}
