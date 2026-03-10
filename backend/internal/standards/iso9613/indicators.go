package iso9613

const (
	// BuiltinModelVersion identifies the preview point-source baseline contract.
	BuiltinModelVersion = "phase19-preview-v2"

	// ReportingPrecisionDB documents the intended public reporting boundary.
	ReportingPrecisionDB = 0.1
)

// ProvenanceMetadata returns ISO 9613-2 scaffold metadata for run provenance.
func ProvenanceMetadata(params map[string]string) map[string]string {
	metadata := map[string]string{
		"model_version":          BuiltinModelVersion,
		"reporting_precision_db": "0.1",
		"indicator_order":        IndicatorLpAeq,
		"compliance_boundary":    "phase19-iso9613-point-source-preview",
		"implementation_status":  "preview-point-source-run-wired",
		"source_scope":           SourceTypePoint,
		"meteorology_assumption": MeteorologyDownwind,
	}

	for _, key := range []string{
		"grid_resolution_m",
		"grid_padding_m",
		"receiver_height_m",
		"iso9613_source_height_m",
		"iso9613_sound_power_level_db",
		"iso9613_directivity_correction_db",
		"iso9613_tonality_correction_db",
		"iso9613_impulsivity_correction_db",
		"ground_factor",
		"air_temperature_c",
		"relative_humidity_percent",
		"meteorology_assumption",
		"barrier_attenuation_db",
		"min_distance_m",
	} {
		if value, ok := params[key]; ok {
			metadata["key_parameter."+key] = value
		}
	}

	return metadata
}
