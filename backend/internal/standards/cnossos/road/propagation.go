package road

import (
	"errors"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

const maxIntegrationStepM = 10.0

// PropagationConfig defines deterministic attenuation terms.
type PropagationConfig struct {
	AirAbsorptionDBPerKM float64
	GroundAttenuationDB  float64
	BarrierAttenuationDB float64
	MinDistanceM         float64
}

type propagationTerms struct {
	DistanceM   float64
	GeometricDB float64
	AirDB       float64
	GroundDB    float64
	BarrierDB   float64
}

type lineSubsegment struct {
	Midpoint geo.Point2D
	LengthM  float64
}

// DefaultPropagationConfig returns baseline propagation terms.
func DefaultPropagationConfig() PropagationConfig {
	return PropagationConfig{
		AirAbsorptionDBPerKM: 0.7,
		GroundAttenuationDB:  1.5,
		BarrierAttenuationDB: 0.0,
		MinDistanceM:         3.0,
	}
}

func (cfg PropagationConfig) Validate() error {
	if math.IsNaN(cfg.AirAbsorptionDBPerKM) || math.IsInf(cfg.AirAbsorptionDBPerKM, 0) || cfg.AirAbsorptionDBPerKM < 0 {
		return errors.New("air_absorption_db_per_km must be finite and >= 0")
	}

	if math.IsNaN(cfg.GroundAttenuationDB) || math.IsInf(cfg.GroundAttenuationDB, 0) || cfg.GroundAttenuationDB < 0 {
		return errors.New("ground_attenuation_db must be finite and >= 0")
	}

	if math.IsNaN(cfg.BarrierAttenuationDB) || math.IsInf(cfg.BarrierAttenuationDB, 0) || cfg.BarrierAttenuationDB < 0 {
		return errors.New("barrier_attenuation_db must be finite and >= 0")
	}

	if math.IsNaN(cfg.MinDistanceM) || math.IsInf(cfg.MinDistanceM, 0) || cfg.MinDistanceM <= 0 {
		return errors.New("min_distance_m must be finite and > 0")
	}

	return nil
}

func effectivePropagationDistance(distanceM float64, cfg PropagationConfig) float64 {
	d := distanceM
	if d < cfg.MinDistanceM {
		return cfg.MinDistanceM
	}

	return d
}

func geometricDivergence(distanceM float64) float64 {
	return 20*math.Log10(distanceM) + 11.0
}

func airAbsorption(distanceM float64, cfg PropagationConfig) float64 {
	return cfg.AirAbsorptionDBPerKM * (distanceM / 1000.0)
}

func groundEffect(cfg PropagationConfig) float64 {
	return cfg.GroundAttenuationDB
}

func barrierEffect(cfg PropagationConfig) float64 {
	return cfg.BarrierAttenuationDB
}

func attenuationTerms(distanceM float64, cfg PropagationConfig) propagationTerms {
	effectiveDistance := effectivePropagationDistance(distanceM, cfg)

	return propagationTerms{
		DistanceM:   effectiveDistance,
		GeometricDB: geometricDivergence(effectiveDistance),
		AirDB:       airAbsorption(effectiveDistance, cfg),
		GroundDB:    groundEffect(cfg),
		BarrierDB:   barrierEffect(cfg),
	}
}

func totalAttenuation(terms propagationTerms) float64 {
	return terms.GeometricDB + terms.AirDB + terms.GroundDB + terms.BarrierDB
}

func discretizeLineSegment(a geo.Point2D, b geo.Point2D) []lineSubsegment {
	length := geo.Distance(a, b)
	if math.IsNaN(length) || math.IsInf(length, 0) || length <= 0 {
		return nil
	}

	subsegments := max(int(math.Ceil(length/maxIntegrationStepM)), 1)
	stepLength := length / float64(subsegments)
	segments := make([]lineSubsegment, 0, subsegments)

	for j := range subsegments {
		fraction := (float64(j) + 0.5) / float64(subsegments)
		segments = append(segments, lineSubsegment{
			Midpoint: geo.Point2D{
				X: a.X + (b.X-a.X)*fraction,
				Y: a.Y + (b.Y-a.Y)*fraction,
			},
			LengthM: stepLength,
		})
	}

	return segments
}

func subsegmentContribution(emissionDB float64, receiver geo.Point2D, subsegment lineSubsegment, cfg PropagationConfig) float64 {
	distance := geo.Distance(receiver, subsegment.Midpoint)
	terms := attenuationTerms(distance, cfg)

	return emissionDB + 10*math.Log10(subsegment.LengthM) - totalAttenuation(terms)
}

func lineSourceLevelAtReceiver(emissionDB float64, receiver geo.Point2D, centerline []geo.Point2D, cfg PropagationConfig) float64 {
	contribs := make([]float64, 0, len(centerline))

	for i := range len(centerline) - 1 {
		a := centerline[i]
		b := centerline[i+1]

		for _, subsegment := range discretizeLineSegment(a, b) {
			contribs = append(contribs, subsegmentContribution(emissionDB, receiver, subsegment, cfg))
		}
	}

	return energySumDB(contribs)
}

// ComputeReceiverPeriodLevels computes Lday/Levening/Lnight at one receiver.
func ComputeReceiverPeriodLevels(receiver geo.Point2D, sources []RoadSource, cfg PropagationConfig) (PeriodLevels, error) {
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

	dayContrib := make([]float64, 0, len(sources))
	eveningContrib := make([]float64, 0, len(sources))
	nightContrib := make([]float64, 0, len(sources))

	for _, source := range sources {
		err := source.Validate()
		if err != nil {
			return PeriodLevels{}, err
		}

		emission, err := ComputeEmission(source)
		if err != nil {
			return PeriodLevels{}, err
		}

		dayContrib = append(dayContrib, lineSourceLevelAtReceiver(emission.Lday, receiver, source.Centerline, cfg))
		eveningContrib = append(eveningContrib, lineSourceLevelAtReceiver(emission.Levening, receiver, source.Centerline, cfg))
		nightContrib = append(nightContrib, lineSourceLevelAtReceiver(emission.Lnight, receiver, source.Centerline, cfg))
	}

	return PeriodLevels{
		Lday:     energySumDB(dayContrib),
		Levening: energySumDB(eveningContrib),
		Lnight:   energySumDB(nightContrib),
	}, nil
}
