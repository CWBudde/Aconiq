package cli

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
	"github.com/aconiq/backend/internal/standards/iso9613"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

const (
	featureKindSource           = "source"
	geometryTypeLineString      = "LineString"
	geometryTypeMultiLineString = "MultiLineString"
)

type rls19LineGeometry struct {
	Centerline           []geo.Point2D
	CenterlineElevations []float64
}

type rls19DirectionalSourceSpec struct {
	IDHint    string
	Geometry  rls19LineGeometry
	Overrides map[string]any
}

func extractDummySources(model modelgeojson.Model, emissionDB float64, supportedSourceTypes []string) ([]freefield.Source, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]freefield.Source, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != featureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractDummySources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		points, err := sourcePointsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractDummySources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("source-%03d", featureIndex)
		}

		for pointIndex, point := range points {
			sourceID := baseID
			if len(points) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, pointIndex+1)
			}

			sources = append(sources, freefield.Source{
				ID:         sourceID,
				Point:      point,
				EmissionDB: emissionDB,
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractDummySources", "model does not contain any supported source features", nil)
	}

	return sources, nil
}

//nolint:gocognit,gocyclo,cyclop,funlen,maintidx // Extracted from the former monolithic run command without changing per-feature override behavior.
func extractCnossosRoadSources(model modelgeojson.Model, options cnossosRoadRunOptions, supportedSourceTypes []string) ([]cnossosroad.RoadSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]cnossosroad.RoadSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != featureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractCnossosRoadSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("road-source-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			sourceID := baseID
			if len(lines) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			surfaceType := options.SurfaceType

			value, ok, err := featurePropertyString(feature, "road_surface_type")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				surfaceType = value
			}

			roadCategory := options.RoadCategory

			value, ok, err = featurePropertyString(feature, "road_category")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				roadCategory = value
			}

			speedKPH := options.SpeedKPH

			valueFloat, ok, err := featurePropertyFloat(feature, "road_speed_kph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				speedKPH = valueFloat
			}

			gradientPercent := options.GradientPercent

			valueFloat, ok, err = featurePropertyFloat(feature, "road_gradient_percent")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				gradientPercent = valueFloat
			}

			junctionType := options.JunctionType

			value, ok, err = featurePropertyString(feature, "road_junction_type")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				junctionType = value
			}

			junctionDistanceM := options.JunctionDistanceM

			valueFloat, ok, err = featurePropertyFloat(feature, "road_junction_distance_m")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				junctionDistanceM = valueFloat
			}

			temperatureC := options.TemperatureC

			valueFloat, ok, err = featurePropertyFloat(feature, "road_temperature_c")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				temperatureC = valueFloat
			}

			studdedTyreShare := options.StuddedTyreShare

			valueFloat, ok, err = featurePropertyFloat(feature, "road_studded_tyre_share")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				studdedTyreShare = valueFloat
			}

			trafficDayLightVPH := options.TrafficDayLightVPH

			valueFloat, ok, err = featurePropertyFloat(feature, "traffic_day_light_vph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDayLightVPH = valueFloat
			}

			trafficDayMediumVPH := options.TrafficDayMediumVPH

			valueFloat, ok, err = featurePropertyFloat(feature, "traffic_day_medium_vph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDayMediumVPH = valueFloat
			}

			trafficDayHeavyVPH := options.TrafficDayHeavyVPH

			valueFloat, ok, err = featurePropertyFloat(feature, "traffic_day_heavy_vph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDayHeavyVPH = valueFloat
			}

			trafficEveningLightVPH := options.TrafficEveningLightVPH

			valueFloat, ok, err = featurePropertyFloat(feature, "traffic_evening_light_vph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEveningLightVPH = valueFloat
			}

			trafficEveningMediumVPH := options.TrafficEveningMediumVPH

			valueFloat, ok, err = featurePropertyFloat(feature, "traffic_evening_medium_vph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEveningMediumVPH = valueFloat
			}

			trafficEveningHeavyVPH := options.TrafficEveningHeavyVPH

			valueFloat, ok, err = featurePropertyFloat(feature, "traffic_evening_heavy_vph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEveningHeavyVPH = valueFloat
			}

			trafficNightLightVPH := options.TrafficNightLightVPH

			valueFloat, ok, err = featurePropertyFloat(feature, "traffic_night_light_vph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNightLightVPH = valueFloat
			}

			trafficNightMediumVPH := options.TrafficNightMediumVPH

			valueFloat, ok, err = featurePropertyFloat(feature, "traffic_night_medium_vph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNightMediumVPH = valueFloat
			}

			trafficNightHeavyVPH := options.TrafficNightHeavyVPH

			valueFloat, ok, err = featurePropertyFloat(feature, "traffic_night_heavy_vph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNightHeavyVPH = valueFloat
			}

			trafficDayPTWVPH := options.TrafficDayPTWVPH

			valueFloat, ok, err = featurePropertyFloat(feature, "traffic_day_ptw_vph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficDayPTWVPH = valueFloat
			}

			trafficEveningPTWVPH := options.TrafficEveningPTWVPH

			valueFloat, ok, err = featurePropertyFloat(feature, "traffic_evening_ptw_vph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficEveningPTWVPH = valueFloat
			}

			trafficNightPTWVPH := options.TrafficNightPTWVPH

			valueFloat, ok, err = featurePropertyFloat(feature, "traffic_night_ptw_vph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trafficNightPTWVPH = valueFloat
			}

			sources = append(sources, cnossosroad.RoadSource{
				ID:                sourceID,
				Centerline:        line,
				RoadCategory:      roadCategory,
				SurfaceType:       surfaceType,
				SpeedKPH:          speedKPH,
				GradientPercent:   gradientPercent,
				JunctionType:      junctionType,
				JunctionDistanceM: junctionDistanceM,
				TemperatureC:      temperatureC,
				StuddedTyreShare:  studdedTyreShare,
				TrafficDay: cnossosroad.TrafficPeriod{
					LightVehiclesPerHour:      trafficDayLightVPH,
					MediumVehiclesPerHour:     trafficDayMediumVPH,
					HeavyVehiclesPerHour:      trafficDayHeavyVPH,
					PoweredTwoWheelersPerHour: trafficDayPTWVPH,
				},
				TrafficEvening: cnossosroad.TrafficPeriod{
					LightVehiclesPerHour:      trafficEveningLightVPH,
					MediumVehiclesPerHour:     trafficEveningMediumVPH,
					HeavyVehiclesPerHour:      trafficEveningHeavyVPH,
					PoweredTwoWheelersPerHour: trafficEveningPTWVPH,
				},
				TrafficNight: cnossosroad.TrafficPeriod{
					LightVehiclesPerHour:      trafficNightLightVPH,
					MediumVehiclesPerHour:     trafficNightMediumVPH,
					HeavyVehiclesPerHour:      trafficNightHeavyVPH,
					PoweredTwoWheelersPerHour: trafficNightPTWVPH,
				},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

//nolint:gocognit,gocyclo,cyclop,funlen // Extracted from the former monolithic run command without changing per-feature override behavior.
func extractCnossosRailSources(model modelgeojson.Model, options cnossosRailRunOptions, supportedSourceTypes []string) ([]cnossosrail.RailSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]cnossosrail.RailSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != featureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractCnossosRailSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("rail-source-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			sourceID := baseID
			if len(lines) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			tractionType := options.TractionType

			{
				value, ok, err := featurePropertyString(feature, "rail_traction_type")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					tractionType = value
				}
			}

			trackType := options.TrackType

			{
				value, ok, err := featurePropertyString(feature, "rail_track_type")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trackType = value
				}
			}

			roughnessClass := options.TrackRoughnessClass

			{
				value, ok, err := featurePropertyString(feature, "rail_track_roughness_class")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					roughnessClass = value
				}
			}

			averageSpeedKPH := options.AverageTrainSpeedKPH

			{
				value, ok, err := featurePropertyFloat(feature, "rail_average_train_speed_kph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					averageSpeedKPH = value
				}
			}

			brakingShare := options.BrakingShare

			{
				value, ok, err := featurePropertyFloat(feature, "rail_braking_share")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					brakingShare = value
				}
			}

			curveRadiusM := options.CurveRadiusM

			{
				value, ok, err := featurePropertyFloat(feature, "rail_curve_radius_m")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					curveRadiusM = value
				}
			}

			onBridge := options.OnBridge

			{
				value, ok, err := featurePropertyBool(feature, "rail_on_bridge")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					onBridge = value
				}
			}

			trafficDay := options.TrafficDayTrainsPerHour

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_day_trains_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficDay = value
				}
			}

			trafficEvening := options.TrafficEveningTrainsPerHour

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_evening_trains_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficEvening = value
				}
			}

			trafficNight := options.TrafficNightTrainsPerHour

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_night_trains_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficNight = value
				}
			}

			sources = append(sources, cnossosrail.RailSource{
				ID:                   sourceID,
				TrackCenterline:      line,
				TractionType:         tractionType,
				TrackType:            trackType,
				TrackRoughnessClass:  roughnessClass,
				AverageTrainSpeedKPH: averageSpeedKPH,
				BrakingShare:         brakingShare,
				CurveRadiusM:         curveRadiusM,
				OnBridge:             onBridge,
				TrafficDay:           cnossosrail.TrafficPeriod{TrainsPerHour: trafficDay},
				TrafficEvening:       cnossosrail.TrafficPeriod{TrainsPerHour: trafficEvening},
				TrafficNight:         cnossosrail.TrafficPeriod{TrainsPerHour: trafficNight},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

//nolint:gocognit,gocyclo,cyclop,funlen,maintidx // Extracted from the former monolithic run command without changing per-feature override behavior.
func extractBUBRoadSources(model modelgeojson.Model, options bubRoadRunOptions, supportedSourceTypes []string) ([]bubroad.RoadSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]bubroad.RoadSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != featureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractBUBRoadSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("bub-road-source-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			sourceID := baseID
			if len(lines) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			surfaceType := options.SurfaceType

			value, ok, err := featurePropertyString(feature, "road_surface_type")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				surfaceType = value
			}

			roadFunctionClass := options.RoadFunctionClass

			value, ok, err = featurePropertyString(feature, "road_function_class")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				roadFunctionClass = value
			}

			junctionType := options.JunctionType

			value, ok, err = featurePropertyString(feature, "road_junction_type")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				junctionType = value
			}

			speedKPH := options.SpeedKPH

			valueFloat, ok, err := featurePropertyFloat(feature, "road_speed_kph")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				speedKPH = valueFloat
			}

			gradientPercent := options.GradientPercent

			valueFloat, ok, err = featurePropertyFloat(feature, "road_gradient_percent")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				gradientPercent = valueFloat
			}

			junctionDistanceM := options.JunctionDistanceM

			{
				value, ok, err := featurePropertyFloat(feature, "road_junction_distance_m")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					junctionDistanceM = value
				}
			}

			temperatureC := options.TemperatureC

			{
				value, ok, err := featurePropertyFloat(feature, "road_temperature_c")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					temperatureC = value
				}
			}

			studdedTyreShare := options.StuddedTyreShare

			{
				value, ok, err := featurePropertyFloat(feature, "road_studded_tyre_share")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					studdedTyreShare = value
				}
			}

			trafficDayLightVPH := options.TrafficDayLightVPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_day_light_vph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficDayLightVPH = value
				}
			}

			trafficDayMediumVPH := options.TrafficDayMediumVPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_day_medium_vph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficDayMediumVPH = value
				}
			}

			trafficDayHeavyVPH := options.TrafficDayHeavyVPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_day_heavy_vph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficDayHeavyVPH = value
				}
			}

			trafficDayPTWVPH := options.TrafficDayPTWVPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_day_ptw_vph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficDayPTWVPH = value
				}
			}

			trafficEveningLightVPH := options.TrafficEveningLightVPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_evening_light_vph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficEveningLightVPH = value
				}
			}

			trafficEveningMediumVPH := options.TrafficEveningMediumVPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_evening_medium_vph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficEveningMediumVPH = value
				}
			}

			trafficEveningHeavyVPH := options.TrafficEveningHeavyVPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_evening_heavy_vph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficEveningHeavyVPH = value
				}
			}

			trafficEveningPTWVPH := options.TrafficEveningPTWVPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_evening_ptw_vph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficEveningPTWVPH = value
				}
			}

			trafficNightLightVPH := options.TrafficNightLightVPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_night_light_vph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficNightLightVPH = value
				}
			}

			trafficNightMediumVPH := options.TrafficNightMediumVPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_night_medium_vph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficNightMediumVPH = value
				}
			}

			trafficNightHeavyVPH := options.TrafficNightHeavyVPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_night_heavy_vph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficNightHeavyVPH = value
				}
			}

			trafficNightPTWVPH := options.TrafficNightPTWVPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_night_ptw_vph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficNightPTWVPH = value
				}
			}

			sources = append(sources, bubroad.RoadSource{
				ID:                sourceID,
				Centerline:        line,
				SurfaceType:       surfaceType,
				RoadFunctionClass: roadFunctionClass,
				SpeedKPH:          speedKPH,
				GradientPercent:   gradientPercent,
				JunctionType:      junctionType,
				JunctionDistanceM: junctionDistanceM,
				TemperatureC:      temperatureC,
				StuddedTyreShare:  studdedTyreShare,
				TrafficDay: bubroad.TrafficPeriod{
					LightVehiclesPerHour:      trafficDayLightVPH,
					MediumVehiclesPerHour:     trafficDayMediumVPH,
					HeavyVehiclesPerHour:      trafficDayHeavyVPH,
					PoweredTwoWheelersPerHour: trafficDayPTWVPH,
				},
				TrafficEvening: bubroad.TrafficPeriod{
					LightVehiclesPerHour:      trafficEveningLightVPH,
					MediumVehiclesPerHour:     trafficEveningMediumVPH,
					HeavyVehiclesPerHour:      trafficEveningHeavyVPH,
					PoweredTwoWheelersPerHour: trafficEveningPTWVPH,
				},
				TrafficNight: bubroad.TrafficPeriod{
					LightVehiclesPerHour:      trafficNightLightVPH,
					MediumVehiclesPerHour:     trafficNightMediumVPH,
					HeavyVehiclesPerHour:      trafficNightHeavyVPH,
					PoweredTwoWheelersPerHour: trafficNightPTWVPH,
				},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUBRoadSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

// rls19AcousticOverrideKeys are the feature property keys that constitute a
// per-source acoustic override for RLS-19 road. Presence of any one of these
// keys causes the source to be counted as having feature-level overrides.
var rls19AcousticOverrideKeys = []string{
	"surface_type", "road_surface_type",
	"road_speed_kph", "speed_pkw_kph", "speed_lkw1_kph", "speed_lkw2_kph", "speed_krad_kph",
	"gradient_percent", "road_gradient_percent",
	"junction_type", "road_junction_type",
	"junction_distance_m", "road_junction_distance_m",
	"reflection_surcharge_db",
	"traffic_day_pkw", "traffic_day_lkw1", "traffic_day_lkw2", "traffic_day_krad",
	"traffic_night_pkw", "traffic_night_lkw1", "traffic_night_lkw2", "traffic_night_krad",
}

// rls19FeatureHasAcousticOverrides reports whether a source feature carries any
// per-source acoustic property that would override the run-wide defaults.
func rls19FeatureHasAcousticOverrides(feature modelgeojson.Feature) bool {
	return rls19PropertiesHaveAcousticOverrides(feature.Properties)
}

func rls19PropertiesHaveAcousticOverrides(properties map[string]any) bool {
	for _, key := range rls19AcousticOverrideKeys {
		if v, ok := properties[key]; ok && v != nil {
			return true
		}
	}

	return false
}

// extractRLS19RoadSources extracts RLS-19 road sources from the normalized
// model, applying per-source feature properties as overrides over the run-wide
// defaults in options. Sources are returned in model feature order, preserving
// deterministic extraction regardless of worker count. The second return value
// is the count of source features that had at least one per-source acoustic
// override (any key listed in rls19AcousticOverrideKeys).
//
//nolint:gocognit,gocyclo,cyclop,funlen,maintidx // The override-merging rules are intentionally explicit and were preserved during extraction.
func extractRLS19RoadSources(model modelgeojson.Model, options rls19RoadRunOptions, supportedSourceTypes []string) ([]rls19road.RoadSource, int, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]rls19road.RoadSource, 0)
	overrideCount := 0

	for featureIndex, feature := range model.Features {
		if feature.Kind != featureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, 0, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractRLS19RoadSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		directionalSources, err := extractRLS19DirectionalSourceSpecs(feature)
		if err != nil {
			return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("rls19-road-source-%03d", featureIndex)
		}

		if rls19FeatureHasAcousticOverrides(feature) {
			overrideCount++
		} else {
			for _, spec := range directionalSources {
				if rls19PropertiesHaveAcousticOverrides(spec.Overrides) {
					overrideCount++
					break
				}
			}
		}

		seenSourceIDs := make(map[string]struct{}, len(directionalSources))

		for lineIndex, directional := range directionalSources {
			sourceID := baseID
			if directional.IDHint != "" {
				sourceID = fmt.Sprintf("%s-%s", baseID, normalizeDirectionalSourceID(directional.IDHint, lineIndex))
			} else if len(directionalSources) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			if _, exists := seenSourceIDs[sourceID]; exists {
				return nil, 0, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractRLS19RoadSources",
					fmt.Sprintf("feature %q contains duplicate directional source id %q", feature.ID, sourceID),
					nil,
				)
			}

			seenSourceIDs[sourceID] = struct{}{}

			properties := mergedProperties(feature.Properties, directional.Overrides)
			surfaceType := options.SurfaceType

			{
				value, ok, err := propertyString(properties, "surface_type", "road_surface_type")
				if err != nil {
					return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					surfaceType = value
				}
			}

			speedPkwKPH := options.SpeedPkwKPH
			speedLkw1KPH := options.SpeedLkw1KPH
			speedLkw2KPH := options.SpeedLkw2KPH
			speedKradKPH := options.SpeedKradKPH

			{
				value, ok, err := propertyFloat(properties, "road_speed_kph")
				if err != nil {
					return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					speedPkwKPH = value
					speedLkw1KPH = value
					speedLkw2KPH = value
					speedKradKPH = value
				}
			}

			for _, item := range []struct {
				keys   []string
				target *float64
			}{
				{[]string{"speed_pkw_kph"}, &speedPkwKPH},
				{[]string{"speed_lkw1_kph"}, &speedLkw1KPH},
				{[]string{"speed_lkw2_kph"}, &speedLkw2KPH},
				{[]string{"speed_krad_kph"}, &speedKradKPH},
			} {
				{
					value, ok, err := propertyFloat(properties, item.keys...)
					if err != nil {
						return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						*item.target = value
					}
				}
			}

			gradientPercent := options.GradientPercent

			{
				value, ok, err := propertyFloat(properties, "gradient_percent", "road_gradient_percent")
				if err != nil {
					return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					gradientPercent = value
				}
			}

			junctionDistanceM := 0.0

			{
				value, ok, err := propertyFloat(properties, "junction_distance_m", "road_junction_distance_m")
				if err != nil {
					return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					junctionDistanceM = value
				}
			}

			reflectionSurchargeDB := 0.0

			{
				value, ok, err := propertyFloat(properties, "reflection_surcharge_db")
				if err != nil {
					return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					reflectionSurchargeDB = value
				}
			}

			junctionType := rls19road.JunctionNone

			{
				value, ok, err := propertyString(properties, "junction_type", "road_junction_type")
				if err != nil {
					return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					parsed, err := rls19road.ParseJunctionType(value)
					if err != nil {
						return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
					}

					junctionType = parsed
				}
			}

			trafficDay := rls19road.TrafficInput{
				PkwPerHour:  options.TrafficDayPkw,
				Lkw1PerHour: options.TrafficDayLkw1,
				Lkw2PerHour: options.TrafficDayLkw2,
				KradPerHour: options.TrafficDayKrad,
			}
			trafficNight := rls19road.TrafficInput{
				PkwPerHour:  options.TrafficNightPkw,
				Lkw1PerHour: options.TrafficNightLkw1,
				Lkw2PerHour: options.TrafficNightLkw2,
				KradPerHour: options.TrafficNightKrad,
			}

			for _, item := range []struct {
				keys   []string
				target *float64
			}{
				{[]string{"traffic_day_pkw"}, &trafficDay.PkwPerHour},
				{[]string{"traffic_day_lkw1"}, &trafficDay.Lkw1PerHour},
				{[]string{"traffic_day_lkw2"}, &trafficDay.Lkw2PerHour},
				{[]string{"traffic_day_krad"}, &trafficDay.KradPerHour},
				{[]string{"traffic_night_pkw"}, &trafficNight.PkwPerHour},
				{[]string{"traffic_night_lkw1"}, &trafficNight.Lkw1PerHour},
				{[]string{"traffic_night_lkw2"}, &trafficNight.Lkw2PerHour},
				{[]string{"traffic_night_krad"}, &trafficNight.KradPerHour},
			} {
				{
					value, ok, err := propertyFloat(properties, item.keys...)
					if err != nil {
						return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						*item.target = value
					}
				}
			}

			source := rls19road.RoadSource{
				ID:                   sourceID,
				Centerline:           directional.Geometry.Centerline,
				CenterlineElevations: directional.Geometry.CenterlineElevations,
				SurfaceType:          rls19road.SurfaceType(surfaceType),
				Speeds: rls19road.SpeedInput{
					PkwKPH:  speedPkwKPH,
					Lkw1KPH: speedLkw1KPH,
					Lkw2KPH: speedLkw2KPH,
					KradKPH: speedKradKPH,
				},
				GradientPercent:       gradientPercent,
				JunctionType:          junctionType,
				JunctionDistanceM:     junctionDistanceM,
				ReflectionSurchargeDB: reflectionSurchargeDB,
				TrafficDay:            trafficDay,
				TrafficNight:          trafficNight,
			}

			err := source.Validate()
			if err != nil {
				return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			}

			sources = append(sources, source)
		}
	}

	if len(sources) == 0 {
		return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", "model does not contain any supported line source features", nil)
	}

	return sources, overrideCount, nil
}

