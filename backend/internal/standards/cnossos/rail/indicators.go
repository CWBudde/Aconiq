package rail

import (
	"github.com/aconiq/backend/internal/acoustics"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	// BuiltinModelVersion identifies the current bundled preview coefficient set.
	BuiltinModelVersion = "baseline-preview-rail-v2"

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

// ProvenanceMetadata returns CNOSSOS rail baseline metadata for run provenance.
func ProvenanceMetadata(params map[string]string) map[string]string {
	metadata := map[string]string{
		"model_version":          BuiltinModelVersion,
		"reporting_precision_db": "0.1",
		"indicator_order":        IndicatorLday + "," + IndicatorLevening + "," + IndicatorLnight + "," + IndicatorLden,
		"compliance_boundary":    "baseline-preview-expanded-rail-contract",
		"emission_model":         "rolling-traction-braking-infrastructure-components",
	}

	return framework.StampKeyParameters(metadata, params, []string{
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
	})
}
