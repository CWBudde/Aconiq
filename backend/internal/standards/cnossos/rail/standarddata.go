package rail

import "github.com/aconiq/backend/internal/standards/framework"

// StandardData returns the preview coefficient sets this scaffold module
// computes with.
//
// The values are this project's own invented preview constants, kept stable so
// results stay reproducible. They are not CNOSSOS-EU coefficients, and no
// coefficient from Directive 2015/996 Annex II exists in this package; see
// docs/conformance/cnossos-umfangserklaerung.md. A digest of invented constants
// is still worth carrying: it answers "could these two runs have produced
// different numbers?", which is a question about the data a module holds and
// not about the authority behind it.
//
// This module declares no coefficient table at all — every constant sits in a
// switch statement or inside a component expression — so the tables below are
// produced by evaluating those functions over their enumerated domain. A digest
// that covered only declared package-level variables would be empty here, and
// would stay empty while a hand-edited switch changed every number.
//
// The flow term is not represented: 10 lg(Q) is a formula over an unbounded
// domain with no constant of its own. Neither are the purely geometric
// propagation terms — the 20 lg(d) + 11 divergence and the line-integration
// step are closed-form code rather than data, and a change to them is a change
// to the code path, which the tool version already identifies.
func StandardData() framework.StandardData {
	return framework.StandardData{Tables: []framework.StandardDataTable{
		{Name: "preview-rail/model-version", Value: BuiltinModelVersion},
		{Name: "preview-rail/component-reference-levels", Value: componentReferenceLevelTable()},
		{Name: "preview-rail/roughness-corrections", Value: roughnessCorrectionTable()},
		{Name: "preview-rail/track-type-corrections", Value: trackTypeCorrectionTable()},
		{Name: "preview-rail/traction-corrections", Value: tractionCorrectionTable()},
		{Name: "preview-rail/bridge-emission-corrections", Value: bridgeEmissionCorrectionTable()},
		{Name: "preview-rail/curve-emission-samples", Value: curveEmissionSampleTable()},
		{Name: "preview-rail/braking-correction-samples", Value: brakingCorrectionSampleTable()},
		{Name: "preview-rail/rolling-speed-correction-samples", Value: speedCorrectionSampleTable()},
		{Name: "preview-rail/traction-speed-correction-samples", Value: tractionSpeedCorrectionSampleTable()},
		{Name: "preview-rail/propagation-curve-samples", Value: propagationCurveSampleTable()},
		{Name: "preview-rail/propagation-defaults", Value: DefaultPropagationConfig()},
	}}
}

// standardDataReferenceSource is the neutral source the component levels are
// evaluated at: standard roughness on ballasted track with electric traction,
// no braking, no curve and no bridge, at 100 km/h.
func standardDataReferenceSource() RailSource {
	return RailSource{
		TractionType:         TractionElectric,
		TrackType:            TrackTypeBallasted,
		TrackRoughnessClass:  RoughnessStandard,
		AverageTrainSpeedKPH: 100,
		BrakingShare:         0,
		CurveRadiusM:         0,
		OnBridge:             false,
	}
}

// standardDataSpeedSamples lists the speeds both speed corrections are sampled
// at. They bracket the shared 30 and 250 km/h clamps, the 80 and 160 km/h
// branch thresholds of the rolling term and the 120 km/h threshold of the
// traction term, and include the 80, 90, 100, 120 and 160 km/h reference speeds
// those branches divide by, so a change to any clamp, threshold or reference
// moves at least one sampled value.
func standardDataSpeedSamples() []float64 {
	return []float64{20, 30, 79, 80, 90, 100, 120, 121, 160, 161, 250, 300}
}

// componentReferenceLevelTable evaluates the four emission components — rolling,
// traction, braking and infrastructure — at the neutral reference source with
// the flow term set to zero, which pins each component base level together with
// the corrections that are not neutral there.
func componentReferenceLevelTable() []float64 {
	source := standardDataReferenceSource()

	return []float64{
		rollingEmission(source, 0),
		tractionEmission(source, 0),
		brakingEmission(source, 0),
		infrastructureEmission(source, 0),
	}
}

// roughnessCorrectionTable evaluates the roughness correction over every
// allowed roughness class.
func roughnessCorrectionTable() []float64 {
	classes := []string{RoughnessSmooth, RoughnessStandard, RoughnessRough}

	values := make([]float64, 0, len(classes))
	for _, class := range classes {
		values = append(values, roughnessCorrection(class))
	}

	return values
}

// trackTypeCorrectionTable evaluates the track-type correction over every
// allowed track type.
func trackTypeCorrectionTable() []float64 {
	trackTypes := []string{TrackTypeBallasted, TrackTypeSlab}

	values := make([]float64, 0, len(trackTypes))
	for _, trackType := range trackTypes {
		values = append(values, trackTypeCorrection(trackType))
	}

	return values
}

// tractionCorrectionTable evaluates the traction correction over every allowed
// traction type.
func tractionCorrectionTable() []float64 {
	tractionTypes := []string{TractionElectric, TractionDiesel, TractionMixed}

	values := make([]float64, 0, len(tractionTypes))
	for _, tractionType := range tractionTypes {
		values = append(values, tractionCorrection(tractionType))
	}

	return values
}

// bridgeEmissionCorrectionTable evaluates the bridge correction over both
// states of its boolean domain.
func bridgeEmissionCorrectionTable() []float64 {
	return []float64{
		bridgeEmissionCorrection(false),
		bridgeEmissionCorrection(true),
	}
}

// curveEmissionSampleTable samples the curve correction. The sample radii
// bracket the 500 m cut-off from both sides and include the zero sentinel that
// means "no curve", so both the taper slope and the cut-off are pinned.
func curveEmissionSampleTable() []float64 {
	radii := []float64{0, 100, 250, 499, 500, 1000}

	values := make([]float64, 0, len(radii))
	for _, radius := range radii {
		values = append(values, curveEmissionCorrection(radius))
	}

	return values
}

// brakingCorrectionSampleTable samples the braking correction across its
// validated [0,1] share domain.
func brakingCorrectionSampleTable() []float64 {
	shares := []float64{0, 0.25, 0.5, 0.75, 1}

	values := make([]float64, 0, len(shares))
	for _, share := range shares {
		values = append(values, brakingCorrection(share))
	}

	return values
}

// speedCorrectionSampleTable samples the rolling-noise speed correction.
func speedCorrectionSampleTable() []float64 {
	speeds := standardDataSpeedSamples()

	values := make([]float64, 0, len(speeds))
	for _, speed := range speeds {
		values = append(values, speedCorrection(speed))
	}

	return values
}

// tractionSpeedCorrectionSampleTable samples the traction-noise speed
// correction.
func tractionSpeedCorrectionSampleTable() []float64 {
	speeds := standardDataSpeedSamples()

	values := make([]float64, 0, len(speeds))
	for _, speed := range speeds {
		values = append(values, tractionSpeedCorrection(speed))
	}

	return values
}

// propagationCurveSampleTable samples the propagation-side curve squeal taper
// at the default configuration. It repeats the 500 m cut-off of the emission
// side with a separate magnitude, so both have to be pinned separately.
func propagationCurveSampleTable() []float64 {
	radii := []float64{0, 100, 250, 499, 500, 1000}
	cfg := DefaultPropagationConfig()

	values := make([]float64, 0, len(radii))
	for _, radius := range radii {
		values = append(values, curveEffect(RailSource{CurveRadiusM: radius}, cfg))
	}

	return values
}