//nolint:nestif // The input decoding keeps the fallback precedence explicit for directional source specs.
func extractRLS19DirectionalSourceSpecs(feature modelgeojson.Feature) ([]rls19DirectionalSourceSpec, error) {
	rawDirectionalSources, ok := feature.Properties["rls19_directional_sources"]
	if ok && rawDirectionalSources != nil {
		items, ok := rawDirectionalSources.([]any)
		if !ok || len(items) == 0 {
			return nil, fmt.Errorf("property %q must be a non-empty array", "rls19_directional_sources")
		}

		specs := make([]rls19DirectionalSourceSpec, 0, len(items))
		for idx, item := range items {
			properties, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("property %q[%d] must be an object", "rls19_directional_sources", idx)
			}

			geometryValue, ok := properties["centerline"]
			if !ok || geometryValue == nil {
				if fallback, exists := properties["coordinates"]; exists && fallback != nil {
					geometryValue = fallback
				} else {
					return nil, fmt.Errorf("property %q[%d] requires centerline or coordinates", "rls19_directional_sources", idx)
				}
			}

			geometry, err := parseRLS19LineGeometry(geometryValue, properties)
			if err != nil {
				return nil, fmt.Errorf("property %q[%d]: %w", "rls19_directional_sources", idx, err)
			}

			idHint, _, err := propertyString(properties, "id", "direction_id", "direction")
			if err != nil {
				return nil, fmt.Errorf("property %q[%d]: %w", "rls19_directional_sources", idx, err)
			}

			specs = append(specs, rls19DirectionalSourceSpec{
				IDHint:    idHint,
				Geometry:  geometry,
				Overrides: properties,
			})
		}

		return specs, nil
	}

	geometries, err := rls19LineGeometriesFromFeature(feature)
	if err != nil {
		return nil, err
	}

	specs := make([]rls19DirectionalSourceSpec, 0, len(geometries))
	for _, geometry := range geometries {
		specs = append(specs, rls19DirectionalSourceSpec{Geometry: geometry})
	}

	return specs, nil
}

