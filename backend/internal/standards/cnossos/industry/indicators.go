package industry

import (
	"github.com/aconiq/backend/internal/acoustics"
	"github.com/aconiq/backend/internal/standards/framework"
)

const (
	// BuiltinModelVersion identifies the current bundled preview coefficient set.
	BuiltinModelVersion = "baseline-preview-industry-v2"

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

// ProvenanceMetadata returns CNOSSOS industry baseline metadata for run provenance.
func ProvenanceMetadata(params map[string]string) map[string]string {
	metadata := map[string]string{
		"model_version":          BuiltinModelVersion,
		"reporting_precision_db": "0.1",
		"indicator_order":        IndicatorLday + "," + IndicatorLevening + "," + IndicatorLnight + "," + IndicatorLden,
		"compliance_boundary":    "baseline-preview-expanded-industry-contract",
		"emission_model":         "category-enclosure-operation-components",
	}

	return framework.StampKeyParameters(metadata, params, []string{
		"grid_resolution_m",
		"grid_padding_m",
		"receiver_height_m",
		"industry_source_category",
		"industry_enclosure_state",
		"industry_sound_power_level_db",
		"industry_source_height_m",
		"industry_tonality_correction_db",
		"industry_impulsivity_correction_db",
		"operation_day_factor",
		"operation_evening_factor",
		"operation_night_factor",
		"air_absorption_db_per_km",
		"ground_attenuation_db",
		"screening_attenuation_db",
		"facade_reflection_db",
		"min_distance_m",
	})
}
