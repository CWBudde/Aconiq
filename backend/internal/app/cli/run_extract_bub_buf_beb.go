package cli

import (
	"fmt"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
)

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
