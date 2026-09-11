package cli

import (
	"fmt"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
	cnossosindustry "github.com/aconiq/backend/internal/standards/cnossos/industry"
	cnossosrail "github.com/aconiq/backend/internal/standards/cnossos/rail"
	cnossosroad "github.com/aconiq/backend/internal/standards/cnossos/road"
)

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

			source, buildErr := buildCnossosRoadSource(feature, options, sourceID, line)
			if buildErr != nil {
				return nil, buildErr
			}

			sources = append(sources, source)
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRoadSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

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

			source := cnossosrail.RailSource{
				ID:                   sourceID,
				TrackCenterline:      line,
				TractionType:         options.TractionType,
				TrackType:            options.TrackType,
				TrackRoughnessClass:  options.TrackRoughnessClass,
				AverageTrainSpeedKPH: options.AverageTrainSpeedKPH,
				BrakingShare:         options.BrakingShare,
				CurveRadiusM:         options.CurveRadiusM,
				OnBridge:             options.OnBridge,
				TrafficDay:           cnossosrail.TrafficPeriod{TrainsPerHour: options.TrafficDayTrainsPerHour},
				TrafficEvening:       cnossosrail.TrafficPeriod{TrainsPerHour: options.TrafficEveningTrainsPerHour},
				TrafficNight:         cnossosrail.TrafficPeriod{TrainsPerHour: options.TrafficNightTrainsPerHour},
			}

			overrideErr := applyFeatureOverrides(feature, "cli.extractCnossosRailSources", []propertyOverride{
				overrideString(&source.TractionType, "rail_traction_type"),
				overrideString(&source.TrackType, "rail_track_type"),
				overrideString(&source.TrackRoughnessClass, "rail_track_roughness_class"),
				overrideFloat(&source.AverageTrainSpeedKPH, "rail_average_train_speed_kph"),
				overrideFloat(&source.BrakingShare, "rail_braking_share"),
				overrideFloat(&source.CurveRadiusM, "rail_curve_radius_m"),
				overrideBool(&source.OnBridge, "rail_on_bridge"),
				overrideFloat(&source.TrafficDay.TrainsPerHour, "traffic_day_trains_per_hour"),
				overrideFloat(&source.TrafficEvening.TrainsPerHour, "traffic_evening_trains_per_hour"),
				overrideFloat(&source.TrafficNight.TrainsPerHour, "traffic_night_trains_per_hour"),
			})
			if overrideErr != nil {
				return nil, overrideErr
			}

			sources = append(sources, source)
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosRailSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

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

		trackErr := applyFeatureOverrides(feature, "cli.extractCnossosAircraftSources", []propertyOverride{
			overrideFloat(&trackOptions.TrackStartHeightM, "track_start_height_m"),
			overrideFloat(&trackOptions.TrackEndHeightM, "track_end_height_m"),
		})
		if trackErr != nil {
			return nil, trackErr
		}

		tracks, err := flightTracksFromFeature(feature, cnossosaircraft.StandardID, trackOptions.TrackStartHeightM, trackOptions.TrackEndHeightM)
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

			source, buildErr := buildCnossosAircraftSource(feature, options, sourceID, track)
			if buildErr != nil {
				return nil, buildErr
			}

			sources = append(sources, source)
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosAircraftSources", "model does not contain any supported line source features", nil)
	}

	return sources, nil
}

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

		parts, err := cnossosIndustryParts(feature, normalizedSourceType)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		for partIndex, part := range parts {
			sourceID := baseID
			if len(parts) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, partIndex+1)
			}

			source, buildErr := buildCnossosIndustrySource(feature, options, sourceID, normalizedSourceType)
			if buildErr != nil {
				return nil, buildErr
			}

			part(&source)

			sources = append(sources, source)
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractCnossosIndustrySources", "model does not contain any supported point/area source features", nil)
	}

	return sources, nil
}

