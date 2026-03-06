package road

import "math"

type periodEmission struct {
	LrDay   float64
	LrNight float64
}

// ComputeEmission computes period emissions for one RLS-19 road source.
//
// This preview implementation combines deterministic traffic, speed, surface,
// gradient, and junction-distance terms for transparent regression behavior.
func ComputeEmission(source RoadSource) (periodEmission, error) {
	if err := source.Validate(); err != nil {
		return periodEmission{}, err
	}

	return periodEmission{
		LrDay:   emissionForPeriod(source, source.TrafficDay),
		LrNight: emissionForPeriod(source, source.TrafficNight),
	}, nil
}

func emissionForPeriod(source RoadSource, traffic TrafficPeriod) float64 {
	surfaceCorr := surfaceCorrection(source.SurfaceType)
	gradientCorr := gradientCorrection(source.GradientPercent)
	junctionCorr := junctionCorrection(source.JunctionDistanceM)

	lightDB := 27.7 + 10*math.Log10(traffic.LightVehiclesPerHour+1) + speedLightCorrection(source.SpeedLightKPH) + surfaceCorr + junctionCorr
	heavyDB := 36.5 + 10*math.Log10(traffic.HeavyVehiclesPerHour+1) + speedHeavyCorrection(source.SpeedHeavyKPH) + surfaceCorr + gradientCorr + junctionCorr
	return energySumDB([]float64{lightDB, heavyDB})
}

func speedLightCorrection(speedKPH float64) float64 {
	clamped := speedKPH
	if clamped < 30 {
		clamped = 30
	}
	if clamped > 130 {
		clamped = 130
	}
	return 17 * math.Log10(clamped/100.0)
}

func speedHeavyCorrection(speedKPH float64) float64 {
	clamped := speedKPH
	if clamped < 30 {
		clamped = 30
	}
	if clamped > 100 {
		clamped = 100
	}
	return 23 * math.Log10(clamped/80.0)
}

func surfaceCorrection(surfaceType string) float64 {
	switch surfaceType {
	case SurfaceDenseAsphalt:
		return 0.0
	case SurfaceOpenAsphalt:
		return -2.0
	case SurfaceConcrete:
		return 1.0
	case SurfacePaving:
		return 2.5
	default:
		return 0.0
	}
}

func gradientCorrection(gradientPercent float64) float64 {
	switch {
	case gradientPercent > 2:
		return 0.4 * (gradientPercent - 2)
	case gradientPercent < -2:
		return -0.1 * (math.Abs(gradientPercent) - 2)
	default:
		return 0
	}
}

func junctionCorrection(junctionDistanceM float64) float64 {
	switch {
	case junctionDistanceM <= 25:
		return 3.0
	case junctionDistanceM <= 50:
		return 2.0
	case junctionDistanceM <= 100:
		return 1.0
	default:
		return 0.0
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
