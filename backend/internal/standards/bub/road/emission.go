package road

import "math"

type periodEmission struct {
	Lday     float64
	Levening float64
	Lnight   float64
}

// ComputeEmission computes period emissions for one BUB road source.
func ComputeEmission(source RoadSource) (periodEmission, error) {
	err := source.Validate()
	if err != nil {
		return periodEmission{}, err
	}

	return periodEmission{
		Lday:     emissionForPeriod(source, source.TrafficDay),
		Levening: emissionForPeriod(source, source.TrafficEvening),
		Lnight:   emissionForPeriod(source, source.TrafficNight),
	}, nil
}

func emissionForPeriod(source RoadSource, traffic TrafficPeriod) float64 {
	surfaceCorr := surfaceCorrection(source.SurfaceType)
	functionCorr := roadFunctionCorrection(source.RoadFunctionClass)
	junctionCorr := junctionCorrection(source.JunctionType, source.JunctionDistanceM)
	temperatureCorr := temperatureCorrection(source.TemperatureC)
	studdedTyreCorr := studdedTyreCorrection(source.StuddedTyreShare)
	gradientCorr := gradientCorrection(source.GradientPercent)

	lightDB := lightVehicleEmission(traffic.LightVehiclesPerHour, source.SpeedKPH, surfaceCorr, functionCorr, junctionCorr, temperatureCorr, studdedTyreCorr)
	mediumDB := mediumVehicleEmission(traffic.MediumVehiclesPerHour, source.SpeedKPH, surfaceCorr, functionCorr, junctionCorr, temperatureCorr, gradientCorr)
	heavyDB := heavyVehicleEmission(traffic.HeavyVehiclesPerHour, source.SpeedKPH, surfaceCorr, functionCorr, junctionCorr, temperatureCorr, gradientCorr)
	ptwDB := poweredTwoWheelerEmission(traffic.PoweredTwoWheelersPerHour, source.SpeedKPH, surfaceCorr, functionCorr, junctionCorr)

	return energySumDB([]float64{lightDB, mediumDB, heavyDB, ptwDB})
}

func lightVehicleEmission(flowPerHour float64, speedKPH float64, surfaceCorr float64, functionCorr float64, junctionCorr float64, temperatureCorr float64, studdedTyreCorr float64) float64 {
	speedCorr := lightSpeedCorrection(speedKPH)
	return 35.0 + 10*math.Log10(flowPerHour+1) + speedCorr + surfaceCorr + functionCorr + junctionCorr + temperatureCorr + studdedTyreCorr
}

func mediumVehicleEmission(flowPerHour float64, speedKPH float64, surfaceCorr float64, functionCorr float64, junctionCorr float64, temperatureCorr float64, gradientCorr float64) float64 {
	speedCorr := mediumSpeedCorrection(speedKPH)
	return 39.0 + 10*math.Log10(flowPerHour+1) + speedCorr + surfaceCorr + functionCorr + junctionCorr + 0.5*temperatureCorr + 0.6*gradientCorr
}

func heavyVehicleEmission(flowPerHour float64, speedKPH float64, surfaceCorr float64, functionCorr float64, junctionCorr float64, temperatureCorr float64, gradientCorr float64) float64 {
	speedCorr := heavySpeedCorrection(speedKPH)
	return 42.5 + 10*math.Log10(flowPerHour+1) + speedCorr + surfaceCorr + functionCorr + junctionCorr + 0.25*temperatureCorr + gradientCorr
}

func poweredTwoWheelerEmission(flowPerHour float64, speedKPH float64, surfaceCorr float64, functionCorr float64, junctionCorr float64) float64 {
	speedCorr := ptwSpeedCorrection(speedKPH)
	return 33.5 + 10*math.Log10(flowPerHour+1) + speedCorr + 0.5*surfaceCorr + functionCorr + 0.5*junctionCorr
}

func lightSpeedCorrection(speedKPH float64) float64 {
	clamped := speedKPH
	if clamped < 20 {
		clamped = 20
	}

	if clamped > 130 {
		clamped = 130
	}

	switch {
	case clamped < 40:
		return -2.0
	case clamped <= 80:
		base := math.Log10(clamped / 50)
		return 8 * base
	default:
		base := math.Log10(clamped / 80)
		return 1.5 + 6*base
	}
}

func mediumSpeedCorrection(speedKPH float64) float64 {
	clamped := math.Min(math.Max(speedKPH, 20), 110)
	if clamped <= 60 {
		return 5 * math.Log10(clamped/50)
	}

	return 1.0 + 4.5*math.Log10(clamped/60)
}

func heavySpeedCorrection(speedKPH float64) float64 {
	clamped := math.Min(math.Max(speedKPH, 20), 100)
	if clamped <= 60 {
		return 4.5 * math.Log10(clamped/45)
	}

	return 1.2 + 4.0*math.Log10(clamped/60)
}

func ptwSpeedCorrection(speedKPH float64) float64 {
	clamped := math.Min(math.Max(speedKPH, 20), 100)
	if clamped <= 50 {
		return 3.0 * math.Log10(clamped/40)
	}

	return 0.8 + 4.0*math.Log10(clamped/50)
}

func surfaceCorrection(surfaceType string) float64 {
	switch surfaceType {
	case SurfaceDenseAsphalt:
		return 0.0
	case SurfacePorousAsphalt:
		return -1.5
	case SurfaceConcrete:
		return 1.0
	case SurfaceCobblestone:
		return 3.0
	default:
		return 0.0
	}
}

func roadFunctionCorrection(functionClass string) float64 {
	switch functionClass {
	case FunctionUrbanMain:
		return 1.5
	case FunctionUrbanLocal:
		return -0.5
	case FunctionRuralMain:
		return 0.5
	default:
		return 0
	}
}

func junctionCorrection(junctionType string, distanceM float64) float64 {
	if junctionType == JunctionNone {
		return 0
	}

	influence := math.Max(0, 1-distanceM/150.0)

	switch junctionType {
	case JunctionTrafficLight:
		return 2.0 * influence
	case JunctionRoundabout:
		return 1.0 * influence
	default:
		return 0
	}
}

func temperatureCorrection(temperatureC float64) float64 {
	return (20.0 - temperatureC) * 0.03
}

func studdedTyreCorrection(share float64) float64 {
	return 2.5 * share
}

func gradientCorrection(gradientPercent float64) float64 {
	switch {
	case gradientPercent > 2:
		return 0.3 * (gradientPercent - 2)
	case gradientPercent < -2:
		return -0.1 * (math.Abs(gradientPercent) - 2)
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