// buildCnossosIndustrySource merges the run options with one feature's
// property overrides into a single industry source. The caller assigns the
// geometry, which is the only thing the point and area branches disagree on.
func buildCnossosIndustrySource(feature modelgeojson.Feature, options cnossosIndustryRunOptions, sourceID, sourceType string) (cnossosindustry.IndustrySource, error) {
	source := cnossosindustry.IndustrySource{
		ID:                      sourceID,
		SourceType:              sourceType,
		SourceCategory:          options.SourceCategory,
		EnclosureState:          options.EnclosureState,
		SourceHeightM:           options.SourceHeightM,
		SoundPowerLevelDB:       options.SoundPowerLevelDB,
		TonalityCorrectionDB:    options.TonalityCorrectionDB,
		ImpulsivityCorrectionDB: options.ImpulsivityCorrectionDB,
		OperationDay:            cnossosindustry.OperationPeriod{OperatingFactor: options.OperationDayFactor},
		OperationEvening:        cnossosindustry.OperationPeriod{OperatingFactor: options.OperationEveningFactor},
		OperationNight:          cnossosindustry.OperationPeriod{OperatingFactor: options.OperationNightFactor},
	}

	overrideErr := applyFeatureOverrides(feature, "cli.extractCnossosIndustrySources", []propertyOverride{
		overrideFloat(&source.SourceHeightM, "industry_source_height_m"),
		overrideFloat(&source.SoundPowerLevelDB, "industry_sound_power_level_db"),
		overrideString(&source.SourceCategory, "industry_source_category"),
		overrideString(&source.EnclosureState, "industry_enclosure_state"),
		overrideFloat(&source.TonalityCorrectionDB, "industry_tonality_correction_db"),
		overrideFloat(&source.ImpulsivityCorrectionDB, "industry_impulsivity_correction_db"),
		overrideFloat(&source.OperationDay.OperatingFactor, "operation_day_factor"),
		overrideFloat(&source.OperationEvening.OperatingFactor, "operation_evening_factor"),
		overrideFloat(&source.OperationNight.OperatingFactor, "operation_night_factor"),
	})
	if overrideErr != nil {
		return cnossosindustry.IndustrySource{}, overrideErr
	}

	return source, nil
}

// cnossosIndustryParts resolves a feature's geometry into one assignment per
// source it will emit. Point and area features differ only in which field of
// the source they fill, so holding that difference in a closure lets the
// extraction loop stay single instead of being written twice.
//
// A source type that is neither point nor area yields no parts, which is what
// the switch this replaces did by having no default arm.
func cnossosIndustryParts(feature modelgeojson.Feature, sourceType string) ([]func(*cnossosindustry.IndustrySource), error) {
	switch sourceType {
	case cnossosindustry.SourceTypePoint:
		points, err := sourcePointsFromFeature(feature)
		if err != nil {
			return nil, err
		}

		parts := make([]func(*cnossosindustry.IndustrySource), 0, len(points))
		for _, point := range points {
			parts = append(parts, func(source *cnossosindustry.IndustrySource) { source.Point = point })
		}

		return parts, nil
	case cnossosindustry.SourceTypeArea:
		polygons, err := polygonsFromFeature(feature)
		if err != nil {
			return nil, err
		}

		parts := make([]func(*cnossosindustry.IndustrySource), 0, len(polygons))
		for _, polygon := range polygons {
			parts = append(parts, func(source *cnossosindustry.IndustrySource) { source.AreaPolygon = polygon })
		}

		return parts, nil
	default:
		return nil, nil
	}
}

// buildCnossosRoadSource merges the run options with one feature's property
// overrides into a single road source. Split out of extractCnossosRoadSources
// to keep the extraction loop under the length limit.
func buildCnossosRoadSource(feature modelgeojson.Feature, options cnossosRoadRunOptions, sourceID string, line []geo.Point2D) (cnossosroad.RoadSource, error) {
	source := cnossosroad.RoadSource{
		ID:                sourceID,
		Centerline:        line,
		RoadCategory:      options.RoadCategory,
		SurfaceType:       options.SurfaceType,
		SpeedKPH:          options.SpeedKPH,
		GradientPercent:   options.GradientPercent,
		JunctionType:      options.JunctionType,
		JunctionDistanceM: options.JunctionDistanceM,
		TemperatureC:      options.TemperatureC,
		StuddedTyreShare:  options.StuddedTyreShare,
		TrafficDay: cnossosroad.TrafficPeriod{
			LightVehiclesPerHour:      options.TrafficDayLightVPH,
			MediumVehiclesPerHour:     options.TrafficDayMediumVPH,
			HeavyVehiclesPerHour:      options.TrafficDayHeavyVPH,
			PoweredTwoWheelersPerHour: options.TrafficDayPTWVPH,
		},
		TrafficEvening: cnossosroad.TrafficPeriod{
			LightVehiclesPerHour:      options.TrafficEveningLightVPH,
			MediumVehiclesPerHour:     options.TrafficEveningMediumVPH,
			HeavyVehiclesPerHour:      options.TrafficEveningHeavyVPH,
			PoweredTwoWheelersPerHour: options.TrafficEveningPTWVPH,
		},
		TrafficNight: cnossosroad.TrafficPeriod{
			LightVehiclesPerHour:      options.TrafficNightLightVPH,
			MediumVehiclesPerHour:     options.TrafficNightMediumVPH,
			HeavyVehiclesPerHour:      options.TrafficNightHeavyVPH,
			PoweredTwoWheelersPerHour: options.TrafficNightPTWVPH,
		},
	}

	overrideErr := applyFeatureOverrides(feature, "cli.extractCnossosRoadSources", []propertyOverride{
		overrideString(&source.SurfaceType, "road_surface_type"),
		overrideString(&source.RoadCategory, "road_category"),
		overrideFloat(&source.SpeedKPH, "road_speed_kph"),
		overrideFloat(&source.GradientPercent, "road_gradient_percent"),
		overrideString(&source.JunctionType, "road_junction_type"),
		overrideFloat(&source.JunctionDistanceM, "road_junction_distance_m"),
		overrideFloat(&source.TemperatureC, "road_temperature_c"),
		overrideFloat(&source.StuddedTyreShare, "road_studded_tyre_share"),
		overrideFloat(&source.TrafficDay.LightVehiclesPerHour, "traffic_day_light_vph"),
		overrideFloat(&source.TrafficDay.MediumVehiclesPerHour, "traffic_day_medium_vph"),
		overrideFloat(&source.TrafficDay.HeavyVehiclesPerHour, "traffic_day_heavy_vph"),
		overrideFloat(&source.TrafficEvening.LightVehiclesPerHour, "traffic_evening_light_vph"),
		overrideFloat(&source.TrafficEvening.MediumVehiclesPerHour, "traffic_evening_medium_vph"),
		overrideFloat(&source.TrafficEvening.HeavyVehiclesPerHour, "traffic_evening_heavy_vph"),
		overrideFloat(&source.TrafficNight.LightVehiclesPerHour, "traffic_night_light_vph"),
		overrideFloat(&source.TrafficNight.MediumVehiclesPerHour, "traffic_night_medium_vph"),
		overrideFloat(&source.TrafficNight.HeavyVehiclesPerHour, "traffic_night_heavy_vph"),
		overrideFloat(&source.TrafficDay.PoweredTwoWheelersPerHour, "traffic_day_ptw_vph"),
		overrideFloat(&source.TrafficEvening.PoweredTwoWheelersPerHour, "traffic_evening_ptw_vph"),
		overrideFloat(&source.TrafficNight.PoweredTwoWheelersPerHour, "traffic_night_ptw_vph"),
	})
	if overrideErr != nil {
		return cnossosroad.RoadSource{}, overrideErr
	}

	return source, nil
}

