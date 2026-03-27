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
	// Reflector falls back to ReflectionLossDB (or 1.0 dB if unset).
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

	// ReflectionLossDB is the explicit energy loss per reflection [dB].
	// Used only when Type is ReflectorTypeUnspecified.
	// Defaults to 1.0 dB when zero or unset (backward-compatible default).
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
// backward-compatible default of 1.0 dB.
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
		return 1.0
	}

	return r.ReflectionLossDB
}

// reflectedPath holds one reflected sound path from source to receiver.
type reflectedPath struct {
	planDistM  float64 // plan-view distance of the full reflected path [m]
	slantDistM float64 // 3D slant distance [m]
	lossDB     float64 // total reflection loss (sum over all bounces) [dB]
}

// wallSeg is an internal view of one wall segment from a Reflector.
type wallSeg struct {
	a, b    geo.Point2D
	loss    float64 // per-reflection loss for this wall
	heightM float64 // wall top height above ground [m]
}

// reflectorWalls flattens all reflectors into individual wall segments.
func reflectorWalls(reflectors []Reflector) []wallSeg {
	var walls []wallSeg

	for _, r := range reflectors {
		for i := range len(r.Geometry) - 1 {
			walls = append(walls, wallSeg{
				a:       r.Geometry[i],
				b:       r.Geometry[i+1],
				loss:    r.effectiveLoss(),
				heightM: r.HeightM,
			})
		}
	}

	return walls
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

	walls := reflectorWalls(reflectors)
	dz := receiverZ - sourceZ

	paths := firstOrderReflections(source, receiver, sourceZ, dz, walls)

	return append(paths, secondOrderReflections(source, receiver, sourceZ, dz, walls)...)
}

func firstOrderReflections(source, receiver geo.Point2D, sourceZ, dz float64, walls []wallSeg) []reflectedPath {
	var paths []reflectedPath

	for _, w := range walls {
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
			planDistM:  planDist,
			slantDistM: slantDist,
			lossDB:     w.loss, // D_RV1 for this 1st-order path (RLS-19 Eq. 2)
		})
	}

	return paths
}

func secondOrderReflections(source, receiver geo.Point2D, sourceZ, dz float64, walls []wallSeg) []reflectedPath {
	var paths []reflectedPath

	for i, w1 := range walls {
		img1 := mirrorPoint(source, w1.a, w1.b) // image after 1st bounce

		for j, w2 := range walls {
			if i == j {
				continue // same wall segment: skip
			}

			img2 := mirrorPoint(img1, w2.a, w2.b) // image after 2nd bounce

			// Check: S''→R crosses wall 2 (gives 2nd reflection point P2).
			p2, _, ok2 := geo.LineStringIntersectsSegment([]geo.Point2D{w2.a, w2.b}, img2, receiver)
			if !ok2 {
				continue
			}

			// Check: P2→S' crosses wall 1 (gives 1st reflection point P1 and
			// confirms the 1st leg is geometrically valid).
			p1, _, ok1 := geo.LineStringIntersectsSegment([]geo.Point2D{w1.a, w1.b}, p2, img1)
			if !ok1 {
				continue
			}

			// RLS-19 Tabelle 8 normative height condition at each bounce point.
			// At P1 (1st bounce on W1): a_R = min(dist(S,P1), dist(P1,P2)).
			// At P2 (2nd bounce on W2): a_R = min(dist(P1,P2), dist(P2,R)).
			aR1 := math.Min(dist2D(source, p1), dist2D(p1, p2))
			if w1.heightM < 1.0 || w1.heightM < 0.3*math.Sqrt(aR1) {
				continue
			}

			aR2 := math.Min(dist2D(p1, p2), dist2D(p2, receiver))
			if w2.heightM < 1.0 || w2.heightM < 0.3*math.Sqrt(aR2) {
				continue
			}

			// Geometric height condition at P2: wall 2 must be tall enough so
			// the ray does not pass over it.
			// Height at P2 = sourceZ + dz · dist(img2, P2) / dist(img2, receiver).
			planDist := dist2D(img2, receiver)
			if planDist > 0 {
				t2 := dist2D(img2, p2) / planDist

				heightAtP2 := sourceZ + dz*t2
				if w2.heightM < heightAtP2 {
					continue // ray passes over wall 2 at P2
				}
			}

			slantDist := math.Sqrt(planDist*planDist + dz*dz)
			paths = append(paths, reflectedPath{
				planDistM:  planDist,
				slantDistM: slantDist,
				lossDB:     w1.loss + w2.loss, // D_RV1 + D_RV2 (RLS-19 Eq. 3)
			})
		}
	}

	return paths
}
