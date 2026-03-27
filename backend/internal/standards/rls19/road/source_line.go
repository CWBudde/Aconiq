package road

import (
	"math"

	"github.com/aconiq/backend/internal/geo"
)

const automaticSourceLineLaneWidthM = 3.5

// SourceLineOffsetM returns the automatic lateral offset implied by LaneCount.
// The offset is applied to the right of travel relative to the source line
// direction and assumes a 3.5 m lane width when LaneCount > 0.
func (s RoadSource) SourceLineOffsetM() float64 {
	switch {
	case s.LaneCount <= 1:
		return 0
	case s.LaneCount == 2:
		return 0.5 * automaticSourceLineLaneWidthM
	case s.LaneCount <= 4:
		return 0.5 * float64(s.LaneCount-2) * automaticSourceLineLaneWidthM
	default:
		return 0.5 * float64(s.LaneCount-3) * automaticSourceLineLaneWidthM
	}
}

// EffectiveCenterline returns the source line used for geometry-dependent
// calculations. When LaneCount is unset, the original centerline is used.
func (s RoadSource) EffectiveCenterline() []geo.Point2D {
	offsetM := s.SourceLineOffsetM()
	if offsetM == 0 || len(s.Centerline) < 2 {
		return s.Centerline
	}

	return offsetPolylineRight(s.Centerline, offsetM)
}

func offsetPolylineRight(line []geo.Point2D, offsetM float64) []geo.Point2D {
	if len(line) < 2 || offsetM == 0 {
		return line
	}

	offset := make([]geo.Point2D, len(line))
	for i, point := range line {
		nx, ny := polylineVertexRightNormal(line, i)
		offset[i] = geo.Point2D{X: point.X + offsetM*nx, Y: point.Y + offsetM*ny}
	}

	return offset
}

func polylineVertexRightNormal(line []geo.Point2D, index int) (float64, float64) {
	prevX, prevY, prevOK := polylineSegmentRightNormal(line, index-1)
	nextX, nextY, nextOK := polylineSegmentRightNormal(line, index)

	switch {
	case prevOK && nextOK:
		nx := prevX + nextX
		ny := prevY + nextY

		norm := math.Hypot(nx, ny)
		if norm > 0 {
			return nx / norm, ny / norm
		}

		return nextX, nextY
	case prevOK:
		return prevX, prevY
	case nextOK:
		return nextX, nextY
	default:
		return 0, 0
	}
}

func polylineSegmentRightNormal(line []geo.Point2D, segmentIndex int) (float64, float64, bool) {
	if segmentIndex < 0 || segmentIndex >= len(line)-1 {
		return 0, 0, false
	}

	dx := line[segmentIndex+1].X - line[segmentIndex].X
	dy := line[segmentIndex+1].Y - line[segmentIndex].Y

	length := math.Hypot(dx, dy)
	if length == 0 {
		return 0, 0, false
	}

	return dy / length, -dx / length, true
}
