package road

import (
	"fmt"
	"math"
)

type periodEmission struct {
	Lday     float64
	Levening float64
	Lnight   float64
}

// ComputeEmission computes period emissions for one road source.
//
// This preview implementation uses deterministic piecewise corrections for
// speed, surface, and gradient and is intended as a transparent baseline.
func ComputeEmission(source RoadSource) (periodEmission, error) {
	err := source.Validate()
	if err != nil {
		return periodEmission{}, err
	}

	day := emissionForPeriod(source, source.TrafficDay)
	evening := emissionForPeriod(source, source.TrafficEvening)
	night := emissionForPeriod(source, source.TrafficNight)

	return periodEmission{
		Lday:     day,
		Levening: evening,
		Lnight:   night,
	}, nil
}

func emissionForPeriod(source RoadSource, traffic TrafficPeriod) float64 {
	surfaceCorr := surfaceCorrection(source.SurfaceType)
	speedLightCorr, speedHeavyCorr := speedCorrection(source.SpeedKPH)
	gradientCorr := gradientCorrection(source.GradientPercent)

	lightDB := 36.0 + 10*math.Log10(traffic.LightVehiclesPerHour+1) + speedLightCorr + surfaceCorr
	heavyDB := 42.0 + 10*math.Log10(traffic.HeavyVehiclesPerHour+1) + speedHeavyCorr + surfaceCorr + gradientCorr

	return energySumDB([]float64{lightDB, heavyDB})
}

func speedCorrection(speedKPH float64) (lightCorr float64, heavyCorr float64) {
	clamped := speedKPH
	if clamped < 20 {
		clamped = 20
	}

	if clamped > 130 {
		clamped = 130
	}

	switch {
	case clamped < 40:
		return -2.5, -1.5
	case clamped <= 80:
		base := math.Log10(clamped / 50)
		return 9 * base, 6 * base
	default:
		base := math.Log10(clamped / 80)
		return 2.0 + 7*base, 1.5 + 5*base
	}
}

func surfaceCorrection(surfaceType string) float64 {
	switch surfaceType {
	case SurfaceDenseAsphalt:
		return 0.0
	case SurfacePorousAsphalt:
		return -2.0
	case SurfaceConcrete:
		return 1.0
	case SurfaceCobblestone:
		return 3.0
	default:
		return 0.0
	}
}

func gradientCorrection(gradientPercent float64) float64 {
	switch {
	case gradientPercent > 2:
		return 0.35 * (gradientPercent - 2)
	case gradientPercent < -2:
		return -0.15 * (math.Abs(gradientPercent) - 2)
	default:
		return 0
	}
}

func energySumDB(levels []float64) float64 {
	sum := 0.0

	for _, level := range levels {
		if math.IsNaN(level) || math.IsInf(level, 0) {
			continue
		}

		sum += math.Pow(10, level/10)
	}

	if sum <= 0 {
		return -999.0
	}

	return 10 * math.Log10(sum)
}

func mustFinite(level float64, field string) error {
	if math.IsNaN(level) || math.IsInf(level, 0) {
		return fmt.Errorf("%s is not finite", field)
	}

	return nil
}
