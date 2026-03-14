package schall03

import "math"

const (
	// BuiltinModelVersion identifies the bundled normative coefficient set for
	// the phase20 Eisenbahn Strecke implementation.
	BuiltinModelVersion = "phase20-normative-eisenbahn-strecke-v1"

	// ReportingPrecisionDB documents the intended reporting boundary for this
	// module. Internal computation remains float64 without intermediate rounding.
	ReportingPrecisionDB = 0.1
)

// NormativeReceiverLevels holds the unrounded L_pAeq and L_r planning period
// levels computed via the normative Gl. 1-2 (emission) + Gl. 8-16 (propagation)
// + Gl. 33-34 (assessment) chain.
type NormativeReceiverLevels struct {
	LpAeqDay   float64 // unrounded L_pAeq,Tag
	LpAeqNight float64 // unrounded L_pAeq,Nacht
	LrDay      float64 // L_r,Tag = LpAeqDay + K_S (K_S = 0 for Eisenbahnen)
	LrNight    float64 // L_r,Nacht = LpAeqNight + K_S
}

// beurteilungspegel computes the Beurteilungspegel per Gl. 33.
//
//	L_r = L_pAeq + K_S
//
// K_S is the Schienenbonus; for Eisenbahnen it is 0 dB since the 2015
// amendment to 16. BImSchV.
func beurteilungspegel(lpAeq, ks float64) float64 {
	return lpAeq + ks
}

// roundToWholeDB rounds a level to the nearest whole dB using round-half-away
// from zero (standard German engineering rounding for Schall 03 outputs).
func roundToWholeDB(l float64) float64 {
	return math.Round(l)
}

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
		"data_pack_version":      BuiltinDataPackVersion,
		"reporting_precision_db": "0.1",
		"reporting_rounding":     "round-half-away-from-zero at report boundary",
		"indicator_order":        IndicatorLrDay + "," + IndicatorLrNight,
		"compliance_boundary":    "baseline-preview-no-normative-tables",
		"band_model":             "octave-63Hz-8000Hz",
	}

	for _, key := range []string{
		"receiver_height_m",
		"rail_train_class",
		"rail_traction_type",
		"rail_track_type",
		"rail_track_form",
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
