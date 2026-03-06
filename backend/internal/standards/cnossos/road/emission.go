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

type vehicleClass string

const (
	vehicleClassLight              vehicleClass = "light"
	vehicleClassMedium             vehicleClass = "medium"
	vehicleClassHeavy              vehicleClass = "heavy"
	vehicleClassPoweredTwoWheelers vehicleClass = "powered_two_wheelers"
)

// ComputeEmission computes period emissions for one road source.
//
// This preview implementation uses deterministic, named emission components so
// the road baseline reads more like a method implementation than a compact
// heuristic.
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
	return energySumDB([]float64{
		emissionForVehicleClass(source, traffic.LightVehiclesPerHour, vehicleClassLight),
		emissionForVehicleClass(source, traffic.MediumVehiclesPerHour, vehicleClassMedium),
		emissionForVehicleClass(source, traffic.HeavyVehiclesPerHour, vehicleClassHeavy),
		emissionForVehicleClass(source, traffic.PoweredTwoWheelersPerHour, vehicleClassPoweredTwoWheelers),
	})
}

func emissionForVehicleClass(source RoadSource, vehiclesPerHour float64, class vehicleClass) float64 {
	if vehiclesPerHour <= 0 {
		return -999.0
	}

	return baseEmissionLevel(class) +
		trafficFlowCorrection(vehiclesPerHour) +
		roadCategoryCorrection(source.RoadCategory, class) +
		speedCorrection(source.SpeedKPH, class) +
		surfaceCorrection(source.SurfaceType, class) +
		gradientCorrection(source.GradientPercent, class) +
		junctionCorrection(source.JunctionType, source.JunctionDistanceM, class) +
		temperatureCorrection(source.TemperatureC, class) +
		studdedTyreCorrection(source.StuddedTyreShare, class)
}

func baseEmissionLevel(class vehicleClass) float64 {
	switch class {
	case vehicleClassLight:
		return 35.0
	case vehicleClassMedium:
		return 39.0
	case vehicleClassHeavy:
		return 42.0
	case vehicleClassPoweredTwoWheelers:
		return 37.0
	default:
		return 35.0
	}
}

func trafficFlowCorrection(vehiclesPerHour float64) float64 {
	return 10 * math.Log10(vehiclesPerHour+1)
}

func roadCategoryCorrection(category string, class vehicleClass) float64 {
	switch category {
	case CategoryUrbanMotorway:
		switch class {
		case vehicleClassLight:
			return 0.6
		case vehicleClassMedium, vehicleClassHeavy:
			return 0.8
		default:
			return 0.3
		}
	case CategoryUrbanMajor:
		return 0.2
	case CategoryUrbanLocal:
		switch class {
		case vehicleClassHeavy:
			return -0.4
		case vehicleClassPoweredTwoWheelers:
			return 0.2
		default:
			return -0.2
		}
	case CategoryRuralMotorway:
		switch class {
		case vehicleClassLight:
			return 0.4
		case vehicleClassMedium, vehicleClassHeavy:
			return 0.7
		default:
			return 0
		}
	case CategoryRuralMajor:
		switch class {
		case vehicleClassHeavy:
			return 0.3
		default:
			return 0
		}
	default:
		return 0
	}
}

func speedCorrection(speedKPH float64, class vehicleClass) float64 {
	clamped := speedKPH
	if clamped < 20 {
		clamped = 20
	}

	if clamped > 130 {
		clamped = 130
	}

	switch {
	case clamped < 40:
		switch class {
		case vehicleClassLight:
			return -2.5
		case vehicleClassMedium:
			return -2.0
		case vehicleClassHeavy:
			return -1.5
		case vehicleClassPoweredTwoWheelers:
			return -1.0
		default:
			return 0
		}
	case clamped <= 80:
		base := math.Log10(clamped / 50)

		switch class {
		case vehicleClassLight:
			return 9 * base
		case vehicleClassMedium:
			return 7.5 * base
		case vehicleClassHeavy:
			return 6 * base
		case vehicleClassPoweredTwoWheelers:
			return 10 * base
		default:
			return 0
		}
	default:
		base := math.Log10(clamped / 80)

		switch class {
		case vehicleClassLight:
			return 2.0 + 7*base
		case vehicleClassMedium:
			return 1.7 + 6*base
		case vehicleClassHeavy:
			return 1.5 + 5*base
		case vehicleClassPoweredTwoWheelers:
			return 2.5 + 7.5*base
		default:
			return 0
		}
	}
}