// buildCnossosAircraftSource merges the run options with one feature's
// property overrides into a single aircraft source.
func buildCnossosAircraftSource(feature modelgeojson.Feature, options cnossosAircraftRunOptions, sourceID string, track []geo.Point3D) (cnossosaircraft.AircraftSource, error) {
	source := cnossosaircraft.AircraftSource{
		ID:         sourceID,
		SourceType: cnossosaircraft.SourceTypeLine,
		Airport: cnossosaircraft.AirportRef{
			AirportID: options.AirportID,
			RunwayID:  options.RunwayID,
		},
		OperationType:         options.OperationType,
		AircraftClass:         options.AircraftClass,
		ProcedureType:         options.ProcedureType,
		ThrustMode:            options.ThrustMode,
		FlightTrack:           track,
		LateralOffsetM:        options.LateralOffsetM,
		ReferencePowerLevelDB: options.ReferencePowerLevelDB,
		EngineStateFactor:     options.EngineStateFactor,
		BankAngleDeg:          options.BankAngleDeg,
		MovementDay:           cnossosaircraft.MovementPeriod{MovementsPerHour: options.MovementDayPerHour},
		MovementEvening:       cnossosaircraft.MovementPeriod{MovementsPerHour: options.MovementEveningPerHour},
		MovementNight:         cnossosaircraft.MovementPeriod{MovementsPerHour: options.MovementNightPerHour},
	}

	overrideErr := applyFeatureOverrides(feature, "cli.extractCnossosAircraftSources", []propertyOverride{
		overrideString(&source.Airport.AirportID, "airport_id"),
		overrideString(&source.Airport.RunwayID, "runway_id"),
		overrideString(&source.OperationType, "aircraft_operation_type"),
		overrideString(&source.AircraftClass, "aircraft_class"),
		overrideString(&source.ProcedureType, "aircraft_procedure_type"),
		overrideString(&source.ThrustMode, "aircraft_thrust_mode"),
		overrideFloat(&source.ReferencePowerLevelDB, "reference_power_level_db"),
		overrideFloat(&source.EngineStateFactor, "engine_state_factor"),
		overrideFloat(&source.BankAngleDeg, "bank_angle_deg"),
		overrideFloat(&source.LateralOffsetM, "lateral_offset_m"),
		overrideFloat(&source.MovementDay.MovementsPerHour, "movement_day_per_hour"),
		overrideFloat(&source.MovementEvening.MovementsPerHour, "movement_evening_per_hour"),
		overrideFloat(&source.MovementNight.MovementsPerHour, "movement_night_per_hour"),
	})
	if overrideErr != nil {
		return cnossosaircraft.AircraftSource{}, overrideErr
	}

	return source, nil
}