func rls19LineGeometriesFromFeature(feature modelgeojson.Feature) ([]rls19LineGeometry, error) {
	switch feature.GeometryType {
	case geometryTypeLineString:
		line, err := parseRLS19LineGeometry(feature.Coordinates, feature.Properties)
		if err != nil {
			return nil, err
		}

		return []rls19LineGeometry{line}, nil
	case geometryTypeMultiLineString:
		rawLines, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiLineString coordinates must be an array")
		}

		lines := make([]rls19LineGeometry, 0, len(rawLines))
		for _, rawLine := range rawLines {
			line, err := parseRLS19LineGeometry(rawLine, feature.Properties)
			if err != nil {
				return nil, err
			}

			lines = append(lines, line)
		}

		return lines, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (rls19-road supports LineString/MultiLineString only)", feature.GeometryType)
	}
}

func parseRLS19LineGeometry(value any, properties map[string]any) (rls19LineGeometry, error) {
	centerline, elevations, hasZ, err := parseLineStringCoordinates3D(value)
	if err != nil {
		return rls19LineGeometry{}, err
	}

	{
		propertyElevations, ok, err := propertyFloatSlice(properties, "centerline_elevations")
		if err != nil {
			return rls19LineGeometry{}, err
		} else if ok {
			if len(propertyElevations) != len(centerline) {
				return rls19LineGeometry{}, fmt.Errorf("centerline_elevations length %d must match centerline length %d", len(propertyElevations), len(centerline))
			}

			elevations = propertyElevations
			hasZ = true
		}
	}

	if !hasZ {
		{
			elevationM, ok, err := propertyFloat(properties, "elevation_m")
			if err != nil {
				return rls19LineGeometry{}, err
			} else if ok {
				elevations = make([]float64, len(centerline))
				for i := range elevations {
					elevations[i] = elevationM
				}

				hasZ = true
			}
		}
	}

	geometry := rls19LineGeometry{Centerline: centerline}
	if hasZ {
		geometry.CenterlineElevations = elevations
	}

	return geometry, nil
}