func surfaceCorrection(surfaceType string, class vehicleClass) float64 {
	switch surfaceType {
	case SurfaceDenseAsphalt:
		return 0.0
	case SurfacePorousAsphalt:
		switch class {
		case vehicleClassHeavy:
			return -1.0
		default:
			return -2.0
		}
	case SurfaceConcrete:
		switch class {
		case vehicleClassPoweredTwoWheelers:
			return 0.5
		default:
			return 1.0
		}
	case SurfaceCobblestone:
		switch class {
		case vehicleClassHeavy:
			return 2.0
		default:
			return 3.0
		}
	default:
		return 0.0
	}
}

func gradientCorrection(gradientPercent float64, class vehicleClass) float64 {
	switch {
	case gradientPercent > 2:
		switch class {
		case vehicleClassLight:
			return 0.15 * (gradientPercent - 2)
		case vehicleClassMedium:
			return 0.25 * (gradientPercent - 2)
		case vehicleClassHeavy:
			return 0.35 * (gradientPercent - 2)
		case vehicleClassPoweredTwoWheelers:
			return 0.1 * (gradientPercent - 2)
		default:
			return 0
		}
	case gradientPercent < -2:
		switch class {
		case vehicleClassLight:
			return -0.08 * (math.Abs(gradientPercent) - 2)
		case vehicleClassMedium:
			return -0.12 * (math.Abs(gradientPercent) - 2)
		case vehicleClassHeavy:
			return -0.15 * (math.Abs(gradientPercent) - 2)
		case vehicleClassPoweredTwoWheelers:
			return -0.05 * (math.Abs(gradientPercent) - 2)
		default:
			return 0
		}
	default:
		return 0
	}
}

func junctionCorrection(junctionType string, distanceM float64, class vehicleClass) float64 {
	if junctionType == JunctionNone {
		return 0
	}

	influence := 1.0

	switch {
	case distanceM >= 100:
		influence = 0
	case distanceM > 0:
		influence = 1 - distanceM/100
	}

	if influence <= 0 {
		return 0
	}

	base := 0.0

	switch junctionType {
	case JunctionTrafficLight:
		switch class {
		case vehicleClassLight:
			base = 1.2
		case vehicleClassMedium:
			base = 1.5
		case vehicleClassHeavy:
			base = 1.8
		case vehicleClassPoweredTwoWheelers:
			base = 0.9
		}
	case JunctionRoundabout:
		switch class {
		case vehicleClassLight:
			base = 0.7
		case vehicleClassMedium:
			base = 0.9
		case vehicleClassHeavy:
			base = 1.1
		case vehicleClassPoweredTwoWheelers:
			base = 0.5
		}
	}

	return base * influence
}

func temperatureCorrection(temperatureC float64, class vehicleClass) float64 {
	delta := temperatureC - 20.0

	switch class {
	case vehicleClassLight, vehicleClassMedium:
		return -0.03 * delta
	case vehicleClassHeavy:
		return -0.02 * delta
	case vehicleClassPoweredTwoWheelers:
		return -0.01 * delta
	default:
		return 0
	}
}

func studdedTyreCorrection(share float64, class vehicleClass) float64 {
	switch class {
	case vehicleClassLight:
		return 4.0 * share
	case vehicleClassMedium:
		return 2.0 * share
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
