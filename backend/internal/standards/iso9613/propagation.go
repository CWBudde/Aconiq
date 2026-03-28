package iso9613

import (
	"errors"
	"fmt"
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// PropagationConfig defines the attenuation terms for ISO 9613-2 octave-band processing.
type PropagationConfig struct {
	GroundFactor            float64
	AirTemperatureC         float64
	RelativeHumidityPercent float64
	MeteorologyAssumption   string
	Barrier                 *BarrierGeometry
	C0                      float64
	MinDistanceM            float64

	// BarrierAttenuationDB is retained for backward compatibility with the CLI
	// and acceptance tests. It is ignored by the octave-band chain; use the
	// Barrier field instead.
	BarrierAttenuationDB float64
}

// DefaultPropagationConfig returns the default ISO 9613-2 propagation configuration.
func DefaultPropagationConfig() PropagationConfig {
	return PropagationConfig{
		GroundFactor:            0.5,
		AirTemperatureC:         10,
		RelativeHumidityPercent: 70,
		MeteorologyAssumption:   MeteorologyDownwind,
		Barrier:                 nil,
		C0:                      0,
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

	if math.IsNaN(cfg.C0) || math.IsInf(cfg.C0, 0) || cfg.C0 < 0 {
		return errors.New("c0 must be finite and >= 0")
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

func sourceDistance(receiver geo.PointReceiver, source PointSource) float64 {
	horizontal := geo.Distance(receiver.Point, source.Point)
	heightDelta := receiver.HeightM - source.SourceHeightM

	return math.Hypot(horizontal, heightDelta)
}

func geometricDivergence(distanceM float64) float64 {
	return 20*math.Log10(distanceM) + 11
}

// BandAttenuation computes per-octave-band attenuation A(j) for one source-receiver path.
// Returns the 8-band attenuation and the effective source-receiver distance.
func BandAttenuation(receiver geo.PointReceiver, source PointSource, cfg PropagationConfig) (BandLevels, float64) {
	distance := effectiveDistance(sourceDistance(receiver, source), cfg)
	hs := source.SourceHeightM
	hr := receiver.HeightM
	dp := geo.Distance(receiver.Point, source.Point) // projected ground distance

	adiv := geometricDivergence(distance)
	aatm := AtmosphericAbsorptionBands(cfg.AirTemperatureC, cfg.RelativeHumidityPercent, distance)
	agr := GroundEffectBands(cfg.GroundFactor, cfg.GroundFactor, cfg.GroundFactor, hs, hr, dp)
	abar := BarrierAttenuationBands(cfg.Barrier, agr, 20)

	var totalAtten BandLevels
	for i := range NumBands {
		totalAtten[i] = adiv + aatm[i] + agr[i] + abar[i]
	}

	return totalAtten, distance
}

// ComputeDownwindLevel computes L_AT(DW) for one receiver from all sources (Eq. 5).
func ComputeDownwindLevel(receiver geo.PointReceiver, sources []PointSource, cfg PropagationConfig) float64 {
	sum := 0.0

	for _, source := range sources {
		bandLevels := EffectiveBandLevels(source)
		atten, _ := BandAttenuation(receiver, source, cfg)

		for j := range NumBands {
			lft := bandLevels[j] - atten[j]
			sum += math.Pow(10, 0.1*(lft+AWeighting[j]))
		}
	}

	if sum <= 0 {
		return -999
	}

	return 10 * math.Log10(sum)
}

// MeteorologicalCorrection computes C_met from Eq. 21-22.
// c0 depends on local meteorological statistics; default 0 for pure downwind.
// hs is source height, hr is receiver height, dp is projected distance.
func MeteorologicalCorrection(c0, hs, hr, dp float64) float64 {
	limit := 10 * (hs + hr)
	if dp <= limit {
		return 0
	}

	return c0 * (1 - limit/dp)
}
