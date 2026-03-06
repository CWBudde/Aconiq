package freefield

import (
	"math"

	"github.com/soundplan/soundplan/backend/internal/geo"
)

const (
	// StandardID identifies this demonstrator standard module.
	StandardID = "dummy-freefield"
)

// Source represents one simplified point source for the non-normative model.
type Source struct {
	ID         string
	Point      geo.Point2D
	EmissionDB float64
}

// ComputeReceiverLevelDB computes a non-normative free-field level at a receiver.
//
// Formula used (simplified):
// - contribution_i = Emission_i - 20*log10(distance_i)
// - total = 10*log10(sum(10^(contribution_i/10)))
//
// This is explicitly for technical E2E validation and not a normative method.
func ComputeReceiverLevelDB(receiver geo.Point2D, sources []Source) float64 {
	linearSum := 0.0
	for _, source := range sources {
		d := geo.Distance(receiver, source.Point)
		if d < 1 {
			d = 1
		}
		contributionDB := source.EmissionDB - 20*math.Log10(d)
		linearSum += math.Pow(10, contributionDB/10)
	}
	if linearSum <= 0 {
		return -999.0
	}
	return 10 * math.Log10(linearSum)
}