func normalizeDirectionalSourceID(raw string, fallbackIndex int) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Sprintf("%02d", fallbackIndex+1)
	}

	var builder strings.Builder

	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}

	normalized := strings.Trim(builder.String(), "-_.")
	if normalized == "" {
		return fmt.Sprintf("%02d", fallbackIndex+1)
	}

	return normalized
}

//nolint:gocognit,gocyclo,cyclop,funlen,maintidx // Extracted from the former monolithic run command without changing per-feature override behavior.
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
		if feature.Kind != featureKindSource {
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

			trainClass := options.TrainClass

			{
				value, ok, err := featurePropertyString(feature, "rail_train_class")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trainClass = value
				}
			}

			tractionType := options.TractionType

			{
				value, ok, err := featurePropertyString(feature, "rail_traction_type")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					tractionType = value
				}
			}

			trackType := options.TrackType

			{
				value, ok, err := featurePropertyString(feature, "rail_track_type")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trackType = value
				}
			}

			trackForm := options.TrackForm

			{
				value, ok, err := featurePropertyString(feature, "rail_track_form")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trackForm = value
				}
			}

			roughnessClass := options.TrackRoughnessClass

			{
				value, ok, err := featurePropertyString(feature, "rail_track_roughness_class")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					roughnessClass = value
				}
			}

			averageSpeedKPH := options.AverageTrainSpeedKPH

			{
				value, ok, err := featurePropertyFloat(feature, "rail_average_train_speed_kph")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					averageSpeedKPH = value
				}
			}

			curveRadiusM := options.CurveRadiusM

			{
				value, ok, err := featurePropertyFloat(feature, "rail_curve_radius_m")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					curveRadiusM = value
				}
			}

			onBridge := options.OnBridge

			{
				value, ok, err := featurePropertyBool(feature, "rail_on_bridge")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					onBridge = value
				}
			}

			elevationM := 0.0

			{
				value, ok, err := featurePropertyFloat(feature, "elevation_m")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					elevationM = value
				}
			}

			trafficDay := options.TrafficDayTrainsPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_day_trains_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficDay = value
				}
			}

			trafficNight := options.TrafficNightTrainsPH

			{
				value, ok, err := featurePropertyFloat(feature, "traffic_night_trains_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					trafficNight = value
				}
			}

			sources = append(sources, schall03.RailSource{
				ID:              sourceID,
				TrackCenterline: line,
				ElevationM:      elevationM,
				TrainClass:      trainClass,
				AverageSpeedKPH: averageSpeedKPH,
				Infrastructure: schall03.RailInfrastructure{
					TractionType:        tractionType,
					TrackType:           trackType,
					TrackForm:           trackForm,
					TrackRoughnessClass: roughnessClass,
					OnBridge:            onBridge,
					CurveRadiusM:        curveRadiusM,
				},
				TrafficDay:   schall03.TrafficPeriod{TrainsPerHour: trafficDay},
				TrafficNight: schall03.TrafficPeriod{TrainsPerHour: trafficNight},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractSchall03Sources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

func extractRLS19Barriers(model modelgeojson.Model) ([]rls19road.Barrier, error) {
	barriers := make([]rls19road.Barrier, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "barrier" {
			continue
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Barriers", fmt.Sprintf("feature %q", feature.ID), err)
		}

		heightM, ok, err := featurePropertyFloat(feature, "height_m", "barrier_height_m")
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Barriers", fmt.Sprintf("feature %q", feature.ID), err)
		}

		if !ok {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Barriers", fmt.Sprintf("feature %q missing barrier height_m", feature.ID), nil)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("rls19-barrier-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			barrierID := baseID
			if len(lines) > 1 {
				barrierID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			barrier := rls19road.Barrier{
				ID:       barrierID,
				Geometry: line,
				HeightM:  heightM,
			}

			err := barrier.Validate()
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Barriers", fmt.Sprintf("feature %q", feature.ID), err)
			}

			barriers = append(barriers, barrier)
		}
	}

	return barriers, nil
}

func extractRLS19Buildings(model modelgeojson.Model) ([]rls19road.Building, error) {
	buildings := make([]rls19road.Building, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "building" {
			continue
		}

		polygons, err := polygonsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		heightM, ok, err := featurePropertyFloat(feature, "height_m", "building_height_m")
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		if !ok {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q missing building height_m", feature.ID), nil)
		}

		reflectionLossDB := 1.0

		{
			value, ok, err := featurePropertyFloat(feature, "reflection_loss_db")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				reflectionLossDB = value
			}
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("rls19-building-%03d", featureIndex)
		}

		for polygonIndex, polygon := range polygons {
			buildingID := baseID
			if len(polygons) > 1 {
				buildingID = fmt.Sprintf("%s-%02d", baseID, polygonIndex+1)
			}

			building := rls19road.Building{
				ID:               buildingID,
				Footprint:        polygon[0],
				HeightM:          heightM,
				ReflectionLossDB: reflectionLossDB,
			}

			err := building.Validate()
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q", feature.ID), err)
			}

			buildings = append(buildings, building)
		}
	}

	return buildings, nil
}

