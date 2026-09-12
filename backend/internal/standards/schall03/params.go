package schall03

import "github.com/aconiq/backend/internal/standards/framework"

// param is one published Schall 03 run parameter, written once.
//
// A parameter name used to appear three times in this package: in the schema
// Descriptor publishes, in the bounds table PropagationConfig.Validate walks, and
// in the provenance key list ProvenanceMetadata stamps. Nothing tied the three
// together, so a typo in any one of them failed silently — a renamed schema
// parameter would still validate against the old spelling and still be stamped
// under it, and for a normative module that means a manifest that misdescribes
// the run that produced the levels. Every site now derives from params().
type param struct {
	// definition is what Descriptor publishes.
	definition framework.ParameterDefinition

	// provenance marks a parameter ProvenanceMetadata stamps as
	// key_parameter.<name>. The grid geometry and the two source placeholders
	// the model itself resolves are deliberately not stamped.
	provenance bool

	// configValue reads the propagation term this parameter fills; it is non-nil
	// exactly for the six PropagationConfig terms, and configMin is the floor
	// Validate enforces on that term. The floor is not always the schema Min
	// above: min_distance_m publishes a user-facing bound of 0.001 m but the
	// config only has to stay clear of a division by zero.
	configValue func(PropagationConfig) float64
	configMin   float64
}

