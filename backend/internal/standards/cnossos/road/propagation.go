package road

import (
	"fmt"
	"math"

	"github.com/soundplan/soundplan/backend/internal/geo"
)

// PropagationConfig defines deterministic attenuation terms.
type PropagationConfig struct {
	AirAbsorptionDBPerKM float64
	GroundAttenuationDB  float64
	BarrierAttenuationDB float64
	MinDistanceM         float64
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
		return fmt.Errorf("air_absorption_db_per_km must be finite and >= 0")
	}
	if math.IsNaN(cfg.GroundAttenuationDB) || math.IsInf(cfg.GroundAttenuationDB, 0) || cfg.GroundAttenuationDB < 0 {
		return fmt.Errorf("ground_attenuation_db must be finite and >= 0")
	}
	if math.IsNaN(cfg.BarrierAttenuationDB) || math.IsInf(cfg.BarrierAttenuationDB, 0) || cfg.BarrierAttenuationDB < 0 {
		return fmt.Errorf("barrier_attenuation_db must be finite and >= 0")
	}
	if math.IsNaN(cfg.MinDistanceM) || math.IsInf(cfg.MinDistanceM, 0) || cfg.MinDistanceM <= 0 {
		return fmt.Errorf("min_distance_m must be finite and > 0")
	}
	return nil
}

func attenuation(distanceM float64, cfg PropagationConfig) float64 {
	d := distanceM
	if d < cfg.MinDistanceM {
		d = cfg.MinDistanceM
	}

	// Line-source style geometric attenuation baseline.
	geometric := 10*math.Log10(d) + 8.0
	air := cfg.AirAbsorptionDBPerKM * (d / 1000.0)
	return geometric + air + cfg.GroundAttenuationDB + cfg.BarrierAttenuationDB
}

// ComputeReceiverPeriodLevels computes Lday/Levening/Lnight at one receiver.
func ComputeReceiverPeriodLevels(receiver geo.Point2D, sources []RoadSource, cfg PropagationConfig) (PeriodLevels, error) {
	if err := cfg.Validate(); err != nil {
		return PeriodLevels{}, err
	}
	if !receiver.IsFinite() {
		return PeriodLevels{}, fmt.Errorf("receiver is not finite")
	}
	if len(sources) == 0 {
		return PeriodLevels{}, fmt.Errorf("at least one source is required")
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

		distance := geo.DistancePointToLineString(receiver, source.Centerline)
		a := attenuation(distance, cfg)
		dayContrib = append(dayContrib, emission.Lday-a)
		eveningContrib = append(eveningContrib, emission.Levening-a)
		nightContrib = append(nightContrib, emission.Lnight-a)
	}

	return PeriodLevels{
		Lday:     energySumDB(dayContrib),
		Levening: energySumDB(eveningContrib),
		Lnight:   energySumDB(nightContrib),
	}, nil
}
