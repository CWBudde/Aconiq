package industry

import (
	"math"

	"github.com/aconiq/backend/internal/geo"
)

type periodEmission struct {
	Lday     float64
	Levening float64
	Lnight   float64
}

// ComputeEmission computes period emissions for one industry source.
func ComputeEmission(source IndustrySource) (periodEmission, error) {
	err := source.Validate()
	if err != nil {
		return periodEmission{}, err
	}

	return periodEmission{
		Lday:     emissionForPeriod(source, source.OperationDay),
		Levening: emissionForPeriod(source, source.OperationEvening),
		Lnight:   emissionForPeriod(source, source.OperationNight),
	}, nil
}

func emissionForPeriod(source IndustrySource, operation OperationPeriod) float64 {
	factor := operation.OperatingFactor
	if factor <= 0 {
		return -999.0
	}

	return source.SoundPowerLevelDB +
		operationFactorCorrection(factor) +
		sourceCategoryCorrection(source.SourceCategory, source.SourceType) +
		enclosureCorrection(source.EnclosureState) +
		heightCorrection(source.SourceHeightM, source.SourceType) +
		tonalityCorrection(source.TonalityCorrectionDB) +
		impulsivityCorrection(source.ImpulsivityCorrectionDB) +
		areaEmissionCorrection(source.SourceType, source.AreaPolygon)
}

func operationFactorCorrection(factor float64) float64 {
	return 10 * math.Log10(factor)
}

func sourceCategoryCorrection(category string, sourceType string) float64 {
	switch category {
	case CategoryProcess:
		if sourceType == SourceTypePoint {
			return 1.0
		}

		return 0.5
	case CategoryStack:
		return 1.5
	case CategoryYard:
		if sourceType == SourceTypeArea {
			return 1.2
		}

		return 0.2
	default:
		return 0
	}
}

func enclosureCorrection(state string) float64 {
	switch state {
	case EnclosureOpen:
		return 0
	case EnclosurePartial:
		return -2.0
	case EnclosureEnclosed:
		return -5.0
	default:
		return 0
	}
}

func heightCorrection(heightM float64, sourceType string) float64 {
	if sourceType == SourceTypePoint {
		return math.Min(3.0, 10*math.Log10(1+heightM/10))
	}

	return math.Min(2.0, 10*math.Log10(1+heightM/15))
}

func tonalityCorrection(correctionDB float64) float64 {
	return correctionDB
}

func impulsivityCorrection(correctionDB float64) float64 {
	return correctionDB
}

func areaEmissionCorrection(sourceType string, rings [][]geo.Point2D) float64 {
	if sourceType != SourceTypeArea {
		return 0
	}

	areaM2 := areaPlanArea(rings)
	if areaM2 <= 0 {
		return 0
	}

	return 10 * math.Log10(areaM2)
}

func areaPlanArea(rings [][]geo.Point2D) float64 {
	if len(rings) == 0 {
		return 0
	}

	total := math.Abs(ringArea(rings[0]))
	for _, hole := range rings[1:] {
		total -= math.Abs(ringArea(hole))
	}

	if total < 0 {
		return 0
	}

	return total
}

func ringArea(ring []geo.Point2D) float64 {
	if len(ring) < 3 {
		return 0
	}

	sum := 0.0
	for i := range len(ring) - 1 {
		sum += ring[i].X*ring[i+1].Y - ring[i+1].X*ring[i].Y
	}

	return 0.5 * sum
}

func energySumDB(levels []float64) float64 {
	sum := 0.0

	for _, level := range levels {
		if math.IsNaN(level) || math.IsInf(level, 0) || level <= -900 {
			continue
		}

		sum += math.Pow(10, level/10)
	}

	if sum <= 0 {
		return -999.0
	}

	return 10 * math.Log10(sum)
}