// params is the Schall 03 parameter table: the single place each parameter name
// is spelled.
func params() []param {
	minZero := 0.0
	minPositive := 0.001

	return []param{
		{definition: framework.ParameterDefinition{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, DefaultValue: "10", Min: &minPositive, Description: "Receiver grid spacing in meters for the future Schall 03 run/export path"}},
		{definition: framework.ParameterDefinition{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, DefaultValue: "30", Min: &minZero, Description: "Padding around source extent in meters"}},
		{definition: framework.ParameterDefinition{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"}, provenance: true},
		{definition: framework.ParameterDefinition{Name: ParamEngine, Kind: framework.ParameterKindString, DefaultValue: EngineAuto, Enum: []string{EngineAuto, EngineNormative, EnginePreview}, Description: "Computation chain: auto runs the normative Anlage-2 chain when the model carries schall03_operations and fails otherwise; preview opts into the placeholder data pack"}, provenance: true},
		{definition: framework.ParameterDefinition{Name: "rail_train_class", Kind: framework.ParameterKindString, DefaultValue: TrainClassMixed, Enum: []string{TrainClassPassenger, TrainClassFreight, TrainClassMixed}, Description: "Default train class placeholder for Schall 03 source mapping"}, provenance: true},
		{definition: framework.ParameterDefinition{Name: "rail_traction_type", Kind: framework.ParameterKindString, DefaultValue: TractionElectric, Enum: []string{TractionElectric, TractionDiesel, TractionMixed}, Description: "Default traction type for imported Schall 03 rail sources"}, provenance: true},
		{definition: framework.ParameterDefinition{Name: "rail_track_type", Kind: framework.ParameterKindString, DefaultValue: TrackTypeBallasted, Enum: []string{TrackTypeBallasted, TrackTypeSlab}, Description: "Default track construction type for imported rail sources"}, provenance: true},
		{definition: framework.ParameterDefinition{Name: "rail_track_form", Kind: framework.ParameterKindString, DefaultValue: TrackFormMainline, Enum: []string{TrackFormMainline, TrackFormStation, TrackFormSwitches}, Description: "Default track-form placeholder for future Schall 03 source mapping"}, provenance: true},
		{definition: framework.ParameterDefinition{Name: "rail_track_roughness_class", Kind: framework.ParameterKindString, DefaultValue: RoughnessStandard, Enum: []string{RoughnessStandard, RoughnessLowNoise, RoughnessRough}, Description: "Default roughness class for imported rail sources"}, provenance: true},
		{definition: framework.ParameterDefinition{Name: "rail_average_train_speed_kph", Kind: framework.ParameterKindFloat, DefaultValue: "100", Min: &minPositive, Description: "Default train speed for imported rail sources"}, provenance: true},
		{definition: framework.ParameterDefinition{Name: "rail_curve_radius_m", Kind: framework.ParameterKindFloat, DefaultValue: "500", Min: &minZero, Description: "Default curve radius for imported rail sources"}},
		{definition: framework.ParameterDefinition{Name: "rail_on_bridge", Kind: framework.ParameterKindBool, DefaultValue: "false", Description: "Default bridge flag for imported rail sources"}},
		{definition: framework.ParameterDefinition{Name: "traffic_day_trains_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "8", Min: &minZero, Description: "Default day trains per hour for imported rail sources"}, provenance: true},
		{definition: framework.ParameterDefinition{Name: "traffic_night_trains_per_hour", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Default night trains per hour for imported rail sources"}, provenance: true},
		{
			definition:  framework.ParameterDefinition{Name: "air_absorption_db_per_km", Kind: framework.ParameterKindFloat, DefaultValue: "0.7", Min: &minZero, Description: "Baseline air absorption term"},
			provenance:  true,
			configValue: func(cfg PropagationConfig) float64 { return cfg.AirAbsorptionDBPerKM },
			configMin:   0,
		},
		{
			definition:  framework.ParameterDefinition{Name: "ground_attenuation_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.2", Min: &minZero, Description: "Baseline ground attenuation term"},
			provenance:  true,
			configValue: func(cfg PropagationConfig) float64 { return cfg.GroundAttenuationDB },
			configMin:   0,
		},
		{
			definition:  framework.ParameterDefinition{Name: "slab_track_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "1.5", Min: &minZero, Description: "Additional correction for slab track sections"},
			provenance:  true,
			configValue: func(cfg PropagationConfig) float64 { return cfg.SlabTrackCorrectionDB },
			configMin:   0,
		},
		{
			definition:  framework.ParameterDefinition{Name: "bridge_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "2", Min: &minZero, Description: "Additional correction for bridge sections"},
			provenance:  true,
			configValue: func(cfg PropagationConfig) float64 { return cfg.BridgeCorrectionDB },
			configMin:   0,
		},
		{
			definition:  framework.ParameterDefinition{Name: "curve_correction_db", Kind: framework.ParameterKindFloat, DefaultValue: "4", Min: &minZero, Description: "Maximum correction for tight-curve sections"},
			provenance:  true,
			configValue: func(cfg PropagationConfig) float64 { return cfg.CurveCorrectionDB },
			configMin:   0,
		},
		{
			definition:  framework.ParameterDefinition{Name: "min_distance_m", Kind: framework.ParameterKindFloat, DefaultValue: "3", Min: &minPositive, Description: "Minimum source-receiver distance for the future propagation chain"},
			provenance:  true,
			configValue: func(cfg PropagationConfig) float64 { return cfg.MinDistanceM },
			configMin:   0.0000001,
		},
	}
}

// parameterSchema wraps the definitions for the profile descriptor.
func parameterSchema() framework.ParameterSchema {
	all := params()

	definitions := make([]framework.ParameterDefinition, 0, len(all))
	for _, item := range all {
		definitions = append(definitions, item.definition)
	}

	return framework.ParameterSchema{Parameters: definitions}
}

// provenanceParameterNames lists the parameters ProvenanceMetadata stamps, in
// schema order.
func provenanceParameterNames() []string {
	all := params()

	names := make([]string, 0, len(all))

	for _, item := range all {
		if item.provenance {
			names = append(names, item.definition.Name)
		}
	}

	return names
}

// propagationParams lists the parameters that fill a PropagationConfig term, in
// schema order. That order is behaviour: Validate reports the first term that
// fails, so a config with two bad terms names the earlier one.
func propagationParams() []param {
	all := params()

	terms := make([]param, 0, len(all))

	for _, item := range all {
		if item.configValue != nil {
			terms = append(terms, item)
		}
	}

	return terms
}
