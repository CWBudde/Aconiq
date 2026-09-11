package cli

import (
	"fmt"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	bebexposure "github.com/aconiq/backend/internal/standards/beb/exposure"
	bubroad "github.com/aconiq/backend/internal/standards/bub/road"
	bufaircraft "github.com/aconiq/backend/internal/standards/buf/aircraft"
	cnossosaircraft "github.com/aconiq/backend/internal/standards/cnossos/aircraft"
)

func extractBUBRoadSources(model modelgeojson.Model, options bubRoadRunOptions, supportedSourceTypes []string) ([]bubroad.RoadSource, error) {
	return extractSources(model, supportedSourceTypes, sourceExtraction[bubroad.RoadSource, []geo.Point2D]{
		scope:        "cli.extractBUBRoadSources",
		idPrefix:     "bub-road-source-%03d",
		emptyMessage: msgNoLineSourceFeatures,
		parts:        lineStringsFromFeature,
		build: func(feature modelgeojson.Feature, sourceID string, line []geo.Point2D) (bubroad.RoadSource, error) {
			return buildBUBRoadSource(feature, options, sourceID, line)
		},
	})
}

func extractBUFAircraftSources(model modelgeojson.Model, options bufAircraftRunOptions, supportedSourceTypes []string) ([]bufaircraft.AircraftSource, error) {
	sources, err := extractSources(model, supportedSourceTypes, aircraftSourceExtraction(
		cnossosAircraftRunOptions(aircraftRunOptions(options)),
		"cli.extractBUFAircraftSources", "buf-aircraft-source-%03d", bufaircraft.StandardID,
	))
	if err != nil {
		return nil, err
	}

	converted := make([]bufaircraft.AircraftSource, 0, len(sources))
	for _, source := range sources {
		converted = append(converted, toBUFAircraftSource(source))
	}

	return converted, nil
}

// toBUFAircraftSource maps a CNOSSOS aircraft source onto the BUF one. The two
// structs are field-for-field identical, but their nested AirportRef and
// MovementPeriod types are declared per package, so this cannot be a type
// conversion. It stops being needed when buf/aircraft becomes an alias package
// over cnossos/aircraft (PLAN.md Priority 7).
func toBUFAircraftSource(source cnossosaircraft.AircraftSource) bufaircraft.AircraftSource {
	return bufaircraft.AircraftSource{
		ID:         source.ID,
		SourceType: source.SourceType,
		Airport: bufaircraft.AirportRef{
			AirportID: source.Airport.AirportID,
			RunwayID:  source.Airport.RunwayID,
		},
		OperationType:         source.OperationType,
		AircraftClass:         source.AircraftClass,
		ProcedureType:         source.ProcedureType,
		ThrustMode:            source.ThrustMode,
		FlightTrack:           source.FlightTrack,
		LateralOffsetM:        source.LateralOffsetM,
		ReferencePowerLevelDB: source.ReferencePowerLevelDB,
		EngineStateFactor:     source.EngineStateFactor,
		BankAngleDeg:          source.BankAngleDeg,
		MovementDay:           bufaircraft.MovementPeriod{MovementsPerHour: source.MovementDay.MovementsPerHour},
		MovementEvening:       bufaircraft.MovementPeriod{MovementsPerHour: source.MovementEvening.MovementsPerHour},
		MovementNight:         bufaircraft.MovementPeriod{MovementsPerHour: source.MovementNight.MovementsPerHour},
	}
}

func extractBEBBuildings(model modelgeojson.Model, options bebExposureRunOptions) ([]bebexposure.BuildingUnit, error) {
	buildings := make([]bebexposure.BuildingUnit, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindBuilding {
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

		var (
			estimatedDwellings    float64
			hasEstimatedDwellings bool
			estimatedPersons      float64
			hasEstimatedPersons   bool
			floorCount            float64
			hasFloorCount         bool
		)

		overrideErr := applyFeatureOverrides(feature, "cli.extractBEBBuildings", []propertyOverride{
			overrideString(&usageType, "building_usage_type", "usage_type"),
			overrideFloatFound(&estimatedDwellings, &hasEstimatedDwellings, "estimated_dwellings"),
			overrideFloatFound(&estimatedPersons, &hasEstimatedPersons, "estimated_persons", "occupancy", "occupants"),
			overrideFloatFound(&floorCount, &hasFloorCount, "floor_count", "estimated_floors"),
		})
		if overrideErr != nil {
			return nil, overrideErr
		}

		for polygonIndex, polygon := range polygons {
			buildingID := baseID
			if len(polygons) > 1 {
				buildingID = fmt.Sprintf("%s-%02d", baseID, polygonIndex+1)
			}

			buildings = append(buildings, bebexposure.BuildingUnit{
				ID:                 buildingID,
				UsageType:          usageType,
				HeightM:            heightM,
				FloorCount:         optionalFloat(floorCount, hasFloorCount),
				EstimatedDwellings: optionalFloat(estimatedDwellings, hasEstimatedDwellings),
				EstimatedPersons:   optionalFloat(estimatedPersons, hasEstimatedPersons),
				Footprint:          polygon,
			})
		}
	}

	if len(buildings) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractBEBBuildings", "model does not contain any building polygon features", nil)
	}

	return buildings, nil
}