//nolint:gocognit,gocyclo,cyclop,dupl,funlen,maintidx // CNOSSOS and BUF aircraft extraction stay separate because the source/output types differ.
func extractCnossosAircraftSources(model modelgeojson.Model, options cnossosAircraftRunOptions, supportedSourceTypes []string) ([]cnossosaircraft.AircraftSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]cnossosaircraft.AircraftSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != featureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractCnossosAircraftSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		trackOptions := options

		{
			value, ok, err := featurePropertyFloat(feature, "track_start_height_m")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trackOptions.TrackStartHeightM = value
			}
		}

		{
			value, ok, err := featurePropertyFloat(feature, "track_end_height_m")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trackOptions.TrackEndHeightM = value
			}
		}

		tracks, err := flightTracksFromFeature(feature, trackOptions)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("aircraft-source-%03d", featureIndex)
		}

		for trackIndex, track := range tracks {
			sourceID := baseID
			if len(tracks) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, trackIndex+1)
			}

			airportID := options.AirportID

			{
				value, ok, err := featurePropertyString(feature, "airport_id")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					airportID = value
				}
			}

			runwayID := options.RunwayID

			{
				value, ok, err := featurePropertyString(feature, "runway_id")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					runwayID = value
				}
			}

			operationType := options.OperationType

			{
				value, ok, err := featurePropertyString(feature, "aircraft_operation_type")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					operationType = value
				}
			}

			aircraftClass := options.AircraftClass

			{
				value, ok, err := featurePropertyString(feature, "aircraft_class")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					aircraftClass = value
				}
			}

			procedureType := options.ProcedureType

			{
				value, ok, err := featurePropertyString(feature, "aircraft_procedure_type")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					procedureType = value
				}
			}

			thrustMode := options.ThrustMode

			{
				value, ok, err := featurePropertyString(feature, "aircraft_thrust_mode")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					thrustMode = value
				}
			}

			referencePowerLevelDB := options.ReferencePowerLevelDB

			{
				value, ok, err := featurePropertyFloat(feature, "reference_power_level_db")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					referencePowerLevelDB = value
				}
			}

			engineStateFactor := options.EngineStateFactor

			{
				value, ok, err := featurePropertyFloat(feature, "engine_state_factor")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					engineStateFactor = value
				}
			}

			bankAngleDeg := options.BankAngleDeg

			{
				value, ok, err := featurePropertyFloat(feature, "bank_angle_deg")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					bankAngleDeg = value
				}
			}

			lateralOffsetM := options.LateralOffsetM

			{
				value, ok, err := featurePropertyFloat(feature, "lateral_offset_m")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					lateralOffsetM = value
				}
			}

			movementDayPerHour := options.MovementDayPerHour

			{
				value, ok, err := featurePropertyFloat(feature, "movement_day_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					movementDayPerHour = value
				}
			}

			movementEveningPerHour := options.MovementEveningPerHour

			{
				value, ok, err := featurePropertyFloat(feature, "movement_evening_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					movementEveningPerHour = value
				}
			}

			movementNightPerHour := options.MovementNightPerHour

			{
				value, ok, err := featurePropertyFloat(feature, "movement_night_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					movementNightPerHour = value
				}
			}

			sources = append(sources, cnossosaircraft.AircraftSource{
				ID:         sourceID,
				SourceType: cnossosaircraft.SourceTypeLine,
				Airport: cnossosaircraft.AirportRef{
					AirportID: airportID,
					RunwayID:  runwayID,
				},
				OperationType:         operationType,
				AircraftClass:         aircraftClass,
				ProcedureType:         procedureType,
				ThrustMode:            thrustMode,
				FlightTrack:           track,
				LateralOffsetM:        lateralOffsetM,
				ReferencePowerLevelDB: referencePowerLevelDB,
				EngineStateFactor:     engineStateFactor,
				BankAngleDeg:          bankAngleDeg,
				MovementDay:           cnossosaircraft.MovementPeriod{MovementsPerHour: movementDayPerHour},
				MovementEvening:       cnossosaircraft.MovementPeriod{MovementsPerHour: movementEveningPerHour},
				MovementNight:         cnossosaircraft.MovementPeriod{MovementsPerHour: movementNightPerHour},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

//nolint:gocognit,gocyclo,cyclop,dupl,funlen,maintidx // CNOSSOS and BUF aircraft extraction stay separate because the source/output types differ.
func extractBUFAircraftSources(model modelgeojson.Model, options bufAircraftRunOptions, supportedSourceTypes []string) ([]bufaircraft.AircraftSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]bufaircraft.AircraftSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != featureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractBUFAircraftSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		trackOptions := options

		{
			value, ok, err := featurePropertyFloat(feature, "track_start_height_m")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trackOptions.TrackStartHeightM = value
			}
		}

		{
			value, ok, err := featurePropertyFloat(feature, "track_end_height_m")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				trackOptions.TrackEndHeightM = value
			}
		}

		tracks, err := flightTracksFromFeatureBUF(feature, trackOptions)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("buf-aircraft-source-%03d", featureIndex)
		}

		for trackIndex, track := range tracks {
			sourceID := baseID
			if len(tracks) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, trackIndex+1)
			}

			airportID := options.AirportID

			{
				value, ok, err := featurePropertyString(feature, "airport_id")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					airportID = value
				}
			}

			runwayID := options.RunwayID

			{
				value, ok, err := featurePropertyString(feature, "runway_id")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					runwayID = value
				}
			}

			operationType := options.OperationType

			{
				value, ok, err := featurePropertyString(feature, "aircraft_operation_type")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					operationType = value
				}
			}

			aircraftClass := options.AircraftClass

			{
				value, ok, err := featurePropertyString(feature, "aircraft_class")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					aircraftClass = value
				}
			}

			procedureType := options.ProcedureType

			{
				value, ok, err := featurePropertyString(feature, "aircraft_procedure_type")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					procedureType = value
				}
			}

			thrustMode := options.ThrustMode

			{
				value, ok, err := featurePropertyString(feature, "aircraft_thrust_mode")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					thrustMode = value
				}
			}

			referencePowerLevelDB := options.ReferencePowerLevelDB

			{
				value, ok, err := featurePropertyFloat(feature, "reference_power_level_db")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					referencePowerLevelDB = value
				}
			}

			engineStateFactor := options.EngineStateFactor

			{
				value, ok, err := featurePropertyFloat(feature, "engine_state_factor")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					engineStateFactor = value
				}
			}

			bankAngleDeg := options.BankAngleDeg

			{
				value, ok, err := featurePropertyFloat(feature, "bank_angle_deg")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					bankAngleDeg = value
				}
			}

			lateralOffsetM := options.LateralOffsetM

			{
				value, ok, err := featurePropertyFloat(feature, "lateral_offset_m")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					lateralOffsetM = value
				}
			}

			movementDayPerHour := options.MovementDayPerHour

			{
				value, ok, err := featurePropertyFloat(feature, "movement_day_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					movementDayPerHour = value
				}
			}

			movementEveningPerHour := options.MovementEveningPerHour

			{
				value, ok, err := featurePropertyFloat(feature, "movement_evening_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					movementEveningPerHour = value
				}
			}

			movementNightPerHour := options.MovementNightPerHour

			{
				value, ok, err := featurePropertyFloat(feature, "movement_night_per_hour")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					movementNightPerHour = value
				}
			}

			sources = append(sources, bufaircraft.AircraftSource{
				ID:         sourceID,
				SourceType: bufaircraft.SourceTypeLine,
				Airport: bufaircraft.AirportRef{
					AirportID: airportID,
					RunwayID:  runwayID,
				},
				OperationType:         operationType,
				AircraftClass:         aircraftClass,
				ProcedureType:         procedureType,
				ThrustMode:            thrustMode,
				FlightTrack:           track,
				LateralOffsetM:        lateralOffsetM,
				ReferencePowerLevelDB: referencePowerLevelDB,
				EngineStateFactor:     engineStateFactor,
				BankAngleDeg:          bankAngleDeg,
				MovementDay:           bufaircraft.MovementPeriod{MovementsPerHour: movementDayPerHour},
				MovementEvening:       bufaircraft.MovementPeriod{MovementsPerHour: movementEveningPerHour},
				MovementNight:         bufaircraft.MovementPeriod{MovementsPerHour: movementNightPerHour},
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBUFAircraftSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

//nolint:gocognit,cyclop,funlen // Building overrides are explicit and intentionally kept local to extraction.
func extractBEBBuildings(model modelgeojson.Model, options bebExposureRunOptions) ([]bebexposure.BuildingUnit, error) {
	buildings := make([]bebexposure.BuildingUnit, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "building" {
			continue
		}

		polygons, err := polygonsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("beb-building-%03d", featureIndex)
		}

		heightM := options.MinimumBuildingHeightM
		if feature.HeightM != nil && *feature.HeightM > 0 {
			heightM = *feature.HeightM
		}

		usageType := options.BuildingUsageType

		{
			value, ok, err := featurePropertyString(feature, "building_usage_type", "usage_type")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				usageType = value
			}
		}

		estimatedDwellings, hasEstimatedDwellings, err := featurePropertyFloat(feature, "estimated_dwellings")
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		estimatedPersons, hasEstimatedPersons, err := featurePropertyFloat(feature, "estimated_persons", "occupancy", "occupants")
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		floorCount, hasFloorCount, err := featurePropertyFloat(feature, "floor_count", "estimated_floors")
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		for polygonIndex, polygon := range polygons {
			buildingID := baseID
			if len(polygons) > 1 {
				buildingID = fmt.Sprintf("%s-%02d", baseID, polygonIndex+1)
			}

			var dwellingsOverride *float64

			if hasEstimatedDwellings {
				value := estimatedDwellings
				dwellingsOverride = &value
			}

			var personsOverride *float64

			if hasEstimatedPersons {
				value := estimatedPersons
				personsOverride = &value
			}

			var floorCountOverride *float64

			if hasFloorCount {
				value := floorCount
				floorCountOverride = &value
			}

			buildings = append(buildings, bebexposure.BuildingUnit{
				ID:                 buildingID,
				UsageType:          usageType,
				HeightM:            heightM,
				FloorCount:         floorCountOverride,
				EstimatedDwellings: dwellingsOverride,
				EstimatedPersons:   personsOverride,
				Footprint:          polygon,
			})
		}
	}

	if len(buildings) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", "model does not contain any building polygon features", nil)
	}

	return buildings, nil
}

