package schall03

import "math"

// ParamEngine is the run parameter selecting the computation chain.
const ParamEngine = "schall03_engine"

// Engine names select which computation chain a Schall 03 run uses.  They are
// distinct chains, not variants of one: only EngineNormative reaches the
// Anlage-2 tables, and only EnginePreview reads BuiltinDataPack.
const (
	// EngineAuto runs the normative chain when the model carries normative
	// track data, and fails otherwise rather than silently degrading.
	EngineAuto = "auto"
	// EngineNormative runs Gl. 1-2 (emission, Beiblatt 1/2) + Gl. 8-16
	// (propagation) + Gl. 33-34 (assessment).
	EngineNormative = "normative"
	// EnginePreview runs the placeholder data-pack chain.  Its coefficients are
	// invented and its output must not be presented as a Schall 03 result.
	EnginePreview = "preview"
)

const (
	// NormativeModelVersion identifies the Anlage-2 Strecke implementation:
	// Beiblatt 1/2 emission spectra, Tabelle 7/8/15/18 corrections, and the
	// Gl. 8-16 propagation chain.
	NormativeModelVersion = "anlage2-2014-strecke-v1"

	// PreviewModelVersion identifies the placeholder data-pack chain.  It is
	// deliberately named for what it is; nothing about it is normative.
	PreviewModelVersion = "baseline-preview-datapack-v1"

	// ComplianceBoundaryNormative marks output produced from the Anlage-2
	// tables.  See docs/conformance/schall03-konformitaetserklaerung.md for the
	// scope this covers and the deviations it does not.
	ComplianceBoundaryNormative = "anlage2-2014-strecke-eisenbahn-strassenbahn"

	// ComplianceBoundaryPreview marks output produced from invented spectra.
	ComplianceBoundaryPreview = "baseline-preview-no-normative-tables"

	// ReportingPrecisionDB documents the intended reporting boundary for this
	// module. Internal computation remains float64 without intermediate rounding.
	ReportingPrecisionDB = 0.1
)

// ModelVersionForEngine returns the model version stamped for one engine.
func ModelVersionForEngine(engine string) string {
	if engine == EngineNormative {
		return NormativeModelVersion
	}

	return PreviewModelVersion
}

// ComplianceBoundaryForEngine returns the compliance boundary for one engine.
func ComplianceBoundaryForEngine(engine string) string {
	if engine == EngineNormative {
		return ComplianceBoundaryNormative
	}

	return ComplianceBoundaryPreview
}

// ResolvedProvenanceMetadata returns the metadata that only becomes knowable
// once the engine is resolved, which happens after the model is loaded.  It is
// merged into the run provenance by the caller.
//
// The data-pack version is stamped only on the preview path: the normative
// chain never reads a data pack, and recording one there would reintroduce
// exactly the contradiction this split removes.
func ResolvedProvenanceMetadata(engine string) map[string]string {
	metadata := map[string]string{
		ParamEngine:           engine,
		"model_version":       ModelVersionForEngine(engine),
		"compliance_boundary": ComplianceBoundaryForEngine(engine),
	}

	if engine != EngineNormative {
		metadata["data_pack_version"] = BuiltinDataPackVersion
	}

	return metadata
}

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
// K_S is the Schienenbonus; abolished by 11. BImSchG-Änderungsgesetz
// (BGBl. 2013 I S. 1943): effective 2015-01-01 for Eisenbahnen,
// 2019-01-01 for Strassenbahnen.  K_S = 0 dB for both.
func beurteilungspegel(lpAeq, ks float64) float64 {
	return lpAeq + ks
}

// roundToWholeDB rounds a level to the nearest whole dB using round-half-away
// from zero (standard German engineering rounding for Schall 03 outputs).
func roundToWholeDB(l float64) float64 {
	return math.Round(l)
}

// kSStrecke is the Schienenbonus applied to Eisenbahn/Strassenbahn Strecken
// in Gl. 35-36.  Note: K_S does NOT apply to the Rangierbahnhof contribution.
//
// The Schienenbonus (formerly -5 dB) was abolished by the
// 11. BImSchG-Änderungsgesetz (BGBl. 2013 I S. 1943):
//   - Eisenbahnen: effective 2015-01-01
//   - Strassenbahnen: effective 2019-01-01
//
// Ref: BGBl 2014 p. 2275, Anlage 2 Nr. 2.2.18 Anmerkung 1.
const kSStrecke = 0.0

// ComputeCombinedBeurteilungspegel implements Gl. 35-36 for a location
// affected by both a Rangierbahnhof and passing trains (Strecke).
//
//	L_r,Tag   = 10·lg[ 10^(0.1·lpAeqTagR)   + 10^(0.1·(lpAeqTagStrecke   + K_S)) ]
//	L_r,Nacht = 10·lg[ 10^(0.1·lpAeqNachtR) + 10^(0.1·(lpAeqNachtStrecke + K_S)) ]
//
// lpAeqTagR and lpAeqNachtR are yard contributions from Gl. 30.
// lpAeqTagStrecke and lpAeqNachtStrecke are Strecken contributions from Gl. 29.
func ComputeCombinedBeurteilungspegel(
	lpAeqTagR, lpAeqNachtR float64,
	lpAeqTagStrecke, lpAeqNachtStrecke float64,
) (lrTag, lrNacht float64) {
	lrTag = 10 * math.Log10(
		math.Pow(10, 0.1*lpAeqTagR)+
			math.Pow(10, 0.1*(lpAeqTagStrecke+kSStrecke)),
	)
	lrNacht = 10 * math.Log10(
		math.Pow(10, 0.1*lpAeqNachtR)+
			math.Pow(10, 0.1*(lpAeqNachtStrecke+kSStrecke)),
	)

	return
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

// ProvenanceMetadata returns the Schall 03 run metadata that is knowable
// before the model is read.
//
// It deliberately stamps neither model_version nor compliance_boundary: both
// depend on which engine the run resolves to, and the previous version asserted
// a normative model version alongside a preview compliance boundary in the same
// manifest.  The caller merges ResolvedProvenanceMetadata once the engine is
// known.
func ProvenanceMetadata(params map[string]string) map[string]string {
	metadata := map[string]string{
		"reporting_precision_db": "0.1",
		"reporting_rounding":     "round-half-away-from-zero at report boundary",
		"indicator_order":        IndicatorLrDay + "," + IndicatorLrNight,
		"band_model":             "octave-63Hz-8000Hz",
	}

	for _, key := range []string{
		"receiver_height_m",
		ParamEngine,
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
