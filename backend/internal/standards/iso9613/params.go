package iso9613

import "github.com/aconiq/backend/internal/standards/framework"

// paramMeteorologyAssumption names the meteorology parameter. It is the one
// parameter name this package needs outside the table below, because
// ProvenanceMetadata stamps the assumption the method is fixed to as well as the
// operator's value for it.
const paramMeteorologyAssumption = "meteorology_assumption"

// parameterDefinitions is the ISO 9613-2 point-source parameter schema, written
// once.
//
// Descriptor publishes these definitions and ProvenanceMetadata stamps every
// name among them as key_parameter.<name>. The two used to be separate lists
// that happened to agree; a parameter added to one and forgotten in the other
// dropped out of provenance without any test noticing, which for a normative
// module means a run whose manifest does not record what produced it.
func parameterDefinitions() []framework.ParameterDefinition {
	minZero := 0.0
	minPositive := 0.001
	maxGroundFactor := 1.0
	maxHumidity := 100.0
	minTemperature := minAirTemperatureC
	maxTemperature := maxAirTemperatureC

	return []framework.ParameterDefinition{
		{Name: "grid_resolution_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "10", Min: &minPositive, Description: "Receiver grid spacing in meters"},
		{Name: "grid_padding_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "20", Min: &minZero, Description: "Padding around source extent in meters"},
		{Name: "receiver_height_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "4", Min: &minZero, Description: "Receiver height in meters"},
		{Name: "iso9613_source_height_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "5", Min: &minZero, Description: "Default source height for imported ISO 9613-2 point sources"},
		{Name: "iso9613_sound_power_level_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "100", Description: "Reference sound power level for imported ISO 9613-2 point sources"},
		{Name: "iso9613_directivity_correction_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "0", Description: "Directivity correction applied at the source"},
		{Name: "iso9613_tonality_correction_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "0", Description: "Tonality correction applied at the reporting boundary"},
		{Name: "iso9613_impulsivity_correction_db", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "0", Description: "Impulsivity correction applied at the reporting boundary"},
		{Name: "ground_factor", Kind: framework.ParameterKindFloat, DefaultValue: "0.5", Min: &minZero, Max: &maxGroundFactor, Description: "Normalized ground factor G for ground no ground-zone feature covers"},
		// Bounded because the ISO 9613-1 absorption coefficient divides by the
		// absolute temperature: unbounded, -273 °C was accepted and produced
		// infinite attenuation rather than a refusal. See minAirTemperatureC.
		{Name: "air_temperature_c", Kind: framework.ParameterKindFloat, Unit: framework.UnitDegreeCelsius, DefaultValue: "10", Min: &minTemperature, Max: &maxTemperature, Description: "Air temperature used for atmospheric absorption inputs"},
		{Name: "relative_humidity_percent", Kind: framework.ParameterKindFloat, Unit: framework.UnitPercent, DefaultValue: "70", Min: &minZero, Max: &maxHumidity, Description: "Relative humidity used for atmospheric absorption inputs"},
		{Name: paramMeteorologyAssumption, Kind: framework.ParameterKindString, DefaultValue: MeteorologyDownwind, Enum: []string{MeteorologyDownwind}, Description: "Favorable propagation assumption for ISO 9613-2 engineering calculations"},
		// c0_met reads as dimensionless but is not: ISO 9613-2 Eq. 22 makes C_met
		// = C0(1 - 10(hs+hr)/dp), and MeteorologicalCorrection feeds that straight
		// into 10^(-0.1*C_met). C0 therefore carries the unit of C_met, decibels.
		{Name: "c0_met", Kind: framework.ParameterKindFloat, Unit: framework.UnitDecibel, DefaultValue: "0", Min: &minZero, Description: "Meteorological correction factor C0 in dB; 0 for pure downwind assessment (TA Lärm default)"},
		{Name: "min_distance_m", Kind: framework.ParameterKindFloat, Unit: framework.UnitMeter, DefaultValue: "1", Min: &minPositive, Description: "Minimum source-receiver distance for stable propagation calculations"},
	}
}

// parameterSchema wraps the definitions for the profile descriptor.
func parameterSchema() framework.ParameterSchema {
	return framework.ParameterSchema{Parameters: parameterDefinitions()}
}

// parameterNames lists the published parameter names in schema order.
func parameterNames() []string {
	definitions := parameterDefinitions()

	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}

	return names
}