//nolint:gocognit,gocyclo,cyclop,dupl,funlen,maintidx // Industry source extraction mirrors the previous explicit geometry/source-type branching.
func extractCnossosIndustrySources(model modelgeojson.Model, options cnossosIndustryRunOptions, supportedSourceTypes []string) ([]cnossosindustry.IndustrySource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]cnossosindustry.IndustrySource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != featureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType == "" {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q source_type is required for cnossos-industry", feature.ID), nil)
		}

		if _, ok := allowedSourceType[normalizedSourceType]; !ok {
			return nil, domainerrors.New(
				domainerrors.KindValidation,
				"cli.extractCnossosIndustrySources",
				fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
				nil,
			)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("industry-source-%03d", featureIndex)
		}

		switch normalizedSourceType {
		case cnossosindustry.SourceTypePoint:
			points, err := sourcePointsFromFeature(feature)
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
			}

			for pointIndex, point := range points {
				sourceID := baseID
				if len(points) > 1 {
					sourceID = fmt.Sprintf("%s-%02d", baseID, pointIndex+1)
				}

				sourceHeightM := options.SourceHeightM

				{
					value, ok, err := featurePropertyFloat(feature, "industry_source_height_m")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						sourceHeightM = value
					}
				}

				soundPowerLevelDB := options.SoundPowerLevelDB

				{
					value, ok, err := featurePropertyFloat(feature, "industry_sound_power_level_db")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						soundPowerLevelDB = value
					}
				}

				sourceCategory := options.SourceCategory

				{
					value, ok, err := featurePropertyString(feature, "industry_source_category")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						sourceCategory = value
					}
				}

				enclosureState := options.EnclosureState

				{
					value, ok, err := featurePropertyString(feature, "industry_enclosure_state")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						enclosureState = value
					}
				}

				tonalityCorrectionDB := options.TonalityCorrectionDB

				{
					value, ok, err := featurePropertyFloat(feature, "industry_tonality_correction_db")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						tonalityCorrectionDB = value
					}
				}

				impulsivityCorrectionDB := options.ImpulsivityCorrectionDB

				{
					value, ok, err := featurePropertyFloat(feature, "industry_impulsivity_correction_db")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						impulsivityCorrectionDB = value
					}
				}

				operationDayFactor := options.OperationDayFactor

				{
					value, ok, err := featurePropertyFloat(feature, "operation_day_factor")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						operationDayFactor = value
					}
				}

				operationEveningFactor := options.OperationEveningFactor

				{
					value, ok, err := featurePropertyFloat(feature, "operation_evening_factor")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						operationEveningFactor = value
					}
				}

				operationNightFactor := options.OperationNightFactor

				{
					value, ok, err := featurePropertyFloat(feature, "operation_night_factor")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						operationNightFactor = value
					}
				}

				sources = append(sources, cnossosindustry.IndustrySource{
					ID:                      sourceID,
					SourceType:              cnossosindustry.SourceTypePoint,
					SourceCategory:          sourceCategory,
					EnclosureState:          enclosureState,
					Point:                   point,
					SourceHeightM:           sourceHeightM,
					SoundPowerLevelDB:       soundPowerLevelDB,
					TonalityCorrectionDB:    tonalityCorrectionDB,
					ImpulsivityCorrectionDB: impulsivityCorrectionDB,
					OperationDay:            cnossosindustry.OperationPeriod{OperatingFactor: operationDayFactor},
					OperationEvening:        cnossosindustry.OperationPeriod{OperatingFactor: operationEveningFactor},
					OperationNight:          cnossosindustry.OperationPeriod{OperatingFactor: operationNightFactor},
				})
			}
		case cnossosindustry.SourceTypeArea:
			polygons, err := polygonsFromFeature(feature)
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
			}

			for polygonIndex, polygon := range polygons {
				sourceID := baseID
				if len(polygons) > 1 {
					sourceID = fmt.Sprintf("%s-%02d", baseID, polygonIndex+1)
				}

				sourceHeightM := options.SourceHeightM

				{
					value, ok, err := featurePropertyFloat(feature, "industry_source_height_m")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						sourceHeightM = value
					}
				}

				soundPowerLevelDB := options.SoundPowerLevelDB

				{
					value, ok, err := featurePropertyFloat(feature, "industry_sound_power_level_db")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						soundPowerLevelDB = value
					}
				}

				sourceCategory := options.SourceCategory

				{
					value, ok, err := featurePropertyString(feature, "industry_source_category")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						sourceCategory = value
					}
				}

				enclosureState := options.EnclosureState

				{
					value, ok, err := featurePropertyString(feature, "industry_enclosure_state")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						enclosureState = value
					}
				}

				tonalityCorrectionDB := options.TonalityCorrectionDB

				{
					value, ok, err := featurePropertyFloat(feature, "industry_tonality_correction_db")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						tonalityCorrectionDB = value
					}
				}

				impulsivityCorrectionDB := options.ImpulsivityCorrectionDB

				{
					value, ok, err := featurePropertyFloat(feature, "industry_impulsivity_correction_db")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						impulsivityCorrectionDB = value
					}
				}

				operationDayFactor := options.OperationDayFactor

				{
					value, ok, err := featurePropertyFloat(feature, "operation_day_factor")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						operationDayFactor = value
					}
				}

				operationEveningFactor := options.OperationEveningFactor

				{
					value, ok, err := featurePropertyFloat(feature, "operation_evening_factor")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						operationEveningFactor = value
					}
				}

				operationNightFactor := options.OperationNightFactor

				{
					value, ok, err := featurePropertyFloat(feature, "operation_night_factor")
					if err != nil {
						return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						operationNightFactor = value
					}
				}

				sources = append(sources, cnossosindustry.IndustrySource{
					ID:                      sourceID,
					SourceType:              cnossosindustry.SourceTypeArea,
					SourceCategory:          sourceCategory,
					EnclosureState:          enclosureState,
					AreaPolygon:             polygon,
					SourceHeightM:           sourceHeightM,
					SoundPowerLevelDB:       soundPowerLevelDB,
					TonalityCorrectionDB:    tonalityCorrectionDB,
					ImpulsivityCorrectionDB: impulsivityCorrectionDB,
					OperationDay:            cnossosindustry.OperationPeriod{OperatingFactor: operationDayFactor},
					OperationEvening:        cnossosindustry.OperationPeriod{OperatingFactor: operationEveningFactor},
					OperationNight:          cnossosindustry.OperationPeriod{OperatingFactor: operationNightFactor},
				})
			}
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", "model does not contain any supported point/area source features", nil)
	}

	return sources, nil
}

