package industry

import (
	"errors"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// PropagationConfig defines attenuation and industry-specific correction terms.
type PropagationConfig struct {
	AirAbsorptionDBPerKM   float64
	GroundAttenuationDB    float64
	ScreeningAttenuationDB float64
	FacadeReflectionDB     float64
	MinDistanceM           float64
}

type propagationTerms struct {
	DistanceM   float64
	GeometricDB float64
	AirDB       float64
	GroundDB    float64
	ScreeningDB float64
	FacadeDB    float64
	AreaDB      float64
}

// DefaultPropagationConfig returns baseline industry propagation terms.
func DefaultPropagationConfig() PropagationConfig {
	return PropagationConfig{
		AirAbsorptionDBPerKM:   0.7,
		GroundAttenuationDB:    1.0,
		ScreeningAttenuationDB: 0.0,
		FacadeReflectionDB:     0.0,
		MinDistanceM:           3.0,
	}
}

func (cfg PropagationConfig) Validate() error {
	if math.IsNaN(cfg.AirAbsorptionDBPerKM) || math.IsInf(cfg.AirAbsorptionDBPerKM, 0) || cfg.AirAbsorptionDBPerKM < 0 {
		return errors.New("air_absorption_db_per_km must be finite and >= 0")
	}

	if math.IsNaN(cfg.GroundAttenuationDB) || math.IsInf(cfg.GroundAttenuationDB, 0) || cfg.GroundAttenuationDB < 0 {
		return errors.New("ground_attenuation_db must be finite and >= 0")
	}

	if math.IsNaN(cfg.ScreeningAttenuationDB) || math.IsInf(cfg.ScreeningAttenuationDB, 0) || cfg.ScreeningAttenuationDB < 0 {
		return errors.New("screening_attenuation_db must be finite and >= 0")
	}

	if math.IsNaN(cfg.FacadeReflectionDB) || math.IsInf(cfg.FacadeReflectionDB, 0) || cfg.FacadeReflectionDB < 0 {
		return errors.New("facade_reflection_db must be finite and >= 0")
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

func screeningEffect(cfg PropagationConfig) float64 {
	return cfg.ScreeningAttenuationDB
}

func facadeEffect(cfg PropagationConfig) float64 {
	return cfg.FacadeReflectionDB
}

func areaGeometryEffect(receiver geo.PointReceiver, source IndustrySource, cfg PropagationConfig) float64 {
	if source.SourceType != SourceTypeArea {
		return 0
	}

	effectiveRadius := math.Sqrt(areaPlanArea(source.AreaPolygon) / math.Pi)
	if effectiveRadius <= 0 {
		return 0
	}

	distance := math.Max(sourceDistance(receiver, source, cfg), cfg.MinDistanceM)

	return math.Min(6.0, 10*math.Log10(1+effectiveRadius/distance))
}

func attenuationTerms(receiver geo.PointReceiver, source IndustrySource, cfg PropagationConfig) propagationTerms {
	distance := effectivePropagationDistance(sourceDistance(receiver, source, cfg), cfg)

	return propagationTerms{
		DistanceM:   distance,
		GeometricDB: geometricDivergence(distance),
		AirDB:       airAbsorption(distance, cfg),
		GroundDB:    groundEffect(cfg),
		ScreeningDB: screeningEffect(cfg),
		FacadeDB:    facadeEffect(cfg),
		AreaDB:      areaGeometryEffect(receiver, source, cfg),
	}
}

func totalAttenuation(terms propagationTerms) float64 {
	return terms.GeometricDB + terms.AirDB + terms.GroundDB + terms.ScreeningDB - terms.FacadeDB - terms.AreaDB
}

func sourceDistance(receiver geo.PointReceiver, source IndustrySource, cfg PropagationConfig) float64 {
	horizontal := 0.0

	switch source.SourceType {
	case SourceTypePoint:
		horizontal = geo.Distance(receiver.Point, source.Point)
	case SourceTypeArea:
		horizontal = distancePointToPolygon(receiver.Point, source.AreaPolygon)
	}

	heightDelta := receiver.HeightM - source.SourceHeightM

	return math.Hypot(math.Max(horizontal, 0), heightDelta)
}

// ComputeReceiverPeriodLevels computes Lday/Levening/Lnight at one receiver.
func ComputeReceiverPeriodLevels(receiver geo.PointReceiver, sources []IndustrySource, cfg PropagationConfig) (PeriodLevels, error) {
	err := cfg.Validate()
	if err != nil {
		return PeriodLevels{}, err
	}

	if receiver.ID == "" {
		return PeriodLevels{}, errors.New("receiver id is required")
	}

	if !receiver.Point.IsFinite() {
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

		terms := attenuationTerms(receiver, source, cfg)

		dayContrib = append(dayContrib, emission.Lday-totalAttenuation(terms))
		eveningContrib = append(eveningContrib, emission.Levening-totalAttenuation(terms))
		nightContrib = append(nightContrib, emission.Lnight-totalAttenuation(terms))
	}

	return PeriodLevels{
		Lday:     energySumDB(dayContrib),
		Levening: energySumDB(eveningContrib),
		Lnight:   energySumDB(nightContrib),
	}, nil
}

func distancePointToPolygon(point geo.Point2D, rings [][]geo.Point2D) float64 {
	if geo.PointInPolygon(point, rings) {
		return 0
	}

	best := math.MaxFloat64

	for _, ring := range rings {
		d := geo.DistancePointToLineString(point, ring)
		if d < best {
			best = d
		}
	}

	if best == math.MaxFloat64 {
		return math.NaN()
	}

	return best
}
