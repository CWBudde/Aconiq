package terrain

import (
	"math"

	"github.com/aconiq/backend/internal/numeric"
)

// maxPathIntervals bounds the sampling of one path so the cost of a query does
// not grow with the length of the model. A propagation model calls this once
// per source subsegment per receiver, so the bound is what keeps a DTM from
// turning a run into a raster resampling job. 64 intervals resolve the shape of
// the ground far more finely than the mean-height correction that consumes it.
const maxPathIntervals = 64

// MeanRiseAboveChord returns the mean height of the terrain above the straight
// chord joining the terrain at the two ends of a plan-view path.
//
// It is the ground term a propagation model needs for a mean-height correction
// — RLS-19 Eq. 14, DIN ISO 9613-2 h_m — once the ground under each end of the
// path is already known. The mean ground elevation along the path is then
//
//	(groundAtStart + groundAtEnd)/2 + MeanRiseAboveChord(…)
//
// and the mean height of the path above the ground is the mean height of its
// two ends above their own ground, less that rise.
//
// Measuring the DTM against its own two endpoints rather than absolutely is
// what makes it safe to combine with ground elevations that came from
// somewhere else — a road surface elevation declared in the model, say, or a
// receiver ground sampled at one point. A DTM that sits on a different datum,
// or one the model contradicts, then contributes its shape and not its offset.
// Using the absolute samples instead would put the path hundreds of metres
// below ground wherever the two disagree, and invert the correction.
//
// stepM is the nominal sample spacing. The interval count is ceil(d/stepM)
// clamped to [1, maxPathIntervals], and the samples are evenly spaced, so the
// same path always yields the same samples in the same order.
//
// The second return value is false when there is no model, when the path is
// degenerate or not finite, or when either endpoint falls outside the terrain:
// a miss means "no sample here", never an elevation of zero. An interior miss
// contributes no rise, which is the answer terrain following the chord gives.
func MeanRiseAboveChord(model Model, x0, y0, x1, y1, stepM float64) (float64, bool) {
	if model == nil {
		return 0, false
	}

	dx, dy := x1-x0, y1-y0

	distance := math.Hypot(dx, dy)
	if !isFinite(distance) || distance < 1e-9 || !isFinite(stepM) || stepM <= 0 {
		return 0, false
	}

	intervals := min(max(int(math.Ceil(distance/stepM)), 1), maxPathIntervals)

	startZ, ok := model.ElevationAt(x0, y0)
	if !ok {
		return 0, false
	}

	endZ, ok := model.ElevationAt(x1, y1)
	if !ok {
		return 0, false
	}

	// Trapezoidal mean over evenly spaced samples. The two endpoints sit on the
	// chord by construction and so contribute a rise of zero, which collapses
	// the trapezoid rule to the interior samples divided by the interval count.
	var rise numeric.CompensatedSum

	for i := 1; i < intervals; i++ {
		t := float64(i) / float64(intervals)

		z, ok := model.ElevationAt(x0+t*dx, y0+t*dy)
		if !ok {
			continue
		}

		rise.Add(z - (startZ + t*(endZ-startZ)))
	}

	return rise.Sum() / float64(intervals), true
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
