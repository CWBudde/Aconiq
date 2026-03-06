package road

import "math"

const (
	// BuiltinModelVersion identifies the current bundled preview coefficient set.
	BuiltinModelVersion = "phase10-preview-v2"

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

// ProvenanceMetadata returns CNOSSOS road baseline metadata for run provenance.
func ProvenanceMetadata(params map[string]string) map[string]string {
	metadata := map[string]string{
		"model_version":          BuiltinModelVersion,
		"reporting_precision_db": "0.1",
		"indicator_order":        IndicatorLday + "," + IndicatorLevening + "," + IndicatorLnight + "," + IndicatorLden,
		"compliance_boundary":    "baseline-preview-expanded-road-contract",
		"emission_model":         "vehicle-class-components-with-road-context",
	}

	for _, key := range []string{
		"receiver_height_m",
		"road_category",
		"road_surface_type",
		"road_speed_kph",
		"road_gradient_percent",
		"road_junction_type",
		"road_junction_distance_m",
		"road_temperature_c",
		"road_studded_tyre_share",
		"traffic_day_light_vph",
		"traffic_day_medium_vph",
		"traffic_day_heavy_vph",
		"traffic_day_ptw_vph",
		"traffic_evening_light_vph",
		"traffic_evening_medium_vph",
		"traffic_evening_heavy_vph",
		"traffic_evening_ptw_vph",
		"traffic_night_light_vph",
		"traffic_night_medium_vph",
		"traffic_night_heavy_vph",
		"traffic_night_ptw_vph",
		"air_absorption_db_per_km",
		"ground_attenuation_db",
		"barrier_attenuation_db",
		"min_distance_m",
	} {
		if value, ok := params[key]; ok {
			metadata["key_parameter."+key] = value
		}
	}

	return metadata
}
