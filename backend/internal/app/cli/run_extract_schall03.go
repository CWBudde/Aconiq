package cli

import (
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func extractSchall03Sources(model modelgeojson.Model, options schall03RunOptions, supportedSourceTypes []string) ([]schall03.RailSource, error) {
	return extractSources(model, supportedSourceTypes, sourceExtraction[schall03.RailSource, []geo.Point2D]{
		scope:        "cli.extractSchall03Sources",
		idPrefix:     "schall03-source-%03d",
		emptyMessage: msgNoLineSourceFeatures,
		parts:        lineStringsFromFeature,
		build: func(feature modelgeojson.Feature, sourceID string, line []geo.Point2D) (schall03.RailSource, error) {
			return buildSchall03RailSource(feature, options, sourceID, line)
		},
	})
}

// buildSchall03RailSource merges the run options with one feature's property
// overrides into a single rail source.
func buildSchall03RailSource(feature modelgeojson.Feature, options schall03RunOptions, sourceID string, line []geo.Point2D) (schall03.RailSource, error) {
	source := schall03.RailSource{
		ID:              sourceID,
		TrackCenterline: line,
		ElevationM:      0.0,
		TrainClass:      options.TrainClass,
		AverageSpeedKPH: options.AverageTrainSpeedKPH,
		Infrastructure: schall03.RailInfrastructure{
			TractionType:        options.TractionType,
			TrackType:           options.TrackType,
			TrackForm:           options.TrackForm,
			TrackRoughnessClass: options.TrackRoughnessClass,
			OnBridge:            options.OnBridge,
			CurveRadiusM:        options.CurveRadiusM,
		},
		TrafficDay:   schall03.TrafficPeriod{TrainsPerHour: options.TrafficDayTrainsPH},
		TrafficNight: schall03.TrafficPeriod{TrainsPerHour: options.TrafficNightTrainsPH},
	}

	overrideErr := applyFeatureOverrides(feature, "cli.extractSchall03Sources", []propertyOverride{
		overrideString(&source.TrainClass, "rail_train_class"),
		overrideString(&source.Infrastructure.TractionType, "rail_traction_type"),
		overrideString(&source.Infrastructure.TrackType, "rail_track_type"),
		overrideString(&source.Infrastructure.TrackForm, "rail_track_form"),
		overrideString(&source.Infrastructure.TrackRoughnessClass, "rail_track_roughness_class"),
		overrideFloat(&source.AverageSpeedKPH, "rail_average_train_speed_kph"),
		overrideFloat(&source.Infrastructure.CurveRadiusM, "rail_curve_radius_m"),
		overrideBool(&source.Infrastructure.OnBridge, "rail_on_bridge"),
		overrideFloat(&source.ElevationM, "elevation_m"),
		overrideFloat(&source.TrafficDay.TrainsPerHour, "traffic_day_trains_per_hour"),
		overrideFloat(&source.TrafficNight.TrainsPerHour, "traffic_night_trains_per_hour"),
	})
	if overrideErr != nil {
		return schall03.RailSource{}, overrideErr
	}

	return source, nil
}