//nolint:gocognit,cyclop,funlen // Point-source override handling is intentionally explicit.
func extractISO9613Sources(model modelgeojson.Model, options iso9613RunOptions, supportedSourceTypes []string) ([]iso9613.PointSource, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]iso9613.PointSource, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != featureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType == "" {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q source_type is required for iso9613", feature.ID), nil)
		}

		if _, ok := allowedSourceType[normalizedSourceType]; !ok {
			return nil, domainerrors.New(
				domainerrors.KindValidation,
				"cli.extractISO9613Sources",
				fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
				nil,
			)
		}

		points, err := sourcePointsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("iso9613-source-%03d", featureIndex)
		}

		for pointIndex, point := range points {
			sourceID := baseID
			if len(points) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, pointIndex+1)
			}

			sourceHeightM := options.SourceHeightM

			{
				value, ok, err := featurePropertyFloat(feature, "iso9613_source_height_m")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					sourceHeightM = value
				}
			}

			soundPowerLevelDB := options.SoundPowerLevelDB

			{
				value, ok, err := featurePropertyFloat(feature, "iso9613_sound_power_level_db")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					soundPowerLevelDB = value
				}
			}

			directivityCorrectionDB := options.DirectivityCorrectionDB

			{
				value, ok, err := featurePropertyFloat(feature, "iso9613_directivity_correction_db")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					directivityCorrectionDB = value
				}
			}

			tonalityCorrectionDB := options.TonalityCorrectionDB

			{
				value, ok, err := featurePropertyFloat(feature, "iso9613_tonality_correction_db")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					tonalityCorrectionDB = value
				}
			}

			impulsivityCorrectionDB := options.ImpulsivityCorrectionDB

			{
				value, ok, err := featurePropertyFloat(feature, "iso9613_impulsivity_correction_db")
				if err != nil {
					return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					impulsivityCorrectionDB = value
				}
			}

			sources = append(sources, iso9613.PointSource{
				ID:                      sourceID,
				Point:                   point,
				SourceHeightM:           sourceHeightM,
				SoundPowerLevelDB:       soundPowerLevelDB,
				DirectivityCorrectionDB: directivityCorrectionDB,
				TonalityCorrectionDB:    tonalityCorrectionDB,
				ImpulsivityCorrectionDB: impulsivityCorrectionDB,
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractISO9613Sources", "model does not contain any supported point source features", nil)
	}

	return sources, nil
}

func sourcePointsFromFeature(feature modelgeojson.Feature) ([]geo.Point2D, error) {
	switch feature.GeometryType {
	case "Point":
		point, err := parsePointCoordinate(feature.Coordinates)
		if err != nil {
			return nil, err
		}

		return []geo.Point2D{point}, nil
	case "MultiPoint":
		rawPoints, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiPoint coordinates must be an array")
		}

		points := make([]geo.Point2D, 0, len(rawPoints))
		for _, raw := range rawPoints {
			point, err := parsePointCoordinate(raw)
			if err != nil {
				return nil, err
			}

			points = append(points, point)
		}

		return points, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (dummy-freefield supports Point/MultiPoint only)", feature.GeometryType)
	}
}

func lineStringsFromFeature(feature modelgeojson.Feature) ([][]geo.Point2D, error) {
	switch feature.GeometryType {
	case geometryTypeLineString:
		line, err := parseLineStringCoordinates(feature.Coordinates)
		if err != nil {
			return nil, err
		}

		return [][]geo.Point2D{line}, nil
	case geometryTypeMultiLineString:
		rawLines, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiLineString coordinates must be an array")
		}

		lines := make([][]geo.Point2D, 0, len(rawLines))
		for _, rawLine := range rawLines {
			line, err := parseLineStringCoordinates(rawLine)
			if err != nil {
				return nil, err
			}

			lines = append(lines, line)
		}

		return lines, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (cnossos-road supports LineString/MultiLineString only)", feature.GeometryType)
	}
}

func polygonsFromFeature(feature modelgeojson.Feature) ([][][]geo.Point2D, error) {
	switch feature.GeometryType {
	case "Polygon":
		polygon, err := parsePolygonCoordinates(feature.Coordinates)
		if err != nil {
			return nil, err
		}

		return [][][]geo.Point2D{polygon}, nil
	case "MultiPolygon":
		rawPolygons, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiPolygon coordinates must be an array")
		}

		polygons := make([][][]geo.Point2D, 0, len(rawPolygons))
		for _, rawPolygon := range rawPolygons {
			polygon, err := parsePolygonCoordinates(rawPolygon)
			if err != nil {
				return nil, err
			}

			polygons = append(polygons, polygon)
		}

		return polygons, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (cnossos-industry supports Point/MultiPoint/Polygon/MultiPolygon only)", feature.GeometryType)
	}
}

func flightTracksFromFeature(feature modelgeojson.Feature, options cnossosAircraftRunOptions) ([][]geo.Point3D, error) {
	switch feature.GeometryType {
	case geometryTypeLineString:
		line, err := parseFlightTrackCoordinates(feature.Coordinates, options)
		if err != nil {
			return nil, err
		}

		return [][]geo.Point3D{line}, nil
	case geometryTypeMultiLineString:
		rawLines, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiLineString coordinates must be an array")
		}

		lines := make([][]geo.Point3D, 0, len(rawLines))
		for _, rawLine := range rawLines {
			line, err := parseFlightTrackCoordinates(rawLine, options)
			if err != nil {
				return nil, err
			}

			lines = append(lines, line)
		}

		return lines, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (cnossos-aircraft supports LineString/MultiLineString only)", feature.GeometryType)
	}
}

