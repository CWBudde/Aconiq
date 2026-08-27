package iso9613

// EffectiveBandLevels returns the octave-band sound power levels L_W of a
// source together with the directivity correction D_c of Eq. 3.
//
// The second result is false when the source carries only a single A-weighted
// sound power level. Such a source has no octave-band spectrum to evaluate, so
// callers must fall back to the 500 Hz estimate of ISO 9613-2:1996 NOTE 1
// (see NoteOneBandIndex and EffectiveAWeightedPowerLevel) instead of
// fabricating a spectrum.
func EffectiveBandLevels(source PointSource) (BandLevels, bool) {
	if source.OctaveBandLevels == nil {
		return BandLevels{}, false
	}

	levels := *source.OctaveBandLevels

	dc := source.DirectivityCorrectionDB
	for i := range levels {
		levels[i] += dc
	}

	return levels, true
}

// EffectiveAWeightedPowerLevel returns L_WA + D_c for a source that carries
// only an A-weighted sound power level. The value is already A-weighted and is
// therefore used without any further weighting in the NOTE 1 estimate.
func EffectiveAWeightedPowerLevel(source PointSource) float64 {
	return source.SoundPowerLevelDB + source.DirectivityCorrectionDB
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
