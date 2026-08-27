package cli

import (
	"fmt"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
)

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
		if feature.Kind != modelgeojson.FeatureKindSource {
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
		if feature.Kind != modelgeojson.FeatureKindSource {
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
		if feature.Kind != modelgeojson.FeatureKindSource {
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
		if feature.Kind != modelgeojson.FeatureKindSource {
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
