package rail

import "math"

const (
	// BuiltinModelVersion identifies the current bundled preview coefficient set.
	BuiltinModelVersion = "phase11-preview-v2"

	// ReportingPrecisionDB documents the intended reporting boundary for exported
	// indicators. Internal computation remains float64 without intermediate rounding.
	ReportingPrecisionDB = 0.1
)

// PeriodLevels stores receiver levels per time period.
type PeriodLevels struct {
	Lday     float64
	Levening float64
	Lnight   float64
}

// ReceiverIndicators stores exported indicators for one receiver.
type ReceiverIndicators struct {
	Lday     float64
	Levening float64
	Lnight   float64
	Lden     float64
}

// ComputeLden computes the day-evening-night indicator from period levels.
func ComputeLden(levels PeriodLevels) float64 {
	dayLin := 12 * math.Pow(10, levels.Lday/10)
	eveningLin := 4 * math.Pow(10, (levels.Levening+5)/10)
	nightLin := 8 * math.Pow(10, (levels.Lnight+10)/10)

	total := (dayLin + eveningLin + nightLin) / 24.0
	if total <= 0 {
		return -999.0
	}

	return 10 * math.Log10(total)
}

// ToReceiverIndicators builds the final indicator payload.
func (levels PeriodLevels) ToReceiverIndicators() ReceiverIndicators {
	return ReceiverIndicators{
		Lday:     levels.Lday,
		Levening: levels.Levening,
		Lnight:   levels.Lnight,
		Lden:     ComputeLden(levels),
	}
}

// ProvenanceMetadata returns CNOSSOS rail baseline metadata for run provenance.
func ProvenanceMetadata(params map[string]string) map[string]string {
	metadata := map[string]string{
		"model_version":          BuiltinModelVersion,
		"reporting_precision_db": "0.1",
		"indicator_order":        IndicatorLday + "," + IndicatorLevening + "," + IndicatorLnight + "," + IndicatorLden,
		"compliance_boundary":    "baseline-preview-expanded-rail-contract",
		"emission_model":         "rolling-traction-braking-infrastructure-components",
	}

	for _, key := range []string{
		"grid_resolution_m",
		"grid_padding_m",
		"receiver_height_m",
		"rail_traction_type",
		"rail_track_type",
		"rail_track_roughness_class",
		"rail_average_train_speed_kph",
		"rail_braking_share",
		"rail_curve_radius_m",
		"rail_on_bridge",
		"traffic_day_trains_per_hour",
		"traffic_evening_trains_per_hour",
		"traffic_night_trains_per_hour",
		"air_absorption_db_per_km",
		"ground_attenuation_db",
		"bridge_correction_db",
		"curve_squeal_db",
		"min_distance_m",
	} {
		if value, ok := params[key]; ok {
			metadata["key_parameter."+key] = value
		}
	}

	return metadata
}
