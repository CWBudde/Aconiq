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

	// ReceiverHeightM is the receiver height above ground [m].
	ReceiverHeightM float64

	// ReceiverTerrainZ is the absolute terrain elevation at receiver positions.
	// Default 0 (ground at Z = 0 relative to the source datum).
	// Receiver absolute Z = ReceiverTerrainZ + ReceiverHeightM.
	ReceiverTerrainZ float64

	// Terrain describes topographic features (cuts, embankments) near the road.
	// Used for terrain-edge shielding (Böschungskante) and the h_m calculation.
	Terrain []TerrainProfile

	// Reflectors are building facades or other vertical surfaces that reflect
	// sound to the receiver via additional propagation paths (up to 2 bounces).
	Reflectors []Reflector
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

	if !isFinite(cfg.ReceiverTerrainZ) {
		return errors.New("receiver_terrain_z must be finite")
	}

	return nil
}

// Segment represents one sub-segment of a source line for the
// Teilstueckverfahren (partial segment method).
type Segment struct {
	MidPoint geo.Point2D // midpoint of this sub-segment (plan view)
	MidZ     float64     // absolute elevation of the midpoint [m]
	LengthM  float64     // length of this sub-segment [m]
}

// SplitLineIntoSegments deterministically splits a polyline into
// equal-length sub-segments for the Teilstueckverfahren.
// elevations provides per-vertex absolute Z (same length as line); if nil or
// mismatched, MidZ = 0 for all segments (flat road at Z = 0).
// The splitting is stable: same input always produces the same segments.
func SplitLineIntoSegments(line []geo.Point2D, elevations []float64, targetLengthM float64) []Segment {
	if len(line) < 2 || targetLengthM <= 0 {
		return nil
	}

	// Validate elevations.
	haveElevations := len(elevations) == len(line)

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

		midZ := 0.0
		if haveElevations {
			midZ = interpolateZAlongPolyline(line, elevations, midDist)
		}

		segments = append(segments, Segment{
			MidPoint: midPt,
			MidZ:     midZ,
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

// interpolateZAlongPolyline linearly interpolates elevation at a given
// cumulative distance along a polyline with per-vertex elevations.
func interpolateZAlongPolyline(line []geo.Point2D, elevations []float64, distance float64) float64 {
	if distance <= 0 {
		return elevations[0]
	}

	cumDist := 0.0

	for i := 1; i < len(line); i++ {
		segLen := dist2D(line[i-1], line[i])
		if cumDist+segLen >= distance {
			t := (distance - cumDist) / segLen
			if t < 0 {
				t = 0
			}

			if t > 1 {
				t = 1
			}

			return elevations[i-1] + t*(elevations[i]-elevations[i-1])
		}

		cumDist += segLen
	}

	return elevations[len(elevations)-1]
}

func dist2D(a, b geo.Point2D) float64 {
	dx := b.X - a.X
	dy := b.Y - a.Y

	return math.Sqrt(dx*dx + dy*dy)
}

// AttenuationComponents holds the individual attenuation terms for one
// source-receiver path (for diagnostics/reporting).
type AttenuationComponents struct {
	GeometricDivergence  float64 // D_div: geometric spreading [dB]
	AirAbsorption        float64 // D_atm: atmospheric absorption [dB]
	GroundMeteorological float64 // D_gr: ground + meteorological [dB]
	BarrierShielding     float64 // D_z: barrier/terrain-edge insertion loss [dB]
	Total                float64 // total attenuation [dB]
}

// computeAttenuation computes free-field propagation attenuation.
//
// RLS-19 / DIN ISO 9613-2 point-source attenuation:
//
//	D_div = 20·lg(s) + 11      geometric spreading  [3D slant distance s]
//	D_atm = α · s / 1000       air absorption       [3D slant distance s]
//	D_gr  = ground correction  [plan distance s_gr, mean height h_m]
//
// When a barrier or terrain edge shields the path, the caller replaces D_gr
// with D_z and resets Total (RLS-19 rule: D_z replaces, not adds to, D_gr).
func computeAttenuation(planDistM, slantDistM, hm float64, cfg PropagationConfig) AttenuationComponents {
	s := slantDistM
	if s < cfg.MinDistanceM {
		s = cfg.MinDistanceM
	}

	sgr := planDistM
	if sgr < cfg.MinDistanceM {
		sgr = cfg.MinDistanceM
	}

	// Geometric divergence (point source, 3D slant distance).
	aDiv := 20*math.Log10(s) + 11.0

	// Air absorption (3D slant distance).
	aAtm := PropagationConstants.AirAbsorptionCoeff * (s / 1000.0)

	// Ground + meteorological correction.
	aGround := computeGroundCorrection(sgr, hm)

	total := aDiv + aAtm + aGround

	return AttenuationComponents{
		GeometricDivergence:  aDiv,
		AirAbsorption:        aAtm,
		GroundMeteorological: aGround,
		Total:                total,
	}
}

// computeGroundCorrection returns the combined ground and meteorological
// attenuation term D_gr (RLS-19 / DIN ISO 9613-2):
//
//	D_gr = 4.8 − (2·h_m / s_gr) · (17 + 300/s_gr),   minimum 0
//
// h_m is the mean path height above terrain; s_gr is the plan-view distance.
func computeGroundCorrection(planDistM, hm float64) float64 {
	if planDistM <= 0 {
		return 0
	}

	dgr := 4.8 - (2*hm/planDistM)*(17+300/planDistM)
	if dgr < 0 {
		return 0
	}

	return dgr
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

	// Absolute receiver Z = terrain elevation + height above ground.
	receiverZ := cfg.ReceiverTerrainZ + cfg.ReceiverHeightM

	// Source height above road surface (RLS-19: 0.5 m).
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

		// Prepare per-vertex elevations: use CenterlineElevations if provided,
		// otherwise fill from the uniform ElevationM field.
		elevations := source.CenterlineElevations
		if len(elevations) != len(source.Centerline) {
			elevations = make([]float64, len(source.Centerline))
			for i := range elevations {
				elevations[i] = source.ElevationM
			}
		}

		// Split the source line into sub-segments (Teilstueckverfahren).
		segments := SplitLineIntoSegments(source.Centerline, elevations, cfg.SegmentLengthM)
		if len(segments) == 0 {
			continue
		}

		totalLength := polylineLength(source.Centerline)
		if totalLength <= 0 {
			continue
		}

		for _, seg := range segments {
			// Absolute source Z (road surface + 0.5 m source height).
			sourceZ := seg.MidZ + sourceHeightM

			// Plan-view and 3D slant distances.
			planDist := dist2D(seg.MidPoint, receiver)
			dz := receiverZ - sourceZ
			slantDist := math.Sqrt(planDist*planDist + dz*dz)

			// Mean height above terrain for ground correction.
			hm := computeMeanHeight(seg.MidPoint, receiver, sourceZ, receiverZ, cfg.Terrain)

			// Free-field attenuation (D_div + D_atm + D_gr).
			att := computeAttenuation(planDist, slantDist, hm, cfg)

			// Barrier shielding (relative heights, existing approach).
			barrierLoss := 0.0

			if len(barriers) > 0 {
				shielding := ComputeShielding(
					seg.MidPoint, sourceHeightM,
					receiver, cfg.ReceiverHeightM,
					barriers,
				)
				barrierLoss = shielding.InsertionLoss
			}

			// Terrain-edge shielding (absolute Z).
			terrainLoss := 0.0

			if len(cfg.Terrain) > 0 {
				terrainShield := computeTerrainEdgeShielding(
					seg.MidPoint, sourceZ,
					receiver, receiverZ,
					cfg.Terrain,
				)
				terrainLoss = terrainShield.InsertionLoss
			}

			// RLS-19 rule: when shielded, D_z replaces D_gr (not added on top).
			totalShielding := math.Max(barrierLoss, terrainLoss)
			if totalShielding > 0 {
				att.BarrierShielding = totalShielding
				att.Total = att.GeometricDivergence + att.AirAbsorption + totalShielding
			}

			// Length weighting: each sub-segment contributes proportionally.
			lengthWeight := 10 * math.Log10(seg.LengthM/totalLength)

			dayContrib = append(dayContrib, emission.LmEDay+lengthWeight-att.Total)
			nightContrib = append(nightContrib, emission.LmENight+lengthWeight-att.Total)

			// Reflected paths: each adds an independent energy contribution via
			// the image-source method. Ground correction uses mean height along
			// the reflected path (flat terrain approximation for reflected legs).
			appendReflectedContribs(
				&dayContrib, &nightContrib,
				emission, lengthWeight,
				seg.MidPoint, sourceZ, receiver, receiverZ,
				cfg,
			)
		}
	}

	return PeriodLevels{
		LrDay:   energySumDB(dayContrib),
		LrNight: energySumDB(nightContrib),
	}, nil
}

// appendReflectedContribs appends reflected-path energy contributions to the
// day and night contribution slices. Each reflected path is treated as an
// additional point source at the image-source position; ground correction uses
// the mean height of the direct path (flat terrain approximation for the
// reflected legs, which is adequate for first-order use).
func appendReflectedContribs(
	dayContrib, nightContrib *[]float64,
	emission EmissionResult,
	lengthWeight float64,
	source geo.Point2D, sourceZ float64,
	receiver geo.Point2D, receiverZ float64,
	cfg PropagationConfig,
) {
	if len(cfg.Reflectors) == 0 {
		return
	}

	reflPaths := computeReflectedPaths(source, sourceZ, receiver, receiverZ, cfg.Reflectors)

	for _, rp := range reflPaths {
		hmRefl := (sourceZ + receiverZ) / 2.0
		attRefl := computeAttenuation(rp.planDistM, rp.slantDistM, hmRefl, cfg)
		attRefl.Total += rp.lossDB
		*dayContrib = append(*dayContrib, emission.LmEDay+lengthWeight-attRefl.Total)
		*nightContrib = append(*nightContrib, emission.LmENight+lengthWeight-attRefl.Total)
	}
}
