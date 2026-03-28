package iso9613

import (
	"errors"
	"fmt"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// PropagationConfig defines the preview attenuation terms used by the Phase 19 baseline.
type PropagationConfig struct {
	GroundFactor            float64
	AirTemperatureC         float64
	RelativeHumidityPercent float64
	MeteorologyAssumption   string
	BarrierAttenuationDB    float64
	MinDistanceM            float64
}

type attenuationTerms struct {
	DistanceM   float64
	GeometricDB float64
	AirDB       float64
	GroundDB    float64
	BarrierDB   float64
}

// DefaultPropagationConfig returns the preview ISO 9613-2 baseline configuration.
func DefaultPropagationConfig() PropagationConfig {
	return PropagationConfig{
		GroundFactor:            0.5,
		AirTemperatureC:         10,
		RelativeHumidityPercent: 70,
		MeteorologyAssumption:   MeteorologyDownwind,
		BarrierAttenuationDB:    0,
		MinDistanceM:            1,
	}
}

// Validate checks propagation inputs for sane ranges.
func (cfg PropagationConfig) Validate() error {
	if math.IsNaN(cfg.GroundFactor) || math.IsInf(cfg.GroundFactor, 0) || cfg.GroundFactor < 0 || cfg.GroundFactor > 1 {
		return errors.New("ground_factor must be finite and within [0,1]")
	}

	if math.IsNaN(cfg.AirTemperatureC) || math.IsInf(cfg.AirTemperatureC, 0) {
		return errors.New("air_temperature_c must be finite")
	}

	if math.IsNaN(cfg.RelativeHumidityPercent) || math.IsInf(cfg.RelativeHumidityPercent, 0) || cfg.RelativeHumidityPercent < 0 || cfg.RelativeHumidityPercent > 100 {
		return errors.New("relative_humidity_percent must be finite and within [0,100]")
	}

	if cfg.MeteorologyAssumption != MeteorologyDownwind {
		return fmt.Errorf("meteorology_assumption must be %q", MeteorologyDownwind)
	}

	if math.IsNaN(cfg.BarrierAttenuationDB) || math.IsInf(cfg.BarrierAttenuationDB, 0) || cfg.BarrierAttenuationDB < 0 {
		return errors.New("barrier_attenuation_db must be finite and >= 0")
	}

	if math.IsNaN(cfg.MinDistanceM) || math.IsInf(cfg.MinDistanceM, 0) || cfg.MinDistanceM <= 0 {
		return errors.New("min_distance_m must be finite and > 0")
	}

	return nil
}

func effectiveDistance(distanceM float64, cfg PropagationConfig) float64 {
	if distanceM < cfg.MinDistanceM {
		return cfg.MinDistanceM
	}

	return distanceM
}

func sourceDistance(receiver geo.PointReceiver, source PointSource, cfg PropagationConfig) float64 {
	horizontal := geo.Distance(receiver.Point, source.Point)
	heightDelta := receiver.HeightM - source.SourceHeightM

	return math.Hypot(horizontal, heightDelta)
}

func geometricDivergence(distanceM float64) float64 {
	return 20*math.Log10(distanceM) + 11
}

func atmosphericAbsorption(distanceM float64, cfg PropagationConfig) float64 {
	base := 0.0015
	tempFactor := 0.00003 * (cfg.AirTemperatureC - 10)
	humidityFactor := -0.00001 * (cfg.RelativeHumidityPercent - 70)
	coefficient := math.Max(0.0002, base+tempFactor+humidityFactor)

	return coefficient * distanceM
}

func groundEffect(distanceM float64, cfg PropagationConfig) float64 {
	if distanceM <= cfg.MinDistanceM {
		return 0
	}

	effect := (0.5 + 1.5*cfg.GroundFactor) * math.Log10(1+distanceM/50.0)
	if effect < 0 {
		return 0
	}

	return math.Min(6, effect)
}

func attenuation(receiver geo.PointReceiver, source PointSource, cfg PropagationConfig) attenuationTerms {
	distance := effectiveDistance(sourceDistance(receiver, source, cfg), cfg)

	return attenuationTerms{
		DistanceM:   distance,
		GeometricDB: geometricDivergence(distance),
		AirDB:       atmosphericAbsorption(distance, cfg),
		GroundDB:    groundEffect(distance, cfg),
		BarrierDB:   cfg.BarrierAttenuationDB,
	}
}

func totalAttenuation(terms attenuationTerms) float64 {
	return terms.GeometricDB + terms.AirDB + terms.GroundDB + terms.BarrierDB
}

// MeteorologicalCorrection computes C_met from Eq. 21–22.
// c0 depends on local meteorological statistics; default 0 for pure downwind.
// hs is source height, hr is receiver height, dp is projected distance.
func MeteorologicalCorrection(c0, hs, hr, dp float64) float64 {
	limit := 10 * (hs + hr)
	if dp <= limit {
		return 0
	}

	return c0 * (1 - limit/dp)
}
