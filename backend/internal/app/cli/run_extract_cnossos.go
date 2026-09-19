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

// cnossosRoadDecodeOrder is the order cnossos-road decodes its road source
// properties in: the classification second, the speed before the junction type,
// and all three powered-two-wheeler counts after every other period count.
var cnossosRoadDecodeOrder = []roadProperty{
	roadPropertySurfaceType,
	roadPropertyClassification,
	roadPropertySpeedKPH,
	roadPropertyGradientPercent,
	roadPropertyJunctionType,
	roadPropertyJunctionDistanceM,
	roadPropertyTemperatureC,
	roadPropertyStuddedTyreShare,
	roadPropertyTrafficDayLight,
	roadPropertyTrafficDayMedium,
	roadPropertyTrafficDayHeavy,
	roadPropertyTrafficEveningLight,
	roadPropertyTrafficEveningMedium,
	roadPropertyTrafficEveningHeavy,
	roadPropertyTrafficNightLight,
	roadPropertyTrafficNightMedium,
	roadPropertyTrafficNightHeavy,
	roadPropertyTrafficDayPTW,
	roadPropertyTrafficEveningPTW,
	roadPropertyTrafficNightPTW,
}

func extractCnossosRoadSources(model modelgeojson.Model, options cnossosRoadRunOptions, supportedSourceTypes []string) ([]cnossosroad.RoadSource, error) {
	return extractSources(model, supportedSourceTypes, roadSourceExtraction(options, roadSourceSpec{
		scope:                  "cli.extractCnossosRoadSources",
		idPrefix:               "road-source-%03d",
		standardID:             cnossosroad.StandardID,
		classificationProperty: "road_category",
		decodeOrder:            cnossosRoadDecodeOrder,
	}))
}

// roadProperty names one property a road source can be overridden by.
//
// cnossos-road and bub-road accept the same twenty properties and fill the
// same fields from them — bub/road shares cnossos/road's source model — but
// they decode them in different orders, and the order is behaviour: the first
// decode error is the one the user sees, so a feature carrying two malformed
// properties reports whichever its standard reads first. Each standard
// therefore supplies its order as data, in the decode-order table above its
// own extractor, and TestExtractOverrideErrorPrecedence pins both.
type roadProperty int

const (
	roadPropertySurfaceType roadProperty = iota
	roadPropertyClassification
	roadPropertySpeedKPH
	roadPropertyGradientPercent
	roadPropertyJunctionType
	roadPropertyJunctionDistanceM
	roadPropertyTemperatureC
	roadPropertyStuddedTyreShare
	roadPropertyTrafficDayLight
	roadPropertyTrafficDayMedium
	roadPropertyTrafficDayHeavy
	roadPropertyTrafficDayPTW
	roadPropertyTrafficEveningLight
	roadPropertyTrafficEveningMedium
	roadPropertyTrafficEveningHeavy
	roadPropertyTrafficEveningPTW
	roadPropertyTrafficNightLight
	roadPropertyTrafficNightMedium
	roadPropertyTrafficNightHeavy
	roadPropertyTrafficNightPTW
	roadPropertyCount
)

// roadSourceSpec is everything one road standard contributes to the shared
// build: the scope its errors are reported under, the format its fallback
// source IDs are numbered with, the standard named in a geometry error, the
// property its classification arrives under — road_category for cnossos-road,
// road_function_class for bub-road, both filling RoadCategory — and the order
// it decodes in.
type roadSourceSpec struct {
	scope                  string
	idPrefix               string
	standardID             string
	classificationProperty string
	decodeOrder            []roadProperty
}

// roadSourceExtraction builds the spec both road standards run.
func roadSourceExtraction(options cnossosRoadRunOptions, spec roadSourceSpec) sourceExtraction[cnossosroad.RoadSource, []geo.Point2D] {
	return sourceExtraction[cnossosroad.RoadSource, []geo.Point2D]{
		scope:        spec.scope,
		idPrefix:     spec.idPrefix,
		emptyMessage: msgNoLineSourceFeatures,
		parts: func(feature modelgeojson.Feature) ([][]geo.Point2D, error) {
			return lineStringsFromFeature(feature, spec.standardID)
		},
		build: func(feature modelgeojson.Feature, sourceID string, line []geo.Point2D) (cnossosroad.RoadSource, error) {
			return buildRoadSource(feature, options, spec, sourceID, line)
		},
	}
}

