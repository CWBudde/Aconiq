package cli

import (
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

type rls19RoadRunOptions struct {
	GridResolutionM   float64
	GridPaddingM      float64
	ReceiverHeightM   float64
	SurfaceType       string
	SpeedPkwKPH       float64
	SpeedLkw1KPH      float64
	SpeedLkw2KPH      float64
	SpeedKradKPH      float64
	GradientPercent   float64
	TrafficDayPkw     float64
	TrafficDayLkw1    float64
	TrafficDayLkw2    float64
	TrafficDayKrad    float64
	TrafficNightPkw   float64
	TrafficNightLkw1  float64
	TrafficNightLkw2  float64
	TrafficNightKrad  float64
	SegmentLengthM    float64
	SegmentLengthMode string
	MinDistanceM      float64
}

// rls19RoadParamBindings binds the rls19-road parameter schema, which carries
// its own speed and traffic vocabulary rather than the shared road one.
//
// dupl matches this against schall03ParamBindings: two declarative lists of
// the same length, binding the same handful of methods, are the same token
// sequence to it whatever the parameters are called. There is nothing to
// extract — the two standards share no parameter that is not already bound
// through the shared helpers — and factoring by shape alone would put
// rls19-road's vocabulary and schall03's behind one indirection that hides
// which standard declares what. See docs/lint-triage.md.
//
//nolint:dupl // a parameter list per standard; the shape matches, the meaning does not
func rls19RoadParamBindings(options *rls19RoadRunOptions) []boundParam {
	return []boundParam{
		runParams.SurfaceType.str(&options.SurfaceType),
		runParams.GridResolutionM.float(&options.GridResolutionM),
		runParams.GridPaddingM.float(&options.GridPaddingM),
		runParams.ReceiverHeightM.float(&options.ReceiverHeightM),
		runParams.SpeedPkwKPH.float(&options.SpeedPkwKPH),
		runParams.SpeedLkw1KPH.float(&options.SpeedLkw1KPH),
		runParams.SpeedLkw2KPH.float(&options.SpeedLkw2KPH),
		runParams.SpeedKradKPH.float(&options.SpeedKradKPH),
		runParams.GradientPercent.float(&options.GradientPercent),
		runParams.TrafficDayPkw.float(&options.TrafficDayPkw),
		runParams.TrafficDayLkw1.float(&options.TrafficDayLkw1),
		runParams.TrafficDayLkw2.float(&options.TrafficDayLkw2),
		runParams.TrafficDayKrad.float(&options.TrafficDayKrad),
		runParams.TrafficNightPkw.float(&options.TrafficNightPkw),
		runParams.TrafficNightLkw1.float(&options.TrafficNightLkw1),
		runParams.TrafficNightLkw2.float(&options.TrafficNightLkw2),
		runParams.TrafficNightKrad.float(&options.TrafficNightKrad),
		runParams.SegmentLengthM.float(&options.SegmentLengthM),
		runParams.SegmentLengthMode.str(&options.SegmentLengthMode),
		runParams.MinDistanceM.float(&options.MinDistanceM),
	}
}

func parseRLS19RoadRunOptions(params map[string]string) (rls19RoadRunOptions, error) {
	options := rls19RoadRunOptions{}

	err := applyBoundParams("cli.parseRLS19RoadRunOptions", params, rls19RoadParamBindings(&options))
	if err != nil {
		return rls19RoadRunOptions{}, err
	}

	return options, nil
}

func (o rls19RoadRunOptions) PropagationConfig() rls19road.PropagationConfig {
	return rls19road.PropagationConfig{
		SegmentLengthM:    o.SegmentLengthM,
		SegmentLengthMode: rls19road.SegmentLengthMode(o.SegmentLengthMode),
		MinDistanceM:      o.MinDistanceM,
		ReceiverHeightM:   o.ReceiverHeightM,
	}
}
