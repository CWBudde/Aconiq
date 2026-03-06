package exposure

const (
	// BuiltinModelVersion identifies the current bundled preview aggregation model.
	BuiltinModelVersion = "phase16-preview-v2"

	// ReportingPrecisionCount documents the intended reporting boundary for exported
	// aggregation counts. Internal computation remains float64 without intermediate rounding.
	ReportingPrecisionCount = 0.1
)

// ProvenanceMetadata returns BEB exposure baseline metadata for run provenance.
func ProvenanceMetadata(params map[string]string) map[string]string {
	metadata := map[string]string{
		"model_version":          BuiltinModelVersion,
		"reporting_precision_db": "0.1",
		"compliance_boundary":    "baseline-preview-expanded-beb-exposure-contract",
		"aggregation_model":      "occupancy-facade-threshold-components",
		"indicator_order": IndicatorLden + "," + IndicatorLnight + "," +
			IndicatorEstimatedDwellings + "," + IndicatorEstimatedPersons + "," +
			IndicatorAffectedDwellingsLden + "," + IndicatorAffectedPersonsLden + "," +
			IndicatorAffectedDwellingsLnight + "," + IndicatorAffectedPersonsLnight,
	}

	for _, key := range []string{
		"upstream_mapping_standard",
		"building_usage_type",
		"minimum_building_height_m",
		"floor_height_m",
		"dwellings_per_floor",
		"persons_per_dwelling",
		"threshold_lden_db",
		"threshold_lnight_db",
		"occupancy_mode",
		"facade_evaluation_mode",
		"facade_receiver_height_m",
	} {
		if value, ok := params[key]; ok {
			metadata["key_parameter."+key] = value
		}
	}

	return metadata
}