// buildRoadSource merges the run options with one feature's property overrides
// into a single road source.
func buildRoadSource(feature modelgeojson.Feature, options cnossosRoadRunOptions, spec roadSourceSpec, sourceID string, line []geo.Point2D) (cnossosroad.RoadSource, error) {
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

	overrideErr := applyFeatureOverrides(feature, spec.scope, roadSourceOverrides(&source, spec))
	if overrideErr != nil {
		return cnossosroad.RoadSource{}, overrideErr
	}

	return source, nil
}

// roadSourceOverrides returns one override per property, in the order the spec
// decodes them. The table is indexed by property rather than written in an
// order, so neither standard's order is the one the code happens to be written
// in and a reordering of either is a one-line change to its own table.
func roadSourceOverrides(source *cnossosroad.RoadSource, spec roadSourceSpec) []propertyOverride {
	table := [roadPropertyCount]propertyOverride{
		roadPropertySurfaceType:          overrideString(&source.SurfaceType, "road_surface_type"),
		roadPropertyClassification:       overrideString(&source.RoadCategory, spec.classificationProperty),
		roadPropertySpeedKPH:             overrideFloat(&source.SpeedKPH, "road_speed_kph"),
		roadPropertyGradientPercent:      overrideFloat(&source.GradientPercent, "road_gradient_percent"),
		roadPropertyJunctionType:         overrideString(&source.JunctionType, "road_junction_type"),
		roadPropertyJunctionDistanceM:    overrideFloat(&source.JunctionDistanceM, "road_junction_distance_m"),
		roadPropertyTemperatureC:         overrideFloat(&source.TemperatureC, "road_temperature_c"),
		roadPropertyStuddedTyreShare:     overrideFloat(&source.StuddedTyreShare, "road_studded_tyre_share"),
		roadPropertyTrafficDayLight:      overrideFloat(&source.TrafficDay.LightVehiclesPerHour, "traffic_day_light_vph"),
		roadPropertyTrafficDayMedium:     overrideFloat(&source.TrafficDay.MediumVehiclesPerHour, "traffic_day_medium_vph"),
		roadPropertyTrafficDayHeavy:      overrideFloat(&source.TrafficDay.HeavyVehiclesPerHour, "traffic_day_heavy_vph"),
		roadPropertyTrafficDayPTW:        overrideFloat(&source.TrafficDay.PoweredTwoWheelersPerHour, "traffic_day_ptw_vph"),
		roadPropertyTrafficEveningLight:  overrideFloat(&source.TrafficEvening.LightVehiclesPerHour, "traffic_evening_light_vph"),
		roadPropertyTrafficEveningMedium: overrideFloat(&source.TrafficEvening.MediumVehiclesPerHour, "traffic_evening_medium_vph"),
		roadPropertyTrafficEveningHeavy:  overrideFloat(&source.TrafficEvening.HeavyVehiclesPerHour, "traffic_evening_heavy_vph"),
		roadPropertyTrafficEveningPTW:    overrideFloat(&source.TrafficEvening.PoweredTwoWheelersPerHour, "traffic_evening_ptw_vph"),
		roadPropertyTrafficNightLight:    overrideFloat(&source.TrafficNight.LightVehiclesPerHour, "traffic_night_light_vph"),
		roadPropertyTrafficNightMedium:   overrideFloat(&source.TrafficNight.MediumVehiclesPerHour, "traffic_night_medium_vph"),
		roadPropertyTrafficNightHeavy:    overrideFloat(&source.TrafficNight.HeavyVehiclesPerHour, "traffic_night_heavy_vph"),
		roadPropertyTrafficNightPTW:      overrideFloat(&source.TrafficNight.PoweredTwoWheelersPerHour, "traffic_night_ptw_vph"),
	}

	overrides := make([]propertyOverride, 0, len(spec.decodeOrder))
	for _, property := range spec.decodeOrder {
		overrides = append(overrides, table[property])
	}

	return overrides
}

func extractCnossosRailSources(model modelgeojson.Model, options cnossosRailRunOptions, supportedSourceTypes []string) ([]cnossosrail.RailSource, error) {
	return extractSources(model, supportedSourceTypes, sourceExtraction[cnossosrail.RailSource, []geo.Point2D]{
		scope:        "cli.extractCnossosRailSources",
		idPrefix:     "rail-source-%03d",
		emptyMessage: msgNoLineSourceFeatures,
		parts: func(feature modelgeojson.Feature) ([][]geo.Point2D, error) {
			return lineStringsFromFeature(feature, cnossosrail.StandardID)
		},
		build: func(feature modelgeojson.Feature, sourceID string, line []geo.Point2D) (cnossosrail.RailSource, error) {
			return buildCnossosRailSource(feature, options, sourceID, line)
		},
	})
}

