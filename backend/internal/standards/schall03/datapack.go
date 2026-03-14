package schall03

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
)

// DataPack holds coefficient-like preview data behind a replaceable boundary.
// The bundled values remain repo-safe placeholders until a legally safe
// normative pack is provided out-of-repo.
type DataPack struct {
	Version            string              `json:"version"`
	ComplianceBoundary string              `json:"compliance_boundary"`
	Emission           EmissionDataPack    `json:"emission"`
	Propagation        PropagationDataPack `json:"propagation"`
}

type EmissionDataPack struct {
	BaseRollingSpectrum OctaveSpectrum            `json:"base_rolling_spectrum"`
	TractionSpectra     map[string]OctaveSpectrum `json:"traction_spectra"`
	RoughnessSpectra    map[string]OctaveSpectrum `json:"roughness_spectra"`
	TrainClassSpectra   map[string]OctaveSpectrum `json:"train_class_spectra"`
	TrackFormSpectra    map[string]OctaveSpectrum `json:"track_form_spectra"`
	SpeedModel          SpeedModel                `json:"speed_model"`
}

type SpeedModel struct {
	LowSpeedThresholdKPH  float64 `json:"low_speed_threshold_kph"`
	HighSpeedThresholdKPH float64 `json:"high_speed_threshold_kph"`
	LowOffsetDB           float64 `json:"low_offset_db"`
	LowSlope              float64 `json:"low_slope"`
	MidReferenceKPH       float64 `json:"mid_reference_kph"`
	MidSlope              float64 `json:"mid_slope"`
	HighOffsetDB          float64 `json:"high_offset_db"`
	HighSlope             float64 `json:"high_slope"`
	MinSpeedKPH           float64 `json:"min_speed_kph"`
	MaxSpeedKPH           float64 `json:"max_speed_kph"`
}

type PropagationDataPack struct {
	AirAbsorptionBandFactor OctaveSpectrum    `json:"air_absorption_band_factor"`
	DefaultConfig           PropagationConfig `json:"default_config"`
}

const BuiltinDataPackVersion = "builtin-schall03-preview-v2"

// BuiltinDataPack returns the bundled repo-safe Schall 03 preview data pack.
func BuiltinDataPack() DataPack {
	return DataPack{
		Version:            BuiltinDataPackVersion,
		ComplianceBoundary: "baseline-preview-no-normative-tables",
		Emission: EmissionDataPack{
			BaseRollingSpectrum: OctaveSpectrum{73, 76, 80, 84, 87, 85, 81, 76},
			TractionSpectra: map[string]OctaveSpectrum{
				TractionElectric: {3, 3, 2, 1, 0, -1, -2, -3},
				TractionDiesel:   {5, 5, 4, 3, 1, 0, -1, -2},
				TractionMixed:    {4, 4, 3, 2, 1, -0.5, -1.5, -2.5},
			},
			RoughnessSpectra: map[string]OctaveSpectrum{
				RoughnessStandard: {0, 0, 0, 0, 0, 0, 0, 0},
				RoughnessLowNoise: {0, -0.5, -1, -1.5, -2, -2, -2, -2},
				RoughnessRough:    {0.5, 1, 1.5, 2, 2.5, 2.5, 2, 1.5},
			},
			TrainClassSpectra: map[string]OctaveSpectrum{
				TrainClassPassenger: {0, 0, 0.2, 0.4, 0.6, 0.4, 0.1, 0},
				TrainClassFreight:   {1.2, 1.0, 0.8, 0.4, 0.1, -0.2, -0.4, -0.6},
				TrainClassMixed:     {0, 0, 0, 0, 0, 0, 0, 0},
			},
			TrackFormSpectra: map[string]OctaveSpectrum{
				TrackFormMainline: {0, 0, 0, 0, 0, 0, 0, 0},
				TrackFormStation:  {0.2, 0.2, 0.1, 0, 0, -0.1, -0.1, -0.2},
				TrackFormSwitches: {0.8, 0.8, 0.6, 0.4, 0.2, 0, -0.1, -0.2},
			},
			SpeedModel: SpeedModel{
				LowSpeedThresholdKPH:  80,
				HighSpeedThresholdKPH: 160,
				LowOffsetDB:           -1.5,
				LowSlope:              8,
				MidReferenceKPH:       100,
				MidSlope:              9,
				HighOffsetDB:          1.8,
				HighSlope:             6,
				MinSpeedKPH:           30,
				MaxSpeedKPH:           250,
			},
		},
		Propagation: PropagationDataPack{
			AirAbsorptionBandFactor: OctaveSpectrum{0.3, 0.4, 0.55, 0.75, 1.0, 1.35, 1.8, 2.4},
			DefaultConfig: PropagationConfig{
				AirAbsorptionDBPerKM:  0.7,
				GroundAttenuationDB:   1.2,
				SlabTrackCorrectionDB: 1.5,
				BridgeCorrectionDB:    2.0,
				CurveCorrectionDB:     4.0,
				MinDistanceM:          3.0,
			},
		},
	}
}

