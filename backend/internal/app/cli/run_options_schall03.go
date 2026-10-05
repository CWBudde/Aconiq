package cli

import (
	"github.com/aconiq/backend/internal/standards/schall03"
)

type schall03RunOptions struct {
	Engine                string
	GridResolutionM       float64
	GridPaddingM          float64
	ReceiverHeightM       float64
	TrainClass            string
	TractionType          string
	TrackType             string
	TrackForm             string
	TrackRoughnessClass   string
	AverageTrainSpeedKPH  float64
	CurveRadiusM          float64
	OnBridge              bool
	TrafficDayTrainsPH    float64
	TrafficNightTrainsPH  float64
	AirAbsorptionDBPerKM  float64
	GroundAttenuationDB   float64
	SlabTrackCorrectionDB float64
	BridgeCorrectionDB    float64
	CurveCorrectionDB     float64
	MinDistanceM          float64
}

// schall03ParamBindings binds the schall03 parameter schema.
//
//nolint:dupl // see rls19RoadParamBindings: same shape, different vocabulary
func schall03ParamBindings(options *schall03RunOptions) []boundParam {
	return []boundParam{
		runParams.GridResolutionM.float(&options.GridResolutionM),
		runParams.GridPaddingM.float(&options.GridPaddingM),
		runParams.ReceiverHeightM.float(&options.ReceiverHeightM),
		runParams.RailAverageTrainSpeedKPH.float(&options.AverageTrainSpeedKPH),
		runParams.RailCurveRadiusM.float(&options.CurveRadiusM),
		runParams.TrafficDayTrainsPerHour.float(&options.TrafficDayTrainsPH),
		runParams.TrafficNightTrainsPerHour.float(&options.TrafficNightTrainsPH),
		runParams.AirAbsorptionDBPerKM.float(&options.AirAbsorptionDBPerKM),
		runParams.GroundAttenuationDB.float(&options.GroundAttenuationDB),
		runParams.SlabTrackCorrectionDB.float(&options.SlabTrackCorrectionDB),
		runParams.BridgeCorrectionDB.float(&options.BridgeCorrectionDB),
		runParams.CurveCorrectionDB.float(&options.CurveCorrectionDB),
		runParams.MinDistanceM.float(&options.MinDistanceM),
		runParams.RailTractionType.str(&options.TractionType),
		runParams.RailTrainClass.str(&options.TrainClass),
		runParams.RailTrackType.str(&options.TrackType),
		runParams.RailTrackForm.str(&options.TrackForm),
		runParams.RailTrackRoughnessClass.str(&options.TrackRoughnessClass),
		runParams.Schall03Engine.str(&options.Engine),
		runParams.RailOnBridge.boolean(&options.OnBridge),
	}
}

func parseSchall03RunOptions(params map[string]string) (schall03RunOptions, error) {
	options := schall03RunOptions{}

	err := applyBoundParams("cli.parseSchall03RunOptions", params, schall03ParamBindings(&options))
	if err != nil {
		return schall03RunOptions{}, err
	}

	return options, nil
}

func (o schall03RunOptions) PropagationConfig() schall03.PropagationConfig {
	return schall03.PropagationConfig{
		AirAbsorptionDBPerKM:  o.AirAbsorptionDBPerKM,
		GroundAttenuationDB:   o.GroundAttenuationDB,
		SlabTrackCorrectionDB: o.SlabTrackCorrectionDB,
		BridgeCorrectionDB:    o.BridgeCorrectionDB,
		CurveCorrectionDB:     o.CurveCorrectionDB,
		MinDistanceM:          o.MinDistanceM,
	}
}
