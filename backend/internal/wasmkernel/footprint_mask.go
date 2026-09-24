package wasmkernel

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aconiq/backend/internal/geo"
)

// FootprintMaskRequest is the JSON `aconiq.maskFootprints` takes.
//
// Points are flat and interleaved — x0, y0, x1, y1, … — as `transform` takes
// them, because a grid run sends thousands and an object per point is what the
// batch shape exists to avoid. Footprints are polygons in GeoJSON's nesting:
// one entry per polygon, its first ring the exterior and the rest holes, each
// ring a list of [x, y] pairs. A MultiPolygon is passed as its parts. Both are
// in the same metric CRS; the kernel has no way to check that, and does not.
type FootprintMaskRequest struct {
	Points     []float64        `json:"points"`
	Footprints [][][][2]float64 `json:"footprints"`
}

// FootprintMaskResponse lists, ascending, the index of every point that lies
// strictly inside a footprint. An index, not an [x, y]: the caller already
// holds the points, and a sparse list is smaller than a flag per point.
type FootprintMaskResponse struct {
	Masked []int `json:"masked"`
}

// MaskFootprints answers which receivers stand inside a building, through
// geo.MaskPointsInFootprints — the function `aconiq run` calls for the same
// question. The browser reaching it here rather than carrying its own
// point-in-polygon is what keeps the two targets from masking different cells,
// on the facade above all, where the answer is a rule rather than arithmetic.
func MaskFootprints(input []byte) ([]byte, error) {
	var req FootprintMaskRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("invalid input JSON: %w", err)
	}

	if len(req.Points)%2 != 0 {
		return nil, errors.New("maskFootprints: points must be interleaved x, y pairs")
	}

	points := make([]geo.Point2D, len(req.Points)/2)
	for i := range points {
		points[i] = geo.Point2D{X: req.Points[2*i], Y: req.Points[2*i+1]}
	}

	footprints := make([][][]geo.Point2D, len(req.Footprints))
	for i, polygon := range req.Footprints {
		rings := make([][]geo.Point2D, len(polygon))
		for j, ring := range polygon {
			rings[j] = make([]geo.Point2D, len(ring))
			for k, xy := range ring {
				rings[j][k] = geo.Point2D{X: xy[0], Y: xy[1]}
			}
		}

		footprints[i] = rings
	}

	resp := FootprintMaskResponse{Masked: []int{}}

	for i, masked := range geo.MaskPointsInFootprints(points, footprints) {
		if masked {
			resp.Masked = append(resp.Masked, i)
		}
	}

	out, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	return out, nil
}