// LoadDataPack loads a Schall 03 data pack from JSON.
func LoadDataPack(path string) (DataPack, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return DataPack{}, fmt.Errorf("read Schall 03 data pack: %w", err)
	}

	var pack DataPack

	err = json.Unmarshal(payload, &pack)
	if err != nil {
		return DataPack{}, fmt.Errorf("decode Schall 03 data pack: %w", err)
	}

	err = pack.Validate()
	if err != nil {
		return DataPack{}, err
	}

	return pack, nil
}

// Validate checks that a data pack is structurally usable.
func (p DataPack) Validate() error {
	if p.Version == "" {
		return errors.New("schall03 data pack version is required")
	}

	if p.ComplianceBoundary == "" {
		return errors.New("schall03 data pack compliance_boundary is required")
	}

	err := p.Emission.BaseRollingSpectrum.Validate("base_rolling_spectrum")
	if err != nil {
		return err
	}

	err = validateSpectrumMap("traction_spectra", p.Emission.TractionSpectra, allowedTractionTypes)
	if err != nil {
		return err
	}

	err = validateSpectrumMap("roughness_spectra", p.Emission.RoughnessSpectra, allowedRoughnessClasses)
	if err != nil {
		return err
	}

	err = validateSpectrumMap("train_class_spectra", p.Emission.TrainClassSpectra, allowedTrainClasses)
	if err != nil {
		return err
	}

	err = validateSpectrumMap("track_form_spectra", p.Emission.TrackFormSpectra, allowedTrackForms)
	if err != nil {
		return err
	}

	err = p.Propagation.AirAbsorptionBandFactor.Validate("air_absorption_band_factor")
	if err != nil {
		return err
	}

	err = p.Propagation.DefaultConfig.Validate()
	if err != nil {
		return err
	}

	for _, item := range []struct {
		name string
		v    float64
		min  float64
	}{
		{"speed_model.low_speed_threshold_kph", p.Emission.SpeedModel.LowSpeedThresholdKPH, 0.0000001},
		{"speed_model.high_speed_threshold_kph", p.Emission.SpeedModel.HighSpeedThresholdKPH, 0.0000001},
		{"speed_model.mid_reference_kph", p.Emission.SpeedModel.MidReferenceKPH, 0.0000001},
		{"speed_model.min_speed_kph", p.Emission.SpeedModel.MinSpeedKPH, 0.0000001},
		{"speed_model.max_speed_kph", p.Emission.SpeedModel.MaxSpeedKPH, 0.0000001},
		{"speed_model.low_offset_db", p.Emission.SpeedModel.LowOffsetDB, math.Inf(-1)},
		{"speed_model.low_slope", p.Emission.SpeedModel.LowSlope, math.Inf(-1)},
		{"speed_model.mid_slope", p.Emission.SpeedModel.MidSlope, math.Inf(-1)},
		{"speed_model.high_offset_db", p.Emission.SpeedModel.HighOffsetDB, math.Inf(-1)},
		{"speed_model.high_slope", p.Emission.SpeedModel.HighSlope, math.Inf(-1)},
	} {
		if math.IsNaN(item.v) || math.IsInf(item.v, 0) || item.v < item.min {
			return fmt.Errorf("%s must be finite", item.name)
		}
	}

	if p.Emission.SpeedModel.MinSpeedKPH > p.Emission.SpeedModel.MaxSpeedKPH {
		return errors.New("speed_model min_speed_kph must be <= max_speed_kph")
	}

	if p.Emission.SpeedModel.LowSpeedThresholdKPH >= p.Emission.SpeedModel.HighSpeedThresholdKPH {
		return errors.New("speed_model low_speed_threshold_kph must be < high_speed_threshold_kph")
	}

	return nil
}

func validateSpectrumMap(name string, values map[string]OctaveSpectrum, allowed map[string]struct{}) error {
	if len(values) != len(allowed) {
		return fmt.Errorf("%s must provide %d entries", name, len(allowed))
	}

	for key := range allowed {
		spectrum, ok := values[key]
		if !ok {
			return fmt.Errorf("%s missing %q", name, key)
		}

		err := spectrum.Validate(name + "." + key)
		if err != nil {
			return err
		}
	}

	return nil
}
