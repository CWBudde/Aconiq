package road

import (
	"errors"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// PropagationConfig defines parameters for the RLS-19 propagation model.
type PropagationConfig struct {
	// SegmentLengthM is the target sub-segment length for the Teilstueckverfahren.
	// Shorter values give more accurate results at higher computation cost.
	SegmentLengthM float64

	// MinDistanceM is the minimum propagation distance (clamped).
	MinDistanceM float64

	// ReceiverHeightM is the receiver height above ground.
	ReceiverHeightM float64
}

// DefaultPropagationConfig returns baseline propagation parameters.
func DefaultPropagationConfig() PropagationConfig {
	return PropagationConfig{
		SegmentLengthM:  1.0,
		MinDistanceM:    3.0,
		ReceiverHeightM: 4.0,
	}
}

// Validate checks propagation configuration.
func (cfg PropagationConfig) Validate() error {
	if !isFinite(cfg.SegmentLengthM) || cfg.SegmentLengthM <= 0 {
		return errors.New("segment_length_m must be finite and > 0")
	}

	if !isFinite(cfg.MinDistanceM) || cfg.MinDistanceM <= 0 {
		return errors.New("min_distance_m must be finite and > 0")
	}

	if !isFinite(cfg.ReceiverHeightM) || cfg.ReceiverHeightM < 0 {
		return errors.New("receiver_height_m must be finite and >= 0")
	}

	return nil
}

// Segment represents one sub-segment of a source line for the
// Teilstueckverfahren (partial segment method).
type Segment struct {
	MidPoint geo.Point2D // midpoint of this sub-segment
	LengthM  float64     // length of this sub-segment [m]
}

// SplitLineIntoSegments deterministically splits a polyline into
// equal-length sub-segments for the Teilstueckverfahren.
// The splitting is stable: same input always produces the same segments.
func SplitLineIntoSegments(line []geo.Point2D, targetLengthM float64) []Segment {
	if len(line) < 2 || targetLengthM <= 0 {
		return nil
	}

	// Calculate total line length.
	totalLength := polylineLength(line)
	if totalLength <= 0 {
		return nil
	}

	// Determine number of sub-segments (at least 1).
	n := max(int(math.Ceil(totalLength/targetLengthM)), 1)

	segLen := totalLength / float64(n)

	segments := make([]Segment, 0, n)
	for i := range n {
		// Find midpoint of the i-th sub-segment along the polyline.
		startDist := float64(i) * segLen
		endDist := startDist + segLen
		midDist := (startDist + endDist) / 2.0

		midPt := interpolateAlongPolyline(line, midDist)
		segments = append(segments, Segment{
			MidPoint: midPt,
			LengthM:  segLen,
		})
	}

	return segments
}

// polylineLength computes the total length of a polyline.
func polylineLength(line []geo.Point2D) float64 {
	total := 0.0
	for i := 1; i < len(line); i++ {
		total += dist2D(line[i-1], line[i])
	}

	return total
}

// interpolateAlongPolyline finds the point at a given distance along a polyline.
func interpolateAlongPolyline(line []geo.Point2D, distance float64) geo.Point2D {
	if distance <= 0 {
		return line[0]
	}

	cumDist := 0.0

	for i := 1; i < len(line); i++ {
		segLen := dist2D(line[i-1], line[i])
		if cumDist+segLen >= distance {
			// Interpolate within this edge.
			t := (distance - cumDist) / segLen
			if t < 0 {
				t = 0
			}

			if t > 1 {
				t = 1
			}

			return geo.Point2D{
				X: line[i-1].X + t*(line[i].X-line[i-1].X),
				Y: line[i-1].Y + t*(line[i].Y-line[i-1].Y),
			}
		}

		cumDist += segLen
	}

	return line[len(line)-1]
}

func dist2D(a, b geo.Point2D) float64 {
	dx := b.X - a.X
	dy := b.Y - a.Y

	return math.Sqrt(dx*dx + dy*dy)
}

// AttenuationComponents holds the individual attenuation terms for one
// source-receiver path (for diagnostics/reporting).
type AttenuationComponents struct {
	GeometricDivergence  float64 // A_div: geometric spreading [dB]
	AirAbsorption        float64 // A_atm: atmospheric absorption [dB]
	GroundMeteorological float64 // A_ground: ground + meteorological [dB]
	BarrierShielding     float64 // A_bar: barrier insertion loss [dB]
	Total                float64 // total attenuation [dB]
}

// computeAttenuation computes the propagation attenuation from a point
// source to a receiver in the free-field case (no barriers/reflections).
//
// RLS-19 free-field propagation for a point source at distance d:
//
//	A_div   = 20*lg(d) + 11  (geometric divergence for point source)
//	A_atm   = alpha_air * d / 1000  (air absorption)
//	A_ground = ground + meteorological correction
//
// For the Teilstueckverfahren, each sub-segment is treated as a point source
// at its midpoint. The line-source character is recovered by summing the
// energy contributions of all sub-segments with their length weighting.
func computeAttenuation(distanceM float64, cfg PropagationConfig) AttenuationComponents {
	d := distanceM
	if d < cfg.MinDistanceM {
		d = cfg.MinDistanceM
	}

	// Geometric divergence (point source).
	aDiv := 20*math.Log10(d) + 11.0

	// Air absorption.
	aAtm := PropagationConstants.AirAbsorptionCoeff * (d / 1000.0)

	// Ground + meteorological correction.
	// Simplified: use a distance-dependent ground correction that
	// increases with distance (accounts for meteorological favorability).
	aGround := computeGroundCorrection(d)

	total := aDiv + aAtm + aGround

	return AttenuationComponents{
		GeometricDivergence:  aDiv,
		AirAbsorption:        aAtm,
		GroundMeteorological: aGround,
		Total:                total,
	}
}

// computeGroundCorrection returns the combined ground and meteorological
// correction term A_ground for a given distance.
// This is a simplified representation of the RLS-19 ground correction;
// a full implementation would account for ground type, terrain profile,
// source and receiver heights, and meteorological conditions.
func computeGroundCorrection(distanceM float64) float64 {
	if distanceM <= 0 {
		return 0
	}
	// RLS-19 uses a favorable meteorological correction that depends on
	// the source-receiver geometry. As a baseline free-field approximation:
	// A_ground = 4.8 - (2*h_m/d)*(17 + 300/d)
	// where h_m = mean height above ground. For simplicity we use a
	// conservative fixed estimate.
	//
	// This returns a small positive correction (attenuation increases
	// slightly with distance beyond geometric spreading).
	return math.Max(0, 1.5*(1-math.Exp(-distanceM/200)))
}

// ComputeReceiverLevels computes LrDay/LrNight at one receiver from all sources
// using the Teilstueckverfahren (partial segment method).
// Barriers are optional; pass nil for free-field calculation.
func ComputeReceiverLevels(receiver geo.Point2D, sources []RoadSource, barriers []Barrier, cfg PropagationConfig) (PeriodLevels, error) {
	err := cfg.Validate()
	if err != nil {
		return PeriodLevels{}, err
	}

	if !receiver.IsFinite() {
		return PeriodLevels{}, errors.New("receiver is not finite")
	}

	if len(sources) == 0 {
		return PeriodLevels{}, errors.New("at least one source is required")
	}

	// Source height above ground (RLS-19: road surface level, typically 0.5 m).
	const sourceHeightM = 0.5

	dayContrib := make([]float64, 0, len(sources)*4)
	nightContrib := make([]float64, 0, len(sources)*4)

	for _, source := range sources {
		if err := source.Validate(); err != nil {
			return PeriodLevels{}, err
		}

		emission, err := ComputeEmission(source)
		if err != nil {
			return PeriodLevels{}, err
		}

		// Split the source line into sub-segments (Teilstueckverfahren).
		segments := SplitLineIntoSegments(source.Centerline, cfg.SegmentLengthM)
		if len(segments) == 0 {
			continue
		}

		totalLength := polylineLength(source.Centerline)
		if totalLength <= 0 {
			continue
		}

		// For each sub-segment, compute attenuation from midpoint to receiver
		// and weight by segment length relative to total source length.
		for _, seg := range segments {
			d := dist2D(seg.MidPoint, receiver)
			att := computeAttenuation(d, cfg)

			// Check for barrier shielding on this sub-segment path.
			if len(barriers) > 0 {
				shielding := ComputeShielding(
					seg.MidPoint, sourceHeightM,
					receiver, cfg.ReceiverHeightM,
					barriers,
				)

				att.BarrierShielding = shielding.InsertionLoss
				att.Total += shielding.InsertionLoss
			}

			// Length weighting: the emission level applies to the full source.
			// Each sub-segment contributes proportionally to its length.
			lengthWeight := 10 * math.Log10(seg.LengthM/totalLength)

			dayContrib = append(dayContrib, emission.LmEDay+lengthWeight-att.Total)
			nightContrib = append(nightContrib, emission.LmENight+lengthWeight-att.Total)
		}
	}

	return PeriodLevels{
		LrDay:   energySumDB(dayContrib),
		LrNight: energySumDB(nightContrib),
	}, nil
}
