package cli

import (
	"fmt"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func extractSchall03Sources(model modelgeojson.Model, options schall03RunOptions, supportedSourceTypes []string) ([]schall03.RailSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]schall03.RailSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractSchall03Sources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("schall03-source-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			sourceID := baseID
			if len(lines) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

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
				return nil, overrideErr
			}

			sources = append(sources, source)
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}