func extractCnossosAircraftSources(model modelgeojson.Model, options cnossosAircraftRunOptions, supportedSourceTypes []string) ([]cnossosaircraft.AircraftSource, error) {
	return extractSources(model, supportedSourceTypes, aircraftSourceExtraction(
		options, "cli.extractCnossosAircraftSources", "aircraft-source-%03d", cnossosaircraft.StandardID,
	))
}

// aircraftSourceExtraction builds the spec both aircraft standards run. The
// track heights are decoded per feature, before the geometry call, because
// they feed it.
func aircraftSourceExtraction(options cnossosAircraftRunOptions, scope, idPrefix, standardID string) sourceExtraction[cnossosaircraft.AircraftSource, []geo.Point3D] {
	return sourceExtraction[cnossosaircraft.AircraftSource, []geo.Point3D]{
		scope:        scope,
		idPrefix:     idPrefix,
		emptyMessage: msgNoLineSourceFeatures,
		parts: func(feature modelgeojson.Feature) ([][]geo.Point3D, error) {
			trackOptions := options

			err := decodeOverrides(feature.Properties, []propertyOverride{
				overrideFloat(&trackOptions.TrackStartHeightM, "track_start_height_m"),
				overrideFloat(&trackOptions.TrackEndHeightM, "track_end_height_m"),
			})
			if err != nil {
				return nil, err
			}

			return flightTracksFromFeature(feature, standardID, trackOptions.TrackStartHeightM, trackOptions.TrackEndHeightM)
		},
		build: func(feature modelgeojson.Feature, sourceID string, track []geo.Point3D) (cnossosaircraft.AircraftSource, error) {
			return buildAircraftSource(feature, options, scope, sourceID, track)
		},
	}
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
		points, err := sourcePointsFromFeature(feature, cnossosindustry.StandardID)
		if err != nil {
			return nil, err
		}

		parts := make([]func(*cnossosindustry.IndustrySource), 0, len(points))
		for _, point := range points {
			parts = append(parts, func(source *cnossosindustry.IndustrySource) { source.Point = point })
		}

		return parts, nil
	case cnossosindustry.SourceTypeArea:
		polygons, err := polygonsFromFeature(feature, cnossosindustry.StandardID)
		if err != nil {
			return nil, err
		}

		parts := make([]func(*cnossosindustry.IndustrySource), 0, len(polygons))
		for _, polygon := range polygons {
			parts = append(parts, func(source *cnossosindustry.IndustrySource) { source.AreaPolygon = polygon })
		}

		return parts, nil
	default:
		// Unreachable today, and worth keeping that way loudly. The caller
		// admits only a source_type the profile lists in SupportedSourceTypes,
		// and cnossos-industry's single profile lists exactly the two arms
		// above. Whoever adds a third would otherwise get a run that succeeds
		// with every source of that type silently missing from the result.
		return nil, domainerrors.New(
			domainerrors.KindValidation,
			"cli.cnossosIndustryParts",
			fmt.Sprintf("source_type %q is declared supported by cnossos-industry but has no geometry handler", sourceType),
			nil,
		)
	}
}

// buildAircraftSource merges the run options with one feature's property
// overrides into a single aircraft source.
//
// It serves both cnossos-aircraft and buf-aircraft. Since buf/aircraft became
// an alias package the source type is one, so the BUF path reuses the result
// rather than mapping it across.
func buildAircraftSource(feature modelgeojson.Feature, options cnossosAircraftRunOptions, scope, sourceID string, track []geo.Point3D) (cnossosaircraft.AircraftSource, error) {
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

	overrideErr := applyFeatureOverrides(feature, scope, []propertyOverride{
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

// buildCnossosRailSource merges the run options with one feature's property
// overrides into a single rail source.
func buildCnossosRailSource(feature modelgeojson.Feature, options cnossosRailRunOptions, sourceID string, line []geo.Point2D) (cnossosrail.RailSource, error) {
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
		return cnossosrail.RailSource{}, overrideErr
	}

	return source, nil
}
