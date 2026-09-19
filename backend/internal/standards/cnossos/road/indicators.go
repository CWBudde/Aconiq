package road

import (
	"github.com/aconiq/backend/internal/acoustics"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	// BuiltinModelVersion identifies the current bundled preview coefficient set.
	BuiltinModelVersion = "baseline-preview-road-v2"

	// ReportingPrecisionDB documents the intended reporting boundary for exported
	// indicators. Internal computation remains float64 without intermediate rounding.
	ReportingPrecisionDB = 0.1
)

// The day/evening/night model is the directive's, not this module's: these are
// aliases of internal/acoustics, so the Lden formula and the payload it fills
// exist once for every module that reports the END set.
type (
	PeriodLevels       = acoustics.PeriodLevels
	ReceiverIndicators = acoustics.ReceiverIndicators
)

// ComputeLden computes the day-evening-night indicator from period levels.
func ComputeLden(levels PeriodLevels) float64 {
	return acoustics.ComputeLden(levels)
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

	return framework.StampKeyParameters(metadata, params, []string{
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
	})
}
