package schall03

const (
	// BuiltinModelVersion identifies the bundled preview coefficient set used by
	// the current baseline implementation.
	BuiltinModelVersion = "phase18-preview-v1"

	// ReportingPrecisionDB documents the intended reporting boundary for this
	// baseline. Internal computation remains float64 without intermediate rounding.
	ReportingPrecisionDB = 0.1
)

// PeriodLevels stores receiver levels per Schall 03 planning period.
type PeriodLevels struct {
	LrDay   float64 `json:"lr_day"`
	LrNight float64 `json:"lr_night"`
}

// ReceiverIndicators stores exported indicators for one receiver.
type ReceiverIndicators struct {
	LrDay   float64 `json:"lr_day"`
	LrNight float64 `json:"lr_night"`
}

// ToReceiverIndicators builds the final indicator payload from period levels.
func (levels PeriodLevels) ToReceiverIndicators() ReceiverIndicators {
	return ReceiverIndicators(levels)
}

// ProvenanceMetadata returns Schall 03 baseline metadata for run provenance.
func ProvenanceMetadata(params map[string]string) map[string]string {
	metadata := map[string]string{
		"model_version":          BuiltinModelVersion,
		"reporting_precision_db": "0.1",
		"indicator_order":        IndicatorLrDay + "," + IndicatorLrNight,
		"compliance_boundary":    "baseline-preview-no-normative-tables",
		"band_model":             "octave-63Hz-8000Hz",
	}

	for _, key := range []string{
		"receiver_height_m",
		"rail_traction_type",
		"rail_track_type",
		"rail_track_roughness_class",
		"rail_average_train_speed_kph",
		"traffic_day_trains_per_hour",
		"traffic_night_trains_per_hour",
		"air_absorption_db_per_km",
		"ground_attenuation_db",
		"slab_track_correction_db",
		"bridge_correction_db",
		"curve_correction_db",
		"min_distance_m",
	} {
		if value, ok := params[key]; ok {
			metadata["key_parameter."+key] = value
		}
	}

	return metadata
}
