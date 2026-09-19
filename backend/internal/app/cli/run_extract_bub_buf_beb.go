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

// bubRoadDecodeOrder is the order bub-road decodes its road source properties
// in: the classification second, the junction type before the speed, and each
// powered-two-wheeler count with its own period rather than all three last.
var bubRoadDecodeOrder = []roadProperty{
	roadPropertySurfaceType,
	roadPropertyClassification,
	roadPropertyJunctionType,
	roadPropertySpeedKPH,
	roadPropertyGradientPercent,
	roadPropertyJunctionDistanceM,
	roadPropertyTemperatureC,
	roadPropertyStuddedTyreShare,
	roadPropertyTrafficDayLight,
	roadPropertyTrafficDayMedium,
	roadPropertyTrafficDayHeavy,
	roadPropertyTrafficDayPTW,
	roadPropertyTrafficEveningLight,
	roadPropertyTrafficEveningMedium,
	roadPropertyTrafficEveningHeavy,
	roadPropertyTrafficEveningPTW,
	roadPropertyTrafficNightLight,
	roadPropertyTrafficNightMedium,
	roadPropertyTrafficNightHeavy,
	roadPropertyTrafficNightPTW,
}

func extractBUBRoadSources(model modelgeojson.Model, options bubRoadRunOptions, supportedSourceTypes []string) ([]bubroad.RoadSource, error) {
	return extractSources(model, supportedSourceTypes, roadSourceExtraction(bubRoadSourceOptions(options), roadSourceSpec{
		scope:                  "cli.extractBUBRoadSources",
		idPrefix:               "bub-road-source-%03d",
		standardID:             bubroad.StandardID,
		classificationProperty: "road_function_class",
		decodeOrder:            bubRoadDecodeOrder,
	}))
}

// bubRoadSourceOptions restates the BUB road run options as the CNOSSOS ones
// the shared builder seeds a source from. The two option sets carry the same
// source defaults under the same names, except the classification, which BUB
// parameterises as a function class; the propagation terms each standard adds
// of its own are not part of a source and are left behind here.
func bubRoadSourceOptions(options bubRoadRunOptions) cnossosRoadRunOptions {
	return cnossosRoadRunOptions{
		RoadCategory:            options.RoadFunctionClass,
		SurfaceType:             options.SurfaceType,
		SpeedKPH:                options.SpeedKPH,
		GradientPercent:         options.GradientPercent,
		JunctionType:            options.JunctionType,
		JunctionDistanceM:       options.JunctionDistanceM,
		TemperatureC:            options.TemperatureC,
		StuddedTyreShare:        options.StuddedTyreShare,
		TrafficDayLightVPH:      options.TrafficDayLightVPH,
		TrafficDayMediumVPH:     options.TrafficDayMediumVPH,
		TrafficDayHeavyVPH:      options.TrafficDayHeavyVPH,
		TrafficDayPTWVPH:        options.TrafficDayPTWVPH,
		TrafficEveningLightVPH:  options.TrafficEveningLightVPH,
		TrafficEveningMediumVPH: options.TrafficEveningMediumVPH,
		TrafficEveningHeavyVPH:  options.TrafficEveningHeavyVPH,
		TrafficEveningPTWVPH:    options.TrafficEveningPTWVPH,
		TrafficNightLightVPH:    options.TrafficNightLightVPH,
		TrafficNightMediumVPH:   options.TrafficNightMediumVPH,
		TrafficNightHeavyVPH:    options.TrafficNightHeavyVPH,
		TrafficNightPTWVPH:      options.TrafficNightPTWVPH,
	}
}

func extractBUFAircraftSources(model modelgeojson.Model, options bufAircraftRunOptions, supportedSourceTypes []string) ([]bufaircraft.AircraftSource, error) {
	return extractSources(model, supportedSourceTypes, aircraftSourceExtraction(
		cnossosAircraftRunOptions(aircraftRunOptions(options)),
		"cli.extractBUFAircraftSources", "buf-aircraft-source-%03d", bufaircraft.StandardID,
	))
}

func extractBEBBuildings(model modelgeojson.Model, options bebExposureRunOptions) ([]bebexposure.BuildingUnit, error) {
	buildings := make([]bebexposure.BuildingUnit, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindBuilding {
			continue
		}

		polygons, err := polygonsFromFeature(feature, bebexposure.StandardID)
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
