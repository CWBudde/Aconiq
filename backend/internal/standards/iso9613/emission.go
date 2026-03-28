package iso9613

// EffectiveBandLevels returns the octave-band sound power levels for a source.
// If the source provides explicit octave-band levels those are used; otherwise
// the single A-weighted sound power level is distributed across all bands.
func EffectiveBandLevels(source PointSource) BandLevels {
	var levels BandLevels
	if source.OctaveBandLevels != nil {
		levels = *source.OctaveBandLevels
	} else {
		levels = BandLevelsFromAWeighted(source.SoundPowerLevelDB)
	}

	dc := source.DirectivityCorrectionDB
	for i := range levels {
		levels[i] += dc
	}

	return levels
}

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
