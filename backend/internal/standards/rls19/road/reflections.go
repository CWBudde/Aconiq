package road

import (
	"errors"
	"fmt"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// ReflectorType classifies the acoustic absorption of a reflector surface
// per RLS-19 Tabelle 8. When set on a Reflector, the corresponding loss value
// takes precedence over the explicit ReflectionLossDB field.
type ReflectorType int

const (
	// ReflectorTypeUnspecified means no typed surface class is set; the
	// Reflector falls back to ReflectionLossDB, or — if that is unset too — to
	// the Tabelle 8 facade row, 0.5 dB.
	ReflectorTypeUnspecified ReflectorType = iota

	// ReflectorTypeFacadeOrReflecting is a schallharte Fassade oder Wand
	// (hard/reflective facade or wall): D_RV = 0.5 dB (Tabelle 8).
	ReflectorTypeFacadeOrReflecting

	// ReflectorTypeReflectionReducing is a schallabsorbierende Wand
	// (sound-absorbing wall): D_RV = 3.0 dB (Tabelle 8).
	ReflectorTypeReflectionReducing

	// ReflectorTypeStronglyReflectionReducing is a stark schallabsorbierende
	// Wand (strongly sound-absorbing wall): D_RV = 5.0 dB (Tabelle 8).
	ReflectorTypeStronglyReflectionReducing
)

// Reflector is a building facade or other planar vertical surface that can
// reflect sound from a source to a receiver via the image-source method.
//
// RLS-19 allows up to two reflections (3rd-order reflections are ignored per
// the standard). Each reflection adds energy via an additional propagation
// path; the reflection loss (D_RV per Tabelle 8) is subtracted from that
// path's level. A Reflector is distinct from a Barrier: barriers attenuate
// the direct path, reflectors add new indirect paths.
//
// A reflector only participates when the RLS-19 Tabelle 8 height condition is
// satisfied at the reflection point P:
//
//	h_R ≥ 1.0 m  AND  h_R ≥ 0.3·√(a_R)
//
// where a_R is the smaller of the source-to-P and P-to-receiver plan distances.
//
// Future building/courtyard scenarios (Phase 17) will compose buildings from
// multiple Reflector facades.
type Reflector struct {
	ID       string        `json:"id"`
	Geometry []geo.Point2D `json:"geometry"` // facade polyline in plan view
	HeightM  float64       `json:"height_m"` // facade height above ground [m]

	// Type classifies the surface absorption per RLS-19 Tabelle 8.
	// When non-zero, the typed loss value takes precedence over ReflectionLossDB.
	Type ReflectorType `json:"type,omitempty"`

	// ReflectionLossDB is a deliberate override of the Tabelle 8 value, in dB
	// per reflection. It applies only when Type is ReflectorTypeUnspecified.
	// When it is zero or unset the loss falls back to the Tabelle 8 facade row
	// (0.5 dB) — see effectiveLoss.
	ReflectionLossDB float64 `json:"reflection_loss_db,omitempty"`
}

// Validate checks a reflector definition.
func (r Reflector) Validate() error {
	if r.ID == "" {
		return errors.New("reflector id is required")
	}

	if len(r.Geometry) < 2 {
		return fmt.Errorf("reflector %q geometry must contain at least 2 points", r.ID)
	}

	for i, pt := range r.Geometry {
		if !pt.IsFinite() {
			return fmt.Errorf("reflector %q geometry point[%d] is not finite", r.ID, i)
		}
	}

	if !isFinite(r.HeightM) || r.HeightM <= 0 {
		return fmt.Errorf("reflector %q height_m must be finite and > 0", r.ID)
	}

	if !isFinite(r.ReflectionLossDB) || r.ReflectionLossDB < 0 {
		return fmt.Errorf("reflector %q reflection_loss_db must be finite and >= 0", r.ID)
	}

	return nil
}

// effectiveLoss returns the per-reflection loss D_RV [dB].
//
// Priority: typed surface class (Tabelle 8) > explicit ReflectionLossDB >
// the Tabelle 8 default.
//
// The default is the "Gebäudefassaden und reflektierende Lärmschutzwände" row,
// 0.5 dB. RLS-19 Tabelle 8 has only three rows — 0.5, 3.0 and 5.0 dB — so an
// untyped reflector has to land on one of them, and the facade row is both the
// commonest untyped case and the conservative one: it is the smallest loss in
// the table, hence the highest resulting level.
func (r Reflector) effectiveLoss() float64 {
	switch r.Type {
	case ReflectorTypeFacadeOrReflecting:
		return 0.5
	case ReflectorTypeReflectionReducing:
		return 3.0
	case ReflectorTypeStronglyReflectionReducing:
		return 5.0
	}

	if r.ReflectionLossDB <= 0 {
		return 0.5
	}

	return r.ReflectionLossDB
}

// reflectedPath holds one reflected sound path from source to receiver.
//
// imagePoint and reflectorIDs exist so the path can be attenuated like any
// other path. RLS-19 Nr. 3.5 states that the source of a propagation
// calculation may itself be a Spiegelschallquelle, so the mirrored path takes
// the same Eq. 11 chain the direct one does — and the shielding calculation
// needs the image position to run from, plus the identity of the surfaces it
// bounced off, because a reflected ray crosses its own reflector by
// construction. See dropExcludedBarriers.
type reflectedPath struct {
	planDistM  float64 // plan-view distance of the full reflected path [m]
	slantDistM float64 // 3D slant distance [m]
	lossDB     float64 // total reflection loss (sum over all bounces) [dB]

	imagePoint   geo.Point2D // final image source position (1st or 2nd order)
	reflectorIDs []string    // reflectors this path bounced off, in bounce order
}

// wallSeg is an internal view of one wall segment from a Reflector.
type wallSeg struct {
	a, b    geo.Point2D
	ownerID string  // ID of the Reflector this segment came from
	loss    float64 // per-reflection loss for this wall
	heightM float64 // wall top height above ground [m]

	// bbox is the segment's own bounding box, kept so that a prune can reject
	// the wall without touching its endpoints again.
	bbox geo.BBox

	// maxRangeM is the largest a_R at which RLS-19 Tabelle 8's height
	// condition can still hold for this wall: h_R ≥ 0.3·√(a_R) rearranges to
	// a_R ≤ (h_R/0.3)². It is negative when the wall is below the 1.0 m the
	// same condition demands outright, so that no distance is ever in range
	// and the reflection search skips the wall exactly as the condition does.
	maxRangeM   float64
	maxRangeSqM float64
}

// reflectorWalls flattens all reflectors into individual wall segments.
func reflectorWalls(reflectors []Reflector) []wallSeg {
	var walls []wallSeg

	for _, r := range reflectors {
		loss := r.effectiveLoss()
		for i := range len(r.Geometry) - 1 {
			walls = append(walls, newWallSeg(r.Geometry[i], r.Geometry[i+1], r.ID, loss, r.HeightM))
		}
	}

	return walls
}

// newWallSeg derives the cached geometry and Tabelle 8 range of one wall.
func newWallSeg(a, b geo.Point2D, ownerID string, loss, heightM float64) wallSeg {
	maxRange := -1.0
	if heightM >= 1.0 {
		// The margin is one part in a billion plus a millimetre, so that a
		// wall sitting exactly at its own limit is kept rather than pruned:
		// the search compares 0.3·√(a_R) against h_R, and squaring that
		// comparison here must not be able to flip it.
		maxRange = (heightM/0.3)*(heightM/0.3)*(1+1e-9) + queryMarginM
	}

	return wallSeg{
		a:           a,
		b:           b,
		ownerID:     ownerID,
		loss:        loss,
		heightM:     heightM,
		bbox:        segmentBBox(a, b),
		maxRangeM:   maxRange,
		maxRangeSqM: maxRange * maxRange,
	}
}

// reflectorField is the receiver-independent half of the reflector set: the
// reflector polylines already flattened into wall segments, and the spatial
// index over them.
//
// Flattening used to happen inside computeReflectedPaths, which runs once per
// Teilstück and receiver — a 200-building model rebuilt the same 800 walls
// for every pair. The index is what turns the second-order search from a walk
// over every pair of walls into a walk over the pairs that can exist.
type reflectorField struct {
	walls []wallSeg
	grid  *geo.BBoxGrid

	// all is 0..len(walls)-1, the candidate list that narrows nothing. It is
	// held rather than built so that the unindexed path allocates nothing per
	// call either.
	all []int
}

// newReflectorField flattens the reflectors and indexes the result.
func newReflectorField(reflectors []Reflector) reflectorField {
	walls := reflectorWalls(reflectors)

	return reflectorField{
		walls: walls,
		grid: buildBBoxGrid(len(walls), func(i int) (geo.BBox, bool) {
			return walls[i].bbox, walls[i].bbox.IsFinite() && walls[i].bbox.IsValid()
		}),
		all: appendAllIndices(nil, len(walls)),
	}
}

// mirrorPoint mirrors p across the infinite line defined by segment a→b.
func mirrorPoint(p, a, b geo.Point2D) geo.Point2D {
	dx := b.X - a.X
	dy := b.Y - a.Y
	len2 := dx*dx + dy*dy

	if len2 < 1e-12 {
		return p // degenerate segment — return p unchanged
	}

	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / len2
	fx := a.X + t*dx
	fy := a.Y + t*dy

	return geo.Point2D{X: 2*fx - p.X, Y: 2*fy - p.Y}
}

// computeReflectedPaths returns all valid 1st- and 2nd-order reflected paths
// from source to receiver via the given reflectors, using the image-source
// method. The returned slice may be empty when no valid geometry exists.
//
// For a 1st-order reflection off wall W:
//   - S' = mirror of S across W
//   - Valid if segment S'→R crosses W
//   - Plan dist = dist2D(S', R)
//
// For a 2nd-order reflection off wall W1 then W2:
//   - S'  = mirror of S  across W1
//   - S” = mirror of S' across W2
//   - Valid if segment S”→R crosses W2 (gives P2) AND segment P2→S' crosses W1
//   - Plan dist = dist2D(S”, R)
func computeReflectedPaths(
	source geo.Point2D, sourceZ float64,
	receiver geo.Point2D, receiverZ float64,
	reflectors []Reflector,
) []reflectedPath {
	if len(reflectors) == 0 {
		return nil
	}

	field := newReflectorField(reflectors)

	return field.appendPaths(nil, source, sourceZ, receiver, receiverZ, nil)
}

// appendPaths appends every valid 1st- and 2nd-order path to dst, first order
// first and each order in wall order.
//
// The order is load-bearing: acoustics.EnergySum adds the contributions in
// slice order, so reordering the paths moves the result in the last bits.
// Every prune below therefore keeps the surviving paths in the order the
// unpruned search would have produced them.
func (f *reflectorField) appendPaths(
	dst []reflectedPath,
	source geo.Point2D, sourceZ float64,
	receiver geo.Point2D, receiverZ float64,
	scratch *pathScratch,
) []reflectedPath {
	if len(f.walls) == 0 {
		return dst
	}

	dz := receiverZ - sourceZ

	dst = f.appendFirstOrder(dst, source, receiver, sourceZ, dz)

	return f.appendSecondOrder(dst, source, receiver, sourceZ, dz, scratch)
}

// appendFirstOrder walks every wall, because a single bounce admits no exact
// spatial prune: the specular point of a distant wall can be anywhere, and
// RLS-19 sets no distance beyond which a reflection stops counting.
//
// What it does skip is the wall whose own Tabelle 8 range cannot reach either
// end of the path — the same condition the loop tests below, in the only form
// that can be checked before the mirror is computed. For a tall facade that
// range is hundreds of metres and the prune barely bites; for a low screen it
// removes the wall outright.
func (f *reflectorField) appendFirstOrder(
	dst []reflectedPath,
	source, receiver geo.Point2D,
	sourceZ, dz float64,
) []reflectedPath {
	paths := dst

	for i := range f.walls {
		w := &f.walls[i]
		if !w.inTabelle8Range(w.bbox.SquaredDistanceToPoint(source), w.bbox.SquaredDistanceToPoint(receiver)) {
			continue
		}

		img := mirrorPoint(source, w.a, w.b)
		p, _, ok := geo.LineStringIntersectsSegment([]geo.Point2D{w.a, w.b}, img, receiver)

		if !ok {
			continue
		}

		// RLS-19 Tabelle 8 normative height condition at reflection point P:
		//   h_R ≥ 1.0 m  AND  h_R ≥ 0.3·√(a_R)
		// a_R is the smaller of the source-to-P and P-to-receiver plan distances.
		aR := math.Min(dist2D(source, p), dist2D(p, receiver))
		if w.heightM < 1.0 || w.heightM < 0.3*math.Sqrt(aR) {
			continue
		}

		// Geometric height condition: the wall must be tall enough at P so
		// the ray does not pass over it.
		// Height at P = sourceZ + dz · dist(img, P) / dist(img, receiver).
		planDist := dist2D(img, receiver)
		if planDist > 0 {
			t := dist2D(img, p) / planDist

			heightAtP := sourceZ + dz*t
			if w.heightM < heightAtP {
				continue // ray passes over the wall
			}
		}

		slantDist := math.Sqrt(planDist*planDist + dz*dz)
		paths = append(paths, reflectedPath{
			planDistM:    planDist,
			slantDistM:   slantDist,
			lossDB:       w.loss, // D_RV1 for this 1st-order path (RLS-19 Eq. 2)
			imagePoint:   img,
			reflectorIDs: []string{w.ownerID},
		})
	}

	return paths
}

// appendSecondOrder walks the wall pairs that can carry a second-order path.
//
// The search is the same one it has always been; what changed is which pairs
// reach it. Two prunes narrow the inner loop, and both are exact — a pair they
// drop is a pair the loop below would have rejected anyway:
//
//   - The second bounce P2 lies on the ray from the image source through the
//     first bounce P1, extended beyond it: the ok1 test is precisely that P1
//     sits on the segment P2→S'. So P2 lies in the shadow w1 casts away from
//     S', and a wall outside that shadow cannot hold it. See
//     geo.SegmentShadow.
//   - Tabelle 8's height condition caps how far a bounce point may be from
//     the two path legs that meet there. The legs are at least as long as the
//     gaps between the bounding boxes they run between, so a wall out of
//     range of both of its own legs fails the condition the loop tests.
func (f *reflectorField) appendSecondOrder(
	dst []reflectedPath,
	source, receiver geo.Point2D,
	sourceZ, dz float64,
	scratch *pathScratch,
) []reflectedPath {
	paths := dst

	for i := range f.walls {
		w1 := &f.walls[i]
		if w1.maxRangeM < 0 {
			continue // below 1.0 m: Tabelle 8 rejects it at either bounce
		}

		img1 := mirrorPoint(source, w1.a, w1.b) // image after 1st bounce

		shadow := geo.NewSegmentShadow(img1, w1.a, w1.b, queryMarginM)

		candidates, reachable := f.secondBounceCandidates(&shadow, scratch)
		if !reachable {
			continue
		}

		sourceGapSq := w1.bbox.SquaredDistanceToPoint(source)

		for _, j := range candidates {
			if i == j {
				continue // same wall segment: skip
			}

			w2 := &f.walls[j]
			if !shadow.MayContainSegment(w2.a, w2.b) {
				continue // the second bounce cannot land on this wall
			}

			gapSq := w1.bbox.SquaredDistanceToBBox(w2.bbox)
			if !w1.inTabelle8Range(sourceGapSq, gapSq) ||
				!w2.inTabelle8Range(gapSq, w2.bbox.SquaredDistanceToPoint(receiver)) {
				continue
			}

			path, ok := secondOrderPath(w1, w2, img1, source, receiver, sourceZ, dz)
			if ok {
				paths = append(paths, path)
			}
		}
	}

	return paths
}

// secondOrderPath is the unchanged two-bounce geometry: mirror the image
// source across the second wall, confirm both bounce points lie on their own
// wall, and apply RLS-19's height conditions at each of them.
func secondOrderPath(
	w1, w2 *wallSeg,
	img1 geo.Point2D,
	source, receiver geo.Point2D,
	sourceZ, dz float64,
) (reflectedPath, bool) {
	img2 := mirrorPoint(img1, w2.a, w2.b) // image after 2nd bounce

	// Check: S''→R crosses wall 2 (gives 2nd reflection point P2).
	p2, _, ok2 := geo.LineStringIntersectsSegment([]geo.Point2D{w2.a, w2.b}, img2, receiver)
	if !ok2 {
		return reflectedPath{}, false
	}

	// Check: P2→S' crosses wall 1 (gives 1st reflection point P1 and
	// confirms the 1st leg is geometrically valid).
	p1, _, ok1 := geo.LineStringIntersectsSegment([]geo.Point2D{w1.a, w1.b}, p2, img1)
	if !ok1 {
		return reflectedPath{}, false
	}

	// RLS-19 Tabelle 8 normative height condition at each bounce point.
	// At P1 (1st bounce on W1): a_R = min(dist(S,P1), dist(P1,P2)).
	// At P2 (2nd bounce on W2): a_R = min(dist(P1,P2), dist(P2,R)).
	aR1 := math.Min(dist2D(source, p1), dist2D(p1, p2))
	if w1.heightM < 1.0 || w1.heightM < 0.3*math.Sqrt(aR1) {
		return reflectedPath{}, false
	}

	aR2 := math.Min(dist2D(p1, p2), dist2D(p2, receiver))
	if w2.heightM < 1.0 || w2.heightM < 0.3*math.Sqrt(aR2) {
		return reflectedPath{}, false
	}

	// Geometric height condition at P2: wall 2 must be tall enough so
	// the ray does not pass over it.
	// Height at P2 = sourceZ + dz · dist(img2, P2) / dist(img2, receiver).
	planDist := dist2D(img2, receiver)
	if planDist > 0 {
		t2 := dist2D(img2, p2) / planDist

		heightAtP2 := sourceZ + dz*t2
		if w2.heightM < heightAtP2 {
			return reflectedPath{}, false // ray passes over wall 2 at P2
		}
	}

	return reflectedPath{
		planDistM:    planDist,
		slantDistM:   math.Sqrt(planDist*planDist + dz*dz),
		lossDB:       w1.loss + w2.loss, // D_RV1 + D_RV2 (RLS-19 Eq. 3)
		imagePoint:   img2,
		reflectorIDs: []string{w1.ownerID, w2.ownerID},
	}, true
}

// secondBounceCandidates returns the walls that can hold the second bounce of
// a path whose first bounce is on w1, in ascending wall order, and reports
// false when no wall can.
//
// Without an index it answers with every wall, so an unindexed field runs the
// same search over the same order and only does more work.
func (f *reflectorField) secondBounceCandidates(
	shadow *geo.SegmentShadow,
	scratch *pathScratch,
) ([]int, bool) {
	if scratch == nil || !scratch.wallCursor.Ready() {
		return f.all, true
	}

	scratch.wallCandidates = scratch.wallCursor.QueryShadow(shadow, queryMarginM, scratch.wallCandidates[:0])

	return scratch.wallCandidates, len(scratch.wallCandidates) > 0
}

// inTabelle8Range reports whether a bounce on this wall can satisfy RLS-19
// Tabelle 8's height condition, given lower bounds on the squared lengths of
// the two path legs meeting at that bounce.
//
// a_R is the smaller of the two legs, so the condition can only hold when at
// least one of them is within the wall's range; if both lower bounds exceed
// it, a_R does too and 0.3·√(a_R) is above the wall.
func (w *wallSeg) inTabelle8Range(firstLegSqM, secondLegSqM float64) bool {
	if w.maxRangeM < 0 {
		return false // below 1.0 m, which the condition rejects outright
	}

	return firstLegSqM <= w.maxRangeSqM || secondLegSqM <= w.maxRangeSqM
}