func flightTracksFromFeatureBUF(feature modelgeojson.Feature, options bufAircraftRunOptions) ([][]geo.Point3D, error) {
	switch feature.GeometryType {
	case geometryTypeLineString:
		line, err := parseFlightTrackCoordinatesBUF(feature.Coordinates, options)
		if err != nil {
			return nil, err
		}

		return [][]geo.Point3D{line}, nil
	case geometryTypeMultiLineString:
		rawLines, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiLineString coordinates must be an array")
		}

		lines := make([][]geo.Point3D, 0, len(rawLines))
		for _, rawLine := range rawLines {
			line, err := parseFlightTrackCoordinatesBUF(rawLine, options)
			if err != nil {
				return nil, err
			}

			lines = append(lines, line)
		}

		return lines, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (buf-aircraft supports LineString/MultiLineString only)", feature.GeometryType)
	}
}

//nolint:dupl // Aircraft track interpolation is kept separate because the option types differ.
func parseFlightTrackCoordinates(value any, options cnossosAircraftRunOptions) ([]geo.Point3D, error) {
	rawPoints, ok := value.([]any)
	if !ok {
		return nil, errors.New("line coordinates must be an array")
	}

	if len(rawPoints) < 2 {
		return nil, errors.New("line coordinates must contain at least 2 points")
	}

	points := make([]geo.Point3D, 0, len(rawPoints))

	lastIndex := len(rawPoints) - 1
	for i, rawPoint := range rawPoints {
		xy, z, hasZ, err := parsePointCoordinate3D(rawPoint)
		if err != nil {
			return nil, err
		}

		if !hasZ {
			fraction := 0.0
			if lastIndex > 0 {
				fraction = float64(i) / float64(lastIndex)
			}

			z = options.TrackStartHeightM + fraction*(options.TrackEndHeightM-options.TrackStartHeightM)
		}

		points = append(points, geo.Point3D{X: xy.X, Y: xy.Y, Z: z})
	}

	return points, nil
}

//nolint:dupl // Aircraft track interpolation is kept separate because the option types differ.
func parseFlightTrackCoordinatesBUF(value any, options bufAircraftRunOptions) ([]geo.Point3D, error) {
	rawPoints, ok := value.([]any)
	if !ok {
		return nil, errors.New("line coordinates must be an array")
	}

	if len(rawPoints) < 2 {
		return nil, errors.New("line coordinates must contain at least 2 points")
	}

	points := make([]geo.Point3D, 0, len(rawPoints))

	lastIndex := len(rawPoints) - 1
	for i, rawPoint := range rawPoints {
		xy, z, hasZ, err := parsePointCoordinate3D(rawPoint)
		if err != nil {
			return nil, err
		}

		if !hasZ {
			fraction := 0.0
			if lastIndex > 0 {
				fraction = float64(i) / float64(lastIndex)
			}

			z = options.TrackStartHeightM + fraction*(options.TrackEndHeightM-options.TrackStartHeightM)
		}

		points = append(points, geo.Point3D{X: xy.X, Y: xy.Y, Z: z})
	}

	return points, nil
}

func parsePointCoordinate3D(value any) (geo.Point2D, float64, bool, error) {
	raw, ok := value.([]any)
	if !ok {
		return geo.Point2D{}, 0, false, errors.New("point coordinates must be [x,y] or [x,y,z]")
	}

	if len(raw) < 2 {
		return geo.Point2D{}, 0, false, errors.New("point coordinates must have at least 2 values")
	}

	x, err := parseCoordinateNumber(raw[0])
	if err != nil {
		return geo.Point2D{}, 0, false, err
	}

	y, err := parseCoordinateNumber(raw[1])
	if err != nil {
		return geo.Point2D{}, 0, false, err
	}

	point := geo.Point2D{X: x, Y: y}
	if !point.IsFinite() {
		return geo.Point2D{}, 0, false, errors.New("point coordinates must be finite")
	}

	if len(raw) < 3 {
		return point, 0, false, nil
	}

	z, err := parseCoordinateNumber(raw[2])
	if err != nil {
		return geo.Point2D{}, 0, false, err
	}

	if math.IsNaN(z) || math.IsInf(z, 0) {
		return geo.Point2D{}, 0, false, errors.New("point z must be finite")
	}

	return point, z, true, nil
}

func parsePolygonCoordinates(value any) ([][]geo.Point2D, error) {
	rawRings, ok := value.([]any)
	if !ok || len(rawRings) == 0 {
		return nil, errors.New("polygon coordinates must contain at least one ring")
	}

	rings := make([][]geo.Point2D, 0, len(rawRings))
	for _, rawRing := range rawRings {
		ring, err := parseRingCoordinates(rawRing)
		if err != nil {
			return nil, err
		}

		rings = append(rings, ring)
	}

	return rings, nil
}

func parseRingCoordinates(value any) ([]geo.Point2D, error) {
	rawPoints, ok := value.([]any)
	if !ok || len(rawPoints) < 4 {
		return nil, errors.New("polygon ring must contain at least 4 points")
	}

	points := make([]geo.Point2D, 0, len(rawPoints))
	for _, rawPoint := range rawPoints {
		point, err := parsePointCoordinate(rawPoint)
		if err != nil {
			return nil, err
		}

		points = append(points, point)
	}

	return points, nil
}

func parseLineStringCoordinates(value any) ([]geo.Point2D, error) {
	rawPoints, ok := value.([]any)
	if !ok {
		return nil, errors.New("line coordinates must be an array")
	}

	if len(rawPoints) < 2 {
		return nil, errors.New("line coordinates must contain at least 2 points")
	}

	points := make([]geo.Point2D, 0, len(rawPoints))
	for _, rawPoint := range rawPoints {
		point, err := parsePointCoordinate(rawPoint)
		if err != nil {
			return nil, err
		}

		points = append(points, point)
	}

	return points, nil
}

func parseLineStringCoordinates3D(value any) ([]geo.Point2D, []float64, bool, error) {
	rawPoints, ok := value.([]any)
	if !ok {
		return nil, nil, false, errors.New("line coordinates must be an array")
	}

	if len(rawPoints) < 2 {
		return nil, nil, false, errors.New("line coordinates must contain at least 2 points")
	}

	points := make([]geo.Point2D, 0, len(rawPoints))
	elevations := make([]float64, 0, len(rawPoints))
	hasAnyZ := false
	hasMissingZ := false

	for _, rawPoint := range rawPoints {
		point, z, hasZ, err := parsePointCoordinate3D(rawPoint)
		if err != nil {
			return nil, nil, false, err
		}

		points = append(points, point)
		elevations = append(elevations, z)

		if hasZ {
			hasAnyZ = true
		} else {
			hasMissingZ = true
		}
	}

	if hasAnyZ && hasMissingZ {
		return nil, nil, false, errors.New("line coordinates must use either 2D points only or 3D points for every vertex")
	}

	if !hasAnyZ {
		return points, nil, false, nil
	}

	return points, elevations, true, nil
}

func parsePointCoordinate(value any) (geo.Point2D, error) {
	raw, ok := value.([]any)
	if !ok {
		return geo.Point2D{}, errors.New("point coordinates must be [x,y]")
	}

	if len(raw) < 2 {
		return geo.Point2D{}, errors.New("point coordinates must have at least 2 values")
	}

	x, err := parseCoordinateNumber(raw[0])
	if err != nil {
		return geo.Point2D{}, err
	}

	y, err := parseCoordinateNumber(raw[1])
	if err != nil {
		return geo.Point2D{}, err
	}

	point := geo.Point2D{X: x, Y: y}
	if !point.IsFinite() {
		return geo.Point2D{}, errors.New("point coordinates must be finite")
	}

	return point, nil
}

func parseCoordinateNumber(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid numeric coordinate %q: %w", typed, err)
		}

		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported coordinate type %T", value)
	}
}