// optionalFloat renders a decoded-or-absent value as the pointer the BEB
// building unit wants. It takes value by copy, so each building of a
// multi-polygon feature gets its own address rather than sharing one.
func optionalFloat(value float64, present bool) *float64 {
	if !present {
		return nil
	}

	return &value
}

// buildBUBRoadSource merges the run options with one feature's property
// overrides into a single road source.
//
// dupl matches this against buildCnossosRoadSource, and cannot see either of
// the two things that keep them apart: bubroad.RoadSource and
// cnossosroad.RoadSource are distinct types differing in one field
// (road_function_class against road_category), and the override tables are
// ordered differently on purpose — CNOSSOS decodes all three PTW periods last,
// BUB interleaves each with its period, which decides which error a feature
// carrying two malformed properties reports. Merging them here would change
// that. The duplication is real and its fix is structural: bub/road should
// share cnossos/road's source model, the way bub/rail and bub/industry already
// alias cnossos, which PLAN.md Priority 7 owns.
//
//nolint:dupl // see above; the two road source models differ by one field and the decode orders differ deliberately
func buildBUBRoadSource(feature modelgeojson.Feature, options bubRoadRunOptions, sourceID string, line []geo.Point2D) (bubroad.RoadSource, error) {
	source := bubroad.RoadSource{
		ID:                sourceID,
		Centerline:        line,
		SurfaceType:       options.SurfaceType,
		RoadFunctionClass: options.RoadFunctionClass,
		SpeedKPH:          options.SpeedKPH,
		GradientPercent:   options.GradientPercent,
		JunctionType:      options.JunctionType,
		JunctionDistanceM: options.JunctionDistanceM,
		TemperatureC:      options.TemperatureC,
		StuddedTyreShare:  options.StuddedTyreShare,
		TrafficDay: bubroad.TrafficPeriod{
			LightVehiclesPerHour:      options.TrafficDayLightVPH,
			MediumVehiclesPerHour:     options.TrafficDayMediumVPH,
			HeavyVehiclesPerHour:      options.TrafficDayHeavyVPH,
			PoweredTwoWheelersPerHour: options.TrafficDayPTWVPH,
		},
		TrafficEvening: bubroad.TrafficPeriod{
			LightVehiclesPerHour:      options.TrafficEveningLightVPH,
			MediumVehiclesPerHour:     options.TrafficEveningMediumVPH,
			HeavyVehiclesPerHour:      options.TrafficEveningHeavyVPH,
			PoweredTwoWheelersPerHour: options.TrafficEveningPTWVPH,
		},
		TrafficNight: bubroad.TrafficPeriod{
			LightVehiclesPerHour:      options.TrafficNightLightVPH,
			MediumVehiclesPerHour:     options.TrafficNightMediumVPH,
			HeavyVehiclesPerHour:      options.TrafficNightHeavyVPH,
			PoweredTwoWheelersPerHour: options.TrafficNightPTWVPH,
		},
	}

	overrideErr := applyFeatureOverrides(feature, "cli.extractBUBRoadSources", []propertyOverride{
		overrideString(&source.SurfaceType, "road_surface_type"),
		overrideString(&source.RoadFunctionClass, "road_function_class"),
		overrideString(&source.JunctionType, "road_junction_type"),
		overrideFloat(&source.SpeedKPH, "road_speed_kph"),
		overrideFloat(&source.GradientPercent, "road_gradient_percent"),
		overrideFloat(&source.JunctionDistanceM, "road_junction_distance_m"),
		overrideFloat(&source.TemperatureC, "road_temperature_c"),
		overrideFloat(&source.StuddedTyreShare, "road_studded_tyre_share"),
		overrideFloat(&source.TrafficDay.LightVehiclesPerHour, "traffic_day_light_vph"),
		overrideFloat(&source.TrafficDay.MediumVehiclesPerHour, "traffic_day_medium_vph"),
		overrideFloat(&source.TrafficDay.HeavyVehiclesPerHour, "traffic_day_heavy_vph"),
		overrideFloat(&source.TrafficDay.PoweredTwoWheelersPerHour, "traffic_day_ptw_vph"),
		overrideFloat(&source.TrafficEvening.LightVehiclesPerHour, "traffic_evening_light_vph"),
		overrideFloat(&source.TrafficEvening.MediumVehiclesPerHour, "traffic_evening_medium_vph"),
		overrideFloat(&source.TrafficEvening.HeavyVehiclesPerHour, "traffic_evening_heavy_vph"),
		overrideFloat(&source.TrafficEvening.PoweredTwoWheelersPerHour, "traffic_evening_ptw_vph"),
		overrideFloat(&source.TrafficNight.LightVehiclesPerHour, "traffic_night_light_vph"),
		overrideFloat(&source.TrafficNight.MediumVehiclesPerHour, "traffic_night_medium_vph"),
		overrideFloat(&source.TrafficNight.HeavyVehiclesPerHour, "traffic_night_heavy_vph"),
		overrideFloat(&source.TrafficNight.PoweredTwoWheelersPerHour, "traffic_night_ptw_vph"),
	})
	if overrideErr != nil {
		return bubroad.RoadSource{}, overrideErr
	}

	return source, nil
}
