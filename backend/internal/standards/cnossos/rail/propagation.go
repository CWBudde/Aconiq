package rail

import (
	"errors"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// PropagationConfig defines attenuation and rail-specific correction terms.
type PropagationConfig struct {
	AirAbsorptionDBPerKM float64
	GroundAttenuationDB  float64
	BridgeCorrectionDB   float64
	CurveSquealDB        float64
	MinDistanceM         float64
}

// DefaultPropagationConfig returns baseline rail propagation terms.
func DefaultPropagationConfig() PropagationConfig {
	return PropagationConfig{
		AirAbsorptionDBPerKM: 0.7,
		GroundAttenuationDB:  1.2,
		BridgeCorrectionDB:   2.0,
		CurveSquealDB:        5.0,
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

	if math.IsNaN(cfg.BridgeCorrectionDB) || math.IsInf(cfg.BridgeCorrectionDB, 0) || cfg.BridgeCorrectionDB < 0 {
		return errors.New("bridge_correction_db must be finite and >= 0")
	}

	if math.IsNaN(cfg.CurveSquealDB) || math.IsInf(cfg.CurveSquealDB, 0) || cfg.CurveSquealDB < 0 {
		return errors.New("curve_squeal_db must be finite and >= 0")
	}

	if math.IsNaN(cfg.MinDistanceM) || math.IsInf(cfg.MinDistanceM, 0) || cfg.MinDistanceM <= 0 {
		return errors.New("min_distance_m must be finite and > 0")
	}

	return nil
}

func attenuation(distanceM float64, cfg PropagationConfig) float64 {
	d := distanceM
	if d < cfg.MinDistanceM {
		d = cfg.MinDistanceM
	}

	geometric := 10*math.Log10(d) + 8.5
	air := cfg.AirAbsorptionDBPerKM * (d / 1000.0)

	return geometric + air + cfg.GroundAttenuationDB
}

func railAdjustment(source RailSource, cfg PropagationConfig) float64 {
	adjustment := 0.0
	if source.OnBridge {
		adjustment += cfg.BridgeCorrectionDB
	}

	if source.CurveRadiusM > 0 && source.CurveRadiusM < 500 {
		severity := (500 - source.CurveRadiusM) / 500
		adjustment += severity * cfg.CurveSquealDB
	}

	return adjustment
}

// ComputeReceiverPeriodLevels computes Lday/Levening/Lnight at one receiver.
func ComputeReceiverPeriodLevels(receiver geo.Point2D, sources []RailSource, cfg PropagationConfig) (PeriodLevels, error) {
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
		if err := source.Validate(); err != nil {
			return PeriodLevels{}, err
		}

		emission, err := ComputeEmission(source)
		if err != nil {
			return PeriodLevels{}, err
		}

		distance := geo.DistancePointToLineString(receiver, source.TrackCenterline)
		baseAttenuation := attenuation(distance, cfg)
		adjustment := railAdjustment(source, cfg)
		dayContrib = append(dayContrib, emission.Lday-baseAttenuation+adjustment)
		eveningContrib = append(eveningContrib, emission.Levening-baseAttenuation+adjustment)
		nightContrib = append(nightContrib, emission.Lnight-baseAttenuation+adjustment)
	}

	return PeriodLevels{
		Lday:     energySumDB(dayContrib),
		Levening: energySumDB(eveningContrib),
		Lnight:   energySumDB(nightContrib),
	}, nil
}
