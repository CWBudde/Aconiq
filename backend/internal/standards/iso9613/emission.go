package iso9613

import "math"

// ComputeEmission returns the A-weighted source emission level for one point source.
func ComputeEmission(source PointSource) (float64, error) {
	err := source.Validate()
	if err != nil {
		return 0, err
	}

	return source.SoundPowerLevelDB +
		source.DirectivityCorrectionDB +
		source.TonalityCorrectionDB +
		source.ImpulsivityCorrectionDB, nil
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
