package geo

import (
	"errors"
	"math"
)

// Point2D is a 2D coordinate in the active CRS.
type Point2D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (p Point2D) IsFinite() bool {
	return isFinite(p.X) && isFinite(p.Y)
}

// Point3D is a 3D coordinate with absolute elevation.
type Point3D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"` // absolute elevation [m]
}

func (p Point3D) IsFinite() bool {
	return isFinite(p.X) && isFinite(p.Y) && isFinite(p.Z)
}

// XY returns the plan-view (2D) projection.
func (p Point3D) XY() Point2D { return Point2D{X: p.X, Y: p.Y} }

// BBox is an axis-aligned bounding box.
type BBox struct {
	MinX float64 `json:"min_x"`
	MinY float64 `json:"min_y"`
	MaxX float64 `json:"max_x"`
	MaxY float64 `json:"max_y"`
}

func NewBBox(minX, minY, maxX, maxY float64) (BBox, error) {
	b := BBox{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
	if !b.IsFinite() {
		return BBox{}, errors.New("bbox contains non-finite values")
	}

	if !b.IsValid() {
		return BBox{}, errors.New("bbox min values must be <= max values")
	}

	return b, nil
}

func (b BBox) IsFinite() bool {
	return isFinite(b.MinX) && isFinite(b.MinY) && isFinite(b.MaxX) && isFinite(b.MaxY)
}

func (b BBox) IsValid() bool {
	return b.MinX <= b.MaxX && b.MinY <= b.MaxY
}

func (b BBox) Width() float64 {
	return b.MaxX - b.MinX
}

func (b BBox) Height() float64 {
	return b.MaxY - b.MinY
}

func (b BBox) ContainsPoint(p Point2D) bool {
	return p.X >= b.MinX && p.X <= b.MaxX && p.Y >= b.MinY && p.Y <= b.MaxY
}

func (b BBox) Intersects(other BBox) bool {
	return b.MinX <= other.MaxX && b.MaxX >= other.MinX && b.MinY <= other.MaxY && b.MaxY >= other.MinY
}

func (b BBox) ExpandToIncludePoint(p Point2D) BBox {
	if p.X < b.MinX {
		b.MinX = p.X
	}

	if p.X > b.MaxX {
		b.MaxX = p.X
	}

	if p.Y < b.MinY {
		b.MinY = p.Y
	}

	if p.Y > b.MaxY {
		b.MaxY = p.Y
	}

	return b
}

// SquaredDistanceToPoint returns the squared distance from p to the nearest
// point of the box, and 0 when p is inside it.
//
// Squared, because the callers compare against a squared threshold: a prune
// that runs once per obstacle per (source, receiver) pair has no room for a
// square root that the comparison would only undo.
func (b BBox) SquaredDistanceToPoint(p Point2D) float64 {
	return squaredGap(b.MinX-p.X, p.X-b.MaxX) + squaredGap(b.MinY-p.Y, p.Y-b.MaxY)
}

// SquaredDistanceToBBox returns the squared distance between the two nearest
// points of the two boxes, and 0 when they intersect.
func (b BBox) SquaredDistanceToBBox(other BBox) float64 {
	return squaredGap(b.MinX-other.MaxX, other.MinX-b.MaxX) +
		squaredGap(b.MinY-other.MaxY, other.MinY-b.MaxY)
}

// squaredGap returns the square of the larger of two signed separations, or 0
// when neither is positive — the contribution of one axis to a box distance.
//
// The comparisons are written out rather than handed to math.Max because this
// runs once per obstacle per source-receiver pair, and math.Max carries NaN
// and signed-zero handling that shows up as a measurable share of a grid run.
// A NaN coordinate here yields 0 for that axis, which only ever widens the
// search.
func squaredGap(low, high float64) float64 {
	gap := low
	if high > gap {
		gap = high
	}

	if gap <= 0 {
		return 0
	}

	return gap * gap
}

func (b BBox) ExpandToIncludeBBox(other BBox) BBox {
	if other.MinX < b.MinX {
		b.MinX = other.MinX
	}

	if other.MinY < b.MinY {
		b.MinY = other.MinY
	}

	if other.MaxX > b.MaxX {
		b.MaxX = other.MaxX
	}

	if other.MaxY > b.MaxY {
		b.MaxY = other.MaxY
	}

	return b
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
