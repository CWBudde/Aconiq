package cli

import (
	"github.com/aconiq/backend/internal/standards/iso9613"
)

type iso9613RunOptions struct {
	GridResolutionM         float64
	GridPaddingM            float64
	ReceiverHeightM         float64
	SourceHeightM           float64
	SoundPowerLevelDB       float64
	DirectivityCorrectionDB float64
	TonalityCorrectionDB    float64
	ImpulsivityCorrectionDB float64
	GroundFactor            float64
	AirTemperatureC         float64
	RelativeHumidityPercent float64
	MeteorologyAssumption   string
	C0Met                   float64
	MinDistanceM            float64
}

// iso9613ParamBindings binds the iso9613 parameter schema.
func iso9613ParamBindings(options *iso9613RunOptions) []boundParam {
	return []boundParam{
		runParams.MeteorologyAssumption.str(&options.MeteorologyAssumption),
		runParams.GridResolutionM.float(&options.GridResolutionM),
		runParams.GridPaddingM.float(&options.GridPaddingM),
		runParams.ReceiverHeightM.float(&options.ReceiverHeightM),
		runParams.ISO9613SourceHeightM.float(&options.SourceHeightM),
		runParams.ISO9613SoundPowerLevelDB.float(&options.SoundPowerLevelDB),
		runParams.ISO9613DirectivityCorrectionDB.float(&options.DirectivityCorrectionDB),
		runParams.ISO9613TonalityCorrectionDB.float(&options.TonalityCorrectionDB),
		runParams.ISO9613ImpulsivityCorrectionDB.float(&options.ImpulsivityCorrectionDB),
		runParams.GroundFactor.float(&options.GroundFactor),
		runParams.AirTemperatureC.float(&options.AirTemperatureC),
		runParams.RelativeHumidityPercent.float(&options.RelativeHumidityPercent),
		runParams.C0Met.float(&options.C0Met),
		runParams.MinDistanceM.float(&options.MinDistanceM),
	}
}

func parseISO9613RunOptions(params map[string]string) (iso9613RunOptions, error) {
	options := iso9613RunOptions{}

	err := applyBoundParams("cli.parseISO9613RunOptions", params, iso9613ParamBindings(&options))
	if err != nil {
		return iso9613RunOptions{}, err
	}

	return options, nil
}

func (o iso9613RunOptions) PropagationConfig() iso9613.PropagationConfig {
	return iso9613.PropagationConfig{
		GroundFactor:            o.GroundFactor,
		AirTemperatureC:         o.AirTemperatureC,
		RelativeHumidityPercent: o.RelativeHumidityPercent,
		MeteorologyAssumption:   o.MeteorologyAssumption,
		C0:                      o.C0Met,
		MinDistanceM:            o.MinDistanceM,
	}
}
