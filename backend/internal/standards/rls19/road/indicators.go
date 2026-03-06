package road

const (
	// BuiltinDataPackVersion identifies the bundled coefficient/table set.
	BuiltinDataPackVersion = "builtin-rls19-2019-v1"

	// ReportingPrecisionDB documents the public reporting boundary for exported
	// indicators. Internal computation remains float64 without intermediate
	// rounding; user-facing reports should round to 0.1 dB.
	ReportingPrecisionDB = 0.1
)

// PeriodLevels stores receiver levels per RLS-19 time period.
type PeriodLevels struct {
	LrDay   float64 `json:"lr_day"`   // Beurteilungspegel Tag (06-22)
	LrNight float64 `json:"lr_night"` // Beurteilungspegel Nacht (22-06)
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

// ProvenanceMetadata returns RLS-19 specific provenance fields that complement
// the generic standard/version/profile and parameter recording in run
// provenance.
func ProvenanceMetadata(params map[string]string) map[string]string {
	metadata := map[string]string{
		"data_pack_version":      BuiltinDataPackVersion,
		"reporting_precision_db": "0.1",
		"reporting_rounding":     "round-half-away-from-zero at report boundary",
		"indicator_order":        IndicatorLrDay + "," + IndicatorLrNight,
	}

	for _, key := range []string{
		"surface_type",
		"receiver_height_m",
		"segment_length_m",
		"min_distance_m",
		"traffic_day_pkw",
		"traffic_day_lkw1",
		"traffic_day_lkw2",
		"traffic_day_krad",
		"traffic_night_pkw",
		"traffic_night_lkw1",
		"traffic_night_lkw2",
		"traffic_night_krad",
	} {
		value, ok := params[key]
		if ok {
			metadata["key_parameter."+key] = value
		}
	}

	return metadata
}
