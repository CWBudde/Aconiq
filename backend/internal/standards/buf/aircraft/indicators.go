package aircraft

import (
	"github.com/aconiq/backend/internal/acoustics"
)

const (
	// BuiltinModelVersion identifies the current bundled preview coefficient set.
	BuiltinModelVersion = "baseline-preview-aircraft-mapping-v2"

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

// ProvenanceMetadata returns BUF aircraft baseline metadata for run provenance.
func ProvenanceMetadata(params map[string]string) map[string]string {
	metadata := map[string]string{
		"model_version":          BuiltinModelVersion,
		"reporting_precision_db": "0.1",
		"indicator_order":        IndicatorLday + "," + IndicatorLevening + "," + IndicatorLnight + "," + IndicatorLden,
		"compliance_boundary":    "baseline-preview-expanded-buf-aircraft-contract",
		"emission_model":         "class-operation-procedure-thrust-components",
	}

	for _, key := range []string{
		"grid_resolution_m",
		"grid_padding_m",
		"receiver_height_m",
		"airport_id",
		"runway_id",
		"aircraft_operation_type",
		"aircraft_class",
		"aircraft_procedure_type",
		"aircraft_thrust_mode",
		"reference_power_level_db",
		"engine_state_factor",
		"bank_angle_deg",
		"lateral_offset_m",
		"track_start_height_m",
		"track_end_height_m",
		"movement_day_per_hour",
		"movement_evening_per_hour",
		"movement_night_per_hour",
		"air_absorption_db_per_km",
		"ground_attenuation_db",
		"lateral_directivity_db",
		"approach_correction_db",
		"climb_correction_db",
		"min_slant_distance_m",
	} {
		if value, ok := params[key]; ok {
			metadata["key_parameter."+key] = value
		}
	}

	return metadata
}
